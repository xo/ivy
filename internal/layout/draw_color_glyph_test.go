// This file is not a port. It tests the drawing of the color glyphs of the
// tables COLR and SVG of a font, such as the color emojis, which upstream
// draws with Cairo.

package layout

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	ivyfont "github.com/xo/ivy/internal/font"
)

// svgTable returns a table SVG with one document, for the glyph gid only.
func svgTable(gid uint16, doc string) []byte {
	// The header: the version, the offset of the list of the documents, and
	// a reserved field.
	b := binary.BigEndian.AppendUint16(nil, 0)
	b = binary.BigEndian.AppendUint32(b, 10)
	b = binary.BigEndian.AppendUint32(b, 0)
	// The list of the documents: one record, then the document, at an
	// offset from the start of the list.
	b = binary.BigEndian.AppendUint16(b, 1)
	b = binary.BigEndian.AppendUint16(b, gid)
	b = binary.BigEndian.AppendUint16(b, gid)
	b = binary.BigEndian.AppendUint32(b, 2+12)
	b = binary.BigEndian.AppendUint32(b, uint32(len(doc)))
	return append(b, doc...)
}

// colorPixels counts the pixels of a page that are near the color of want,
// within 40 for each channel.
func colorPixels(pixels []rgb, want rgb) int {
	n := 0
	near := func(a, b int) bool { return max(a, b)-min(a, b) <= 40 }
	for _, p := range pixels {
		if near(p[0], want[0]) && near(p[1], want[1]) && near(p[2], want[2]) {
			n++
		}
	}
	return n
}

// TestColorGlyphSVG checks that a glyph of the table SVG draws its
// document, with the colors of the document, and not its outline.
func TestColorGlyphSVG(t *testing.T) {
	plain, err := ivyfont.Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	gid, _ := plain.Glyph('H')
	// A green square of 1024 by 1024 font units, half an em, on the
	// baseline. y grows down in the document.
	doc := `<svg xmlns="http://www.w3.org/2000/svg"><rect x="0" y="-1024" width="1024" height="1024" fill="#00ff00"/></svg>`
	font := fontWithTables(goregular.TTF, map[string][]byte{"SVG ": svgTable(uint16(gid), doc)})
	data := base64.StdEncoding.EncodeToString(font)
	_, _, pixels := htmlToPixels(t, `
      <style>
        @font-face { src: url(data:font/ttf;base64,`+data+`); font-family: color }
        @page { size: 40px 40px }
        body { margin: 0; color: red; font: 32px/40px color }
      </style>H`)
	// The square is 16 by 16 pixels.
	if green := colorPixels(pixels, rgb{0, 255, 0}); green < 200 || green > 300 {
		t.Errorf("expected the 256 green pixels of the square, got %d", green)
	}
	if red := colorPixels(pixels, rgb{255, 0, 0}); red != 0 {
		t.Errorf("expected no red pixel of the outline, got %d", red)
	}
}

// TestColorGlyphCOLR checks that an emoji of the table COLR, version 1,
// draws in its colors. Its glyph has no outline, so it drew nothing before
// okage D57.
func TestColorGlyphCOLR(t *testing.T) {
	page := func(text string) []rgb {
		_, _, pixels := htmlToPixels(t, `
      <style>
        @font-face { src: url(NotoColorEmojiCOLR.subset.ttf); font-family: emoji }
        @page { size: 50px 50px }
        body { margin: 0; color: black; font: 40px/50px emoji }
      </style>`+text)
		return pixels
	}
	ink := func(pixels []rgb) (colored, dark int) {
		for _, p := range pixels {
			lo, hi := min(p[0], p[1], p[2]), max(p[0], p[1], p[2])
			switch {
			case hi-lo > 60:
				colored++
			case hi < 100:
				dark++
			}
		}
		return colored, dark
	}
	// U+1F436 is the face of a dog, and U+1F41D a honeybee.
	for _, r := range []rune{0x1F436, 0x1F41D} {
		colored, _ := ink(page(string(r)))
		if colored < 150 {
			t.Errorf("%U: expected a color glyph, got %d pixels in color", r, colored)
		}
	}
	// A blank text draws nothing.
	if colored, dark := ink(page(" ")); colored != 0 || dark != 0 {
		t.Errorf("expected a blank page, got %d pixels in color and %d dark ones", colored, dark)
	}
}

// TestColorGlyphOpacity checks that the alpha of the color of the text is
// the opacity of a color glyph, as for a bitmap glyph.
func TestColorGlyphOpacity(t *testing.T) {
	plain, err := ivyfont.Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	gid, _ := plain.Glyph('H')
	doc := `<svg xmlns="http://www.w3.org/2000/svg"><rect x="0" y="-1024" width="1024" height="1024" fill="#0000ff"/></svg>`
	font := fontWithTables(goregular.TTF, map[string][]byte{"SVG ": svgTable(uint16(gid), doc)})
	data := base64.StdEncoding.EncodeToString(font)
	_, _, pixels := htmlToPixels(t, `
      <style>
        @font-face { src: url(data:font/ttf;base64,`+data+`); font-family: color }
        @page { size: 40px 40px }
        body { margin: 0; color: rgba(255, 0, 0, 0.5); font: 32px/40px color }
      </style>H`)
	if half := colorPixels(pixels, rgb{127, 127, 255}); half < 200 {
		t.Errorf("expected the light blue pixels of the square at half its opacity, got %d", half)
	}
}

// TestFontVariations checks that the variations of a style hold the axes of
// font-variation-settings, and the axes of its weight, its stretch and its
// style that okage adds.
func TestFontVariations(t *testing.T) {
	for _, c := range []struct {
		css  string
		want string
	}{
		{"p { font-weight: bold }", "wght700 "},
		{"p { font-stretch: condensed; font-style: italic }", "wght400 wdth75 ital1 "},
		{"p { font-style: oblique }", "wght400 slnt-12 "},
		{`p { font-weight: bold; font-variation-settings: "wght" 300, "GRAD" 50 }`, "wght300 GRAD50 "},
	} {
		style, _ := fontFaceStyle(t, c.css)
		var b strings.Builder
		for _, v := range fontVariations(style) {
			fmt.Fprintf(&b, "%s%g ", v.Tag[:], v.Value)
		}
		if got := b.String(); got != c.want {
			t.Errorf("%s: expected %q, got: %q", c.css, c.want, got)
		}
	}
}
