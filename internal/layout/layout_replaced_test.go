// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_image.py: the tests of the layout of the
// images. It also tests replacedBoxLayout, which the drawing calls.

package layout

import (
	"fmt"
	"log/slog"
	"math"
	"strings"
	"testing"
)

// inlineGetImg is get_img in tests/layout/test_image.py: the body of a
// document of one page, and its image, or nil.
func inlineGetImg(t *testing.T, html string) (body, img *Box) {
	t.Helper()
	body = inlineBody(t, html)
	if len(body.Children) > 0 {
		line := only(t, body)
		img = only(t, line)
	}
	return body, img
}

// inlineApprox reports whether two lists of numbers are equal, as
// pytest.approx compares them.
func inlineApprox(got, want []float64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > max(1e-6*math.Abs(want[i]), 1e-12) {
			return false
		}
	}
	return true
}

func TestImages1(t *testing.T) {
	sources := make([]string, 0, 14)
	for _, url := range []string{
		"pattern.png", "pattern.gif", "blue.jpg", "pattern.svg",
		"data:image/svg+xml,<svg width='4' height='4'></svg>",
		"DatA:image/svg+xml,<svg width='4px' height='4px'></svg>",
	} {
		sources = append(sources, fmt.Sprintf(`<img src="%s">`, url))
	}
	sources = append(sources,
		"<embed src=pattern.png>",
		"<embed src=pattern.svg>",
		"<embed src=really-a-png.svg type=image/png>",
		"<embed src=really-a-svg.png type=image/svg+xml>",

		"<object data=pattern.png>",
		"<object data=pattern.svg>",
		"<object data=really-a-png.svg type=image/png>",
		"<object data=really-a-svg.png type=image/svg+xml>",
	)
	for _, html := range sources {
		t.Run(html, func(t *testing.T) {
			_, img := inlineGetImg(t, html)
			if img == nil {
				t.Fatal("no image")
			}
			inlineEqual(t, "img.width", img.Width, 4)
			inlineEqual(t, "img.height", img.Height, 4)
		})
	}
}

func TestImages2(t *testing.T) {
	// With physical units
	url := "data:image/svg+xml,<svg width='2.54cm' height='0.5in'></svg>"
	_, img := inlineGetImg(t, fmt.Sprintf(`<img src="%s">`, url))
	inlineEqual(t, "img.width", img.Width, 96)
	inlineEqual(t, "img.height", img.Height, 48)
}

func TestImages3(t *testing.T) {
	for _, url := range []string{
		"nonexistent.png",
		"unknownprotocol://weasyprint.org/foo.png",
		"data:image/unknowntype,Not an image",
		// Invalid protocol
		`datå:image/svg+xml,<svg width="4" height="4"></svg>`,
		// zero-byte images
		"data:image/png,",
		"data:image/jpeg,",
		"data:image/svg+xml,",
		// Incorrect format
		"data:image/png,Not a PNG",
		"data:image/jpeg,Not a JPEG",
		"data:image/svg+xml,<svg>invalid xml",
	} {
		t.Run(url, func(t *testing.T) {
			// Invalid images
			log, c := captureLogs(slog.LevelWarn)
			pages := renderPagesLog(t, fmt.Sprintf("<img src='%s' alt='invalid image'>", url), testBaseURL, log)
			logs := c.logs()
			if len(logs) != 1 || !strings.Contains(logs[0], "ERROR: Failed to load image") {
				t.Errorf("logs are %q", logs)
			}
			_, body := pageBody(t, unpack(t, &Box{Children: pages}, 1)[0])
			img := only(t, only(t, body))
			if img.Kind != KindInline { // not a replaced box
				t.Errorf("%s is not an inline box", img)
			}
			inlineText(t, only(t, img), "invalid image")
		})
	}
}

