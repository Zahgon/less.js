package less

import (
	"regexp"
	"strings"
	"testing"
)

// TestWarnings is the original suite's warning assertions: every warning and deprecation notice a render can emit,
// counted by the messages that match.
func TestWarnings(t *testing.T) {
	cases := []struct {
		label, src   string
		js, compress bool
		pattern      string
		count        int
	}{
		{"bare @var in an at-rule prelude", "@bar: x;\n@foo @bar { a: b }", false, false, `A bare @variable in an at-rule prelude is deprecated`, 1},
		{"@var inside [] is top-level", "@v: x;\n@foo bar[@v] { a: b }", false, false, `A bare @variable in an at-rule prelude is deprecated`, 1},
		{"@var inside () is a declaration value", "@v: 1px;\n@foo (x: @v) { a: b }", false, false, `A bare @variable in an at-rule prelude is deprecated`, 0},
		{"bare lookup in a @media prelude", "@m: { q: ~\"(min-width: 1px)\"; };\n@media @m[q] { a: b }", false, false, `A bare @variable in an at-rule prelude is deprecated`, 1},
		{"bare lookup in an at-rule name", "@m: { n: fade; };\n@keyframes @m[n] { from { a: b } }", false, false, `A bare @variable in an at-rule prelude is deprecated`, 1},
		{"bare lookup in an unknown at-rule prelude", "@m: { s: ~\"(display: grid)\"; };\n@supports @m[s] { a: b }", false, false, `A bare @variable in an at-rule prelude is deprecated`, 1},
		{"bare chained lookup warns once", "@m: { @n: { k: base; } };\n@layer @m[@n][k] { a: b }", false, false, `A bare @variable in an at-rule prelude is deprecated`, 1},
		{"interpolated lookup in a prelude", "@m: { q: ~\"(min-width: 1px)\"; };\n@media @{m[q]} { a: b }", false, false, `A bare @variable in an at-rule prelude is deprecated`, 0},
		{"interpolated lookup in an at-rule name", "@m: { n: fade; };\n@keyframes @{m[n]} { from { a: b } }", false, false, `A bare @variable in an at-rule prelude is deprecated`, 0},
		{"numeric-leading variable names", "@1: name;\n.@{1} { value: name }", false, false, `Variable names beginning with a number are deprecated`, 2},
		{"identifier-leading variable names", "@foo-1: name;\n.@{foo-1} { value: @foo-1 }", false, false, `Variable names beginning with a number are deprecated`, 0},
		{"dash-only variable definitions and references", "@-: name;\n.a { value: @- }", false, false, `dash-only variable names @- and @\{-\} are deprecated`, 2},
		{"dash-only variable interpolation", "@-: name;\n.@{-} { value: name }", false, false, `dash-only variable names @- and @\{-\} are deprecated`, 2},
		{"hyphen-led variable names", "@-foo: name;\n.a { value: @-foo }", false, false, `dash-only variable names @- and @\{-\} are deprecated`, 0},
		{"numeric-leading variable property names", "@1: prop;\n.a { @{1}: value }", false, false, `Variable names beginning with a number are deprecated`, 2},
		{"dash-only variable property names", "@-: prop;\n.a { @{-}: value }", false, false, `dash-only variable names @- and @\{-\} are deprecated`, 2},
		{"numeric-leading variable calls", "@1: { value: yes; }\n.a { @1(); }", false, false, `Variable names beginning with a number are deprecated`, 2},
		{"dash-only variable calls", "@-: { value: yes; }\n.a { @-(); }", false, false, `dash-only variable names @- and @\{-\} are deprecated`, 2},
		{"dash-only mixin definitions and calls", ".-() { value: yes }\n.a { .-() }", false, false, `dash-only mixin names \.-\(\) and #-\(\) are deprecated`, 2},
		{"hash dash-only mixin definitions and calls", "#-() { value: yes }\n.a { #-() }", false, false, `dash-only mixin names \.-\(\) and #-\(\) are deprecated`, 2},
		{"hyphen-led mixin names", ".-foo() { value: yes }\n.a { .-foo() }", false, false, `dash-only mixin names \.-\(\) and #-\(\) are deprecated`, 0},
		{"dynamic @charset interpolation", "@encoding: 8;\n@charset \"UTF-@{encoding}\";", false, false, `Dynamic @charset interpolation is deprecated`, 1},
		{"static @charset", "@charset \"UTF-8\";", false, false, `Dynamic @charset interpolation is deprecated`, 0},
		{"inline JavaScript (backtick)", "@x: `1 + 1`;\n.a { w: @x }", true, false, `Inline JavaScript evaluation`, 1},
		{"whitespace before mixin-call parens", ".m() { a: b }\n.a { .m () }", false, false, `Whitespace between a mixin name and parentheses`, 1},
		{"mixin call without parens", ".m() { a: b }\n.a { .m }", false, false, `Calling a mixin without parentheses is deprecated`, 1},
		{"the ./ operator", ".a { w: 2px ./ 1 }", false, false, `\./ operator is deprecated`, 1},
		{"targeting complex selectors with :extend", ".b .c { x: y }\n.a:extend(.b .c) { }", false, false, `Targeting complex selectors`, 1},
		{"an :extend with no matches", ".a:extend(.zzz) { }", false, false, `extend '.*' has no matches`, 1},
		{"the compress option", ".a { b: c }", false, true, `compress option has been deprecated`, 1},
		{"the @plugin directive", "@plugin \"less-plugin-nonexistent-xyz\";\n.a { b: c }", false, false, `@plugin directive is deprecated`, 1},
	}
	defer SetLogger(Logger{})
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			var warnings []string
			SetLogger(Logger{Warn: func(m string) { warnings = append(warnings, m) }})
			opts := DefaultOptions()
			opts.JavascriptEnabled, opts.Compress = c.js, c.compress
			// a render may fail after the warning (the missing plugin); the warning is what is asserted
			_, _ = Render(c.src, opts)
			re := regexp.MustCompile(c.pattern)
			n := 0
			for _, w := range warnings {
				if re.MatchString(w) {
					n++
				}
			}
			if n != c.count {
				t.Errorf("%d warnings match %q, want %d: %q", n, c.pattern, c.count, warnings)
			}
		})
	}
}

// TestDisablePluginRule checks that @plugin is refused when the option is set.
func TestDisablePluginRule(t *testing.T) {
	opts := DefaultOptions()
	opts.DisablePluginRule = true
	_, err := Render(`@plugin "../../plugin/some_plugin";`, opts)
	const want = "@plugin statements are not allowed when disablePluginRule is set to true"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want an error containing %q, got %v", want, err)
	}
}

// TestNoOptions renders an empty stylesheet with the default options.
func TestNoOptions(t *testing.T) {
	res, err := Render("", DefaultOptions())
	if err != nil || res.CSS != "" {
		t.Fatalf("got %v, %v", res, err)
	}
}
