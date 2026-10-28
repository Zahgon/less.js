package js

import (
	"math"
	"strconv"
	"strings"
)

// NumberToString formats n the way Number.prototype.toString() does with radix 10.
func NumberToString(n float64) string {
	switch {
	case math.IsNaN(n):
		return "NaN"
	case n == 0:
		return "0"
	case math.IsInf(n, 1):
		return "Infinity"
	case math.IsInf(n, -1):
		return "-Infinity"
	}
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	e := strconv.FormatFloat(n, 'e', -1, 64)
	mant, expPart, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	exp, _ := strconv.Atoi(expPart)
	k := len(digits)
	p := exp + 1
	var out string
	switch {
	case k <= p && p <= 21:
		out = digits + strings.Repeat("0", p-k)
	case 0 < p && p <= 21:
		out = digits[:p] + "." + digits[p:]
	case -6 < p && p <= 0:
		out = "0." + strings.Repeat("0", -p) + digits
	default:
		es := "+"
		if p-1 < 0 {
			es = "-"
		}
		ev := p - 1
		if ev < 0 {
			ev = -ev
		}
		if k == 1 {
			out = digits + "e" + es + strconv.Itoa(ev)
		} else {
			out = digits[:1] + "." + digits[1:] + "e" + es + strconv.Itoa(ev)
		}
	}
	return sign + out
}