func TestImages4(t *testing.T) {
	for _, url := range []string{
		// GIF with JPEG mimetype
		"data:image/jpeg;base64," +
			"R0lGODlhAQABAIABAP///wAAACwAAAAAAQABAAACAkQBADs=",
		// GIF with PNG mimetype
		"data:image/png;base64," +
			"R0lGODlhAQABAIABAP///wAAACwAAAAAAQABAAACAkQBADs=",
		// PNG with JPEG mimetype
		"data:image/jpeg;base64," +
			"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC" +
			"0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=",
		// SVG with PNG mimetype
		`data:image/png,<svg width="1" height="1"></svg>`,
		"really-a-svg.png",
		// PNG with SVG
		"data:image/svg+xml;base64," +
			"R0lGODlhAQABAIABAP///wAAACwAAAAAAQABAAACAkQBADs=",
		"really-a-png.svg",
	} {
		t.Run(url, func(t *testing.T) {
			// Sniffing, no logs
			inlineGetImg(t, fmt.Sprintf("<img src='%s'>", url))
		})
	}
}

func TestImages5(t *testing.T) {
	log, c := captureLogs(slog.LevelWarn)
	renderPagesLog(t, "<img src=nonexistent.png><img src=nonexistent.png>", testBaseURL, log)
	// Failures are cached too: only one error
	logs := c.logs()
	if len(logs) != 1 || !strings.Contains(logs[0], "ERROR: Failed to load image") {
		t.Errorf("logs are %q", logs)
	}
}

func TestImages6(t *testing.T) {
	// Layout rules try to preserve the ratio, so the height should be 40px
	// too:
	body, img := inlineGetImg(t, `<body style="font-size: 0">
        <img src="pattern.png" style="width: 40px">`)
	inlineEqual(t, "body.height", body.Height, 40)
	inlineEqual(t, "img.position_y", img.PositionY, 0)
	inlineEqual(t, "img.width", img.Width, 40)
	inlineEqual(t, "img.height", img.Height, 40)
}

func TestImages7(t *testing.T) {
	body, img := inlineGetImg(t, `<body style="font-size: 0">
        <img src="pattern.png" style="height: 40px">`)
	inlineEqual(t, "body.height", body.Height, 40)
	inlineEqual(t, "img.position_y", img.PositionY, 0)
	inlineEqual(t, "img.width", img.Width, 40)
	inlineEqual(t, "img.height", img.Height, 40)
}

func TestImages8(t *testing.T) {
	// Same with percentages
	body := inlineBody(t, `<body style="font-size: 0"><p style="width: 200px">
        <img src="pattern.png" style="width: 20%">`)
	img := only(t, only(t, only(t, body)))
	inlineEqual(t, "body.height", body.Height, 40)
	inlineEqual(t, "img.position_y", img.PositionY, 0)
	inlineEqual(t, "img.width", img.Width, 40)
	inlineEqual(t, "img.height", img.Height, 40)
}

func TestImages9(t *testing.T) {
	body, img := inlineGetImg(t, `<body style="font-size: 0">
        <img src="pattern.png" style="min-width: 40px">`)
	inlineEqual(t, "body.height", body.Height, 40)
	inlineEqual(t, "img.position_y", img.PositionY, 0)
	inlineEqual(t, "img.width", img.Width, 40)
	inlineEqual(t, "img.height", img.Height, 40)
}

func TestImages10(t *testing.T) {
	_, img := inlineGetImg(t, `<img src="pattern.png" style="max-width: 2px">`)
	inlineEqual(t, "img.width", img.Width, 2)
	inlineEqual(t, "img.height", img.Height, 2)
}

func TestImages11(t *testing.T) {
	// display: table-cell is ignored. XXX Should it?
	body := inlineBody(t, `<body style="font-size: 0">
        <img src="pattern.png" style="width: 40px">
        <img src="pattern.png" style="width: 60px; display: table-cell">
    `)
	line := only(t, body)
	imgs := unpack(t, line, 2)
	inlineEqual(t, "body.height", body.Height, 60)
	inlineEqual(t, "img_1.width", imgs[0].Width, 40)
	inlineEqual(t, "img_1.height", imgs[0].Height, 40)
	inlineEqual(t, "img_2.width", imgs[1].Width, 60)
	inlineEqual(t, "img_2.height", imgs[1].Height, 60)
	inlineEqual(t, "img_1.position_y", imgs[0].PositionY, 20)
	inlineEqual(t, "img_2.position_y", imgs[1].PositionY, 0)
}

