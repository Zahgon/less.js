package less

import (
	pathpkg "path"
	"strings"

	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
)

// importManager is lib/less/import-manager.js. Each load is a chain of queued file system operations that
// completes when its last operation is drained.
type importManager struct {
	contents             map[string]string
	contentsOrder        []string // keys of contents in insertion order, like a JavaScript object's
	contentsIgnoredChars map[string]int
	files                map[string]*importedFile
	fileOrder            []string
	rootFilename         string
	paths                []string
	context              *parseContext
	err                  *errorInfo
	pending              []func()
}

type importedFile struct {
	root    Node
	options *importOptions
}

func newImportManager(ctx *parseContext, root *FileInfo) *importManager {
	return &importManager{contents: map[string]string{}, contentsIgnoredChars: map[string]int{},
		files: map[string]*importedFile{}, rootFilename: root.Filename, paths: ctx.paths, context: ctx}
}

// setContents records a file's text, keeping the first-insertion order of the file names.
func (m *importManager) setContents(filename, text string) {
	if _, ok := m.contents[filename]; !ok {
		m.contentsOrder = append(m.contentsOrder, filename)
	}
	m.contents[filename] = text
}

func (m *importManager) contentsOf(filename string) (string, bool) {
	s, ok := m.contents[filename]
	return s, ok
}

// importCallback receives the error, the parsed root (or the text of an inline import), whether the file is the
// root file, and the resolved path.
type importCallback func(e *errorInfo, root Node, text *string, atRoot bool, fullPath string)

// after runs fn once n more file system operations have completed. The original reads files asynchronously,
// so a load finishes after its own open, stat, read and close requests; requests complete in the order they
// were made, which decides the order the loaded files are recorded in (the dependency list of -M).
func (m *importManager) after(n int, fn func()) {
	m.pending = append(m.pending, func() {
		if n > 1 {
			m.after(n-1, fn)
			return
		}
		fn()
	})
}

func (m *importManager) drain() {
	for len(m.pending) > 0 {
		next := m.pending[0]
		m.pending = m.pending[1:]
		next()
	}
}

func (m *importManager) push(path string, tryAppend bool, current *FileInfo, opts *importOptions, cb importCallback) {
	parsed := func(e *errorInfo, root Node, text *string, fullPath string, found bool) {
		key := fullPath
		if !found {
			key = "undefined"
		}
		atRoot := found && fullPath == m.rootFilename
		if isTrue(opts.optional) && e != nil {
			cb(nil, newRuleset(nil, []Node{}, false, nil), nil, false, "")
			name := "undefined"
			if found {
				name = fullPath
			}
			lessLogger.info("The file " + name + " was skipped because it was not found and the import was marked optional.")
			return
		}
		if _, ok := m.files[key]; !ok && !isTrue(opts.inline) {
			m.files[key] = &importedFile{root: root, options: opts}
			m.fileOrder = append(m.fileOrder, key)
		}
		if e != nil && m.err == nil {
			m.err = e
		}
		cb(e, root, text, atRoot, fullPath)
	}
	info := &FileInfo{EntryPath: current.EntryPath, Rootpath: current.Rootpath, RootFilename: current.RootFilename}
	ext := ""
	if tryAppend {
		ext = ".less"
		if opts.isPlugin {
			ext = ".js"
		}
	}
	lo := loadOptions{paths: m.context.paths, ext: ext}
	if opts.isPlugin && !isExplicitPlugin(path) {
		lo.prefixes = []string{"less-plugin-", ""}
	}
	f, err, ops := loadFileOps(path, current.CurrentDirectory, lo)
	m.after(ops, func() {
		if err != nil {
			parsed(err, nil, nil, "", false)
			return
		}
		contents, filename := strings.TrimPrefix(f.contents, "\uFEFF"), f.filename
		info.CurrentDirectory = getPath(filename)
		if m.context.rewriteUrls != 0 {
			info.Rootpath = joinPath(m.context.rootpath, pathDiff(info.CurrentDirectory, info.EntryPath))
		}
		info.Filename = filename
		m.setContents(filename, contents)
		if current.Reference || isTrue(opts.reference) {
			info.Reference = true
		}
		switch {
		case opts.isPlugin:
			parsed(&errorInfo{Type: "Plugin", Message: "JavaScript plugins cannot run in the Go implementation: " + filename}, nil, nil, filename, true)
		case isTrue(opts.inline):
			parsed(nil, nil, &contents, filename, true)
		default:
			if f, ok := m.files[filename]; ok && !isTrue(f.options.multiple) && !isTrue(opts.multiple) {
				parsed(nil, f.root, nil, filename, true)
				return
			}
			env := *m.context
			env.processImports = false
			p := newParser(&env, m, info, 0)
			root, perr := p.parse(contents, nil)
			if perr != nil {
				root = nil
				parsed(perr, nil, nil, filename, true)
				return
			}
			parsed(nil, root, nil, filename, true)
		}
	})
}

