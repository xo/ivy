package odf

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// drawRender opens the file and renders its page n at 72 dots per inch,
// over white, so that a pixel is a point.
func drawRender(t *testing.T, buf []byte, n int, opts ...Option) image.Image {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf), opts...)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), n)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// drawNear reports whether the pixel of img at (x, y) is within tol of c in
// each channel.
func drawNear(img image.Image, x, y int, c color.NRGBA, tol int) bool {
	got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	d := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	return d(got.R, c.R) <= tol && d(got.G, c.G) <= tol && d(got.B, c.B) <= tol
}

// drawExpect fails the test if the pixel at (x, y) is not near c.
func drawExpect(t *testing.T, img image.Image, x, y int, c color.NRGBA) {
	t.Helper()
	if !drawNear(img, x, y, c, 12) {
		t.Errorf("pixel (%d, %d): expected %v, got %v", x, y, c, color.NRGBAModel.Convert(img.At(x, y)))
	}
}

// drawCount counts the pixels of the rectangle r of img that are near c.
func drawCount(img image.Image, r image.Rectangle, c color.NRGBA, tol int) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if drawNear(img, x, y, c, tol) {
				n++
			}
		}
	}
	return n
}

var (
	drawWhite = color.NRGBA{255, 255, 255, 255}
	drawRed   = color.NRGBA{255, 0, 0, 255}
	drawGreen = color.NRGBA{0, 255, 0, 255}
	drawBlue  = color.NRGBA{0, 0, 255, 255}
)

// drawStyles are automatic styles of the tests: a red fill with a blue
// line of 6 points, a green fill without a line, and text styles.
const drawStyles = `<style:style style:name="red" style:family="graphic" style:parent-style-name="standard">
<style:graphic-properties draw:fill="solid" draw:fill-color="#ff0000" draw:stroke="solid" svg:stroke-color="#0000ff" svg:stroke-width="6pt"/></style:style>
<style:style style:name="green" style:family="graphic" style:parent-style-name="standard">
<style:graphic-properties draw:fill="solid" draw:fill-color="#00ff00" draw:stroke="none"/></style:style>
<style:style style:name="frame" style:family="graphic" style:parent-style-name="standard">
<style:graphic-properties draw:fill="none" draw:stroke="none" fo:padding-left="0cm" fo:padding-top="0cm"/></style:style>
<style:style style:name="hidden" style:family="drawing-page"><style:drawing-page-properties presentation:visibility="hidden"/></style:style>
<style:style style:name="T1" style:family="text"><style:text-properties fo:color="#ff0000" fo:font-size="40pt"/></style:style>
<style:style style:name="T2" style:family="text"><style:text-properties fo:color="#0000ff" fo:font-size="40pt"/></style:style>
<text:list-style style:name="L1"><text:list-level-style-bullet text:level="1" text:bullet-char="●">
<style:list-level-properties text:min-label-width="1cm"/><style:text-properties fo:color="#ff0000" fo:font-size="100%"/></text:list-level-style-bullet></text:list-style>`

func TestDrawPages(t *testing.T) {
	buf := (&fixture.ODP{
		AutomaticStyles: drawStyles,
		Pages:           []string{">", `draw:style-name="hidden">`, ">"},
	}).Bytes()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages without the hidden one, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); int(w+0.5) != 794 || int(h+0.5) != 446 {
		t.Errorf("expected 794x446 points, got: %gx%g", w, h)
	}
	// a drawing keeps each page
	buf = (&fixture.ODP{
		Mime:            mimePrefix + "graphics",
		AutomaticStyles: drawStyles,
		Pages:           []string{">", `draw:style-name="hidden">`},
	}).Bytes()
	d, err = Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 2 {
		t.Errorf("expected 2 pages, got: %d, %v", n, err)
	}
}

func TestDrawShapes(t *testing.T) {
	page := `><draw:rect draw:style-name="red" svg:x="100pt" svg:y="100pt" svg:width="200pt" svg:height="100pt"/>
<draw:ellipse draw:style-name="green" svg:x="400pt" svg:y="100pt" svg:width="200pt" svg:height="100pt"/>
<draw:custom-shape draw:style-name="green" svg:x="100pt" svg:y="250pt" svg:width="100pt" svg:height="100pt">
<draw:enhanced-geometry svg:viewBox="0 0 21600 21600" draw:type="isosceles-triangle" draw:modifiers="10800"
 draw:enhanced-path="M ?f0 0 L 21600 21600 0 21600 Z N"><draw:equation draw:name="f0" draw:formula="$0 "/></draw:enhanced-geometry></draw:custom-shape>
<draw:custom-shape draw:style-name="green" svg:x="300pt" svg:y="250pt" svg:width="100pt" svg:height="100pt">
<draw:enhanced-geometry draw:type="ellipse"/></draw:custom-shape>`
	img := drawRender(t, (&fixture.ODP{AutomaticStyles: drawStyles, Pages: []string{page}}).Bytes(), 0)
	drawExpect(t, img, 200, 150, drawRed)
	drawExpect(t, img, 100, 150, drawBlue)
	drawExpect(t, img, 500, 150, drawGreen)
	drawExpect(t, img, 405, 105, drawWhite)
	// the triangle: green under its apex, white at the corners of its top
	drawExpect(t, img, 150, 330, drawGreen)
	drawExpect(t, img, 105, 260, drawWhite)
	drawExpect(t, img, 195, 260, drawWhite)
	// the ellipse of the preset of its type
	drawExpect(t, img, 350, 300, drawGreen)
	drawExpect(t, img, 303, 253, drawWhite)
}

