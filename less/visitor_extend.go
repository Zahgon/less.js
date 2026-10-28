package less

import (
	"strconv"
)

// extendState holds the properties the extend visitors add to nodes (allExtends, extendOnEveryPath).
type extendState struct {
	allExtends        map[Node][]*Extend
	extendOnEveryPath map[*Ruleset]bool
}

// extendFinderVisitor is ExtendFinderVisitor in visitors/extend-visitor.js.
type extendFinderVisitor struct {
	st              *extendState
	v               *Visitor
	allExtendsStack []*[]*Extend
	foundExtends    bool
	rootList        []*Extend
}

func (f *extendFinderVisitor) isReplacing() bool { return false }

func (f *extendFinderVisitor) visitIn(n Node, args *visitArgs) (any, bool) {
	switch x := n.(type) {
	case *Declaration, *MixinDefinition:
		args.visitDeeper = false
	case *Ruleset:
		if x.root {
			return nil, true
		}
		var all []*Extend
		for _, r := range x.rules {
			if e, ok := r.(*Extend); ok {
				all = append(all, e)
				f.st.extendOnEveryPath[x] = true
			}
		}
		for _, path := range x.paths {
			last := path[len(path)-1]
			var list []*Extend
			for _, n := range last.extendList {
				list = append(list, n.(*Extend))
			}
			list = append(list, all...)
			for j, e := range list {
				c := e.clone()
				f.foundExtends = true
				c.findSelfSelectors(path)
				c.ruleset = x
				if j == 0 {
					c.firstExtendOnThisSelectorPath = true
				}
				top := f.allExtendsStack[len(f.allExtendsStack)-1]
				*top = append(*top, c)
			}
		}
	case *Media, *AtRule:
		list := &[]*Extend{}
		f.allExtendsStack = append(f.allExtendsStack, list)
		f.st.allExtends[n] = nil
	default:
		return nil, false
	}
	return nil, true
}

func (f *extendFinderVisitor) visitOut(n any) {
	switch x := n.(type) {
	case *Media, *AtRule:
		top := f.allExtendsStack[len(f.allExtendsStack)-1]
		f.st.allExtends[x.(Node)] = *top
		f.allExtendsStack = f.allExtendsStack[:len(f.allExtendsStack)-1]
	}
}

// processExtendsVisitor is ProcessExtendsVisitor in visitors/extend-visitor.js.
type processExtendsVisitor struct {
	st              *extendState
	v               *Visitor
	allExtendsStack [][]*Extend
	extendIndices   map[string]bool
}

func runExtendVisitor(root *Ruleset) {
	st := &extendState{allExtends: map[Node][]*Extend{}, extendOnEveryPath: map[*Ruleset]bool{}}
	rootList := &[]*Extend{}
	finder := &extendFinderVisitor{st: st, allExtendsStack: []*[]*Extend{rootList}}
	finder.v = newVisitor(finder)
	finder.v.visit(root)
	if !finder.foundExtends {
		return
	}
	p := &processExtendsVisitor{st: st, extendIndices: map[string]bool{}}
	p.v = newVisitor(p)
	all := append(*rootList, p.doExtendChaining(*rootList, *rootList, 0)...)
	p.allExtendsStack = [][]*Extend{all}
	p.v.visit(root)
	p.checkExtendsForNonMatched(all)
}

