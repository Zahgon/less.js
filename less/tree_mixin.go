package less

import (
	"strings"

	"github.com/less/less.js/internal/js"
)

// MixinCall is tree/mixin-call.js.
type MixinCall struct {
	nodeBase
	selector  *Selector
	arguments []mixinArg
	important bool
}

func newMixinCall(elements []*Element, args []mixinArg, index int, fi *FileInfo, important bool) *MixinCall {
	m := &MixinCall{selector: newSelector(elementNodes(elements), nil, nil, 0, nil, nil), arguments: args, important: important}
	m.index = index
	m.fileInfo = fi
	setParent(m.selector, m)
	return m
}

// Type is the node's type name, as the original's type getter reports it.
func (m *MixinCall) Type() string { return "MixinCall" }

func (m *MixinCall) accept(v visitor) {
	if m.selector != nil {
		if s, ok := v.visit(m.selector).(*Selector); ok {
			m.selector = s
		}
	}
	if _, generic := v.(*Visitor); generic {
		for _, a := range m.arguments {
			if a.value != nil {
				v.visitAny(a.value)
			}
		}
	}
}

func (m *MixinCall) eval(ctx *evalContext) Node { return &nodeList{nodes: m.evalRules(ctx)} }

const (
	defFalseEitherCase = -1
	defNone            = 0
	defTrue            = 1
	defFalse           = 2
)

type mixinCandidate struct {
	mixin Node
	group int
}

// matcher is a mixin candidate: a mixin definition or a plain ruleset used as a mixin.
type matcher interface {
	matchArgs(args []mixinArg, ctx *evalContext) bool
	matchCondition(args []mixinArg, ctx *evalContext) bool
}

func (m *MixinCall) evalRules(ctx *evalContext) []Node {
	m.selector = m.selector.eval(ctx).(*Selector)
	var args []mixinArg
	for _, a := range m.arguments {
		v := evalNode(a.value, ctx)
		if a.expand {
			if list, ok := arrayValue(v); ok {
				for _, x := range list {
					args = append(args, mixinArg{value: x})
				}
				continue
			}
		}
		if e, ok := v.(*Expression); ok && len(e.value) == 0 {
			continue
		}
		args = append(args, mixinArg{name: a.name, value: v})
	}
	conditionResult := [2]bool{}
	calcDefGroup := func(mixin Node, path []Node) int {
		for f := 0; f < 2; f++ {
			conditionResult[f] = true
			defaultFn.setValue(f == 1)
			for p := 0; p < len(path) && conditionResult[f]; p++ {
				if ns, ok := path[p].(matcher); ok {
					conditionResult[f] = conditionResult[f] && ns.matchCondition(nil, ctx)
				}
			}
			if mc, ok := mixin.(matcher); ok {
				conditionResult[f] = conditionResult[f] && mc.matchCondition(args, ctx)
			}
		}
		if conditionResult[0] || conditionResult[1] {
			if conditionResult[0] != conditionResult[1] {
				if conditionResult[1] {
					return defTrue
				}
				return defFalse
			}
			return defNone
		}
		return defFalseEitherCase
	}
	noArgs := func(rule Node) bool {
		mc, ok := rule.(matcher)
		return ok && mc.matchArgs(nil, ctx)
	}
	isOneFound := false
	var candidates []mixinCandidate
	var rules []Node
	match := false
	for i := 0; i < len(ctx.frames); i++ {
		var finder interface {
			find(*Selector, Node, func(Node) bool) []mixinMatch
		}
		switch f := ctx.frames[i].(type) {
		case *Ruleset:
			finder = f
		case *MixinDefinition:
			finder = f.Ruleset
		default:
			typeError("context.frames[i].find is not a function")
		}
		mixins := finder.find(m.selector, nil, noArgs)
		if len(mixins) == 0 {
			continue
		}
		isOneFound = true
		for _, mm := range mixins {
			recursive := false
			if _, isDef := mm.rule.(*MixinDefinition); !isDef {
				for _, fr := range ctx.frames {
					if mm.rule == originalOf(fr) {
						recursive = true
						break
					}
				}
			}
			if recursive {
				continue
			}
			if mm.rule.(matcher).matchArgs(args, ctx) {
				c := mixinCandidate{mixin: mm.rule, group: calcDefGroup(mm.rule, mm.path)}
				if c.group != defFalseEitherCase {
					candidates = append(candidates, c)
				}
				match = true
			}
		}
		defaultFn.reset()
		count := [3]int{}
		for _, c := range candidates {
			count[c.group]++
		}
		defaultResult := defTrue
		if count[defNone] > 0 {
			defaultResult = defFalse
		} else if count[defTrue]+count[defFalse] > 1 {
			panic(&errorInfo{Type: "Runtime", Message: "Ambiguous use of `default()` found when matching for `" + m.format(args) + "`",
				Index: intp(getIndex(m)), Filename: fileInfoOf(m).Filename})
		}
		for _, c := range candidates {
			if c.group == defNone || c.group == defaultResult {
				rules = append(rules, m.callCandidate(ctx, c.mixin, args)...)
			}
		}
		if match {
			return rules
		}
	}
	if isOneFound {
		panic(&errorInfo{Type: "Runtime", Message: "No matching definition was found for `" + m.format(args) + "`",
			Index: intp(getIndex(m)), Filename: fileInfoOf(m).Filename})
	}
	panic(&errorInfo{Type: "Name", Message: js.Trim(toCSS(m.selector, &evalContext{})) + " is undefined",
		Index: intp(getIndex(m)), Filename: fileInfoOf(m).Filename})
}

