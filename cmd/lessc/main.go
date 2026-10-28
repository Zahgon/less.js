// Command lessc compiles a Less stylesheet to CSS. It is the Go port of packages/less/bin/lessc: the same
// options, messages, output files and exit status.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
	"github.com/less/less.js/less"
)

var (
	reIncludeArg = jsre.MustCompile(`^-I(.+)$`, "")
	reOptionArg  = jsre.MustCompile(`^--?([a-z][0-9a-z-]*)(?:=(.*))?$`, "i")
	reBooleanArg = jsre.MustCompile(`^((on|t|true|y|yes)|(off|f|false|n|no))$`, "i")
	rePluginArg  = jsre.MustCompile(`^([^=]+)(=(.*))?`, "")
)

// match returns the capture groups of the first match of re in s (nil for an unmatched group), or nil.
func match(re *jsre.Regexp, s string) []*string {
	in := js.ToU16(s)
	m := re.Search(in, 0)
	if m == nil {
		return nil
	}
	groups := make([]*string, len(m)/2)
	for g := range groups {
		if m[2*g] >= 0 {
			v := js.FromU16(in[m[2*g]:m[2*g+1]])
			groups[g] = &v
		}
	}
	return groups
}

type pluginRequest struct {
	name string
}

// lessc holds the state bin/lessc keeps in module variables.
type lessc struct {
	stdin               io.Reader
	stdout, stderr      io.Writer
	cwd                 string
	exitCode            int
	continueProcessing  bool
	silent, quiet       bool
	verbose             bool
	options             *less.Options
	sourceMap           bool
	sourceMapOptions    less.SourceMapOptions
	sourceMapFileInline bool
	queuePlugins        []pluginRequest
	pendingDeprecations []string
	args                []string
}

func main() {
	cwd, err := syscall.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lessc: "+err.Error())
		os.Exit(1)
	}
	c := &lessc{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr, cwd: cwd}
	os.Exit(c.run(os.Args[1:]))
}

func (c *lessc) println(w io.Writer, s string) { io.WriteString(w, s+"\n") }

func (c *lessc) fail(lines ...string) {
	for _, l := range lines {
		c.println(c.stderr, l)
	}
	c.exitCode = 1
}

// checkArg is checkArgFunc: an option that needs a value fails without one.
func (c *lessc) checkArg(name string, value *string) bool {
	if value == nil || *value == "" {
		c.fail(name + " option requires a parameter")
		c.continueProcessing = false
		return false
	}
	return true
}

// checkBoolean is checkBooleanArg.
func (c *lessc) checkBoolean(value string) bool {
	m := match(reBooleanArg, value)
	if m == nil {
		c.fail(" unable to parse " + value + " as a boolean. use one of on/t/true/y/yes/off/f/false/n/no")
		c.continueProcessing = false
		return false
	}
	return m[2] != nil
}

// setVariable is parseVariableOption on a JavaScript object: option.split('=', 2), where a repeated name keeps
// its first position and __proto__ cannot be assigned a string.
func setVariable(vars []less.VarOption, option string) []less.VarOption {
	parts := js.Split16(option, "=")
	name := parts[0]
	var value *string
	if len(parts) > 1 {
		v := parts[1]
		value = &v
	}
	if name == "__proto__" {
		return vars
	}
	for i := range vars {
		if vars[i].Name == name {
			vars[i].Value = value
			return vars
		}
	}
	return append(vars, less.VarOption{Name: name, Value: value})
}

func (c *lessc) printUsage() {
	io.WriteString(c.stdout, usage())
	c.continueProcessing = false
}

func (c *lessc) run(argv []string) int {
	c.options = less.DefaultOptions()
	c.continueProcessing = true
	c.args = []string{"lessc"}
	for _, raw := range argv {
		arg := js.DecodeUTF8([]byte(raw))
		if m := match(reIncludeArg, arg); m != nil {
			c.options.Paths = append(c.options.Paths, *m[1])
			continue
		}
		m := match(reOptionArg, arg)
		if m == nil {
			if arg != "" {
				c.args = append(c.args, arg)
			}
			continue
		}
		c.option(*m[1], m[2])
	}
	if !c.silent && !c.quiet && !c.options.QuietDeprecations {
		for _, msg := range c.pendingDeprecations {
			c.println(c.stderr, msg)
		}
	}
	if len(c.queuePlugins) > 0 {
		c.processPluginQueue()
	} else {
		c.render()
	}
	return c.exitCode
}

func (c *lessc) deprecate(msg string) { c.pendingDeprecations = append(c.pendingDeprecations, msg) }

// processPluginQueue loads the queued plugins; the Go command cannot run JavaScript plugins, so a plugin that
// is found stops the run with an explanation, and one that is not found gets the original's message.
func (c *lessc) processPluginQueue() {
	var found []string
	missing := false
	for _, p := range c.queuePlugins {
		file, ok := less.FindPlugin(p.name, c.cwd, c.options.Paths)
		if !ok {
			c.fail("Unable to load plugin " + p.name +
				" please make sure that it is installed under or at the same level as less")
			missing = true
			continue
		}
		found = append(found, file)
	}
	if !missing {
		c.fail("lessc: JavaScript plugins cannot run in the Go implementation: " + strings.Join(found, ", "))
	}
}

