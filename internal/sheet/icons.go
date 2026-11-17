package sheet

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// This file draws the icons of the icon sets of the conditional formats as
// small SVG images: arrows, circles, the quarters of a circle and the bars
// of a rating, in the colors of Excel.

// The colors of the icons.
const (
	iconRed    = "#d9302c"
	iconYellow = "#f2c12e"
	iconGreen  = "#4caf50"
	iconGray   = "#8c8c8c"
	iconBlack  = "#262626"
	iconPink   = "#f19c9c"
	iconBlue   = "#4472c4"
)

// iconPath returns the path in the files of the layout of the icon of an
// icon set with count icons, of the index from the lowest.
func iconPath(name string, index, count int) string {
	return "cf/" + strings.ToLower(name) + "-" + strconv.Itoa(count) + "-" + strconv.Itoa(index) + ".svg"
}

// iconSVG returns the SVG image of the icon of an icon set with count
// icons, of the index from the lowest.
func iconSVG(name string, index, count int) []byte {
	lower := strings.ToLower(name)
	var body string
	switch {
	case strings.Contains(lower, "arrows"):
		body = arrow(index, count, strings.Contains(lower, "gray"))
	case strings.Contains(lower, "quarters"):
		body = quarters(index, count)
	case strings.Contains(lower, "rating"), strings.Contains(lower, "boxes"):
		body = rating(index, count)
	default:
		body = `<circle cx="8" cy="8" r="6.5" fill="` + circleColor(lower, index, count) + `"/>`
	}
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16">` + body + `</svg>`)
}

// arrow returns an arrow from down, for the lowest icon, to up.
func arrow(index, count int, gray bool) string {
	// the angle from down to up, in degrees clockwise from up
	angle := 180.0
	if count > 1 {
		angle = 180 - 180*float64(index)/float64(count-1)
	}
	col := iconYellow
	switch {
	case gray:
		col = iconGray
	case index == 0:
		col = iconRed
	case index == count-1:
		col = iconGreen
	}
	return `<g transform="rotate(` + strconv.FormatFloat(angle, 'f', 1, 64) + ` 8 8)" fill="` + col + `">` +
		`<path d="M8 1.5 L14 8 H10.5 V14.5 H5.5 V8 H2 Z"/></g>`
}

// circleColor returns the color of a circle of an icon set, such as the
// traffic lights, from red, for the lowest icon, to green.
func circleColor(name string, index, count int) string {
	switch {
	case strings.Contains(name, "redtoblack"):
		return []string{iconBlack, iconGray, iconPink, iconRed}[min(index, 3)]
	case count >= 4:
		return []string{iconBlack, iconRed, iconYellow, iconGreen}[min(index, 3)]
	case index == 0:
		return iconRed
	case index == count-1:
		return iconGreen
	}
	return iconYellow
}

// quarters returns a circle filled by quarters: empty for the lowest icon,
// full for the highest.
func quarters(index, count int) string {
	f := 1.0
	if count > 1 {
		f = float64(index) / float64(count-1)
	}
	out := `<circle cx="8" cy="8" r="6.5" fill="#ffffff" stroke="` + iconBlack + `" stroke-width="1"/>`
	switch {
	case f >= 1:
		out += `<circle cx="8" cy="8" r="6.5" fill="` + iconBlack + `"/>`
	case f > 0:
		a := 2 * math.Pi * f
		x, y := 8+6.5*math.Sin(a), 8-6.5*math.Cos(a)
		large := 0
		if f > 0.5 {
			large = 1
		}
		out += fmt.Sprintf(`<path d="M8 8 V1.5 A6.5 6.5 0 %d 1 %.2f %.2f Z" fill="%s"/>`, large, x, y, iconBlack)
	}
	return out
}

// rating returns the bars of a rating: no bar for the lowest icon, all for
// the highest.
func rating(index, count int) string {
	var b strings.Builder
	bars := 4
	for k := range bars {
		h := 3 + 3*float64(k)
		col := "#d9d9d9"
		if count > 1 && float64(k) < float64(index)*float64(bars)/float64(count-1) {
			col = iconBlue
		}
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="2.5" height="%.1f" fill="%s"/>`, 1.5+3.5*float64(k), 14.5-h, h, col)
	}
	return b.String()
}
