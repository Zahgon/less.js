package less

// Expression is tree/expression.js: a space separated list.
type Expression struct {
	nodeBase
	value      []Node
	noSpacing  bool
	parens     bool
	parensInOp bool
}

func newExpression(v []Node, noSpacing bool) *Expression {
	if v == nil {
		v = []Node{}
	}
	return &Expression{value: v, noSpacing: noSpacing}
}

func (e *Expression) Type() string { return "Expression" }

func (e *Expression) cmpValue() any { return e.value }

func (e *Expression) setParensInOp() { e.parensInOp = true }

func (e *Expression) accept(v visitor) { e.value = v.visitArray(e.value) }

func (e *Expression) eval(ctx *evalContext) Node {
	mathOn := ctx.isMathOn("")
	doubleParen := false
	if e.parens {
		ctx.inParenthesis()
	}
	var ret Node
	switch {
	case len(e.value) > 1:
		vals := make([]Node, len(e.value))
		for i, n := range e.value {
			vals[i] = evalNode(n, ctx)
		}
		ret = newExpression(vals, e.noSpacing)
	case len(e.value) == 1:
		if first, ok := e.value[0].(*Expression); ok && first.parens && !first.parensInOp && !ctx.inCalc {
			doubleParen = true
		}
		ret = evalNode(e.value[0], ctx)
	default:
		ret = e
	}
	if e.parens {
		ctx.outOfParenthesis()
	}
	if e.parens && e.parensInOp && !mathOn && !doubleParen {
		if _, isDim := ret.(*Dimension); !isDim {
			ret = newParen(ret)
		}
	}
	return ret
}

func (e *Expression) genCSS(ctx *evalContext, out *output) {
	for i, n := range e.value {
		genCSS(n, ctx, out)
		if !e.noSpacing && i+1 < len(e.value) {
			if a, ok := e.value[i+1].(*Anonymous); !ok || a.valueNode != nil || a.value != "," {
				out.add(" ")
			}
		}
	}
}

func (e *Expression) throwAwayComments() {
	kept := e.value[:0:0]
	for _, n := range e.value {
		if _, ok := n.(*Comment); !ok {
			kept = append(kept, n)
		}
	}
	e.value = kept
}

// Paren is tree/paren.js.
type Paren struct {
	nodeBase
	value      Node
	noSpacing  bool
	parensInOp bool
}

func newParen(n Node) *Paren { return &Paren{value: n} }

func (p *Paren) Type() string { return "Paren" }

func (p *Paren) cmpValue() any { return p.value }

func (p *Paren) setParensInOp() { p.parensInOp = true }

func (p *Paren) accept(v visitor) { p.value = v.visit(p.value) }

func (p *Paren) genCSS(ctx *evalContext, out *output) {
	out.add("(")
	genCSS(p.value, ctx, out)
	out.add(")")
}

func (p *Paren) eval(ctx *evalContext) Node {
	r := newParen(evalNode(p.value, ctx))
	if p.noSpacing {
		r.noSpacing = true
	}
	return r
}

// Negative is tree/negative.js.
type Negative struct {
	nodeBase
	value      Node
	parensInOp bool
}

func newNegative(n Node) *Negative { return &Negative{value: n} }

func (n *Negative) Type() string { return "Negative" }

func (n *Negative) cmpValue() any { return n.value }

func (n *Negative) setParensInOp() { n.parensInOp = true }

func (n *Negative) accept(v visitor) { n.value = v.visit(n.value) }

func (n *Negative) genCSS(ctx *evalContext, out *output) {
	out.add("-")
	genCSS(n.value, ctx, out)
}

func (n *Negative) eval(ctx *evalContext) Node {
	if ctx.isMathOn("*") {
		return newOperation("*", []Node{newDimension(-1, nil), n.value}, false).eval(ctx)
	}
	return newNegative(evalNode(n.value, ctx))
}

// Assignment is tree/assignment.js (`opacity=50` inside IE filters).
type Assignment struct {
	nodeBase
	key   string
	value Node
}

func newAssignment(key string, v Node) *Assignment { return &Assignment{key: key, value: v} }

func (a *Assignment) Type() string { return "Assignment" }

func (a *Assignment) cmpValue() any { return a.value }

func (a *Assignment) accept(v visitor) { a.value = v.visit(a.value) }

func (a *Assignment) eval(ctx *evalContext) Node {
	if _, ok := a.value.(evaler); ok {
		return newAssignment(a.key, evalNode(a.value, ctx))
	}
	return a
}

func (a *Assignment) genCSS(ctx *evalContext, out *output) {
	out.add(a.key + "=")
	genCSS(a.value, ctx, out)
}

// Attribute is tree/attribute.js (`[key op value cif]`); key and value are a string or a node.
type Attribute struct {
	nodeBase
	key      Node
	keyStr   string
	op       string
	value    Node
	valueStr string
	cif      string
}

func newAttribute(key Node, keyStr, op string, value Node, valueStr, cif string) *Attribute {
	return &Attribute{key: key, keyStr: keyStr, op: op, value: value, valueStr: valueStr, cif: cif}
}

func (a *Attribute) Type() string { return "Attribute" }

func (a *Attribute) eval(ctx *evalContext) Node {
	r := *a
	r.nodeBase = nodeBase{}
	if a.key != nil {
		r.key = evalNode(a.key, ctx)
	}
	if a.value != nil {
		r.value = evalNode(a.value, ctx)
	}
	return &r
}

func (a *Attribute) genCSS(ctx *evalContext, out *output) { out.add(a.toCSS(ctx)) }

func (a *Attribute) toCSS(ctx *evalContext) string {
	value := a.keyStr
	if a.key != nil {
		value = toCSS(a.key, ctx)
	}
	if a.op != "" {
		value += a.op
		switch {
		case a.value != nil:
			value += toCSS(a.value, ctx)
		case a.valueStr != "":
			value += a.valueStr
		default:
			typeError("Cannot read properties of undefined (reading 'toCSS')")
		}
	}
	if a.cif != "" {
		value += " " + a.cif
	}
	return "[" + value + "]"
}
