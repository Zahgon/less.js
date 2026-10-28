package less

// Ruleset is tree/ruleset.js.
type Ruleset struct {
	nodeBase
	selectors        []*Selector
	rules            []Node
	lookups          map[string][]mixinMatch
	variablesCache   map[string]*Declaration
	propertiesCache  map[string][]*Declaration
	strictImports    bool
	root             bool
	firstRoot        bool
	allowImports     bool
	functionRegistry *functionRegistry
	originalRuleset  Node
	debugInfo        *debugInfo
	paths            [][]*Selector
	evalFirst        bool
	multiMedia       bool
}

// mixinMatch is one result of Ruleset.find: the matching rule and the namespaces leading to it.
type mixinMatch struct {
	rule Node
	path []Node
}

func newRuleset(selectors []*Selector, rules []Node, strictImports bool, vis *visibilityInfo) *Ruleset {
	r := &Ruleset{selectors: selectors, rules: rules, strictImports: strictImports, lookups: map[string][]mixinMatch{}}
	r.copyVisibilityInfo(vis)
	for _, s := range selectors {
		setParent(s, r)
	}
	setParents(rules, r)
	return r
}

func (r *Ruleset) Type() string { return "Ruleset" }

func (r *Ruleset) isRulesetLike() bool { return true }

func (r *Ruleset) registry() *functionRegistry { return r.functionRegistry }

func (r *Ruleset) accept(v visitor) {
	if r.paths != nil {
		for _, p := range r.paths {
			v.visitArray(selectorNodes(p), true)
		}
	} else if r.selectors != nil {
		r.selectors = toSelectors(v.visitArray(selectorNodes(r.selectors)))
	}
	if len(r.rules) > 0 {
		r.rules = v.visitArray(r.rules)
	}
}

func toSelectors(ns []Node) []*Selector {
	out := make([]*Selector, 0, len(ns))
	for _, n := range ns {
		if s, ok := n.(*Selector); ok {
			out = append(out, s)
		}
	}
	return out
}

