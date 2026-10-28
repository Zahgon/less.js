// Package imagesize reads the width and height of an image the way the synchronous probe of probe-image-size
// 7.3.0 does, trying the AVIF/HEIF, BMP, GIF, ICO, JPEG, PNG, PSD, SVG, TIFF and WebP parsers in that order.
//
// Byte reads follow the JavaScript originals: a read past the end of the buffer is undefined, which is 0 inside
// a bitwise expression and NaN inside an arithmetic one, so offsets and sizes are float64.
package imagesize

import (
	"math"

	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
)

// Size is a probed image size.
type Size struct {
	Width, Height float64
}

type buffer []byte

// at is data[offset]: ok is false when the index is not an integer inside the buffer (undefined).
func (d buffer) at(off float64) (int, bool) {
	if off != math.Trunc(off) || off < 0 || off >= float64(len(d)) {
		return 0, false
	}
	return int(d[int(off)]), true
}

// bit is data[offset] inside a bitwise expression, where undefined is 0.
func (d buffer) bit(off float64) int {
	v, _ := d.at(off)
	return v
}

func (d buffer) eq(off float64, sig string) bool {
	for j := 0; j < len(sig); j++ {
		v, ok := d.at(off + float64(j))
		if !ok || v != int(sig[j]) {
			return false
		}
	}
	return true
}

func (d buffer) u16le(off float64) float64 { return float64(d.bit(off) | d.bit(off+1)<<8) }

func (d buffer) u16be(off float64) float64 { return float64(d.bit(off)<<8 | d.bit(off+1)) }

func (d buffer) i16le(off float64) float64 {
	return float64(int16(uint16(d.bit(off) | d.bit(off+1)<<8)))
}

func (d buffer) u32le(off float64) float64 {
	low := float64(int32(d.bit(off) | d.bit(off+1)<<8 | d.bit(off+2)<<16))
	hi, ok := d.at(off + 3)
	if !ok {
		return math.NaN()
	}
	return low + float64(hi)*0x1000000
}

func (d buffer) u32be(off float64) float64 {
	hi, ok := d.at(off)
	if !ok {
		return math.NaN()
	}
	return float64(hi)*0x1000000 + float64(int32(d.bit(off+1)<<16|d.bit(off+2)<<8|d.bit(off+3)))
}

// toInt32 is the JavaScript ToInt32 conversion of a bitwise operand.
func toInt32(f float64) int32 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return int32(uint32(int64(math.Mod(math.Trunc(f), 4294967296))))
}

// slice is Buffer.prototype.slice with integer bounds clamped to the buffer.
func (d buffer) slice(start, end float64) buffer {
	clamp := func(v float64) int {
		if math.IsNaN(v) {
			return 0
		}
		if v < 0 {
			v = math.Max(0, float64(len(d))+v)
		}
		return int(math.Min(math.Trunc(v), float64(len(d))))
	}
	s, e := clamp(start), clamp(end)
	if e < s {
		return buffer{}
	}
	return d[s:e]
}

// Probe returns the image size, or false when no parser recognises the data or the size is not positive.
func Probe(data []byte) (Size, bool) {
	d := buffer(data)
	for _, parse := range []func(buffer) (Size, bool){avif, bmp, gif, ico, jpeg, png, psd, svg, tiff, webp} {
		if s, ok := parse(d); ok {
			if s.Width > 0 && s.Height > 0 {
				return s, true
			}
			return Size{}, false
		}
	}
	return Size{}, false
}

func png(d buffer) (Size, bool) {
	if len(d) < 24 || !d.eq(0, "\x89PNG\r\n\x1a\n") || !d.eq(12, "IHDR") {
		return Size{}, false
	}
	return Size{d.u32be(16), d.u32be(20)}, true
}

func gif(d buffer) (Size, bool) {
	if len(d) < 10 || !d.eq(0, "GIF87a") && !d.eq(0, "GIF89a") {
		return Size{}, false
	}
	return Size{d.u16le(6), d.u16le(8)}, true
}

