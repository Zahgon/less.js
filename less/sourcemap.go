package less

import (
	"encoding/base64"
	"sort"
	"strings"

	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/sourcemap"
)

var reInputExtension = re(`\.[^/.]+$`)

// lastPathSegment is filename.split(/[/\\]/).pop().
func lastPathSegment(filename string) string {
	if i := strings.LastIndexAny(filename, "/\\"); i >= 0 {
		return filename[i+1:]
	}
	return filename
}

// normalizeSourceMapOptions is the source map part of parse-tree.js toCSS.
func normalizeSourceMapOptions(sm *SourceMapOptions, filename string) {
	if sm.SourceMapInputFilename == "" && filename != "" {
		sm.SourceMapInputFilename = filename
	}
	if sm.SourceMapBasepath == "" && filename != "" {
		if i := strings.LastIndexAny(filename, "/\\"); i >= 0 {
			sm.SourceMapBasepath = filename[:i]
		} else {
			sm.SourceMapBasepath = "."
		}
	}
	inputBasename := func() string { return reInputExtension.Replace(lastPathSegment(filename), "") }
	if sm.SourceMapFullFilename != "" && !sm.SourceMapFileInline {
		if sm.SourceMapFilename == "" && sm.SourceMapURL == "" {
			sm.SourceMapFilename = lastPathSegment(sm.SourceMapFullFilename)
		}
	} else if sm.SourceMapFilename == "" && sm.SourceMapURL == "" {
		if sm.SourceMapOutputFilename != "" {
			sm.SourceMapFilename = sm.SourceMapOutputFilename + ".map"
		} else if filename != "" {
			sm.SourceMapFilename = inputBasename() + ".css.map"
		}
	}
	if sm.SourceMapOutputFilename == "" {
		if filename != "" {
			sm.SourceMapOutputFilename = inputBasename() + ".css"
		} else {
			sm.SourceMapOutputFilename = "output.css"
		}
	}
}

// sourceMapOutput is source-map-output.js: it collects the CSS chunks and records a mapping for each chunk that
// carries a file and an index.
type sourceMapOutput struct {
	css               []string
	imports           *importManager
	sourceMapFilename string
	outputFilename    string
	sourceMapURL      string
	basepath          string
	rootpath          string
	outputSourceFiles bool
	generator         *sourcemap.Generator
	lineNumber        int
	column            int
	sources           map[string]*sourceIndex
}

// sourceIndex caches a file's text (less its ignored banner) as UTF-16 with its newline offsets, so that the line
// and column of inputSource.substring(0, index).split('\n') are found without re-splitting the text.
type sourceIndex struct {
	length   int
	newlines []int
}

func backslashesToSlashes(s string) string { return strings.ReplaceAll(s, "\\", "/") }

func newSourceMapOutput(sm *SourceMapOptions, imports *importManager) *sourceMapOutput {
	o := &sourceMapOutput{imports: imports, sourceMapURL: sm.SourceMapURL, outputSourceFiles: sm.OutputSourceFiles,
		sources: map[string]*sourceIndex{}}
	if sm.SourceMapFilename != "" {
		o.sourceMapFilename = backslashesToSlashes(sm.SourceMapFilename)
	}
	o.outputFilename = backslashesToSlashes(sm.SourceMapOutputFilename)
	if sm.SourceMapBasepath != "" {
		o.basepath = backslashesToSlashes(sm.SourceMapBasepath)
	}
	if sm.SourceMapRootpath != "" {
		o.rootpath = backslashesToSlashes(sm.SourceMapRootpath)
		if !strings.HasSuffix(o.rootpath, "/") {
			o.rootpath += "/"
		}
	}
	return o
}

func (o *sourceMapOutput) removeBasepath(path string) string {
	if o.basepath != "" && strings.HasPrefix(path, o.basepath) {
		path = path[len(o.basepath):]
		if strings.HasPrefix(path, "\\") || strings.HasPrefix(path, "/") {
			path = path[1:]
		}
	}
	return path
}

func (o *sourceMapOutput) normalizeFilename(filename string) string {
	return o.rootpath + o.removeBasepath(backslashesToSlashes(filename))
}

