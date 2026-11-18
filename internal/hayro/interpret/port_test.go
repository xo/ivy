// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file holds the tests of the port that upstream does not have: the
// interpretation of each page of the test files of the ports, and the fuzz
// test.

package interpret

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// recorder is a Device that counts the calls, and that runs the content of
// glyphs of Type 3, tiling patterns and soft masks, and decodes the images
// and samples the shadings, as a renderer does.
type recorder struct {
	t testing.TB

	paths, rects, glyphRuns, glyphs, images int
	clips, groups, marked                   int
	clipDepth, groupDepth, markedDepth      int
	textures, gradients, decoded            int
	meshTriangles                           int

	// shadings holds the shading patterns that the recorder painted with.
	shadings []*ShadingPattern

	// depth counts the nested calls of the recorder, so that the test
	// stops a recursion that the interpreter does not stop.
	depth int
}

const maxRecorderDepth = 200

func (r *recorder) enter() bool {
	if r.depth >= maxRecorderDepth {
		r.t.Errorf("the recursion of the device reaches %d", r.depth)
		return false
	}
	r.depth++
	return true
}

func (r *recorder) leave() { r.depth-- }

func (r *recorder) paint(p Paint, isStroke bool) {
	switch v := p.(type) {
	case *TilingPattern:
		if r.enter() {
			v.Interpret(r, v.Matrix, isStroke)
			r.leave()
		}
	case *ShadingPattern:
		r.shadings = append(r.shadings, v)
		encoded := v.Encode()
		if ra, ok := encoded.ShadingType.(*EncodedRadialAxialShading); ok {
			if g, ok := ra.AsSVGGradient(encoded, 0.01); ok && len(g.Stops) > 0 {
				r.gradients++
			}
		}
		tex := encoded.RenderTexture(kurbo.NewRect(0, 0, 24, 24), 1)
		if len(tex.Data) != int(tex.Width)*int(tex.Height)*4 {
			r.t.Errorf("texture of %dx%d holds %d bytes", tex.Width, tex.Height, len(tex.Data))
		}
		r.textures++
		encoded.MeshTriangles(kurbo.NewRect(0, 0, 2000, 2000), func(t MeshTriangle) {
			for _, c := range t.Colors {
				for _, v := range c {
					if !(v >= -1e-3 && v <= 1+1e-3) {
						r.t.Errorf("a triangle of a mesh has the color %v", c)
						return
					}
				}
			}
			r.meshTriangles++
		})
	case Color:
		_ = v.ToRGBA()
	}
}

func (r *recorder) mask(m *SoftMask) {
	if m == nil || !r.enter() {
		return
	}
	defer r.leave()
	_ = m.BackgroundColor().ToRGBA()
	_ = m.CacheKey()
	m.Interpret(r)
}

func (r *recorder) DrawPath(path *kurboext.BezPath, props DrawProps, mode DrawMode) {
	r.paths++
	_ = path.BoundingBox()
	_, isStroke := mode.(DrawModeStroke)
	r.paint(props.Paint, isStroke)
	r.mask(props.SoftMask)
}

func (r *recorder) DrawRect(_ kurbo.Rect, props DrawProps, mode DrawMode) {
	r.rects++
	_, isStroke := mode.(DrawModeStroke)
	r.paint(props.Paint, isStroke)
	r.mask(props.SoftMask)
}

func (r *recorder) PushClipPath(clip *ClipPath) {
	r.clips++
	r.clipDepth++
	_ = clip.CacheKey()
}

func (r *recorder) PushClipRect(kurbo.Rect) {
	r.clips++
	r.clipDepth++
}

func (r *recorder) PopClip() {
	r.clipDepth--
	if r.clipDepth < 0 {
		r.t.Error("PopClip without PushClipPath")
	}
}

func (r *recorder) PushTransparencyGroup(_ float32, mask *SoftMask, _ BlendMode, _, _ bool) {
	r.groups++
	r.groupDepth++
	r.mask(mask)
}

func (r *recorder) PopTransparencyGroup() {
	r.groupDepth--
	if r.groupDepth < 0 {
		r.t.Error("PopTransparencyGroup without PushTransparencyGroup")
	}
}

