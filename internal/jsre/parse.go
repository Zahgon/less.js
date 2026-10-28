package jsre

import (
	"math"
	"unicode"
)

const endMarker = rune(-1)
const infinity = math.MaxInt32

type parseError struct{ msg string }

type parser struct {
	src        []rune
	pos        int
	unicode    bool
	vmode      bool
	ncap       int
	totalCaps  int
	hasNamed   bool
	names      []string
	namedRefs  []string
	inClass    bool
	ignoreCase bool
	openGroups []int
}

func (p *parser) cur() rune {
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return endMarker
}

func (p *parser) peek(n int) rune {
	if p.pos+n < len(p.src) {
		return p.src[p.pos+n]
	}
	return endMarker
}

func (p *parser) fail(msg string) { panic(parseError{msg}) }

// scanCaptures counts capturing groups and notes named ones, the way V8 scans ahead.
func (p *parser) scanCaptures() {
	s := p.src
	for i := 0; i < len(s); {
		c := s[i]
		i++
		switch c {
		case '\\':
			i++
		case '[':
			for i < len(s) {
				d := s[i]
				i++
				if d == '\\' {
					i++
				} else if d == ']' {
					break
				}
			}
		case '(':
			if i < len(s) && s[i] == '?' {
				i++
				if i >= len(s) || s[i] != '<' {
					continue
				}
				i++
				if i < len(s) && (s[i] == '=' || s[i] == '!') {
					continue
				}
				p.hasNamed = true
			}
			p.totalCaps++
		}
	}
}

func (p *parser) parsePattern() *node {
	p.scanCaptures()
	n := p.parseDisjunction()
	if p.cur() == ')' {
		p.fail("Unmatched ')'")
	}
	for _, ref := range p.namedRefs {
		found := false
		for _, nm := range p.names {
			if nm == ref {
				found = true
			}
		}
		if !found {
			p.fail("Invalid named capture referenced")
		}
	}
	return n
}

func (p *parser) parseDisjunction() *node {
	alts := []*node{p.parseAlternative()}
	for p.cur() == '|' {
		p.pos++
		alts = append(alts, p.parseAlternative())
	}
	if len(alts) == 1 {
		return alts[0]
	}
	return &node{kind: nAlt, subs: alts}
}

func (p *parser) parseAlternative() *node {
	var terms []*node
	for {
		c := p.cur()
		if c == endMarker || c == '|' || c == ')' {
			break
		}
		capBefore := p.ncap
		atom, quantifiable, lookbehind, lookahead := p.parseAtom()
		if atom == nil {
			continue
		}
		if !quantifiable {
			terms = append(terms, atom)
			continue
		}
		min, max, ok := p.parseQuantifier()
		if !ok {
			terms = append(terms, atom)
			continue
		}
		greedy := true
		if p.cur() == '?' {
			greedy = false
			p.pos++
		}
		if lookbehind || (lookahead && p.unicode) {
			p.fail("Invalid quantifier")
		}
		if atom.kind == nEmpty {
			terms = append(terms, atom)
			continue
		}
		terms = append(terms, &node{kind: nRepeat, subs: []*node{atom}, min: min, max: max, greedy: greedy,
			capFrom: capBefore + 1, capTo: p.ncap + 1})
	}
	if len(terms) == 1 {
		return terms[0]
	}
	return &node{kind: nSeq, subs: terms}
}

// parseQuantifier reads a quantifier prefix; ok is false when none follows.
func (p *parser) parseQuantifier() (int, int, bool) {
	switch p.cur() {
	case '*':
		p.pos++
		return 0, -1, true
	case '+':
		p.pos++
		return 1, -1, true
	case '?':
		p.pos++
		return 0, 1, true
	case '{':
		if min, max, ok := p.parseInterval(); ok {
			if max != -1 && max < min {
				p.fail("numbers out of order in {} quantifier")
			}
			return min, max, true
		}
		if p.unicode {
			p.fail("Incomplete quantifier")
		}
	}
	return 0, 0, false
}

// parseInterval parses {n}, {n,} or {n,m} at the current position, leaving the position unchanged on failure.
func (p *parser) parseInterval() (int, int, bool) {
	start := p.pos
	p.pos++
	readNum := func() (int, bool) {
		if p.cur() < '0' || p.cur() > '9' {
			return 0, false
		}
		v := 0
		for p.cur() >= '0' && p.cur() <= '9' {
			d := int(p.cur() - '0')
			if v < (infinity-d)/10 {
				v = v*10 + d
			} else {
				v = infinity
			}
			p.pos++
		}
		return v, true
	}
	min, ok := readNum()
	if !ok {
		p.pos = start
		return 0, 0, false
	}
	max := min
	if p.cur() == ',' {
		p.pos++
		if p.cur() == '}' {
			max = -1
		} else {
			m, ok2 := readNum()
			if !ok2 {
				p.pos = start
				return 0, 0, false
			}
			max = m
		}
	}
	if p.cur() != '}' {
		p.pos = start
		return 0, 0, false
	}
	p.pos++
	if max == infinity {
		max = -1
	}
	return min, max, true
}

