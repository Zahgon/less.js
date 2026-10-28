package jsre

import (
	"strings"
	"sync"
	"unicode"

	"github.com/less/less.js/internal/js"
)

// Regexp is a compiled JavaScript regular expression.
type Regexp struct {
	source     string
	flags      string
	global     bool
	ignoreCase bool
	multiline  bool
	dotAll     bool
	unicode    bool
	vmode      bool
	sticky     bool
	hasIndices bool
	ncap       int
	names      []string
	prog       matcher
}

const flagOrder = "dgimsuvy"

// ValidFlags reports whether flags is a valid flags string for the RegExp constructor.
func ValidFlags(flags string) bool {
	seen := map[rune]bool{}
	for _, f := range flags {
		if !strings.ContainsRune(flagOrder, f) || seen[f] {
			return false
		}
		seen[f] = true
	}
	return !(seen['u'] && seen['v'])
}

// Compile parses pattern (a JavaScript string) with flags and returns the error the RegExp constructor throws.
func Compile(pattern, flags string) (re *Regexp, err *SyntaxError) {
	if !ValidFlags(flags) {
		return nil, &SyntaxError{Message: "Invalid flags supplied to RegExp constructor '" + flags + "'"}
	}
	re = &Regexp{source: pattern}
	for _, f := range flagOrder {
		if strings.ContainsRune(flags, f) {
			re.flags += string(f)
		}
	}
	re.hasIndices = strings.Contains(flags, "d")
	re.global = strings.Contains(flags, "g")
	re.ignoreCase = strings.Contains(flags, "i")
	re.multiline = strings.Contains(flags, "m")
	re.dotAll = strings.Contains(flags, "s")
	re.vmode = strings.Contains(flags, "v")
	re.unicode = strings.Contains(flags, "u") || re.vmode
	re.sticky = strings.Contains(flags, "y")
	units := js.ToU16(pattern)
	var src []rune
	for i := 0; i < len(units); i++ {
		c := rune(units[i])
		if re.unicode && c >= 0xD800 && c <= 0xDBFF && i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF {
			src = append(src, 0x10000+(c-0xD800)<<10+(rune(units[i+1])-0xDC00))
			i++
			continue
		}
		src = append(src, c)
	}
	p := &parser{src: src, unicode: re.unicode, vmode: re.vmode, ignoreCase: re.ignoreCase}
	var root *node
	func() {
		defer func() {
			if r := recover(); r != nil {
				pe, ok := r.(parseError)
				if !ok {
					panic(r)
				}
				err = &SyntaxError{Message: "Invalid regular expression: /" + pattern + "/" + re.flags + ": " + pe.msg}
			}
		}()
		root = p.parsePattern()
	}()
	if err != nil {
		return nil, err
	}
	re.ncap = p.ncap
	re.names = p.names
	re.prog = (&compiler{re: re}).compile(root, false)
	return re, nil
}

// MustCompile compiles a pattern known to be valid.
func MustCompile(pattern, flags string) *Regexp {
	re, err := Compile(pattern, flags)
	if err != nil {
		panic(err.Message)
	}
	return re
}

// Source returns the pattern text.
func (re *Regexp) Source() string { return re.source }

// Flags returns the flags in canonical order.
func (re *Regexp) Flags() string { return re.flags }

// Global reports the g flag.
func (re *Regexp) Global() bool { return re.global }

// Sticky reports the y flag.
func (re *Regexp) Sticky() bool { return re.sticky }

// FullUnicode reports the u or v flag.
func (re *Regexp) FullUnicode() bool { return re.unicode }

// NumCaptures returns the number of capturing groups.
func (re *Regexp) NumCaptures() int { return re.ncap }

// GroupNames returns the name of each capture group, "" for an unnamed one.
func (re *Regexp) GroupNames() []string { return re.names }

// HasNamedGroups reports whether any group has a name.
func (re *Regexp) HasNamedGroups() bool {
	for _, n := range re.names {
		if n != "" {
			return true
		}
	}
	return false
}

