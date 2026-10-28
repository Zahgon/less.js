package less

import (
	"strings"

	"github.com/less/less.js/internal/js"
	"github.com/less/less.js/internal/jsre"
)

// string functions (functions/string.js) and svg-gradient (functions/svg.js).

// quotedValue is `str.value` where the original reads a Quoted's text; other nodes give their own value.
func quotedValue(n Node) string {
	if n == nil {
		readUndefined("value")
	}
	if s, ok := stringValue(n); ok {
		return s
	}
	if d, ok := n.(*Dimension); ok {
		return js.NumberToString(d.value)
	}
	return "undefined"
}

func uriError() { panic(&errorInfo{Message: "URI malformed", jsError: true}) }

var (
	rePercentToken  = jsre.MustCompile(`%[sda]`, "i")
	reTokenS        = jsre.MustCompile(`s`, "i")
	reTokenUpper    = re(`[A-Z]$`)
	reDoublePercent = jsre.MustCompile(`%%`, "g")
)

func fnEscape(n Node) Node {
	enc, err := js.EncodeURI(quotedValue(n))
	if err != nil {
		uriError()
	}
	r := strings.NewReplacer("=", "%3D", ":", "%3A", "#", "%23", ";", "%3B", "(", "%28", ")", "%29")
	v := r.Replace(enc)
	a := newAnonymous(v, 0, nil, false, false, nil)
	a.preventReparse = strings.Contains(v, "%")
	return a
}

func fnReplace(str, pattern, replacement, flags Node) Node {
	if str == nil || pattern == nil || replacement == nil {
		readUndefined("value")
	}
	result := quotedValue(str)
	repl := ""
	if q, ok := replacement.(*Quoted); ok {
		repl = q.value
	} else {
		repl = toCSS(replacement, nil)
	}
	f := ""
	if flags != nil {
		f = quotedValue(flags)
	}
	rx, serr := jsre.Compile(quotedValue(pattern), f)
	if serr != nil {
		panic(&errorInfo{Message: serr.Message, jsError: true})
	}
	result = rx.Replace(result, repl)
	q, _ := str.(*Quoted)
	quote, escaped := "", false
	if q != nil {
		quote, escaped = q.quote, q.escaped
	}
	return newQuoted(quote, result, escaped, 0, nil)
}

func fnFormat(args []Node) Node {
	str := argAt(args, 0)
	result := quotedValue(str)
	for _, arg := range args[1:] {
		result = rePercentToken.ReplaceFunc(result, func(token string, _ []*string) string {
			var value string
			if q, ok := arg.(*Quoted); ok && reTokenS.Test(token) {
				value = q.value
			} else {
				value = toCSS(arg, nil)
			}
			if reTokenUpper.Test(token) {
				enc, err := js.EncodeURIComponent(value)
				if err != nil {
					uriError()
				}
				return enc
			}
			return value
		})
	}
	result = reDoublePercent.Replace(result, "%")
	q, _ := str.(*Quoted)
	quote, escaped := "", false
	if q != nil {
		quote, escaped = q.quote, q.escaped
	}
	return newQuoted(quote, result, escaped, 0, nil)
}

func svgArgumentError() {
	argumentError("svg-gradient expects direction, start_color [start_position], [color position,]..., end_color [end_position] or direction, color list")
}

func fnSvgGradient(fc *funcCaller, args []Node) Node {
	direction := argAt(args, 0)
	if direction == nil {
		readUndefined("toCSS")
	}
	renderEnv := &evalContext{}
	directionValue := toCSS(direction, renderEnv)
	var stops []Node
	// stringStops is a string `value`: its characters are the stops, and the first is not a colour
	stringStops := false
	switch {
	case len(args) == 2:
		if list, ok := arrayValue(args[1]); ok {
			if len(list) < 2 {
				svgArgumentError()
			}
			stops = list
		} else if s, isStr := stringValue(args[1]); isStr {
			if js.Len16(s) < 2 {
				svgArgumentError()
			}
			stringStops = true
		} else if _, isNode := valueNodeOf(args[1]); !isNode {
			if _, isDim := args[1].(*Dimension); !isDim {
				readUndefined("length")
			}
		}
		// a number or a node has no length: there are no stops
	case len(args) < 3:
		svgArgumentError()
	default:
		stops = args[1:]
	}
	gradientType := "linear"
	rect := `x="0" y="0" width="1" height="1"`
	var dir string
	switch directionValue {
	case "to bottom":
		dir = `x1="0%" y1="0%" x2="0%" y2="100%"`
	case "to right":
		dir = `x1="0%" y1="0%" x2="100%" y2="0%"`
	case "to bottom right":
		dir = `x1="0%" y1="0%" x2="100%" y2="100%"`
	case "to top right":
		dir = `x1="0%" y1="100%" x2="100%" y2="0%"`
	case "ellipse", "ellipse at center":
		gradientType = "radial"
		dir = `cx="50%" cy="50%" r="75%"`
		rect = `x="-50" y="-50" width="101" height="101"`
	default:
		argumentError("svg-gradient direction must be 'to bottom', 'to right', 'to bottom right', 'to top right' or 'ellipse at center'")
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><` + gradientType + `Gradient id="g" ` + dir + `>`)
	if stringStops {
		svgArgumentError()
	}
	for i, stop := range stops {
		var color, position Node
		if e, ok := stop.(*Expression); ok {
			color, position = argAt(e.value, 0), argAt(e.value, 1)
		} else {
			color = stop
		}
		c, isColor := color.(*Color)
		_, posDim := position.(*Dimension)
		if !isColor || (!((i == 0 || i+1 == len(stops)) && position == nil) && !posDim) {
			svgArgumentError()
		}
		pos := "100%"
		switch {
		case position != nil:
			pos = toCSS(position, renderEnv)
		case i == 0:
			pos = "0%"
		}
		b.WriteString(`<stop offset="` + pos + `" stop-color="` + c.toRGB() + `"`)
		if c.alpha < 1 {
			b.WriteString(` stop-opacity="` + js.NumberToString(c.alpha) + `"`)
		}
		b.WriteString("/>")
	}
	b.WriteString(`</` + gradientType + `Gradient><rect ` + rect + ` fill="url(#g)" /></svg>`)
	enc, err := js.EncodeURIComponent(b.String())
	if err != nil {
		uriError()
	}
	uri := "data:image/svg+xml," + enc
	return newURL(newQuoted("'"+uri+"'", uri, false, fc.index, fc.fileInfo), fc.index, fc.fileInfo, false)
}

func init() {
	registerFunctions(map[string]func(*funcCaller, []Node) Node{
		"e": func(fc *funcCaller, a []Node) Node {
			str := argAt(a, 0)
			if j, ok := str.(*JavaScript); ok {
				return newQuoted("\"", j.expression, true, 0, nil)
			}
			return newQuoted("\"", quotedValue(str), true, 0, nil)
		},
		"escape": func(fc *funcCaller, a []Node) Node { return fnEscape(argAt(a, 0)) },
		"replace": func(fc *funcCaller, a []Node) Node {
			return fnReplace(argAt(a, 0), argAt(a, 1), argAt(a, 2), argAt(a, 3))
		},
		"%":            func(fc *funcCaller, a []Node) Node { return fnFormat(a) },
		"svg-gradient": fnSvgGradient,
	})
}