func valueOr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// option applies one recognised command-line option, the switch of bin/lessc.
func (c *lessc) option(name string, value *string) {
	o, sm := c.options, &c.sourceMapOptions
	switch name {
	case "v", "version":
		c.println(c.stdout, "lessc 4.9.1 (Less Compiler) [Go]")
		c.continueProcessing = false
	case "verbose":
		c.verbose = true
	case "s", "silent":
		c.silent, o.Silent = true, true
	case "quiet":
		c.quiet, o.Quiet = true, true
	case "quiet-deprecations":
		o.QuietDeprecations = true
	case "l", "lint":
		o.Lint = true
	case "strict-imports":
		o.StrictImports = true
	case "h", "help":
		c.printUsage()
	case "x", "compress":
		o.Compress = true
	case "insecure":
		o.Insecure = true
	case "M", "depends":
		o.Depends = true
	case "max-line-len":
		// parsed and validated like the original, which no longer uses the value
		c.checkArg(name, value)
	case "no-color":
		o.Color = false
	case "js":
		o.JavascriptEnabled = true
		c.deprecate("Warning: Inline JavaScript (--js) is deprecated and will be removed in Less 5.x. Use Less functions or custom plugins instead. (js-eval)")
	case "no-js":
		c.println(c.stderr, `The "--no-js" argument is deprecated, as inline JavaScript is disabled by default. Use "--js" to enable inline JavaScript (not recommended).`)
	case "include-path":
		if c.checkArg(name, value) {
			parts := js.Split16(*value, ":")
			o.Paths = make([]string, len(parts))
			for i, p := range parts {
				if p != "" {
					o.Paths[i] = resolvePath(c.cwd, p)
				}
			}
		}
	case "line-numbers":
		if c.checkArg(name, value) {
			o.DumpLineNumbers = *value
			c.deprecate("Warning: The --line-numbers option is deprecated and will be removed in Less 5.x. Use source maps instead (--source-map). (dump-line-numbers)")
		}
	case "source-map":
		c.sourceMap = true
		if value != nil && *value != "" {
			sm.SourceMapFullFilename = *value
		}
	case "source-map-rootpath":
		if c.checkArg(name, value) {
			sm.SourceMapRootpath = *value
		}
	case "source-map-basepath":
		if c.checkArg(name, value) {
			sm.SourceMapBasepath = *value
		}
	case "source-map-inline", "source-map-map-inline":
		c.sourceMapFileInline, c.sourceMap = true, true
	case "source-map-include-source", "source-map-less-inline":
		sm.OutputSourceFiles = true
	case "source-map-url":
		if c.checkArg(name, value) {
			sm.SourceMapURL = *value
		}
	case "source-map-no-annotation":
		sm.DisableSourcemapAnnotation = true
	case "rp", "rootpath":
		if c.checkArg(name, value) {
			o.Rootpath = strings.ReplaceAll(*value, "\\", "/")
		}
	case "ie-compat":
		c.deprecate("Warning: The --ie-compat option is deprecated, as it has no effect on compilation.")
	case "relative-urls":
		c.deprecate("Warning: The --relative-urls option has been deprecated. Use --rewrite-urls=all.")
		o.RewriteUrls = less.RewriteUrlsAll
	case "ru", "rewrite-urls":
		c.rewriteUrls(valueOr(value))
	case "sm", "strict-math":
		c.deprecate("Warning: The --strict-math option has been deprecated. Use --math=strict.")
		if c.checkArg(name, value) && c.checkBoolean(*value) {
			o.Math = less.MathParens
		}
	case "m", "math":
		if c.checkArg(name, value) {
			c.math(*value)
		}
	case "su", "strict-units":
		if c.checkArg(name, value) {
			o.StrictUnits = c.checkBoolean(*value)
		}
	case "global-var":
		if c.checkArg(name, value) {
			o.GlobalVars = setVariable(o.GlobalVars, *value)
		}
	case "modify-var":
		if c.checkArg(name, value) {
			o.ModifyVars = setVariable(o.ModifyVars, *value)
		}
	case "url-args":
		if c.checkArg(name, value) {
			o.URLArgs = *value
		}
	case "plugin":
		c.pluginOption(value)
	case "disable-plugin-rule":
		o.DisablePluginRule = true
	default:
		c.queuePlugins = append(c.queuePlugins, pluginRequest{name: name})
	}
}

func (c *lessc) rewriteUrls(m string) {
	switch m {
	case "":
		c.options.RewriteUrls = less.RewriteUrlsAll
	case "local":
		c.options.RewriteUrls = less.RewriteUrlsLocal
	case "off":
		c.options.RewriteUrls = less.RewriteUrlsOff
	case "all":
		c.options.RewriteUrls = less.RewriteUrlsAll
	default:
		c.fail("Unknown rewrite-urls argument " + m)
		c.continueProcessing = false
	}
}

func (c *lessc) math(m string) {
	switch m {
	case "always":
		c.deprecate("Warning: --math=always is deprecated and will be removed in Less 5.x. Use --math=parens-division (default) or --math=parens. (math-always)")
		c.options.Math = less.MathAlways
	case "parens-division":
		c.options.Math = less.MathParensDivision
	case "parens", "strict":
		c.options.Math = less.MathParens
	case "strict-legacy":
		c.deprecate("Warning: --math=strict-legacy has been removed. Defaulting to --math=strict.")
		c.options.Math = less.MathParens
	}
}

// pluginOption is --plugin=NAME[=OPTIONS]. Without a usable value the original fails with a TypeError while
// reading its arguments, before anything else runs.
func (c *lessc) pluginOption(value *string) {
	if value == nil {
		c.crash("TypeError: Cannot read properties of undefined (reading 'match')")
		return
	}
	m := match(rePluginArg, *value)
	if m == nil {
		c.crash("TypeError: Cannot read properties of null (reading '1')")
		return
	}
	c.queuePlugins = append(c.queuePlugins, pluginRequest{name: *m[1]})
}

// crash ends the run the way an uncaught exception ends the original: the error on standard error, status 1 and
// nothing else processed.
func (c *lessc) crash(msg string) {
	c.println(c.stderr, msg)
	os.Exit(1)
}
