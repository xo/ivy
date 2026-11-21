package pdf

import (
	"image/color"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

// linkFile returns a file of three pages with a link to a URI, a link to an
// explicit destination on the turned page 2, a link to a named destination,
// a name tree, the dictionary Dests of the catalog and an outline of two
// levels.
func linkFile() []byte {
	return build(
		"<< /Type /Catalog /Pages 2 0 R /Names << /Dests 9 0 R >> /Outlines 10 0 R /Dests << /old [5 0 R /Fit] >> >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R 5 0 R] /Count 3 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 13 0 R /Annots [6 0 R 7 0 R 8 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 13 0 R /Rotate 90 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 13 0 R >>",
		"<< /Type /Annot /Subtype /Link /Rect [10 10 50 30] /A << /S /URI /URI (https://example.com/) >> >>",
		"<< /Type /Annot /Subtype /Link /Rect [60 10 100 30] /Dest [4 0 R /XYZ 20 80 0] >>",
		"<< /Type /Annot /Subtype /Link /Rect [110 10 150 30] /A << /S /GoTo /D (chap) >> >>",
		"<< /Kids [11 0 R] >>",
		"<< /First 12 0 R /Last 12 0 R /Count 2 >>",
		"<< /Names [(chap) [5 0 R /XYZ 0 50 null]] /Limits [(chap) (chap)] >>",
		"<< /Title (One) /Parent 10 0 R /Dest [3 0 R /Fit] /First 14 0 R /Last 14 0 R /Count 1 >>",
		stream("", "0 0 1 rg 0 0 10 10 re f"),
		"<< /Title <FEFF00540077006F> /Parent 12 0 R /Dest (chap) >>",
		"<< >>",
	)
}

// linkLines draws page n of d onto a list, and returns its commands of the
// links, the anchors and the bookmarks.
func linkLines(t *testing.T, d *Document, n int) []string {
	t.Helper()
	p, ok := page(t, d, n).(*Page)
	if !ok {
		t.Fatal("expected a *Page")
	}
	l := &display.List{}
	if err := p.Draw(t.Context(), l, 1); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for line := range strings.SplitSeq(l.String(), "\n") {
		for _, prefix := range []string{"link ", "anchor ", "bookmark "} {
			if strings.HasPrefix(line, prefix) {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

func TestLinks(t *testing.T) {
	t.Parallel()
	d := open(t, linkFile())
	tests := []struct {
		n    int
		want []string
	}{
		{0, []string{
			`bookmark 1 "One" 0 0 true`,
			`link [10 70 50 90] "https://example.com/"`,
			`link [60 70 100 90] #"ivy-page-2-80-20"`,
			`link [110 70 150 90] #"chap"`,
		}},
		{1, []string{
			`anchor "ivy-page-2-80-20" 80 20`,
		}},
		{2, []string{
			`bookmark 2 "Two" 0 50 false`,
			`anchor "chap" 0 50`,
			`anchor "old" 0 0`,
		}},
	}
	for _, tt := range tests {
		if got := linkLines(t, d, tt.n); !slices.Equal(got, tt.want) {
			t.Errorf("page %d: expected:\n%s\ngot:\n%s", tt.n+1, strings.Join(tt.want, "\n"), strings.Join(got, "\n"))
		}
	}
}

// TestLinksNotRead checks that a render, whose device takes no links,
// reads no link of the file.
func TestLinksNotRead(t *testing.T) {
	t.Parallel()
	d := open(t, linkFile())
	if _, err := ivy.RenderImage(t.Context(), page(t, d, 0), 72, color.White); err != nil {
		t.Fatal(err)
	}
	if d.linkIdx != nil {
		t.Error("expected no index of the links after a render")
	}
}

// TestLinksCycles checks that an outline and a name tree that loop end.
func TestLinksCycles(t *testing.T) {
	t.Parallel()
	d := open(t, build(
		"<< /Type /Catalog /Pages 2 0 R /Names << /Dests 5 0 R >> /Outlines 4 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] >>",
		"<< /First 6 0 R >>",
		"<< /Kids [5 0 R 7 0 R] >>",
		"<< /Title (Loop) /Dest (a) /Next 6 0 R /First 6 0 R >>",
		"<< /Names [(a) [3 0 R /FitH 40]] /Kids [5 0 R] >>",
		"<< >>",
	))
	want := []string{`bookmark 1 "Loop" 0 60 false`, `anchor "a" 0 60`}
	if got := linkLines(t, d, 0); !slices.Equal(got, want) {
		t.Errorf("expected:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

// TestLinksLimit checks that MaxObjects bounds the links that the index
// reads.
func TestLinksLimit(t *testing.T) {
	t.Parallel()
	d := open(t, linkFile(), WithConfig(ivy.Config{Limits: ivy.Limits{MaxObjects: 4}}))
	// the four objects are the two names of the file and two of the three
	// links of page 1, so the outline is not read
	if got := linkLines(t, d, 2); slices.ContainsFunc(got, func(s string) bool { return strings.HasPrefix(s, "bookmark") }) {
		t.Errorf("expected no bookmark past the limit, got:\n%s", strings.Join(got, "\n"))
	}
}
