package djvu

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/iff"
)

// standalone returns a standalone IW44 file of the type typ, FORM:BM44 or
// FORM:PM44, with the chunks of the background of the first page of the
// test file name.
func standalone(t *testing.T, name, typ string) []byte {
	t.Helper()
	data := testFile(t, name)
	root, err := iff.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	chunks := root.All("BG44")
	kids := make([][]byte, 0, len(chunks))
	for _, c := range chunks {
		kids = append(kids, chunk(typ, c.Data))
	}
	if len(kids) == 0 {
		t.Fatalf("%s has no BG44 chunk", name)
	}
	return form(typ, kids...)
}

func TestStandaloneIW44(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, typ string }{{"photo.djvu", "PM44"}, {"gray.djvu", "BM44"}} {
		t.Run(c.typ, func(t *testing.T) {
			// the page of the test file holds only its background, so the
			// standalone image looks as the page does
			want := render(t, open(t, c.name, ivy.Config{}), 0, 30)
			d, err := Open(t.Context(), bytes.NewReader(standalone(t, c.name, c.typ)))
			if err != nil {
				t.Fatal(err)
			}
			if n, _ := d.PageCount(t.Context()); n != 1 {
				t.Fatalf("expected one page, got: %d", n)
			}
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			w, h := page.Size()
			if w < 100 || h < 100 {
				t.Fatalf("expected a page of one point for each pixel, got: %vx%v", w, h)
			}
			// the page renders at the size of the page of the test file
			got, err := ivy.RenderImage(t.Context(), page, 72*float32(want.Bounds().Dx())/w, color.White)
			if err != nil {
				t.Fatal(err)
			}
			// the sizes can differ by a pixel, from the rounding of the
			// resolution
			if d := got.Bounds().Size().Sub(want.Bounds().Size()); d.X < -1 || d.X > 1 || d.Y < -1 || d.Y > 1 {
				t.Fatalf("expected %v, got: %v", want.Bounds().Size(), got.Bounds().Size())
			}
			b := want.Bounds().Intersect(got.Bounds())
			for _, r := range []image.Rectangle{
				image.Rect(0, 0, b.Dx()/2, b.Dy()/2),
				image.Rect(b.Dx()/2, b.Dy()/2, b.Dx(), b.Dy()),
			} {
				if g, w := gray(got, r), gray(want, r); g < w-12 || g > w+12 {
					t.Errorf("in %v: expected a mean of %.0f, got: %.0f", r, w, g)
				}
			}
			if _, err := d.Page(t.Context(), 1); err == nil {
				t.Error("expected an error for page 2")
			}
		})
	}
}
