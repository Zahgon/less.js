package less

import (
	"math"

	"github.com/less/less.js/internal/jsre"
)

// colour functions (functions/color.js). A function returning nil leaves the call in the output unevaluated.

// colorTry runs fn and turns any thrown error into undefined, as rgba and hsla do with their try/catch.
func colorTry(fn func() *Color) (c *Color) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*errorInfo); !ok {
				panic(r)
			}
			c = nil
		}
	}()
	return fn()
}

func fnRgba(r, g, b, a Node) *Color {
	return colorTry(func() *Color {
		if c, ok := r.(*Color); ok {
			alpha := c.alpha
			if g != nil {
				alpha = colorNumber(g)
			}
			return newColorRGB(c.rgb, alpha, "rgba")
		}
		rgb := []float64{colorScaled(r, 255), colorScaled(g, 255), colorScaled(b, 255)}
		return newColorRGB(rgb, colorNumber(a), "rgba")
	})
}

func fnHsla(h, s, l, a Node) *Color {
	return colorTry(func() *Color {
		if c, ok := h.(*Color); ok {
			alpha := c.alpha
			if s != nil {
				alpha = colorNumber(s)
			}
			return newColorRGB(c.rgb, alpha, "hsla")
		}
		hv := jsMod(colorNumber(h), 360) / 360
		sv, lv, av := clamp01(colorNumber(s)), clamp01(colorNumber(l)), clamp01(colorNumber(a))
		var m2 float64
		if lv <= 0.5 {
			m2 = lv * (sv + 1)
		} else {
			m2 = lv + sv - lv*sv
		}
		m1 := lv*2 - m2
		hue := func(x float64) float64 {
			if x < 0 {
				x++
			} else if x > 1 {
				x--
			}
			switch {
			case x*6 < 1:
				return m1 + (m2-m1)*x*6
			case x*2 < 1:
				return m2
			case x*3 < 2:
				return m1 + (m2-m1)*(2.0/3-x)*6
			}
			return m1
		}
		rgb := []float64{hue(hv+1.0/3) * 255, hue(hv) * 255, hue(hv-1.0/3) * 255}
		return newColorRGB(rgb, av, "hsla")
	})
}

// hslaFrom is the module's hsla(origColor, hsl) helper: it keeps an rgb or hsl original form.
func hslaFrom(orig *Color, h hsla) Node {
	c := fnHsla(num(h.h), num(h.s), num(h.l), num(h.a))
	if c == nil {
		return nil
	}
	if orig.value != "" && reRgbHsl.Test(orig.value) {
		c.value = orig.value
	} else {
		c.value = "rgb"
	}
	return c
}

var reRgbHsl = re(`^(rgb|hsl)`)

// splitSpaceArgs handles rgb(r g b / a) and hsl(h s l / a): one Expression argument holding the channels.
func splitSpaceArgs(args []Node) (Node, Node, Node, Node) {
	x, y, z := argAt(args, 0), argAt(args, 1), argAt(args, 2)
	var a Node = num(1)
	if e, ok := x.(*Expression); ok {
		x, y, z = argAt(e.value, 0), argAt(e.value, 1), argAt(e.value, 2)
		if op, isOp := z.(*Operation); isOp {
			z, a = op.operands[0], op.operands[1]
		}
	}
	return x, y, z, a
}

func adjust(name string, fn func(h *hsla, amount float64, relative bool)) func(*funcCaller, []Node) Node {
	return func(fc *funcCaller, args []Node) Node {
		color, amount, method := argAt(args, 0), argAt(args, 1), argAt(args, 2)
		if name == "saturate" {
			if _, ok := color.(*Color); !ok {
				if color == nil {
					readUndefined("rgb")
				}
				return nil
			}
		}
		h := toHSLArg(color)
		relative := false
		if method != nil {
			s, _ := stringValue(method)
			relative = s == "relative"
		}
		fn(&h, valueOf(amount), relative)
		return hslaFrom(color.(*Color), h)
	}
}

func shift(field func(h *hsla) *float64, sign float64) func(h *hsla, amount float64, relative bool) {
	return func(h *hsla, amount float64, relative bool) {
		f := field(h)
		if relative {
			*f += sign * (*f * amount / 100)
		} else {
			*f += sign * (amount / 100)
		}
		*f = clamp01(*f)
	}
}

func fieldS(h *hsla) *float64 { return &h.s }
func fieldL(h *hsla) *float64 { return &h.l }
func fieldA(h *hsla) *float64 { return &h.a }

