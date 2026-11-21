// Package fonts holds fonts with the metrics of the fonts of Microsoft
// Office, under the SIL Open Font License 1.1 (D24). Each font has the same
// advance widths as the font of Office that it replaces, so that a line of a
// document breaks at the same word and a page holds the same lines.
//
// The package also holds Droid Sans Fallback, under the Apache License 2.0.
// It draws the Chinese, Japanese and Korean (CJK) characters that the other
// fonts do not draw.
//
// The package also holds Latin Modern and Latin Modern Math, the fonts of
// TeX, under the GUST Font License (D62). TeX gives them apart from FS, so
// that the other formats do not take them for a font of Office. The package
// latex sets the text and the math of a LaTeX document with them, as TeX
// does:
//
//	cfg := ivy.Config{Fonts: fonts.FS(), TeX: fonts.TeX()}
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
// and does not embed that font. A format uses Droid Sans Fallback for a CJK
// character that the font of the text does not draw, and for the CJK text
// of a PDF file that does not embed its font.
//
// The fonts are in their own module, so that a program that does not import
// it does not hold them. They make a program larger by about 14.1 MB.
package fonts

import (
	"embed"
	"io/fs"
)

// files holds the folders of the fonts.
//
//go:embed liberation carlito caladea droid
var files embed.FS

// tex holds the folder of Latin Modern.
//
//go:embed lm
var tex embed.FS

// FS returns the font files and their licenses. Each folder holds the
// TrueType files of one project, and the license of the project. The fonts
// of Office have four styles each: regular, bold, italic and bold italic.
//
//   - The folder liberation holds Liberation Sans, Liberation Serif and
//     Liberation Mono, and the license in LICENSE.
//   - The folder carlito holds Carlito, and the license in OFL.txt.
//   - The folder caladea holds Caladea, and the license in OFL.txt.
//   - The folder droid holds Droid Sans Fallback in the regular style only,
//     and the license in NOTICE.
//
// Each call returns the same value, so a format can keep the fonts that it
// read from it.
func FS() fs.FS {
	return files
}

// TeX returns the fonts of TeX, in the folder lm: the OpenType files of
// Latin Modern Roman and Latin Modern Mono at their optical sizes, and Latin
// Modern Math. The license is in GUST-FONT-LICENSE.txt, the notices of the
// authors are in README-Latin-Modern.txt and README-Latin-Modern-Math.txt,
// and the manifests are in MANIFEST-Latin-Modern.txt and
// MANIFEST-Latin-Modern-Math.txt. A program gives them to the format latex
// with the field TeX of ivy.Config, as the tree of a TeX distribution holds
// them (D65).
//
// Each call returns the same value.
func TeX() fs.FS {
	return tex
}