func bmp(d buffer) (Size, bool) {
	if len(d) < 26 || !d.eq(0, "BM") {
		return Size{}, false
	}
	switch header := d.u32le(14); {
	case header == 12:
		return Size{d.i16le(18), math.Abs(d.i16le(20))}, true
	case header > 12:
		return Size{float64(toInt32(d.u32le(18))), math.Abs(float64(toInt32(d.u32le(22))))}, true
	}
	return Size{}, false
}

func psd(d buffer) (Size, bool) {
	if len(d) < 22 || !d.eq(0, "8BPS\x00\x01") {
		return Size{}, false
	}
	return Size{d.u32be(18), d.u32be(14)}, true
}

func ico(d buffer) (Size, bool) {
	header, typ, n := d.u16le(0), d.u16le(2), d.u16le(4)
	if header != 0 || typ != 1 || n == 0 || float64(len(d)) < 6+n*16 {
		return Size{}, false
	}
	var best Size
	for i := 0.0; i < n; i++ {
		w, h := float64(d.bit(6+16*i)), float64(d.bit(6+16*i+1))
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		if w > best.Width || h > best.Height {
			best = Size{w, h}
		}
	}
	return best, true
}

func jpeg(d buffer) (Size, bool) {
	if len(d) < 2 {
		return Size{}, false
	}
	if v, ok := d.at(2); d.bit(0) != 0xFF || d.bit(1) != 0xD8 || !ok || v != 0xFF {
		return Size{}, false
	}
	n := float64(len(d))
	off := 2.0
	for {
		for {
			if n-off < 2 {
				return Size{}, false
			}
			v, ok := d.at(off)
			off++
			if ok && v == 0xFF {
				break
			}
		}
		code, ok := d.at(off)
		off++
		for ok && code == 0xFF {
			code, ok = d.at(off)
			off++
		}
		var length float64
		switch {
		case ok && (code >= 0xD0 && code <= 0xD9 || code == 0x01):
			length = 0
		case ok && code >= 0xC0 && code <= 0xFE:
			if n-off < 2 {
				return Size{}, false
			}
			length = d.u16be(off) - 2
			off += 2
		default:
			return Size{}, false
		}
		if code == 0xD9 || code == 0xDA {
			return Size{}, false
		}
		if length >= 5 && code >= 0xC0 && code <= 0xCF && code != 0xC4 && code != 0xC8 && code != 0xCC {
			if n-off < length {
				return Size{}, false
			}
			return Size{d.u16be(off + 3), d.u16be(off + 1)}, true
		}
		off += length
	}
}

func tiff(d buffer) (Size, bool) {
	if len(d) < 8 || !d.eq(0, "II\x2A\x00") && !d.eq(0, "MM\x00\x2A") {
		return Size{}, false
	}
	big := d.bit(0) == 77
	u16 := func(off float64) float64 {
		if big {
			return d.u16be(off)
		}
		return d.u16le(off)
	}
	u32 := func(off float64) float64 {
		if big {
			return d.u32be(off)
		}
		return d.u32le(off)
	}
	value := func(off float64) float64 {
		typ, count := u16(off+2), u32(off+4)
		if count != 1 || typ != 3 && typ != 4 {
			return 0
		}
		if typ == 3 {
			return u16(off + 8)
		}
		return u32(off + 8)
	}
	count := u32(4) - 8
	if count < 0 || math.IsNaN(count) {
		return Size{}, false
	}
	off := count + 8
	if float64(len(d))-off < 2 {
		return Size{}, false
	}
	ifd := u16(off) * 12
	if ifd <= 0 {
		return Size{}, false
	}
	off += 2
	if float64(len(d))-off < ifd {
		return Size{}, false
	}
	var w, h float64
	for i := 0.0; i < ifd; i += 12 {
		switch u16(off + i) {
		case 256:
			w = value(off + i)
		case 257:
			h = value(off + i)
		}
	}
	if w == 0 || h == 0 || math.IsNaN(w) || math.IsNaN(h) {
		return Size{}, false
	}
	return Size{w, h}, true
}