// inlineBlockImage checks the block-level image of test_images_12 to
// test_images_15.
func inlineBlockImage(t *testing.T, css string, width, height, contentBoxX float64) {
	t.Helper()
	body := inlineBody(t, `
        <style>
            @page { size: 100px }
            img { `+css+`; margin: 10px auto; display: block }
        </style>
        <body>
            <img src="pattern.png">
    `)
	img := only(t, body)
	if img.ElementTag != "img" {
		t.Errorf("element tag %q", img.ElementTag)
	}
	inlineEqual(t, "img.position_x", img.PositionX, 0)
	inlineEqual(t, "img.position_y", img.PositionY, 0)
	inlineEqual(t, "img.width", img.Width, width)
	inlineEqual(t, "img.height", img.Height, height)
	inlineEqual(t, "img.content_box_x()", img.ContentBoxX(), contentBoxX)
	inlineEqual(t, "img.content_box_y()", img.ContentBoxY(), 10)
}

func TestImages12(t *testing.T) {
	// Block-level image:
	inlineBlockImage(t, "width: 40px", 40, 40, 30) // (100 - 40) / 2 == 30px for margin-left
}

func TestImages13(t *testing.T) {
	inlineBlockImage(t, "min-width: 40%", 40, 40, 30) // (100 - 40) / 2 == 30px for margin-left
}

func TestImages14(t *testing.T) {
	inlineBlockImage(t, "min-width: 40px", 40, 40, 30) // (100 - 40) / 2 == 30px for margin-left
}

func TestImages15(t *testing.T) {
	inlineBlockImage(t, "min-height: 30px; max-width: 2px", 2, 30, 49) // (100 - 2) / 2 == 49px for margin-left
}

func TestImages16(t *testing.T) {
	body := inlineBody(t, `
        <body style="float: left">
        <img style="height: 200px; margin: 10px; display: block" src="
            data:image/svg+xml,
            <svg width='150' height='100'></svg>
        ">
    `)
	img := only(t, body)
	inlineEqual(t, "body.width", body.Width, 320)
	inlineEqual(t, "body.height", body.Height, 220)
	if img.ElementTag != "img" {
		t.Errorf("element tag %q", img.ElementTag)
	}
	inlineEqual(t, "img.width", img.Width, 300)
	inlineEqual(t, "img.height", img.Height, 200)
}

func TestImages17(t *testing.T) {
	body := inlineBody(t, `
        <div style="width: 300px; height: 300px">
        <img src="
            data:image/svg+xml,
            <svg viewBox='0 0 20 10'></svg>
        ">`)
	div := only(t, body)
	img := only(t, only(t, div))
	inlineEqual(t, "div.width", div.Width, 300)
	inlineEqual(t, "div.height", div.Height, 300)
	if img.ElementTag != "img" {
		t.Errorf("element tag %q", img.ElementTag)
	}
	inlineEqual(t, "img.width", img.Width, 300)
	inlineEqual(t, "img.height", img.Height, 150)
}

func TestImages18(t *testing.T) {
	// Regression test for #1050.
	renderPages(t, `
        <img style="position: absolute" src="
            data:image/svg+xml,
            <svg viewBox='0 0 20 10'></svg>
        ">`)
}