func (r *Ruleset) eval(ctx *evalContext) Node {
	var selectors []*Selector
	hasOnePassing := false
	if len(r.selectors) > 0 {
		selectors = make([]*Selector, len(r.selectors))
		defaultFn.setError(&errorInfo{Type: "Syntax", Message: "it is currently only allowed in parametric mixin guards,"})
		hasVariable := false
		for i, s := range r.selectors {
			sel := s.eval(ctx).(*Selector)
			for _, e := range sel.elements {
				if el, ok := e.(*Element); ok && el.isVariable {
					hasVariable = true
					break
				}
			}
			selectors[i] = sel
			if sel.evaldCondition {
				hasOnePassing = true
			}
		}
		if hasVariable {
			parts := make([]string, len(selectors))
			for i, s := range selectors {
				parts[i] = toCSS(s, ctx)
			}
			p := newParser(&parseContext{}, nil, fileInfoOf(selectors[0]), getIndex(selectors[0]))
			if res, ok := p.parseNode(joinStrings(parts, ","), []string{"selectors"}); ok {
				if list, isList := res[0].(*selectorList); isList {
					selectors = list.list
				}
			}
		}
		defaultFn.reset()
	} else {
		hasOnePassing = true
	}
	var rules []Node
	if r.rules != nil {
		rules = append([]Node{}, r.rules...)
	}
	rs := newRuleset(selectors, rules, r.strictImports, r.visibilityInfo())
	rs.originalRuleset = r
	rs.root = r.root
	rs.firstRoot = r.firstRoot
	rs.allowImports = r.allowImports
	if r.debugInfo != nil {
		rs.debugInfo = r.debugInfo
	}
	if !hasOnePassing {
		rs.rules = rs.rules[:0]
	}
	var found *functionRegistry
	for _, f := range ctx.frames {
		if fr, ok := f.(frameNode); ok {
			if found = fr.registry(); found != nil {
				break
			}
		}
	}
	if found == nil {
		found = globalFunctions
	}
	rs.functionRegistry = found.inherit()
	ctx.frames = append([]Node{rs}, ctx.frames...)
	ctx.selectors = append([][]*Selector{r.selectors}, ctx.selectors...)
	if rs.root || rs.allowImports || !rs.strictImports {
		rs.evalImports(ctx)
	}
	for i := 0; i < len(rs.rules) && rs.rules[i] != nil; i++ {
		if isEvalFirst(rs.rules[i]) {
			rs.rules[i] = evalNode(rs.rules[i], ctx)
		}
	}
	mediaBlockCount := len(ctx.mediaBlocks)
	for i := 0; i < len(rs.rules) && rs.rules[i] != nil; i++ {
		switch rule := rs.rules[i].(type) {
		case *MixinCall:
			var kept []Node
			for _, n := range rule.evalRules(ctx) {
				if d, ok := n.(*Declaration); ok && d.variable && rs.variable(d.name) != nil {
					continue
				}
				kept = append(kept, n)
			}
			rs.rules = spliceNodes(rs.rules, i, kept)
			i += len(kept) - 1
			rs.resetCache()
		case *VariableCall:
			var kept []Node
			for _, n := range rule.eval(ctx).(*Ruleset).rules {
				if d, ok := n.(*Declaration); ok && d.variable {
					continue
				}
				kept = append(kept, n)
			}
			rs.rules = spliceNodes(rs.rules, i, kept)
			i += len(kept) - 1
			rs.resetCache()
		}
	}
	for i := 0; i < len(rs.rules) && rs.rules[i] != nil; i++ {
		if !isEvalFirst(rs.rules[i]) {
			rs.rules[i] = evalNode(rs.rules[i], ctx)
		}
	}
	for i := 0; i < len(rs.rules) && rs.rules[i] != nil; i++ {
		rule, ok := rs.rules[i].(*Ruleset)
		if !ok || len(rule.selectors) != 1 || rule.selectors[0] == nil || !rule.selectors[0].isJustParentSelector() {
			continue
		}
		rs.rules = append(rs.rules[:i:i], rs.rules[i+1:]...)
		i--
		for j := 0; j < len(rule.rules) && rule.rules[j] != nil; j++ {
			sub := rule.rules[j]
			sub.base().copyVisibilityInfo(rule.visibilityInfo())
			if d, isDecl := sub.(*Declaration); !isDecl || !d.variable {
				i++
				rs.rules = insertNode(rs.rules, i, sub)
			}
		}
	}
	ctx.frames = ctx.frames[1:]
	ctx.selectors = ctx.selectors[1:]
	for i := mediaBlockCount; i < len(ctx.mediaBlocks); i++ {
		if b, ok := ctx.mediaBlocks[i].(interface{ bubbleSelectors([]*Selector) }); ok {
			b.bubbleSelectors(selectors)
		}
	}
	return rs
}

func isEvalFirst(n Node) bool {
	switch v := n.(type) {
	case *MixinDefinition:
		return true
	case *DetachedRuleset:
		return true
	case *Ruleset:
		return v.evalFirst
	}
	return false
}

func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// spliceNodes replaces nodes[i] with repl.
func spliceNodes(nodes []Node, i int, repl []Node) []Node {
	out := make([]Node, 0, len(nodes)+len(repl))
	out = append(out, nodes[:i]...)
	out = append(out, repl...)
	return append(out, nodes[i+1:]...)
}

func insertNode(nodes []Node, i int, n Node) []Node {
	if i >= len(nodes) {
		return append(nodes, n)
	}
	nodes = append(nodes[:i+1], nodes[i:]...)
	nodes[i] = n
	return nodes
}

func (r *Ruleset) evalImports(ctx *evalContext) {
	if r.rules == nil {
		return
	}
	for i := 0; i < len(r.rules); i++ {
		imp, ok := r.rules[i].(*Import)
		if !ok {
			continue
		}
		res := imp.evalImport(ctx)
		if list, isList := res.(*nodeList); isList {
			r.rules = spliceNodes(r.rules, i, list.nodes)
			i += len(list.nodes) - 1
		} else {
			r.rules = spliceNodes(r.rules, i, []Node{res})
		}
		r.resetCache()
	}
}