// position returns the line (1-based) and the UTF-16 column of index in the named file.
func (o *sourceMapOutput) position(filename string, text string, index int) (int, int) {
	si, ok := o.sources[filename]
	if !ok {
		u := js.ToU16(text)
		si = &sourceIndex{length: len(u)}
		for i, c := range u {
			if c == '\n' {
				si.newlines = append(si.newlines, i)
			}
		}
		o.sources[filename] = si
	}
	if index > si.length {
		index = si.length
	}
	n := sort.SearchInts(si.newlines, index)
	if n == 0 {
		return 1, index
	}
	return n + 1, index - si.newlines[n-1] - 1
}

func (o *sourceMapOutput) add(chunk string, fi *FileInfo, index int, mapLines bool) {
	if chunk == "" {
		return
	}
	mapped := fi != nil && fi.Filename != ""
	var srcLine, srcColumn int
	if mapped {
		input, ok := o.imports.contents[fi.Filename]
		if ignored := o.imports.contentsIgnoredChars[fi.Filename]; ignored != 0 {
			index -= ignored
			if index < 0 {
				index = 0
			}
			input = js.SliceJS(input, ignored, js.Len16(input))
		}
		if !ok {
			o.css = append(o.css, chunk)
			return
		}
		srcLine, srcColumn = o.position(fi.Filename, input, index)
	}
	lines := strings.Split(chunk, "\n")
	lastColumn := js.Len16(lines[len(lines)-1])
	if mapped {
		source := o.normalizeFilename(fi.Filename)
		if !mapLines {
			o.addMapping(o.lineNumber+1, o.column, srcLine, srcColumn, source)
		} else {
			for i := range lines {
				if i == 0 {
					o.addMapping(o.lineNumber+1, o.column, srcLine, srcColumn, source)
				} else {
					o.addMapping(o.lineNumber+i+1, 0, srcLine+i, 0, source)
				}
			}
		}
	}
	if len(lines) == 1 {
		o.column += lastColumn
	} else {
		o.lineNumber += len(lines) - 1
		o.column = lastColumn
	}
	o.css = append(o.css, chunk)
}

func (o *sourceMapOutput) addMapping(genLine, genColumn, line, column int, source string) {
	err := o.generator.AddMapping(sourcemap.Mapping{Generated: sourcemap.Position{Line: genLine, Column: genColumn},
		Original: &sourcemap.Position{Line: line, Column: column}, Source: &source})
	if err != nil {
		panic(&errorInfo{Message: err.Error()})
	}
}

// toCSS generates the CSS through the mapping output and returns it with the map text, which is absent when
// nothing was generated.
func (o *sourceMapOutput) toCSS(root Node, ctx *evalContext) (string, *string) {
	file := o.outputFilename
	o.generator = sourcemap.NewGenerator(&file)
	if o.outputSourceFiles {
		for _, filename := range o.imports.contentsOrder {
			source := o.imports.contents[filename]
			if ignored := o.imports.contentsIgnoredChars[filename]; ignored != 0 {
				source = js.SliceJS(source, ignored, js.Len16(source))
			}
			o.generator.SetSourceContent(o.normalizeFilename(filename), &source)
		}
	}
	out := &output{hook: o.add, empty: func() bool { return len(o.css) == 0 }}
	genCSS(root, ctx, out)
	var sourceMap *string
	if len(o.css) > 0 {
		content := o.generator.JSON()
		if o.sourceMapURL == "" {
			o.sourceMapURL = o.sourceMapFilename
		}
		sourceMap = &content
	}
	return strings.Join(o.css, ""), sourceMap
}

// renderWithSourceMap is the source map branch of parse-tree.js toCSS with source-map-builder.js: it returns the
// CSS with its sourceMappingURL appendage and the external map text.
func renderWithSourceMap(root *Ruleset, ctx *evalContext, opts *Options, imports *importManager) (string, string) {
	sm := opts.SourceMap
	normalizeSourceMapOptions(sm, opts.Filename)
	o := newSourceMapOutput(sm, imports)
	css, sourceMap := o.toCSS(root, ctx)
	url := o.sourceMapURL
	if url != "" {
		url = o.removeBasepath(url)
	}
	mapText := ""
	if sourceMap != nil {
		mapText = *sourceMap
	}
	if sm.SourceMapFileInline {
		if sourceMap == nil {
			return css, mapText
		}
		url = "data:application/json;base64," + base64.StdEncoding.EncodeToString([]byte(mapText))
	}
	if sm.DisableSourcemapAnnotation || url == "" {
		return css, mapText
	}
	return css + "/*# sourceMappingURL=" + url + " */", mapText
}