func TestDrawPicture(t *testing.T) {
	page := `><draw:frame draw:style-name="frame" svg:x="100pt" svg:y="100pt" svg:width="100pt" svg:height="50pt">
<draw:image xlink:href="Pictures/a.png" xlink:type="simple"/></draw:frame>`
	o := &fixture.ODP{
		AutomaticStyles: drawStyles,
		Pages:           []string{page},
		Entries:         []fixture.Entry{{Name: "Pictures/a.png", Data: fixture.PNG(4, 4, drawGreen)}},
	}
	img := drawRender(t, o.Bytes(), 0)
	drawExpect(t, img, 150, 125, drawGreen)
	drawExpect(t, img, 95, 125, drawWhite)
	// the picture of an encrypted file is decrypted
	buf := fixture.EncryptODF(fixture.ODFEncryption{Password: "secret", Cipher: fixture.ODFAESCBC, Iterations: 100},
		mimePrefix+"presentation", o.Parts()...)
	img = drawRender(t, buf, 0, WithConfig(ivy.Config{Password: "secret"}))
	drawExpect(t, img, 150, 125, drawGreen)
}

func TestDrawChart(t *testing.T) {
	page := `><draw:frame draw:style-name="frame" svg:x="100pt" svg:y="100pt" svg:width="300pt" svg:height="200pt">
<draw:object xlink:href="./Object 1" xlink:type="simple"/><draw:image xlink:href="./ObjectReplacements/Object 1" xlink:type="simple"/></draw:frame>`
	o := &fixture.ODP{
		AutomaticStyles: drawStyles,
		Pages:           []string{page},
		Entries:         []fixture.Entry{{Name: "Object 1/content.xml", Data: fixture.ODFChart("ff0000")}},
	}
	check := func(img image.Image) {
		t.Helper()
		in := image.Rect(100, 100, 400, 300)
		if n := drawCount(img, in, drawRed, 40); n < 1000 {
			t.Errorf("expected the red bars of the chart in its frame, got %d red pixels", n)
		}
		if n := drawCount(img, img.Bounds(), drawRed, 40) - drawCount(img, in, drawRed, 40); n > 0 {
			t.Errorf("expected no red outside the frame, got %d red pixels", n)
		}
	}
	check(drawRender(t, o.Bytes(), 0))
	// the chart of an encrypted file is decrypted
	buf := fixture.EncryptODF(fixture.ODFEncryption{Password: "secret", Cipher: fixture.ODFAESCBC, Iterations: 100},
		mimePrefix+"presentation", o.Parts()...)
	check(drawRender(t, buf, 0, WithConfig(ivy.Config{Password: "secret"})))
}

func TestDrawText(t *testing.T) {
	page := `><draw:frame draw:style-name="frame" svg:x="50pt" svg:y="50pt" svg:width="500pt" svg:height="100pt"><draw:text-box>
<text:list text:style-name="L1"><text:list-item><text:p><text:span text:style-name="T1">MMM</text:span><text:span text:style-name="T2">MMM</text:span></text:p></text:list-item></text:list>
</draw:text-box></draw:frame>`
	img := drawRender(t, (&fixture.ODP{AutomaticStyles: drawStyles, Pages: []string{page}}).Bytes(), 0)
	// the bullet in the label of 1 cm, then the red run and the blue run
	if n := drawCount(img, image.Rect(50, 50, 78, 110), drawRed, 60); n < 4 {
		t.Errorf("expected a red bullet, got %d red pixels", n)
	}
	if n := drawCount(img, image.Rect(78, 50, 170, 110), drawRed, 60); n < 50 {
		t.Errorf("expected the red run, got %d red pixels", n)
	}
	if n := drawCount(img, image.Rect(170, 50, 300, 110), drawBlue, 60); n < 50 {
		t.Errorf("expected the blue run, got %d blue pixels", n)
	}
	if n := drawCount(img, image.Rect(78, 50, 170, 110), drawBlue, 60); n > 0 {
		t.Errorf("expected no blue before the red run, got %d blue pixels", n)
	}
}

