package xps

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// The colors of the pixels that the tests read.
var (
	red   = [3]uint8{255, 0, 0}
	blue  = [3]uint8{0, 0, 255}
	white = [3]uint8{255, 255, 255}
)

// fillPage returns a Path that fills the page of pkgFile with brush.
func fillPage(brush string) string {
	return `<Path Data="M 0,0 H 96 V 96 H 0 Z"><Path.Fill>` + brush + `</Path.Fill></Path>`
}

// redCorner is a visual of 16 by 16 units, red in its top left quarter, and
// transparent elsewhere.
const redCorner = `<VisualBrush.Visual><Path Data="M 0,0 H 8 V 8 H 0 Z" Fill="#FF0000"/></VisualBrush.Visual>`

// near8 reports whether each channel of got is within tol of want.
func near8(got, want [3]uint8, tol int) bool {
	for i := range got {
		if d := int(got[i]) - int(want[i]); d < -tol || d > tol {
			return false
		}
	}
	return true
}

// expectPixels makes sure that img has the colors of want at their points.
func expectPixels(t *testing.T, img image.Image, want map[image.Point][3]uint8, tol int) {
	t.Helper()
	for p, c := range want {
		if got := rgb(img, p.X, p.Y); !near8(got, c, tol) {
			t.Errorf("at %v: expected %v, got: %v", p, c, got)
		}
	}
}

func TestOpacityMask(t *testing.T) {
	gradient := `<LinearGradientBrush MappingMode="Absolute" StartPoint="0,0" EndPoint="96,0">
		<LinearGradientBrush.GradientStops>
			<GradientStop Offset="0" Color="#FF000000"/>
			<GradientStop Offset="1" Color="#00FFFFFF"/>
		</LinearGradientBrush.GradientStops></LinearGradientBrush>`
	tests := []struct {
		name string
		body string
		want map[image.Point][3]uint8
	}{
		{
			"path",
			`<Path Data="M 0,0 H 96 V 96 H 0 Z" Fill="#FF0000"><Path.OpacityMask>` + gradient + `</Path.OpacityMask></Path>`,
			map[image.Point][3]uint8{{1, 50}: red, {48, 50}: {255, 128, 128}, {94, 50}: white},
		},
		{
			"path with an opacity",
			`<Path Data="M 0,0 H 96 V 96 H 0 Z" Fill="#FF0000" Opacity="0.5"><Path.OpacityMask>` + gradient + `</Path.OpacityMask></Path>`,
			map[image.Point][3]uint8{{1, 50}: {255, 128, 128}, {48, 50}: {255, 192, 192}, {94, 50}: white},
		},
		{
			"canvas",
			`<Canvas><Canvas.OpacityMask>` + gradient + `</Canvas.OpacityMask>
				<Path Data="M 0,0 H 96 V 48 H 0 Z" Fill="#FF0000"/>
				<Path Data="M 0,48 H 96 V 96 H 0 Z" Fill="#0000FF"/></Canvas>`,
			map[image.Point][3]uint8{{1, 20}: red, {1, 70}: blue, {48, 70}: {128, 128, 255}, {94, 20}: white},
		},
		{
			"canvas with an opacity and a resource",
			`<FixedPage.Resources><ResourceDictionary>` + strings.Replace(gradient, "<LinearGradientBrush ", `<LinearGradientBrush x:Key="fade" `, 1) +
				`</ResourceDictionary></FixedPage.Resources>
				<Canvas Opacity="0.5" OpacityMask="{StaticResource fade}"><Path Data="M 0,0 H 96 V 96 H 0 Z" Fill="#0000FF"/></Canvas>`,
			map[image.Point][3]uint8{{1, 50}: {128, 128, 255}, {94, 50}: white},
		},
		{
			"solid color",
			`<Path Data="M 0,0 H 96 V 96 H 0 Z" Fill="#0000FF" OpacityMask="#80000000"/>`,
			map[image.Point][3]uint8{{48, 48}: {127, 127, 255}},
		},
		{
			"mask with a render transform",
			// the mask is in the space of the path, after its RenderTransform
			`<Path Data="M 0,0 H 48 V 96 H 0 Z" Fill="#FF0000" RenderTransform="2,0,0,1,0,0"><Path.OpacityMask>
				<LinearGradientBrush MappingMode="Absolute" StartPoint="0,0" EndPoint="48,0"><LinearGradientBrush.GradientStops>
				<GradientStop Offset="0" Color="#FF000000"/><GradientStop Offset="1" Color="#00000000"/>
				</LinearGradientBrush.GradientStops></LinearGradientBrush></Path.OpacityMask></Path>`,
			map[image.Point][3]uint8{{1, 50}: red, {48, 50}: {255, 128, 128}, {94, 50}: white},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			img, err := render(t, pkgFile(test.body))
			if err != nil {
				t.Fatal(err)
			}
			expectPixels(t, img, test.want, 6)
		})
	}
}

