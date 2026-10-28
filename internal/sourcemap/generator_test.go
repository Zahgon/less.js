package sourcemap

import "testing"

func TestEncodeVLQ(t *testing.T) {
	for v, want := range map[int]string{0: "A", 1: "C", -1: "D", 15: "e", 16: "gB", -16: "hB", 1000: "w+B", 123456: "gkxH"} {
		if got := EncodeVLQ(v); got != want {
			t.Errorf("EncodeVLQ(%d) = %q, want %q", v, got, want)
		}
	}
}

func TestGeneratorJSON(t *testing.T) {
	file := "out.css"
	g := NewGenerator(&file)
	src := "a.less"
	for _, m := range []Mapping{
		{Generated: Position{1, 0}, Original: &Position{1, 0}, Source: &src},
		{Generated: Position{1, 0}, Original: &Position{1, 0}, Source: &src},
		{Generated: Position{2, 2}, Original: &Position{2, 2}, Source: &src},
	} {
		if err := g.AddMapping(m); err != nil {
			t.Fatal(err)
		}
	}
	content := "a {\n  b: c;\n}\n"
	g.SetSourceContent(src, &content)
	want := `{"version":3,"sources":["a.less"],"names":[],"mappings":"AAAA;EACE","file":"out.css","sourcesContent":["a {\n  b: c;\n}\n"]}`
	if got := g.JSON(); got != want {
		t.Errorf("JSON() =\n%s\nwant\n%s", got, want)
	}
	empty := ""
	err := g.AddMapping(Mapping{Generated: Position{1, 0}, Original: &Position{1, 0}, Source: &empty})
	if err == nil || err.Error() != `Invalid mapping: {"generated":{"line":1,"column":0},"source":"","original":{"line":1,"column":0},"name":null}` {
		t.Errorf("unexpected validation error %v", err)
	}
}
