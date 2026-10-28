package less

import (
	"strings"
)

// toCSSVisitor is visitors/to-css-visitor.js: it drops invisible and silent nodes, flattens nested rulesets and
// merges declarations before output.
type toCSSVisitor struct {
	v       *Visitor
	ctx     *evalContext
	charset bool
}

func newToCSSVisitor(ctx *evalContext) *toCSSVisitor {
	t := &toCSSVisitor{ctx: ctx}
	t.v = newVisitor(t)
	return t
}

func (t *toCSSVisitor) isReplacing() bool { return true }

func (t *toCSSVisitor) visitOut(n any) {}

func rulesOf(n Node) ([]Node, bool) {
	switch x := n.(type) {
	case *Ruleset:
		return x.rules, x.rules != nil
	case *Media:
		return x.rules, x.rules != nil
	case *AtRule:
		return x.rules, x.rules != nil
	case *MixinDefinition:
		return x.rules, x.rules != nil
	}
	return nil, false
}

func isEmptyOwner(n Node) bool {
	rules, ok := rulesOf(n)
	return !ok || len(rules) == 0
}

func (t *toCSSVisitor) resolveVisibility(n Node) Node {
	b := n.base()
	if !b.blocksVisibility() {
		if isEmptyOwner(n) {
			return nil
		}
		return n
	}
	rules, _ := rulesOf(n)
	if len(rules) == 0 {
		return nil
	}
	body := rules[0]
	if rs, ok := body.(*Ruleset); ok && rs.rules != nil {
		var kept []Node
		for _, r := range rs.rules {
			if isTrue(r.base().isVisible()) {
				kept = append(kept, r)
			}
		}
		if kept == nil {
			kept = []Node{}
		}
		rs.rules = kept
	}
	if isEmptyOwner(body) {
		return nil
	}
	b.ensureVisibility()
	b.removeVisibilityBlock()
	return n
}

func (t *toCSSVisitor) visitIn(n Node, args *visitArgs) (any, bool) {
	switch x := n.(type) {
	case *Declaration:
		if x.blocksVisibility() || x.variable {
			return nil, true
		}
		return x, true
	case *MixinDefinition:
		x.frames = []Node{}
		return nil, true
	case *Extend:
		return nil, true
	case *Comment:
		if x.blocksVisibility() || x.isSilent(t.ctx) {
			return nil, true
		}
		return x, true
	case *Media:
		x.accept(t.v)
		args.visitDeeper = false
		return nodeOrNilAny(t.resolveVisibility(x)), true
	case *Import:
		if x.blocksVisibility() {
			return nil, true
		}
		return x, true
	case *AtRule:
		if len(x.rules) > 0 {
			x.accept(t.v)
			args.visitDeeper = false
			if !isEmptyOwner(x) {
				if rs, ok := x.rules[0].(*Ruleset); ok {
					rs.rules = mergeRules(rs.rules)
				}
			}
			return nodeOrNilAny(t.resolveVisibility(x)), true
		}
		return t.atRuleWithoutBody(x), true
	case *Anonymous:
		if !x.blocksVisibility() {
			x.accept(t.v)
			return x, true
		}
		return nil, true
	case *Ruleset:
		return t.visitRuleset(x, args), true
	}
	return nil, false
}

func nodeOrNilAny(n Node) any {
	if n == nil {
		return nil
	}
	return n
}

var reNewlines = re(`\n`)

func (t *toCSSVisitor) atRuleWithoutBody(a *AtRule) any {
	if a.blocksVisibility() {
		return nil
	}
	if a.name == "@charset" {
		if t.charset {
			if a.debugInfo != nil {
				c := newComment("/* "+strings.ReplaceAll(toCSS(a, t.ctx), "\n", "")+" */\n", false, 0, nil)
				c.debugInfo = a.debugInfo
				return nodeOrNilAny(t.v.visit(c))
			}
			return nil
		}
		t.charset = true
	}
	return a
}

func (t *toCSSVisitor) checkValidNodes(rules []Node, isRoot bool) {
	for _, r := range rules {
		if d, ok := r.(*Declaration); ok && isRoot && !d.variable {
			panic(&errorInfo{Message: "Properties must be inside selector blocks. They cannot be in the root",
				Index: intp(getIndex(d)), Filename: fileInfoOf(d).Filename})
		}
		if c, ok := r.(*Call); ok {
			panic(&errorInfo{Message: "Function '" + c.name + "' did not return a root node",
				Index: intp(getIndex(c)), Filename: fileInfoOf(c).Filename})
		}
		if r.Type() != "" && !allowRoot(r) {
			panic(&errorInfo{Message: r.Type() + " node returned by a function is not valid here",
				Index: intp(getIndex(r)), Filename: fileInfoOf(r).Filename})
		}
	}
}

