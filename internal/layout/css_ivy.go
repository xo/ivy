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

package layout

// ivyPrefix is the prefix of the properties that ivy adds to the layout.
const ivyPrefix = "-ivy-"

// ivyProperties are the properties that ivy adds to the layout, by name in
// kebab case. They are only known with the prefix -ivy-.
var ivyProperties = map[string]bool{
	"tab-stops":      true,
	"margin-box-fit": true,
	"static-top":     true,
	"text-autospace": true,
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
