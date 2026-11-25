package test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

// The text tests of D58 read the text of each written page with pdftotext
// of Poppler and with mutool draw -F txt of MuPDF. The words of the page
// must be in each text, in order. The words come from the runs of glyphs
// that the source page draws with a text and with a font that the file can
// embed. pdffonts of Poppler must show each font of the file as embedded
// and as a subset.

// textReaders are the programs that read the text of page n, from 1, of
// the PDF file at path. pdftotext -raw keeps the order of the content
// stream, and breaks a word from right to left at a mark of its letters,
// even in the PDF file of Word. Its default mode applies the bidirectional
// algorithm and reads such a word whole, so a word that the first text
// does not hold is looked for in the second, alt.
var textReaders = []struct {
	name string
	read func(ctx context.Context, path string, n int) (string, error)
	alt  func(ctx context.Context, path string, n int) (string, error)
}{
	{name: "pdftotext", read: func(ctx context.Context, path string, n int) (string, error) {
		page := strconv.Itoa(n)
		return output(ctx, "pdftotext", "-raw", "-enc", "UTF-8", "-f", page, "-l", page, path, "-")
	}, alt: func(ctx context.Context, path string, n int) (string, error) {
		page := strconv.Itoa(n)
		return output(ctx, "pdftotext", "-enc", "UTF-8", "-f", page, "-l", page, path, "-")
	}},
	{name: "mutool", read: func(ctx context.Context, path string, n int) (string, error) {
		return output(ctx, "mutool", "draw", "-q", "-F", "txt", "-o", "-", path, strconv.Itoa(n))
	}},
}

// output runs a program and returns what it writes.
func output(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", fmt.Errorf("running %s: %w", name, err)
	}
	return string(out), nil
}

// pageWords returns the words of the runs of glyphs that page draws with a
// text and with a font that a PDF file can embed, in the order of the
// runs. A word holds the characters of the glyphs whose middle is inside
// the clip and the page. A glyph .notdef has no word, because a reader of
// the text of a PDF file cannot tell one .notdef from another. A run inside
// a group that the PDF device draws as pixels, such as a group of a blend
// mode that PDF does not have, has no words.
func pageWords(ctx context.Context, page ivy.Page) ([]string, error) {
	d, ok := page.(display.Drawer)
	if !ok {
		return nil, nil
	}
	w, h := page.Size()
	clip, ok := okage.RectFromXYWH(0, 0, w, h)
	if !ok {
		return nil, nil
	}
	c := &wordCollector{state: collectorState{t: okage.IdentityTransform(), clip: clip}}
	if err := d.Draw(ctx, c, 300.0/72); err != nil {
		if _, partial := errors.AsType[*ivy.PartialError](err); !partial {
			return nil, fmt.Errorf("drawing the page: %w", err)
		}
	}
	return c.words, nil
}

// wordCollector is a device that keeps the words of the runs of glyphs.
type wordCollector struct {
	words []string
	state collectorState
	stack []collectorState
	// pixels counts the groups that the PDF device draws as pixels, from
	// the outer one.
	pixels int
}

// collectorState is the transform and the bounds of the clip on the page.
// hidden is true when the clip is empty.
type collectorState struct {
	t      okage.Transform
	clip   okage.Rect
	hidden bool
}

// Save satisfies the display.Device interface.
func (c *wordCollector) Save() {
	c.stack = append(c.stack, c.state)
}

// Restore satisfies the display.Device interface.
func (c *wordCollector) Restore() {
	if n := len(c.stack); n > 0 {
		c.state = c.stack[n-1]
		c.stack = c.stack[:n-1]
	}
}

// Concat satisfies the display.Device interface.
func (c *wordCollector) Concat(t okage.Transform) {
	c.state.t = c.state.t.PreConcat(t)
}

// ClipPath satisfies the display.Device interface.
func (c *wordCollector) ClipPath(p *okage.Path, _ okage.FillRule) {
	if p == nil {
		c.state.hidden = true
		return
	}
	b, ok := p.Bounds().Transform(c.state.t)
	if !ok {
		return
	}
	clip, ok := c.state.clip.Intersect(b)
	if !ok {
		c.state.hidden = true
		return
	}
	c.state.clip = clip
}