// callCandidate evaluates one matched mixin; a thrown error is re-thrown at the call's position.
func (m *MixinCall) callCandidate(ctx *evalContext, mixin Node, args []mixinArg) (rules []Node) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*errorInfo)
			if !ok {
				panic(r)
			}
			c := *e
			c.isLess = false
			if e.jsError {
				c = errorInfo{Message: "undefined", lineColumnSet: e.lineColumnSet}
			}
			c.Index = intp(getIndex(m))
			c.Filename = fileInfoOf(m).Filename
			panic(&c)
		}
	}()
	def, ok := mixin.(*MixinDefinition)
	if !ok {
		rs := mixin.(*Ruleset)
		original := Node(rs)
		if rs.originalRuleset != nil {
			original = rs.originalRuleset
		}
		def = newMixinDefinition("", nil, rs.rules, nil, false, nil, original.base().visibilityInfo())
		def.originalRuleset = original
	}
	newRules := def.evalCall(ctx, args, m.important).rules
	if m.blocksVisibility() {
		for _, r := range newRules {
			r.base().addVisibilityBlock()
		}
	}
	return newRules
}

func (m *MixinCall) format(args []mixinArg) string {
	parts := make([]string, len(args))
	for i, a := range args {
		s := ""
		if a.name != "" {
			s += a.name + ":"
		}
		s += toCSS(a.value, &evalContext{})
		parts[i] = s
	}
	return js.Trim(toCSS(m.selector, &evalContext{})) + "(" + strings.Join(parts, ", ") + ")"
}

// arrayValue returns a node's value when it is an array (Expression and Value lists).
func arrayValue(n Node) ([]Node, bool) {
	switch v := n.(type) {
	case *Expression:
		return v.value, true
	case *Value:
		return v.value, true
	}
	return nil, false
}

// originalOf is `frame.originalRuleset || frame`.
func originalOf(fr Node) Node {
	switch v := fr.(type) {
	case *Ruleset:
		if v.originalRuleset != nil {
			return v.originalRuleset
		}
	case *MixinDefinition:
		if v.originalRuleset != nil {
			return v.originalRuleset
		}
	}
	return fr
}

// MixinDefinition is tree/mixin-definition.js; it extends Ruleset. drRules holds the block of a detached
// ruleset with parameters (`#(@v, @k) {...}`), which the original stores in place of the rules array.
type MixinDefinition struct {
	*Ruleset
	name      string
	params    []mixinArg
	condition Node
	variadic  bool
	arity     int
	required  int
	frames    []Node
	drRules   *Ruleset
}

