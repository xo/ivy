package pptx

import (
	_ "embed"
	"sync"

	"github.com/xo/okage/roxmltree"
)

// mediumStyle2Accent1 is the ID of the table style "Medium Style 2 -
// Accent 1", the default table style of PowerPoint.
const mediumStyle2Accent1 = "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"

// mediumStyle2Accent1XML is the definition of the table style
// mediumStyle2Accent1, as PowerPoint writes it into ppt/tableStyles.xml. A
// file that names the style and does not define it draws its tables with
// it, as PowerPoint does: PowerPoint knows its own styles. The definition
// is a copy from the file table_test2.pptx of the test data of Apache POI,
// under the Apache License 2.0, which the folder tablestyles keeps.
//
//go:embed tablestyles/medium-style-2-accent-1.xml
var mediumStyle2Accent1XML string

// builtinStyle holds the table style mediumStyle2Accent1, which the
// package reads once.
var builtinStyle = sync.OnceValues(func() (roxmltree.Node, bool) {
	doc, err := roxmltree.Parse(mediumStyle2Accent1XML)
	if err != nil {
		return roxmltree.Node{}, false
	}
	return doc.RootElement(), true
})

// builtinTableStyle returns the table style of PowerPoint with the ID that
// the package knows: the default table style.
func builtinTableStyle(id string) (roxmltree.Node, bool) {
	if id != mediumStyle2Accent1 {
		return roxmltree.Node{}, false
	}
	return builtinStyle()
}
