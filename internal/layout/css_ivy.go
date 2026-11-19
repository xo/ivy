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
//   - -ivy-line-multiple puts the extra space of a multiple line spacing
//     below the text of a line, see leadingShift (D56).
//   - -ivy-page-top-space leaves out the space before the first block of
//     a page after a break that is not forced, see
//     layoutContext.discardsSpaceBefore (D56).

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

// validatePageTopSpace validates -ivy-page-top-space: 'keep' or 'discard'.
// With 'discard', a block of the flow of the page that starts a page after
// a break that is not forced has no space before it, as a paragraph of
// Word (D56). The property is inherited.
var validatePageTopSpace = singleKeyword(func(keyword string) bool {
	return keyword == "keep" || keyword == "discard"
})
