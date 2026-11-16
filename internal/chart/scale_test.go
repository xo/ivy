package chart

import (
	"math"
	"testing"
)

func TestAutoScale(t *testing.T) {
	fixed := newAxis()
	fixed.min, fixed.max, fixed.major = 0, 25, 5
	logAxis := newAxis()
	logAxis.logBase = 10
	tests := []struct {
		name            string
		lo, hi          float64
		a               axis
		count           int
		min, max, major float64
	}{
		{"columns", 0, 8.2, newAxis(), 10, 0, 9, 1},
		{"lines from zero", 1, 17, newAxis(), 10, 0, 18, 2},
		{"headroom", 0, 5, newAxis(), 8, 0, 6, 1},
		{"short axis", 0, 9, newAxis(), 5, 0, 10, 2},
		{"negative", -3, 14, newAxis(), 10, -4, 16, 2},
		{"far from zero", 90, 100, newAxis(), 10, 88, 102, 2},
		{"all negative", -5, -1, newAxis(), 10, -6, 0, 1},
		{"one value", 5, 5, newAxis(), 10, 0, 6, 1},
		{"zero", 0, 0, newAxis(), 10, 0, 1, 0.1},
		{"no data", math.NaN(), math.NaN(), newAxis(), 10, 0, 1, 0.1},
		{"fixed", 3, 7, fixed, 10, 0, 25, 5},
		{"log", 3, 2000, logAxis, 10, 1, 10000, 10},
		{"large", 0, 1234567, newAxis(), 8, 0, 1400000, 200000},
		{"small", 0.001, 0.0047, newAxis(), 10, 0, 0.005, 0.0005},
	}
	for _, test := range tests {
		s := autoScale(test.lo, test.hi, test.a, test.count)
		if !near(s.min, test.min) || !near(s.max, test.max) || !near(s.major, test.major) {
			t.Errorf("%s: expected %g to %g by %g, got %g to %g by %g", test.name,
				test.min, test.max, test.major, s.min, s.max, s.major)
		}
	}
}

// near says whether two numbers are equal but for the rounding.
func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*max(1, math.Abs(a), math.Abs(b))
}

func TestScaleTicks(t *testing.T) {
	s := autoScale(0, 0.95, newAxis(), 10)
	ticks := s.ticks(false)
	if len(ticks) != 11 || ticks[3] != 0.3 || ticks[10] != 1 {
		t.Errorf("expected the ticks 0 to 1 by 0.1, got: %v", ticks)
	}
	// A major unit that gives more ticks than the limit takes an
	// automatic unit.
	a := newAxis()
	a.major = 1e-9
	s = autoScale(0, 1e9, a, 10)
	if n := len(s.ticks(false)); n > 11 {
		t.Errorf("expected at most 11 ticks, got %d", n)
	}
	if n := len(s.ticks(true)); n > maxTicks {
		t.Errorf("expected at most %d minor ticks, got %d", maxTicks, n)
	}
	// A huge span stays finite.
	s = autoScale(-1e300, 1e300, newAxis(), 10)
	if math.IsInf(s.min, 0) || math.IsInf(s.max, 0) || len(s.ticks(false)) > maxTicks {
		t.Errorf("expected a finite scale, got %+v", s)
	}
}

func TestPercentAxis(t *testing.T) {
	p := &plot{kind: kindBar, grouping: "percentStacked"}
	a := percentAxis(newAxis(), []*plot{p}, false, 0)
	s := autoScale(0, 1, a, 10)
	if s.min != 0 || s.max != 1 || !near(s.major, 0.1) {
		t.Errorf("expected 0 to 100 percent by 10, got %+v", s)
	}
}
