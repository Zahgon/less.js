package less

import (
	"strings"
)

// jsBool is a JavaScript boolean that flows through the tree: Condition.eval returns one, and functions such as
// boolean() and if() receive it as an argument.
type jsBool struct {
	nodeBase
	v bool
}

func (b *jsBool) Type() string { return "" }

func (b *jsBool) cmpValue() any { return nil }

// truthy is JavaScript truthiness of an evaluated condition or node.
func truthy(n Node) bool {
	if n == nil {
		return false
	}
	if b, ok := n.(*jsBool); ok {
		return b.v
	}
	return true
}

func conditionValue(n Node, ctx *evalContext) bool { return truthy(evalNode(n, ctx)) }

// Condition is tree/condition.js.
type Condition struct {
	nodeBase
	op     string
	lvalue Node
	rvalue Node
	negate bool
}

func newCondition(op string, l, r Node, index int, negate bool) *Condition {
	c := &Condition{op: strings.TrimSpace(op), lvalue: l, rvalue: r, negate: negate}
	c.index = index
	return c
}

func (c *Condition) Type() string { return "Condition" }

func (c *Condition) accept(v visitor) {
	c.lvalue = v.visit(c.lvalue)
	c.rvalue = v.visit(c.rvalue)
}

func (c *Condition) eval(ctx *evalContext) Node {
	a := evalNode(c.lvalue, ctx)
	b := evalNode(c.rvalue, ctx)
	var result bool
	switch c.op {
	case "and":
		result = truthy(a) && truthy(b)
	case "or":
		result = truthy(a) || truthy(b)
	default:
		switch compareNodes(a, b) {
		case -1:
			result = c.op == "<" || c.op == "=<" || c.op == "<="
		case 0:
			result = c.op == "=" || c.op == ">=" || c.op == "=<" || c.op == "<="
		case 1:
			result = c.op == ">" || c.op == ">="
		}
	}
	if c.negate {
		result = !result
	}
	return &jsBool{v: result}
}

// QueryInParens is tree/query-in-parens.js (a media range such as `(400px <= width < 700px)`).
type QueryInParens struct {
	nodeBase
	op     string
	lvalue Node
	mvalue Node
	op2    string
	rvalue Node
}

func newQueryInParens(op string, l, m Node, op2 string, r Node, index int) *QueryInParens {
	q := &QueryInParens{op: strings.TrimSpace(op), lvalue: l, mvalue: m, op2: strings.TrimSpace(op2), rvalue: r}
	q.index = index
	return q
}

func (q *QueryInParens) Type() string { return "QueryInParens" }

func (q *QueryInParens) accept(v visitor) {
	q.lvalue = v.visit(q.lvalue)
	q.mvalue = v.visit(q.mvalue)
	if q.rvalue != nil {
		q.rvalue = v.visit(q.rvalue)
	}
}

func (q *QueryInParens) eval(ctx *evalContext) Node {
	if q.lvalue == nil || q.mvalue == nil {
		typeError("Cannot read properties of undefined (reading 'eval')")
	}
	var r Node
	if q.rvalue != nil {
		r = evalNode(q.rvalue, ctx)
	}
	return newQueryInParens(q.op, evalNode(q.lvalue, ctx), evalNode(q.mvalue, ctx), q.op2, r, q.index)
}

func (q *QueryInParens) genCSS(ctx *evalContext, out *output) {
	genCSS(q.lvalue, ctx, out)
	out.add(" " + q.op + " ")
	genCSS(q.mvalue, ctx, out)
	if q.rvalue != nil {
		out.add(" " + q.op2 + " ")
		genCSS(q.rvalue, ctx, out)
	}
}

var extendNextID = 0

// Extend is tree/extend.js.
type Extend struct {
	nodeBase
	selector                      *Selector
	option                        string
	objectID                      int
	parentIDs                     []int
	allowBefore                   bool
	allowAfter                    bool
	selfSelectors                 []*Selector
	ruleset                       *Ruleset
	firstExtendOnThisSelectorPath bool
	hasFoundMatches               bool
}

func newExtend(sel *Selector, option string, index int, fi *FileInfo) *Extend {
	return newExtendVis(sel, option, index, fi, nil)
}

func newExtendVis(sel *Selector, option string, index int, fi *FileInfo, vis *visibilityInfo) *Extend {
	e := &Extend{selector: sel, option: option, objectID: extendNextID}
	extendNextID++
	e.parentIDs = []int{e.objectID}
	e.index = index
	e.fileInfo = fi
	e.copyVisibilityInfo(vis)
	if option == "!all" || option == "all" {
		e.allowBefore, e.allowAfter = true, true
	}
	setParent(sel, e)
	return e
}

func (e *Extend) Type() string { return "Extend" }

func (e *Extend) accept(v visitor) {
	if s, ok := v.visit(e.selector).(*Selector); ok {
		e.selector = s
	}
}

func (e *Extend) eval(ctx *evalContext) Node {
	return newExtendVis(e.selector.eval(ctx).(*Selector), e.option, getIndex(e), fileInfoOf(e), e.visibilityInfo())
}

func (e *Extend) clone() *Extend {
	return newExtendVis(e.selector, e.option, getIndex(e), fileInfoOf(e), e.visibilityInfo())
}

func (e *Extend) findSelfSelectors(selectors []*Selector) {
	var self []Node
	for i, s := range selectors {
		if i > 0 && len(s.elements) > 0 {
			if el, ok := s.elements[0].(*Element); ok && el.combinator.value == "" {
				el.combinator.value = " "
			}
		}
		self = append(self, s.elements...)
	}
	sel := newSelector(self, nil, nil, 0, nil, nil)
	sel.copyVisibilityInfo(e.visibilityInfo())
	e.selfSelectors = []*Selector{sel}
}

// JavaScript is tree/javascript.js. Inline JavaScript cannot run in this port: with the option off it raises the
// original error, and with it on evaluation fails with an explicit error.
type JavaScript struct {
	nodeBase
	escaped    bool
	expression string
}

func newJavaScript(s string, escaped bool, index int, fi *FileInfo) *JavaScript {
	j := &JavaScript{escaped: escaped, expression: s}
	j.index = index
	j.fileInfo = fi
	return j
}

func (j *JavaScript) Type() string { return "JavaScript" }

func (j *JavaScript) eval(ctx *evalContext) Node {
	msg := "Inline JavaScript is not enabled. Is it set in your options?"
	if ctx.javascriptEnabled {
		msg = "JavaScript evaluation error: inline JavaScript is not supported by this implementation"
	}
	panic(&errorInfo{Message: msg, Filename: fileInfoOf(j).Filename, Index: intp(getIndex(j))})
}