func newMixinDefinition(name string, params []mixinArg, rules []Node, cond Node, variadic bool, frames []Node, vis *visibilityInfo) *MixinDefinition {
	label := name
	if label == "" {
		label = "anonymous mixin"
	}
	rs := newRuleset(nil, nil, false, nil)
	rs.rules = rules
	md := &MixinDefinition{Ruleset: rs, name: label, params: params, condition: cond, variadic: variadic,
		arity: len(params), frames: frames}
	rs.selectors = []*Selector{newSelector([]Node{newElementStr(nil, name, false, 0, nil)}, nil, nil, 0, nil, nil)}
	for _, p := range params {
		if p.name == "" || p.value == nil {
			md.required++
		}
	}
	md.copyVisibilityInfo(vis)
	md.evalFirst = true
	return md
}

// Type is the node's type name, as the original's type getter reports it.
func (md *MixinDefinition) Type() string { return "MixinDefinition" }

func (md *MixinDefinition) base() *nodeBase { return &md.Ruleset.nodeBase }

func (md *MixinDefinition) accept(v visitor) {
	if _, generic := v.(*Visitor); generic {
		for _, p := range md.params {
			if p.value != nil {
				v.visitAny(p.value)
			}
		}
	}
	md.rules = v.visitArray(md.rules)
	if md.condition != nil {
		md.condition = v.visit(md.condition)
	}
}

func (md *MixinDefinition) evalParams(ctx, mixinEnv *evalContext, args []mixinArg, evaldArguments []Node) *Ruleset {
	frame := newRuleset(nil, nil, false, nil)
	if len(mixinEnv.frames) > 0 {
		if fr, ok := mixinEnv.frames[0].(frameNode); ok && fr.registry() != nil {
			frame.functionRegistry = fr.registry().inherit()
		}
	}
	mixinEnv = mixinEnv.copyEval(append([]Node{frame}, mixinEnv.frames...))
	argsLength := 0
	if args != nil {
		args = append([]mixinArg{}, args...)
		argsLength = len(args)
		for i := 0; i < argsLength; i++ {
			name := args[i].name
			if name == "" {
				continue
			}
			found := false
			for j := range md.params {
				if evaldArguments[j] == nil && name == md.params[j].name {
					evaldArguments[j] = evalNode(args[i].value, ctx)
					frame.prependRule(newDeclaration(nil, name, evalNode(args[i].value, ctx), "", "", 0, nil, false, nil))
					found = true
					break
				}
			}
			if !found {
				panic(&errorInfo{Type: "Runtime", Message: "Named argument for " + md.name + " " + args[i].name + " not found"})
			}
			args = append(args[:i:i], args[i+1:]...)
			i--
			argsLength = len(args)
		}
		argsLength = len(args)
	}
	argIndex := 0
	for i, param := range md.params {
		if evaldArguments[i] != nil {
			continue
		}
		var arg *mixinArg
		if args != nil && argIndex < len(args) {
			arg = &args[argIndex]
		}
		if param.name != "" {
			if param.variadic {
				var varargs []Node
				for j := argIndex; j < argsLength; j++ {
					varargs = append(varargs, evalNode(args[j].value, ctx))
				}
				frame.prependRule(newDeclaration(nil, param.name, newExpression(varargs, false).eval(ctx), "", "", 0, nil, false, nil))
			} else {
				var val Node
				switch {
				case arg != nil && arg.value != nil:
					if list, ok := arg.value.(*nodeList); ok {
						val = newDetachedRuleset(newRuleset(nil, list.nodes, false, nil), nil)
					} else {
						val = evalNode(arg.value, ctx)
					}
				case param.value != nil:
					val = evalNode(param.value, mixinEnv)
					frame.resetCache()
				default:
					panic(&errorInfo{Type: "Runtime", Message: "wrong number of arguments for " + md.name + " (" +
						itoa(argsLength) + " for " + itoa(md.arity) + ")"})
				}
				frame.prependRule(newDeclaration(nil, param.name, val, "", "", 0, nil, false, nil))
				evaldArguments[i] = val
			}
		}
		if param.variadic && args != nil {
			for j := argIndex; j < argsLength; j++ {
				evaldArguments[j] = evalNode(args[j].value, ctx)
			}
		}
		argIndex++
	}
	return frame
}

