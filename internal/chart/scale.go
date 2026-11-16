package chart

import (
	"math"
	"strconv"
)

// This file computes the scale of an axis of values as Excel does. The
// minimum is 0 when the values are not negative and the smallest is below
// five sixths of the largest. The maximum is the largest value plus a
// twentieth of the span of the axis, rounded up to the major unit. The
// major unit is 1, 2 or 5 times a power of ten, the smallest that gives no
// more major intervals than the length of the axis holds.

// scale is the scale of an axis of values.
type scale struct {
	min, max     float64
	major, minor float64
	// log is the base of a logarithmic scale, or zero.
	log     float64
	reverse bool
}

// frac returns the place of v on the scale, from 0 at its start to 1 at
// its end.
func (s scale) frac(v float64) float64 {
	var f float64
	if s.log > 0 {
		lo, hi := math.Log(s.min), math.Log(s.max)
		if !(v > 0) || hi <= lo {
			return math.NaN()
		}
		f = (math.Log(v) - lo) / (hi - lo)
	} else {
		if s.max <= s.min {
			return math.NaN()
		}
		f = (v - s.min) / (s.max - s.min)
	}
	if s.reverse {
		f = 1 - f
	}
	return f
}

// clamp returns v within the scale.
func (s scale) clamp(v float64) float64 {
	return max(s.min, min(s.max, v))
}

// ticks returns the values of the major ticks, or of the minor ticks.
func (s scale) ticks(minor bool) []float64 {
	if s.log > 0 {
		var out []float64
		for v := s.min; v <= s.max*(1+1e-9) && len(out) < maxTicks; v *= s.log {
			out = append(out, v)
			if minor {
				for k := 2.0; k < s.log && len(out) < maxTicks; k++ {
					if v*k < s.max {
						out = append(out, v*k)
					}
				}
			}
		}
		return out
	}
	step := s.major
	if minor {
		step = s.minor
	}
	if !(step > 0) || (s.max-s.min)/step > maxTicks {
		return []float64{s.min, s.max}
	}
	var out []float64
	first := math.Ceil(s.min/step - 1e-9)
	for i := first; ; i++ {
		v := clean(i*step, step)
		if v > s.max+step*1e-9 || len(out) >= maxTicks {
			break
		}
		out = append(out, v)
	}
	return out
}

// clean rounds v to the precision of step, so that 0.1+0.2 labels as 0.3.
func clean(v, step float64) float64 {
	if v == 0 || !(step > 0) {
		return v
	}
	digits := max(0, int(math.Ceil(-math.Log10(step)))+2)
	r, err := strconv.ParseFloat(strconv.FormatFloat(v, 'f', min(digits, 15), 64), 64)
	if err != nil {
		return v
	}
	return r
}

// niceStep returns the smallest step of 1, 2 or 5 times a power of ten that
// is not below raw.
func niceStep(raw float64) float64 {
	if !(raw > 0) || math.IsInf(raw, 0) {
		return 1
	}
	p := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 5, 10} {
		if m*p >= raw*(1-1e-9) {
			return m * p
		}
	}
	return 10 * p
}

// autoScale returns the scale of an axis for values from lo to hi, with the
// fixed limits of a. count is the largest number of major intervals that
// the length of the axis holds. zero says that the axis can start at zero,
// as the axes of values of the charts of categories do.
func autoScale(lo, hi float64, a axis, count int) scale {
	s := scale{reverse: a.reverse}
	count = max(1, min(count, 10))
	empty := math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) || (lo == 0 && hi == 0)
	if empty {
		lo, hi = 0, 1
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	if a.logBase > 0 {
		return logScale(lo, hi, a)
	}
	fixedMin, fixedMax := !math.IsNaN(a.min), !math.IsNaN(a.max)
	minV, maxV := a.min, a.max
	if fixedMin && fixedMax && maxV <= minV {
		fixedMin, fixedMax = false, false
	}
	// The automatic minimum and maximum, before the rounding to the unit.
	if !fixedMin {
		switch {
		case lo >= 0 && (lo == hi || lo < hi*5/6):
			minV = 0
		case lo >= 0:
			minV = lo
		default:
			minV = lo
		}
		if fixedMax && minV >= maxV {
			minV = maxV - 1
		}
	}
	if !fixedMax {
		switch {
		case hi <= 0 && (lo == hi || hi > lo*5/6):
			maxV = 0
		default:
			maxV = hi
		}
		if maxV <= minV {
			maxV = minV + 1
		}
	}
	span := maxV - minV
	// Excel adds a twentieth of the span above the largest value, and below
	// the smallest one, before it rounds to the unit.
	top, bottom := maxV, minV
	if !fixedMax && maxV != 0 && !empty {
		top = maxV + span/20
	}
	if !fixedMin && minV != 0 {
		bottom = minV - span/20
	}
	major := a.major
	if !(major > 0) || (top-bottom)/major > maxTicks {
		major = niceStep((top - bottom) / float64(count))
		for range 8 {
			lo2, hi2 := bottom, top
			if !fixedMin {
				lo2 = math.Floor(bottom/major+1e-9) * major
			}
			if !fixedMax {
				hi2 = math.Ceil(top/major-1e-9) * major
			}
			if (hi2-lo2)/major <= float64(count)+1e-9 {
				break
			}
			major = niceStep(major * 1.01)
		}
	}
	if !fixedMin {
		minV = clean(math.Floor(bottom/major+1e-9)*major, major)
	}
	if !fixedMax {
		maxV = clean(math.Ceil(top/major-1e-9)*major, major)
	}
	if maxV <= minV {
		maxV = minV + major
	}
	s.min, s.max, s.major = minV, maxV, major
	s.minor = a.minor
	if !(s.minor > 0) || (s.max-s.min)/s.minor > maxTicks {
		s.minor = major / 5
	}
	return s
}

// logScale returns the scale of a logarithmic axis: from the power of the
// base below the smallest value to the power above the largest.
func logScale(lo, hi float64, a axis) scale {
	b := a.logBase
	s := scale{log: b, reverse: a.reverse, major: b, minor: b}
	if !(lo > 0) {
		lo = 1
	}
	if !(hi > lo) {
		hi = lo * b
	}
	s.min = math.Pow(b, math.Floor(math.Log(lo)/math.Log(b)+1e-9))
	s.max = math.Pow(b, math.Ceil(math.Log(hi)/math.Log(b)-1e-9))
	if a.min > 0 {
		s.min = a.min
	}
	if a.max > s.min {
		s.max = a.max
	}
	if s.max <= s.min {
		s.max = s.min * b
	}
	if math.Log(s.max/s.min)/math.Log(b) > maxTicks {
		s.max = s.min * math.Pow(b, maxTicks)
	}
	return s
}

// cross returns the value where an axis crosses the axis with the scale s:
// zero when the scale holds it, else the end of the scale nearest to zero,
// or the end that crosses names.
func (s scale) cross(crosses string, at float64) float64 {
	switch {
	case !math.IsNaN(at):
		return s.clamp(at)
	case crosses == "min":
		return s.min
	case crosses == "max":
		return s.max
	case s.log > 0:
		return s.min
	}
	return s.clamp(0)
}