func (r *Ruleset) makeImportant() Node {
	rules := make([]Node, len(r.rules))
	for i, n := range r.rules {
		if m, ok := n.(interface{ makeImportant() Node }); ok {
			rules[i] = m.makeImportant()
		} else {
			rules[i] = n
		}
	}
	return newRuleset(r.selectors, rules, r.strictImports, r.visibilityInfo())
}

func (r *Ruleset) matchArgs(args []mixinArg, _ *evalContext) bool { return len(args) == 0 }

func (r *Ruleset) matchCondition(args []mixinArg, ctx *evalContext) bool {
	last := r.selectors[len(r.selectors)-1]
	if !last.evaldCondition {
		return false
	}
	if last.condition != nil && !conditionValue(last.condition, ctx.copyEval(ctx.frames)) {
		return false
	}
	return true
}

func (r *Ruleset) resetCache() {
	r.variablesCache = nil
	r.propertiesCache = nil
	r.lookups = map[string][]mixinMatch{}
}

func (r *Ruleset) variablesMap() map[string]*Declaration {
	if r.variablesCache == nil {
		vars := map[string]*Declaration{}
		for _, n := range r.rules {
			if d, ok := n.(*Declaration); ok && d.variable {
				vars[d.name] = d
			}
			if imp, ok := n.(*Import); ok && imp.root != nil {
				if root, isFrame := imp.root.(frameNode); isFrame {
					for name := range root.variablesMap() {
						vars[name] = root.variable(name)
					}
				}
			}
		}
		r.variablesCache = vars
	}
	return r.variablesCache
}

func (r *Ruleset) propertiesMap() map[string][]*Declaration {
	if r.propertiesCache == nil {
		props := map[string][]*Declaration{}
		for _, n := range r.rules {
			d, ok := n.(*Declaration)
			if !ok || d.variable {
				continue
			}
			name := d.name
			if d.nameNodes != nil {
				if k, isKw := d.nameNodes[0].(*Keyword); isKw && len(d.nameNodes) == 1 {
					name = k.value
				} else {
					name = d.nameKey()
				}
			}
			props["$"+name] = append(props["$"+name], d)
		}
		r.propertiesCache = props
	}
	return r.propertiesCache
}

func (r *Ruleset) hasVariables() bool { return true }

func (r *Ruleset) hasProperties() bool { return true }

func (r *Ruleset) variable(name string) *Declaration {
	if d := r.variablesMap()[name]; d != nil {
		return rsParseValue(r, d)
	}
	return nil
}

func (r *Ruleset) property(name string) []*Declaration {
	ds := r.propertiesMap()[name]
	if ds == nil {
		return nil
	}
	out := make([]*Declaration, len(ds))
	for i, d := range ds {
		out[i] = rsParseValue(r, d)
	}
	return out
}

// rsParseValue is Ruleset.parseValue: an anonymous declaration value is parsed on first use.
func rsParseValue(_ frameNode, d *Declaration) *Declaration {
	a, ok := d.value.(*Anonymous)
	if !ok || a.preventReparse || d.parsed {
		return d
	}
	if a.valueNode != nil {
		d.parsed = true
		return d
	}
	p := newParser(nil, nil, fileInfoOf(d), getIndex(a))
	res, finished := p.parseNode(a.value, []string{"value", "important"})
	if !finished {
		d.parsed = true
		return d
	}
	d.value = res[0]
	d.important = ""
	if k, isKw := res[1].(*Keyword); isKw {
		d.important = k.value
	}
	d.parsed = true
	return d
}

func (r *Ruleset) rulesetsList() []Node {
	var out []Node
	for _, n := range r.rules {
		if n == nil {
			break
		}
		switch n.(type) {
		case *Ruleset, *MixinDefinition:
			out = append(out, n)
		}
	}
	return out
}

func (r *Ruleset) prependRule(rule Node) {
	r.rules = append([]Node{rule}, r.rules...)
	setParent(rule, r)
}

