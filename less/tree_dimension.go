package less

import (
	"math"
	"sort"
	"strings"

	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
)

// unitGroup is one group of data/unit-conversions.js; the factors are computed at run time in float64, as the
// JavaScript source computes them.
type unitGroup struct {
	name    string
	units   []string
	factors map[string]float64
}

var unitConversions = func() []unitGroup {
	m, pi := 0.0254, math.Pi
	return []unitGroup{
		{"length", []string{"m", "cm", "mm", "in", "px", "pt", "pc"}, map[string]float64{
			"m": 1, "cm": 0.01, "mm": 0.001, "in": m, "px": m / 96, "pt": m / 72, "pc": float64(m/72) * 12}},
		{"duration", []string{"s", "ms"}, map[string]float64{"s": 1, "ms": 0.001}},
		{"angle", []string{"rad", "deg", "grad", "turn"}, map[string]float64{
			"rad": 1 / float64(2*pi), "deg": 1.0 / 360, "grad": 1.0 / 400, "turn": 1}},
	}
}()

func unitGroupByName(name string) *unitGroup {
	for i := range unitConversions {
		if unitConversions[i].name == name {
			return &unitConversions[i]
		}
	}
	return nil
}

// Unit is tree/unit.js.
type Unit struct {
	nodeBase
	numerator   []string
	denominator []string
	backupUnit  string
}

func sortU16(s []string) []string {
	sort.SliceStable(s, func(i, j int) bool { return lessU16(s[i], s[j]) })
	return s
}

func newUnit(numerator, denominator []string, backupUnit string) *Unit {
	u := &Unit{numerator: sortU16(append([]string{}, numerator...)), denominator: sortU16(append([]string{}, denominator...))}
	if backupUnit != "" {
		u.backupUnit = backupUnit
	} else if len(numerator) > 0 {
		u.backupUnit = numerator[0]
	}
	return u
}

// Type is the node's type name, as the original's type getter reports it.
func (u *Unit) Type() string { return "Unit" }

func (u *Unit) clone() *Unit { return newUnit(u.numerator, u.denominator, u.backupUnit) }

func (u *Unit) genCSS(ctx *evalContext, out *output) {
	strict := ctx != nil && ctx.strictUnits
	switch {
	case len(u.numerator) == 1:
		out.add(u.numerator[0])
	case !strict && u.backupUnit != "":
		out.add(u.backupUnit)
	case !strict && len(u.denominator) > 0:
		out.add(u.denominator[0])
	}
}

// String returns the unit as CSS prints it (unit.js toString).
func (u *Unit) String() string {
	s := strings.Join(u.numerator, "*")
	for _, d := range u.denominator {
		s += "/" + d
	}
	return s
}

func (u *Unit) compare(other Node) cmpResult {
	o, ok := other.(*Unit)
	if ok && u.is(o.String()) {
		return 0
	}
	return cmpNone
}

func (u *Unit) is(s string) bool { return js.ToUpperCase(u.String()) == js.ToUpperCase(s) }

var reLengthUnit = jsre.MustCompile(`^(px|em|ex|ch|rem|in|cm|mm|pc|pt|ex|vw|vh|vmin|vmax)$`, "gi")

func (u *Unit) isLength() bool { return reLengthUnit.Test(toCSS(u, &evalContext{})) }

func (u *Unit) isEmpty() bool { return len(u.numerator) == 0 && len(u.denominator) == 0 }

func (u *Unit) isSingular() bool { return len(u.numerator) <= 1 && len(u.denominator) == 0 }

func (u *Unit) mapUnits(fn func(atomic string, denominator bool) string) {
	for i := range u.numerator {
		u.numerator[i] = fn(u.numerator[i], false)
	}
	for i := range u.denominator {
		u.denominator[i] = fn(u.denominator[i], true)
	}
}

// usedUnits returns, per conversion group, the first unit of that group the unit uses.
func (u *Unit) usedUnits() []conversion {
	var result []conversion
	for _, g := range unitConversions {
		found := false
		u.mapUnits(func(atomic string, _ bool) string {
			if _, ok := g.factors[atomic]; ok && !found {
				found = true
				result = append(result, conversion{group: g.name, unit: atomic})
			}
			return atomic
		})
	}
	return result
}

func (u *Unit) cancel() {
	counter := map[string]int{}
	var order []string
	for _, a := range u.numerator {
		if _, ok := counter[a]; !ok {
			order = append(order, a)
		}
		counter[a]++
	}
	for _, a := range u.denominator {
		if _, ok := counter[a]; !ok {
			order = append(order, a)
		}
		counter[a]--
	}
	u.numerator, u.denominator = []string{}, []string{}
	for _, a := range order {
		c := counter[a]
		for i := 0; i < c; i++ {
			u.numerator = append(u.numerator, a)
		}
		for i := 0; i < -c; i++ {
			u.denominator = append(u.denominator, a)
		}
	}
	sortU16(u.numerator)
	sortU16(u.denominator)
}

// conversion maps a unit group to its target unit, in the order the JavaScript object would iterate.
type conversion struct {
	group string
	unit  string
}