func TestImages19(t *testing.T) {
	for _, c := range []struct {
		html     string
		children []string
	}{
		{"<embed>", nil},
		{`<embed src="unknown">`, nil},
		{"<object></object>", nil},
		{`<object data="unknown"></object>`, nil},
		{"<object>abc</object>", []string{"TextBox"}},
		{`<object data="unknown">abc</object>`, []string{"TextBox"}},
	} {
		t.Run(c.html, func(t *testing.T) {
			pages := renderPages(t, c.html)
			_, body := pageBody(t, unpack(t, &Box{Children: pages}, 1)[0])
			var img *Box
			if len(body.Children) > 0 {
				img = only(t, only(t, body))
			}
			var names []string
			if img != nil {
				for _, child := range img.Children {
					names = append(names, child.Kind.String()+"Box")
				}
			}
			if strings.Join(names, ",") != strings.Join(c.children, ",") {
				t.Errorf("children are %q, want %q", names, c.children)
			}
		})
	}
}

// gradientLayouter is a gradient: a *LinearGradient or a *RadialGradient.
type gradientLayouter interface {
	Layout(width, height float64, style *Style) GradientLayout
}

// inlineGradient lays out the background gradient of a page, as the
// function layout of test_linear_gradient and test_radial_gradient does.
func inlineGradient(t *testing.T, gradientCSS string) GradientLayout {
	t.Helper()
	page := inlineRender(t, "<style>@page { background: "+gradientCSS)
	if page.Background == nil || len(page.Background.Layers) != 1 {
		t.Fatalf("the page has no layer of background")
	}
	image, ok := page.Background.Layers[0].Image.(gradientLayouter)
	if !ok {
		t.Fatalf("the image %T is not a gradient", page.Background.Layers[0].Image)
	}
	return image.Layout(400, 300, page.Style)
}

// inlineCheckGradient checks the result of the layout of a gradient. A nil
// init is None.
func inlineCheckGradient(t *testing.T, result GradientLayout, scaleY float64, typ string, init, positions []float64, colors [][4]float64) {
	t.Helper()
	if math.Abs(result.ScaleY-scaleY) > 1e-6*math.Abs(scaleY) {
		t.Errorf("scale_y = %v, want %v", result.ScaleY, scaleY)
	}
	if result.Type != typ {
		t.Errorf("type = %q, want %q", result.Type, typ)
	}
	if (init == nil) != (result.Points == nil) || !inlineApprox(result.Points, init) {
		t.Errorf("init = %v, want %v", result.Points, init)
	}
	if !inlineApprox(result.Positions, positions) {
		t.Errorf("positions = %v, want %v", result.Positions, positions)
	}
	for i := range min(len(result.Colors), len(colors)) {
		r, g, b, a := srgba(result.Colors[i])
		if !inlineApprox([]float64{r, g, b, a}, colors[i][:]) {
			t.Errorf("color %d = %v, want %v", i, []float64{r, g, b, a}, colors[i])
		}
	}
}

