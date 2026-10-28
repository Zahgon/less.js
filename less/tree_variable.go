package less

import (
	"strings"

	"github.com/less/less.js/internal/js"
)

// frameNode is a scope on the frames stack: a Ruleset or a mixin definition.
type frameNode interface {
	Node
	variable(name string) *Declaration
	property(name string) []*Declaration
	hasVariables() bool
	hasProperties() bool
	registry() *functionRegistry
	variablesMap() map[string]*Declaration
}

// jsValueString is `${node.value}`: the template-literal string of a node's value property.
func jsValueString(n Node) string {
	switch v := n.(type) {
	case *Dimension:
		return js.NumberToString(v.value)
	case *Expression:
		return strings.TrimSuffix(strings.Repeat("[object Object],", len(v.value)), ",")
	case *Value:
		return strings.TrimSuffix(strings.Repeat("[object Object],", len(v.value)), ",")
	case *Color:
		if v.value == "" {
			return "undefined"
		}
		return v.value
	}
	if s, ok := stringValue(n); ok {
		return s
	}
	return "undefined"
}

// Variable is tree/variable.js.
type Variable struct {
	nodeBase
	name       string
	evaluating bool
	parensInOp bool
}

func newVariable(name string, index int, fi *FileInfo) *Variable {
	v := &Variable{name: name}
	v.index = index
	v.fileInfo = fi
	return v
}

// Type is the node's type name, as the original's type getter reports it.
func (v *Variable) Type() string { return "Variable" }

func (v *Variable) setParensInOp() { v.parensInOp = true }

func (v *Variable) eval(ctx *evalContext) Node {
	name := v.name
	if strings.HasPrefix(name, "@@") {
		inner := newVariable(js.Slice16(name, 1, js.Len16(name)), getIndex(v), fileInfoOf(v)).eval(ctx)
		name = "@" + jsValueString(inner)
	}
	if v.evaluating {
		panic(&errorInfo{Type: "Name", Message: "Recursive variable definition for " + name,
			Filename: fileInfoOf(v).Filename, Index: intp(getIndex(v))})
	}
	v.evaluating = true
	for _, f := range ctx.frames {
		fr, ok := f.(frameNode)
		if !ok {
			continue
		}
		d := fr.variable(name)
		if d == nil {
			continue
		}
		if d.important != "" {
			ctx.importantScope[len(ctx.importantScope)-1].important = d.important
		}
		var r Node
		if ctx.inCalc {
			r = newCall("_SELF", []Node{d.value}, 0, nil).eval(ctx)
		} else {
			r = evalNode(d.value, ctx)
		}
		if r != nil {
			v.evaluating = false
			return r
		}
	}
	panic(&errorInfo{Type: "Name", Message: "variable " + name + " is undefined",
		Filename: fileInfoOf(v).Filename, Index: intp(getIndex(v))})
}

// Property is tree/property.js (`$prop`).
type Property struct {
	nodeBase
	name       string
	evaluating bool
	parensInOp bool
}

func newProperty(name string, index int, fi *FileInfo) *Property {
	p := &Property{name: name}
	p.index = index
	p.fileInfo = fi
	return p
}

// Type is the node's type name, as the original's type getter reports it.
func (p *Property) Type() string { return "Property" }

func (p *Property) setParensInOp() { p.parensInOp = true }

func (p *Property) eval(ctx *evalContext) Node {
	if p.evaluating {
		panic(&errorInfo{Type: "Name", Message: "Recursive property reference for " + p.name,
			Filename: fileInfoOf(p).Filename, Index: intp(getIndex(p))})
	}
	p.evaluating = true
	for _, f := range ctx.frames {
		fr, ok := f.(frameNode)
		if !ok {
			continue
		}
		arr := fr.property(p.name)
		if arr == nil {
			continue
		}
		for i, d := range arr {
			arr[i] = newDeclaration(d.nameNodes, d.name, d.value, d.important, d.merge, getIndex(d), fileInfoOf(d), d.inline, &d.variable)
		}
		arr = mergeDeclarations(arr)
		last := arr[len(arr)-1]
		if last.important != "" {
			ctx.importantScope[len(ctx.importantScope)-1].important = last.important
		}
		if r := evalNode(last.value, ctx); r != nil {
			p.evaluating = false
			return r
		}
	}
	panic(&errorInfo{Type: "Name", Message: "Property '" + p.name + "' is undefined",
		Filename: fileInfoOf(p).Filename, Index: intp(getIndex(p))})
}

// VariableCall is tree/variable-call.js (`@detached();`).
type VariableCall struct {
	nodeBase
	variable string
}

func newVariableCall(name string, index int, fi *FileInfo) *VariableCall {
	c := &VariableCall{variable: name}
	c.index = index
	c.fileInfo = fi
	return c
}

// Type is the node's type name, as the original's type getter reports it.
func (c *VariableCall) Type() string { return "VariableCall" }

