package less

import (
	"math"
	"strconv"
	"strings"

	"github.com/less/less.js/internal/js"
)

// Color is tree/color.js. value is the original form (a keyword or the source hex) when it is kept.
type Color struct {
	nodeBase
	rgb        []float64
	alpha      float64
	value      string
	parensInOp bool
}

// newColorRGB is new Color(rgbArray, a, originalForm); alpha is used as given.
func newColorRGB(rgb []float64, alpha float64, original string) *Color {
	return &Color{rgb: rgb, alpha: alpha, value: original}
}

// newColorHex is new Color(hexDigits, undefined, originalForm) for 3, 4, 6 or 8 hex digits.
func newColorHex(hex, original string) *Color {
	c := &Color{alpha: 1, value: original}
	if js.Len16(hex) >= 6 {
		for i := 0; i+2 <= len(hex); i += 2 {
			v := parseHexByte(hex[i : i+2])
			if i/2 < 3 {
				c.rgb = append(c.rgb, v)
			} else {
				c.alpha = v / 255
			}
		}
	} else {
		for i := 0; i < len(hex); i++ {
			v := parseHexByte(hex[i:i+1] + hex[i:i+1])
			if i < 3 {
				c.rgb = append(c.rgb, v)
			} else {
				c.alpha = v / 255
			}
		}
	}
	return c
}

func parseHexByte(s string) float64 {
	n, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return math.NaN()
	}
	return float64(n)
}

func (c *Color) Type() string { return "Color" }

func (c *Color) setParensInOp() { c.parensInOp = true }

func (c *Color) luma() float64 {
	ch := func(v float64) float64 {
		v = v / 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	r, g, b := ch(c.rgb[0]), ch(c.rgb[1]), ch(c.rgb[2])
	return float64(float64(0.2126*r)+float64(0.7152*g)) + float64(0.0722*b)
}

func (c *Color) genCSS(ctx *evalContext, out *output) { out.add(c.toCSS(ctx, false)) }

func clamp(v, max float64) float64 { return math.Min(math.Max(v, 0), max) }

func (c *Color) toCSS(ctx *evalContext, doNotCompress bool) string {
	compress := ctx != nil && ctx.compress && !doNotCompress
	alpha := fround(ctx, c.alpha)
	colorFunction := ""
	if c.value != "" {
		switch {
		case strings.HasPrefix(c.value, "rgb"):
			if alpha < 1 {
				colorFunction = "rgba"
			}
		case strings.HasPrefix(c.value, "hsl"):
			colorFunction = "hsl"
			if alpha < 1 {
				colorFunction = "hsla"
			}
		default:
			return c.value
		}
	} else if alpha < 1 {
		colorFunction = "rgba"
	}
	var args []string
	switch colorFunction {
	case "rgba":
		for _, v := range c.rgb {
			args = append(args, js.NumberToString(clamp(js.Round(v), 255)))
		}
		args = append(args, js.NumberToString(clamp(alpha, 1)))
	case "hsla", "hsl":
		hsl := c.toHSL()
		args = []string{js.NumberToString(fround(ctx, hsl.h)), js.NumberToString(fround(ctx, hsl.s*100)) + "%",
			js.NumberToString(fround(ctx, hsl.l*100)) + "%"}
		if colorFunction == "hsla" {
			args = append(args, js.NumberToString(clamp(alpha, 1)))
		}
	}
	if colorFunction != "" {
		sep := ", "
		if compress {
			sep = ","
		}
		return colorFunction + "(" + strings.Join(args, sep) + ")"
	}
	color := c.toRGB()
	if compress {
		u := js.ToU16(color)
		if len(u) >= 7 && u[1] == u[2] && u[3] == u[4] && u[5] == u[6] {
			color = "#" + js.FromU16([]uint16{u[1], u[3], u[5]})
		}
	}
	return color
}

func (c *Color) operate(ctx *evalContext, op string, other *Color) *Color {
	alpha := float64(c.alpha*(1-other.alpha)) + other.alpha
	rgb := make([]float64, 3)
	for i := 0; i < 3; i++ {
		rgb[i] = operate(op, c.rgb[i], other.rgb[i])
	}
	return newColorRGB(rgb, alpha, "")
}

func (c *Color) toRGB() string { return toHex(c.rgb) }

type hsla struct{ h, s, l, a float64 }

type hsva struct{ h, s, v, a float64 }

func hueOf(r, g, b, max, d float64) float64 {
	var h float64
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	case b:
		h = (r-g)/d + 4
	default:
		h = math.NaN()
	}
	return h / 6
}

func (c *Color) toHSL() hsla {
	r, g, b := c.rgb[0]/255, c.rgb[1]/255, c.rgb[2]/255
	max, min := jsMax(r, g, b), jsMin(r, g, b)
	l := (max + min) / 2
	d := max - min
	var h, s float64
	if max != min {
		if l > 0.5 {
			s = d / (2 - max - min)
		} else {
			s = d / (max + min)
		}
		h = hueOf(r, g, b, max, d)
	}
	return hsla{h * 360, s, l, c.alpha}
}

func (c *Color) toHSV() hsva {
	r, g, b := c.rgb[0]/255, c.rgb[1]/255, c.rgb[2]/255
	max, min := jsMax(r, g, b), jsMin(r, g, b)
	d := max - min
	s := 0.0
	if max != 0 {
		s = d / max
	}
	h := 0.0
	if max != min {
		h = hueOf(r, g, b, max, d)
	}
	return hsva{h * 360, s, max, c.alpha}
}

// jsMax and jsMin are Math.max and Math.min: NaN wins and +0 is above -0.
func jsMax(vs ...float64) float64 {
	m := math.Inf(-1)
	for _, v := range vs {
		if math.IsNaN(v) {
			return v
		}
		m = math.Max(m, v)
	}
	return m
}

func jsMin(vs ...float64) float64 {
	m := math.Inf(1)
	for _, v := range vs {
		if math.IsNaN(v) {
			return v
		}
		m = math.Min(m, v)
	}
	return m
}

func (c *Color) toARGB() string { return toHex(append([]float64{c.alpha * 255}, c.rgb...)) }

func (c *Color) compare(x Node) cmpResult {
	o, ok := x.(*Color)
	if ok && o.rgb[0] == c.rgb[0] && o.rgb[1] == c.rgb[1] && o.rgb[2] == c.rgb[2] && o.alpha == c.alpha {
		return 0
	}
	return cmpNone
}

func colorFromKeyword(keyword string) *Color {
	key := js.ToLowerCase(keyword)
	var c *Color
	if hex, ok := namedColors[key]; ok {
		c = newColorHex(hex[1:], "")
	} else if key == "transparent" {
		c = newColorRGB([]float64{0, 0, 0}, 0, "")
	}
	if c != nil {
		c.value = keyword
	}
	return c
}

func toHex(v []float64) string {
	var b strings.Builder
	b.WriteString("#")
	for _, c := range v {
		c = clamp(js.Round(c), 255)
		if c < 16 {
			b.WriteString("0")
		}
		if math.IsNaN(c) {
			b.WriteString("NaN")
		} else {
			b.WriteString(strconv.FormatInt(int64(c), 16))
		}
	}
	return b.String()
}