func TestLayoutLinearGradient(t *testing.T) {
	red := [4]float64{1, 0, 0, 1}
	lime := [4]float64{0, 1, 0, 1}
	blue := [4]float64{0, 0, 1, 1}
	blueLime := [][4]float64{blue, lime}

	for _, c := range []struct {
		css       string
		typ       string
		init      []float64
		positions []float64
		colors    [][4]float64
	}{
		{"linear-gradient(blue)", "solid", nil, nil, [][4]float64{blue}},
		{"repeating-linear-gradient(blue)", "solid", nil, nil, [][4]float64{blue}},
		{"linear-gradient(blue, lime)", "linear", []float64{200, 0, 200, 300}, nil, nil},
		{"repeating-linear-gradient(blue, lime)", "linear", []float64{200, 0, 200, 300}, nil, nil},
		{"repeating-linear-gradient(blue, lime 100px)", "linear", []float64{200, 0, 200, 300}, []float64{0, 1, 1, 2, 2, 3}, nil},

		{"linear-gradient(to bottom, blue, lime)", "linear", []float64{200, 0, 200, 300}, nil, nil},
		{"linear-gradient(to top, blue, lime)", "linear", []float64{200, 300, 200, 0}, nil, nil},
		{"linear-gradient(to right, blue, lime)", "linear", []float64{0, 150, 400, 150}, nil, nil},
		{"linear-gradient(to left, blue, lime)", "linear", []float64{400, 150, 0, 150}, nil, nil},

		{"linear-gradient(to top left, blue, lime)", "linear", []float64{344, 342, 56, -42}, nil, nil},
		{"linear-gradient(to top right, blue, lime)", "linear", []float64{56, 342, 344, -42}, nil, nil},
		{"linear-gradient(to bottom left, blue, lime)", "linear", []float64{344, -42, 56, 342}, nil, nil},
		{"linear-gradient(to bottom right, blue, lime)", "linear", []float64{56, -42, 344, 342}, nil, nil},

		{"linear-gradient(270deg, blue, lime)", "linear", []float64{400, 150, 0, 150}, nil, nil},
		{"linear-gradient(.75turn, blue, lime)", "linear", []float64{400, 150, 0, 150}, nil, nil},
		{"linear-gradient(45deg, blue, lime)", "linear", []float64{25, 325, 375, -25}, nil, nil},
		{"linear-gradient(.125turn, blue, lime)", "linear", []float64{25, 325, 375, -25}, nil, nil},
		{"linear-gradient(.375turn, blue, lime)", "linear", []float64{25, -25, 375, 325}, nil, nil},
		{"linear-gradient(.625turn, blue, lime)", "linear", []float64{375, -25, 25, 325}, nil, nil},
		{"linear-gradient(.875turn, blue, lime)", "linear", []float64{375, 325, 25, -25}, nil, nil},

		{"linear-gradient(blue 2em, lime 20%)", "linear", []float64{200, 32, 200, 60}, nil, nil},
		{"linear-gradient(blue 100px, red, blue, red 160px, lime)", "linear", []float64{200, 100, 200, 300},
			[]float64{0, .1, .2, .3, 1}, [][4]float64{blue, red, blue, red, lime}},
		{"linear-gradient(blue -100px, blue 0, red -12px, lime 50%)", "linear", []float64{200, -100, 200, 150},
			[]float64{0, .4, .4, 1}, [][4]float64{blue, blue, red, lime}},
		{"linear-gradient(blue, blue, red, lime -7px)", "linear", []float64{200, -1, 200, 1},
			[]float64{0, 0.5, 0.5, 0.5, 0.5, 1}, [][4]float64{blue, blue, blue, red, lime, lime}},
		{"repeating-linear-gradient(blue, blue, lime, lime -7px)", "solid", nil, nil, [][4]float64{{0, .5, .5, 1}}},
	} {
		t.Run(c.css, func(t *testing.T) {
			positions, colors := c.positions, c.colors
			if positions == nil && c.typ != "solid" {
				positions = []float64{0, 1}
			}
			if colors == nil {
				colors = blueLime
			}
			inlineCheckGradient(t, inlineGradient(t, c.css), 1, c.typ, c.init, positions, colors)
		})
	}
}

