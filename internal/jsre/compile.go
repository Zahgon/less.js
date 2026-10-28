package jsre

type machine struct {
	in   []uint16
	caps []int
	re   *Regexp
}

type cont func(m *machine, pos int) bool
type matcher func(m *machine, pos int, k cont) bool

// single matches one character at pos in the given direction and returns the new position, or -1.
type single func(m *machine, pos int) int

type compiler struct {
	re *Regexp
}

func (re *Regexp) readFwd(in []uint16, pos int) (rune, int) {
	if pos >= len(in) {
		return -1, 0
	}
	c := rune(in[pos])
	if re.unicode && c >= 0xD800 && c <= 0xDBFF && pos+1 < len(in) && in[pos+1] >= 0xDC00 && in[pos+1] <= 0xDFFF {
		return 0x10000 + (c-0xD800)<<10 + (rune(in[pos+1]) - 0xDC00), 2
	}
	return c, 1
}

func (re *Regexp) readBack(in []uint16, pos int) (rune, int) {
	if pos <= 0 {
		return -1, 0
	}
	c := rune(in[pos-1])
	if re.unicode && c >= 0xDC00 && c <= 0xDFFF && pos-2 >= 0 && in[pos-2] >= 0xD800 && in[pos-2] <= 0xDBFF {
		return 0x10000 + (rune(in[pos-2])-0xD800)<<10 + (c - 0xDC00), 2
	}
	return c, 1
}

func (c *compiler) singleOf(n *node, back bool) single {
	re := c.re
	var test func(ch rune) bool
	switch n.kind {
	case nChar:
		want := n.ch
		if re.ignoreCase {
			cw := re.canon(want)
			test = func(ch rune) bool { return ch == want || re.canon(ch) == cw }
		} else {
			test = func(ch rune) bool { return ch == want }
		}
	case nAny:
		if re.dotAll {
			test = func(rune) bool { return true }
		} else {
			test = func(ch rune) bool { return !isLineTerminator(ch) }
		}
	case nClass:
		cls := n.cls
		cls.ranges = normalize(cls.ranges)
		if re.ignoreCase {
			test = func(ch rune) bool { return re.classFold(cls, ch) != cls.negate }
		} else {
			test = func(ch rune) bool { return cls.contains(ch) != cls.negate }
		}
	default:
		return nil
	}
	if back {
		return func(m *machine, pos int) int {
			ch, w := re.readBack(m.in, pos)
			if w == 0 || !test(ch) {
				return -1
			}
			return pos - w
		}
	}
	return func(m *machine, pos int) int {
		ch, w := re.readFwd(m.in, pos)
		if w == 0 || !test(ch) {
			return -1
		}
		return pos + w
	}
}

func (c *compiler) compile(n *node, back bool) matcher {
	switch n.kind {
	case nEmpty:
		return func(m *machine, pos int, k cont) bool { return k(m, pos) }
	case nChar, nAny, nClass:
		s := c.singleOf(n, back)
		return func(m *machine, pos int, k cont) bool {
			p := s(m, pos)
			return p >= 0 && k(m, p)
		}
	case nSeq:
		ms := make([]matcher, len(n.subs))
		for i, sub := range n.subs {
			ms[i] = c.compile(sub, back)
		}
		if back {
			for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
				ms[i], ms[j] = ms[j], ms[i]
			}
		}
		return seqMatcher(ms)
	case nAlt:
		ms := make([]matcher, len(n.subs))
		for i, sub := range n.subs {
			ms[i] = c.compile(sub, back)
		}
		return func(m *machine, pos int, k cont) bool {
			for _, mm := range ms {
				if mm(m, pos, k) {
					return true
				}
			}
			return false
		}
	case nGroup:
		body := c.compile(n.subs[0], back)
		if n.capIndex == 0 {
			return body
		}
		i := n.capIndex
		return func(m *machine, pos int, k cont) bool {
			return body(m, pos, func(m *machine, p int) bool {
				s0, e0 := m.caps[2*i], m.caps[2*i+1]
				if back {
					m.caps[2*i], m.caps[2*i+1] = p, pos
				} else {
					m.caps[2*i], m.caps[2*i+1] = pos, p
				}
				if k(m, p) {
					return true
				}
				m.caps[2*i], m.caps[2*i+1] = s0, e0
				return false
			})
		}
	case nLook:
		return c.compileLook(n)
	case nBackref, nNamedBackref:
		return c.compileBackref(n, back)
	case nRepeat:
		return c.compileRepeat(n, back)
	case nBOL:
		re := c.re
		return func(m *machine, pos int, k cont) bool {
			if pos == 0 || (re.multiline && isLineTerminator(rune(m.in[pos-1]))) {
				return k(m, pos)
			}
			return false
		}
	case nEOL:
		re := c.re
		return func(m *machine, pos int, k cont) bool {
			if pos == len(m.in) || (re.multiline && isLineTerminator(rune(m.in[pos]))) {
				return k(m, pos)
			}
			return false
		}
	case nWordB, nNotWordB:
		re := c.re
		want := n.kind == nWordB
		return func(m *machine, pos int, k cont) bool {
			a := pos > 0 && re.isWord(rune(m.in[pos-1]))
			b := pos < len(m.in) && re.isWord(rune(m.in[pos]))
			if (a != b) == want {
				return k(m, pos)
			}
			return false
		}
	}
	panic("jsre: unknown node")
}

func seqMatcher(ms []matcher) matcher {
	switch len(ms) {
	case 0:
		return func(m *machine, pos int, k cont) bool { return k(m, pos) }
	case 1:
		return ms[0]
	}
	first := ms[0]
	rest := seqMatcher(ms[1:])
	return func(m *machine, pos int, k cont) bool {
		return first(m, pos, func(m *machine, p int) bool { return rest(m, p, k) })
	}
}

func (c *compiler) compileLook(n *node) matcher {
	body := c.compile(n.subs[0], n.behind)
	negate := n.negate
	return func(m *machine, pos int, k cont) bool {
		saved := append([]int(nil), m.caps...)
		matched := body(m, pos, func(*machine, int) bool { return true })
		if negate {
			copy(m.caps, saved)
			if matched {
				return false
			}
			return k(m, pos)
		}
		if !matched {
			copy(m.caps, saved)
			return false
		}
		if k(m, pos) {
			return true
		}
		copy(m.caps, saved)
		return false
	}
}

func (c *compiler) compileBackref(n *node, back bool) matcher {
	re := c.re
	return func(m *machine, pos int, k cont) bool {
		idx := n.capIndex
		if n.kind == nNamedBackref {
			idx = re.nameIndex(n.name)
		}
		s, e := m.caps[2*idx], m.caps[2*idx+1]
		if s < 0 || e < 0 {
			return k(m, pos)
		}
		ln := e - s
		var from int
		if back {
			from = pos - ln
			if from < 0 {
				return false
			}
		} else {
			from = pos
			if from+ln > len(m.in) {
				return false
			}
		}
		for i := 0; i < ln; i++ {
			a, b := rune(m.in[s+i]), rune(m.in[from+i])
			if a != b && !(re.ignoreCase && re.canon(a) == re.canon(b)) {
				return false
			}
		}
		if back {
			return k(m, from)
		}
		return k(m, from+ln)
	}
}