// Dimension is tree/dimension.js.
type Dimension struct {
	nodeBase
	value      float64
	unit       *Unit
	parensInOp bool
}

func newDimension(v float64, unit *Unit) *Dimension {
	if math.IsNaN(v) {
		panic(&errorInfo{Message: "Dimension is not a number."})
	}
	if v == 0 {
		v = 0 // parseFloat(String(-0)) is 0
	}
	if unit == nil {
		unit = newUnit(nil, nil, "")
	}
	d := &Dimension{value: v, unit: unit}
	setParent(unit, d)
	return d
}

// newDimensionFromString is new Dimension(string, unitString).
func newDimensionFromString(v, unit string) *Dimension {
	var u *Unit
	if unit != "" {
		u = newUnit([]string{unit}, nil, "")
	}
	return newDimension(js.ParseFloat(v), u)
}

// Type is the node's type name, as the original's type getter reports it.
func (d *Dimension) Type() string { return "Dimension" }

func (d *Dimension) setParensInOp() { d.parensInOp = true }

func (d *Dimension) accept(v visitor) {
	if u, ok := v.visit(d.unit).(*Unit); ok {
		d.unit = u
	}
}

func (d *Dimension) toColor() *Color { return newColorRGB([]float64{d.value, d.value, d.value}, 1, "") }

func (d *Dimension) genCSS(ctx *evalContext, out *output) {
	if ctx != nil && ctx.strictUnits && !d.unit.isSingular() {
		panic(&errorInfo{Message: "Multiple units in dimension. Correct the units or use the unit function. Bad unit: " + d.unit.String()})
	}
	value := fround(ctx, d.value)
	str := js.NumberToString(value)
	if value != 0 && value < 0.000001 && value > -0.000001 {
		str = strings.TrimRight(js.MustToFixed(value, 20), "0")
	}
	if ctx != nil && ctx.compress {
		if value == 0 && d.unit.isLength() {
			out.add(str)
			return
		}
		if value > 0 && value < 1 {
			str = js.Slice16(str, 1, js.Len16(str))
		}
	}
	out.add(str)
	genCSS(d.unit, ctx, out)
}

func (d *Dimension) operate(ctx *evalContext, op string, other *Dimension) *Dimension {
	value := operate(op, d.value, other.value)
	unit := d.unit.clone()
	switch op {
	case "+", "-":
		if len(unit.numerator) == 0 && len(unit.denominator) == 0 {
			unit = other.unit.clone()
			if d.unit.backupUnit != "" {
				unit.backupUnit = d.unit.backupUnit
			}
		} else if len(other.unit.numerator) == 0 && len(unit.denominator) == 0 {
			// the other operand has no unit: keep this one
		} else {
			other = other.convertTo(d.unit.usedUnits())
			if ctx.strictUnits && other.unit.String() != unit.String() {
				panic(&errorInfo{Message: "Incompatible units. Change the units or use the unit function. Bad units: '" +
					unit.String() + "' and '" + other.unit.String() + "'."})
			}
			value = operate(op, d.value, other.value)
		}
	case "*":
		unit.numerator = sortU16(append(unit.numerator, other.unit.numerator...))
		unit.denominator = sortU16(append(unit.denominator, other.unit.denominator...))
		unit.cancel()
	case "/":
		unit.numerator = sortU16(append(unit.numerator, other.unit.denominator...))
		unit.denominator = sortU16(append(unit.denominator, other.unit.numerator...))
		unit.cancel()
	}
	return newDimension(value, unit)
}

func (d *Dimension) compare(other Node) cmpResult {
	o, ok := other.(*Dimension)
	if !ok {
		return cmpNone
	}
	a, b := d, o
	if !d.unit.isEmpty() && !o.unit.isEmpty() {
		a, b = d.unify(), o.unify()
		if a.unit.compare(b.unit) != 0 {
			return cmpNone
		}
	}
	return numericCompare(a.value, b.value)
}

var unifyConversions = []conversion{{"length", "px"}, {"duration", "s"}, {"angle", "rad"}}

func (d *Dimension) unify() *Dimension { return d.convertTo(unifyConversions) }

// convertToUnit is convertTo with a unit name: the last group holding the unit is converted.
func (d *Dimension) convertToUnit(name string) *Dimension {
	var conv []conversion
	for _, g := range unitConversions {
		if _, ok := g.factors[name]; ok {
			conv = []conversion{{g.name, name}}
		}
	}
	return d.convertTo(conv)
}

func (d *Dimension) convertTo(conversions []conversion) *Dimension {
	value := d.value
	unit := d.unit.clone()
	for _, c := range conversions {
		g := unitGroupByName(c.group)
		if g == nil {
			continue
		}
		target := c.unit
		unit.mapUnits(func(atomic string, denominator bool) string {
			f, ok := g.factors[atomic]
			if !ok {
				return atomic
			}
			ratio := f / g.factors[target]
			if denominator {
				value = value / ratio
			} else {
				value = value * ratio
			}
			return target
		})
	}
	unit.cancel()
	return newDimension(value, unit)
}
