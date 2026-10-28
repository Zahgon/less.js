package js

import (
	"strings"
	"unicode/utf8"
)

// decodeWTF8 decodes s into code points, keeping lone surrogates (stored as three-byte sequences) and joining a
// high surrogate followed by a low surrogate into one code point.
func decodeWTF8(s string) []rune {
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			out = append(out, rune(c))
			i++
			continue
		}
		if c == 0xED && i+2 < len(s) && s[i+1] >= 0xA0 && s[i+1] <= 0xBF && s[i+2]&0xC0 == 0x80 {
			cp := rune(0xD000) | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F)
			i += 3
			if cp >= 0xDC00 && len(out) > 0 {
				prev := out[len(out)-1]
				if prev >= 0xD800 && prev <= 0xDBFF {
					out[len(out)-1] = 0x10000 + (prev-0xD800)<<10 + (cp - 0xDC00)
					continue
				}
			}
			out = append(out, cp)
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		out = append(out, r)
		i += size
	}
	return out
}

// ToU16 converts a string to UTF-16 code units.
func ToU16(s string) []uint16 {
	cps := decodeWTF8(s)
	out := make([]uint16, 0, len(cps))
	for _, r := range cps {
		if r > 0xFFFF {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
			continue
		}
		out = append(out, uint16(r))
	}
	return out
}