func (r *recorder) DrawGlyphRun(run *GlyphRun, props DrawProps, mode DrawMode) {
	r.glyphRuns++
	_, isStroke := mode.(DrawModeStroke)
	for i := range run.Glyphs() {
		g := &run.Glyphs()[i]
		r.glyphs++
		_, _ = g.AsUnicode()
		switch v := g.Glyph.(type) {
		case *OutlineGlyph:
			_ = kurboext.MulBezPath(g.Transform(), v.Outline())
		case *Type3Glyph:
			if r.enter() {
				v.Interpret(r, props.Transform, g.Transform(), props.Paint)
				r.leave()
			}
		}
	}
	r.paint(props.Paint, isStroke)
	r.mask(props.SoftMask)
}

func (r *recorder) DrawImage(image Image, props ImageDrawProps) {
	r.images++
	switch v := image.(type) {
	case *RasterImage:
		v.WithRGBA(func(data ImageData, alpha *LumaData) {
			r.decoded++
			w, h := data.Width(), data.Height()
			if data.RGB != nil && len(data.RGB.Data) != int(w)*int(h)*3 {
				r.t.Errorf("image of %dx%d holds %d bytes of RGB", w, h, len(data.RGB.Data))
			}
			if data.Luma != nil && len(data.Luma.Data) != int(w)*int(h) {
				r.t.Errorf("image of %dx%d holds %d bytes of gray", w, h, len(data.Luma.Data))
			}
			if alpha != nil && len(alpha.Data) != int(alpha.Width)*int(alpha.Height) {
				r.t.Errorf("alpha of %dx%d holds %d bytes", alpha.Width, alpha.Height, len(alpha.Data))
			}
		}, [2]uint32{})
	case *StencilImage:
		v.WithStencil(func(data LumaData, p Paint) {
			r.decoded++
			if len(data.Data) != int(data.Width)*int(data.Height) {
				r.t.Errorf("stencil of %dx%d holds %d bytes", data.Width, data.Height, len(data.Data))
			}
			r.paint(p, false)
		}, [2]uint32{})
	}
	r.mask(props.SoftMask)
}

func (r *recorder) BeginMarkedContent(syntax.Name, *syntax.Dict) {
	r.marked++
	r.markedDepth++
}

func (r *recorder) EndMarkedContent() {
	r.markedDepth--
}

// testFiles returns the PDF files of the testdata folders of the ports.
func testFiles(t testing.TB) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"testdata", "../syntax/testdata", "../pdffont/testdata"} {
		matches, err := filepath.Glob(filepath.Join(dir, "*.pdf"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		t.Fatal("no test files")
	}
	return files
}

// interpretPages interprets each page of a PDF file into the recorder, up
// to max pages.
func interpretPages(data []byte, rec *recorder, settings InterpreterSettings, maxPages int) ([]error, error) {
	pdf, err := syntax.NewPdfWithOptions(data, syntax.Options{Limits: settings.Limits, Decoders: *settings.Decoders})
	if err != nil {
		return nil, err
	}
	cache := NewInterpreterCache()
	var errs []error
	for i, page := range pdf.Pages().Slice() {
		if i >= maxPages {
			break
		}
		w, h := page.RenderDimensions()
		transform := TransformToKurbo(page.InitialTransform(true))
		ctx := NewContext(transform, kurbo.NewRect(0, 0, float64(w), float64(h)), cache, pdf.XRef(), settings)
		errs = append(errs, InterpretPage(page, ctx, rec))
	}
	return errs, nil
}

// TestInterpretPages interprets each page of the test files of the ports,
// and checks that no page fails, that the clips and the groups balance, and
// that the decoded images have their sizes.
func TestInterpretPages(t *testing.T) {
	t.Parallel()
	for _, file := range testFiles(t) {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			rec := &recorder{t: t}
			var warnings []InterpreterWarning
			settings := DefaultInterpreterSettings()
			settings.WarningSink = func(w InterpreterWarning) { warnings = append(warnings, w) }
			errs, err := interpretPages(data, rec, settings, 1000)
			if err != nil {
				t.Skipf("cannot open: %v", err)
			}
			if len(errs) == 0 {
				t.Skip("no pages")
			}
			for i, err := range errs {
				if err != nil {
					t.Errorf("page %d: %v", i, err)
				}
			}
			if rec.clipDepth != 0 || rec.groupDepth != 0 || rec.markedDepth != 0 {
				t.Errorf("unbalanced: %d clips, %d groups, %d marked contents", rec.clipDepth, rec.groupDepth, rec.markedDepth)
			}
			t.Logf("%d pages: %d paths, %d rects, %d runs of %d glyphs, %d images (%d decoded), %d clips, %d groups, %d textures, %d gradients, warnings %v",
				len(errs), rec.paths, rec.rects, rec.glyphRuns, rec.glyphs, rec.images, rec.decoded, rec.clips, rec.groups, rec.textures, rec.gradients, warnings)
			if rec.paths+rec.rects+rec.glyphRuns+rec.images == 0 && !strings.Contains(file, "version_") {
				t.Log("the file draws nothing")
			}
		})
	}
}

