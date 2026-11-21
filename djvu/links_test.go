package djvu

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

// linkLines draws page n of d onto a list, and returns the lines of the
// list that are links, anchors and bookmarks.
func linkLines(t *testing.T, d *Document, n int) []string {
	t.Helper()
	var out []string
	for line := range strings.Lines(drawList(t, d, n)) {
		line = strings.TrimSuffix(line, "\n")
		for _, prefix := range []string{"link ", "anchor ", "bookmark "} {
			if strings.HasPrefix(line, prefix) {
				out = append(out, line)
			}
		}
	}
	return out
}

// drawList draws page n of d onto a list, and returns its commands.
func drawList(t *testing.T, d *Document, n int) string {
	t.Helper()
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	l := &display.List{}
	if err := p.(display.Drawer).Draw(t.Context(), l, 1); err != nil {
		if _, ok := errors.AsType[*ivy.PartialError](err); !ok {
			t.Fatal(err)
		}
	}
	return l.String()
}

// TestLinksCorpus reads the outline and the links of the bundled files of
// the corpus, which djvused made.
func TestLinksCorpus(t *testing.T) {
	t.Parallel()
	want := map[string][][]string{
		"bundled.djvu": {
			{`bookmark 1 "Text" 0 0 true`, `anchor "ivy-page-1-0-0" 0 0`, `link [72 720 240 737] "https://example.com/"`, `link [300 720 401 737] #"ivy-page-3-0-0"`},
			{`bookmark 1 "Pictures" 0 0 true`, `bookmark 2 "Background" 0 0 true`, `anchor "ivy-page-2-0-0" 0 0`},
			{`bookmark 2 "Colours" 0 0 true`, `anchor "ivy-page-3-0-0" 0 0`},
			{`bookmark 1 "Compound" 0 0 true`, `anchor "ivy-page-4-0-0" 0 0`},
		},
		"pdf2djvu.djvu": {
			{`bookmark 1 "Text" 0 0 true`, `anchor "ivy-page-1-0-0" 0 0`, `link [72 720 240 737] "https://example.com/"`, `link [300 720 400 737] #"ivy-page-3-0-0"`},
			{`bookmark 1 "Pictures" 0 0 true`, `bookmark 2 "Background" 0 0 true`, `anchor "ivy-page-2-0-0" 0 0`},
			{`bookmark 2 "Coloured text" 0 0 true`, `anchor "ivy-page-3-0-0" 0 0`},
			{`bookmark 1 "More text" 0 0 true`, `anchor "ivy-page-4-0-0" 0 0`},
		},
	}
	for name, pages := range want {
		d := open(t, name, ivy.Config{})
		for i, w := range pages {
			if got := linkLines(t, d, i); !slices.Equal(got, w) {
				t.Errorf("%s page %d: expected %q, got: %q", name, i+1, w, got)
			}
		}
	}
}

// TestLinks reads the maparea annotations of a single page: a link to a
// URI, a link to the page, and the areas that give no link.
func TestLinks(t *testing.T) {
	t.Parallel()
	anno := `(maparea "http://example.com/a" "" (rect 300 3000 600 150))
(maparea (url "#1" "_self") "to the page" (oval 0 0 300 300) (xor))
(maparea "#+1" "" (rect 0 0 10 10))
(maparea "" "no url" (rect 0 0 10 10))
(maparea "mailto:a@example.com" "" (poly 0 3300 2550 3300 1275 3000))
(maparea "http://example.com/line" "" (line 0 0 300 300))
(maparea "http://example.com/empty" "" (rect 0 0 0 10))
(maparea "http://example.com/bad" "" (rect 0 0 x 10))
(zoom page)`
	data := form("DJVU", chunk("INFO", info2550()), chunk("ANTa", []byte(anno)))
	d, err := Open(t.Context(), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`anchor "ivy-page-1-0-0" 0 0`,
		`link [72 36 216 72] "http://example.com/a"`,
		`link [0 720 72 792] #"ivy-page-1-0-0"`,
		`link [0 0 612 72] "mailto:a@example.com"`,
		`link [0 720 72 792] "http://example.com/line"`,
	}
	if got := linkLines(t, d, 0); !slices.Equal(got, want) {
		t.Errorf("expected %q, got: %q", want, got)
	}

	// a turned page draws its links through the rotation of the page
	d, err = Open(t.Context(), bytes.NewReader(patchInfo(t, data, 9, 5)))
	if err != nil {
		t.Fatal(err)
	}
	if got := linkLines(t, d, 0); !slices.Equal(got, want) {
		t.Errorf("expected %q, got: %q", want, got)
	}
	if got := drawList(t, d, 0); !strings.Contains(got, "concat {0 -1 1 0 792 0}\n"+want[1]) {
		t.Errorf("expected the links after the rotation, got:\n%s", got)
	}

	// the links of a page stop at MaxObjects
	d, err = Open(t.Context(), bytes.NewReader(data), WithConfig(ivy.Config{Limits: ivy.Limits{MaxObjects: 2}}))
	if err != nil {
		t.Fatal(err)
	}
	if got := linkLines(t, d, 0); !slices.Equal(got, want[:3]) {
		t.Errorf("expected %q, got: %q", want[:3], got)
	}
}

// TestLinksOnlyForWriters draws nothing more onto a device that is not a
// Linker or a Bookmarker.
func TestLinksOnlyForWriters(t *testing.T) {
	t.Parallel()
	d := open(t, "bundled.djvu", ivy.Config{})
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	var dev plainDevice
	if err := p.(display.Drawer).Draw(t.Context(), &dev, 1); err != nil {
		t.Fatal(err)
	}
	if d.nav != nil {
		t.Error("expected no outline for a device that takes no links")
	}
}

// plainDevice is a device that is not a Linker or a Bookmarker.
type plainDevice struct {
	display.List
}

// LinkURI hides LinkURI of the list, so that the device is not a Linker.
func (*plainDevice) LinkURI() {}

// Bookmark hides Bookmark of the list, so that the device is not a
// Bookmarker.
func (*plainDevice) Bookmark() {}