func fnMix(color1, color2, weight Node) Node {
	p := 0.5
	if weight != nil {
		p = valueOf(weight) / 100.0
	} else {
		p = 50.0 / 100.0
	}
	w := p*2 - 1
	a := toHSLArg(color1).a - toHSLArg(color2).a
	var w1 float64
	if w*a == -1 {
		w1 = (w + 1) / 2.0
	} else {
		w1 = ((w+a)/(1+w*a) + 1) / 2.0
	}
	w2 := 1 - w1
	c1, c2 := color1.(*Color), color2.(*Color)
	rgb := []float64{c1.rgb[0]*w1 + c2.rgb[0]*w2, c1.rgb[1]*w1 + c2.rgb[1]*w2, c1.rgb[2]*w1 + c2.rgb[2]*w2}
	return newColorRGB(rgb, c1.alpha*p+c2.alpha*(1-p), "")
}

var reHexColorArg = jsre.MustCompile(`^#([A-Fa-f0-9]{8}|[A-Fa-f0-9]{6}|[A-Fa-f0-9]{3,4})$`, "i")

func init() {
	dim := func(v float64, unit string) Node {
		var u *Unit
		if unit != "" {
			u = newUnit([]string{unit}, nil, "")
		}
		return newDimension(v, u)
	}
	registerFunctions(map[string]func(*funcCaller, []Node) Node{
		"rgb": func(fc *funcCaller, args []Node) Node {
			r, g, b, a := splitSpaceArgs(args)
			if c := fnRgba(r, g, b, a); c != nil {
				c.value = "rgb"
				return c
			}
			return nil
		},
		"rgba": func(fc *funcCaller, args []Node) Node {
			if c := fnRgba(argAt(args, 0), argAt(args, 1), argAt(args, 2), argAt(args, 3)); c != nil {
				return c
			}
			return nil
		},
		"hsl": func(fc *funcCaller, args []Node) Node {
			h, s, l, a := splitSpaceArgs(args)
			if c := fnHsla(h, s, l, a); c != nil {
				c.value = "hsl"
				return c
			}
			return nil
		},
		"hsla": func(fc *funcCaller, args []Node) Node {
			if c := fnHsla(argAt(args, 0), argAt(args, 1), argAt(args, 2), argAt(args, 3)); c != nil {
				return c
			}
			return nil
		},
		"hsv": func(fc *funcCaller, args []Node) Node {
			return fnHsva(argAt(args, 0), argAt(args, 1), argAt(args, 2), num(1.0))
		},
		"hsva": func(fc *funcCaller, args []Node) Node {
			return fnHsva(argAt(args, 0), argAt(args, 1), argAt(args, 2), argAt(args, 3))
		},
		"hue":           func(fc *funcCaller, a []Node) Node { return dim(toHSLArg(argAt(a, 0)).h, "") },
		"saturation":    func(fc *funcCaller, a []Node) Node { return dim(toHSLArg(argAt(a, 0)).s*100, "%") },
		"lightness":     func(fc *funcCaller, a []Node) Node { return dim(toHSLArg(argAt(a, 0)).l*100, "%") },
		"hsvhue":        func(fc *funcCaller, a []Node) Node { return dim(toHSVArg(argAt(a, 0)).h, "") },
		"hsvsaturation": func(fc *funcCaller, a []Node) Node { return dim(toHSVArg(argAt(a, 0)).s*100, "%") },
		"hsvvalue":      func(fc *funcCaller, a []Node) Node { return dim(toHSVArg(argAt(a, 0)).v*100, "%") },
		"red":           func(fc *funcCaller, a []Node) Node { return dim(rgbOf(argAt(a, 0), 0), "") },
		"green":         func(fc *funcCaller, a []Node) Node { return dim(rgbOf(argAt(a, 0), 1), "") },
		"blue":          func(fc *funcCaller, a []Node) Node { return dim(rgbOf(argAt(a, 0), 2), "") },
		"alpha":         func(fc *funcCaller, a []Node) Node { return dim(toHSLArg(argAt(a, 0)).a, "") },
		"luma": func(fc *funcCaller, a []Node) Node {
			c := colorMethod(argAt(a, 0), "color", "luma")
			return dim(c.luma()*c.alpha*100, "%")
		},
		"luminance": func(fc *funcCaller, a []Node) Node {
			c := argAt(a, 0)
			l := 0.2126*rgbOf(c, 0)/255 + 0.7152*rgbOf(c, 1)/255 + 0.0722*rgbOf(c, 2)/255
			return dim(l*c.(*Color).alpha*100, "%")
		},
		"saturate":   adjust("saturate", shift(fieldS, 1)),
		"desaturate": adjust("desaturate", shift(fieldS, -1)),
		"lighten":    adjust("lighten", shift(fieldL, 1)),
		"darken":     adjust("darken", shift(fieldL, -1)),
		"fadein":     adjust("fadein", shift(fieldA, 1)),
		"fadeout":    adjust("fadeout", shift(fieldA, -1)),
		"fade": func(fc *funcCaller, a []Node) Node {
			h := toHSLArg(argAt(a, 0))
			h.a = clamp01(valueOf(argAt(a, 1)) / 100)
			return hslaFrom(a[0].(*Color), h)
		},
		"spin": func(fc *funcCaller, a []Node) Node {
			h := toHSLArg(argAt(a, 0))
			hue := math.Mod(h.h+valueOf(argAt(a, 1)), 360)
			if hue < 0 {
				hue += 360
			}
			h.h = hue
			return hslaFrom(a[0].(*Color), h)
		},
		"mix": func(fc *funcCaller, a []Node) Node { return fnMix(argAt(a, 0), argAt(a, 1), argAt(a, 2)) },
		"greyscale": func(fc *funcCaller, a []Node) Node {
			return globalFunctions.get("desaturate").fn(fc, []Node{argAt(a, 0), dim(100, "")})
		},
		"contrast": func(fc *funcCaller, a []Node) Node { return fnContrast(a) },
		"argb": func(fc *funcCaller, a []Node) Node {
			c := colorMethod(argAt(a, 0), "color", "toARGB")
			return newAnonymous(c.toARGB(), 0, nil, false, false, nil)
		},
		"color": func(fc *funcCaller, a []Node) Node { return fnColor(argAt(a, 0)) },
		"tint": func(fc *funcCaller, a []Node) Node {
			return fnMix(fnRgbValue(255, 255, 255), argAt(a, 0), argAt(a, 1))
		},
		"shade": func(fc *funcCaller, a []Node) Node {
			return fnMix(fnRgbValue(0, 0, 0), argAt(a, 0), argAt(a, 1))
		},
	})
}

