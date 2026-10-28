package jsre

func hexVal(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// parseHex reads exactly n hex digits, leaving the position unchanged on failure.
func (p *parser) parseHex(n int) (rune, bool) {
	start := p.pos
	v := rune(0)
	for i := 0; i < n; i++ {
		d := hexVal(p.cur())
		if d < 0 {
			p.pos = start
			return 0, false
		}
		v = v*16 + rune(d)
		p.pos++
	}
	return v, true
}

// parseUnicodeEscapeValue parses the part after "\u". In Unicode mode it accepts \u{...} and joins an escaped
// surrogate pair.
func (p *parser) parseUnicodeEscapeValue(unicodeMode bool) (rune, bool) {
	start := p.pos
	if p.cur() == '{' && unicodeMode {
		p.pos++
		v := rune(0)
		digits := 0
		for hexVal(p.cur()) >= 0 {
			v = v*16 + rune(hexVal(p.cur()))
			if v > 0x10FFFF {
				p.pos = start
				return 0, false
			}
			digits++
			p.pos++
		}
		if digits == 0 || p.cur() != '}' {
			p.pos = start
			return 0, false
		}
		p.pos++
		return v, true
	}
	v, ok := p.parseHex(4)
	if !ok {
		return 0, false
	}
	if unicodeMode && v >= 0xD800 && v <= 0xDBFF && p.cur() == '\\' && p.peek(1) == 'u' {
		save := p.pos
		p.pos += 2
		if lo, ok2 := p.parseHex(4); ok2 && lo >= 0xDC00 && lo <= 0xDFFF {
			return 0x10000 + (v-0xD800)<<10 + (lo - 0xDC00), true
		}
		p.pos = save
	}
	return v, true
}

func isSyntaxChar(c rune) bool {
	switch c {
	case '^', '$', '\\', '.', '*', '+', '?', '(', ')', '[', ']', '{', '}', '|', '/':
		return true
	}
	return false
}

// parseCharacterEscape parses a CharacterEscape; the position is on the character after the backslash.
func (p *parser) parseCharacterEscape(inClass bool) rune {
	c := p.cur()
	switch c {
	case 'f':
		p.pos++
		return '\f'
	case 'n':
		p.pos++
		return '\n'
	case 'r':
		p.pos++
		return '\r'
	case 't':
		p.pos++
		return '\t'
	case 'v':
		p.pos++
		return '\v'
	case 'c':
		ctl := p.peek(1)
		letter := ctl &^ ('A' ^ 'a')
		if ctl != endMarker && letter >= 'A' && letter <= 'Z' {
			p.pos += 2
			return ctl & 0x1F
		}
		if p.unicode {
			p.fail("Invalid Unicode escape")
		}
		if inClass && ((ctl >= '0' && ctl <= '9') || ctl == '_') {
			p.pos += 2
			return ctl & 0x1F
		}
		return '\\'
	case '0':
		if p.peek(1) < '0' || p.peek(1) > '9' {
			p.pos++
			return 0
		}
		if p.unicode {
			p.fail("Invalid decimal escape")
		}
		return p.parseOctal()
	case '1', '2', '3', '4', '5', '6', '7':
		if p.unicode {
			p.fail("Invalid decimal escape")
		}
		return p.parseOctal()
	case 'x':
		p.pos++
		if v, ok := p.parseHex(2); ok {
			return v
		}
		if p.unicode {
			p.fail("Invalid escape")
		}
		return 'x'
	case 'u':
		p.pos++
		if v, ok := p.parseUnicodeEscapeValue(p.unicode); ok {
			return v
		}
		if p.unicode {
			p.fail("Invalid Unicode escape")
		}
		return 'u'
	}
	if !p.unicode {
		if c == 'k' && p.hasNamed {
			p.fail("Invalid escape")
		}
		p.pos++
		return c
	}
	if isSyntaxChar(c) || (inClass && c == '-') {
		p.pos++
		return c
	}
	p.fail("Invalid escape")
	return 0
}

func (p *parser) parseOctal() rune {
	v := p.cur() - '0'
	p.pos++
	if p.cur() >= '0' && p.cur() <= '7' {
		v = v*8 + p.cur() - '0'
		p.pos++
		if v < 32 && p.cur() >= '0' && p.cur() <= '7' {
			v = v*8 + p.cur() - '0'
			p.pos++
		}
	}
	return v
}

// classEscape returns the ranges of \d, \D, \s, \S, \w, \W and, in Unicode mode, \p and \P.
func (p *parser) classEscape(c rune) ([]rng, bool) {
	maxChar := rune(0xFFFF)
	if p.unicode {
		maxChar = 0x10FFFF
	}
	switch c {
	case 'd':
		return digitRanges, true
	case 'D':
		return complement(digitRanges, maxChar), true
	case 's':
		return spaceRanges, true
	case 'S':
		return complement(spaceRanges, maxChar), true
	case 'w':
		return wordRanges, true
	case 'W':
		if p.unicode && p.ignoreCase {
			return complement(normalize(append([]rng{{0x017F, 0x017F}, {0x212A, 0x212A}}, wordRanges...)), maxChar), true
		}
		return complement(wordRanges, maxChar), true
	case 'p', 'P':
		if p.unicode {
			return p.parseProperty(c == 'P'), true
		}
	}
	return nil, false
}

func (p *parser) parseAtomEscape() (*node, bool, bool, bool) {
	next := p.peek(1)
	switch {
	case next == endMarker:
		p.fail("\\ at end of pattern")
	case next == 'b':
		p.pos += 2
		return &node{kind: nWordB}, false, false, false
	case next == 'B':
		p.pos += 2
		return &node{kind: nNotWordB}, false, false, false
	case next >= '1' && next <= '9':
		start := p.pos
		p.pos++
		v := 0
		for p.cur() >= '0' && p.cur() <= '9' {
			v = v*10 + int(p.cur()-'0')
			if v > 1<<16 {
				break
			}
			p.pos++
		}
		if v <= p.totalCaps && v <= 1<<16 {
			for _, g := range p.openGroups {
				if g == v {
					return &node{kind: nEmpty}, true, false, false
				}
			}
			return &node{kind: nBackref, capIndex: v}, true, false, false
		}
		p.pos = start
		if p.unicode {
			p.fail("Invalid escape")
		}
		if next == '8' || next == '9' {
			p.pos += 2
			return &node{kind: nChar, ch: next}, true, false, false
		}
		p.pos++
		return &node{kind: nChar, ch: p.parseOctal()}, true, false, false
	case next == 'k' && (p.unicode || p.hasNamed):
		p.pos += 2
		if p.cur() != '<' {
			p.fail("Invalid named reference")
		}
		p.pos++
		name := p.parseGroupName()
		p.namedRefs = append(p.namedRefs, name)
		return &node{kind: nNamedBackref, name: name}, true, false, false
	}
	p.pos++
	if rs, ok := p.classEscape(next); ok {
		p.pos++
		cls := &charClass{ranges: rs}
		return &node{kind: nClass, cls: cls}, true, false, false
	}
	c := p.parseCharacterEscape(false)
	return &node{kind: nChar, ch: c}, true, false, false
}

// parseClassAtom parses one class atom, returning either a character or the ranges of a class escape.
func (p *parser) parseClassAtom() (rune, []rng, bool) {
	c := p.cur()
	if c != '\\' {
		p.pos++
		return c, nil, false
	}
	next := p.peek(1)
	switch next {
	case 'b':
		p.pos += 2
		return '\b', nil, false
	case '-':
		if p.unicode {
			p.pos += 2
			return '-', nil, false
		}
	case endMarker:
		p.fail("\\ at end of pattern")
	}
	p.pos++
	if rs, ok := p.classEscape(next); ok {
		p.pos++
		return 0, rs, true
	}
	return p.parseCharacterEscape(true), nil, false
}

func (p *parser) parseClass() *charClass {
	p.inClass = true
	defer func() { p.inClass = false }()
	p.pos++
	cls := &charClass{}
	if p.cur() == '^' {
		cls.negate = true
		p.pos++
	}
	if p.vmode {
		p.parseClassSetContents(cls)
		return cls
	}
	for p.cur() != endMarker && p.cur() != ']' {
		c1, r1, isClass1 := p.parseClassAtom()
		if p.cur() == '-' {
			p.pos++
			if p.cur() == endMarker {
				break
			}
			if p.cur() == ']' {
				if isClass1 {
					cls.ranges = append(cls.ranges, r1...)
				} else {
					cls.add(c1, c1)
				}
				cls.add('-', '-')
				break
			}
			c2, r2, isClass2 := p.parseClassAtom()
			if isClass1 || isClass2 {
				if p.unicode {
					p.fail("Invalid character class")
				}
				if isClass1 {
					cls.ranges = append(cls.ranges, r1...)
				} else {
					cls.add(c1, c1)
				}
				cls.add('-', '-')
				if isClass2 {
					cls.ranges = append(cls.ranges, r2...)
				} else {
					cls.add(c2, c2)
				}
				continue
			}
			if c1 > c2 {
				p.fail("Range out of order in character class")
			}
			cls.add(c1, c2)
			continue
		}
		if isClass1 {
			cls.ranges = append(cls.ranges, r1...)
		} else {
			cls.add(c1, c1)
		}
	}
	if p.cur() == endMarker {
		p.fail("Unterminated character class")
	}
	p.pos++
	return cls
}
