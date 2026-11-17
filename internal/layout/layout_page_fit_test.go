// This file is not a port. It tests -ivy-margin-box-fit.

package layout

import "testing"

func TestMarginBoxFit(t *testing.T) {
	for _, tt := range []struct {
		fit       string
		top       float64
		bottom    float64
		paragraph float64
	}{
		// The header of 50px, with the margin of its paragraph, under a
		// padding of 5px pushes the body down to 55px, and the footer of 30px over a padding of 10px pushes
		// the bottom of the body up to 160px.
		{"grow", 55, 40, 55},
		{"none", 20, 20, 20},
		// A margin grows to at most 40% of the height of the page.
		{"grow; size: 200px 100px", 40, 40, 40},
	} {
		pages := renderPages(t, `
      <style>
        @page {
          size: 200px 200px; margin: 20px; -ivy-margin-box-fit: `+tt.fit+`;
          @top-center { content: element(header); vertical-align: top; padding-top: 5px }
          @bottom-center { content: element(footer); vertical-align: bottom; padding-bottom: 10px }
        }
        body { margin: 0 }
        div { height: 30px }
      </style>
      <header style="display: block; position: running(header)"><p style="display: block; margin: 0 0 10px; height: 40px"></p></header>
      <footer style="display: block; position: running(footer); height: 30px"></footer>
      <div></div><div></div><div></div><div></div>`)
		page := pages[0]
		assertApprox(t, tt.fit+": margin_top", page.MarginTop, tt.top)
		assertApprox(t, tt.fit+": margin_bottom", page.MarginBottom, tt.bottom)
		body := only(t, page.Children[0])
		assertApprox(t, tt.fit+": position_y", body.Children[0].PositionY, tt.paragraph)
		// The body ends above the bottom margin, when more than one block
		// fits.
		last := body.Children[len(body.Children)-1]
		if end := last.PositionY + last.MarginHeight(); len(body.Children) > 1 && end > page.MarginTop+page.Height+1e-6 {
			t.Errorf("%s: the body ends at %v, past %v", tt.fit, end, page.MarginTop+page.Height)
		}
	}
}

func TestMarginBoxFitPages(t *testing.T) {
	// The pages after a grown page go on where it stops, and each page
	// takes the height of its own header.
	pages := renderPages(t, `
      <style>
        @page {
          size: 200px 200px; margin: 20px; -ivy-margin-box-fit: grow;
          @top-center { content: element(header); vertical-align: top }
        }
        body { margin: 0 }
        div { height: 40px }
      </style>
      <header style="display: block; position: running(header); height: 60px"></header>
      <div>1</div><div>2</div><div>3</div><div>4</div><div>5</div>
      <div style="break-before: page">6<header style="display: block; position: running(header); height: 10px"></header></div>`)
	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3", len(pages))
	}
	for i, want := range []float64{60, 60, 20} {
		assertApprox(t, "margin_top", pages[i].MarginTop, want)
	}
}

func TestStaticTopBlock(t *testing.T) {
	// The paragraph goes on from the first page. The box that its second
	// page anchors is 10px below the top of the paragraph on that page,
	// and not below its line.
	pages := renderPages(t, `
      <style>
        @page { size: 200px 100px; margin: 0 }
        body { margin: 0; font: 20px/20px weasyprint }
        @font-face { src: url(weasyprint.otf); font-family: weasyprint }
        span { display: block; position: absolute; margin-top: 10px; width: 5px; height: 5px }
      </style>
      <p style="margin: 0">a<br>b<br>c<br>d<br>e<br>f<br>g<br><span style="-ivy-static-top: block"></span>h<span></span></p>`)
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
	var boxes []*Box
	pages[1].Descendants(func(b *Box) bool {
		if b.ElementTag == "span" {
			boxes = append(boxes, b)
		}
		return true
	})
	if len(boxes) != 2 {
		t.Fatalf("got %d boxes, want 2", len(boxes))
	}
	assertApprox(t, "block: position_y", boxes[0].PositionY, 0)
	assertApprox(t, "block: content top", boxes[0].ContentBoxY(), 10)
	// Without it, the static position is below the line of the box.
	assertApprox(t, "auto: content top", boxes[1].ContentBoxY(), 70)
}
