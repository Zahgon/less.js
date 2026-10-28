package jsre

import (
	"strings"

	"github.com/less/less.js/internal/js"
)

// Replace implements String.prototype.replace(re, replacement) for a replacement string: a global pattern
// replaces every match from index 0, any other pattern the first match (a sticky one only at index 0).
func (re *Regexp) Replace(s, replacement string) string {
	in := js.ToU16(s)
	var b strings.Builder
	last := 0
	pos := 0
	for pos <= len(in) {
		var caps []int
		if re.sticky {
			caps = re.MatchAt(in, pos)
		} else {
			caps = re.Search(in, pos)
		}
		if caps == nil {
			break
		}
		start, end := caps[0], caps[1]
		matched := js.FromU16(in[start:end])
		captures := make([]*string, re.ncap)
		for g := 1; g <= re.ncap; g++ {
			if caps[2*g] >= 0 {
				v := js.FromU16(in[caps[2*g]:caps[2*g+1]])
				captures[g-1] = &v
			}
		}
		var groups map[string]*string
		if re.HasNamedGroups() {
			groups = map[string]*string{}
			for g, name := range re.names {
				if name != "" {
					groups[name] = captures[g]
				}
			}
		}
		b.WriteString(js.FromU16(in[last:start]))
		b.WriteString(js.ExpandReplacement(replacement, matched, s, start, captures, groups))
		last = end
		if !re.global {
			break
		}
		if end == start {
			pos = re.advance(in, end)
		} else {
			pos = end
		}
	}
	b.WriteString(js.FromU16(in[last:]))
	return b.String()
}
