package js

import "testing"

// The expected strings are what Buffer.toString('utf8') and Buffer.from(string) produce in Node.
func TestDecodeUTF8(t *testing.T) {
	cases := map[string]string{
		"plain":              "plain",
		"\xe2\x82\xac":       "\u20ac",
		"\xe2\x82a":          "\ufffda",
		"\xed\xa0\x80":       "\ufffd\ufffd\ufffd",
		"\xf0\x9f\x98":       "\ufffd",
		"\xc0\xaf":           "\ufffd\ufffd",
		"a\xffb\xf4\x90\x80": "a\ufffdb\ufffd\ufffd\ufffd",
	}
	for in, want := range cases {
		if got := DecodeUTF8([]byte(in)); got != want {
			t.Errorf("DecodeUTF8(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToUTF8(t *testing.T) {
	lone := FromU16([]uint16{'a', 0xD83D, 'b'})
	if got := ToUTF8(lone); got != "a\ufffdb" {
		t.Errorf("ToUTF8 = %q", got)
	}
	if got := ToUTF8("😀"); got != "😀" {
		t.Errorf("ToUTF8 changed a valid pair: %q", got)
	}
}
