// This file is not a port. It holds the properties that ivy adds to the
// layout, for the formats of the word processors. Upstream does not have
// them, and a style sheet names each one with the prefix -ivy-:
//
//   - -ivy-tab-stops places the tabs of a text at the tab stops of a
//     paragraph, see layout_tabs.go.
//   - -ivy-margin-box-fit grows the margins of a page to hold its margin
//     boxes, see layout_page_fit.go.
//   - -ivy-static-top places an absolutely positioned box of a line from
//     the top of its block container on the page.
//   - -ivy-text-autospace adds a space between ideographs and Latin
//     letters or digits, see text_autospace.go.
//   - -ivy-page-overflow lets the last line of a page, when it has no
//     text, pass the bottom of the page by up to its descent, see
//     lineboxLayout (D52).
//   - -ivy-line-numbers draws the numbers of the lines of a section in
//     the margin, see numberLines (D56).
//   - -ivy-line-multiple puts the line gap of a single line above the
//     text of a line, and the extra space of a multiple line spacing below
//     it, see leadingShift (D56).
//   - -ivy-page-top-space leaves out the space before the first block of
//     a page, of a column or of the part of a split table cell after a
//     break that is not forced, see layoutContext.discardsSpaceBefore
//     (D56).
//   - -ivy-float-position places a float of the flow of the page at a
//     vertical position from the top of the page or of the page area, see
//     layout_page_float.go (D60).
//   - -ivy-float-overlap moves such a float down below the floats of the
//     page that it covers, see layout_page_float.go (D60).
//   - -ivy-wrap-flow puts the text on both sides of a float, or on one
//     side of it, as a word processor wraps the text around a picture,
//     see layout_wrap_flow.go (D63).
//   - -ivy-stroke-adjust puts the collapsed borders of a table cell on
//     whole pixels, as a spreadsheet draws its grid, see
//     drawCollapsedBorders.
//   - -ivy-line-baseline puts the baseline of a line where PowerPoint
//     puts it, from the Windows metrics of the font, see baselineShift
//     (D71).

package layout

import "github.com/xo/okage/tinycss2"

// ivyPrefix is the prefix of the properties that ivy adds to the layout.
const ivyPrefix = "-ivy-"

// ivyProperties are the properties that ivy adds to the layout, by name in
// kebab case. They are only known with the prefix -ivy-.
var ivyProperties = map[string]bool{
	"tab-stops":      true,
	"margin-box-fit": true,
	"static-top":     true,
	"text-autospace": true,
	"page-overflow":  true,
	"line-numbers":   true,
	"line-multiple":  true,
	"page-top-space": true,
	"float-position": true,
	"float-overlap":  true,
	"wrap-flow":      true,
	"stroke-adjust":  true,
	"line-baseline":  true,
}

// validateMarginBoxFit validates -ivy-margin-box-fit: 'none' or 'grow'.
var validateMarginBoxFit = singleKeyword(func(keyword string) bool {
	return keyword == "none" || keyword == "grow"
})

// validateStaticTop validates -ivy-static-top: 'auto' or 'block'. With
// 'block', the static position of an absolutely positioned box of a line
// is at the top of the part of the block container of the line on its
// page, as a word processor places a shape from the top of the paragraph
// that anchors it.
var validateStaticTop = singleKeyword(func(keyword string) bool {
	return keyword == "auto" || keyword == "block"
})

// validatePageOverflow validates -ivy-page-overflow: 'none' or 'descent'.
// With 'descent', the last line of a page can pass the bottom of the page
// by up to its descent when it has no text, as the line of an empty
// paragraph or of a paragraph of spaces, see lineDescent. Word keeps such
// a line on the page (D52). The property is inherited.
var validatePageOverflow = singleKeyword(func(keyword string) bool {
	return keyword == "none" || keyword == "descent"
})

// validateLineNumbers validates -ivy-line-numbers: 'none', or the distance
// of the numbers from the text, the step, the first number and when the
// numbers restart, in this order, such as '18pt 1 1 page'. The distance is
// a length that is not negative. The step and the first number are
// integers from 1. The restart is 'page', 'section' or 'continuous'. The
// property is inherited.
func validateLineNumbers(tokens []tinycss2.Node, _ string) (Value, error) {
	if getSingleKeyword(tokens) == "none" {
		return String("none"), nil
	}
	if len(tokens) != 4 {
		return nil, nil
	}
	distance, ok := getLength(tokens[0], false, false).(Dimension)
	if !ok {
		return nil, nil
	}
	var counts [2]Value
	for i, token := range tokens[1:3] {
		n, ok := getNumber(token, false, true).(Dimension)
		if !ok || n.Value < 1 || n.Value > maxLineNumber {
			return nil, nil
		}
		counts[i] = Int(n.Value)
	}
	restart := getKeyword(tokens[3])
	if !in(restart, "page", "section", "continuous") {
		return nil, nil
	}
	return Tuple{distance, counts[0], counts[1], String(restart)}, nil
}

// computeLineNumbers computes -ivy-line-numbers: the distance in pixels.
func computeLineNumbers(style *Style, name string, value Value) Value {
	t, ok := value.(Tuple)
	if !ok || len(t) != 4 {
		return value
	}
	d, ok := t[0].(Dimension)
	if !ok {
		return value
	}
	return Tuple{Float(toPixels(d, style, name, -1)), t[1], t[2], t[3]}
}