func (md *MixinDefinition) makeImportant() Node {
	var rules []Node
	if md.rules != nil {
		rules = make([]Node, len(md.rules))
		for i, r := range md.rules {
			if mi, ok := r.(interface{ makeImportant() Node }); ok {
				rules[i] = mi.makeImportant()
			} else {
				rules[i] = r
			}
		}
	}
	return newMixinDefinition(md.rawName(), md.params, rules, md.condition, md.variadic, md.frames, nil)
}

// rawName undoes the "anonymous mixin" label, which the constructor applies again.
func (md *MixinDefinition) rawName() string {
	if md.name == "anonymous mixin" {
		return ""
	}
	return md.name
}

func (md *MixinDefinition) eval(ctx *evalContext) Node {
	frames := md.frames
	if frames == nil {
		frames = append([]Node{}, ctx.frames...)
	}
	n := newMixinDefinition(md.rawName(), md.params, md.rules, md.condition, md.variadic, frames, nil)
	n.drRules = md.drRules
	return n
}

func (md *MixinDefinition) mixinFrames(ctx *evalContext) []Node {
	if md.frames != nil {
		return append(append([]Node{}, md.frames...), ctx.frames...)
	}
	return ctx.frames
}

func (md *MixinDefinition) evalCall(ctx *evalContext, args []mixinArg, important bool) *Ruleset {
	evaldArgs := make([]Node, max(len(md.params), len(args)))
	mixinFrames := md.mixinFrames(ctx)
	frame := md.evalParams(ctx, ctx.copyEval(mixinFrames), args, evaldArgs)
	var argNodes []Node
	for _, a := range evaldArgs {
		if a != nil {
			argNodes = append(argNodes, a)
		}
	}
	frame.prependRule(newDeclaration(nil, "@arguments", newExpression(argNodes, false).eval(ctx), "", "", 0, nil, false, nil))
	rules := append([]Node{}, md.rules...)
	rs := newRuleset(nil, rules, false, nil)
	rs.originalRuleset = md
	res := rs.eval(ctx.copyEval(append([]Node{md, frame}, mixinFrames...))).(*Ruleset)
	if important {
		res = res.makeImportant().(*Ruleset)
	}
	return res
}

func (md *MixinDefinition) matchCondition(args []mixinArg, ctx *evalContext) bool {
	if md.condition == nil {
		return true
	}
	params := md.evalParams(ctx, ctx.copyEval(md.mixinFrames(ctx)), args, make([]Node, max(len(md.params), len(args))))
	frames := append([]Node{params}, md.frames...)
	frames = append(frames, ctx.frames...)
	return conditionValue(md.condition, ctx.copyEval(frames))
}

func (md *MixinDefinition) matchArgs(args []mixinArg, ctx *evalContext) bool {
	evald := make([]*mixinArg, len(md.params))
	var positional []*mixinArg
	for i := range args {
		a := &args[i]
		if a.name == "" {
			positional = append(positional, a)
			continue
		}
		idx := -1
		for j, p := range md.params {
			if p.name == a.name && evald[j] == nil {
				idx = j
				break
			}
		}
		if idx < 0 {
			return false
		}
		evald[idx] = a
	}
	pi := 0
	for i, p := range md.params {
		if evald[i] != nil {
			continue
		}
		if p.variadic {
			pi = len(positional)
			continue
		}
		if pi < len(positional) {
			evald[i] = positional[pi]
			pi++
		} else if p.name == "" || p.value == nil {
			return false
		}
	}
	if pi < len(positional) {
		return false
	}
	for i := 0; i < md.arity; i++ {
		p := md.params[i]
		if p.name == "" && !p.variadic {
			if toCSS(evalNode(evald[i].value, ctx), &evalContext{}) != toCSS(evalNode(p.value, ctx), &evalContext{}) {
				return false
			}
		}
	}
	return true
}
