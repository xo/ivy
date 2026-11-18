package ivy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"sync/atomic"
	"testing"

	"github.com/xo/okage"
)

// bandPage is a page that draws a gradient, a circle and a thin diagonal
// line, and counts its calls of Render. It returns err from the call that
// draws the pixel row errRow, when err is set.
type bandPage struct {
	w, h   float32
	calls  atomic.Int64
	err    error
	errRow float32
	// active counts the calls of Render that run, when it is set.
	active *atomic.Int64
}

func (p *bandPage) Size() (float32, float32) { return p.w, p.h }

func (p *bandPage) Render(_ context.Context, dst *okage.Pixmap, t okage.Transform) error {
	p.calls.Add(1)
	if p.active != nil {
		p.active.Add(1)
		defer p.active.Add(-1)
	}
	grad, ok := okage.NewLinearGradient(okage.PointFromXY(0, 0), okage.PointFromXY(p.w, p.h), []okage.GradientStop{
		okage.NewGradientStop(0, okage.ColorFromRGBA8(200, 30, 30, 255)),
		okage.NewGradientStop(1, okage.ColorFromRGBA8(30, 30, 200, 255)),
	}, okage.SpreadModePad, okage.IdentityTransform())
	if !ok {
		return errors.New("making the gradient")
	}
	paint := okage.DefaultPaint()
	paint.Shader = grad
	rect, _ := okage.RectFromXYWH(0, 0, p.w, p.h)
	dst.FillRect(rect, &paint, t, nil)
	circle, ok := okage.PathFromCircle(p.w/2, p.h/2, p.w/3)
	if !ok {
		return errors.New("making the circle")
	}
	paint = okage.DefaultPaint()
	paint.SetColorRGBA8(20, 160, 60, 200)
	dst.FillPath(circle, &paint, okage.FillRuleWinding, t, nil)
	pb := okage.NewPathBuilder()
	pb.MoveTo(1, 1)
	pb.LineTo(p.w-1, p.h-1)
	line, _ := pb.Finish()
	stroke := okage.DefaultStroke()
	stroke.Width = 0.7
	paint.SetColorRGBA8(0, 0, 0, 255)
	dst.StrokePath(line, &paint, &stroke, t, nil)
	if p.err != nil {
		// the band holds the row when its transform maps the row into it
		y := p.errRow*t.SY + t.TY
		if y >= 0 && y < float32(dst.Height()) {
			return p.err
		}
	}
	return nil
}

func TestRenderBandsWorkers(t *testing.T) {
	// a page of 1000 by 1300 pixels at the scale 4/3: 6 bands
	page := &bandPage{w: 750, h: 975}
	render := func(workers int) []byte {
		dst, ok := okage.NewPixmap(1000, 1300)
		if !ok {
			t.Fatal("making the pixmap")
		}
		dst.Fill(okage.ColorWhite)
		if err := renderBands(t.Context(), page, dst, 4.0/3, workers); err != nil {
			t.Fatalf("workers %d: %v", workers, err)
		}
		return dst.Data()
	}
	want := render(1)
	if n := page.calls.Load(); n != 6 {
		t.Errorf("expected 6 calls of Render, one for each band, got %d", n)
	}
	for _, workers := range []int{2, 8, 64} {
		if got := render(workers); !bytes.Equal(got, want) {
			t.Errorf("expected the same pixels with %d workers as with one", workers)
		}
	}
}

func TestRenderBandsSmall(t *testing.T) {
	// a page of fewer than minBandPixels pixels renders in one call
	page := &bandPage{w: 300, h: 300}
	dst, _ := okage.NewPixmap(400, 400)
	if err := renderBands(t.Context(), page, dst, 4.0/3, 8); err != nil {
		t.Fatal(err)
	}
	if n := page.calls.Load(); n != 1 {
		t.Errorf("expected one call of Render, got %d", n)
	}
}

func TestRenderBandsErrors(t *testing.T) {
	partial := &PartialError{Err: errors.New("left out a shape")}
	page := &bandPage{w: 750, h: 975, err: partial, errRow: 700}
	dst, _ := okage.NewPixmap(1000, 1300)
	err := renderBands(t.Context(), page, dst, 4.0/3, 4)
	if !errors.Is(err, partial) {
		t.Errorf("expected the *PartialError of the band, got: %v", err)
	}
	failed := errors.New("failed")
	page = &bandPage{w: 750, h: 975, err: failed, errRow: 100}
	if err := renderBands(t.Context(), page, dst, 4.0/3, 4); !errors.Is(err, failed) {
		t.Errorf("expected the error of the band, got: %v", err)
	}
}

// bandPager is a pager of pages of bandPage, each one taller than the last.
// active counts the calls of Render of its pages that run.
type bandPager struct {
	n      int
	active atomic.Int64
}

func (p *bandPager) PageCount(context.Context) (int, error) { return p.n, nil }

func (p *bandPager) Page(_ context.Context, n int) (Page, error) {
	if n < 0 || n >= p.n {
		return nil, fmt.Errorf("page %d: %w", n, ErrPageRange)
	}
	return &bandPage{w: 400, h: float32(300 + 50*n), active: &p.active}, nil
}

func TestRenderPages(t *testing.T) {
	pager := &bandPager{n: 6}
	pages := []int{5, 0, 3, 1, 4, 2, 9}
	i := 0
	for img, err := range RenderPages(t.Context(), pager, pages, 144, color.White) {
		n := pages[i]
		i++
		if n >= pager.n {
			if !errors.Is(err, ErrPageRange) {
				t.Errorf("page %d: expected ErrPageRange, got: %v", n, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("page %d: %v", n, err)
		}
		page, _ := pager.Page(t.Context(), n)
		want, err := RenderImage(t.Context(), page, 144, color.White)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(img.(*okage.Pixmap).Data(), want.(*okage.Pixmap).Data()) {
			t.Errorf("page %d: expected the pixels of RenderImage", n)
		}
	}
	if i != len(pages) {
		t.Errorf("expected %d images, got %d", len(pages), i)
	}
}

func TestRenderPagesStop(t *testing.T) {
	pager := &bandPager{n: 40}
	pages := make([]int, 40)
	for i := range pages {
		pages[i] = i
	}
	n := 0
	for _, err := range RenderPages(t.Context(), pager, pages, 72, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if n++; n == 3 {
			break
		}
	}
	// RenderPages returns after its goroutines stop, so no page renders
	if n := pager.active.Load(); n != 0 {
		t.Errorf("expected no page to render after the loop stopped, got %d", n)
	}
}
