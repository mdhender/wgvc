// Copyright (c) 2026 Michael D Henderson. All rights reserved.
//
// The algorithms in this file are copied from the Go standard library's math
// package (Copyright 2009 The Go Authors, BSD-style license; see
// https://go.dev/LICENSE) and, through it, from the Cephes math library by
// Stephen L. Moshier.

// Package fmath provides floating-point functions whose results are
// bit-identical on every architecture.
//
// The standard library does not promise this. Go lets a compiler fuse x*y + z
// into one multiply-add with a single rounding, and the arm64 backend does
// while amd64 does not, so pure-Go functions such as math.Atan2 round
// differently on the two. Other functions have per-architecture assembly:
// math.Hypot on amd64, and math.Exp on both, where the amd64 version even
// picks fused instructions at run time by CPU feature. World generation must
// produce the same bits everywhere, so it uses these copies, in which every
// product that feeds an addition or subtraction is wrapped in an explicit
// float64 conversion. Per the language specification, an explicit conversion
// rounds the product and prevents fusion. The results match Go's unfused
// pure-Go implementations exactly.
package fmath

import "math"

// Hypot returns Sqrt(p*p + q*q), taking care to avoid unnecessary overflow and
// underflow. The special cases are those of math.Hypot.
func Hypot(p, q float64) float64 {
	p, q = math.Abs(p), math.Abs(q)
	switch {
	case math.IsInf(p, 1) || math.IsInf(q, 1):
		return math.Inf(1)
	case math.IsNaN(p) || math.IsNaN(q):
		return math.NaN()
	}
	if p < q {
		p, q = q, p
	}
	if p == 0 {
		return 0
	}
	q = q / p
	return p * math.Sqrt(1+float64(q*q))
}

// Atan2 returns the arc tangent of y/x, using the signs of the two to determine
// the quadrant of the return value. The special cases are those of math.Atan2.
func Atan2(y, x float64) float64 {
	switch {
	case math.IsNaN(y) || math.IsNaN(x):
		return math.NaN()
	case y == 0:
		if x >= 0 && !math.Signbit(x) {
			return math.Copysign(0, y)
		}
		return math.Copysign(math.Pi, y)
	case x == 0:
		return math.Copysign(math.Pi/2, y)
	case math.IsInf(x, 0):
		if math.IsInf(x, 1) {
			if math.IsInf(y, 0) {
				return math.Copysign(math.Pi/4, y)
			}
			return math.Copysign(0, y)
		}
		if math.IsInf(y, 0) {
			return math.Copysign(3*math.Pi/4, y)
		}
		return math.Copysign(math.Pi, y)
	case math.IsInf(y, 0):
		return math.Copysign(math.Pi/2, y)
	}

	q := Atan(y / x)
	if x < 0 {
		if q <= 0 {
			return q + math.Pi
		}
		return q - math.Pi
	}
	return q
}

// Atan returns the arctangent, in radians, of x. The special cases are those
// of math.Atan.
func Atan(x float64) float64 {
	if x == 0 {
		return x
	}
	if x > 0 {
		return satan(x)
	}
	return -satan(-x)
}

// satan reduces its positive argument to the range [0, 0.66] and calls xatan.
func satan(x float64) float64 {
	const (
		morebits = 6.123233995736765886130e-17 // pi/2 = PIO2 + morebits
		tan3pio8 = 2.41421356237309504880      // tan(3*pi/8)
	)
	if x <= 0.66 {
		return xatan(x)
	}
	if x > tan3pio8 {
		return math.Pi/2 - xatan(1/x) + morebits
	}
	return math.Pi/4 + xatan((x-1)/(x+1)) + 0.5*morebits
}

// xatan evaluates a series valid in the range [0, 0.66].
func xatan(x float64) float64 {
	const (
		p0 = -8.750608600031904122785e-01
		p1 = -1.615753718733365076637e+01
		p2 = -7.500855792314704667340e+01
		p3 = -1.228866684490136173410e+02
		p4 = -6.485021904942025371773e+01
		q0 = +2.485846490142306297962e+01
		q1 = +1.650270098316988542046e+02
		q2 = +4.328810604912902668951e+02
		q3 = +4.853903996359136964868e+02
		q4 = +1.945506571482613964425e+02
	)
	z := x * x
	numerator := float64(p0*z) + p1
	numerator = float64(numerator*z) + p2
	numerator = float64(numerator*z) + p3
	numerator = float64(numerator*z) + p4
	denominator := z + q0
	denominator = float64(denominator*z) + q1
	denominator = float64(denominator*z) + q2
	denominator = float64(denominator*z) + q3
	denominator = float64(denominator*z) + q4
	z = z * numerator / denominator
	return float64(x*z) + x
}

// Exp returns e**x, the base-e exponential of x. The special cases are those
// of math.Exp.
func Exp(x float64) float64 {
	const (
		ln2Hi = 6.93147180369123816490e-01
		ln2Lo = 1.90821492927058770002e-10
		log2e = 1.44269504088896338700e+00

		overflow  = 7.09782712893383973096e+02
		underflow = -7.45133219101941108420e+02
		nearZero  = 1.0 / (1 << 28) // 2**-28
	)

	switch {
	case math.IsNaN(x):
		return x
	case x > overflow:
		return math.Inf(1)
	case x < underflow:
		return 0
	case -nearZero < x && x < nearZero:
		return 1 + x
	}

	// Reduce; computed as r = hi - lo for extra precision.
	var k int
	switch {
	case x < 0:
		k = int(float64(log2e*x) - 0.5)
	case x > 0:
		k = int(float64(log2e*x) + 0.5)
	}
	hi := x - float64(float64(k)*ln2Hi)
	lo := float64(k) * ln2Lo

	return expmulti(hi, lo, k)
}

// expmulti returns e**r × 2**k where r = hi - lo and |r| ≤ ln(2)/2.
func expmulti(hi, lo float64, k int) float64 {
	const (
		p1 = 1.66666666666666657415e-01
		p2 = -2.77777777770155933842e-03
		p3 = 6.61375632143793436117e-05
		p4 = -1.65339022054652515390e-06
		p5 = 4.13813679705723846039e-08
	)

	r := hi - lo
	t := r * r
	series := p4 + float64(t*p5)
	series = p3 + float64(t*series)
	series = p2 + float64(t*series)
	series = p1 + float64(t*series)
	c := r - float64(t*series)
	y := 1 - ((lo - (r*c)/(2-c)) - hi)
	return math.Ldexp(y, k)
}
