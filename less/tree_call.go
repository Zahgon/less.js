package less

import (
	"strings"

	"github.com/less/less.js/internal/jsre"
)

// Operation is tree/operation.js.
type Operation struct {
	nodeBase
	op         string
	operands   []Node
	isSpaced   bool
	parensInOp bool
}

func newOperation(op string, operands []Node, isSpaced bool) *Operation {
	return &Operation{op: strings.TrimSpace(op), operands: operands, isSpaced: isSpaced}
}

// Type is the node's type name, as the original's type getter reports it.
func (o *Operation) Type() string { return "Operation" }

func (o *Operation) setParensInOp() { o.parensInOp = true }

func (o *Operation) accept(v visitor) { o.operands = v.visitArray(o.operands) }

type colorOrDimension interface{ isOperand() }

func (d *Dimension) isOperand() {}
func (c *Color) isOperand()     {}

func (o *Operation) eval(ctx *evalContext) Node {
	a, b := evalNode(o.operands[0], ctx), evalNode(o.operands[1], ctx)
	if !ctx.isMathOn(o.op) {
		return newOperation(o.op, []Node{a, b}, o.isSpaced)
	}
	op := o.op
	if op == "./" {
		op = "/"
	}
	if d, ok := a.(*Dimension); ok {
		if _, isColor := b.(*Color); isColor {
			a = d.toColor()
		}
	}
	if d, ok := b.(*Dimension); ok {
		if _, isColor := a.(*Color); isColor {
			b = d.toColor()
		}
	}
	_, aok := a.(colorOrDimension)
	_, bok := b.(colorOrDimension)
	if !aok || !bok {
		_, aOp := a.(*Operation)
		_, bOp := b.(*Operation)
		if aOp || bOp {
			if ao, ok := a.(*Operation); ok && ao.op == "/" && ctx.math == MathParensDivision {
				return newOperation(o.op, []Node{a, b}, o.isSpaced)
			}
		}
		panic(&errorInfo{Type: "Operation", Message: "Operation on an invalid type"})
	}
	if d, ok := a.(*Dimension); ok {
		bd, isDim := b.(*Dimension)
		if !isDim {
			typeError("Cannot read properties of undefined (reading 'numerator')")
		}
		return d.operate(ctx, op, bd)
	}
	bc, isColor := b.(*Color)
	if !isColor {
		typeError("Cannot read properties of undefined (reading '0')")
	}
	return a.(*Color).operate(ctx, op, bc)
}

func (o *Operation) genCSS(ctx *evalContext, out *output) {
	genCSS(o.operands[0], ctx, out)
	if o.isSpaced {
		out.add(" ")
	}
	out.add(o.op)
	if o.isSpaced {
		out.add(" ")
	}
	genCSS(o.operands[1], ctx, out)
}

// Call is tree/call.js.
type Call struct {
	nodeBase
	name       string
	args       []Node
	calc       bool
	parensInOp bool
}

func newCall(name string, args []Node, index int, fi *FileInfo) *Call {
	c := &Call{name: name, args: args, calc: name == "calc"}
	c.index = index
	c.fileInfo = fi
	return c
}

// Type is the node's type name, as the original's type getter reports it.
func (c *Call) Type() string { return "Call" }

func (c *Call) setParensInOp() { c.parensInOp = true }

func (c *Call) accept(v visitor) {
	if c.args != nil {
		c.args = v.visitArray(c.args)
	}
}

func (c *Call) eval(ctx *evalContext) Node {
	currentMath := ctx.mathOn
	ctx.mathOn = !c.calc
	if c.calc || ctx.inCalc {
		ctx.enterCalc()
	}
	exitCalc := func() {
		if c.calc || ctx.inCalc {
			ctx.exitCalc()
		}
		ctx.mathOn = currentMath
	}
	fc := newFuncCaller(c.name, ctx, getIndex(c), fileInfoOf(c))
	if fc.isValid() {
		result := c.callFunction(fc)
		exitCalc()
		if result != nil {
			result.base().index = c.index
			result.base().fileInfo = c.fileInfo
			return result
		}
	}
	args := make([]Node, len(c.args))
	for i, a := range c.args {
		args[i] = evalNode(a, ctx)
	}
	exitCalc()
	return newCall(c.name, args, getIndex(c), fileInfoOf(c))
}

// callFunction runs a registered function and wraps a thrown error the way Call.eval does.
func (c *Call) callFunction(fc *funcCaller) (result Node) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		e, ok := r.(*errorInfo)
		if !ok {
			panic(r)
		}
		if e.hasLineColumn() {
			panic(e)
		}
		typ := e.Type
		if typ == "" {
			typ = "Runtime"
		}
		msg := "Error evaluating function `" + c.name + "`"
		if e.Message != "" {
			msg += ": " + e.Message
		}
		panic(&errorInfo{Type: typ, Message: msg, Index: intp(getIndex(c)), Filename: fileInfoOf(c).Filename, lineColumnSet: true})
	}()
	return fc.call(c.args)
}

func (c *Call) genCSS(ctx *evalContext, out *output) {
	out.addAt(c.name+"(", fileInfoOf(c), getIndex(c), false)
	for i, a := range c.args {
		genCSS(a, ctx, out)
		if i+1 < len(c.args) {
			out.add(", ")
		}
	}
	out.add(")")
}

// URL is tree/url.js.
type URL struct {
	nodeBase
	value   Node
	isEvald bool
}

func newURL(v Node, index int, fi *FileInfo, isEvald bool) *URL {
	u := &URL{value: v, isEvald: isEvald}
	u.index = index
	u.fileInfo = fi
	return u
}

// Type is the node's type name, as the original's type getter reports it.
func (u *URL) Type() string { return "Url" }

func (u *URL) cmpValue() any { return u.value }

func (u *URL) accept(v visitor) { u.value = v.visit(u.value) }

func (u *URL) genCSS(ctx *evalContext, out *output) {
	out.add("url(")
	genCSS(u.value, ctx, out)
	out.add(")")
}

var (
	reEscapePath = jsre.MustCompile(`[()'"\s]`, "g")
	reDataURI    = re(`^\s*data:`)
)

func escapePath(path string) string {
	return reEscapePath.ReplaceFunc(path, func(m string, _ []*string) string { return "\\" + m })
}

func (u *URL) eval(ctx *evalContext) Node {
	val := evalNode(u.value, ctx)
	if !u.isEvald {
		rootpath := fileInfoOf(u).Rootpath
		s, isString := stringValue(val)
		if isString && ctx.pathRequiresRewrite(s) {
			if q, ok := val.(*Quoted); !ok || q.quote == "" {
				rootpath = escapePath(rootpath)
			}
			s = ctx.rewritePath(s, rootpath)
		} else {
			if !isString {
				failStringValue(val, "split")
			}
			s = normalizePath(s)
		}
		if ctx.urlArgs != "" && !reDataURI.Test(s) {
			delimiter := "?"
			if strings.Contains(s, "?") {
				delimiter = "&"
			}
			args := delimiter + ctx.urlArgs
			if strings.Contains(s, "#") {
				s = strings.Replace(s, "#", args+"#", 1)
			} else {
				s += args
			}
		}
		setStringValue(val, s)
	}
	return newURL(val, getIndex(u), fileInfoOf(u), true)
}
