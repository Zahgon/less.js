package imagesize

import (
	"encoding/binary"
	"testing"
)

func pngHeader(w, h uint32) []byte {
	b := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR"), make([]byte, 13)...)
	binary.BigEndian.PutUint32(b[16:], w)
	binary.BigEndian.PutUint32(b[20:], h)
	return b
}

func TestProbe(t *testing.T) {
	ico := []byte{0, 0, 1, 0, 2, 0, 16, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	cases := []struct {
		name string
		data []byte
		want Size
		ok   bool
	}{
		{"png", pngHeader(3, 7), Size{3, 7}, true},
		{"gif", []byte("GIF89a\x05\x00\x09\x00\x00\x00"), Size{5, 9}, true},
		{"ico with a 0 (256) entry", ico, Size{256, 256}, true},
		{"svg width and height", []byte(`<svg width="10" height="20.5"></svg>`), Size{10, 20.5}, true},
		{"svg viewBox ratio", []byte(`<svg width="30" viewBox="0 0 3 4"/>`), Size{30, 40}, true},
		{"svg data-width is not width", []byte(`<svg data-width="9" viewBox="0 0 2 3"/>`), Size{2, 3}, true},
		{"zero width", pngHeader(0, 7), Size{}, false},
		{"unknown", []byte("not an image"), Size{}, false},
	}
	for _, c := range cases {
		got, ok := Probe(c.data)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: Probe = %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}