var (
	reSVGHeader  = jsre.MustCompile(`<[-_.:a-zA-Z0-9][^>]*>`, "")
	reSVGTag     = jsre.MustCompile(`^<([-_.:a-zA-Z0-9]+:)?svg\s`, "")
	reSVGWidth   = jsre.MustCompile(`[^-]\bwidth="([^%]+?)"|[^-]\bwidth='([^%]+?)'`, "")
	reSVGHeight  = jsre.MustCompile(`\bheight="([^%]+?)"|\bheight='([^%]+?)'`, "")
	reSVGViewbox = jsre.MustCompile(`\bview[bB]ox="(.+?)"|\bview[bB]ox='(.+?)'`, "")
)

func isFinitePositive(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 }

// attr returns match[1] || match[2] of the first match, and whether it is a non-empty string.
func attr(r *jsre.Regexp, s string) (string, bool) {
	u := js.ToU16(s)
	m := r.Search(u, 0)
	if m == nil {
		return "", false
	}
	for g := 1; g <= 2; g++ {
		if m[2*g] >= 0 && m[2*g+1] > m[2*g] {
			return js.FromU16(u[m[2*g]:m[2*g+1]]), true
		}
	}
	return "", false
}

func svg(d buffer) (Size, bool) {
	i := 0
	if len(d) >= 3 && d[0] == 0xEF && d[1] == 0xBB && d[2] == 0xBF {
		i = 3
	}
	for i < len(d) && (d[i] == 0x20 || d[i] == 0x09 || d[i] == 0x0D || d[i] == 0x0A) {
		i++
	}
	if i >= len(d) || d[i] != '<' {
		return Size{}, false
	}
	runes := make([]rune, len(d))
	for k, b := range d {
		runes[k] = rune(b)
	}
	str := js.ToU16(string(runes))
	tag := ""
	if m := reSVGHeader.Search(str, 0); m != nil {
		tag = js.FromU16(str[m[0]:m[1]])
	}
	if !reSVGTag.Test(tag) {
		return Size{}, false
	}
	ws, hasW := attr(reSVGWidth, tag)
	hs, hasH := attr(reSVGHeight, tag)
	vb, _ := attr(reSVGViewbox, tag)
	width, height := js.ParseFloat(ws), js.ParseFloat(hs)
	if hasW && hasH {
		if !isFinitePositive(width) || !isFinitePositive(height) {
			return Size{}, false
		}
		return Size{width, height}, true
	}
	parts := js.Split16(vb, " ")
	part := func(k int) (string, bool) {
		if k < len(parts) {
			return parts[k], true
		}
		return "", false
	}
	vw, okW := part(2)
	vh, okH := part(3)
	vbWidth, vbHeight := math.NaN(), math.NaN()
	if okW {
		vbWidth = js.ParseFloat(vw)
	}
	if okH {
		vbHeight = js.ParseFloat(vh)
	}
	if !isFinitePositive(vbWidth) || !isFinitePositive(vbHeight) || svgUnits(vw) != svgUnits(vh) {
		return Size{}, false
	}
	ratio := vbWidth / vbHeight
	switch {
	case hasW:
		if !isFinitePositive(width) {
			return Size{}, false
		}
		return Size{width, width / ratio}, true
	case hasH:
		if !isFinitePositive(height) {
			return Size{}, false
		}
		return Size{height * ratio, height}, true
	}
	return Size{vbWidth, vbHeight}, true
}

// svgUnits is the units() helper: the trailing unit of a length, or px.
func svgUnits(s string) string {
	for _, u := range []string{"in", "mm", "cm", "pt", "pc", "px", "em", "ex"} {
		if len(s) >= 2 && s[len(s)-2:] == u {
			return u
		}
	}
	return "px"
}

func latin1(d buffer) string {
	runes := make([]rune, len(d))
	for k, b := range d {
		runes[k] = rune(b)
	}
	return string(runes)
}

