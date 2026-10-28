package js

import (
	"math"
	"strconv"
	"strings"
)

// RangeError is the error Number.prototype.toFixed throws for a digits argument outside 0 to 100.
type RangeError struct{ Message string }

// Error returns the message the JavaScript runtime gives a RangeError.
func (e *RangeError) Error() string { return "RangeError: " + e.Message }

// ToFixed formats x like Number.prototype.toFixed(digits): the exact binary value is rounded to digits decimals
// with ties going to the larger magnitude, and values of 1e21 or more use Number.prototype.toString.
func ToFixed(x float64, digits float64) (string, error) {
	f := toIntegerOrInfinity(digits)
	if math.IsInf(f, 0) || f < 0 || f > 100 {
		return "", &RangeError{Message: "toFixed() digits argument must be between 0 and 100"}
	}
	if math.IsNaN(x) {
		return "NaN", nil
	}
	if math.Abs(x) >= 1e21 || math.IsInf(x, 0) {
		return NumberToString(x), nil
	}
	return fixedDigits(x, int(f)), nil
}

// MustToFixed is ToFixed for digit counts known to be in range.
func MustToFixed(x float64, digits int) string {
	s, err := ToFixed(x, float64(digits))
	if err != nil {
		panic(err)
	}
	return s
}

func toIntegerOrInfinity(n float64) float64 {
	if math.IsNaN(n) {
		return 0
	}
	return math.Trunc(n)
}

func fixedDigits(x float64, f int) string {
	sign := ""
	if x < 0 {
		sign = "-"
		x = -x
	} else if x == 0 {
		x = 0 // toFixed prints negative zero without a sign
	}
	// 1074 fractional digits hold every float64 exactly, so the rounding below sees the exact value.
	exact := strconv.FormatFloat(x, 'f', 1074, 64)
	dot := strings.IndexByte(exact, '.')
	intPart, frac := exact[:dot], exact[dot+1:]
	kept := []byte(intPart + frac[:f])
	rest := frac[f:]
	if rest != "" && rest[0] >= '5' {
		i := len(kept) - 1
		for ; i >= 0; i-- {
			if kept[i] == '9' {
				kept[i] = '0'
				continue
			}
			kept[i]++
			break
		}
		if i < 0 {
			kept = append([]byte{'1'}, kept...)
		}
	}
	digits := string(kept)
	n := len(digits) - f
	out := digits[:n]
	if f > 0 {
		out += "." + digits[n:]
	}
	return sign + out
}

// Round implements Math.round: halves round toward positive infinity and the sign of zero is kept.
func Round(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) || x == 0 {
		return x
	}
	if x > 0 && x < 0.5 {
		return 0
	}
	if x < 0 && x >= -0.5 {
		return math.Copysign(0, -1)
	}
	r := math.Floor(x)
	if x-r >= 0.5 {
		r++
	}
	return r
}
