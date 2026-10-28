package less

import (
	"os"
	"strings"

	"github.com/less/less.js/internal/js"
)

// loadOptions are the options lib/less-node/file-manager.js loadFile reads: the include paths, the extension to
// try, the name prefixes (plugins try "less-plugin-" first) and whether the contents stay raw bytes.
type loadOptions struct {
	paths     []string
	ext       string
	prefixes  []string
	rawBuffer bool
}

// loadedFile is a loaded file: its text (or raw bytes) and the name it was found under.
type loadedFile struct {
	contents string
	raw      []byte
	filename string
}

// nodeBuiltins are the module names require.resolve answers with themselves on Node 22.
var nodeBuiltins = map[string]bool{}

func init() {
	names := "_http_agent _http_client _http_common _http_incoming _http_outgoing _http_server _stream_duplex " +
		"_stream_passthrough _stream_readable _stream_transform _stream_wrap _stream_writable _tls_common _tls_wrap " +
		"assert assert/strict async_hooks buffer child_process cluster console constants crypto dgram " +
		"diagnostics_channel dns dns/promises domain events fs fs/promises http http2 https inspector " +
		"inspector/promises module net os path path/posix path/win32 perf_hooks process punycode querystring " +
		"readline readline/promises repl stream stream/consumers stream/promises stream/web string_decoder sys " +
		"timers timers/promises tls trace_events tty url util util/types v8 vm wasi worker_threads zlib"
	for _, n := range strings.Fields(names) {
		nodeBuiltins[n] = true
		nodeBuiltins["node:"+n] = true
	}
	for _, n := range []string{"node:sea", "node:sqlite", "node:test", "node:test/reporters"} {
		nodeBuiltins[n] = true
	}
}

// requireResolve is the part of require.resolve, called from the original's file manager, that can succeed for a
// stylesheet name: a Node built-in module resolves to its own name. Packages are never installed beside the
// command, so every other bare name is not found.
func requireResolve(id string) (string, bool) {
	if nodeBuiltins[id] {
		return id, true
	}
	return "", false
}

// loadFileWith is FileManager.loadFile: it tries every search path with every prefix, records each attempt, and
// fails with the list of names it tried.
func loadFileWith(filename, currentDirectory string, o loadOptions) (*loadedFile, *errorInfo) {
	absolute := isPathAbsoluteFM(filename)
	prefix := js.Slice16(filename, 0, 1)
	explicit := prefix == "." || prefix == "/"
	var tried []string
	paths := []string{currentDirectory}
	if absolute {
		paths = []string{""}
	}
	paths = append(paths, o.paths...)
	if !absolute {
		hasDot := false
		for _, p := range paths {
			hasDot = hasDot || p == "."
		}
		if !hasDot {
			paths = append(paths, ".")
		}
	}
	prefixes := o.prefixes
	if len(prefixes) == 0 {
		prefixes = []string{""}
	}
	parts := extractURLParts(filename)
	for _, dir := range paths {
		for _, pfx := range prefixes {
			isNodeModule := false
			full := parts.rawPath + pfx + parts.filename
			if dir != "" {
				if strings.HasPrefix(dir, "#") {
					full = dir[1:] + full
				} else {
					full = nodeJoin(dir, full)
				}
			}
			withExtension := func() {
				ext := full
				if o.ext != "" {
					ext = tryAppendExtension(full, o.ext)
				}
				if ext != full && !explicit && dir == "." {
					if resolved, ok := requireResolve(ext); ok {
						full, isNodeModule = resolved, true
						return
					}
					tried = append(tried, "npm://"+ext)
				}
				full = ext
			}
			if !explicit && dir == "." {
				if resolved, ok := requireResolve(full); ok {
					full, isNodeModule = resolved, true
				} else {
					tried = append(tried, "npm://"+full)
					withExtension()
				}
			} else {
				withExtension()
			}
			data, err := os.ReadFile(full)
			if err != nil {
				if isNodeModule {
					tried = append(tried, "npm://"+full)
				} else {
					tried = append(tried, full)
				}
				continue
			}
			if o.rawBuffer {
				return &loadedFile{raw: data, filename: full}, nil
			}
			return &loadedFile{contents: js.DecodeUTF8(data), filename: full}, nil
		}
	}
	return nil, &errorInfo{Type: "File", Message: "'" + filename + "' wasn't found. Tried - " + strings.Join(tried, ",")}
}

// loadFile loads a stylesheet as text.
func loadFile(filename, currentDirectory string, optPaths []string, ext string) (string, string, *errorInfo) {
	f, err := loadFileWith(filename, currentDirectory, loadOptions{paths: optPaths, ext: ext})
	if err != nil {
		return "", "", err
	}
	return f.contents, f.filename, nil
}

// isExplicitPlugin is the plugin loader's test for a name that is used as given: a relative or absolute path, or
// a name ending in .js. Other plugin names are also tried with the "less-plugin-" prefix.
func isExplicitPlugin(name string) bool {
	prefix := js.Slice16(name, 0, 1)
	n := js.Len16(name)
	return prefix == "." || prefix == "/" || js.ToLowerCase(js.SliceJS(name, max(0, n-3), n)) == ".js"
}

// FindPlugin reports whether the plugin lessc's --plugin option names can be found from cwd and the include
// paths, the way the plugin loader searches for it.
func FindPlugin(name, cwd string, paths []string) (found string, ok bool) {
	defer func() {
		// a name the URL pattern cannot split rejects the load, which lessc reports as a missing plugin
		if r := recover(); r != nil {
			if _, isErr := r.(*errorInfo); !isErr {
				panic(r)
			}
			found, ok = "", false
		}
	}()
	lo := loadOptions{paths: paths}
	if !isExplicitPlugin(name) {
		lo.prefixes = []string{"less-plugin-", ""}
	}
	f, err := loadFileWith(name, cwd, lo)
	if err != nil {
		return "", false
	}
	return f.filename, true
}
