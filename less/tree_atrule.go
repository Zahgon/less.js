package less

import (
	"github.com/less/less.js/internal/js"
)

// AtRule is tree/atrule.js.
type AtRule struct {
	nodeBase
	name         string
	value        Node
	simpleBlock  bool
	declarations []Node
	rules        []Node
	debugInfo    *debugInfo
	isRooted     bool
}

// newAtRule is new AtRule(...). The parser passes its block as a single Ruleset (block), evaluation passes an
// array of rules.
func newAtRule(name string, value Node, block *Ruleset, rules []Node, index int, fi *FileInfo, dbg *debugInfo, isRooted bool, vis *visibilityInfo) *AtRule {
	a := &AtRule{name: name, value: value}
	if block != nil {
		if declarationsBlock(block.rules, false) && !isRooted && value == nil {
			a.simpleBlock = true
			a.declarations = block.rules
		} else {
			a.rules = []Node{block}
			block.selectors = newSelector([]Node{}, nil, nil, index, fi, nil).createEmptySelectors()
			block.allowImports = true
		}
		setParents(a.declarations, a)
		setParents(a.rules, a)
	} else if rules != nil {
		allDecls := declarationsBlock(rules, false)
		allRulesetDecls := true
		for _, r := range rules {
			if rs, ok := r.(*Ruleset); ok && rs.rules != nil {
				allRulesetDecls = allRulesetDecls && declarationsBlock(rs.rules, true)
			}
		}
		switch {
		case allDecls && !isRooted:
			a.simpleBlock = true
			a.declarations = rules
		case allRulesetDecls && len(rules) == 1 && !isRooted && a.value == nil:
			a.simpleBlock = true
			if rs, ok := rules[0].(*Ruleset); ok && rs.rules != nil {
				a.declarations = rs.rules
			} else {
				a.declarations = rules
			}
		default:
			a.rules = rules
		}
		if !a.simpleBlock {
			for _, r := range a.rules {
				if rs, ok := r.(*Ruleset); ok {
					rs.allowImports = true
				}
			}
		}
		setParents(a.declarations, a)
		setParents(a.rules, a)
	}
	a.index = index
	a.fileInfo = fi
	a.debugInfo = dbg
	a.isRooted = isRooted
	a.copyVisibilityInfo(vis)
	return a
}

func declarationsBlock(rules []Node, mergeable bool) bool {
	for _, n := range rules {
		switch v := n.(type) {
		case *Declaration:
			if !mergeable && v.merge != "" {
				return false
			}
		case *Comment:
		default:
			return false
		}
	}
	return true
}

// Type is the node's type name, as the original's type getter reports it.
func (a *AtRule) Type() string { return "AtRule" }

func (a *AtRule) accept(v visitor) {
	if a.rules != nil {
		a.rules = v.visitArray(a.rules)
	} else if a.declarations != nil {
		a.declarations = v.visitArray(a.declarations)
	}
	if a.value != nil {
		a.value = v.visit(a.value)
	}
}

func (a *AtRule) isRulesetLike() bool { return a.rules != nil || !a.isCharset() }

func (a *AtRule) isCharset() bool { return a.name == "@charset" }

func (a *AtRule) genCSS(ctx *evalContext, out *output) {
	out.addAt(a.name, fileInfoOf(a), getIndex(a), false)
	if a.value != nil {
		out.add(" ")
		genCSS(a.value, ctx, out)
	}
	switch {
	case a.simpleBlock:
		outputRuleset(ctx, out, a.declarations)
	case a.rules != nil:
		outputRuleset(ctx, out, a.rules)
	case a.declarations != nil:
		outputRuleset(ctx, out, a.declarations)
	default:
		out.add(";")
	}
}