func fnRgbValue(r, g, b float64) *Color {
	c := fnRgba(num(r), num(g), num(b), num(1))
	c.value = "rgb"
	return c
}

func fnHsva(h, s, v, a Node) Node {
	hv := (jsMod(colorNumber(h), 360) / 360) * 360
	sv, vv, av := colorNumber(s), colorNumber(v), colorNumber(a)
	i := math.Floor(math.Mod(hv/60, 6))
	f := hv/60 - i
	vs := []float64{vv, vv * (1 - sv), vv * (1 - f*sv), vv * (1 - (1-f)*sv)}
	perm := [][3]int{{0, 3, 1}, {2, 0, 1}, {1, 0, 3}, {1, 2, 0}, {3, 1, 0}, {0, 1, 2}}
	if math.IsNaN(i) || i < 0 || int(i) >= len(perm) {
		readUndefined("0")
	}
	p := perm[int(i)]
	if c := fnRgba(num(vs[p[0]]*255), num(vs[p[1]]*255), num(vs[p[2]]*255), num(av)); c != nil {
		return c
	}
	return nil
}

func fnContrast(a []Node) Node {
	color, dark, light, threshold := argAt(a, 0), argAt(a, 1), argAt(a, 2), argAt(a, 3)
	c, ok := color.(*Color)
	if !ok {
		if color == nil {
			readUndefined("rgb")
		}
		return nil
	}
	if light == nil {
		light = fnRgba(num(255), num(255), num(255), num(1.0))
	}
	if dark == nil {
		dark = fnRgba(num(0), num(0), num(0), num(1.0))
	}
	if colorMethod(dark, "dark", "luma").luma() > colorMethod(light, "light", "luma").luma() {
		light, dark = dark, light
	}
	t := 0.43
	if threshold != nil {
		t = colorNumber(threshold)
	}
	if c.luma() < t {
		return light
	}
	return dark
}

func fnColor(c Node) Node {
	if q, ok := c.(*Quoted); ok && reHexColorArg.Test(q.value) {
		val := q.value[1:]
		return newColorHex(val, "#"+val)
	}
	if col, ok := c.(*Color); ok {
		col.value = ""
		return col
	}
	if c == nil {
		readUndefined("value")
	}
	s, ok := stringValue(c)
	if !ok {
		switch c.(type) {
		case *Dimension, *Expression, *Value:
			typeError("keyword.toLowerCase is not a function")
		}
		readUndefined("toLowerCase")
	}
	if col := colorFromKeyword(s); col != nil {
		col.value = ""
		return col
	}
	panic(&errorInfo{Type: "Argument", Message: "argument must be a color keyword or 3|4|6|8 digit hex e.g. #FFF"})
}
