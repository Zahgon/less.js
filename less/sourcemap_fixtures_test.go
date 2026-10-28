package less

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func init() {
	// the functions the original's test runner registers for tests-unit/functions
	registerFunctions(map[string]func(*funcCaller, []Node) Node{
		"add": func(fc *funcCaller, a []Node) Node {
			return newDimension(valueOf(argAt(a, 0))+valueOf(argAt(a, 1)), nil)
		},
		"increment": func(fc *funcCaller, a []Node) Node { return newDimension(valueOf(argAt(a, 0))+1, nil) },
		"_color": func(fc *funcCaller, a []Node) Node {
			if s, ok := stringValue(argAt(a, 0)); ok && s == "evil red" {
				return newColorHex("600", "")
			}
			return nil
		},
	})
}

// sourceMapsDir holds the expected maps the original's suite keeps beside its tests.
var sourceMapsDir = filepath.Join("..", "packages", "less", "test")

// normalizedMap parses a source map and re-serialises it, as the original's runner compares them.
func normalizedMap(t *testing.T, text string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("invalid source map %q: %v", text, err)
	}
	return v
}

// TestSourcemapFixtures compiles the source map fixtures and compares their maps with test/sourcemaps.
func TestSourcemapFixtures(t *testing.T) {
	files := globFixtures(t, []string{"tests-config/sourcemaps/*.less", "tests-config/sourcemaps/*/*.less",
		"tests-config/sourcemaps-url/*.less", "tests-config/sourcemaps-rootpath/*.less",
		"tests-config/sourcemaps-basepath/*.less", "tests-config/sourcemaps-include-source/*.less"}, nil)
	for _, file := range files {
		name := fixtureName(file)
		t.Run(name, func(t *testing.T) {
			res, rerr, ok := renderFixture(t, file, nil)
			if !ok {
				return
			}
			if rerr != nil {
				t.Fatalf("render failed: %s", rerr.Error())
			}
			const prefix = "/*# sourceMappingURL="
			i := strings.Index(res.CSS, prefix)
			if i < 0 || strings.TrimSpace(strings.TrimSuffix(res.CSS[i+len(prefix):], " */")) == "" {
				t.Fatalf("no sourceMappingURL in the output")
			}
			want, err := os.ReadFile(filepath.Join(sourceMapsDir, "sourcemaps", filepath.Base(name)+".json"))
			if err != nil {
				t.Skip("no expected map")
			}
			dir := filepath.Dir(file) + "/"
			expected := strings.ReplaceAll(string(want), "{path}", dir)
			expected = globalReplacements(expected, testDataDir+"/", file)
			if got, exp := normalizedMap(t, res.Map), normalizedMap(t, expected); !reflect.DeepEqual(got, exp) {
				t.Errorf("map differs\nwant: %s\ngot:  %s", expected, res.Map)
			}
		})
	}
}

// expectationPath is the original's sourcemapExpectationPath: the fixture path without tests-config/.
func expectationPath(name string) string {
	return filepath.Join(sourceMapsDir, strings.TrimPrefix(filepath.ToSlash(name), "tests-config/")) + ".json"
}

// TestSourcemapVariants covers the empty map, the map without annotation and a variable in a selector.
func TestSourcemapVariants(t *testing.T) {
	annotation := regexp.MustCompile(`/\*# sourceMappingURL=.+\.css\.map \*/$`)
	for _, file := range globFixtures(t, []string{"tests-config/sourcemaps-empty/*.less"}, nil) {
		t.Run(fixtureName(file), func(t *testing.T) {
			res, rerr, ok := renderFixture(t, file, nil)
			if !ok {
				return
			}
			if rerr != nil {
				t.Fatalf("render failed: %s", rerr.Error())
			}
			if res.CSS != "" || res.Map != "" {
				t.Errorf("want empty CSS and no map, got %q and %q", res.CSS, res.Map)
			}
		})
	}
	for _, set := range []string{"sourcemaps-disable-annotation", "sourcemaps-variable-selector"} {
		for _, file := range globFixtures(t, []string{"tests-config/" + set + "/*.less"}, nil) {
			name := fixtureName(file)
			t.Run(name, func(t *testing.T) {
				res, rerr, ok := renderFixture(t, file, nil)
				if !ok {
					return
				}
				if rerr != nil {
					t.Fatalf("render failed: %s", rerr.Error())
				}
				if set == "sourcemaps-disable-annotation" && annotation.MatchString(res.CSS) {
					t.Errorf("sourceMappingURL found in %q", res.CSS)
				}
				// a fixture without an expectation file (vars.less only defines variables) expects no map
				want, err := os.ReadFile(expectationPath(name))
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if res.Map != string(want) {
					t.Errorf("map differs\nwant: %s\ngot:  %s", want, res.Map)
				}
			})
		}
	}
}