func TestVisualBrushTileModes(t *testing.T) {
	const box = ` Viewbox="0,0,16,16" Viewport="0,0,16,16"`
	tests := []struct {
		name  string
		mode  string
		extra string
		want  map[image.Point][3]uint8
	}{
		{"none", "None", box, map[image.Point][3]uint8{
			{4, 4}: red, {12, 4}: white, {20, 4}: white, {4, 20}: white, {60, 60}: white,
		}},
		{"tile", "Tile", box, map[image.Point][3]uint8{
			{4, 4}: red, {12, 4}: white, {20, 4}: red, {28, 4}: white, {4, 20}: red, {20, 20}: red, {52, 68}: red, {60, 68}: white,
		}},
		{"flip x", "FlipX", box, map[image.Point][3]uint8{
			// the second column is mirrored left to right, and the rows
			// are not
			{4, 4}: red, {20, 4}: white, {28, 4}: red, {4, 20}: red, {28, 20}: red, {36, 4}: red, {60, 4}: red, {52, 4}: white,
		}},
		{"flip y", "FlipY", box, map[image.Point][3]uint8{
			{4, 4}: red, {20, 4}: red, {4, 20}: white, {4, 28}: red, {20, 28}: red, {4, 36}: red, {4, 60}: red,
		}},
		{"flip xy", "FlipXY", box, map[image.Point][3]uint8{
			{4, 4}: red, {28, 4}: red, {20, 4}: white, {4, 28}: red, {4, 20}: white, {28, 28}: red, {20, 20}: white, {36, 36}: red,
		}},
		{"tile with a transform", "Tile", box + ` Transform="2,0,0,2,0,0"`, map[image.Point][3]uint8{
			{4, 4}: red, {12, 12}: red, {20, 4}: white, {36, 4}: red, {4, 36}: red,
		}},
		{"tile of a part of the visual", "Tile", ` Viewbox="0,0,8,8" Viewport="0,0,16,16"`, map[image.Point][3]uint8{
			// the viewbox holds the red quarter only, which fills the
			// viewport
			{4, 4}: red, {12, 12}: red, {20, 20}: red, {90, 90}: red,
		}},
		{"tile with an opacity", "Tile", box + ` Opacity="0.5"`, map[image.Point][3]uint8{
			{4, 4}: {255, 128, 128}, {12, 4}: white,
		}},
		{"none with an opacity", "None", ` Viewbox="0,0,16,16" Viewport="16,16,16,16" Opacity="0.5"`, map[image.Point][3]uint8{
			{4, 4}: white, {20, 20}: {255, 128, 128}, {28, 20}: white, {36, 36}: white,
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			brush := `<VisualBrush TileMode="` + test.mode + `" ViewboxUnits="Absolute" ViewportUnits="Absolute"` + test.extra + `>` +
				redCorner + `</VisualBrush>`
			img, err := render(t, pkgFile(fillPage(brush)))
			if err != nil {
				t.Fatal(err)
			}
			expectPixels(t, img, test.want, 2)
		})
	}
}

func TestVisualBrushStroke(t *testing.T) {
	solid := `<VisualBrush Viewbox="0,0,4,4" Viewport="0,0,4,4" TileMode="%s">
		<VisualBrush.Visual><Path Data="M 0,0 H 4 V 4 H 0 Z" Fill="#0000FF"/></VisualBrush.Visual></VisualBrush>`
	for _, mode := range []string{"Tile", "None"} {
		t.Run(mode, func(t *testing.T) {
			body := `<Path Data="M 0,2 H 90" StrokeThickness="4"><Path.Stroke>` + fmt.Sprintf(solid, mode) + `</Path.Stroke></Path>`
			img, err := render(t, pkgFile(body))
			if err != nil {
				t.Fatal(err)
			}
			want := map[image.Point][3]uint8{{2, 1}: blue, {2, 10}: white}
			if mode == "Tile" {
				want[image.Point{60, 1}] = blue
			} else {
				want[image.Point{60, 1}] = white
			}
			expectPixels(t, img, want, 2)
		})
	}
}

