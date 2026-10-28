// Package jsre implements JavaScript regular expressions as the V8 engine in Node.js 22 runs them: the pattern
// grammar with its web-compatibility extensions, the flags d, g, i, m, s, u, v and y, UTF-16 code-unit matching,
// lookaround, backreferences, named groups and the engine's syntax-error texts.
package jsre

type nodeKind uint8

const (
	nEmpty nodeKind = iota
	nChar
	nAny
	nClass
	nSeq
	nAlt
	nGroup
	nLook
	nBackref
	nNamedBackref
	nRepeat
	nBOL
	nEOL
	nWordB
	nNotWordB
)

type node struct {
	kind     nodeKind
	ch       rune
	cls      *charClass
	subs     []*node
	capIndex int    // nGroup: capture number, 0 for a non-capturing group; nBackref: referenced group
	name     string // nGroup: group name; nNamedBackref: referenced name
	behind   bool   // nLook
	negate   bool   // nLook
	min, max int    // nRepeat; max -1 means unbounded
	greedy   bool
	capFrom  int // nRepeat: first capture inside the repeated atom
	capTo    int // nRepeat: one past the last capture inside
}

type rng struct{ lo, hi rune }

type charClass struct {
	ranges []rng
	negate bool
}

func (c *charClass) add(lo, hi rune) { c.ranges = append(c.ranges, rng{lo, hi}) }

func (c *charClass) contains(r rune) bool {
	for _, x := range c.ranges {
		if r >= x.lo && r <= x.hi {
			return true
		}
	}
	return false
}

var digitRanges = []rng{{'0', '9'}}
var wordRanges = []rng{{'0', '9'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'}}
var spaceRanges = []rng{{0x09, 0x0D}, {0x20, 0x20}, {0xA0, 0xA0}, {0x1680, 0x1680}, {0x2000, 0x200A},
	{0x2028, 0x2029}, {0x202F, 0x202F}, {0x205F, 0x205F}, {0x3000, 0x3000}, {0xFEFF, 0xFEFF}}

// complement returns the ranges not covered by rs (which must be sorted and disjoint) up to maxChar.
func complement(rs []rng, maxChar rune) []rng {
	var out []rng
	next := rune(0)
	for _, r := range rs {
		if r.lo > next {
			out = append(out, rng{next, r.lo - 1})
		}
		next = r.hi + 1
	}
	if next <= maxChar {
		out = append(out, rng{next, maxChar})
	}
	return out
}

func isLineTerminator(c rune) bool {
	return c == '\n' || c == '\r' || c == 0x2028 || c == 0x2029
}

func isWordChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// SyntaxError is a pattern or flags error with V8's message text.
type SyntaxError struct{ Message string }

func (e *SyntaxError) Error() string { return e.Message }
