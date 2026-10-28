package jsre

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/less/less.js/internal/js"
)

// TestPatterns checks compile errors and first matches against the results of the V8 engine.
func TestPatterns(t *testing.T) {
	cases := []struct {
		pattern, flags, input, want string
	}{
		{"[z-a]+", "g", "abc", "ERR \"Invalid regular expression: /[z-a]+/g: Range out of order in character class\""},
		{"[", "", "x", "ERR \"Invalid regular expression: /[/: Unterminated character class\""},
		{"(", "gi", "x", "ERR \"Invalid regular expression: /(/gi: Unterminated group\""},
		{"a)", "", "x", "ERR \"Invalid regular expression: /a)/: Unmatched ')'\""},
		{"*a", "", "", "ERR \"Invalid regular expression: /*a/: Nothing to repeat\""},
		{"a**", "", "", "ERR \"Invalid regular expression: /a**/: Nothing to repeat\""},
		{"a{2,1}", "", "", "ERR \"Invalid regular expression: /a{2,1}/: numbers out of order in {} quantifier\""},
		{"{1}", "", "", "ERR \"Invalid regular expression: /{1}/: Nothing to repeat\""},
		{"a{", "", "a{", "[\"a{\"]"},
		{"{", "", "{", "[\"{\"]"},
		{"]", "", "]", "[\"]\"]"},
		{"}", "", "}", "[\"}\"]"},
		{"(?<=a)*", "", "", "ERR \"Invalid regular expression: /(?<=a)*/: Invalid quantifier\""},
		{"(?=a)*", "", "a", "[\"\"]"},
		{"(?=a)*", "u", "a", "ERR \"Invalid regular expression: /(?=a)*/u: Invalid quantifier\""},
		{"^*", "", "", "ERR \"Invalid regular expression: /^*/: Nothing to repeat\""},
		{"\\b+", "", "", "ERR \"Invalid regular expression: /\\\\b+/: Nothing to repeat\""},
		{"(?<n>a)\\k<n>", "", "aa", "[\"aa\",\"a\"]"},
		{"\\k<n>", "", "k<n>", "[\"k<n>\"]"},
		{"(?<n>a)\\k<m>", "", "", "ERR \"Invalid regular expression: /(?<n>a)\\\\k<m>/: Invalid named capture referenced\""},
		{"(?<n>a)(?<n>b)", "", "", "ERR \"Invalid regular expression: /(?<n>a)(?<n>b)/: Duplicate capture group name\""},
		{"[]]+", "", "a]b", "null"},
		{"[^]", "", "\n", "[\"\\n\"]"},
		{"[]", "", "a", "null"},
		{"\\", "", "", "ERR \"Invalid regular expression: /\\\\/: \\\\ at end of pattern\""},
		{"a\\", "", "", "ERR \"Invalid regular expression: /a\\\\/: \\\\ at end of pattern\""},
		{"\\c", "", "\\c", "[\"\\\\c\"]"},
		{"\\cA", "", "\u0001", "[\"\\u0001\"]"},
		{"[\\c1]", "", "\u0011", "[\"\\u0011\"]"},
		{"[\\c]", "", "\\", "[\"\\\\\"]"},
		{"\\1(a)", "", "a", "[\"a\",\"a\"]"},
		{"(a)\\1", "", "aa", "[\"aa\",\"a\"]"},
		{"\\2(a)", "", "\u0002a", "[\"\\u0002a\",\"a\"]"},
		{"\\8", "", "8", "[\"8\"]"},
		{"\\08", "", "\u00008", "[\"\\u00008\"]"},
		{"\\377", "", "ÿ", "[\"ÿ\"]"},
		{"\\400", "", " 0", "[\" 0\"]"},
		{"\\u{41}", "", "u{41}", "null"},
		{"\\u{41}", "u", "A", "[\"A\"]"},
		{"\\u{110000}", "u", "", "ERR \"Invalid regular expression: /\\\\u{110000}/u: Invalid Unicode escape\""},
		{"\\xZ", "", "xZ", "[\"xZ\"]"},
		{"\\xZ", "u", "", "ERR \"Invalid regular expression: /\\\\xZ/u: Invalid escape\""},
		{"\\p{L}", "", "p{L}", "[\"p{L}\"]"},
		{"\\p{L}+", "u", "héllo", "[\"héllo\"]"},
		{"\\p{Foo}", "u", "", "ERR \"Invalid regular expression: /\\\\p{Foo}/u: Invalid property name\""},
		{"[\\p{Foo}]", "u", "", "ERR \"Invalid regular expression: /[\\\\p{Foo}]/u: Invalid property name in character class\""},
		{"\\-", "u", "", "ERR \"Invalid regular expression: /\\\\-/u: Invalid escape\""},
		{"[\\-]", "u", "-", "[\"-\"]"},
		{"\\a", "u", "", "ERR \"Invalid regular expression: /\\\\a/u: Invalid escape\""},
		{"\\a", "", "a", "[\"a\"]"},
		{"[a-\\d]", "", "-", "[\"-\"]"},
		{"[a-\\d]", "u", "", "ERR \"Invalid regular expression: /[a-\\\\d]/u: Invalid character class\""},
		{"[\\d-z]", "", "-", "[\"-\"]"},
		{"a{,5}", "", "a{,5}", "[\"a{,5}\"]"},
		{"a{,5}", "u", "", "ERR \"Invalid regular expression: /a{,5}/u: Incomplete quantifier\""},
		{"x{1", "u", "", "ERR \"Invalid regular expression: /x{1/u: Incomplete quantifier\""},
		{"(?:a|b)+c", "", "abac", "[\"abac\"]"},
		{"(a|ab)(c|bcd)(d*)", "", "abcd", "[\"abcd\",\"a\",\"bcd\",\"\"]"},
		{"(a*)*b", "", "aab", "[\"aab\",\"aa\"]"},
		{"(a*)+?b", "", "aab", "[\"aab\",\"aa\"]"},
		{"(z)((a+)?(b+)?(c))*", "", "zaacbbbcac", "[\"zaacbbbcac\",\"z\",\"ac\",\"a\",null,\"c\"]"},
		{"^\\s+$", "", "\u000b\u00a0\ufeff", "[\"\\u000b\u00a0\ufeff\"]"},
		{"^\\s+$", "", "", "null"},
		{"^\\s+$", "", "᠎", "null"},
		{"^[A-Z]+$", "i", "ſ", "null"},
		{"^[A-Z]+$", "i", "K", "null"},
		{"^[A-Z]+$", "iu", "ſK", "[\"ſK\"]"},
		{"ß", "i", "ẞ", "null"},
		{"ß", "iu", "ẞ", "[\"ẞ\"]"},
		{"^.$", "", "\r", "null"},
		{"^.$", "", "\u2028", "null"},
		{"^.$", "s", "\r", "[\"\\r\"]"},
		{"^.$", "", "😀", "null"},
		{"^.$", "u", "😀", "[\"😀\"]"},
		{"(?<=\\$)\\d+", "", "cost $42", "[\"42\"]"},
		{"(?<!\\$)\\d+", "", "$42 17", "[\"2\"]"},
		{"(?<=(\\d)(\\d))x", "", "12x", "[\"x\",\"1\",\"2\"]"},
		{"(?=(\\d+))\\w+\\1", "", "123123", "null"},
		{"\\bfoo\\b", "", "a foo b", "[\"foo\"]"},
		{"\\Bfoo", "", "afoo", "[\"foo\"]"},
		{"^a$", "m", "b\na\nc", "[\"a\"]"},
		{"(a)|b", "", "b", "[\"b\",null]"},
		{"(?:(a)|b)+", "", "ab", "[\"ab\",null]"},
		{"(a)?\\1b", "", "b", "[\"b\",null]"},
		{"[\\w-]+", "", "a-b_c", "[\"a-b_c\"]"},
		{"[\\s\\S]", "", "", "null"},
		{"\\0", "", "\u0000", "[\"\\u0000\"]"},
		{"a|", "", "b", "[\"\"]"},
		{"|a", "", "a", "[\"\"]"},
		{"()", "", "", "[\"\",\"\"]"},
		{"(?:)", "", "", "[\"\"]"},
		{"a{3}", "", "aaaa", "[\"aaa\"]"},
		{"a{2,}?", "", "aaaa", "[\"aa\"]"},
		{"a??b", "", "ab", "[\"ab\"]"},
		{"\\W", "iu", "ſ", "null"},
		{"\\w", "iu", "ſ", "[\"ſ\"]"},
		{"(?<a>.)(?<b>.)", "", "xy", "[\"xy\",\"x\",\"y\"]"},
		{"[İ]", "i", "i", "null"},
		{"[i]", "i", "İ", "null"},
		{"ı", "i", "I", "null"},
		{"σ", "i", "Σ", "[\"Σ\"]"},
		{"ς", "i", "Σ", "[\"Σ\"]"},
		{"ς", "iu", "σ", "[\"σ\"]"},
		{"[^a]", "i", "A", "null"},
		{"[^\\W]", "i", "K", "null"},
		{"x*", "y", "axx", "[\"\"]"},
		{"\\d+", "g", "ab12", "[\"12\"]"},
		{"[😀]", "u", "😀", "[\"😀\"]"},
		{"a", "gg", "", "ERR \"Invalid flags supplied to RegExp constructor 'gg'\""},
		{"a", "x", "", "ERR \"Invalid flags supplied to RegExp constructor 'x'\""},
		{"a", "uv", "", "ERR \"Invalid flags supplied to RegExp constructor 'uv'\""},
		{"(?<1a>x)", "", "", "ERR \"Invalid regular expression: /(?<1a>x)/: Invalid capture group name\""},
		{"(?<$a>x)\\k<$a>", "", "xx", "[\"xx\",\"x\"]"},
		{"\\k", "u", "", "ERR \"Invalid regular expression: /\\\\k/u: Invalid named reference\""},
		{"(?<a>x)\\k", "", "", "ERR \"Invalid regular expression: /(?<a>x)\\\\k/: Invalid named reference\""},
		{"[a-z", "", "", "ERR \"Invalid regular expression: /[a-z/: Unterminated character class\""},
		{"(?", "", "", "ERR \"Invalid regular expression: /(?/: Invalid group\""},
		{"(?<", "", "", "ERR \"Invalid regular expression: /(?</: Invalid capture group name\""},
		{"(?<a", "", "", "ERR \"Invalid regular expression: /(?<a/: Invalid capture group name\""},
		{"a{1,2}{3}", "", "", "ERR \"Invalid regular expression: /a{1,2}{3}/: Nothing to repeat\""},
		{"a(?=b)?", "", "a", "[\"a\"]"},
		{"\\1", "", "", "null"},
		{"\\1", "u", "", "ERR \"Invalid regular expression: /\\\\1/u: Invalid escape\""},
		{"[\\1]", "", "\u0001", "[\"\\u0001\"]"},
		{"[\\1]", "u", "", "ERR \"Invalid regular expression: /[\\\\1]/u: Invalid decimal escape\""},
		{"\\cz", "", "\u001a", "[\"\\u001a\"]"},
		{"[\\b]", "", "\b", "[\"\\b\"]"},
		{"\\b", "u", "", "null"},
	}
	for _, c := range cases {
		if got := firstMatch(c.pattern, c.flags, c.input); got != c.want {
			t.Errorf("/%s/%s on %q = %s, want %s", c.pattern, c.flags, c.input, got, c.want)
		}
	}
}

// firstMatch renders a compile error as ERR "message", a failed search as null and a match as a JSON array.
func firstMatch(pattern, flags, input string) string {
	re, err := Compile(pattern, flags)
	if err != nil {
		return "ERR " + marshal(err.Message)
	}
	in := js.ToU16(input)
	caps := re.Search(in, 0)
	if caps == nil {
		return "null"
	}
	var out []any
	for i := 0; i < len(caps); i += 2 {
		if caps[i] < 0 {
			out = append(out, nil)
		} else {
			out = append(out, js.FromU16(in[caps[i]:caps[i+1]]))
		}
	}
	return marshal(out)
}

func marshal(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "marshal error: " + err.Error()
	}
	return strings.TrimSuffix(b.String(), "\n")
}
