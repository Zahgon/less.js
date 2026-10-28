package jsre

func (c *compiler) compileRepeat(n *node, back bool) matcher {
	min, max, greedy := n.min, n.max, n.greedy
	child := n.subs[0]
	if s := c.singleOf(child, back); s != nil {
		return singleRepeat(s, min, max, greedy)
	}
	body := c.compile(child, back)
	from, to := n.capFrom, n.capTo
	var rep func(m *machine, pos, count int, k cont) bool
	iter := func(m *machine, pos, count int, k cont) bool {
		saved := make([]int, 0, 2*(to-from))
		for i := from; i < to; i++ {
			saved = append(saved, m.caps[2*i], m.caps[2*i+1])
			m.caps[2*i], m.caps[2*i+1] = -1, -1
		}
		ok := body(m, pos, func(m *machine, p int) bool {
			if count >= min && p == pos {
				return false
			}
			return rep(m, p, count+1, k)
		})
		if !ok {
			for j, i := 0, from; i < to; i, j = i+1, j+2 {
				m.caps[2*i], m.caps[2*i+1] = saved[j], saved[j+1]
			}
		}
		return ok
	}
	rep = func(m *machine, pos, count int, k cont) bool {
		if max != -1 && count >= max {
			return k(m, pos)
		}
		if count < min {
			return iter(m, pos, count, k)
		}
		if greedy {
			if iter(m, pos, count, k) {
				return true
			}
			return k(m, pos)
		}
		if k(m, pos) {
			return true
		}
		return iter(m, pos, count, k)
	}
	return func(m *machine, pos int, k cont) bool { return rep(m, pos, 0, k) }
}

// singleRepeat repeats a one-character matcher without recursion per iteration.
func singleRepeat(s single, min, max int, greedy bool) matcher {
	return func(m *machine, pos int, k cont) bool {
		positions := []int{pos}
		p := pos
		for len(positions)-1 < min {
			p = s(m, p)
			if p < 0 {
				return false
			}
			positions = append(positions, p)
		}
		if greedy {
			for max == -1 || len(positions)-1 < max {
				np := s(m, p)
				if np < 0 {
					break
				}
				p = np
				positions = append(positions, p)
			}
			for i := len(positions) - 1; i >= min; i-- {
				if k(m, positions[i]) {
					return true
				}
			}
			return false
		}
		for {
			if k(m, p) {
				return true
			}
			if max != -1 && len(positions)-1 >= max {
				return false
			}
			np := s(m, p)
			if np < 0 {
				return false
			}
			p = np
			positions = append(positions, p)
		}
	}
}