// parseAtom parses one term. It reports whether a quantifier may follow and whether the term is a lookbehind
// or lookahead, which have their own quantifier rules.
func (p *parser) parseAtom() (*node, bool, bool, bool) {
	c := p.cur()
	switch c {
	case '^':
		p.pos++
		return &node{kind: nBOL}, false, false, false
	case '$':
		p.pos++
		return &node{kind: nEOL}, false, false, false
	case '.':
		p.pos++
		return &node{kind: nAny}, true, false, false
	case '(':
		return p.parseGroup()
	case '[':
		return &node{kind: nClass, cls: p.parseClass()}, true, false, false
	case '*', '+', '?':
		p.fail("Nothing to repeat")
	case '{':
		if _, _, ok := p.parseInterval(); ok {
			p.fail("Nothing to repeat")
		}
		if p.unicode {
			p.fail("Lone quantifier brackets")
		}
	case '}', ']':
		if p.unicode {
			p.fail("Lone quantifier brackets")
		}
	case '\\':
		return p.parseAtomEscape()
	}
	p.pos++
	return &node{kind: nChar, ch: c}, true, false, false
}

func (p *parser) parseGroup() (*node, bool, bool, bool) {
	p.pos++
	kind := nGroup
	capIndex := 0
	name := ""
	behind, negate, look, capture := false, false, false, false
	if p.cur() == '?' {
		switch p.peek(1) {
		case ':':
			p.pos += 2
		case '=':
			p.pos += 2
			look = true
		case '!':
			p.pos += 2
			look, negate = true, true
		case '<':
			p.pos++
			switch p.peek(1) {
			case '=':
				p.pos += 2
				look, behind = true, true
			case '!':
				p.pos += 2
				look, behind, negate = true, true, true
			default:
				p.pos++
				capture = true
				p.hasNamed = true
				p.ncap++
				capIndex = p.ncap
				name = p.parseGroupName()
				for _, nm := range p.names {
					if nm == name {
						p.fail("Duplicate capture group name")
					}
				}
			}
		default:
			p.fail("Invalid group")
		}
	} else {
		capture = true
		p.ncap++
		capIndex = p.ncap
	}
	if capture {
		for len(p.names) < capIndex {
			p.names = append(p.names, "")
		}
		p.names[capIndex-1] = name
		p.openGroups = append(p.openGroups, capIndex)
	}
	body := p.parseDisjunction()
	if p.cur() != ')' {
		p.fail("Unterminated group")
	}
	p.pos++
	if capture {
		p.openGroups = p.openGroups[:len(p.openGroups)-1]
	}
	if look {
		return &node{kind: nLook, subs: []*node{body}, behind: behind, negate: negate}, true, behind, !behind
	}
	return &node{kind: kind, subs: []*node{body}, capIndex: capIndex, name: name}, true, false, false
}

func isIDStart(c rune) bool {
	return c == '$' || c == '_' || unicode.IsLetter(c) || unicode.Is(unicode.Nl, c) ||
		unicode.Is(unicode.Other_ID_Start, c)
}

func isIDContinue(c rune) bool {
	return isIDStart(c) || unicode.IsDigit(c) || unicode.In(c, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc) ||
		unicode.Is(unicode.Other_ID_Continue, c) || c == 0x200C || c == 0x200D
}

// parseGroupName reads a group name after "<" up to and including ">".
func (p *parser) parseGroupName() string {
	var out []rune
	for first := true; ; first = false {
		c := p.cur()
		if c == '\\' && p.peek(1) == 'u' {
			p.pos += 2
			v, ok := p.parseUnicodeEscapeValue(true)
			if !ok {
				p.fail("Invalid capture group name")
			}
			c = v
		} else if c == '>' {
			if first {
				p.fail("Invalid capture group name")
			}
			p.pos++
			return string(out)
		} else if c == endMarker {
			p.fail("Invalid capture group name")
		} else {
			if c >= 0xD800 && c <= 0xDBFF && p.peek(1) >= 0xDC00 && p.peek(1) <= 0xDFFF {
				c = 0x10000 + (c-0xD800)<<10 + (p.peek(1) - 0xDC00)
				p.pos++
			}
			p.pos++
		}
		if (first && !isIDStart(c)) || (!first && !isIDContinue(c)) {
			p.fail("Invalid capture group name")
		}
		out = append(out, c)
	}
}