// chain returns a page that fills itself with the first of n visual
// brushes. The visual of each brush is a path filled with the next one, and
// the visual of the last one is a red path.
func chain(n int) string {
	var b strings.Builder
	b.WriteString(`<FixedPage.Resources><ResourceDictionary>`)
	for i := range n {
		fill := fmt.Sprintf(`{StaticResource v%d}`, i+1)
		if i == n-1 {
			fill = "#FF0000"
		}
		fmt.Fprintf(&b, `<VisualBrush x:Key="v%d" Viewbox="0,0,16,16" Viewport="0,0,16,16" TileMode="Tile">`+
			`<VisualBrush.Visual><Path Data="M 0,0 H 8 V 8 H 0 Z" Fill="%s"/></VisualBrush.Visual></VisualBrush>`, i, fill)
	}
	b.WriteString(`</ResourceDictionary></FixedPage.Resources><Path Data="M 0,0 H 96 V 96 H 0 Z" Fill="{StaticResource v0}"/>`)
	return b.String()
}

func TestVisualBrushNesting(t *testing.T) {
	// maxVisuals brushes nest, each inside the visual of the one before
	img, err := render(t, pkgFile(chain(maxVisuals)))
	if err != nil {
		t.Fatal(err)
	}
	expectPixels(t, img, map[image.Point][3]uint8{{2, 2}: red, {6, 6}: red, {18, 2}: red, {12, 12}: white, {12, 4}: white}, 2)
	// one more stops at the limit, and the page is partial
	_, err = render(t, pkgFile(chain(maxVisuals+1)))
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok || !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected a partial page at the limit, got: %v", err)
	}
	// Limits.MaxDepth bounds the depth too
	d, err := Open(t.Context(), bytes.NewReader(pkgFile(chain(2))), WithConfig(ivy.Config{Limits: ivy.Limits{MaxDepth: 1}}))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.RenderImage(t.Context(), p, 96, color.White); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit for a depth of 1, got: %v", err)
	}
}

func TestVisualBrushRecursive(t *testing.T) {
	tests := map[string]string{
		"tile": `<VisualBrush x:Key="self" Viewbox="0,0,16,16" Viewport="0,0,16,16" TileMode="Tile"><VisualBrush.Visual><Canvas>
			<Path Data="M 0,0 H 8 V 8 H 0 Z" Fill="#FF0000"/>` +
			strings.Repeat(`<Path Data="M 8,8 H 16 V 16 H 8 Z" Fill="{StaticResource self}"/>`, 8) +
			`</Canvas></VisualBrush.Visual></VisualBrush>`,
		"none": `<VisualBrush x:Key="self" Viewbox="0,0,16,16" Viewport="0,0,16,16"><VisualBrush.Visual><Canvas>
			<Path Data="M 0,0 H 8 V 8 H 0 Z" Fill="#FF0000"/>` +
			strings.Repeat(`<Path Data="M 0,0 H 16 V 16 H 0 Z" Fill="{StaticResource self}"/>`, 8) +
			`</Canvas></VisualBrush.Visual></VisualBrush>`,
	}
	for name, brush := range tests {
		t.Run(name, func(t *testing.T) {
			body := `<FixedPage.Resources><ResourceDictionary>` + brush + `</ResourceDictionary></FixedPage.Resources>` +
				`<Path Data="M 0,0 H 96 V 96 H 0 Z" Fill="{StaticResource self}"/>`
			start := time.Now()
			img, err := render(t, pkgFile(body))
			if _, ok := errors.AsType[*ivy.PartialError](err); !ok || !errors.Is(err, ivy.ErrLimit) {
				t.Errorf("expected a partial page at the limit, got: %v", err)
			}
			if d := time.Since(start); d > 10*time.Second {
				t.Errorf("expected the recursion to stop quickly, took: %v", d)
			}
			if got := rgb(img, 4, 4); got != red {
				t.Errorf("expected the visual, got: %v", got)
			}
		})
	}
}

