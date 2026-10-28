package less

import (
	"strings"

	"github.com/less/less.js/internal/jsre"
)

// Math modes (constants.js).
const (
	MathAlways         = 0
	MathParensDivision = 1
	MathParens         = 2
)

// URL rewriting modes (constants.js).
const (
	RewriteUrlsOff   = 0
	RewriteUrlsLocal = 1
	RewriteUrlsAll   = 2
)

// SourceMapOptions are the source map settings lessc passes in options.sourceMap.
type SourceMapOptions struct {
	SourceMapFullFilename       string
	SourceMapOutputFilename     string
	SourceMapFilename           string
	SourceMapRootpath           string
	SourceMapBasepath           string
	SourceMapURL                string
	OutputSourceFiles           bool
	DisableSourcemapAnnotation  bool
	SourceMapFileInline         bool
	hasSourceMapBasepath        bool
	hasSourceMapRootpath        bool
	SourceMapInputFilename      string
	sourceMapInputFilenameIsSet bool
}

// VarOption is one --global-var or --modify-var entry; values keep their insertion order.
type VarOption struct {
	Name  string
	Value *string // nil when the option had no '='
}

// Options are the render options (default-options.js plus the ones lessc sets).
type Options struct {
	Paths             []string
	Compress          bool
	Lint              bool
	Color             bool
	StrictImports     bool
	Insecure          bool
	Rootpath          string
	RewriteUrls       int
	Math              int
	StrictUnits       bool
	GlobalVars        []VarOption
	ModifyVars        []VarOption
	URLArgs           string
	JavascriptEnabled bool
	DumpLineNumbers   string
	Filename          string
	SourceMap         *SourceMapOptions
	Quiet             bool
	QuietDeprecations bool
	Silent            bool
	Depends           bool
	DisablePluginRule bool
	ImportMultiple    bool
}

// DefaultOptions returns default-options.js.
func DefaultOptions() *Options {
	return &Options{Color: true, Math: MathParensDivision}
}

type evalContext struct {
	paths             []string
	compress          bool
	math              int
	strictUnits       bool
	sourceMap         bool
	importMultiple    bool
	urlArgs           string
	javascriptEnabled bool
	importantScope    []*importantScope
	rewriteUrls       int
	frames            []Node
	calcStack         []bool
	inCalc            bool
	parensStack       []bool
	mathOn            bool
	numPrecision      int
	mediaBlocks       []Node
	mediaPath         []Node
	mediaSet          bool // mediaBlocks and mediaPath are defined (Media.eval creates them, evalTop deletes them)
	tabLevel          int
	lastRule          bool
	selectors         [][]*Selector
	firstSelector     bool
	pluginManager     bool
	dumpLineNumbers   string
}

type importantScope struct {
	important string
}

func newEvalContext(o *Options, frames []Node) *evalContext {
	c := &evalContext{frames: frames, mathOn: true}
	if o != nil {
		c.paths = o.Paths
		c.compress = o.Compress
		c.math = o.Math
		c.strictUnits = o.StrictUnits
		c.sourceMap = o.SourceMap != nil
		c.importMultiple = o.ImportMultiple
		c.urlArgs = o.URLArgs
		c.javascriptEnabled = o.JavascriptEnabled
		c.rewriteUrls = o.RewriteUrls
		c.dumpLineNumbers = o.DumpLineNumbers
	}
	return c
}

// copyEval is new contexts.Eval(context, frames): only the copied properties carry over.
func (c *evalContext) copyEval(frames []Node) *evalContext {
	return &evalContext{
		paths:             c.paths,
		compress:          c.compress,
		math:              c.math,
		strictUnits:       c.strictUnits,
		sourceMap:         c.sourceMap,
		importMultiple:    c.importMultiple,
		urlArgs:           c.urlArgs,
		javascriptEnabled: c.javascriptEnabled,
		pluginManager:     c.pluginManager,
		importantScope:    c.importantScope,
		rewriteUrls:       c.rewriteUrls,
		frames:            frames,
		mathOn:            true,
		dumpLineNumbers:   c.dumpLineNumbers,
	}
}

func (c *evalContext) enterCalc() {
	c.calcStack = append(c.calcStack, true)
	c.inCalc = true
}

func (c *evalContext) exitCalc() {
	c.calcStack = c.calcStack[:len(c.calcStack)-1]
	if len(c.calcStack) == 0 {
		c.inCalc = false
	}
}

func (c *evalContext) inParenthesis() { c.parensStack = append(c.parensStack, true) }

func (c *evalContext) outOfParenthesis() { c.parensStack = c.parensStack[:len(c.parensStack)-1] }

func (c *evalContext) isMathOn(op string) bool {
	if !c.mathOn {
		return false
	}
	if op == "/" && c.math != MathAlways && len(c.parensStack) == 0 {
		return false
	}
	if c.math > MathParensDivision {
		return len(c.parensStack) > 0
	}
	return true
}

func (c *evalContext) pathRequiresRewrite(path string) bool {
	if c.rewriteUrls == RewriteUrlsLocal {
		return isPathLocalRelative(path)
	}
	return isPathRelative(path)
}

func (c *evalContext) rewritePath(path, rootpath string) string {
	newPath := normalizePath(rootpath + path)
	if isPathLocalRelative(path) && isPathRelative(rootpath) && !isPathLocalRelative(newPath) {
		newPath = "./" + newPath
	}
	return newPath
}

func normalizePath(path string) string {
	segments := strings.Split(path, "/")
	var out []string
	for _, seg := range segments {
		switch seg {
		case ".":
		case "..":
			if len(out) == 0 || out[len(out)-1] == ".." {
				out = append(out, seg)
			} else {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, seg)
		}
	}
	return strings.Join(out, "/")
}

var reAbsolutePath = jsre.MustCompile(`^(?:[a-z-]+:|\/|#)`, "i")

func isPathRelative(path string) bool { return !reAbsolutePath.Test(path) }

func isPathLocalRelative(path string) bool { return strings.HasPrefix(path, ".") }