func TestLayoutRadialGradient(t *testing.T) {
	red := [4]float64{1, 0, 0, 1}
	lime := [4]float64{0, 1, 0, 1}
	blue := [4]float64{0, 0, 1, 1}
	blueLime := [][4]float64{blue, lime}
	sqrt2 := math.Sqrt2

	for _, c := range []struct {
		css       string
		typ       string
		init      []float64
		positions []float64
		colors    [][4]float64
		scaleY    float64
	}{
		{"radial-gradient(blue)", "solid", nil, nil, [][4]float64{blue}, 1},
		{"repeating-radial-gradient(blue)", "solid", nil, nil, [][4]float64{blue}, 1},
		{"radial-gradient(100px, blue, lime)", "radial", []float64{200, 150, 0, 100}, nil, nil, 1},

		{"radial-gradient(100px at right 20px bottom 30px, lime, red)", "radial", []float64{380, 270, 0, 100}, nil, [][4]float64{lime, red}, 1},
		{"radial-gradient(0 0, blue, lime)", "radial", []float64{200, 150, 0, 1e-7}, nil, nil, 1},
		{"radial-gradient(1px 0, blue, lime)", "radial", []float64{200, 150, 0, 1e7}, nil, nil, 1e-14},
		{"radial-gradient(0 1px, blue, lime)", "radial", []float64{200, 150, 0, 1e-7}, nil, nil, 1e14},
		{"repeating-radial-gradient(100px 200px, blue, lime)", "radial", []float64{200, 150, 0, 300},
			[]float64{0, 1, 1, 2, 2, 3}, nil, 200.0 / 100},
		{"repeating-radial-gradient(42px, blue -100px, lime 100px)", "radial", []float64{200, 150, 0, 300},
			[]float64{-0.5, 0, 0, 1}, [][4]float64{{0, 0.5, 0.5, 1}, lime, blue, lime}, 1},
		{"radial-gradient(42px, blue -20px, lime -1px)", "solid", nil, nil, [][4]float64{lime}, 1},
		{"radial-gradient(42px, blue -20px, lime 0)", "solid", nil, nil, [][4]float64{lime}, 1},
		{"radial-gradient(42px, blue -20px, lime 20px)", "radial", []float64{200, 150, 0, 20}, nil, [][4]float64{{0, .5, .5, 1}, lime}, 1},

		{"radial-gradient(100px 120px, blue, lime)", "radial", []float64{200, 150, 0, 100}, nil, nil, 120.0 / 100},
		{"radial-gradient(25% 40%, blue, lime)", "radial", []float64{200, 150, 0, 100}, nil, nil, 120.0 / 100},

		{"radial-gradient(circle closest-side, blue, lime)", "radial", []float64{200, 150, 0, 150}, nil, nil, 1},
		{"radial-gradient(circle closest-side at 150px 50px, blue, lime)", "radial", []float64{150, 50, 0, 50}, nil, nil, 1},
		{"radial-gradient(circle closest-side at 45px 50px, blue, lime)", "radial", []float64{45, 50, 0, 45}, nil, nil, 1},
		{"radial-gradient(circle closest-side at 420px 50px, blue, lime)", "radial", []float64{420, 50, 0, 20}, nil, nil, 1},
		{"radial-gradient(circle closest-side at 420px 281px, blue, lime)", "radial", []float64{420, 281, 0, 19}, nil, nil, 1},

		{"radial-gradient(closest-side, blue 20%, lime)", "radial", []float64{200, 150, 40, 200}, nil, nil, 150.0 / 200},
		{"radial-gradient(closest-side at 300px 20%, blue, lime)", "radial", []float64{300, 60, 0, 100}, nil, nil, 60.0 / 100},
		{"radial-gradient(closest-side at 10% 230px, blue, lime)", "radial", []float64{40, 230, 0, 40}, nil, nil, 70.0 / 40},

		{"radial-gradient(circle farthest-side, blue, lime)", "radial", []float64{200, 150, 0, 200}, nil, nil, 1},
		{"radial-gradient(circle farthest-side at 150px 50px, blue, lime)", "radial", []float64{150, 50, 0, 250}, nil, nil, 1},
		{"radial-gradient(circle farthest-side at 45px 50px, blue, lime)", "radial", []float64{45, 50, 0, 355}, nil, nil, 1},
		{"radial-gradient(circle farthest-side at 420px 50px, blue, lime)", "radial", []float64{420, 50, 0, 420}, nil, nil, 1},
		{"radial-gradient(circle farthest-side at 220px 310px, blue, lime)", "radial", []float64{220, 310, 0, 310}, nil, nil, 1},

		{"radial-gradient(farthest-side, blue, lime)", "radial", []float64{200, 150, 0, 200}, nil, nil, 150.0 / 200},
		{"radial-gradient(farthest-side at 300px 20%, blue, lime)", "radial", []float64{300, 60, 0, 300}, nil, nil, 240.0 / 300},
		{"radial-gradient(farthest-side at 10% 230px, blue, lime)", "radial", []float64{40, 230, 0, 360}, nil, nil, 230.0 / 360},

		{"radial-gradient(circle closest-corner, blue, lime)", "radial", []float64{200, 150, 0, 250}, nil, nil, 1},
		{"radial-gradient(circle closest-corner at 340px 80px, blue, lime)", "radial", []float64{340, 80, 0, 100}, nil, nil, 1},
		{"radial-gradient(circle closest-corner at 0 342px, blue, lime)", "radial", []float64{0, 342, 0, 42}, nil, nil, 1},

		{"radial-gradient(closest-corner, blue, lime)", "radial", []float64{200, 150, 0, 200 * sqrt2}, nil, nil, 150.0 / 200},
		{"radial-gradient(closest-corner at 450px 100px, blue, lime)", "radial", []float64{450, 100, 0, 50 * sqrt2}, nil, nil, 100.0 / 50},
		{"radial-gradient(closest-corner at 40px 210px, blue, lime)", "radial", []float64{40, 210, 0, 40 * sqrt2}, nil, nil, 90.0 / 40},

		{"radial-gradient(circle farthest-corner, blue, lime)", "radial", []float64{200, 150, 0, 250}, nil, nil, 1},
		{"radial-gradient(circle farthest-corner at 300px -100px, blue, lime)", "radial", []float64{300, -100, 0, 500}, nil, nil, 1},
		{"radial-gradient(circle farthest-corner at 400px 0, blue, lime)", "radial", []float64{400, 0, 0, 500}, nil, nil, 1},

		{"radial-gradient(farthest-corner, blue, lime)", "radial", []float64{200, 150, 0, 200 * sqrt2}, nil, nil, 150.0 / 200},
		{"radial-gradient(farthest-corner at 450px 100px, blue, lime)", "radial", []float64{450, 100, 0, 450 * sqrt2}, nil, nil, 200.0 / 450},
		{"radial-gradient(farthest-corner at 40px 210px, blue, lime)", "radial", []float64{40, 210, 0, 360 * sqrt2}, nil, nil, 210.0 / 360},
	} {
		t.Run(c.css, func(t *testing.T) {
			init, positions, colors := c.init, c.positions, c.colors
			if c.typ == "radial" {
				centerX, centerY, radius0, radius1 := init[0], init[1], init[2], init[3]
				init = []float64{centerX, centerY / c.scaleY, radius0, centerX, centerY / c.scaleY, radius1}
			}
			if positions == nil && c.typ != "solid" {
				positions = []float64{0, 1}
			}
			if colors == nil {
				colors = blueLime
			}
			inlineCheckGradient(t, inlineGradient(t, c.css), c.scaleY, c.typ, init, positions, colors)
		})
	}
}

