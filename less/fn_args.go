package less

import (
	"math"

	"github.com/less/less.js/internal/js"
)

// Built-in functions receive JavaScript values: a missing argument is undefined, and reading a property of it, or
// calling a method a node does not have, raises the runtime's TypeError. These helpers keep those messages.

func argAt(args []Node, i int) Node {
	if i < len(args) {
		return args[i]
	}
	return nil
}

func readUndefined(prop string) {
	typeError("Cannot read properties of undefined (reading '" + prop + "')")
}

// jsError raises `new Error(msg)`.
func jsError(msg string) { panic(&errorInfo{Message: msg, jsError: true}) }

// argumentError raises `{ type: 'Argument', message }`.
func argumentError(msg string) { panic(&errorInfo{Type: "Argument", Message: msg}) }

// valueOf returns ToNumber(node.value) for arithmetic on an argument's value.
func valueOf(n Node) float64 {
	switch v := n.(type) {
	case nil:
		readUndefined("value")
	case *Dimension:
		return v.value
	case *Expression:
		return arrayToNumber(v.value)
	case *Value:
		return arrayToNumber(v.value)
	case *jsBool:
		return math.NaN()
	}
	if s, ok := stringValue(n); ok {
		return js.StringToNumber(s)
	}
	return math.NaN()
}

// arrayToNumber is ToNumber of an array of nodes: [] is 0, one node converts through its string form.
func arrayToNumber(ns []Node) float64 {
	switch len(ns) {
	case 0:
		return 0
	case 1:
		return js.StringToNumber("[object Object]")
	}
	return math.NaN()
}

// colorArg returns a Color argument for code that reads color.rgb, color.alpha and calls color methods.
func colorArg(n Node, member string) *Color {
	switch c := n.(type) {
	case *Color:
		return c
	case nil:
		readUndefined(member)
	}
	return nil
}

// colorMethod is `color.<method>()` on an argument.
func colorMethod(n Node, name, method string) *Color {
	c := colorArg(n, method)
	if c == nil {
		typeError(name + "." + method + " is not a function")
	}
	return c
}

// rgbOf is `color.rgb` followed by an index read.
func rgbOf(n Node, idx int) float64 {
	c := colorArg(n, "rgb")
	if c == nil {
		readUndefined(itoa(idx))
	}
	return c.rgb[idx]
}

// toHSLArg is the color functions' toHSL helper.
func toHSLArg(n Node) hsla {
	c := colorArg(n, "toHSL")
	if c == nil {
		jsError("Argument cannot be evaluated to a color")
	}
	return c.toHSL()
}

func toHSVArg(n Node) hsva {
	c := colorArg(n, "toHSV")
	if c == nil {
		jsError("Argument cannot be evaluated to a color")
	}
	return c.toHSV()
}

// colorNumber is the color functions' number helper; percentages are fractions.
func colorNumber(n Node) float64 {
	if d, ok := n.(*Dimension); ok {
		if d.unit.is("%") {
			return d.value / 100
		}
		return d.value
	}
	if f, ok := n.(*jsNumber); ok {
		return f.v
	}
	panic(&errorInfo{Type: "Argument", Message: "color functions take numbers as parameters"})
}

func colorScaled(n Node, size float64) float64 {
	if d, ok := n.(*Dimension); ok && d.unit.is("%") {
		return d.value * size / 100
	}
	return colorNumber(n)
}

// jsNumber is a plain JavaScript number passed between built-ins (hsva calls rgba with numbers).
type jsNumber struct {
	nodeBase
	v float64
}

func (n *jsNumber) Type() string { return "" }

func num(v float64) Node { return &jsNumber{v: v} }

func clamp01(v float64) float64 { return math.Min(1, math.Max(0, v)) }

// jsMod is the JavaScript % operator on numbers.
func jsMod(a, b float64) float64 { return math.Mod(a, b) }
