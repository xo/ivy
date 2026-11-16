package drawingml

import (
	"image/color"
	"testing"

	"github.com/xo/okage/roxmltree"
)

const themeXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">
<a:themeElements>
<a:clrScheme name="Office">
<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>
<a:lt1><a:sysClr val="window"/></a:lt1>
<a:dk2><a:srgbClr val="1F497D"/></a:dk2>
<a:accent1><a:srgbClr val="4F81BD"/></a:accent1>
<a:hlink><a:srgbClr val="0000FF"/></a:hlink>
</a:clrScheme>
<a:fontScheme name="Office">
<a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>
<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface="MS Mincho"/><a:cs typeface=""/></a:minorFont>
</a:fontScheme>
</a:themeElements>
</a:theme>`

func TestParseTheme(t *testing.T) {
	doc, err := roxmltree.Parse(themeXML[len(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)+1:])
	if err != nil {
		t.Fatal(err)
	}
	theme, err := ParseTheme(doc)
	if err != nil {
		t.Fatal(err)
	}
	for name, exp := range map[string]color.NRGBA{
		"dk1":         {0, 0, 0, 255},
		"tx1":         {0, 0, 0, 255},
		"lt1":         {255, 255, 255, 255},
		"background1": {255, 255, 255, 255},
		"dk2":         {0x1f, 0x49, 0x7d, 255},
		"accent1":     {0x4f, 0x81, 0xbd, 255},
		"hyperlink":   {0, 0, 255, 255},
	} {
		if c, ok := theme.Color(name); !ok || c != exp {
			t.Errorf("%s: expected %v, got: %v, %t", name, exp, c, ok)
		}
	}
	if _, ok := theme.Color("accent6"); ok {
		t.Error("expected no accent6")
	}
	if theme.Major.Latin != "Calibri Light" || theme.Minor.Latin != "Calibri" || theme.Minor.EastAsian != "MS Mincho" {
		t.Errorf("unexpected fonts: %+v %+v", theme.Major, theme.Minor)
	}
}