// FromU16 converts UTF-16 code units to a string, keeping lone surrogates in their three-byte form.
func FromU16(u []uint16) string {
	var b strings.Builder
	b.Grow(len(u))
	for i := 0; i < len(u); i++ {
		c := u[i]
		switch {
		case c < 0x80:
			b.WriteByte(byte(c))
		case c >= 0xD800 && c <= 0xDBFF && i+1 < len(u) && u[i+1] >= 0xDC00 && u[i+1] <= 0xDFFF:
			r := 0x10000 + (rune(c)-0xD800)<<10 + (rune(u[i+1]) - 0xDC00)
			b.WriteRune(r)
			i++
		case c >= 0xD800 && c <= 0xDFFF:
			b.WriteByte(0xED)
			b.WriteByte(byte(0x80 | (c>>6)&0x3F))
			b.WriteByte(byte(0x80 | c&0x3F))
		default:
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}

// CodePoints returns the code points of s, lone surrogates included.
func CodePoints(s string) []rune { return decodeWTF8(s) }

// FromCodePoints builds a string from code points, lone surrogates included.
func FromCodePoints(cps []rune) string {
	var u []uint16
	for _, r := range cps {
		if r > 0xFFFF {
			r -= 0x10000
			u = append(u, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
			continue
		}
		u = append(u, uint16(r))
	}
	return FromU16(u)
}

// HasLoneSurrogate reports whether s holds an unpaired surrogate.
func HasLoneSurrogate(s string) bool {
	for _, r := range decodeWTF8(s) {
		if r >= 0xD800 && r <= 0xDFFF {
			return true
		}
	}
	return false
}

// Slice16 returns the code units of s from start up to end, clamped like String.prototype.substring.
func Slice16(s string, start, end int) string {
	u := ToU16(s)
	if start < 0 {
		start = 0
	}
	if end > len(u) {
		end = len(u)
	}
	if start >= end {
		return ""
	}
	return FromU16(u[start:end])
}

// SliceJS implements String.prototype.slice with integer arguments (negative counts from the end).
func SliceJS(s string, start, end int) string {
	n := Len16(s)
	if start < 0 {
		start += n
		if start < 0 {
			start = 0
		}
	}
	if end < 0 {
		end += n
		if end < 0 {
			end = 0
		}
	}
	if end > n {
		end = n
	}
	return Slice16(s, start, end)
}

// Substring implements String.prototype.substring with integer arguments.
func Substring(s string, start, end int) string {
	n := Len16(s)
	clamp := func(x int) int {
		if x < 0 {
			return 0
		}
		if x > n {
			return n
		}
		return x
	}
	start, end = clamp(start), clamp(end)
	if start > end {
		start, end = end, start
	}
	return Slice16(s, start, end)
}

// CharAt returns the code unit at index i as a string, or "" when out of range.
func CharAt(s string, i int) string {
	u := ToU16(s)
	if i < 0 || i >= len(u) {
		return ""
	}
	return FromU16(u[i : i+1])
}

// CharCodeAt returns the code unit at index i, or -1 when out of range.
func CharCodeAt(s string, i int) int {
	u := ToU16(s)
	if i < 0 || i >= len(u) {
		return -1
	}
	return int(u[i])
}

// IndexOf16 implements String.prototype.indexOf, returning a code-unit index.
func IndexOf16(s, sub string, from int) int {
	u, w := ToU16(s), ToU16(sub)
	if from < 0 {
		from = 0
	}
	if from > len(u) {
		from = len(u)
	}
	for i := from; i+len(w) <= len(u); i++ {
		match := true
		for j := range w {
			if u[i+j] != w[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// LastIndexOf16 implements String.prototype.lastIndexOf without a position argument.
func LastIndexOf16(s, sub string) int {
	u, w := ToU16(s), ToU16(sub)
	for i := len(u) - len(w); i >= 0; i-- {
		match := true
		for j := range w {
			if u[i+j] != w[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// Split16 implements String.prototype.split with a string separator and no limit.
func Split16(s, sep string) []string {
	u, w := ToU16(s), ToU16(sep)
	if len(w) == 0 {
		out := make([]string, len(u))
		for i := range u {
			out[i] = FromU16(u[i : i+1])
		}
		return out
	}
	var out []string
	start := 0
	for i := 0; i+len(w) <= len(u); {
		match := true
		for j := range w {
			if u[i+j] != w[j] {
				match = false
				break
			}
		}
		if match {
			out = append(out, FromU16(u[start:i]))
			i += len(w)
			start = i
			continue
		}
		i++
	}
	return append(out, FromU16(u[start:]))
}

// ReplaceFirst implements String.prototype.replace with a string pattern and a plain replacement text: only the
// first occurrence changes and $ patterns in the replacement are expanded.
func ReplaceFirst(s, pattern, repl string) string {
	i := IndexOf16(s, pattern, 0)
	if i < 0 {
		return s
	}
	u := ToU16(s)
	pl := Len16(pattern)
	before, after := FromU16(u[:i]), FromU16(u[i+pl:])
	return before + ExpandReplacement(repl, pattern, s, i, nil, nil) + after
}

// ReplaceAllString implements String.prototype.replaceAll with a string pattern.
func ReplaceAllString(s, pattern, repl string) string {
	parts := Split16(s, pattern)
	if len(parts) == 1 {
		return s
	}
	var b strings.Builder
	pos := 0
	for i, p := range parts {
		if i > 0 {
			b.WriteString(ExpandReplacement(repl, pattern, s, pos, nil, nil))
			pos += Len16(pattern)
		}
		b.WriteString(p)
		pos += Len16(p)
	}
	return b.String()
}

// Repeat implements String.prototype.repeat for a count already checked to be finite and non-negative.
func Repeat(s string, n int) string { return strings.Repeat(s, n) }

// IsWhiteSpaceOrLT reports whether c is in the WhiteSpace or LineTerminator sets that String.prototype.trim and
// the regular expression class \s use.
func IsWhiteSpaceOrLT(c rune) bool {
	switch c {
	case 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x20, 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return c >= 0x2000 && c <= 0x200A
}

// Trim implements String.prototype.trim.
func Trim(s string) string { return TrimEnd(TrimStart(s)) }

// TrimStart implements String.prototype.trimStart.
func TrimStart(s string) string {
	cps := decodeWTF8(s)
	i := 0
	for i < len(cps) && IsWhiteSpaceOrLT(cps[i]) {
		i++
	}
	return FromCodePoints(cps[i:])
}

// TrimEnd implements String.prototype.trimEnd.
func TrimEnd(s string) string {
	cps := decodeWTF8(s)
	j := len(cps)
	for j > 0 && IsWhiteSpaceOrLT(cps[j-1]) {
		j--
	}
	return FromCodePoints(cps[:j])
}

// Len16 returns the length of s in UTF-16 code units.
func Len16(s string) int {
	n := 0
	for _, u := range decodeWTF8(s) {
		if u > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}