// validateLineMultiple validates -ivy-line-multiple: 'none' or a number
// that is not negative, the multiple of a single line that the line height
// of a word processor is. The property is inherited.
func validateLineMultiple(tokens []tinycss2.Node, _ string) (Value, error) {
	if getSingleKeyword(tokens) == "none" {
		return String("none"), nil
	}
	if len(tokens) != 1 {
		return nil, nil
	}
	if n, ok := getNumber(tokens[0], false, false).(Dimension); ok {
		return Float(n.Value), nil
	}
	return nil, nil
}

// validateLineBaseline validates -ivy-line-baseline: 'normal' or 'win'.
// With 'win', the baseline of a line is at the share of the height of the
// line that the Windows ascent of the font takes of the Windows ascent and
// descent, as in PowerPoint (D71). The property is inherited.
var validateLineBaseline = singleKeyword(func(keyword string) bool {
	return keyword == "normal" || keyword == "win"
})

// validatePageTopSpace validates -ivy-page-top-space: 'keep' or 'discard'.
// With 'discard', a block of the flow of the page that starts a page after
// a break that is not forced has no space before it, as a paragraph of
// Word (D56). So does a block at the top of a column after a break that is
// not forced. The property is inherited.
var validatePageTopSpace = singleKeyword(func(keyword string) bool {
	return keyword == "keep" || keyword == "discard"
})

// validateFloatPosition validates -ivy-float-position: 'auto', or 'page'
// or 'margin' and a length, 'top', 'center' or 'bottom', in any order,
// such as 'margin 1in' or 'bottom page'. The value is a tuple of the
// keyword of the area and of the length or the keyword of the position.
// The length can be negative. The property is not inherited (D60).
func validateFloatPosition(tokens []tinycss2.Node, _ string) (Value, error) {
	if getSingleKeyword(tokens) == "auto" {
		return String("auto"), nil
	}
	if len(tokens) != 2 {
		return nil, nil
	}
	var area, position Value
	for _, token := range tokens {
		switch keyword := getKeyword(token); {
		case in(keyword, "page", "margin"):
			if area != nil {
				return nil, nil
			}
			area = String(keyword)
		case in(keyword, "top", "center", "bottom"):
			if position != nil {
				return nil, nil
			}
			position = String(keyword)
		default:
			length, ok := getLength(token, true, false).(Dimension)
			if !ok || position != nil {
				return nil, nil
			}
			position = length
		}
	}
	if area == nil || position == nil {
		return nil, nil
	}
	return Tuple{area, position}, nil
}

// validateFloatOverlap validates -ivy-float-overlap: 'allow' or 'avoid'.
// With 'avoid', a page float of -ivy-float-position moves down below the
// page floats before it that it covers, as LibreOffice moves a table
// with w:tblOverlap 'never'. The property is not inherited (D60).
var validateFloatOverlap = singleKeyword(func(keyword string) bool {
	return keyword == "allow" || keyword == "avoid"
})

// computeFloatPosition computes -ivy-float-position: a length in pixels.
func computeFloatPosition(style *Style, name string, value Value) Value {
	t, ok := value.(Tuple)
	if !ok || len(t) != 2 {
		return value
	}
	if d, ok := t[1].(Dimension); ok {
		return Tuple{t[0], Float(toPixels(d, style, name, -1))}
	}
	return value
}

// validateWrapFlow validates -ivy-wrap-flow: 'auto', or 'both', 'start',
// 'end' or 'maximum' and an optional length that is not negative, in any
// order, such as 'both 9pt'. The value is a tuple of the keyword and of
// the length, which is 0 when the value has none. The property is not
// inherited (D63).
func validateWrapFlow(tokens []tinycss2.Node, _ string) (Value, error) {
	if getSingleKeyword(tokens) == "auto" {
		return String("auto"), nil
	}
	if len(tokens) == 0 || len(tokens) > 2 {
		return nil, nil
	}
	var side, distance Value
	for _, token := range tokens {
		if keyword := getKeyword(token); in(keyword, "both", "start", "end", "maximum") {
			if side != nil {
				return nil, nil
			}
			side = String(keyword)
			continue
		}
		length, ok := getLength(token, false, false).(Dimension)
		if !ok || distance != nil {
			return nil, nil
		}
		distance = length
	}
	if side == nil {
		return nil, nil
	}
	if distance == nil {
		distance = Dimension{Value: 0, Unit: "px"}
	}
	return Tuple{side, distance}, nil
}

// computeWrapFlow computes -ivy-wrap-flow: the length in pixels.
func computeWrapFlow(style *Style, name string, value Value) Value {
	t, ok := value.(Tuple)
	if !ok || len(t) != 2 {
		return value
	}
	if d, ok := t[1].(Dimension); ok {
		return Tuple{t[0], Float(toPixels(d, style, name, -1))}
	}
	return value
}

// validateStrokeAdjust validates -ivy-stroke-adjust: 'none' or 'pixels'.
// With 'pixels', a collapsed border that a box gives to the grid of its
// table is drawn with the stroke adjustment of PDF: on a device of pixels,
// it is a whole number of pixels wide, at least 1, and its edges fall on
// the borders of the pixels. A thin line then covers one row or one column
// of pixels, and does not spread over two light ones. The property is not
// inherited.
var validateStrokeAdjust = singleKeyword(func(keyword string) bool {
	return keyword == "none" || keyword == "pixels"
})