// TestInterpretKinds checks that the test files reach each kind of mark.
func TestInterpretKinds(t *testing.T) {
	rec := &recorder{t: t}
	for _, file := range testFiles(t) {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := interpretPages(data, rec, DefaultInterpreterSettings(), 1000); err != nil {
			continue
		}
	}
	for name, n := range map[string]int{
		"paths": rec.paths, "rects": rec.rects, "glyph runs": rec.glyphRuns, "images": rec.images,
		"clips": rec.clips, "groups": rec.groups, "textures": rec.textures, "gradients": rec.gradients,
		"decoded images": rec.decoded, "marked contents": rec.marked, "mesh triangles": rec.meshTriangles,
	} {
		if n == 0 {
			t.Errorf("no %s", name)
		}
	}
}

// TestCancel stops an interpretation with the Cancel function.
func TestCancel(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "text_rendering_5.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	settings := DefaultInterpreterSettings()
	settings.Cancel = func() bool { return true }
	errs, err := interpretPages(data, &recorder{t: t}, settings, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 1 || !errors.Is(errs[0], ErrCanceled) {
		t.Errorf("got %v, want ErrCanceled", errs)
	}
}

// TestOperatorLimit stops an interpretation past the limit of operators.
func TestOperatorLimit(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "text_rendering_5.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	settings := DefaultInterpreterSettings()
	settings.Limits = ivy.Limits{MaxObjects: 1}
	errs, err := interpretPages(data, &recorder{t: t}, settings, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 1 || !errors.Is(errs[0], ivy.ErrLimit) {
		t.Errorf("got %v, want ivy.ErrLimit", errs)
	}
}

// selfPdf returns a PDF file with one page whose content is content, and
// the objects numbered from 10.
func selfPdf(content string, objects []string) []byte {
	var b strings.Builder
	b.WriteString("%PDF-1.7\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	b.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources 5 0 R /Contents 4 0 R >>\nendobj\n")
	b.WriteString("4 0 obj\n<< /Length " + itoa(len(content)) + " >>\nstream\n" + content + "\nendstream\nendobj\n")
	for i, o := range objects {
		b.WriteString(itoa(5+i) + " 0 obj\n" + o + "\nendobj\n")
	}
	b.WriteString("trailer\n<< /Root 1 0 R >>\n%%EOF\n")
	return []byte(b.String())
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}

// TestRecursion interprets a tiling pattern that paints with itself, a
// glyph of Type 3 that shows itself, and a form that draws itself. Each one
// must end.
func TestRecursion(t *testing.T) {
	pattern := "<< /Type /Pattern /PatternType 1 /PaintType 1 /TilingType 1 /BBox [0 0 10 10] /XStep 10 /YStep 10 " +
		"/Resources << /Pattern << /P 6 0 R >> >> /Length 31 >>\nstream\n/Pattern cs /P scn 0 0 5 5 re f\nendstream"
	font := "<< /Type /Font /Subtype /Type3 /FontBBox [0 0 1 1] /FontMatrix [0.001 0 0 0.001 0 0] " +
		"/CharProcs << /a 8 0 R >> /Encoding << /Differences [65 /a] >> /FirstChar 65 /LastChar 65 /Widths [1000] " +
		"/Resources << /Font << /F 7 0 R >> >> >>"
	charProc := "<< /Length 41 >>\nstream\n1000 0 d0 BT /F 1000 Tf (A) Tj ET 0 0 m\nendstream"
	form := "<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] /Resources << /XObject << /X 9 0 R >> >> /Length 6 >>\nstream\n/X Do \nendstream"
	resources := "<< /Pattern << /P 6 0 R >> /Font << /F 7 0 R >> /XObject << /X 9 0 R >> >>"
	content := "/Pattern cs /P scn 0 0 50 50 re f BT /F 12 Tf (A) Tj ET /X Do"
	data := selfPdf(content, []string{resources, pattern, font, charProc, form})
	rec := &recorder{t: t}
	errs, err := interpretPages(data, rec, DefaultInterpreterSettings(), 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if rec.clipDepth != 0 || rec.groupDepth != 0 {
		t.Errorf("unbalanced: %d clips, %d groups", rec.clipDepth, rec.groupDepth)
	}
}

// FuzzInterpret interprets the pages of a malformed PDF file. It fails on
// a panic, which the interpreter turns into a PanicError.
func FuzzInterpret(f *testing.F) {
	for _, file := range testFiles(f) {
		data, err := os.ReadFile(file)
		if err != nil {
			f.Fatal(err)
		}
		if len(data) <= 64<<10 {
			f.Add(data)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		settings := DefaultInterpreterSettings()
		settings.Limits = ivy.Limits{MaxMemory: 1 << 24, MaxPixels: 1 << 20, MaxObjects: 1 << 12}
		decoders := ImageDecoders(nil, settings.Limits)
		settings.Decoders = &decoders
		errs, err := interpretPages(data, &recorder{t: t}, settings, 4)
		if err != nil {
			return
		}
		for _, err := range errs {
			var pe *PanicError
			if errors.As(err, &pe) {
				t.Fatalf("%v\n%s", err, pe.Stack)
			}
		}
	})
}

// TestExponentialFunctions reads a function of type 3 whose array names
// the next level twice at each of 30 levels. The cap of the functions stops
// it.
func TestExponentialFunctions(t *testing.T) {
	const levels = 30
	bodies := make([]string, levels)
	for i := range levels {
		if i+1 == levels {
			bodies[i] = "<< /FunctionType 2 /Domain [0 1] /C0 [0] /C1 [1] /N 1 >>"
		} else {
			next := itoa(10 + i + 1)
			bodies[i] = "<< /FunctionType 3 /Domain [0 1] /Functions [" + next + " 0 R " + next + " 0 R] /Bounds [0.5] /Encode [0 1 0 1] >>"
		}
	}
	pdf := minimalPdf(t, bodies)
	obj, ok := pdf.XRef().Get[syntax.Object](syntax.NewObjectIdentifier(10, 0))
	if !ok {
		t.Fatal("no object 10")
	}
	if _, ok := NewFunction(obj); ok {
		t.Error("a function of 2^30 functions succeeds")
	}
}

// TestHugeImage draws an image whose dictionary states 10^10 pixels, with
// a few bytes of data. The limit of the pixels stops its decoding.
func TestHugeImage(t *testing.T) {
	image := "<< /Type /XObject /Subtype /Image /Width 100000 /Height 100000 /BitsPerComponent 3 " +
		"/ColorSpace /DeviceGray /Length 4 >>\nstream\nabcd\nendstream"
	data := selfPdf("q 100 0 0 100 0 0 cm /I Do Q", []string{"<< /XObject << /I 6 0 R >> >>", image})
	rec := &recorder{t: t}
	var warnings []InterpreterWarning
	settings := DefaultInterpreterSettings()
	settings.WarningSink = func(w InterpreterWarning) { warnings = append(warnings, w) }
	errs, err := interpretPages(data, rec, settings, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 1 || errs[0] != nil {
		t.Errorf("got %v", errs)
	}
	if rec.images != 1 || rec.decoded != 0 || len(warnings) != 1 || warnings[0] != InterpreterWarningImageDecodeFailure {
		t.Errorf("%d images, %d decoded, warnings %v", rec.images, rec.decoded, warnings)
	}
}
