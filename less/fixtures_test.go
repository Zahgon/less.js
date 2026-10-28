package less

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Fixture sets of the original's packages/less/test/index.js, run against packages/test-data.

// excludedFixtures are the globs index.js removes from the compile-and-compare set; they have their own tests.
var excludedFixtures = []string{
	"tests-config/sourcemaps/", "tests-config/sourcemaps-empty/", "tests-config/sourcemaps-disable-annotation/",
	"tests-config/sourcemaps-variable-selector/", "tests-config/globalVars/", "tests-config/modifyVars/",
	"tests-config/js-type-errors/", "tests-config/no-js-errors/", "tests-unit/import/import-remote.less",
}

// globFixtures expands the patterns relative to the test data, sorted and without the excluded paths.
func globFixtures(t *testing.T, patterns []string, exclude []string) []string {
	t.Helper()
	var out []string
	for _, p := range patterns {
		matches, err := filepath.Glob(filepath.Join(testDataDir, p))
		if err != nil {
			t.Fatal(err)
		}
	next:
		for _, m := range matches {
			rel, _ := filepath.Rel(testDataDir, m)
			for _, e := range exclude {
				if strings.HasPrefix(rel, e) {
					continue next
				}
			}
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

// skipReason reports why a fixture cannot run here: it needs a JavaScript plugin, inline JavaScript evaluation,
// an npm module, or a network import.
func skipReason(src string, cfg *lessConfig) string {
	switch {
	case strings.Contains(src, "@plugin"):
		return "loads a JavaScript plugin"
	case cfg.values["plugin"] != nil:
		return "runs with a JavaScript plugin"
	case strings.Contains(src, "`"):
		return "evaluates inline JavaScript"
	case strings.Contains(src, "@less/test-import-module"), strings.Contains(src, "bootstrap-less-port"):
		return "imports an npm module"
	case strings.Contains(src, "http://") && strings.Contains(src, "@import") && !strings.Contains(src, "(css)"),
		strings.Contains(src, "https://") && strings.Contains(src, "@import"):
		return "imports over the network"
	}
	return ""
}

// renderFixture compiles one fixture with the options of its nearest styles.config.json.
func renderFixture(t *testing.T, file string, adjust func(*Options)) (*Result, *Error, bool) {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	cfg := findConfig(t, filepath.Dir(file))
	if reason := skipReason(string(src), cfg); reason != "" {
		t.Skip(reason)
		return nil, nil, false
	}
	opts := cfg.options(file)
	if sm := opts.SourceMap; sm != nil && cfg.mapObject && !sm.SourceMapFileInline {
		// the original's runner names the output after the fixture and roots the sources at testweb/
		if sm.SourceMapOutputFilename == "" {
			sm.SourceMapOutputFilename = fixtureName(file) + ".css"
		}
		if sm.SourceMapRootpath == "" {
			sm.SourceMapRootpath = "testweb/"
		}
	}
	if adjust != nil {
		adjust(opts)
	}
	SetLogger(Logger{})
	res, rerr := Render(string(src), opts)
	return res, rerr, true
}

func fixtureName(file string) string {
	rel, _ := filepath.Rel(testDataDir, file)
	return strings.TrimSuffix(rel, ".less")
}

// TestCompileFixtures compiles every fixture with a .css expectation and compares the output.
func TestCompileFixtures(t *testing.T) {
	files := globFixtures(t, []string{"tests-config/*/*.less", "tests-unit/*/*.less",
		"tests-config/debug/*/linenumbers-*.less"}, excludedFixtures)
	for _, file := range files {
		cssPath := strings.TrimSuffix(file, ".less") + ".css"
		want, err := os.ReadFile(cssPath)
		if err != nil {
			continue
		}
		t.Run(fixtureName(file), func(t *testing.T) {
			res, rerr, ok := renderFixture(t, file, nil)
			if !ok {
				return
			}
			if rerr != nil {
				t.Fatalf("render failed: %s", rerr.Error())
			}
			expected := globalReplacements(string(want), filepath.Dir(file)+"/", "")
			if res.CSS != expected {
				t.Errorf("output differs from %s\n%s", cssPath, firstDifference(expected, res.CSS))
			}
		})
	}
}

// TestErrorFixtures compiles the error fixtures and compares the error text with the .txt expectation.
func TestErrorFixtures(t *testing.T) {
	files := globFixtures(t, []string{"tests-error/eval/*.less", "tests-error/parse/*.less",
		"tests-config/no-js-errors/*.less"}, nil)
	for _, file := range files {
		txtPath := strings.TrimSuffix(file, ".less") + ".txt"
		want, err := os.ReadFile(txtPath)
		if err != nil {
			continue
		}
		t.Run(fixtureName(file), func(t *testing.T) {
			_, rerr, ok := renderFixture(t, file, nil)
			if !ok {
				return
			}
			if rerr == nil {
				t.Fatal("no error")
			}
			expected := globalReplacements(string(want), testDataDir+"/", rerr.Filename())
			if got := rerr.Error(); got != expected {
				t.Errorf("error differs from %s\n%s", txtPath, firstDifference(expected, got))
			}
		})
	}
}

// TestVariableFixtures compiles the globalVars and modifyVars fixtures with the variables of their .json files.
func TestVariableFixtures(t *testing.T) {
	for _, set := range []string{"globalVars", "modifyVars"} {
		for _, file := range globFixtures(t, []string{"tests-config/" + set + "/*.less"}, nil) {
			want, err := os.ReadFile(strings.TrimSuffix(file, ".less") + ".css")
			if err != nil {
				continue
			}
			t.Run(fixtureName(file), func(t *testing.T) {
				data, err := os.ReadFile(strings.TrimSuffix(file, ".less") + ".json")
				vars := []VarOption{}
				if err == nil {
					vars = orderedVars(t, data)
				}
				res, rerr, ok := renderFixture(t, file, func(o *Options) {
					if set == "globalVars" {
						o.GlobalVars = vars
					} else {
						o.ModifyVars = vars
					}
				})
				if !ok {
					return
				}
				if rerr != nil {
					t.Fatalf("render failed: %s", rerr.Error())
				}
				if expected := globalReplacements(string(want), filepath.Dir(file)+"/", ""); res.CSS != expected {
					t.Errorf("output differs\n%s", firstDifference(expected, res.CSS))
				}
			})
		}
	}
}

// firstDifference describes where two texts first differ, with some context.
func firstDifference(want, got string) string {
	i := 0
	for i < len(want) && i < len(got) && want[i] == got[i] {
		i++
	}
	lo := max(0, i-80)
	return "at byte " + itoa(i) + "\nwant: " + quoteSnippet(want, lo, i+80) + "\ngot:  " + quoteSnippet(got, lo, i+80)
}

func quoteSnippet(s string, lo, hi int) string {
	if lo > len(s) {
		lo = len(s)
	}
	return strings.ReplaceAll(s[lo:min(hi, len(s))], "\n", "\\n")
}