func TestImageMinMaxWidth(t *testing.T) {
	keys := []string{"min-width", "max-width", "width", "min-height", "max-height", "height"}
	defaults := map[string]string{
		"min-width": "auto", "max-width": "none", "width": "auto",
		"min-height": "auto", "max-height": "none", "height": "auto",
	}
	for _, c := range []struct {
		props    map[string]string
		divWidth float64
	}{
		{map[string]string{}, 4},
		{map[string]string{"min-width": "10px"}, 10},
		{map[string]string{"max-width": "1px"}, 1},
		{map[string]string{"width": "10px"}, 10},
		{map[string]string{"width": "1px"}, 1},
		{map[string]string{"min-height": "10px"}, 10},
		{map[string]string{"max-height": "1px"}, 1},
		{map[string]string{"height": "10px"}, 10},
		{map[string]string{"height": "1px"}, 1},
		{map[string]string{"min-width": "10px", "min-height": "1px"}, 10},
		{map[string]string{"min-width": "1px", "min-height": "10px"}, 10},
		{map[string]string{"max-width": "10px", "max-height": "1px"}, 1},
		{map[string]string{"max-width": "1px", "max-height": "10px"}, 1},
	} {
		declarations := make([]string, 0, len(keys))
		for _, key := range keys {
			value, ok := c.props[key]
			if !ok {
				value = defaults[key]
			}
			declarations = append(declarations, key+": "+value)
		}
		css := strings.Join(declarations, ";")
		t.Run(fmt.Sprint(c.props), func(t *testing.T) {
			body := inlineBody(t, `
      <style> img { display: block; `+css+` } </style>
      <div style="display: inline-block">
        <img src="pattern.png"><img src="pattern.svg">
      </div>`)
			div := only(t, only(t, body))
			inlineEqual(t, "div.width", div.Width, c.divWidth)
		})
	}
}