// allowRoot reports the allowRoot property of a node type.
func allowRoot(n Node) bool {
	switch x := n.(type) {
	case *Anonymous, *Comment, *Declaration, *Ruleset, *MixinDefinition, *AtRule, *Media, *Import, *Extend,
		*MixinCall, *VariableCall:
		return true
	case *Quoted:
		return x.allowRoot
	}
	return false
}

func (t *toCSSVisitor) visitRuleset(rs *Ruleset, args *visitArgs) any {
	var rulesets []Node
	t.checkValidNodes(rs.rules, rs.firstRoot)
	if !rs.root {
		t.compilePaths(rs)
		n := len(rs.rules)
		for i := 0; i < n; {
			rule := rs.rules[i]
			if _, has := rulesOf(rule); has && rule != nil {
				if v := t.v.visitAny(rule); v != nil {
					switch r := v.(type) {
					case Node:
						rulesets = append(rulesets, r)
					case []Node:
						rulesets = append(rulesets, &nodeList{nodes: r})
					}
				} else {
					rulesets = append(rulesets, nil)
				}
				rs.rules = append(rs.rules[:i:i], rs.rules[i+1:]...)
				n--
				continue
			}
			i++
		}
		if n > 0 {
			rs.accept(t.v)
		} else {
			rs.rules = nil
		}
		args.visitDeeper = false
	} else {
		rs.accept(t.v)
		args.visitDeeper = false
	}
	if rs.rules != nil {
		rs.rules = mergeRules(rs.rules)
		rs.rules = t.removeDuplicateRules(rs.rules)
	}
	if t.isVisibleRuleset(rs) {
		rs.ensureVisibility()
		rulesets = append([]Node{rs}, rulesets...)
	}
	if len(rulesets) == 1 {
		if l, ok := rulesets[0].(*nodeList); ok {
			return l.nodes
		}
		return nodeOrNilAny(rulesets[0])
	}
	var out []Node
	for _, r := range rulesets {
		if l, ok := r.(*nodeList); ok {
			out = append(out, &nodeList{nodes: l.nodes})
		} else {
			out = append(out, r)
		}
	}
	return flattenRulesets(out)
}

// flattenRulesets turns the returned array (with nested arrays from inner rulesets) into the visitor's list.
func flattenRulesets(in []Node) []Node {
	out := []Node{}
	for _, n := range in {
		if l, ok := n.(*nodeList); ok {
			out = append(out, flattenRulesets(l.nodes)...)
		} else if n != nil {
			out = append(out, n)
		}
	}
	return out
}

func (t *toCSSVisitor) isVisibleRuleset(rs *Ruleset) bool {
	if rs.firstRoot {
		return true
	}
	if isEmptyOwner(rs) {
		return false
	}
	if !rs.root && len(rs.paths) == 0 {
		return false
	}
	return true
}

func (t *toCSSVisitor) compilePaths(rs *Ruleset) {
	if rs.paths == nil {
		return
	}
	var kept [][]*Selector
	for _, p := range rs.paths {
		if e, ok := p[0].elements[0].(*Element); ok && e.combinator.value == " " {
			e.combinator = newCombinator("")
		}
		for _, s := range p {
			if isTrue(s.isVisible()) && s.getIsOutput() {
				kept = append(kept, p)
				break
			}
		}
	}
	if kept == nil {
		kept = [][]*Selector{}
	}
	rs.paths = kept
}

func (t *toCSSVisitor) removeDuplicateRules(rules []Node) []Node {
	type entry struct {
		first *Declaration
		list  []string
	}
	cache := map[string]*entry{}
	for i := len(rules) - 1; i >= 0; i-- {
		d, ok := rules[i].(*Declaration)
		if !ok {
			continue
		}
		key := d.nameKey()
		e, seen := cache[key]
		if !seen {
			cache[key] = &entry{first: d}
			continue
		}
		if e.list == nil {
			e.list = []string{toCSS(e.first, t.ctx)}
		}
		css := toCSS(d, t.ctx)
		dup := false
		for _, s := range e.list {
			if s == css {
				dup = true
				break
			}
		}
		if dup {
			rules = append(rules[:i:i], rules[i+1:]...)
		} else {
			e.list = append(e.list, css)
		}
	}
	return rules
}