func (*wordCollector) FillPath(*okage.Path, okage.FillRule, okage.Shader)           {}
func (*wordCollector) StrokePath(*okage.Path, okage.Stroke, okage.Shader)           {}
func (*wordCollector) DrawImage(image.Image, okage.Transform, display.ImageOptions) {}
func (*wordCollector) FillMask(*image.Alpha, okage.Transform, display.ImageOptions, okage.Shader) {
}

// BeginGroup satisfies the display.Device interface. The PDF device draws
// a group of a blend mode that PDF does not have as pixels, and it draws
// nothing of a group of opacity 0.
func (c *wordCollector) BeginGroup(g display.Group) {
	c.Save()
	if c.pixels > 0 || !pdfBlend(g.Blend) || !(g.Opacity > 0) {
		c.pixels++
	}
}

// EndGroup satisfies the display.Device interface.
func (c *wordCollector) EndGroup() {
	if c.pixels > 0 {
		c.pixels--
	}
	c.Restore()
}

// FillGlyphs satisfies the display.Device interface.
func (c *wordCollector) FillGlyphs(run display.GlyphRun, _ okage.Shader) {
	if c.pixels > 0 || c.state.hidden || run.Face == nil || !run.TextRanges() || len(run.Glyphs) == 0 || !(run.Size > 0) {
		return
	}
	if src, ok := run.Face.Source(); !ok || !src.Embeddable() {
		return
	}
	// only the text of the clusters of the glyphs that the page shows
	covered := make([]bool, len(run.Text))
	for _, g := range run.Glyphs {
		// the middle of the glyph: half its advance along the baseline,
		// and a third of an em above it
		mid := okage.Point{X: run.Face.Advance(g.ID) * run.Size / 2, Y: -run.Size / 3}
		run.Transform.MapPoint(&mid)
		mid.X -= run.Transform.TX
		mid.Y -= run.Transform.TY
		p := okage.Point{X: g.X + mid.X, Y: g.Y + mid.Y}
		c.state.t.MapPoint(&p)
		clip := c.state.clip
		if g.ID == 0 || p.X < clip.Left() || p.X > clip.Right() || p.Y < clip.Top() || p.Y > clip.Bottom() {
			continue
		}
		for i := g.Start; i < g.End; i++ {
			covered[i] = true
		}
	}
	var b strings.Builder
	for i, r := range run.Text {
		if covered[i] {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	c.words = append(c.words, strings.Fields(b.String())...)
}

// pdfBlend reports whether PDF has the blend mode b, as the PDF device
// maps it. The zero mode is the normal one.
func pdfBlend(b okage.BlendMode) bool {
	return b == okage.BlendModeClear || b == okage.BlendModeSourceOver || b >= okage.BlendModeScreen
}

// normalText returns s without the characters that a reader of the text
// of a PDF file can add, drop or split: the spaces, the controls, the
// characters that draw nothing, such as the soft hyphen, and the harakat
// of Arabic, such as the tanween and the shadda. pdftotext and MuPDF move
// a haraka before its letter, or out of its word, in the PDF file of Word
// too.
func normalText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == unicode.ReplacementChar ||
			r >= 0x064B && r <= 0x065F || r == 0x0670 {
			return -1
		}
		return r
	}, s)
}

// rightToLeftWord reports whether a word holds a letter of Arabic or of
// Hebrew.
func rightToLeftWord(w string) bool {
	return strings.ContainsFunc(w, func(r rune) bool {
		return unicode.In(r, unicode.Arabic, unicode.Hebrew) && unicode.IsLetter(r)
	})
}

// maxOccurrences bounds the places of one word in a text that
// matchWords looks at.
const maxOccurrences = 4096

// matchWords returns the words that text does not hold, and the number of
// the words that text holds out of order: the words that are not in the
// longest sequence of words that text holds in their order.
func matchWords(words []string, text string) (absent []string, unordered int) {
	text = normalText(text)
	// tails[k] is the smallest place in text where a sequence of k+1 words
	// in order can end, as the patience sort of the longest increasing
	// sequence keeps it
	var tails []int
	present := 0
	// A page with words from right to left can give a word of punctuation
	// only, such as the closing guillemet and the period of a quote, in
	// any place of the line.
	rtl := slices.ContainsFunc(words, rightToLeftWord)
	for _, w := range words {
		w = normalText(w)
		if w == "" || rtl && strings.TrimFunc(w, unicode.IsPunct) == "" {
			continue
		}
		var places []int
		for i := 0; i < len(text) && len(places) < maxOccurrences; {
			j := strings.Index(text[i:], w)
			if j < 0 {
				break
			}
			places = append(places, i+j)
			i += j + 1
		}
		if len(places) == 0 && rightToLeftWord(w) {
			// A reader moves the punctuation at the edge of a word from
			// right to left by its own rules for the neutral characters,
			// such as the Arabic comma after a year.
			if v := strings.TrimFunc(w, unicode.IsPunct); v != "" && v != w {
				if j := strings.Index(text, v); j >= 0 {
					places = append(places, j)
				}
			}
		}
		if len(places) == 0 {
			absent = append(absent, w)
			continue
		}
		present++
		// the places from the last, so that one word extends a sequence
		// once
		for _, p := range slices.Backward(places) {
			k, _ := slices.BinarySearch(tails, p)
			if k == len(tails) {
				tails = append(tails, p)
			} else {
				tails[k] = p
			}
		}
	}
	return absent, present - len(tails)
}

