package less

// joinSelectorVisitor is visitors/join-selector-visitor.js: it computes every ruleset's selector paths.
type joinSelectorVisitor struct {
	contexts [][][]*Selector
	v        *Visitor
}

func newJoinSelectorVisitor() *joinSelectorVisitor {
	j := &joinSelectorVisitor{contexts: [][][]*Selector{{}}}
	j.v = newVisitor(j)
	return j
}

func (j *joinSelectorVisitor) run(root Node) Node { return j.v.visit(root) }

func (j *joinSelectorVisitor) isReplacing() bool { return false }

func (j *joinSelectorVisitor) visitIn(n Node, args *visitArgs) (any, bool) {
	switch x := n.(type) {
	case *Declaration:
		args.visitDeeper = false
	case *MixinDefinition:
		args.visitDeeper = false
	case *Ruleset:
		context := j.contexts[len(j.contexts)-1]
		var paths [][]*Selector
		j.contexts = append(j.contexts, nil)
		if !x.root {
			var sels []*Selector
			if x.selectors != nil {
				for _, s := range x.selectors {
					if s.getIsOutput() {
						sels = append(sels, s)
					}
				}
				if len(sels) > 0 {
					x.selectors = sels
					x.joinSelectors(&paths, context, sels)
				} else {
					x.selectors = nil
				}
			}
			if sels == nil {
				x.rules = nil
			}
			if paths == nil {
				paths = [][]*Selector{}
			}
			x.paths = paths
		}
		j.contexts[len(j.contexts)-1] = paths
	case *Media:
		context := j.contexts[len(j.contexts)-1]
		if rs, ok := x.rules[0].(*Ruleset); ok {
			rs.root = len(context) == 0 || isMultiMedia(context[0])
		}
	case *AtRule:
		context := j.contexts[len(j.contexts)-1]
		if len(x.declarations) > 0 {
			if rs, ok := x.declarations[0].(*Ruleset); ok {
				rs.root = len(context) == 0 || isMultiMedia(context[0])
			}
		} else if len(x.rules) > 0 {
			if rs, ok := x.rules[0].(*Ruleset); ok {
				rs.root = x.isRooted || len(context) == 0
			}
		}
	default:
		return nil, false
	}
	return nil, true
}

// isMultiMedia reads `context[0].multiMedia`, where context[0] is a selector path (an array), which never has it.
func isMultiMedia(_ []*Selector) bool { return false }

func (j *joinSelectorVisitor) visitOut(n any) {
	if _, ok := n.(*Ruleset); ok {
		j.contexts = j.contexts[:len(j.contexts)-1]
	}
}

// visibilityVisitor is visitors/set-tree-visibility-visitor.js.
type visibilityVisitor struct{ visible bool }

func (s *visibilityVisitor) run(root Node) { s.visit(root) }

func (s *visibilityVisitor) visitArray(nodes []Node, _ ...bool) []Node {
	for _, n := range nodes {
		s.visit(n)
	}
	return nodes
}

func (s *visibilityVisitor) visitAny(n Node) any { return s.visit(n) }

func (s *visibilityVisitor) visit(n Node) Node {
	if n == nil {
		return n
	}
	if l, ok := n.(*nodeList); ok {
		s.visitArray(l.nodes)
		return n
	}
	if n.base().blocksVisibility() {
		return n
	}
	if s.visible {
		n.base().ensureVisibility()
	} else {
		n.base().ensureInvisibility()
	}
	if a, ok := n.(acceptor); ok {
		a.accept(s)
	}
	return n
}
