package js

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// URIError is the error encodeURI and encodeURIComponent throw for a lone surrogate.
type URIError struct{}

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
