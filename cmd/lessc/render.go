package main

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"syscall"

	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/nodepath"
	"github.com/less/less.js/less"
)

func resolvePath(cwd, p string) string { return nodepath.Resolve(cwd, p) }

// uvDescriptions are libuv's messages for the errors file operations can report.
var uvDescriptions = map[syscall.Errno][2]string{
	syscall.ENOENT:       {"ENOENT", "no such file or directory"},
	syscall.EISDIR:       {"EISDIR", "illegal operation on a directory"},
	syscall.ENOTDIR:      {"ENOTDIR", "not a directory"},
	syscall.EACCES:       {"EACCES", "permission denied"},
	syscall.EPERM:        {"EPERM", "operation not permitted"},
	syscall.EEXIST:       {"EEXIST", "file already exists"},
	syscall.ENAMETOOLONG: {"ENAMETOOLONG", "name too long"},
	syscall.ELOOP:        {"ELOOP", "too many symbolic links encountered"},
	syscall.EROFS:        {"EROFS", "read-only file system"},
	syscall.ENOSPC:       {"ENOSPC", "no space left on device"},
	syscall.EMFILE:       {"EMFILE", "too many open files"},
	syscall.EINVAL:       {"EINVAL", "invalid argument"},
	syscall.EBUSY:        {"EBUSY", "resource busy or locked"},
	syscall.ETXTBSY:      {"ETXTBSY", "text file is busy"},
	syscall.EFBIG:        {"EFBIG", "file too large"},
	syscall.EIO:          {"EIO", "i/o error"},
	syscall.ENXIO:        {"ENXIO", "no such device or address"},
	syscall.ENODEV:       {"ENODEV", "no such device"},
}

// nodeError describes a failed file operation the way Node's fs errors do: the code, and the message
// "CODE: description, syscall 'path'".
func nodeError(err error) (code, message string) {
	var pe *fs.PathError
	if !errors.As(err, &pe) {
		return "EIO", err.Error()
	}
	var errno syscall.Errno
	if !errors.As(pe.Err, &errno) {
		return "EIO", err.Error()
	}
	d, ok := uvDescriptions[errno]
	if !ok {
		d = [2]string{"UNKNOWN", errno.Error()}
	}
	if pe.Op == "read" || pe.Op == "write" {
		return d[0], d[0] + ": " + d[1] + ", " + pe.Op
	}
	return d[0], d[0] + ": " + d[1] + ", open '" + pe.Path + "'"
}

// ensureDirectory creates the directory of a file to be written when it does not exist.
func ensureDirectory(file string) error {
	dir := nodepath.Dirname(file)
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	return os.MkdirAll(dir, 0o777)
}

// writeFile writes an output file, reporting a failure the way lessc does.
func (c *lessc) writeFile(name, contents string) bool {
	if err := ensureDirectory(name); err != nil {
		c.crash("Error: " + err.Error())
	}
	if err := os.WriteFile(name, []byte(js.ToUTF8(contents)), 0o666); err != nil {
		code, msg := nodeError(err)
		c.fail("lessc: failed to create file "+name, "Error: "+code+" "+msg)
		return false
	}
	return true
}

// configureSourceMap derives the map file names from the output file, as render does before compiling. It
// reports false when the options cannot work together.
func (c *lessc) configureSourceMap(output string) bool {
	sm := &c.sourceMapOptions
	if sm.SourceMapURL != "" && sm.DisableSourcemapAnnotation {
		// the original sets process.exitcode here, so the status stays 0
		c.println(c.stderr, "You cannot provide flag --source-map-url with --source-map-no-annotation.")
		c.println(c.stderr, "Please remove one of those flags.")
		return false
	}
	switch {
	case sm.SourceMapFullFilename != "" && !c.sourceMapFileInline:
		mapDir := nodepath.Dirname(nodepath.Resolve(c.cwd, sm.SourceMapFullFilename))
		if output != "" {
			outputDir := nodepath.Dirname(output)
			sm.SourceMapOutputFilename = nodepath.Join(nodepath.Relative(c.cwd, mapDir, outputDir),
				nodepath.Basename(output))
			sm.SourceMapFilename = nodepath.Join(nodepath.Relative(c.cwd, outputDir, mapDir),
				nodepath.Basename(sm.SourceMapFullFilename))
		} else {
			sm.SourceMapOutputFilename = "output.css"
			sm.SourceMapFilename = nodepath.Basename(sm.SourceMapFullFilename)
		}
	case sm.SourceMapFullFilename == "" && output != "" && !c.sourceMapFileInline:
		sm.SourceMapOutputFilename = nodepath.Basename(output)
		sm.SourceMapFullFilename = output + ".map"
	case output == "" && !c.sourceMapFileInline:
		c.fail("the sourcemap option only has an optional filename if the css filename is given",
			"consider adding --source-map-map-inline which embeds the sourcemap into the css")
		return false
	}
	return true
}

