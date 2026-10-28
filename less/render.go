package less

import (
	"strings"
)

// Result is what a successful render produces: the CSS, the source map text (when requested) and the imported
// files in the order they were loaded.
type Result struct {
	CSS     string
	Map     string
	Imports []string
}

// Error is a compile error; String formats it like the original's LessError.toString, styled with ANSI colours
// when color is true.
type Error struct{ info *errorInfo }

func (e *Error) Error() string { return e.info.String(nil) }

// Filename is the file the error was raised in, or "" when it has none.
func (e *Error) Filename() string { return e.info.Filename }

// Format returns the message lessc prints.
func (e *Error) Format(color bool) string {
	if color {
		return e.info.String(ansiStylize)
	}
	return e.info.String(nil)
}

// Logger receives the warnings and informational messages a render reports.
type Logger struct {
	Warn  func(string)
	Info  func(string)
	Error func(string)
}

// LogInfo reports an informational message to the installed listener, as less.logger.info does.
func LogInfo(msg string) { lessLogger.info(msg) }

// SetLogger installs the listener for warnings and information messages.
func SetLogger(l Logger) {
	lessLogger.onWarn, lessLogger.onInfo, lessLogger.onError = l.Warn, l.Info, l.Error
}

func toError(r any, files fileContents) *Error {
	e, ok := r.(*errorInfo)
	if !ok {
		panic(r)
	}
	// parse-tree.js wraps every evaluation and output error in a new LessError, which recomputes the position
	// from the index and file that the nodes attached on the way up
	return &Error{info: newLessError(e, files, "")}
}

// Render compiles a stylesheet with the given options (parse.js, render.js and parse-tree.js).
func Render(input string, opts *Options) (res *Result, rerr *Error) {
	ctx := &parseContext{paths: opts.Paths, rewriteUrls: opts.RewriteUrls, rootpath: opts.Rootpath,
		strictImports: opts.StrictImports, insecure: opts.Insecure, dumpLineNumbers: opts.DumpLineNumbers,
		compress: opts.Compress, processImports: true, quiet: opts.Quiet, quietDeprecations: opts.QuietDeprecations}
	filename := opts.Filename
	if filename == "" {
		filename = "input"
	}
	entry := filename
	if i := strings.LastIndexAny(entry, "/\\"); i >= 0 {
		entry = entry[:i+1]
	} else {
		entry = ""
	}
	rootpath := ctx.rootpath
	if rootpath != "" && !strings.HasSuffix(rootpath, "/") {
		rootpath += "/"
	}
	fi := &FileInfo{Filename: filename, Rootpath: rootpath, CurrentDirectory: entry, EntryPath: entry, RootFilename: filename}
	imports := newImportManager(ctx, fi)
	p := newParser(ctx, imports, fi, 0)
	root, perr := p.parse(input, opts)
	if root == nil {
		return nil, &Error{info: perr}
	}
	var verr *errorInfo
	if !opts.NoProcessImports {
		verr = newImportVisitor(imports).run(root)
	}
	e := perr
	if e == nil {
		e = verr
	}
	if e == nil {
		e = imports.err
	}
	if e != nil {
		if !e.isLess {
			e = newLessError(e, imports, fi.Filename)
		}
		return nil, &Error{info: e}
	}
	return toCSSRoot(root, opts, imports)
}

func toCSSRoot(root *Ruleset, opts *Options, imports *importManager) (res *Result, rerr *Error) {
	defer func() {
		if r := recover(); r != nil {
			res, rerr = nil, toError(r, imports)
		}
	}()
	evaldRoot := transformTree(root, opts)
	if opts.Compress {
		lessLogger.warn("The compress option has been deprecated. We recommend you use a dedicated css minifier, for instance see less-plugin-clean-css.")
	}
	out := &evalContext{compress: opts.Compress, dumpLineNumbers: opts.DumpLineNumbers, strictUnits: opts.StrictUnits, numPrecision: 8}
	res = &Result{}
	if opts.SourceMap != nil {
		res.CSS, res.Map = renderWithSourceMap(evaldRoot, out, opts, imports)
	} else {
		res.CSS = toCSS(evaldRoot, out)
	}
	for _, f := range imports.fileOrder {
		if f != imports.rootFilename {
			res.Imports = append(res.Imports, f)
		}
	}
	return res, nil
}

// transformTree is transform-tree.js: evaluate, then join selectors, mark visibility, apply extends and prune.
func transformTree(root *Ruleset, opts *Options) *Ruleset {
	evalEnv := newEvalContext(opts, nil)
	evald := root.eval(evalEnv).(*Ruleset)
	newJoinSelectorVisitor().run(evald)
	(&visibilityVisitor{visible: true}).run(evald)
	runExtendVisitor(evald)
	newToCSSVisitor(&evalContext{compress: opts.Compress}).v.visit(evald)
	return evald
}