// mixinSelectorsOf returns the selectors of a ruleset-like rule.
func mixinSelectorsOf(n Node) []*Selector {
	switch v := n.(type) {
	case *Ruleset:
		return v.selectors
	case *MixinDefinition:
		return v.selectors
	}
	return nil
}

func (r *Ruleset) find(selector *Selector, self Node, filter func(Node) bool) []mixinMatch {
	if self == nil {
		self = r
	}
	key := toCSS(selector, &evalContext{})
	if cached, ok := r.lookups[key]; ok {
		return cached
	}
	var rules []mixinMatch
	for _, rule := range r.rulesetsList() {
		if rule == self {
			continue
		}
		for _, s := range mixinSelectorsOf(rule) {
			match := selector.match(s)
			if match == 0 {
				continue
			}
			if len(selector.elements) > match {
				if filter == nil || filter(rule) {
					var sub *Ruleset
					switch v := rule.(type) {
					case *Ruleset:
						sub = v
					case *MixinDefinition:
						sub = v.Ruleset
					}
					found := sub.find(newSelector(selector.elements[match:], nil, nil, 0, nil, nil), self, filter)
					for i := range found {
						found[i].path = append(found[i].path, rule)
					}
					rules = append(rules, found...)
				}
			} else {
				rules = append(rules, mixinMatch{rule: rule, path: []Node{}})
			}
			break
		}
	}
	r.lookups[key] = rules
	return rules
}

// isCharsetRule reports whether a rule is an @charset at-rule.
func isCharsetRule(n Node) bool {
	a, ok := n.(*AtRule)
	return ok && a.isCharset()
}

func (r *Ruleset) genCSS(ctx *evalContext, out *output) {
	if !r.root {
		ctx.tabLevel++
	}
	tabRule, tabSet := "", ""
	if !ctx.compress {
		tabRule = repeatStr("  ", ctx.tabLevel)
		tabSet = repeatStr("  ", ctx.tabLevel-1)
	}
	var ruleNodes []Node
	charsetIndex, importIndex := 0, 0
	for i := 0; i < len(r.rules) && r.rules[i] != nil; i++ {
		rule := r.rules[i]
		switch {
		case isComment(rule):
			if importIndex == i {
				importIndex++
			}
			ruleNodes = append(ruleNodes, rule)
		case isCharsetRule(rule):
			ruleNodes = insertNode(ruleNodes, charsetIndex, rule)
			charsetIndex++
			importIndex++
		case rule.Type() == "Import":
			ruleNodes = insertNode(ruleNodes, importIndex, rule)
			importIndex++
		default:
			ruleNodes = append(ruleNodes, rule)
		}
	}
	if !r.root {
		if dbg := debugInfoText(ctx, r.debugInfo, tabSet); dbg != "" {
			out.add(dbg)
			out.add(tabSet)
		}
		sep := ",\n" + tabSet
		if ctx.compress {
			sep = ","
		}
		if r.paths == nil {
			// a ruleset the selector visitors never reached has no paths to print
			readUndefined("length")
		}
		for i, path := range r.paths {
			if len(path) == 0 {
				continue
			}
			if i > 0 {
				out.add(sep)
			}
			ctx.firstSelector = true
			genCSS(path[0], ctx, out)
			ctx.firstSelector = false
			for _, s := range path[1:] {
				genCSS(s, ctx, out)
			}
		}
		if ctx.compress {
			out.add("{" + tabRule)
		} else {
			out.add(" {\n" + tabRule)
		}
	}
	for i := 0; i < len(ruleNodes) && ruleNodes[i] != nil; i++ {
		rule := ruleNodes[i]
		if i+1 == len(ruleNodes) {
			ctx.lastRule = true
		}
		current := ctx.lastRule
		if isRulesetLikeNode(rule) {
			ctx.lastRule = false
		}
		if _, ok := rule.(cssGenerator); ok {
			genCSS(rule, ctx, out)
		} else if s, isStr := stringValue(rule); isStr && s != "" {
			out.add(s)
		}
		ctx.lastRule = current
		if !ctx.lastRule && isTrue(rule.base().isVisible()) {
			if !ctx.compress {
				out.add("\n" + tabRule)
			}
		} else {
			ctx.lastRule = false
		}
	}
	if !r.root {
		if ctx.compress {
			out.add("}")
		} else {
			out.add("\n" + tabSet + "}")
		}
		ctx.tabLevel--
	}
	if !out.isEmpty() && !ctx.compress && r.firstRoot {
		out.add("\n")
	}
}