// readInput reads the source file, or standard input for "-", as UTF-8 text.
func (c *lessc) readInput(input string) (string, bool) {
	var data []byte
	var err error
	if input == "-" {
		data, err = io.ReadAll(c.stdin)
	} else {
		data, err = os.ReadFile(input)
	}
	if err != nil {
		_, msg := nodeError(err)
		c.fail("lessc: " + msg)
		return "", false
	}
	return strings.TrimPrefix(js.DecodeUTF8(data), "\uFEFF"), true
}

// render is bin/lessc render: it checks the file arguments, compiles, and writes the CSS, the map and the
// dependency list.
func (c *lessc) render() {
	if !c.continueProcessing {
		return
	}
	o := c.options
	input, output, outputbase := "", "", ""
	if len(c.args) > 1 {
		input = c.args[1]
		if input != "-" {
			input = nodepath.Resolve(c.cwd, input)
		}
	}
	if len(c.args) > 2 {
		outputbase = c.args[2]
		output = nodepath.Resolve(c.cwd, outputbase)
	}
	if o.DisablePluginRule && len(c.queuePlugins) > 0 {
		c.fail("--plugin and --disable-plugin-rule may not be used at the same time")
		return
	}
	if c.sourceMap && !c.configureSourceMap(output) {
		return
	}
	if input == "" {
		c.fail("lessc: no input files", "")
		c.write(c.stdout, usage())
		return
	}
	if o.Depends {
		if outputbase == "" {
			c.fail("option --depends requires an output path to be specified")
			return
		}
		c.write(c.stdout, outputbase+": ")
	}
	data, ok := c.readInput(input)
	if !ok {
		return
	}
	o.Paths = append([]string{nodepath.Dirname(input)}, o.Paths...)
	o.Filename = input
	if c.sourceMap && !o.Lint {
		c.sourceMapOptions.SourceMapFileInline = c.sourceMapFileInline
		o.SourceMap = &c.sourceMapOptions
	}
	less.SetLogger(less.Logger{
		Info: func(msg string) {
			if c.verbose {
				c.println(c.stdout, msg)
			}
		},
		Warn: func(msg string) {
			if !c.silent && !c.quiet {
				c.println(c.stderr, msg)
			}
		},
		Error: func(msg string) {
			if !c.silent {
				c.println(c.stderr, msg)
			}
		},
	})
	result, err := less.Render(data, o)
	if err != nil {
		if !o.Silent {
			c.println(c.stderr, err.Format(o.Color))
		}
		c.exitCode = 1
		return
	}
	if !o.Lint {
		c.writeResult(output, result)
	}
}

// writeResult writes the CSS to the output file or standard output, then the map, then the dependency list.
func (c *lessc) writeResult(output string, result *less.Result) {
	o := c.options
	switch {
	case o.Depends:
	case output != "":
		if !c.writeFile(output, result.CSS) {
			return
		}
		less.LogInfo("lessc: wrote " + output)
	default:
		c.write(c.stdout, result.CSS)
	}
	if o.SourceMap != nil && !c.sourceMapFileInline {
		if name := c.sourceMapOptions.SourceMapFullFilename; c.writeFile(name, result.Map) {
			less.LogInfo("lessc: wrote " + name)
		}
	}
	if o.Depends {
		var b strings.Builder
		for _, f := range result.Imports {
			b.WriteString(f + " ")
		}
		c.println(c.stdout, b.String())
	}
}
