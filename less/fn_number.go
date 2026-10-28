package less

import (
	"math"
	"strings"

	"github.com/less/less.js/internal/js"
)

// colour blending (functions/color-blending.js), number and math functions (number.js, math.js, math-helper.js).

func colorBlend(mode func(cb, cs float64) float64) func(*funcCaller, []Node) Node {
	return func(fc *funcCaller, a []Node) Node {
		c1, c2 := colorArg(argAt(a, 0), "alpha"), colorArg(argAt(a, 1), "alpha")
		ab, as := math.NaN(), math.NaN()
		if c1 != nil {
			ab = c1.alpha
		}
		if c2 != nil {
			as = c2.alpha
		}
		ar := as + ab*(1-as)
		r := make([]float64, 3)
		for i := 0; i < 3; i++ {
			cb, cs := rgbOf(a[0], i)/255, rgbOf(a[1], i)/255
			cr := mode(cb, cs)
			if ar != 0 && !math.IsNaN(ar) {
				cr = (as*cs + ab*(cb-as*(cb+cs-cr))) / ar
			}
			r[i] = cr * 255
		}
		return newColorRGB(r, ar, "")
	}
}

func blendMultiply(cb, cs float64) float64 { return cb * cs }
func blendScreen(cb, cs float64) float64   { return cb + cs - cb*cs }
func blendOverlay(cb, cs float64) float64 {
	cb *= 2
	if cb <= 1 {
		return blendMultiply(cb, cs)
	}
	return blendScreen(cb-1, cs)
}

func blendSoftlight(cb, cs float64) float64 {
	d, e := 1.0, cb
	if cs > 0.5 {
		e = 1
		if cb > 0.25 {
			d = math.Sqrt(cb)
		} else {
			d = ((16*cb-12)*cb + 4) * cb
		}
	}
	return cb - (1-2*cs)*e*(d-cb)
}

// mathHelper is functions/math-helper.js; unit "" means "unitless result after unify", nil keeps the unit.
func mathHelper(fn func(float64) float64, unit *string, cssEvaluable bool, n Node) Node {
	d, ok := n.(*Dimension)
	if !ok {
		if _, isCall := n.(*Call); cssEvaluable && isCall {
			return nil
		}
		panic(&errorInfo{Type: "Argument", Message: "argument must be a number"})
	}
	var u *Unit
	if unit == nil {
		u = d.unit
	} else {
		d = d.unify()
		u = newUnit(unitList(*unit), nil, "")
	}
	return newDimension(fn(d.value), u)
}

func unitList(u string) []string {
	if u == "" {
		return nil
	}
	return []string{u}
}

func strp(s string) *string { return &s }

func minMax(fc *funcCaller, isMin bool, args []Node) Node {
	if len(args) == 0 {
		argumentError("one or more arguments required")
	}
	var order []*Dimension
	values := map[string]int{}
	var unitStatic, unitClone *string
	for i := 0; i < len(args); i++ {
		cur, ok := args[i].(*Dimension)
		if !ok {
			if list, isList := arrayValue(args[i]); isList {
				args = append(args, list...)
				continue
			}
			argumentError("incompatible types")
		}
		var curU *Dimension
		if cur.unit.String() == "" && unitClone != nil {
			curU = newDimension(cur.value, newUnit([]string{*unitClone}, nil, "")).unify()
		} else {
			curU = cur.unify()
		}
		unit := curU.unit.String()
		if unit == "" && unitStatic != nil {
			unit = *unitStatic
		}
		if (unit != "" && unitStatic == nil) || (unit != "" && order[0].unify().unit.String() == "") {
			unitStatic = strp(unit)
		}
		if unit != "" && unitClone == nil {
			unitClone = strp(cur.unit.String())
		}
		j, found := values[unit]
		if e, ok := values[""]; ok && unit != "" && unitStatic != nil && unit == *unitStatic {
			j, found = e, true
		}
		if !found {
			if unitStatic != nil && unit != *unitStatic {
				argumentError("incompatible types")
			}
			values[unit] = len(order)
			order = append(order, cur)
			continue
		}
		var ref *Dimension
		if order[j].unit.String() == "" && unitClone != nil {
			ref = newDimension(order[j].value, newUnit([]string{*unitClone}, nil, "")).unify()
		} else {
			ref = order[j].unify()
		}
		if (isMin && curU.value < ref.value) || (!isMin && curU.value > ref.value) {
			order[j] = cur
		}
	}
	if len(order) == 1 {
		return order[0]
	}
	parts := make([]string, len(order))
	for i, d := range order {
		parts[i] = toCSS(d, fc.ctx)
	}
	sep := ", "
	if fc.ctx.compress {
		sep = ","
	}
	name := "max"
	if isMin {
		name = "min"
	}
	return newAnonymous(name+"("+strings.Join(parts, sep)+")", 0, nil, false, false, nil)
}