func (re *Regexp) nameIndex(name string) int {
	for i, n := range re.names {
		if n == name {
			return i + 1
		}
	}
	return 0
}

// MatchAt tries a match starting exactly at pos and returns the capture positions (start and end per group,
// -1 when unset), or nil.
func (re *Regexp) MatchAt(in []uint16, pos int) []int {
	m := &machine{in: in, caps: make([]int, 2*(re.ncap+1)), re: re}
	for i := range m.caps {
		m.caps[i] = -1
	}
	end := -1
	if re.prog(m, pos, func(_ *machine, p int) bool { end = p; return true }) {
		m.caps[0], m.caps[1] = pos, end
		return m.caps
	}
	return nil
}

// Search scans for the first match starting at or after pos; sticky patterns try pos only.
func (re *Regexp) Search(in []uint16, pos int) []int {
	for pos <= len(in) {
		if caps := re.MatchAt(in, pos); caps != nil {
			return caps
		}
		if re.sticky {
			return nil
		}
		pos = re.advance(in, pos)
	}
	return nil
}

func (re *Regexp) advance(in []uint16, pos int) int {
	if re.unicode && pos+1 < len(in) && in[pos] >= 0xD800 && in[pos] <= 0xDBFF && in[pos+1] >= 0xDC00 && in[pos+1] <= 0xDFFF {
		return pos + 2
	}
	return pos + 1
}

// AdvanceIndex implements AdvanceStringIndex for this pattern's unicode flag.
func (re *Regexp) AdvanceIndex(in []uint16, pos int) int { return re.advance(in, pos) }

// Test reports whether s contains a match, ignoring the global and sticky state.
func (re *Regexp) Test(s string) bool {
	in := js.ToU16(s)
	if re.sticky {
		return re.MatchAt(in, 0) != nil
	}
	return re.Search(in, 0) != nil
}

var canonOnce sync.Once
var canonTable []uint16
var canonInverse map[uint16][]uint16

func buildCanon() {
	canonTable = make([]uint16, 0x10000)
	canonInverse = map[uint16][]uint16{}
	for c := 0; c < 0x10000; c++ {
		r := rune(c)
		u := r
		if c < 0xD800 || c > 0xDFFF {
			up := js.ToUpperCase(string(r))
			ur := []rune(up)
			if len(ur) == 1 && ur[0] <= 0xFFFF && !(c >= 128 && ur[0] < 128) {
				u = ur[0]
			}
		}
		canonTable[c] = uint16(u)
		canonInverse[uint16(u)] = append(canonInverse[uint16(u)], uint16(c))
	}
}

func (re *Regexp) canon(c rune) rune {
	if re.unicode {
		return foldRep(c)
	}
	if c > 0xFFFF || c < 0 {
		return c
	}
	if c < 128 {
		if c >= 'a' && c <= 'z' {
			return c - 32
		}
		return c
	}
	canonOnce.Do(buildCanon)
	return rune(canonTable[c])
}

func foldRep(c rune) rune {
	min := c
	for f := unicode.SimpleFold(c); f != c; f = unicode.SimpleFold(f) {
		if f < min {
			min = f
		}
	}
	return min
}

// classFold reports whether a class contains a character equivalent to ch under case-insensitive matching.
func (re *Regexp) classFold(cls *charClass, ch rune) bool {
	if cls.contains(ch) {
		return true
	}
	if re.unicode {
		for f := unicode.SimpleFold(ch); f != ch; f = unicode.SimpleFold(f) {
			if cls.contains(f) {
				return true
			}
		}
		return false
	}
	if ch > 0xFFFF || ch < 0 {
		return false
	}
	canonOnce.Do(buildCanon)
	for _, e := range canonInverse[canonTable[ch]] {
		if cls.contains(rune(e)) {
			return true
		}
	}
	return false
}

func (re *Regexp) isWord(c rune) bool {
	if isWordChar(c) {
		return true
	}
	return re.unicode && re.ignoreCase && (c == 0x017F || c == 0x212A)
}