// pdfFonts returns the fonts of the PDF file at path that pdffonts shows
// as not embedded or not as a subset. A Type 3 font has no font program,
// so it is left out.
func pdfFonts(ctx context.Context, path string) (all int, bad []string, err error) {
	out, err := output(ctx, "pdffonts", path)
	if err != nil {
		return 0, nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines[min(2, len(lines)):] {
		// the columns are the name, the type, the encoding, emb, sub, uni
		// and the object, and the type and the encoding can hold spaces
		f := strings.Fields(line)
		if len(f) < 7 {
			continue
		}
		all++
		emb, sub := f[len(f)-5], f[len(f)-4]
		if strings.Contains(line, "Type 3") {
			continue
		}
		if emb != "yes" || sub != "yes" {
			bad = append(bad, fmt.Sprintf("%s emb=%s sub=%s", f[0], emb, sub))
		}
	}
	return all, bad, nil
}

// textOrderDiffers lists the documents of the corpus whose words pdftotext
// gives in another order, and why.
var textOrderDiffers = map[string]string{
	"idpf/epub/regime-anticancer-arabic.epub": "pdftotext puts a bracket between a text from right to left " +
		"and a text from left to right in the order of the page, not in the order of the text",
	"poi/docx/rtl.docx": "pdftotext puts a bracket between a text from right to left " +
		"and a text from left to right in the order of the page, not in the order of the text",
}

// textPage reads the text of page n, from 1, of the written file at path
// with each reader of textReaders, and looks for the words of src, page
// num of its document from 1. Each word must be in the text of pdftotext,
// in order. mutool sorts the blocks of text by their places on the page,
// so each word must be in its text, and the words out of order are only
// logged.
func textPage(t *testing.T, path string, src ivy.Page, n, num int) {
	t.Helper()
	ctx := t.Context()
	words, err := pageWords(ctx, src)
	if err != nil {
		t.Errorf("page %d: %v", num, err)
		return
	}
	line := fmt.Sprintf("pdfenc text: page %d words=%d", num, len(words))
	for _, r := range textReaders {
		text, err := r.read(ctx, path, n)
		if err != nil {
			t.Errorf("page %d: %v", num, err)
			continue
		}
		absent, unordered := matchWords(words, text)
		if len(absent) > 0 && r.alt != nil {
			alt, err := r.alt(ctx, path, n)
			if err != nil {
				t.Errorf("page %d: %v", num, err)
				continue
			}
			absent, _ = matchWords(absent, alt)
		}
		line += fmt.Sprintf(" %s=%d/%d", r.name, len(absent), unordered)
		switch {
		case len(absent) > 0:
			t.Errorf("page %d: expected each word in the text of %s, got %d of %d words absent, such as %q",
				num, r.name, len(absent), len(words), absent[:min(5, len(absent))])
		case unordered > 0 && r.name == "pdftotext" && textOrderDiffers[strings.TrimPrefix(t.Name(), "TestPDFEnc/")] != "":
			t.Logf("page %d: %s gives %d of %d words in another order: %s", num, r.name, unordered, len(words),
				textOrderDiffers[strings.TrimPrefix(t.Name(), "TestPDFEnc/")])
		case unordered > 0 && r.name == "pdftotext":
			t.Errorf("page %d: expected the words in order in the text of %s, got %d of %d out of order",
				num, r.name, unordered, len(words))
		case unordered > 0:
			t.Logf("page %d: %s gives %d of %d words in another order", num, r.name, unordered, len(words))
		}
	}
	t.Log(line)
}