func (c *VariableCall) eval(ctx *evalContext) Node {
	r := newVariable(c.variable, getIndex(c), fileInfoOf(c)).eval(ctx)
	fail := func() {
		panic(newLessError(&errorInfo{Message: "Could not evaluate variable call " + c.variable}, nil, ""))
	}
	dr, ok := r.(*DetachedRuleset)
	if !ok {
		var rules *Ruleset
		switch x := r.(type) {
		case *Ruleset:
			rules = x
		case *MixinDefinition:
			rules = x.Ruleset
		case *nodeList:
			rules = newRuleset(nil, x.nodes, false, nil)
		case *Value:
			rules = newRuleset(nil, x.value, false, nil)
		case *Expression:
			rules = newRuleset(nil, x.value, false, nil)
		default:
			fail()
		}
		dr = newDetachedRuleset(rules, nil)
	}
	if dr.ruleset == nil {
		fail()
	}
	return dr.callEval(ctx)
}

// DetachedRuleset is tree/detached-ruleset.js.
type DetachedRuleset struct {
	nodeBase
	ruleset *Ruleset
	frames  []Node
}

func newDetachedRuleset(rs *Ruleset, frames []Node) *DetachedRuleset {
	d := &DetachedRuleset{ruleset: rs, frames: frames}
	if rs != nil {
		setParent(rs, d)
	}
	return d
}

// Type is the node's type name, as the original's type getter reports it.
func (d *DetachedRuleset) Type() string { return "DetachedRuleset" }

func (d *DetachedRuleset) accept(v visitor) {
	if rs, ok := v.visit(d.ruleset).(*Ruleset); ok {
		d.ruleset = rs
	}
}

func (d *DetachedRuleset) eval(ctx *evalContext) Node {
	frames := d.frames
	if frames == nil {
		frames = append([]Node{}, ctx.frames...)
	}
	return newDetachedRuleset(d.ruleset, frames)
}

func (d *DetachedRuleset) callEval(ctx *evalContext) *Ruleset {
	if d.frames != nil {
		return d.ruleset.eval(ctx.copyEval(append(append([]Node{}, d.frames...), ctx.frames...))).(*Ruleset)
	}
	return d.ruleset.eval(ctx).(*Ruleset)
}

// NamespaceValue is tree/namespace-value.js (`@map[key]`, `.mixin()[@v]`).
type NamespaceValue struct {
	nodeBase
	value   Node
	lookups []string
}

func newNamespaceValue(call Node, lookups []string, index int, fi *FileInfo) *NamespaceValue {
	n := &NamespaceValue{value: call, lookups: lookups}
	n.index = index
	n.fileInfo = fi
	return n
}

// Type is the node's type name, as the original's type getter reports it.
func (n *NamespaceValue) Type() string { return "NamespaceValue" }

func (n *NamespaceValue) cmpValue() any { return n.value }

func (n *NamespaceValue) accept(v visitor) { n.value = v.visit(n.value) }

func (n *NamespaceValue) eval(ctx *evalContext) Node {
	rules := evalNode(n.value, ctx)
	for _, name := range n.lookups {
		if list, ok := rules.(*nodeList); ok {
			rules = newRuleset([]*Selector{newSelector(nil, nil, nil, 0, nil, nil)}, list.nodes, false, nil)
		}
		rs, isFrame := rules.(frameNode)
		var cur Node
		switch {
		case name == "":
			if !isFrame {
				typeError("rs.lastDeclaration is not a function")
			}
			if d := lastDeclaration(rs); d != nil {
				cur = d
			}
		case strings.HasPrefix(name, "@"):
			if strings.HasPrefix(name, "@@") {
				name = "@" + jsValueString(newVariable(js.Slice16(name, 1, js.Len16(name)), 0, nil).eval(ctx))
			}
			if isFrame && rs.hasVariables() {
				if d := rs.variable(name); d != nil {
					cur = d
				}
			}
			if cur == nil {
				panic(&errorInfo{Type: "Name", Message: "variable " + name + " not found",
					Filename: fileInfoOf(n).Filename, Index: intp(getIndex(n))})
			}
		default:
			if strings.HasPrefix(name, "$@") {
				name = "$" + jsValueString(newVariable(js.Slice16(name, 1, js.Len16(name)), 0, nil).eval(ctx))
			} else if !strings.HasPrefix(name, "$") {
				name = "$" + name
			}
			var arr []*Declaration
			if isFrame && rs.hasProperties() {
				arr = rs.property(name)
			}
			if arr == nil {
				panic(&errorInfo{Type: "Name", Message: "property \"" + js.Slice16(name, 1, js.Len16(name)) + "\" not found",
					Filename: fileInfoOf(n).Filename, Index: intp(getIndex(n))})
			}
			cur = arr[len(arr)-1]
		}
		rules = cur
		if d, ok := cur.(*Declaration); ok {
			rules = d.eval(ctx).(*Declaration).value
		}
		if dr, ok := rules.(*DetachedRuleset); ok && dr.ruleset != nil {
			rules = dr.ruleset.eval(ctx)
		}
	}
	return rules
}

// nodeList is a JavaScript array returned where a node is expected, such as a mixin call's evaluated rules.
type nodeList struct {
	nodeBase
	nodes []Node
}

// Type is the node's type name, as the original's type getter reports it.
func (l *nodeList) Type() string { return "" }

func lastDeclaration(rs frameNode) *Declaration {
	var rules []Node
	switch r := rs.(type) {
	case *Ruleset:
		rules = r.rules
	case *MixinDefinition:
		rules = r.rules
	}
	for i := len(rules); i > 0; i-- {
		if d, ok := rules[i-1].(*Declaration); ok {
			return rsParseValue(rs, d)
		}
	}
	return nil
}