// nodeJoin is Node's path.join for POSIX paths.
func nodeJoin(a, b string) string {
	joined := a
	if b != "" {
		if joined != "" {
			joined += "/" + b
		} else {
			joined = b
		}
	}
	if joined == "" {
		return "."
	}
	trailing := strings.HasSuffix(joined, "/")
	out := pathpkg.Clean(joined)
	if trailing && out != "/" {
		out += "/"
	}
	return out
}

var (
	reHasExtension = re(`(\.[a-z]*$)|([?;].*)$`)
	reAbsoluteFM   = rei(`^(?:[a-z-]+:|\/|\\|#)`)
	reURLParts     = rei(`^((?:[a-z-]+:)?\/{2}(?:[^/?#]*\/)|([/\\]))?((?:[^/\\?#]*[/\\])*)([^/\\?#]*)([#?].*)?$`)
	reBackslashes  = jsre.MustCompile(`\\`, "g")
)

func tryAppendExtension(path, ext string) string {
	if reHasExtension.Test(path) {
		return path
	}
	return path + ext
}

func isPathAbsoluteFM(filename string) bool { return reAbsoluteFM.Test(filename) }

func getPath(filename string) string {
	j := js.LastIndexOf16(filename, "?")
	if j > 0 {
		filename = js.Slice16(filename, 0, j)
	}
	j = js.LastIndexOf16(filename, "/")
	if j < 0 {
		j = js.LastIndexOf16(filename, "\\")
	}
	if j < 0 {
		return ""
	}
	return js.Slice16(filename, 0, j+1)
}

func joinPath(base, later string) string {
	if base == "" {
		return later
	}
	return base + later
}

type urlParts struct {
	hostPart    string
	hasHost     bool
	directories []string
	rawPath     string
	filename    string
}

func extractURLParts(url string) urlParts {
	in := js.ToU16(url)
	caps := reURLParts.MatchAt(in, 0)
	if caps == nil {
		jsError("Could not parse sheet href - '" + url + "'")
	}
	group := func(g int) (string, bool) {
		if caps[2*g] < 0 {
			return "", false
		}
		return js.FromU16(in[caps[2*g]:caps[2*g+1]]), true
	}
	var r urlParts
	r.hostPart, r.hasHost = group(1)
	var raw []string
	if dirs, ok := group(3); ok && dirs != "" {
		raw = strings.Split(reBackslashes.Replace(dirs, "/"), "/")
		for _, d := range raw {
			if d == ".." {
				if len(r.directories) > 0 {
					r.directories = r.directories[:len(r.directories)-1]
				}
			} else if d != "." {
				r.directories = append(r.directories, d)
			}
		}
	}
	r.rawPath = r.hostPart + strings.Join(raw, "/")
	r.filename, _ = group(4)
	return r
}

func pathDiff(url, baseURL string) string {
	u, b := extractURLParts(url), extractURLParts(baseURL)
	if u.hostPart != b.hostPart || u.hasHost != b.hasHost {
		return ""
	}
	n := len(b.directories)
	if len(u.directories) > n {
		n = len(u.directories)
	}
	i := 0
	for ; i < n; i++ {
		if i >= len(b.directories) || i >= len(u.directories) || b.directories[i] != u.directories[i] {
			break
		}
	}
	diff := ""
	for k := 0; k < len(b.directories)-i-1; k++ {
		diff += "../"
	}
	rest := u.directories[min(i, len(u.directories)):]
	for k := 0; k < len(rest)-1; k++ {
		diff += rest[k] + "/"
	}
	return diff
}
