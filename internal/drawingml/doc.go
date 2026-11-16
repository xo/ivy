// Package drawingml holds the code that the formats share to draw
// DrawingML and the shapes of the slides: PowerPoint, Word, Excel, and the
// presentations and the drawings of OpenDocument.
//
// A geometry is a preset shape of ECMA-376, such as rect or star5, or a
// custom geometry from a:custGeom. It has adjust values, guides, paths and
// a text rectangle. Evaluate computes the guides for the size of a shape
// and returns its paths as okage paths, each with its fill mode and its
// stroke flag, and the text rectangle.
//
// Colors resolves the colors of DrawingML through the theme and the color
// map of a slide, with their transforms, such as lumMod and tint. A Paint is
// a fill: solid, a gradient, a picture, a pattern or none. A Line is an
// outline, with its dashes and its arrows. A Picture is a raster image, a
// metafile or SVG. A TextBody is a body of text with the styles of its
// paragraphs and its runs resolved.
//
// A Figure is a shape with its properties resolved. A Scene holds the
// figures of a page with fixed positions, such as a slide, and draws them
// onto the display list of D10. Tree reads a shape tree of DrawingML, such
// as p:spTree, into a scene: the shapes, the pictures, the groups, the
// connectors and the tables. The text goes through internal/layout: a text
// body, or a table, is a small document of HTML and CSS of the size of its
// box, so the text has the line breaks, the shaping and the fonts of the
// ebooks. The package draws an outer shadow, and leaves out the other
// effects. It does not draw the charts.
//
// The tables of the preset shapes in preset_table.go come from
// presetShapeDefinitions.xml of ECMA-376 Part 1. The file is in
// testdata/ecma, with the copyright notice of Ecma (D24). The test
// TestPresetTable generates the tables. Run "go generate
// ./internal/drawingml" after a change to the file or to the generator.
package drawingml

//go:generate go test -run ^TestPresetTable$ -update .