func (p *processExtendsVisitor) checkExtendsForNonMatched(list []*Extend) {
	for _, e := range list {
		if e.hasFoundMatches || len(e.parentIDs) != 1 {
			continue
		}
		selector := "_unknown_"
		func() {
			defer func() { _ = recover() }()
			selector = toCSS(e.selector, &evalContext{})
		}()
		key := strconv.Itoa(getIndex(e)) + " " + selector
		if !p.extendIndices[key] {
			p.extendIndices[key] = true
			lessLogger.warn("WARNING: extend '" + selector + "' has no matches")
		}
	}
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (p *processExtendsVisitor) doExtendChaining(list, target []*Extend, iteration int) []*Extend {
	var toAdd []*Extend
	for _, extend := range list {
		for _, targetExtend := range target {
			if containsInt(extend.parentIDs, targetExtend.objectID) {
				continue
			}
			path := []*Selector{targetExtend.selfSelectors[0]}
			matches := p.findMatch(extend, path)
			if len(matches) == 0 {
				continue
			}
			extend.hasFoundMatches = true
			for _, self := range extend.selfSelectors {
				info := targetExtend.visibilityInfo()
				newSel := p.extendSelector(matches, path, self, isTrue(extend.isVisible()))
				ne := newExtendVis(targetExtend.selector, targetExtend.option, 0, fileInfoOf(targetExtend), info)
				ne.selfSelectors = newSel
				newSel[len(newSel)-1].extendList = []Node{ne}
				toAdd = append(toAdd, ne)
				ne.ruleset = targetExtend.ruleset
				ne.parentIDs = append(append(append([]int{}, ne.parentIDs...), targetExtend.parentIDs...), extend.parentIDs...)
				if targetExtend.firstExtendOnThisSelectorPath {
					ne.firstExtendOnThisSelectorPath = true
					targetExtend.ruleset.paths = append(targetExtend.ruleset.paths, newSel)
				}
			}
		}
	}
	if len(toAdd) == 0 {
		return toAdd
	}
	if iteration > 100 {
		one, two := "{unable to calculate}", "{unable to calculate}"
		func() {
			defer func() { _ = recover() }()
			one = toCSS(toAdd[0].selfSelectors[0], nil)
			two = toCSS(toAdd[0].selector, nil)
		}()
		panic(&errorInfo{Message: "extend circular reference detected. One of the circular extends is currently:" + one + ":extend(" + two + ")"})
	}
	return append(toAdd, p.doExtendChaining(toAdd, target, iteration+1)...)
}

func (p *processExtendsVisitor) isReplacing() bool { return false }

func (p *processExtendsVisitor) visitIn(n Node, args *visitArgs) (any, bool) {
	switch x := n.(type) {
	case *Declaration, *MixinDefinition, *Selector:
		args.visitDeeper = false
	case *Ruleset:
		if x.root {
			return nil, true
		}
		all := p.allExtendsStack[len(p.allExtendsStack)-1]
		var toAdd [][]*Selector
		paths := x.paths
		for _, extend := range all {
			for _, path := range paths {
				if p.st.extendOnEveryPath[x] {
					continue
				}
				if l := path[len(path)-1].extendList; len(l) > 0 {
					continue
				}
				matches := p.findMatch(extend, path)
				if len(matches) == 0 {
					continue
				}
				extend.hasFoundMatches = true
				visible := isTrue(extend.isVisible())
				for _, self := range extend.selfSelectors {
					toAdd = append(toAdd, p.extendSelector(matches, path, self, visible))
				}
			}
		}
		x.paths = append(append([][]*Selector{}, paths...), toAdd...)
	case *Media, *AtRule:
		own := p.st.allExtends[n]
		newAll := append(append([]*Extend{}, own...), p.allExtendsStack[len(p.allExtendsStack)-1]...)
		newAll = append(newAll, p.doExtendChaining(newAll, own, 0)...)
		p.allExtendsStack = append(p.allExtendsStack, newAll)
	default:
		return nil, false
	}
	return nil, true
}

func (p *processExtendsVisitor) visitOut(n any) {
	switch n.(type) {
	case *Media, *AtRule:
		p.allExtendsStack = p.allExtendsStack[:len(p.allExtendsStack)-1]
	}
}

type extendMatch struct {
	pathIndex, index, matched     int
	initialCombinator             *Combinator
	finished                      bool
	length, endPathIndex, endElem int
}

func (p *processExtendsVisitor) findMatch(extend *Extend, haystack []*Selector) []*extendMatch {
	needle := extend.selector.elements
	var potential, matches []*extendMatch
	for si, sel := range haystack {
		for ei, hn := range sel.elements {
			he := elemOf(hn)
			if extend.allowBefore || (si == 0 && ei == 0) {
				potential = append(potential, &extendMatch{pathIndex: si, index: ei, initialCombinator: he.combinator})
			}
			for i := 0; i < len(potential); i++ {
				pm := potential[i]
				target := he.combinator.value
				if target == "" && ei == 0 {
					target = " "
				}
				ne := elemOf(needle[pm.matched])
				if !elementValuesEqual(ne, he) || (pm.matched > 0 && ne.combinator.value != target) {
					pm = nil
				} else {
					pm.matched++
				}
				if pm != nil {
					pm.finished = pm.matched == len(needle)
					if pm.finished && !extend.allowAfter && (ei+1 < len(sel.elements) || si+1 < len(haystack)) {
						pm = nil
					}
				}
				if pm != nil {
					if pm.finished {
						pm.length = len(needle)
						pm.endPathIndex = si
						pm.endElem = ei + 1
						potential = potential[:0]
						matches = append(matches, pm)
					}
				} else {
					potential = append(potential[:i], potential[i+1:]...)
					i--
				}
			}
		}
	}
	return matches
}

// elementValuesEqual is isElementValuesEqual on two elements' values.
func elementValuesEqual(a, b *Element) bool {
	if a.value == nil || b.value == nil {
		if a.value == nil && b.value == nil {
			return a.valueStr == b.valueStr
		}
		return false
	}
	return nodeValuesEqual(a.value, b.value)
}

func nodeValuesEqual(v1, v2 Node) bool {
	if a1, ok := v1.(*Attribute); ok {
		a2, ok2 := v2.(*Attribute)
		if !ok2 {
			return false
		}
		if a1.op != a2.op || a1.key != a2.key || a1.keyStr != a2.keyStr {
			return false
		}
		has1 := a1.value != nil || a1.valueStr != ""
		has2 := a2.value != nil || a2.valueStr != ""
		if !has1 || !has2 {
			return !has1 && !has2
		}
		return attrValueKey(a1) == attrValueKey(a2)
	}
	p1, ok1 := v1.(*Paren)
	p2, ok2 := v2.(*Paren)
	if !ok1 || !ok2 {
		return false
	}
	s1, ok1 := p1.value.(*Selector)
	if !ok1 {
		return false
	}
	s2, ok2 := p2.value.(*Selector)
	if !ok2 || len(s1.elements) != len(s2.elements) {
		return false
	}
	for i := range s1.elements {
		e1, isEl1 := s1.elements[i].(*Element)
		e2, isEl2 := s2.elements[i].(*Element)
		if !isEl1 || !isEl2 {
			return false
		}
		if e1.combinator.value != e2.combinator.value {
			c1, c2 := e1.combinator.value, e2.combinator.value
			if c1 == "" {
				c1 = " "
			}
			if c2 == "" {
				c2 = " "
			}
			if i != 0 || c1 != c2 {
				return false
			}
		}
		if !elementValuesEqual(e1, e2) {
			return false
		}
	}
	return true
}

// attrValueKey is `value.value || value`: a quoted value compares by its text, other values by identity.
func attrValueKey(a *Attribute) any {
	if a.value == nil {
		return a.valueStr
	}
	if s, ok := stringValue(a.value); ok && s != "" {
		return s
	}
	return a.value
}

func (p *processExtendsVisitor) extendSelector(matches []*extendMatch, path []*Selector, replacement *Selector, visible bool) []*Selector {
	curPath, curElem := 0, 0
	var out []*Selector
	for mi, m := range matches {
		sel := path[m.pathIndex]
		r0 := elemOf(replacement.elements[0])
		first := newElementVis(m.initialCombinator, r0.value, r0.valueStr, r0.isVariable, getIndex(r0), fileInfoOf(r0), nil)
		if m.pathIndex > curPath && curElem > 0 {
			last := out[len(out)-1]
			last.elements = append(append([]Node{}, last.elements...), path[curPath].elements[curElem:]...)
			curElem = 0
			curPath++
		}
		newElements := append(append(append([]Node{}, sel.elements[curElem:m.index]...), first), replacement.elements[1:]...)
		if curPath == m.pathIndex && mi > 0 {
			last := out[len(out)-1]
			last.elements = append(append([]Node{}, last.elements...), newElements...)
		} else {
			out = append(out, path[curPath:m.pathIndex]...)
			out = append(out, newSelector(newElements, nil, nil, 0, nil, nil))
		}
		curPath = m.endPathIndex
		curElem = m.endElem
		if curElem >= len(path[curPath].elements) {
			curElem = 0
			curPath++
		}
	}
	if curPath < len(path) && curElem > 0 {
		last := out[len(out)-1]
		last.elements = append(append([]Node{}, last.elements...), path[curPath].elements[curElem:]...)
		curPath++
	}
	out = append(out, path[curPath:]...)
	res := make([]*Selector, len(out))
	for i, s := range out {
		d := s.createDerived(s.elements, nil, nil)
		if visible {
			d.ensureVisibility()
		} else {
			d.ensureInvisibility()
		}
		res[i] = d
	}
	return res
}