func (a *AtRule) eval(ctx *evalContext) Node {
	value := a.value
	rules := a.rules
	if rules == nil {
		rules = a.declarations
	}
	pathBackup, blocksBackup, setBackup := ctx.mediaPath, ctx.mediaBlocks, ctx.mediaSet
	ctx.mediaPath, ctx.mediaBlocks, ctx.mediaSet = []Node{}, []Node{}, true
	if value != nil {
		value = evalNode(value, ctx)
	}
	if rules != nil {
		rules = a.evalRoot(ctx, rules)
	}
	if len(rules) > 0 {
		if rs, ok := rules[0].(*Ruleset); ok && len(rs.rules) > 0 {
			if declarationsBlock(rs.rules, true) && !a.isRooted && value == nil {
				rs.rules = mergeRules(rs.rules)
				rules = rs.rules
				for _, r := range rules {
					if d, isDecl := r.(*Declaration); isDecl {
						d.merge = ""
					}
				}
			}
		}
	}
	if a.simpleBlock && rules != nil {
		if rs, ok := rules[0].(*Ruleset); ok {
			if fr, isFrame := ctx.frames[0].(frameNode); isFrame {
				rs.functionRegistry = fr.registry().inherit()
			}
		}
		evald := make([]Node, len(rules))
		for i, r := range rules {
			evald[i] = evalNode(r, ctx)
		}
		rules = evald
	}
	ctx.mediaPath, ctx.mediaBlocks, ctx.mediaSet = pathBackup, blocksBackup, setBackup
	return newAtRule(a.name, value, nil, rules, getIndex(a), fileInfoOf(a), a.debugInfo, a.isRooted, a.visibilityInfo())
}

func allAmpersands(sels []*Selector) bool {
	for _, s := range sels {
		if len(s.elements) == 0 {
			return false
		}
		for _, e := range s.elements {
			if el, ok := e.(*Element); !ok || !el.isParentRef() {
				return false
			}
		}
	}
	return true
}

func (a *AtRule) evalRoot(ctx *evalContext, rules []Node) []Node {
	ampersandCount, noAmpersandCount := 0, 0
	noAmpersands := true
	if !a.simpleBlock {
		rules = []Node{evalNode(rules[0], ctx)}
	}
	var preceding []*Selector
	for _, f := range ctx.frames {
		if rs, ok := f.(*Ruleset); ok && len(rs.rules) > 0 && !rs.root && len(rs.selectors) > 0 {
			preceding = append(preceding, rs.selectors...)
		}
		if len(preceding) > 0 {
			if allAmpersands(preceding) {
				noAmpersands = false
				noAmpersandCount++
			} else {
				ampersandCount++
			}
		}
	}
	mixed := ampersandCount > 0 && noAmpersandCount > 0 && !noAmpersands
	if (a.isRooted && ampersandCount > 0 && noAmpersandCount == 0 && noAmpersands) || !mixed {
		if len(rules) == 0 {
			typeError("Cannot set properties of undefined (setting 'root')")
		}
		if rs, ok := rules[0].(*Ruleset); ok {
			rs.root = true
		}
	}
	return rules
}

func outputRuleset(ctx *evalContext, out *output, rules []Node) {
	ctx.tabLevel++
	if ctx.compress {
		out.add("{")
		for _, r := range rules {
			genCSS(r, ctx, out)
		}
		out.add("}")
		ctx.tabLevel--
		return
	}
	tabSet := "\n" + repeatStr("  ", ctx.tabLevel-1)
	tabRule := tabSet + "  "
	if len(rules) == 0 {
		out.add(" {" + tabSet + "}")
	} else {
		out.add(" {" + tabRule)
		genCSS(rules[0], ctx, out)
		for _, r := range rules[1:] {
			out.add(tabRule)
			genCSS(r, ctx, out)
		}
		out.add(tabSet + "}")
	}
	ctx.tabLevel--
}

// Media is tree/media.js; with container set it is tree/container.js, which differs only in its keyword and
// in returning itself when evaluated twice.
type Media struct {
	nodeBase
	features  Node
	rules     []Node
	debugInfo *debugInfo
	container bool
	evaluated bool
}

