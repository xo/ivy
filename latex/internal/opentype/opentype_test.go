package opentype

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
)

// lm reads a font of Latin Modern from the module fonts.
func lm(t *testing.T, name string) *Font {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fonts", "lm", name))
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestMetrics(t *testing.T) {
	t.Parallel()
	f := lm(t, "lmroman10-regular.otf")
	if f.UnitsPerEm() != 1000 {
		t.Errorf("expected 1000 units per em, got: %d", f.UnitsPerEm())
	}
	fc := f.NewFaceAt(10)
	if s := fc.Scale(); s != 0.01 {
		t.Errorf("expected a scale of 0.01, got: %g", s)
	}
	// the space of lmroman10 is 333 units, 3.33pt at 10pt
	if w := fc.AdvanceUnits(' '); w < 3.3 || w > 3.4 {
		t.Errorf("expected a space of 3.33pt, got: %g", w)
	}
	if w := fc.AdvanceUnits('\U0010FFFF'); w != 0 {
		t.Errorf("expected no advance for a character without a glyph, got: %g", w)
	}
	gid, ok := f.GlyphIndex('A')
	if !ok {
		t.Fatal("expected a glyph for A")
	}
	segs, ok := fc.GlyphOutline(gid)
	if !ok || len(segs) == 0 || segs[0].Op != 'M' {
		t.Fatalf("expected the outline of A, got: %v %v", segs, ok)
	}
	d, ok := fc.GlyphSVGPath(gid)
	if !ok || !strings.HasPrefix(d, "M") || !strings.HasSuffix(d, "Z") {
		t.Errorf("expected the SVG path of A, got: %q", d)
	}
	if f.DrawFace() == nil {
		t.Error("expected a face that draws")
	}
}

func TestKerning(t *testing.T) {
	t.Parallel()
	fc := lm(t, "lmroman10-regular.otf").NewFaceAt(10)
	// Latin Modern kerns "AV" by -0.111em, as cmr10 does
	if k := fc.KernUnits('A', 'V'); k > -100 || k < -120 {
		t.Errorf("expected a kerning of about -111 units for AV, got: %g", k)
	}
	if k := fc.KernUnits('a', 'b'); k != 0 {
		t.Errorf("expected no kerning for ab, got: %g", k)
	}
	if fc.KerningIsContextual() {
		t.Error("expected a kerning that is not a floor")
	}
}

func TestLigature(t *testing.T) {
	t.Parallel()
	fc := lm(t, "lmroman10-regular.otf").NewFaceAt(10)
	for _, s := range []string{"fi", "fl", "ff", "ffi", "ffl"} {
		if gids := fc.Shape(s, "liga"); len(gids) != 1 || gids[0] == 0 {
			t.Errorf("%s: expected one glyph of a ligature, got: %v", s, gids)
		}
	}
	if gids := fc.Shape("ab", "liga"); len(gids) != 2 {
		t.Errorf("expected two glyphs for ab, got: %v", gids)
	}
	// the Go font has no ligature, and Shape returns the components
	g, err := Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	if gids := g.NewFaceAt(10).Shape("fi", "liga"); len(gids) != 2 {
		t.Errorf("expected two glyphs for fi in the Go font, got: %v", gids)
	}
}

func TestParseError(t *testing.T) {
	t.Parallel()
	for _, data := range [][]byte{nil, []byte("not a font"), []byte("OTTO\x00\x01\x00\x00\x00\x00\x00\x00")} {
		if _, err := Parse(data); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%q: expected ErrFormat, got: %v", data, err)
		}
	}
}
