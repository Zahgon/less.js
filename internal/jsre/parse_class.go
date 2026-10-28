package jsre

import (
	"sort"
	"unicode"
)

var gcLongNames = map[string]string{
	"Letter": "L", "Cased_Letter": "LC", "Uppercase_Letter": "Lu", "Lowercase_Letter": "Ll",
	"Titlecase_Letter": "Lt", "Modifier_Letter": "Lm", "Other_Letter": "Lo", "Mark": "M", "Combining_Mark": "M",
	"Nonspacing_Mark": "Mn", "Spacing_Mark": "Mc", "Enclosing_Mark": "Me", "Number": "N", "Decimal_Number": "Nd",
	"digit": "Nd", "Letter_Number": "Nl", "Other_Number": "No", "Punctuation": "P", "punct": "P",
	"Connector_Punctuation": "Pc", "Dash_Punctuation": "Pd", "Open_Punctuation": "Ps", "Close_Punctuation": "Pe",
	"Initial_Punctuation": "Pi", "Final_Punctuation": "Pf", "Other_Punctuation": "Po", "Symbol": "S",
	"Math_Symbol": "Sm", "Currency_Symbol": "Sc", "Modifier_Symbol": "Sk", "Other_Symbol": "So", "Separator": "Z",
	"Space_Separator": "Zs", "Line_Separator": "Zl", "Paragraph_Separator": "Zp", "Other": "C", "Control": "Cc",
	"cntrl": "Cc", "Format": "Cf", "Surrogate": "Cs", "Private_Use": "Co", "Unassigned": "Cn",
}

func tableRanges(t *unicode.RangeTable) []rng {
	var out []rng
	for _, r := range t.R16 {
		for c := rune(r.Lo); c <= rune(r.Hi); c += rune(r.Stride) {
			if r.Stride == 1 {
				out = append(out, rng{rune(r.Lo), rune(r.Hi)})
				break
			}
			out = append(out, rng{c, c})
		}
	}
	for _, r := range t.R32 {
		for c := rune(r.Lo); c <= rune(r.Hi); c += rune(r.Stride) {
			if r.Stride == 1 {
				out = append(out, rng{rune(r.Lo), rune(r.Hi)})
				break
			}
			out = append(out, rng{c, c})
		}
	}
	return out
}

