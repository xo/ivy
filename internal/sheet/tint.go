package sheet

import (
	"image/color"
	"math"
)

// Tint applies a tint of SpreadsheetML to c: it changes the lightness of c
// in the HLS space, toward black for a negative t and toward white for a
// positive one (ECMA-376 Part 1, 18.8.19). t goes from -1 to 1.
func Tint(c color.NRGBA, t float64) color.NRGBA {
	t = math.Max(-1, math.Min(1, t))
	h, l, sat := toHLS(c)
	if t < 0 {
		l *= 1 + t
	} else {
		l = l*(1-t) + t
	}
	return fromHLS(h, l, sat)
}

// toHLS converts c to hue, lightness and saturation, each from 0 to 1.
func toHLS(c color.NRGBA) (float64, float64, float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l := (hi + lo) / 2
	if hi == lo {
		return 0, l, 0
	}
	d := hi - lo
	s := d / (hi + lo)
	if l > 0.5 {
		s = d / (2 - hi - lo)
	}
	var h float64
	switch hi {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, l, s
}

// fromHLS converts hue, lightness and saturation back to a color.
func fromHLS(h, l, s float64) color.NRGBA {
	l = math.Max(0, math.Min(1, l))
	if s == 0 {
		v := uint8(math.Round(l * 255))
		return color.NRGBA{v, v, v, 255}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	conv := func(t float64) uint8 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		var v float64
		switch {
		case t < 1.0/6:
			v = p + (q-p)*6*t
		case t < 0.5:
			v = q
		case t < 2.0/3:
			v = p + (q-p)*(2.0/3-t)*6
		default:
			v = p
		}
		return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255))
	}
	return color.NRGBA{conv(h + 1.0/3), conv(h), conv(h - 1.0/3), 255}
}