// swallow returns nil when fn throws, as min and max do with their try/catch.
func swallow(fn func() Node) (n Node) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*errorInfo); !ok {
				panic(r)
			}
			n = nil
		}
	}()
	return fn()
}

func dimArg(n Node, prop string) *Dimension {
	d, ok := n.(*Dimension)
	if !ok {
		if n == nil {
			readUndefined(prop)
		}
		return nil
	}
	return d
}

func init() {
	blend := map[string]func(cb, cs float64) float64{
		"multiply": blendMultiply, "screen": blendScreen, "overlay": blendOverlay, "softlight": blendSoftlight,
		"hardlight":  func(cb, cs float64) float64 { return blendOverlay(cs, cb) },
		"difference": func(cb, cs float64) float64 { return math.Abs(cb - cs) },
		"exclusion":  func(cb, cs float64) float64 { return cb + cs - 2*cb*cs },
		"average":    func(cb, cs float64) float64 { return (cb + cs) / 2 },
		"negation":   func(cb, cs float64) float64 { return 1 - math.Abs(cb+cs-1) },
	}
	for name, mode := range blend {
		globalFunctions.add(name, &lessFunction{fn: colorBlend(mode)})
	}
	math1 := map[string]struct {
		fn   func(float64) float64
		unit *string
		css  bool
	}{
		"ceil": {math.Ceil, nil, false}, "floor": {math.Floor, nil, false}, "sqrt": {math.Sqrt, nil, true},
		"abs": {math.Abs, nil, true}, "tan": {math.Tan, strp(""), true}, "sin": {math.Sin, strp(""), true},
		"cos": {math.Cos, strp(""), true}, "atan": {math.Atan, strp("rad"), true},
		"asin": {math.Asin, strp("rad"), true}, "acos": {math.Acos, strp("rad"), true},
	}
	for name, m := range math1 {
		m := m
		globalFunctions.add(name, &lessFunction{fn: func(fc *funcCaller, a []Node) Node {
			return mathHelper(m.fn, m.unit, m.css, argAt(a, 0))
		}})
	}
	registerFunctions(map[string]func(*funcCaller, []Node) Node{
		"round": func(fc *funcCaller, a []Node) Node {
			digits := 0.0
			if f := argAt(a, 1); f != nil {
				digits = valueOf(f)
			}
			return mathHelper(func(v float64) float64 {
				s, err := js.ToFixed(v, digits)
				if err != nil {
					panic(&errorInfo{Message: err.(*js.RangeError).Message, jsError: true})
				}
				return js.StringToNumber(s)
			}, nil, false, argAt(a, 0))
		},
		"min": func(fc *funcCaller, a []Node) Node { return swallow(func() Node { return minMax(fc, true, a) }) },
		"max": func(fc *funcCaller, a []Node) Node { return swallow(func() Node { return minMax(fc, false, a) }) },
		"convert": func(fc *funcCaller, a []Node) Node {
			val, unit := argAt(a, 0), argAt(a, 1)
			d := dimArg(val, "convertTo")
			if d == nil {
				typeError("val.convertTo is not a function")
			}
			if unit == nil {
				readUndefined("value")
			}
			s, ok := stringValue(unit)
			if !ok {
				return d.convertTo(nil)
			}
			return d.convertToUnit(s)
		},
		"pi": func(fc *funcCaller, a []Node) Node { return newDimension(math.Pi, nil) },
		"mod": func(fc *funcCaller, a []Node) Node {
			x, y := argAt(a, 0), argAt(a, 1)
			v := math.Mod(valueOf(x), valueOf(y))
			var u *Unit
			if d, ok := x.(*Dimension); ok {
				u = d.unit
			}
			return newDimension(v, u)
		},
		"pow": func(fc *funcCaller, a []Node) Node {
			x, xok := argAt(a, 0).(*Dimension)
			y, yok := argAt(a, 1).(*Dimension)
			if !xok || !yok {
				argumentError("arguments must be numbers")
			}
			return newDimension(math.Pow(x.value, y.value), x.unit)
		},
		"percentage": func(fc *funcCaller, a []Node) Node {
			return mathHelper(func(v float64) float64 { return v * 100 }, strp("%"), false, argAt(a, 0))
		},
	})
}