func newMediaKind(container bool, value []Node, features []Node, index int, fi *FileInfo, vis *visibilityInfo) *Media {
	m := &Media{container: container}
	m.index = index
	m.fileInfo = fi
	sels := newSelector([]Node{}, nil, nil, index, fi, nil).createEmptySelectors()
	m.features = newValue(features, 0)
	rs := newRuleset(sels, value, false, nil)
	rs.allowImports = true
	m.rules = []Node{rs}
	m.copyVisibilityInfo(vis)
	setParents(selectorNodes(sels), m)
	setParent(m.features, m)
	setParents(m.rules, m)
	return m
}

func newMedia(value, features []Node, index int, fi *FileInfo, vis *visibilityInfo) *Media {
	return newMediaKind(false, value, features, index, fi, vis)
}

func newContainer(value, features []Node, index int, fi *FileInfo, vis *visibilityInfo) *Media {
	return newMediaKind(true, value, features, index, fi, vis)
}

// Type is the node's type name, as the original's type getter reports it.
func (m *Media) Type() string {
	if m.container {
		return "Container"
	}
	return "Media"
}

func (m *Media) keyword() string {
	if m.container {
		return "@container "
	}
	return "@media "
}

func (m *Media) isRulesetLike() bool { return true }

func (m *Media) accept(v visitor) {
	if m.features != nil {
		m.features = v.visit(m.features)
	}
	if m.rules != nil {
		m.rules = v.visitArray(m.rules)
	}
}

func (m *Media) genCSS(ctx *evalContext, out *output) {
	out.addAt(m.keyword(), m.fileInfo, m.index, false)
	genCSS(m.features, ctx, out)
	outputRuleset(ctx, out, m.rules)
}

func (m *Media) eval(ctx *evalContext) Node {
	if m.container && m.evaluated {
		return m
	}
	if !ctx.mediaSet {
		ctx.mediaBlocks, ctx.mediaPath, ctx.mediaSet = []Node{}, []Node{}, true
	}
	media := newMediaKind(m.container, nil, []Node{}, m.index, m.fileInfo, m.visibilityInfo())
	media.evaluated = m.container
	if m.debugInfo != nil {
		m.rules[0].(*Ruleset).debugInfo = m.debugInfo
		media.debugInfo = m.debugInfo
	}
	media.features = evalNode(m.features, ctx)
	ctx.mediaPath = append(ctx.mediaPath, media)
	ctx.mediaBlocks = append(ctx.mediaBlocks, media)
	if fr, ok := ctx.frames[0].(frameNode); ok && fr.registry() != nil {
		m.rules[0].(*Ruleset).functionRegistry = fr.registry().inherit()
	}
	ctx.frames = append([]Node{m.rules[0]}, ctx.frames...)
	media.rules = []Node{evalNode(m.rules[0], ctx)}
	ctx.frames = ctx.frames[1:]
	ctx.mediaPath = ctx.mediaPath[:len(ctx.mediaPath)-1]
	if len(ctx.mediaPath) == 0 {
		return media.evalTop(ctx)
	}
	return media.evalNested(ctx)
}

func (m *Media) evalFunction() {
	vals, ok := arrayValue(m.features)
	if !ok || len(vals) < 1 {
		return
	}
	for i := 0; i < len(vals); i++ {
		expr := vals[i]
		ns := expr.base().noSpacing
		if (expr.Type() == "Keyword" || expr.Type() == "Variable") && i+1 < len(vals) && (ns.truthy() || ns == triUndefined) {
			if p, isParen := vals[i+1].(*Paren); isParen && p.noSpacing.truthy() {
				e := newExpression([]Node{expr, p}, true)
				vals[i] = e
				vals = append(vals[:i+1], vals[i+2:]...)
			}
		}
	}
	switch f := m.features.(type) {
	case *Value:
		f.value = vals
	case *Expression:
		f.value = vals
	}
}

func (m *Media) evalTop(ctx *evalContext) Node {
	m.evalFunction()
	var result Node = m
	if len(ctx.mediaBlocks) > 1 {
		sels := newSelector([]Node{}, nil, nil, getIndex(m), fileInfoOf(m), nil).createEmptySelectors()
		rs := newRuleset(sels, ctx.mediaBlocks, false, nil)
		rs.multiMedia = true
		rs.copyVisibilityInfo(m.visibilityInfo())
		setParent(rs, m)
		result = rs
	}
	ctx.mediaBlocks, ctx.mediaPath, ctx.mediaSet = nil, nil, false
	return result
}

