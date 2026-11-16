// Package fonts holds fonts with the metrics of the fonts of Microsoft
// Office, under the SIL Open Font License 1.1 (D24). Each font has the same
// advance widths as the font of Office that it replaces, so that a line of a
// document breaks at the same word and a page holds the same lines.
//
// A program gives the fonts to a format with the field Fonts of ivy.Config:
//
//	cfg := ivy.Config{Fonts: fonts.FS()}
//
// The fonts, and the fonts of Office that they replace, are:
//
//   - Liberation Sans replaces Arial and Helvetica.
//   - Liberation Serif replaces Times New Roman and Times.
//   - Liberation Mono replaces Courier New and Courier.
//   - Carlito replaces Calibri.
//   - Caladea replaces Cambria.
//
// A format that lays out text uses a font of Config.Fonts when a document
// names it, or when a document names the font of Office that it replaces
// and does not embed that font.
//
// The fonts are in their own module, so that a program that does not import
// it does not hold them. They make a program larger by about 7.4 MB.
package fonts

import (
	"embed"
	"io/fs"
)

// files holds the folders of the fonts.
//
//go:embed liberation carlito caladea
var files embed.FS

// FS returns the font files and their licenses. Each folder holds the
// TrueType files of one project, in four styles each: regular, bold, italic
// and bold italic. It also holds the license of the project.
//
//   - The folder liberation holds Liberation Sans, Liberation Serif and
//     Liberation Mono, and the license in LICENSE.
//   - The folder carlito holds Carlito, and the license in OFL.txt.
//   - The folder caladea holds Caladea, and the license in OFL.txt.
//
// Each call returns the same value, so a format can keep the fonts that it
// read from it.
func FS() fs.FS {
	return files
}
