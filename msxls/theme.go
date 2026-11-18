package msxls

import (
	"archive/zip"
	"bytes"
	"image/color"
	"io"
	"path"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
	"github.com/xo/ivy/internal/zipdoc"
)

// The records of the theme of a workbook of Excel 2007 and later.
const (
	recTheme         = 0x0896
	recContinueFrt12 = 0x087F
	// maxTheme is the size of the package of a theme that the workbook
	// reads, and the size of its part of the theme.
	maxTheme = 1 << 20
)

// themeNames are the colors of the theme by their index in SpreadsheetML,
// which swaps the dark and the light colors.
var themeNames = [...]string{
	"lt1", "dk1", "lt2", "dk2", "accent1", "accent2", "accent3",
	"accent4", "accent5", "accent6", "hlink", "folHlink",
}

// readTheme reads the theme of a record THEME (MS-XLS 2.4.326): the package
// of a theme of DrawingML, a zip archive that the records ContinueFrt12
// after it go on with. It returns nil for the default theme, which the
// record leaves out, and for a package that it cannot read.
func readTheme(pkg []byte, lim ivy.Limits) *drawingml.Theme {
	if len(pkg) == 0 {
		return nil
	}
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		return nil
	}
	for _, f := range zr.File {
		// the part of the theme is theme/theme/theme1.xml, beside
		// themeManager.xml
		dir, name := path.Split(f.Name)
		num := strings.TrimSuffix(strings.TrimPrefix(name, "theme"), ".xml")
		if dir != "theme/theme/" || len(num) != len(name)-9 || strings.Trim(num, "0123456789") != "" {
			continue
		}
		if theme := parseTheme(f, lim); theme != nil {
			return theme
		}
	}
	return nil
}

// parseTheme parses the part of the theme f, or returns nil.
func parseTheme(f *zip.File, lim ivy.Limits) *drawingml.Theme {
	if f.UncompressedSize64 > maxTheme {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	buf, err := io.ReadAll(io.LimitReader(rc, maxTheme))
	_ = rc.Close()
	if err != nil {
		return nil
	}
	doc, err := zipdoc.ParseXML(buf, lim)
	if err != nil {
		return nil
	}
	theme, err := drawingml.ParseTheme(doc)
	if err != nil {
		return nil
	}
	return theme
}

// chartTheme returns the theme of the charts: the theme of the workbook, or
// the theme of Office 2007.
func (w *workbook) chartTheme() *drawingml.Theme {
	if w.theme != nil {
		return w.theme
	}
	return office2007
}

// themeColor returns the color of the index i of the theme in
// SpreadsheetML: the color of the theme of the workbook, or of the theme of
// Office 2007.
func (w *workbook) themeColor(i int) (color.NRGBA, bool) {
	if i < 0 || i >= len(themeNames) {
		return color.NRGBA{}, false
	}
	if c, ok := w.theme.Color(themeNames[i]); ok {
		return c, true
	}
	t := themeColors[i]
	return color.NRGBA{uint8(t >> 16), uint8(t >> 8), uint8(t), 255}, true
}