func (m *Media) evalNested(ctx *evalContext) Node {
	m.evalFunction()
	path := append(append([]Node{}, ctx.mediaPath...), m)
	lists := make([][]Node, len(path))
	for i, p := range path {
		pm, ok := p.(*Media)
		if !ok || pm.Type() != m.Type() {
			for j, b := range ctx.mediaBlocks {
				if b == Node(m) {
					ctx.mediaBlocks = append(ctx.mediaBlocks[:j:j], ctx.mediaBlocks[j+1:]...)
					break
				}
			}
			return m
		}
		if v, isValue := pm.features.(*Value); isValue {
			lists[i] = v.value
		} else {
			lists[i] = []Node{pm.features}
		}
	}
	var exprs []Node
	for _, p := range permute(lists) {
		types := []Node{}
		var rest []Node
		for _, f := range p {
			if startsWithMediaType(f) {
				types = append(types, f)
			} else {
				rest = append(rest, f)
			}
		}
		if len(types) > 0 && len(types) < len(p) {
			p = append(types, rest...)
		}
		var withAnd []Node
		for i, f := range p {
			if i > 0 {
				withAnd = append(withAnd, newAnonymous("and", 0, nil, false, false, nil))
			}
			withAnd = append(withAnd, f)
		}
		exprs = append(exprs, newExpression(withAnd, false))
	}
	m.features = newValue(exprs, 0)
	setParent(m.features, m)
	return newRuleset([]*Selector{}, []Node{}, false, nil)
}

func permute(arr [][]Node) [][]Node {
	switch len(arr) {
	case 0:
		return nil
	case 1:
		out := make([][]Node, len(arr[0]))
		for i, n := range arr[0] {
			out[i] = []Node{n}
		}
		return out
	}
	var result [][]Node
	rest := permute(arr[1:])
	for _, r := range rest {
		for _, first := range arr[0] {
			result = append(result, append([]Node{first}, r...))
		}
	}
	return result
}

func (m *Media) bubbleSelectors(selectors []*Selector) {
	if selectors == nil {
		return
	}
	m.rules = []Node{newRuleset(append([]*Selector{}, selectors...), []Node{m.rules[0]}, false, nil)}
	setParents(m.rules, m)
}

var reMediaTypeSplit = re(`[\s(]`)

func startsWithMediaType(fragment Node) bool {
	head := ""
	isString := false
	switch f := fragment.(type) {
	case *Keyword:
		head, isString = f.value, true
	case *Anonymous:
		head, isString = stringValue(f)
	case *Expression:
		var parts []Node
		for _, p := range f.value {
			if p != nil && hasValueProperty(p) {
				parts = append(parts, p)
			}
		}
		idx := 0
		if len(parts) > 0 {
			first := js.ToLowerCase(jsValueString(parts[0]))
			if first == "not" || first == "only" {
				idx++
			}
		}
		if idx < len(parts) {
			head, isString = stringValue(parts[idx])
		}
	}
	if !isString || head == "" {
		return false
	}
	trimmed := js.ToU16(js.Trim(head))
	end := len(trimmed)
	if caps := reMediaTypeSplit.Search(trimmed, 0); caps != nil {
		end = caps[0]
	}
	first := js.FromU16(trimmed[:end])
	lower := js.ToLowerCase(first)
	return first != "" && lower != "and" && lower != "or"
}

// hasValueProperty reports whether `node.value !== undefined`.
func hasValueProperty(n Node) bool {
	switch v := n.(type) {
	case *Color:
		return v.value != ""
	case *Call, *Variable, *Property, *Operation, *Condition, *MixinCall, *VariableCall, *DetachedRuleset,
		*Ruleset, *Selector, *JavaScript, *Media, *Import, *Extend, *jsBool:
		return false
	}
	return true
}