func TestVisualBrushLimits(t *testing.T) {
	brush := `<VisualBrush Viewbox="0,0,16,16" Viewport="0,0,16,16" TileMode="Tile">` + redCorner + `</VisualBrush>`
	body := fillPage(brush) + fillPage(brush)
	d, err := Open(t.Context(), bytes.NewReader(pkgFile(body)), WithConfig(ivy.Config{Limits: ivy.Limits{MaxPixels: 300}}))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	// the first tile of 256 pixels draws, and the second one passes the
	// limit
	img, err := ivy.RenderImage(t.Context(), p, 96, color.White)
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok || !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected a partial page at the limit, got: %v", err)
	}
	if got := rgb(img, 4, 4); got != red {
		t.Errorf("expected the first tile, got: %v", got)
	}
	// a tile larger than the limit draws at a lower resolution
	d, err = Open(t.Context(), bytes.NewReader(pkgFile(fillPage(brush))), WithConfig(ivy.Config{Limits: ivy.Limits{MaxPixels: 64}}))
	if err != nil {
		t.Fatal(err)
	}
	p, err = d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err = ivy.RenderImage(t.Context(), p, 96, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if got := rgb(img, 2, 2); got[0] < 200 || got[1] > 80 {
		t.Errorf("expected red, got: %v", got)
	}
}

func TestVisualBrushPartial(t *testing.T) {
	body := fillPage(`<VisualBrush Viewbox="0,0,16,16" Viewport="0,0,16,16" TileMode="Tile"><VisualBrush.Visual><Canvas>
		<Path Data="M 0,0 H 8 V 8 H 0 Z" Fill="#FF0000"/>
		<Glyphs FontUri="/missing.ttf" FontRenderingEmSize="10" UnicodeString="a" Fill="#000000"/>
		</Canvas></VisualBrush.Visual></VisualBrush>`) +
		`<Path Data="M 80,80 H 96 V 96 H 80 Z" Fill="#0000FF"/>`
	img, err := render(t, pkgFile(body))
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok {
		t.Fatalf("expected a *PartialError, got: %v", err)
	}
	expectPixels(t, img, map[image.Point][3]uint8{{4, 4}: red, {20, 4}: red, {90, 90}: blue}, 2)
	for _, bad := range []string{
		`<VisualBrush Viewbox="0,0,16,16" Viewport="0,0,16,16"/>`,
		`<VisualBrush Viewport="0,0,16,16">` + redCorner + `</VisualBrush>`,
		`<VisualBrush Viewbox="0,0,16,16" Viewport="0,0,16,16" Visual="{StaticResource missing}"/>`,
	} {
		if _, err := render(t, pkgFile(fillPage(bad))); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%s: expected ErrFormat, got: %v", bad, err)
		}
	}
}

// pngFile returns a PNG of 2 by 2 pixels: red at the top left, and
// transparent elsewhere.
func pngFile(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestImageBrushFlip(t *testing.T) {
	tests := []struct {
		mode string
		red  []image.Point
		none []image.Point
	}{
		{"Tile", []image.Point{{4, 4}, {20, 4}, {4, 20}}, []image.Point{{12, 4}, {28, 4}, {4, 12}}},
		{"FlipX", []image.Point{{4, 4}, {28, 4}, {4, 20}}, []image.Point{{20, 4}, {12, 4}}},
		{"FlipY", []image.Point{{4, 4}, {20, 4}, {4, 28}}, []image.Point{{4, 20}, {4, 12}}},
		{"FlipXY", []image.Point{{4, 4}, {28, 4}, {4, 28}, {28, 28}}, []image.Point{{20, 20}, {20, 4}, {4, 20}}},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			brush := `<ImageBrush ImageSource="/red.png" Viewbox="0,0,2,2" Viewport="0,0,16,16" TileMode="` + test.mode + `"/>`
			img, err := render(t, pkgFile(fillPage(brush), fixture.Entry{Name: "red.png", Data: pngFile(t)}))
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range test.red {
				if got := rgb(img, p.X, p.Y); got[0] < 200 || got[1] > 80 {
					t.Errorf("at %v: expected red, got: %v", p, got)
				}
			}
			for _, p := range test.none {
				if got := rgb(img, p.X, p.Y); got[1] < 180 {
					t.Errorf("at %v: expected white, got: %v", p, got)
				}
			}
		})
	}
}
