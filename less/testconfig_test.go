package less

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testDataDir is packages/test-data, the fixtures shared with the original project.
var testDataDir = func() string {
	dir, err := filepath.Abs(filepath.Join("..", "packages", "test-data"))
	if err != nil {
		panic(err)
	}
	return dir
}()

// lessConfig is the language.less object of a styles.config.json, with the variable objects kept in key order.
type lessConfig struct {
	values     map[string]json.RawMessage
	globalVars []VarOption
	modifyVars []VarOption
	hasGlobal  bool
	hasModify  bool
	// mapObject is a sourceMap given as an object (the runner adds its defaults) rather than as true
	mapObject bool
}

// findConfig returns the nearest styles.config.json at or above dir inside the test data, as the original's test
// runner finds styles.config.cjs with cosmiconfig.
func findConfig(t *testing.T, dir string) *lessConfig {
	t.Helper()
	for d := dir; strings.HasPrefix(d, testDataDir); d = filepath.Dir(d) {
		data, err := os.ReadFile(filepath.Join(d, "styles.config.json"))
		if err != nil {
			continue
		}
		var file struct {
			Language struct {
				Less json.RawMessage `json:"less"`
			} `json:"language"`
		}
		if err := json.Unmarshal(data, &file); err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		return parseLessConfig(t, file.Language.Less)
	}
	return &lessConfig{values: map[string]json.RawMessage{}}
}

func parseLessConfig(t *testing.T, raw json.RawMessage) *lessConfig {
	t.Helper()
	c := &lessConfig{values: map[string]json.RawMessage{}}
	if len(raw) == 0 {
		return c
	}
	if err := json.Unmarshal(raw, &c.values); err != nil {
		t.Fatal(err)
	}
	if v, ok := c.values["globalVars"]; ok {
		c.globalVars, c.hasGlobal = orderedVars(t, v), true
	}
	if v, ok := c.values["modifyVars"]; ok {
		c.modifyVars, c.hasModify = orderedVars(t, v), true
	}
	return c
}

// orderedVars decodes a JSON object of variables keeping its key order.
func orderedVars(t *testing.T, raw json.RawMessage) []VarOption {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	vars := []VarOption{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		var value string
		if err := dec.Decode(&value); err != nil {
			t.Fatal(err)
		}
		vars = append(vars, VarOption{Name: key.(string), Value: &value})
	}
	return vars
}

func (c *lessConfig) bool(key string) bool {
	var b bool
	if v, ok := c.values[key]; ok {
		_ = json.Unmarshal(v, &b)
	}
	return b
}

func (c *lessConfig) str(key string) (string, bool) {
	var s string
	v, ok := c.values[key]
	if !ok || json.Unmarshal(v, &s) != nil {
		return "", false
	}
	return s, true
}

// options converts the configuration the way the original's copyOptions coerces render options: strictMath and
// relativeUrls select parens math and URL rewriting, and math and rewriteUrls may be names.
func (c *lessConfig) options(filename string) *Options {
	o := DefaultOptions()
	o.Filename = filename
	o.Compress = c.bool("compress")
	o.StrictUnits = c.bool("strictUnits")
	o.StrictImports = c.bool("strictImports")
	o.JavascriptEnabled = c.bool("javascriptEnabled")
	o.Rootpath, _ = c.str("rootpath")
	o.URLArgs, _ = c.str("urlArgs")
	o.DumpLineNumbers, _ = c.str("dumpLineNumbers")
	o.Banner, _ = c.str("banner")
	if v, ok := c.values["processImports"]; ok && string(v) == "false" {
		o.NoProcessImports = true
	}
	if c.hasGlobal {
		o.GlobalVars = c.globalVars
	}
	if c.hasModify {
		o.ModifyVars = c.modifyVars
	}
	if c.bool("strictMath") {
		o.Math = MathParens
	}
	if c.bool("relativeUrls") {
		o.RewriteUrls = RewriteUrlsAll
	}
	if m, ok := c.str("math"); ok {
		switch strings.ToLower(m) {
		case "always":
			o.Math = MathAlways
		case "parens-division":
			o.Math = MathParensDivision
		case "strict", "parens", "strict-legacy":
			o.Math = MathParens
		}
	}
	if r, ok := c.str("rewriteUrls"); ok {
		switch strings.ToLower(r) {
		case "off":
			o.RewriteUrls = RewriteUrlsOff
		case "local":
			o.RewriteUrls = RewriteUrlsLocal
		case "all":
			o.RewriteUrls = RewriteUrlsAll
		}
	}
	dir := filepath.Dir(filename)
	var paths []string
	if v, ok := c.values["paths"]; ok {
		_ = json.Unmarshal(v, &paths)
	}
	o.Paths = nil
	seen := false
	for _, p := range append(paths, dir) {
		if p == dir {
			if seen {
				continue
			}
			seen = true
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		o.Paths = append(o.Paths, p)
	}
	if v, ok := c.values["sourceMap"]; ok && string(v) != "false" {
		c.mapObject = strings.HasPrefix(strings.TrimSpace(string(v)), "{")
		var sm struct {
			SourceMapFileInline        bool   `json:"sourceMapFileInline"`
			SourceMapRootpath          string `json:"sourceMapRootpath"`
			SourceMapBasepath          string `json:"sourceMapBasepath"`
			SourceMapURL               string `json:"sourceMapURL"`
			OutputSourceFiles          bool   `json:"outputSourceFiles"`
			DisableSourcemapAnnotation bool   `json:"disableSourcemapAnnotation"`
		}
		_ = json.Unmarshal(v, &sm)
		o.SourceMap = &SourceMapOptions{SourceMapFileInline: sm.SourceMapFileInline,
			SourceMapRootpath: sm.SourceMapRootpath, SourceMapBasepath: sm.SourceMapBasepath,
			SourceMapURL: sm.SourceMapURL, OutputSourceFiles: sm.OutputSourceFiles,
			DisableSourcemapAnnotation: sm.DisableSourcemapAnnotation}
	}
	return o
}

// escapeTestPath is the {pathesc} form of a directory: every dot, colon and slash escaped with a backslash.
func escapeTestPath(p string) string {
	var b strings.Builder
	for _, r := range p {
		if strings.ContainsRune(".:/\\", r) {
			b.WriteByte('\\')
			if r == '\\' {
				r = '/'
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

// globalReplacements substitutes the placeholders of the expected output files, as the original's test runner does.
func globalReplacements(input, directory, filename string) string {
	p := directory
	if filename != "" {
		p = filepath.Dir(filename) + "/"
	}
	pathimport := directory + "import/"
	if i := strings.Index(directory, "/debug/"); i >= 0 {
		rest := directory[i:]
		if strings.Contains(rest, "/comments/") || strings.Contains(rest, "/mediaquery/") || strings.Contains(rest, "/all/") {
			p = directory[:i+len("/debug")] + "/"
			pathimport = p + "import/"
		}
	}
	rel, err := filepath.Rel(testDataDir, p)
	if err != nil {
		rel = p
	}
	r := strings.NewReplacer("{path}", p, "{node}", "", "{/node}", "", "{pathhref}", "", "{404status}", "",
		"{pathrel}", rel+"/", "{pathesc}", escapeTestPath(p), "{pathimport}", pathimport,
		"{pathimportesc}", escapeTestPath(pathimport), "\r\n", "\n")
	return r.Replace(input)
}