func normalize(rs []rng) []rng {
	if len(rs) == 0 {
		return nil
	}
	cp := append([]rng(nil), rs...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].lo < cp[j].lo })
	out := []rng{cp[0]}
	for _, r := range cp[1:] {
		last := &out[len(out)-1]
		if r.lo <= last.hi+1 {
			if r.hi > last.hi {
				last.hi = r.hi
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

func categoryRanges(gc string) ([]rng, bool) {
	switch gc {
	case "LC":
		return normalize(append(append(tableRanges(unicode.Lu), tableRanges(unicode.Ll)...),
			tableRanges(unicode.Lt)...)), true
	case "Cn":
		var all []rng
		for _, t := range unicode.Categories {
			all = append(all, tableRanges(t)...)
		}
		return complement(normalize(all), 0x10FFFF), true
	case "C":
		var all []rng
		for _, t := range unicode.Categories {
			all = append(all, tableRanges(t)...)
		}
		cn := complement(normalize(all), 0x10FFFF)
		return normalize(append(tableRanges(unicode.C), cn...)), true
	}
	if t, ok := unicode.Categories[gc]; ok {
		return normalize(tableRanges(t)), true
	}
	return nil, false
}

func binaryProperty(name string) ([]rng, bool) {
	switch name {
	case "Any":
		return []rng{{0, 0x10FFFF}}, true
	case "ASCII":
		return []rng{{0, 0x7F}}, true
	case "Assigned":
		cn, _ := categoryRanges("Cn")
		return complement(cn, 0x10FFFF), true
	case "Alphabetic", "Alpha":
		return normalize(append(append(tableRanges(unicode.L), tableRanges(unicode.Nl)...),
			tableRanges(unicode.Other_Alphabetic)...)), true
	case "Lowercase", "Lower":
		return normalize(append(tableRanges(unicode.Ll), tableRanges(unicode.Other_Lowercase)...)), true
	case "Uppercase", "Upper":
		return normalize(append(tableRanges(unicode.Lu), tableRanges(unicode.Other_Uppercase)...)), true
	case "White_Space", "space":
		return normalize(tableRanges(unicode.White_Space)), true
	}
	if t, ok := unicode.Properties[name]; ok && name[0] != 'O' {
		return normalize(tableRanges(t)), true
	}
	return nil, false
}

// parseProperty parses \p{...} or \P{...} after the p or P.
func (p *parser) parseProperty(negate bool) []rng {
	bad := func() {
		if p.inClass {
			p.fail("Invalid property name in character class")
		}
		p.fail("Invalid property name")
	}
	p.pos++
	if p.cur() != '{' {
		bad()
	}
	p.pos++
	var name, value []rune
	sawEq := false
	for p.cur() != '}' {
		c := p.cur()
		if c == endMarker {
			bad()
		}
		p.pos++
		if c == '=' && !sawEq {
			sawEq = true
			continue
		}
		if !(c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			bad()
		}
		if sawEq {
			value = append(value, c)
		} else {
			name = append(name, c)
		}
	}
	var rs []rng
	ok := false
	if sawEq {
		switch string(name) {
		case "General_Category", "gc":
			v := string(value)
			if long, isLong := gcLongNames[v]; isLong {
				v = long
			}
			rs, ok = categoryRanges(v)
		case "Script", "sc", "Script_Extensions", "scx":
			if t, has := unicode.Scripts[string(value)]; has {
				rs, ok = normalize(tableRanges(t)), true
			}
		}
	} else {
		v := string(name)
		if long, isLong := gcLongNames[v]; isLong {
			v = long
		}
		if rs, ok = categoryRanges(v); !ok {
			rs, ok = binaryProperty(string(name))
		}
	}
	if !ok {
		bad()
	}
	if negate {
		rs = complement(rs, 0x10FFFF)
	}
	return rs
}

// parseClassSetContents parses the body of a class under the v flag: unions, ranges, nested classes and the
// && and -- operators.
func (p *parser) parseClassSetContents(cls *charClass) {
	cls.ranges = p.parseClassSetUnion()
	if p.cur() != ']' {
		p.fail("Unterminated character class")
	}
	p.pos++
}

func (p *parser) parseClassSetOperand() ([]rng, rune, bool) {
	c := p.cur()
	switch c {
	case '[':
		inner := &charClass{}
		p.pos++
		if p.cur() == '^' {
			inner.negate = true
			p.pos++
		}
		inner.ranges = p.parseClassSetUnion()
		if p.cur() != ']' {
			p.fail("Unterminated character class")
		}
		p.pos++
		rs := normalize(inner.ranges)
		if inner.negate {
			rs = complement(rs, 0x10FFFF)
		}
		return rs, 0, true
	case endMarker:
		p.fail("Unterminated character class")
	case '(', ')', '{', '}', '/', '|':
		p.fail("Invalid character in character class")
	case '-':
		p.fail("Invalid set operation in character class")
	}
	if c == '\\' {
		ch, rs, isClass := p.parseClassAtom()
		if isClass {
			return normalize(rs), 0, true
		}
		return nil, ch, false
	}
	p.pos++
	return nil, c, false
}

func (p *parser) parseClassSetUnion() []rng {
	var acc []rng
	first := true
	for p.cur() != ']' && p.cur() != endMarker {
		if (p.cur() == '&' && p.peek(1) == '&') || (p.cur() == '-' && p.peek(1) == '-') {
			if first {
				p.fail("Invalid set operation in character class")
			}
			op := p.cur()
			p.pos += 2
			rs, ch, isSet := p.parseClassSetOperand()
			if !isSet {
				rs = []rng{{ch, ch}}
			}
			acc = normalize(acc)
			if op == '&' {
				acc = intersect(acc, rs)
			} else {
				acc = intersect(acc, complement(rs, 0x10FFFF))
			}
			continue
		}
		rs, ch, isSet := p.parseClassSetOperand()
		first = false
		if isSet {
			acc = append(acc, rs...)
			continue
		}
		if p.cur() == '-' && p.peek(1) != '-' {
			p.pos++
			_, ch2, isSet2 := p.parseClassSetOperand()
			if isSet2 {
				p.fail("Invalid character class")
			}
			if ch > ch2 {
				p.fail("Range out of order in character class")
			}
			acc = append(acc, rng{ch, ch2})
			continue
		}
		acc = append(acc, rng{ch, ch})
	}
	return acc
}

func intersect(a, b []rng) []rng {
	var out []rng
	for _, x := range a {
		for _, y := range normalize(b) {
			lo, hi := x.lo, x.hi
			if y.lo > lo {
				lo = y.lo
			}
			if y.hi < hi {
				hi = y.hi
			}
			if lo <= hi {
				out = append(out, rng{lo, hi})
			}
		}
	}
	return out
}
