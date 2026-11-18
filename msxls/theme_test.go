package msxls

import (
	"archive/zip"
	"bytes"
	"image/color"
	"testing"

	"github.com/xo/ivy"
)

// themePackage returns the package of a theme whose accent1 is accent, as a
// record THEME holds it.
func themePackage(t *testing.T, accent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := []struct{ name, data string }{
		{"theme/theme/themeManager.xml", `<?xml version="1.0"?><a:themeManager xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`},
		{"theme/theme/theme1.xml", `<?xml version="1.0"?><a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Test"><a:themeElements>` +
			`<a:clrScheme name="Test"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
			`<a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2><a:accent1><a:srgbClr val="` + accent + `"/></a:accent1>` +
			`<a:accent2><a:srgbClr val="ED7D31"/></a:accent2><a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4>` +
			`<a:accent5><a:srgbClr val="4472C4"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6><a:hlink><a:srgbClr val="0563C1"/></a:hlink>` +
			`<a:folHlink><a:srgbClr val="954F72"/></a:folHlink></a:clrScheme>` +
			`<a:fontScheme name="Test"><a:majorFont><a:latin typeface="Calibri Light"/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/></a:minorFont></a:fontScheme>` +
			`</a:themeElements></a:theme>`},
	}
	for _, p := range parts {
		w, err := zw.Create(p.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(p.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTheme(t *testing.T) {
	pkg := themePackage(t, "5B9BD5")
	// the record THEME holds the start of the package, and a record
	// ContinueFrt12 holds the rest
	half := len(pkg) / 2
	var b biff
	b.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	b.record(recTheme, frtHeader(recTheme), u32(0), pkg[:half])
	b.record(recContinueFrt12, frtHeader(recContinueFrt12), pkg[half:])
	b.record(recEOF)
	w, err := readWorkbook(t.Context(), b.buf, ivy.Config{Limits: ivy.Limits{}.WithDefaults()})
	if err != nil {
		t.Fatal(err)
	}
	if w.theme == nil || w.chartTheme() != w.theme {
		t.Fatal("expected the theme of the record THEME")
	}
	// the index 4 of SpreadsheetML is accent1, and 0 is lt1
	for i, want := range map[int]color.NRGBA{4: {0x5B, 0x9B, 0xD5, 255}, 0: {255, 255, 255, 255}, 3: {0x44, 0x54, 0x6A, 255}} {
		if c, ok := w.themeColor(i); !ok || c != want {
			t.Errorf("theme color %d: %v, want %v", i, c, want)
		}
	}
	if _, ok := w.themeColor(12); ok {
		t.Error("expected no theme color 12")
	}
	// without the record, the workbook takes the theme of Office 2007
	var d biff
	d.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	d.record(recTheme, frtHeader(recTheme), u32(124226))
	d.record(recEOF)
	w, err = readWorkbook(t.Context(), d.buf, ivy.Config{Limits: ivy.Limits{}.WithDefaults()})
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := w.themeColor(4); w.theme != nil || w.chartTheme() != office2007 || c != (color.NRGBA{0x4F, 0x81, 0xBD, 255}) {
		t.Errorf("default theme: %v, accent1 %v", w.theme, c)
	}
	// a package that is not a zip archive gives no theme
	if readTheme([]byte("not a zip archive"), ivy.Limits{}) != nil {
		t.Error("expected no theme of a broken package")
	}
}
