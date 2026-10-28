package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runLessc runs the command in dir with the given arguments and standard input.
func runLessc(t *testing.T, dir, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	c := &lessc{stdin: strings.NewReader(stdin), stdout: &out, stderr: &errOut, cwd: dir}
	code = c.run(args)
	return code, out.String(), errOut.String()
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUsageAndNoInput(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := runLessc(t, dir, "")
	if code != 1 || errOut != "lessc: no input files\n\n" || !strings.HasPrefix(out, "usage: lessc [option option=parameter ...]") {
		t.Errorf("no input: %d %q %q", code, out, errOut)
	}
	if code, out, _ := runLessc(t, dir, "", "--help"); code != 0 || out != usage() {
		t.Errorf("--help: %d", code)
	}
}

func TestOptionErrors(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		args []string
		code int
		err  string
	}{
		{[]string{"--include-path"}, 1, "include-path option requires a parameter\n"},
		{[]string{"-su=maybe", "a.less"}, 1, " unable to parse maybe as a boolean. use one of on/t/true/y/yes/off/f/false/n/no\n"},
		{[]string{"--rewrite-urls=some", "a.less"}, 1, "Unknown rewrite-urls argument some\n"},
		{[]string{"--clean-css", "a.less"}, 1, "Unable to load plugin clean-css please make sure that it is installed under or at the same level as less\n"},
		{[]string{"--source-map", "a.less"}, 1, "the sourcemap option only has an optional filename if the css filename is given\nconsider adding --source-map-map-inline which embeds the sourcemap into the css\n"},
	}
	for _, c := range cases {
		code, _, errOut := runLessc(t, dir, "", c.args...)
		if code != c.code || errOut != c.err {
			t.Errorf("%v: got %d %q, want %d %q", c.args, code, errOut, c.code, c.err)
		}
	}
}

func TestCompileToFilesWithMapAndDependencies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.less"), "@import \"b\";\n.a { width: (1px + 2); }\n")
	writeFile(t, filepath.Join(dir, "b.less"), ".b { color: red; }\n")
	code, out, errOut := runLessc(t, dir, "", "--source-map", "a.less", "out/a.css")
	if code != 0 || out != "" || errOut != "" {
		t.Fatalf("compile: %d %q %q", code, out, errOut)
	}
	css, err := os.ReadFile(filepath.Join(dir, "out", "a.css"))
	if err != nil {
		t.Fatal(err)
	}
	want := ".b {\n  color: red;\n}\n.a {\n  width: 3px;\n}\n/*# sourceMappingURL=a.css.map */"
	if string(css) != want {
		t.Errorf("css %q, want %q", css, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "a.css.map")); err != nil {
		t.Errorf("no map file: %v", err)
	}
	code, out, _ = runLessc(t, dir, "", "-M", "a.less", "x.css")
	if code != 0 || out != "x.css: "+filepath.Join(dir, "b.less")+" \n" {
		t.Errorf("-M: %d %q", code, out)
	}
}

func TestStdinAndErrors(t *testing.T) {
	dir := t.TempDir()
	code, out, _ := runLessc(t, dir, ".a { b: c }", "-x", "-")
	if code != 0 || out != ".a{b:c}" {
		t.Errorf("stdin: %d %q", code, out)
	}
	code, out, errOut := runLessc(t, dir, ".a { b: @nope; }", "--no-color", "-")
	wantErr := "NameError: variable @nope is undefined in - on line 1, column 9:\n1 .a { b: @nope; }\n\n"
	if code != 1 || out != "" || errOut != wantErr {
		t.Errorf("error: %d %q %q", code, out, errOut)
	}
	code, _, errOut = runLessc(t, dir, "", "missing.less")
	if code != 1 || errOut != "lessc: ENOENT: no such file or directory, open '"+filepath.Join(dir, "missing.less")+"'\n" {
		t.Errorf("missing input: %d %q", code, errOut)
	}
}
