package js

import (
	"math"
	"strconv"
	"strings"
)

// StringToNumber converts a string to a number like Number(s).
func StringToNumber(s string) float64 {
	t := Trim(s)
	if t == "" {
		return 0
	}
	if len(t) > 2 && t[0] == '0' {
		base := 0
		switch t[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			return parseRadixDigits(t[2:], base, true)
		}
	}
	sign := 1.0
	body := t
	if body[0] == '+' || body[0] == '-' {
		if body[0] == '-' {
			sign = -1
		}
		body = body[1:]
	}
	if body == "Infinity" {
		return sign * math.Inf(1)
	}
	if n := decimalPrefixLen(body); n == len(body) && n > 0 {
		return sign * parseDecimal(body)
	}
	return math.NaN()
}

// decimalPrefixLen returns the length of the longest StrUnsignedDecimalLiteral prefix of s (0 when none).
func decimalPrefixLen(s string) int {
	i := 0
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		j := i + 1
		frac := 0
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
			frac++
		}
		if digits == 0 && frac == 0 {
			return 0
		}
		i = j
		digits += frac
	}
	if digits == 0 {
		return 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		k := j
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		if k > j {
			i = k
		}
	}
	return i
}

func parseDecimal(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return f
		}
		return math.NaN()
	}
	return f
}

func digitVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return 99
}

// parseRadixDigits parses digits in base; whole requires every byte to be a digit.
func parseRadixDigits(s string, base int, whole bool) float64 {
	n := 0
	for n < len(s) && digitVal(s[n]) < base {
		n++
	}
	if n == 0 || (whole && n != len(s)) {
		return math.NaN()
	}
	return radixValue(s[:n], base)
}

func radixValue(digits string, base int) float64 {
	if base == 10 {
		return parseDecimal(digits)
	}
	if base&(base-1) == 0 {
		// Powers of two convert exactly with round-half-even on the 53-bit mantissa.
		return powerOfTwoRadix(digits, base)
	}
	v := 0.0
	for i := 0; i < len(digits); i++ {
		v = v*float64(base) + float64(digitVal(digits[i]))
	}
	return v
}

func powerOfTwoRadix(digits string, base int) float64 {
	bits := 0
	for b := base; b > 1; b >>= 1 {
		bits++
	}
	i := 0
	for i < len(digits) && digits[i] == '0' {
		i++
	}
	digits = digits[i:]
	if digits == "" {
		return 0
	}
	var mant uint64
	exp := 0
	sticky := false
	total := 0
	for k := 0; k < len(digits); k++ {
		d := uint64(digitVal(digits[k]))
		for bit := bits - 1; bit >= 0; bit-- {
			b := (d >> uint(bit)) & 1
			if total == 0 && b == 0 {
				continue
			}
			if total < 54 {
				mant = mant<<1 | b
			} else {
				exp++
				if b == 1 {
					sticky = true
				}
			}
			total++
		}
	}
	if total > 53 {
		half := mant & 1
		mant >>= 1
		exp++
		if half == 1 && (sticky || mant&1 == 1) {
			mant++
		}
	}
	return math.Ldexp(float64(mant), exp)
}

// ParseFloat implements the global parseFloat on a string argument.
func ParseFloat(input string) float64 {
	s := TrimStart(input)
	sign := 1.0
	body := s
	if body != "" && (body[0] == '+' || body[0] == '-') {
		if body[0] == '-' {
			sign = -1
		}
		body = body[1:]
	}
	if strings.HasPrefix(body, "Infinity") {
		return sign * math.Inf(1)
	}
	n := decimalPrefixLen(body)
	if n == 0 {
		return math.NaN()
	}
	return sign * parseDecimal(body[:n])
}
