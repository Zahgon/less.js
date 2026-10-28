package js

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// URIError is the error encodeURI and encodeURIComponent throw for a lone surrogate.
type URIError struct{}

// Error returns the message the JavaScript runtime gives a URIError.
func (e *URIError) Error() string { return "URIError: URI malformed" }

const (
	uriAlnumMark = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.!~*'()"
	uriReserved  = ";/?:@&=+$,#"
)

// EncodeURI implements the global encodeURI.
func EncodeURI(s string) (string, error) { return encodeURISet(s, uriAlnumMark+uriReserved) }

// EncodeURIComponent implements the global encodeURIComponent.
func EncodeURIComponent(s string) (string, error) { return encodeURISet(s, uriAlnumMark) }

func encodeURISet(s, unescaped string) (string, error) {
	var b strings.Builder
	const hex = "0123456789ABCDEF"
	for _, cp := range decodeWTF8(s) {
		if cp < utf8.RuneSelf && strings.IndexByte(unescaped, byte(cp)) >= 0 {
			b.WriteByte(byte(cp))
			continue
		}
		if cp >= 0xD800 && cp <= 0xDFFF {
			return "", &URIError{}
		}
		var buf [4]byte
		n := utf8.EncodeRune(buf[:], cp)
		for _, c := range buf[:n] {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String(), nil
}

// QuoteJSON quotes s the way JSON.stringify quotes a string: only the quote, the backslash, control characters and
// lone surrogates are escaped; '<', '>', '&', U+2028 and U+2029 stay raw.
func QuoteJSON(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, cp := range decodeWTF8(s) {
		switch cp {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			switch {
			case cp < 0x20 || (cp >= 0xD800 && cp <= 0xDFFF):
				h := strconv.FormatInt(int64(cp), 16)
				b.WriteString(`\u` + strings.Repeat("0", 4-len(h)) + h)
			default:
				b.WriteRune(cp)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// DecodeUTF8 decodes bytes the way Node's utf-8 decoder does (the WHATWG algorithm): every maximal invalid
// subsequence becomes one U+FFFD.
func DecodeUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var out strings.Builder
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			out.WriteByte(c)
			i++
			continue
		}
		need, lower, upper := 0, byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c >= 0xE0 && c <= 0xEF:
			need = 2
			if c == 0xE0 {
				lower = 0xA0
			} else if c == 0xED {
				upper = 0x9F
			}
		case c >= 0xF0 && c <= 0xF4:
			need = 3
			if c == 0xF0 {
				lower = 0x90
			} else if c == 0xF4 {
				upper = 0x8F
			}
		default:
			out.WriteString("\uFFFD")
			i++
			continue
		}
		j := i + 1
		for k := 0; k < need; k++ {
			if j >= len(b) || b[j] < lower || b[j] > upper {
				break
			}
			lower, upper = 0x80, 0xBF
			j++
		}
		if j-i == need+1 {
			out.Write(b[i:j])
		} else {
			out.WriteString("\uFFFD")
		}
		i = j
	}
	return out.String()
}