func isComment(n Node) bool {
	_, ok := n.(*Comment)
	return ok
}

func repeatStr(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func (r *Ruleset) joinSelectors(paths *[][]*Selector, context [][]*Selector, selectors []*Selector) {
	for _, s := range selectors {
		r.joinSelector(paths, context, s)
	}
}

func elemOf(n Node) *Element {
	e, ok := n.(*Element)
	if !ok {
		typeError("Cannot read properties of undefined (reading 'value')")
	}
	return e
}

func createParenthesis(elementsToPak []*Selector, original *Element) *Paren {
	if len(elementsToPak) == 0 {
		return newParen(nil)
	}
	inside := make([]Node, len(elementsToPak))
	for j, s := range elementsToPak {
		inside[j] = newElementNode(nil, s, original.isVariable, original.index, original.fileInfo)
	}
	return newParen(newSelector(inside, nil, nil, 0, nil, nil))
}

func createSelectorFrom(contained Node, original *Element) *Selector {
	el := newElementNode(nil, contained, original.isVariable, original.index, original.fileInfo)
	return newSelector([]Node{el}, nil, nil, 0, nil, nil)
}

func addReplacementIntoPath(beginning []*Selector, addPath []*Selector, replaced *Element, original *Selector) []*Selector {
	var newPath []*Selector
	var joined *Selector
	if len(beginning) > 0 {
		newPath = append([]*Selector{}, beginning...)
		last := newPath[len(newPath)-1]
		newPath = newPath[:len(newPath)-1]
		joined = original.createDerived(append([]Node{}, last.elements...), nil, nil)
	} else {
		joined = original.createDerived([]Node{}, nil, nil)
	}
	if len(addPath) > 0 {
		combinator := replaced.combinator
		parentEl := elemOf(addPath[0].elements[0])
		if combinator.emptyOrWhitespace && !parentEl.combinator.emptyOrWhitespace {
			combinator = parentEl.combinator
		}
		joined.elements = append(joined.elements, newElementVis(combinator, parentEl.value, parentEl.valueStr, replaced.isVariable, replaced.index, replaced.fileInfo, nil))
		joined.elements = append(joined.elements, addPath[0].elements[1:]...)
	}
	if len(joined.elements) != 0 {
		newPath = append(newPath, joined)
	}
	if len(addPath) > 1 {
		for _, s := range addPath[1:] {
			newPath = append(newPath, s.createDerived(s.elements, []Node{}, nil))
		}
	}
	return newPath
}

func addAllReplacementsIntoPath(beginning [][]*Selector, addPaths []*Selector, replaced *Element, original *Selector, result [][]*Selector) [][]*Selector {
	for _, b := range beginning {
		result = append(result, addReplacementIntoPath(b, addPaths, replaced, original))
	}
	return result
}

func mergeElementsOnToSelectors(elements []Node, selectors [][]*Selector) [][]*Selector {
	if len(elements) == 0 {
		return selectors
	}
	if len(selectors) == 0 {
		return append(selectors, []*Selector{newSelector(elements, nil, nil, 0, nil, nil)})
	}
	for i := 0; i < len(selectors); i++ {
		sel := selectors[i]
		if len(sel) > 0 {
			last := sel[len(sel)-1]
			sel[len(sel)-1] = last.createDerived(append(append([]Node{}, last.elements...), elements...), nil, nil)
		} else {
			selectors[i] = append(sel, newSelector(elements, nil, nil, 0, nil, nil))
		}
	}
	return selectors
}

func findNestedSelector(e *Element) *Selector {
	p, ok := e.value.(*Paren)
	if !ok {
		return nil
	}
	s, _ := p.value.(*Selector)
	return s
}

func replaceParentSelector(paths *[][]*Selector, context [][]*Selector, in *Selector) bool {
	var current []Node
	newSelectors := [][]*Selector{{}}
	hadParent := false
	for i := 0; i < len(in.elements) && in.elements[i] != nil; i++ {
		el := elemOf(in.elements[i])
		if !el.isParentRef() {
			nested := findNestedSelector(el)
			if nested == nil {
				current = append(current, el)
				continue
			}
			newSelectors = mergeElementsOnToSelectors(current, newSelectors)
			var nestedPaths [][]*Selector
			replaced := false
			var replacedNew [][]*Selector
			hasSub := false
			for _, e := range nested.elements {
				if _, ok := e.(*Selector); ok {
					hasSub = true
					break
				}
			}
			if hasSub {
				var resolved []Node
				for _, sub := range nested.elements {
					s, ok := sub.(*Selector)
					if !ok {
						resolved = append(resolved, sub)
						continue
					}
					var subPaths [][]*Selector
					subReplaced := replaceParentSelector(&subPaths, context, s)
					replaced = replaced || subReplaced
					if len(subPaths) > 0 && len(subPaths[0]) > 0 {
						resolved = append(resolved, subPaths[0][0])
					} else {
						resolved = append(resolved, s)
					}
				}
				hadParent = hadParent || replaced
				resolvedSel := newSelector(resolved, nil, nil, 0, nil, nil)
				replacement := createSelectorFrom(createParenthesis([]*Selector{resolvedSel}, el), el)
				replacedNew = addAllReplacementsIntoPath(newSelectors, []*Selector{replacement}, el, in, replacedNew)
			} else {
				replaced = replaceParentSelector(&nestedPaths, context, nested)
				hadParent = hadParent || replaced
				for _, np := range nestedPaths {
					replacement := createSelectorFrom(createParenthesis(np, el), el)
					replacedNew = addAllReplacementsIntoPath(newSelectors, []*Selector{replacement}, el, in, replacedNew)
				}
			}
			newSelectors = replacedNew
			current = nil
			continue
		}
		hadParent = true
		var multiplied [][]*Selector
		newSelectors = mergeElementsOnToSelectors(current, newSelectors)
		for _, sel := range newSelectors {
			if len(context) == 0 {
				if len(sel) > 0 {
					sel[0].elements = append(sel[0].elements, newElementVis(el.combinator, nil, "", el.isVariable, el.index, el.fileInfo, nil))
				}
				multiplied = append(multiplied, sel)
			} else {
				for _, c := range context {
					multiplied = append(multiplied, addReplacementIntoPath(sel, c, el, in))
				}
			}
		}
		newSelectors = multiplied
		current = nil
	}
	newSelectors = mergeElementsOnToSelectors(current, newSelectors)
	for i := range newSelectors {
		if n := len(newSelectors[i]); n > 0 {
			*paths = append(*paths, newSelectors[i])
			last := newSelectors[i][n-1]
			newSelectors[i][n-1] = last.createDerived(last.elements, in.extendList, nil)
		}
	}
	return hadParent
}

func (r *Ruleset) joinSelector(paths *[][]*Selector, context [][]*Selector, selector *Selector) {
	var newPaths [][]*Selector
	if !replaceParentSelector(&newPaths, context, selector) {
		if len(context) > 0 {
			newPaths = nil
			for _, c := range context {
				concatenated := make([]*Selector, 0, len(c)+1)
				for _, s := range c {
					d := s.createDerived(s.elements, s.extendList, &s.evaldCondition)
					d.copyVisibilityInfo(selector.visibilityInfo())
					concatenated = append(concatenated, d)
				}
				concatenated = append(concatenated, selector)
				newPaths = append(newPaths, concatenated)
			}
		} else {
			newPaths = [][]*Selector{{selector}}
		}
	}
	*paths = append(*paths, newPaths...)
}
