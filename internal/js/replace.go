package js

import "strings"

// ExpandReplacement implements GetSubstitution: it expands the $ patterns of a replacement text for a match of
// matched at code-unit position in str. captures holds the capture groups (nil for an unmatched group) and groups
// the named groups, or nil when the pattern has none.
func ExpandReplacement(repl, matched, str string, position int, captures []*string, groups map[string]*string) string {
	if !strings.Contains(repl, "$") {
		return repl
	}
	r := ToU16(repl)
	var b strings.Builder
	m := len(captures)
	for i := 0; i < len(r); i++ {
		c := r[i]
		if c != '$' || i+1 >= len(r) {
			b.WriteString(FromU16(r[i : i+1]))
			continue
		}
		n := r[i+1]
		switch {
		case n == '$':
			b.WriteByte('$')
			i++
		case n == '&':
			b.WriteString(matched)
			i++
		case n == '`':
			b.WriteString(Slice16(str, 0, position))
			i++
		case n == '\'':
			b.WriteString(Slice16(str, position+Len16(matched), Len16(str)))
			i++
		case n >= '0' && n <= '9':
			d1 := int(n - '0')
			if i+2 < len(r) && r[i+2] >= '0' && r[i+2] <= '9' {
				d2 := d1*10 + int(r[i+2]-'0')
				if d2 >= 1 && d2 <= m {
					if captures[d2-1] != nil {
						b.WriteString(*captures[d2-1])
					}
					i += 2
					continue
				}
			}
			if d1 >= 1 && d1 <= m {
				if captures[d1-1] != nil {
					b.WriteString(*captures[d1-1])
				}
				i++
				continue
			}
			b.WriteByte('$')
		case n == '<':
			if groups == nil {
				b.WriteByte('$')
				continue
			}
			end := -1
			for j := i + 2; j < len(r); j++ {
				if r[j] == '>' {
					end = j
					break
				}
			}
			if end < 0 {
				b.WriteByte('$')
				continue
			}
			name := FromU16(r[i+2 : end])
			if v, ok := groups[name]; ok && v != nil {
				b.WriteString(*v)
			}
			i = end
		default:
			b.WriteByte('$')
		}
	}
	return b.String()
}
