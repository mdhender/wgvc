package fmath

import (
	"math"
	"math/rand/v2"
	"testing"
)

// ulpsApart counts the representable float64 values between a and b.
func ulpsApart(a, b float64) uint64 {
	if a == b {
		return 0
	}
	ua, ub := math.Float64bits(a), math.Float64bits(b)
	if (ua >> 63) != (ub >> 63) {
		return math.MaxUint64
	}
	if ua > ub {
		return ua - ub
	}
	return ub - ua
}

func TestHypotMatchesStandardLibrary(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	for range 10000 {
		p := (random.Float64() - 0.5) * 200
		q := (random.Float64() - 0.5) * 200
		if ulps := ulpsApart(Hypot(p, q), math.Hypot(p, q)); ulps > 2 {
			t.Fatalf("Hypot(%v, %v) = %v, math.Hypot = %v (%d ulps)", p, q, Hypot(p, q), math.Hypot(p, q), ulps)
		}
	}
	cases := []struct{ p, q, want float64 }{
		{3, 4, 5},
		{0, 0, 0},
		{-3, 0, 3},
		{math.Inf(1), math.NaN(), math.Inf(1)},
		{math.NaN(), 1, math.NaN()},
	}
	for _, c := range cases {
		got := Hypot(c.p, c.q)
		if math.IsNaN(c.want) {
			if !math.IsNaN(got) {
				t.Fatalf("Hypot(%v, %v) = %v, want NaN", c.p, c.q, got)
			}
			continue
		}
		if got != c.want {
			t.Fatalf("Hypot(%v, %v) = %v, want %v", c.p, c.q, got, c.want)
		}
	}
}

func TestAtan2MatchesStandardLibrary(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 4))
	for range 10000 {
		y := (random.Float64() - 0.5) * 200
		x := (random.Float64() - 0.5) * 200
		if ulps := ulpsApart(Atan2(y, x), math.Atan2(y, x)); ulps > 2 {
			t.Fatalf("Atan2(%v, %v) = %v, math.Atan2 = %v (%d ulps)", y, x, Atan2(y, x), math.Atan2(y, x), ulps)
		}
	}
	specials := []float64{0, math.Copysign(0, -1), 1, -1, math.Inf(1), math.Inf(-1), math.NaN()}
	for _, y := range specials {
		for _, x := range specials {
			got, want := Atan2(y, x), math.Atan2(y, x)
			if math.IsNaN(want) {
				if !math.IsNaN(got) {
					t.Fatalf("Atan2(%v, %v) = %v, want NaN", y, x, got)
				}
				continue
			}
			if got != want || math.Signbit(got) != math.Signbit(want) {
				t.Fatalf("Atan2(%v, %v) = %v, want %v", y, x, got, want)
			}
		}
	}
}

func TestExpMatchesStandardLibrary(t *testing.T) {
	random := rand.New(rand.NewPCG(5, 6))
	for range 10000 {
		x := (random.Float64() - 0.5) * 100
		if ulps := ulpsApart(Exp(x), math.Exp(x)); ulps > 2 {
			t.Fatalf("Exp(%v) = %v, math.Exp = %v (%d ulps)", x, Exp(x), math.Exp(x), ulps)
		}
	}
	cases := []struct{ x, want float64 }{
		{0, 1},
		{math.Inf(1), math.Inf(1)},
		{math.Inf(-1), 0},
		{1000, math.Inf(1)},
		{-1000, 0},
	}
	for _, c := range cases {
		if got := Exp(c.x); got != c.want {
			t.Fatalf("Exp(%v) = %v, want %v", c.x, got, c.want)
		}
	}
	if !math.IsNaN(Exp(math.NaN())) {
		t.Fatal("Exp(NaN) is not NaN")
	}
}