// inlineSVGNoSizeCases are the cases of test_svg_no_size_width and
// test_svg_no_size_min_width.
var inlineSVGNoSizeCases = []struct {
	css   string
	width float64
}{
	{"width: 10px", 10},
	{"width: 1px", 1},
	{"height: 10px", 20},
	{"height: 1px", 2},
}

func TestSVGNoSizeWidth(t *testing.T) {
	for _, c := range inlineSVGNoSizeCases {
		t.Run(c.css, func(t *testing.T) {
			// Size is undefined when both width and heigh are not set.
			// https://drafts.csswg.org/css2/#inline-replaced-width
			body := inlineBody(t, `
      <style> svg { `+c.css+` } </style>
      <div style="display: inline-block">
        <svg viewBox="0 0 8 4"></svg>
      </div>`)
			div := only(t, only(t, body))
			inlineEqual(t, "div.width", div.Width, c.width)
		})
	}
}

func TestSVGNoSizeMinWidth(t *testing.T) {
	for _, c := range inlineSVGNoSizeCases {
		t.Run(c.css, func(t *testing.T) {
			// Size is undefined when both width and heigh are not set.
			// https://drafts.csswg.org/css2/#inline-replaced-width
			body := inlineBody(t, `
      <style> svg { `+c.css+` } </style>
      <table>
        <tr>
          <td><svg viewBox="0 0 8 4"></svg></td>
          <td style="width: 1000mm"></td>
        </tr>
      </table>`)
			wrapper := only(t, body)
			table := only(t, wrapper)
			rowGroup := only(t, table)
			row := only(t, rowGroup)
			td1 := unpack(t, row, 2)[0]
			inlineEqual(t, "td1.width", td1.Width, c.width)
		})
	}
}

// TestReplacedBoxLayout tests replacedbox_layout, which the drawing of
// upstream calls: the size and the position of an image in its box, for
// object-fit and object-position. The test is not in upstream.
func TestReplacedBoxLayout(t *testing.T) {
	for _, c := range []struct {
		css                 string
		width, height, x, y float64
	}{
		{"object-fit: fill", 40, 20, 0, 0},
		{"object-fit: contain", 20, 20, 10, 0},
		{"object-fit: cover", 40, 40, 0, -10},
		{"object-fit: none", 4, 4, 18, 8},
		{"object-fit: scale-down", 4, 4, 18, 8},
		{"object-fit: contain; object-position: right top", 20, 20, 20, 0},
		{"object-fit: none; object-position: 1px 2px", 4, 4, 1, 2},
	} {
		t.Run(c.css, func(t *testing.T) {
			body := inlineBody(t, `<body style="font-size: 0">
        <img src="pattern.png" style="width: 40px; height: 20px; `+c.css+`">`)
			img := only(t, only(t, body))
			width, height, x, y := replacedBoxLayout(img)
			inlineEqual(t, "draw_width", width, c.width)
			inlineEqual(t, "draw_height", height, c.height)
			inlineEqual(t, "position_x", x, img.ContentBoxX()+c.x)
			inlineEqual(t, "position_y", y, img.ContentBoxY()+c.y)
		})
	}
}