func webp(d buffer) (Size, bool) {
	if len(d) < 16 || !d.eq(0, "RIFF") || !d.eq(8, "WEBP") {
		return Size{}, false
	}
	fileLength := d.u32le(4) + 8
	if fileLength > float64(len(d)) {
		return Size{}, false
	}
	var result *Size
	for off := 12.0; off+8 < fileLength; {
		if v, ok := d.at(off); ok && v == 0 {
			off++
			continue
		}
		header := latin1(d.slice(off, off+4))
		length := d.u32le(off + 4)
		p := off + 8
		switch {
		case header == "VP8 " && length >= 10:
			if result == nil && d.bit(p+3) == 0x9D && d.bit(p+4) == 0x01 && d.bit(p+5) == 0x2A {
				result = &Size{float64(int(d.u16le(p+6)) & 0x3FFF), float64(int(d.u16le(p+8)) & 0x3FFF)}
			}
		case header == "VP8L" && length >= 5:
			if v, ok := d.at(p); result == nil && ok && v == 0x2F {
				bits := toInt32(d.u32le(p + 1))
				result = &Size{float64(bits&0x3FFF) + 1, float64((bits>>14)&0x3FFF) + 1}
			}
		case header == "VP8X" && length >= 10:
			if result == nil {
				result = &Size{float64(d.bit(p+6)<<16|d.bit(p+5)<<8|d.bit(p+4)) + 1,
					float64(d.bit(p+9)<<16|d.bit(p+8)<<8|d.bit(p+7)) + 1}
			}
		case header == "EXIF":
			off = math.Inf(1)
		}
		off += 8 + length
	}
	if result == nil {
		return Size{}, false
	}
	return *result, true
}

type box struct {
	typ  string
	data buffer
	end  float64
}

// unbox is miaf_utils.js unbox.
func unbox(d buffer, off float64) (box, bool) {
	if float64(len(d)) < 4+off {
		return box{}, false
	}
	size := d.u32be(off)
	if float64(len(d)) < size+off || size < 8 || math.IsNaN(size) {
		return box{}, false
	}
	return box{typ: latin1(d.slice(off+4, off+8)), data: d.slice(off+8, off+size), end: off + size}, true
}

// avifBrandOK is the brand test of getMimeType: it only matters whether a MIAF brand is present.
func avifBrandOK(d buffer) bool {
	compat := map[string]bool{latin1(d.slice(0, 4)): true}
	for i := 8.0; i < float64(len(d)); i += 4 {
		compat[latin1(d.slice(i, i+4))] = true
	}
	return compat["mif1"] || compat["msf1"] || compat["miaf"]
}

func avif(d buffer) (Size, bool) {
	if !d.eq(4, "ftyp") {
		return Size{}, false
	}
	first, ok := unbox(d, 0)
	if !ok || !avifBrandOK(first.data) {
		return Size{}, false
	}
	var meta buffer
	found := false
	for off := first.end; ; {
		b, ok := unbox(d, off)
		if !ok {
			break
		}
		off = b.end
		if b.typ == "mdat" {
			return Size{}, false
		}
		if b.typ == "meta" {
			meta, found = b.data, true
			break
		}
	}
	if !found {
		return Size{}, false
	}
	var sizes []Size
	scan := func(d buffer, off float64, fn func(box)) {
		for {
			b, ok := unbox(d, off)
			if !ok {
				return
			}
			fn(b)
			off = b.end
		}
	}
	scan(meta, 4, func(b box) {
		if b.typ == "iprp" {
			scan(b.data, 0, func(c box) {
				if c.typ == "ipco" {
					scan(c.data, 0, func(e box) {
						if e.typ == "ispe" {
							sizes = append(sizes, Size{e.data.u32be(4), e.data.u32be(8)})
						}
					})
				}
			})
		}
	})
	if len(sizes) == 0 {
		return Size{}, false
	}
	return maxSize(sizes), true
}

// maxSize is miaf_utils.js getMaxSize, with JavaScript comparisons (false for NaN).
func maxSize(sizes []Size) Size {
	maxW, maxH := sizes[0], sizes[0]
	for _, b := range sizes[1:] {
		if !(maxW.Width > b.Width || maxW.Width == b.Width && maxW.Height > b.Height) {
			maxW = b
		}
		if !(maxH.Height > b.Height || maxH.Height == b.Height && maxH.Width > b.Width) {
			maxH = b
		}
	}
	if maxW.Width > maxH.Height || maxW.Width == maxH.Height && maxW.Height > maxH.Width {
		return maxW
	}
	return maxH
}