func TestDrawMaster(t *testing.T) {
	// the master page has a background and a shape, which each page draws
	o := &fixture.ODP{
		Styles:          `<style:style style:name="bar" style:family="graphic"><style:graphic-properties draw:fill="solid" draw:fill-color="#00ff00" draw:stroke="none"/></style:style>`,
		MasterStyle:     `draw:fill="solid" draw:fill-color="#00ffff"`,
		MasterShapes:    `<draw:rect draw:style-name="bar" svg:x="0pt" svg:y="400pt" svg:width="794pt" svg:height="40pt"/>`,
		AutomaticStyles: drawStyles,
		Pages:           []string{">"},
	}
	img := drawRender(t, o.Bytes(), 0)
	drawExpect(t, img, 5, 5, color.NRGBA{0, 255, 255, 255})
	drawExpect(t, img, 400, 420, drawGreen)
}

func TestDrawGroupTransform(t *testing.T) {
	// a rectangle of 100 by 50 at the origin, turned by 90 degrees
	// counterclockwise, then moved to (200, 200): it covers x from 200 to
	// 250 and y from 100 to 200
	page := `><draw:g><draw:rect draw:style-name="green" svg:width="100pt" svg:height="50pt" draw:transform="rotate (1.5707963267949) translate (200pt 200pt)"/></draw:g>`
	img := drawRender(t, (&fixture.ODP{AutomaticStyles: drawStyles, Pages: []string{page}}).Bytes(), 0)
	drawExpect(t, img, 225, 150, drawGreen)
	drawExpect(t, img, 260, 150, drawWhite)
	drawExpect(t, img, 225, 210, drawWhite)
}

func TestDrawTable(t *testing.T) {
	styles := drawStyles + `<style:style style:name="c1" style:family="table-cell"><style:graphic-properties draw:fill="solid" draw:fill-color="#ff0000"/></style:style>
<style:style style:name="c2" style:family="table-cell"><style:graphic-properties draw:fill="solid" draw:fill-color="#0000ff"/></style:style>
<style:style style:name="col" style:family="table-column"><style:table-column-properties style:column-width="100pt"/></style:style>
<style:style style:name="row" style:family="table-row"><style:table-row-properties style:row-height="50pt"/></style:style>`
	page := `><draw:frame svg:x="100pt" svg:y="100pt" svg:width="200pt" svg:height="100pt"><table:table>
<table:table-column table:style-name="col" table:number-columns-repeated="2"/>
<table:table-row table:style-name="row"><table:table-cell table:style-name="c1"><text:p>a</text:p></table:table-cell><table:table-cell table:style-name="c2"><text:p>b</text:p></table:table-cell></table:table-row>
<table:table-row table:style-name="row"><table:table-cell table:style-name="c2"><text:p>c</text:p></table:table-cell><table:table-cell table:style-name="c1"><text:p>d</text:p></table:table-cell></table:table-row>
</table:table></draw:frame>`
	img := drawRender(t, (&fixture.ODP{AutomaticStyles: styles, Pages: []string{page}}).Bytes(), 0)
	drawExpect(t, img, 160, 130, drawRed)
	drawExpect(t, img, 260, 130, drawBlue)
	drawExpect(t, img, 160, 180, drawBlue)
	drawExpect(t, img, 260, 180, drawRed)
}

func TestDrawLibreOffice(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "sample.odp"))
	if err != nil {
		t.Fatal(err)
	}
	img := drawRender(t, buf, 0)
	// the text "ivy test slide" of the frame at 2 cm
	if n := drawCount(img, image.Rect(57, 57, 300, 120), color.NRGBA{0, 0, 0, 255}, 100); n < 50 {
		t.Errorf("expected the text of the slide, got %d dark pixels", n)
	}
}

func FuzzDraw(f *testing.F) {
	buf, err := os.ReadFile(filepath.Join("testdata", "sample.odp"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf)
	f.Add((&fixture.ODP{AutomaticStyles: drawStyles, Pages: []string{`><draw:rect draw:style-name="red" svg:x="1cm" svg:y="1cm" svg:width="2cm" svg:height="1cm"><text:p>x</text:p></draw:rect>
<draw:custom-shape draw:style-name="green" svg:width="1cm" svg:height="1cm"><draw:enhanced-geometry draw:enhanced-path="M 0 0 L ?f0 21600 A 0 0 10 10 0 5 5 0 Z N"><draw:equation draw:name="f0" draw:formula="if($0,width/2,?f0)"/></draw:enhanced-geometry></draw:custom-shape>
<draw:path svg:viewBox="0 0 10 10" svg:d="M0 0 L10 10 C 0 5 5 0 10 0z" svg:width="1cm" svg:height="1cm"/>`}}).Bytes())
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1 << 12}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		if _, err := d.PageCount(t.Context()); err != nil {
			return
		}
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		_, _ = ivy.RenderImage(t.Context(), p, 8, nil)
	})
}
