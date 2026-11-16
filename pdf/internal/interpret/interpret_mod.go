// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/interpret/mod.rs.

package interpret

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy"
	"github.com/xo/ivy/pdf/internal/cmap"
	"github.com/xo/ivy/pdf/internal/kurboext"
	"github.com/xo/ivy/pdf/internal/pdffont"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// The flags of annotations in hayro-interpret src/interpret/mod.rs.
const (
	annotationHidden = 1 << 1
	annotationNoView = 1 << 5
)

// The types of hayro-font that hayro-interpret src/interpret/mod.rs
// exports.
type (
	// FontResolverFn is FontResolverFn of hayro-font.
	FontResolverFn = pdffont.FontResolverFn
	// CMapResolverFn is CMapResolverFn of hayro-font.
	CMapResolverFn = pdffont.CMapResolverFn
)

// WarningSinkFn is WarningSinkFn in hayro-interpret src/interpret/mod.rs. It
// receives the warnings of an interpretation.
type WarningSinkFn func(InterpreterWarning)

// InterpreterSettings is InterpreterSettings in hayro-interpret
// src/interpret/mod.rs. It holds the settings of an interpretation.
type InterpreterSettings struct {
	// FontResolver resolves the fonts that the PDF file does not embed,
	// including the 14 standard fonts.
	FontResolver FontResolverFn
	// CMapResolver resolves the CMaps that the PDF file does not embed.
	CMapResolver CMapResolverFn
	// WarningSink receives the warnings. A nil WarningSink drops them.
	WarningSink WarningSinkFn
	// RenderContent selects the parts of a page to interpret.
	RenderContent RenderContent

	// Cancel stops the interpretation when it returns true. The
	// interpreter calls it often, and it must be fast. A caller that holds
	// a context.Context can pass a function that returns ctx.Err() != nil.
	// A nil Cancel never stops.
	//
	// Port: upstream cannot stop an interpretation.
	Cancel func() bool
	// Limits bound the work. MaxObjects bounds the operators of a page, 64
	// for each object. A zero field takes the default of ivy.
	//
	// Port: upstream has no limits.
	Limits ivy.Limits
	// Decoders holds the decoders of JBIG2 and JPX that the images of the
	// page use. A nil Decoders takes the decoders of the options of the
	// document in syntax.
	//
	// Port: upstream decodes the images with its own crates.
	Decoders *syntax.ImageDecoders
}

// warn sends a warning to the sink.
func (s *InterpreterSettings) warn(w InterpreterWarning) {
	if s.WarningSink != nil {
		s.WarningSink(w)
	}
}

// DefaultInterpreterSettings is the Default implementation of
// InterpreterSettings in hayro-interpret src/interpret/mod.rs, with the
// default features embed-fonts and embed-cmaps. It resolves the standard
// fonts to the substitutes of pdffont, the predefined CMaps to those of
// cmap, and the decoders of JBIG2 and JPX to ImageDecoders.
func DefaultInterpreterSettings() InterpreterSettings {
	decoders := ImageDecoders(nil, ivy.Limits{})
	return InterpreterSettings{
		FontResolver:  DefaultFontResolver,
		CMapResolver:  cmap.LoadEmbedded,
		RenderContent: RenderContentAll,
		Decoders:      &decoders,
	}
}

// DefaultFontResolver is the font resolver of the default settings in
// hayro-interpret src/interpret/mod.rs. It returns the substitute of a
// standard font, and for any other font the substitute of the standard font
// that is closest to it. It has no font for a Named query.
func DefaultFontResolver(query *pdffont.FontQuery) (pdffont.FontData, uint32, bool) {
	if query.Named {
		return nil, 0, false
	}
	if query.IsStandard {
		data, index := query.Standard.GetFontData()
		return data, index, true
	}
	data, index := query.Fallback.PickStandardFont().GetFontData()
	return data, index, true
}

// RenderContent is RenderContent in hayro-interpret src/interpret/mod.rs. It
// is a set of flags for the parts of a page to interpret.
type RenderContent uint8

// The flags of RenderContent.
const (
	// RenderContentPage is RenderContent::PAGE: the content stream of the
	// page.
	RenderContentPage RenderContent = 1 << 0
	// RenderContentAnnotations is RenderContent::ANNOTATIONS: the
	// annotations of the page.
	RenderContentAnnotations RenderContent = 1 << 1
	// RenderContentAll is RenderContent::all: every part of the page.
	RenderContentAll = RenderContentPage | RenderContentAnnotations
)

// Contains is RenderContent::contains. It reports whether c holds each
// flag of other.
func (c RenderContent) Contains(other RenderContent) bool {
	return c&other == other
}

// InterpreterWarning is InterpreterWarning in hayro-interpret
// src/interpret/mod.rs. It is a warning of an interpretation.
type InterpreterWarning uint8

// The warnings of an interpretation.
const (
	// InterpreterWarningUnsupportedFont is
	// InterpreterWarning::UnsupportedFont: a font cannot be loaded.
	InterpreterWarningUnsupportedFont InterpreterWarning = iota
	// InterpreterWarningImageDecodeFailure is
	// InterpreterWarning::ImageDecodeFailure: an image failed to decode.
	InterpreterWarningImageDecodeFailure
	// InterpreterWarningUnresolvedAnnotationAppearance is
	// InterpreterWarning::UnresolvedAnnotationAppearance: the normal
	// appearance of an annotation cannot be loaded.
	InterpreterWarningUnresolvedAnnotationAppearance
)

// String returns the Debug text of upstream.
func (w InterpreterWarning) String() string {
	switch w {
	case InterpreterWarningUnsupportedFont:
		return "UnsupportedFont"
	case InterpreterWarningImageDecodeFailure:
		return "ImageDecodeFailure"
	case InterpreterWarningUnresolvedAnnotationAppearance:
		return "UnresolvedAnnotationAppearance"
	}
	return fmt.Sprintf("InterpreterWarning(%d)", uint8(w))
}

// PanicError is the error of a panic of the interpreter. It wraps
// ivy.ErrFormat. A fuzz test fails on it, so that the cause is fixed.
//
// Port: upstream relies on its own checks to never panic (D18).
type PanicError struct {
	// Value is the value of the panic.
	Value any
	// Stack is the stack of the goroutine at the panic.
	Stack []byte
}

// Error returns the message of the error.
func (e *PanicError) Error() string {
	return fmt.Sprintf("interpreting pdf: internal error: %v", e.Value)
}

// Unwrap returns ivy.ErrFormat.
func (e *PanicError) Unwrap() error {
	return ivy.ErrFormat
}

// recoverInto turns a runtime panic into a PanicError in the state of the
// run. Another panic goes on.
func recoverInto(run *runState) {
	r := recover()
	if r == nil {
		return
	}
	if _, ok := r.(runtime.Error); !ok || run == nil {
		panic(r)
	}
	if run.err == nil {
		run.err = &PanicError{Value: r, Stack: debug.Stack()}
	}
}

// InterpretPage is interpret_page in hayro-interpret src/interpret/mod.rs.
// It interprets the page and sends its marks to the device. It returns
// ctx.Err().
//
// Port: upstream returns nothing.
//
//nolint:revive // The name is interpret_page of upstream.
func InterpretPage(page *syntax.Page, ctx *Context, device Device) error {
	defer recoverInto(ctx.run)
	resources := page.Resources()
	renderContent := ctx.settings.RenderContent
	if renderContent.Contains(RenderContentPage) {
		Interpret(page.TypedOperations(), resources, ctx, device)
	}
	if renderContent.Contains(RenderContentAnnotations) {
		if annots, ok := page.Raw().Get[syntax.Array](syntax.KeyAnnots); ok {
			for annot := range annots.Iter[syntax.Dict]().All() {
				if !ctx.run.step(1) {
					break
				}
				interpretAnnotation(annot, resources, ctx, device)
			}
		}
	}
	return ctx.Err()
}

// interpretAnnotation is interpret_annotation in hayro-interpret
// src/interpret/mod.rs. It draws the normal appearance of an annotation
// into its rectangle.
func interpretAnnotation(annot syntax.Dict, resources *syntax.Resources, ctx *Context, device Device) {
	flags, _ := annot.Get[uint32](syntax.KeyF)
	if flags&(annotationHidden|annotationNoView) != 0 {
		return
	}
	var apx *formXObject
	if s, ok := appearanceStream(annot); ok {
		apx, _ = newFormXObject(s)
	}
	if apx == nil {
		if _, ok := annot.Get[syntax.Dict](syntax.KeyAP); ok {
			ctx.settings.warn(InterpreterWarningUnresolvedAnnotationAppearance)
		}
		return
	}
	rect, ok := annot.Get[syntax.Rect](syntax.KeyRect)
	if !ok {
		return
	}
	annotRect := RectToKurbo(rect)
	bboxPath := kurboext.RectToPath(kurbo.NewRect(float64(apx.bbox[0]), float64(apx.bbox[1]), float64(apx.bbox[2]), float64(apx.bbox[3])), 0.1)
	transformed := kurboext.MulBezPath(apx.matrix, bboxPath)
	transformedRect := transformed.BoundingBox()
	sx := annotRect.Width() / transformedRect.Width()
	sy := annotRect.Height() / transformedRect.Height()
	affine := kurbo.NewAffine([6]float64{
		sx, 0, 0, sy, annotRect.X0 - sx*transformedRect.X0, annotRect.Y0 - sy*transformedRect.Y0,
	})
	ctx.saveState()
	ctx.preConcatAffine(affine)
	ctx.pushRootTransform()
	apx.draw(resources, ctx, device)
	ctx.popRootTransform()
	ctx.restoreState(device)
}

// appearanceStream is appearance_stream in hayro-interpret
// src/interpret/mod.rs. It returns the normal appearance of an annotation,
// in the state AS, or in the state Off.
func appearanceStream(annot syntax.Dict) (syntax.Stream, bool) {
	ap, ok := annot.Get[syntax.Dict](syntax.KeyAP)
	if !ok {
		return syntax.Stream{}, false
	}
	n, ok := ap.Get[syntax.Object](syntax.KeyN)
	if !ok {
		return syntax.Stream{}, false
	}
	switch o := n.(type) {
	case syntax.Stream:
		return o, true
	case syntax.Dict:
		if state, ok := annot.Get[syntax.Name](syntax.KeyAS); ok {
			if s, ok := o.Get[syntax.Stream](state); ok {
				return s, true
			}
		}
		return o.Get[syntax.Stream]("Off")
	}
	return syntax.Stream{}, false
}

// Interpret is interpret in hayro-interpret src/interpret/mod.rs. It runs
// the operators of ops with the resources, and sends the marks to the
// device. It stops early when ctx.Err() is not nil.
func Interpret(ops *syntax.TypedIter, resources *syntax.Resources, ctx *Context, device Device) {
	if resources == nil {
		// Port: upstream takes a reference, which is never null.
		resources = syntax.NewResources(syntax.EmptyDict())
	}
	numStates := ctx.numStates()
	fontDictCache := make(map[syntax.Name]syntax.Dict)

	ctx.saveState()

	for ctx.run.step(1) {
		op, ok := ops.Next()
		if !ok {
			break
		}
		interpretOp(op, resources, ctx, device, numStates, fontDictCache)
	}

	// A pending path, with W or W*, cannot continue past its content
	// stream.
	ctx.path.Truncate(0)
	ctx.hasClip = false

	for ctx.numStates() > numStates {
		ctx.restoreState(device)
	}
}

// setStrokeColor sets the color of the stroke in a color space of the
// device.
func setStrokeColor(ctx *Context, cs ColorSpace, nums ...syntax.Number) {
	p := ctx.strokePaintMut()
	p.colorSpace = cs
	p.color = numbersToF32(nums)
	p.pattern = nil
}

// setFillColor sets the color of the fill in a color space of the device.
func setFillColor(ctx *Context, cs ColorSpace, nums ...syntax.Number) {
	p := ctx.fillPaintMut()
	p.colorSpace = cs
	p.color = numbersToF32(nums)
	p.pattern = nil
}

// numbersToF32 converts numbers into components.
func numbersToF32(nums []syntax.Number) ColorComponents {
	out := make(ColorComponents, len(nums))
	for i, n := range nums {
		out[i] = n.AsF32()
	}
	return out
}

// interpretOp runs one operator. It is the body of the loop of interpret in
// hayro-interpret src/interpret/mod.rs.
func interpretOp(op syntax.TypedInstruction, resources *syntax.Resources, ctx *Context, device Device, numStates int, fontDictCache map[syntax.Name]syntax.Dict) {
	switch o := op.(type) {
	case syntax.SaveState:
		ctx.saveState()
	case syntax.StrokeColorDeviceRgb:
		setStrokeColor(ctx, DeviceRGB(), o.V0, o.V1, o.V2)
	case syntax.StrokeColorDeviceGray:
		setStrokeColor(ctx, DeviceGray(), o.V0)
	case syntax.StrokeColorCmyk:
		setStrokeColor(ctx, DeviceCMYK(), o.V0, o.V1, o.V2, o.V3)
	case syntax.LineWidth:
		ctx.strokePropsMut().LineWidth = o.V0.AsF32()
	case syntax.LineCap:
		ctx.strokePropsMut().LineCap = convertLineCap(o)
	case syntax.LineJoin:
		ctx.strokePropsMut().LineJoin = convertLineJoin(o)
	case syntax.MiterLimit:
		ctx.strokePropsMut().MiterLimit = o.V0.AsF32()
	case syntax.TransformOps:
		ctx.preConcatTransform(o)
	case syntax.RectPath:
		x, y := o.V0.AsF64(), o.V1.AsF64()
		rect := kurbo.NewRect(x, y, x+o.V2.AsF64(), y+o.V3.AsF64())
		ctx.path.Extend(rect.PathElements(0.1))
		p := kurbo.NewPoint(x, y)
		ctx.lastPoint, ctx.subPathStart = p, p
	case syntax.MoveTo:
		p := kurbo.NewPoint(o.V0.AsF64(), o.V1.AsF64())
		ctx.lastPoint, ctx.subPathStart = p, p
		ctx.path.MoveTo(p)
	case syntax.FillPathEvenOdd:
		fillPath(ctx, device, FillRuleEvenOdd)
	case syntax.FillPathNonZero, syntax.FillPathNonZeroCompatibility:
		fillPath(ctx, device, FillRuleNonZero)
	case syntax.FillAndStrokeEvenOdd:
		fillStrokePath(ctx, device, FillRuleEvenOdd)
	case syntax.FillAndStrokeNonZero:
		fillStrokePath(ctx, device, FillRuleNonZero)
	case syntax.CloseAndStrokePath:
		closePath(ctx)
		strokePath(ctx, device)
	case syntax.CloseFillAndStrokeEvenOdd:
		closePath(ctx)
		fillStrokePath(ctx, device, FillRuleEvenOdd)
	case syntax.CloseFillAndStrokeNonZero:
		closePath(ctx)
		fillStrokePath(ctx, device, FillRuleNonZero)
	case syntax.NonStrokeColorDeviceGray:
		setFillColor(ctx, DeviceGray(), o.V0)
	case syntax.NonStrokeColorDeviceRgb:
		setFillColor(ctx, DeviceRGB(), o.V0, o.V1, o.V2)
	case syntax.NonStrokeColorCmyk:
		setFillColor(ctx, DeviceCMYK(), o.V0, o.V1, o.V2, o.V3)
	case syntax.LineTo:
		if ctx.path.Len() != 0 {
			last := ctx.lastPoint
			p := kurbo.NewPoint(o.V0.AsF64(), o.V1.AsF64())
			ctx.lastPoint = p
			if last == p {
				// A small delta lets a line of zero length have a round
				// stroke.
				p.X += 0.0001
			}
			ctx.path.LineTo(p)
		}
	case syntax.CubicTo:
		if ctx.path.Len() != 0 {
			p1 := kurbo.NewPoint(o.V0.AsF64(), o.V1.AsF64())
			p2 := kurbo.NewPoint(o.V2.AsF64(), o.V3.AsF64())
			p3 := kurbo.NewPoint(o.V4.AsF64(), o.V5.AsF64())
			ctx.lastPoint = p3
			ctx.path.CurveTo(p1, p2, p3)
		}
	case syntax.CubicStartTo:
		if ctx.path.Len() != 0 {
			p1 := ctx.lastPoint
			p2 := kurbo.NewPoint(o.V0.AsF64(), o.V1.AsF64())
			p3 := kurbo.NewPoint(o.V2.AsF64(), o.V3.AsF64())
			ctx.lastPoint = p3
			ctx.path.CurveTo(p1, p2, p3)
		}
	case syntax.CubicEndTo:
		if ctx.path.Len() != 0 {
			p2 := kurbo.NewPoint(o.V0.AsF64(), o.V1.AsF64())
			p3 := kurbo.NewPoint(o.V2.AsF64(), o.V3.AsF64())
			ctx.lastPoint = p3
			ctx.path.CurveTo(p2, p3, p3)
		}
	case syntax.ClosePath:
		closePath(ctx)
	case syntax.SetGraphicsState:
		if gs, ok := resources.GetExtGState(o.V0); ok {
			handleGs(gs, ctx, resources)
		}
	case syntax.StrokePath:
		strokePath(ctx, device)
	case syntax.EndPath:
		endPath(ctx, device)
	case syntax.NonStrokeColor:
		p := ctx.fillPaintMut()
		p.color = numbersToF32(o.V0)
		p.pattern = nil
	case syntax.StrokeColor:
		p := ctx.strokePaintMut()
		p.color = numbersToF32(o.V0)
		p.pattern = nil
	case syntax.ClipNonZero:
		ctx.clip, ctx.hasClip = FillRuleNonZero, true
	case syntax.ClipEvenOdd:
		ctx.clip, ctx.hasClip = FillRuleEvenOdd, true
	case syntax.RestoreState:
		// A Q without its q must not restore the state that was saved on
		// entry to this stream.
		if ctx.numStates() > numStates+1 {
			ctx.restoreState(device)
		}
	case syntax.ColorSpaceStroke:
		cs := colorSpaceOf(ctx, resources, o.V0)
		p := ctx.strokePaintMut()
		if !cs.isPattern() {
			p.pattern = nil
		}
		p.color = cs.initialColor()
		p.colorSpace = cs
	case syntax.ColorSpaceNonStroke:
		cs := colorSpaceOf(ctx, resources, o.V0)
		p := ctx.fillPaintMut()
		if !cs.isPattern() {
			p.pattern = nil
		}
		p.color = cs.initialColor()
		p.colorSpace = cs
	case syntax.DashPattern:
		props := ctx.strokePropsMut()
		props.DashOffset = o.V1.AsF32()
		// kurbo cannot deal with a dash of exactly 0, as upstream notes.
		props.DashArray = dashArrayOf(o.V0)
	case syntax.NonStrokeColorNamed:
		ctx.fillPaintMut().color = numbersToF32(o.V0)
		pattern := namedPattern(ctx, resources, o.V1)
		ctx.fillPaintMut().pattern = pattern
	case syntax.StrokeColorNamed:
		ctx.strokePaintMut().color = numbersToF32(o.V0)
		pattern := namedPattern(ctx, resources, o.V1)
		ctx.strokePaintMut().pattern = pattern
	case syntax.BeginMarkedContentWithProperties:
		beginMarkedContentWithProperties(o, resources, ctx, device)
	case syntax.EndMarkedContent:
		ctx.ocgState.endMarkedContent()
		endMarkedContent(device)
	case syntax.BeginMarkedContent:
		ctx.ocgState.beginMarkedContent()
		beginMarkedContent(device, o.V0, nil)
	case syntax.BeginText:
		t := ctx.textStateMut()
		t.textMatrix = kurbo.IdentityAffine
		t.textLineMatrix = kurbo.IdentityAffine
	case syntax.SetTextMatrix:
		m := kurbo.NewAffine([6]float64{o.V0.AsF64(), o.V1.AsF64(), o.V2.AsF64(), o.V3.AsF64(), o.V4.AsF64(), o.V5.AsF64()})
		t := ctx.textStateMut()
		t.textLineMatrix = m
		t.textMatrix = m
	case syntax.EndText:
		if ctx.textState().clipPaths.HasSegments() {
			clipPath := kurboext.MulBezPath(ctx.ctm(), ctx.textState().clipPaths)
			ctx.pushClipPath(clipPath, FillRuleNonZero, device)
		}
		ctx.textStateMut().clipPaths.Truncate(0)
	case syntax.TextFont:
		var font *textStateFont
		if fontDict, ok := fontDictCache[o.V0]; ok {
			font = ctx.resolveFont(fontDict)
		} else if fontDict, ok := resources.GetFont(o.V0); ok {
			fontDictCache[o.V0] = fontDict
			font = ctx.resolveFont(fontDict)
		} else {
			font = ctx.fallbackFont()
		}
		t := ctx.textStateMut()
		t.fontSize = o.V1.AsF32()
		t.font = font
	case syntax.ShowText:
		ensureFont(ctx)
		showTextString(ctx, device, resources, o.V0)
	case syntax.ShowTexts:
		ensureFont(ctx)
		beginGlyphRun(ctx)
		for obj := range o.V0.Iter[syntax.Object]().All() {
			switch v := obj.(type) {
			case syntax.Number:
				applyGlyphRunAdjustment(ctx, v.AsF32())
			case syntax.String:
				appendTextString(ctx, resources, v)
			}
		}
		showGlyphRun(ctx, device)
	case syntax.HorizontalScaling:
		ctx.textStateMut().horizontalScaling = o.V0.AsF32()
	case syntax.TextLeading:
		ctx.textStateMut().leading = o.V0.AsF32()
	case syntax.CharacterSpacing:
		ctx.textStateMut().charSpace = o.V0.AsF32()
	case syntax.WordSpacing:
		ctx.textStateMut().wordSpace = o.V0.AsF32()
	case syntax.NextLine:
		nextLine(ctx, o.V0.AsF64(), o.V1.AsF64())
	case syntax.NextLineUsingLeading:
		nextLine(ctx, 0, -float64(ctx.textState().leading))
	case syntax.NextLineAndShowText:
		nextLine(ctx, 0, -float64(ctx.textState().leading))
		showTextString(ctx, device, resources, o.V0)
	case syntax.TextRenderingMode:
		mode := textRenderingModeFill
		if v := o.V0.AsI64(); v >= 0 && v <= 7 {
			mode = textRenderingMode(v)
		}
		ctx.textStateMut().renderMode = mode
	case syntax.NextLineAndSetLeading:
		tx, ty := o.V0.AsF64(), o.V1.AsF64()
		ctx.textStateMut().leading = float32(-ty)
		nextLine(ctx, tx, ty)
	case syntax.XObject:
		drawXObjectOp(o.V0, resources, ctx, device)
	case syntax.InlineImage:
		transferFunction := ctx.compositing().transferFunction
		resolveCS := func(name syntax.Name) (ColorSpace, bool) { return ctx.getColorSpace(resources, name) }
		if x, ok := newImageXObject(o.V0, resolveCS, &ctx.settings, ctx.interpreterCache.objectCache, transferFunction); ok {
			x.draw(ctx, device)
		}
	case syntax.TextRise:
		ctx.textStateMut().rise = o.V0.AsF32()
	case syntax.Shading:
		shadingOp(o.V0, resources, ctx, device)
	case syntax.ShowTextWithParameters:
		t := ctx.textStateMut()
		t.wordSpace = o.V0.AsF32()
		t.charSpace = o.V1.AsF32()
		nextLine(ctx, 0, -float64(ctx.textState().leading))
		showTextString(ctx, device, resources, o.V2)
	}
	// FlatnessTolerance, RenderingIntent, MarkedContentPoint,
	// MarkedContentPointWithProperties, ShapeGlyph, BeginCompatibility,
	// EndCompatibility, ColorGlyph and the other operators do nothing.
}

// colorSpaceOf reads the color space of CS or cs: a name of a device space,
// or a color space of the resources, or DeviceGray.
func colorSpaceOf(ctx *Context, resources *syntax.Resources, name syntax.Name) ColorSpace {
	if cs, ok := colorSpaceFromName(name); ok {
		return cs
	}
	if cs, ok := ctx.getColorSpace(resources, name); ok {
		return cs
	}
	return DeviceGray()
}

// namedPattern reads the pattern of SCN or scn, or returns nil.
func namedPattern(ctx *Context, resources *syntax.Resources, name *syntax.Name) Pattern {
	if name == nil {
		return nil
	}
	obj, ok := resources.GetPattern(*name)
	if !ok {
		return nil
	}
	p, ok := newPattern(obj, ctx)
	if !ok {
		return nil
	}
	return p
}

// ensureFont sets the fallback font when the text state has no font.
func ensureFont(ctx *Context) {
	if ctx.textState().font == nil {
		ctx.textStateMut().font = ctx.fallbackFont()
	}
}

// beginMarkedContentWithProperties runs BDC. It enters the optional content
// of the properties, and calls the device.
func beginMarkedContentWithProperties(o syntax.BeginMarkedContentWithProperties, resources *syntax.Resources, ctx *Context, device Device) {
	name, isName := syntax.ObjectIntoName(o.V1)
	var properties *syntax.Dict
	if isName {
		if d, ok := resources.Properties.Get[syntax.Dict](name); ok {
			properties = &d
		}
	}
	if properties == nil {
		if d, _, ok := syntax.DictOrStream(o.V1); ok {
			properties = &d
		}
	}
	var ocDict syntax.Dict
	var ocRef syntax.ObjRef
	hasOC := false
	if isName {
		if r, ok := resources.Properties.GetRef(name); ok {
			d, ok := resources.Properties.Get[syntax.Dict](name)
			if !ok {
				d = syntax.EmptyDict()
			}
			ocDict, ocRef, hasOC = d, r, true
		}
	}
	if !hasOC {
		if props, _, ok := syntax.DictOrStream(o.V1); ok {
			if r, ok := props.GetRef(syntax.KeyOC); ok {
				d, ok := props.Get[syntax.Dict](syntax.KeyOC)
				if !ok {
					d = syntax.EmptyDict()
				}
				ocDict, ocRef, hasOC = d, r, true
			}
		}
	}
	if hasOC {
		ctx.ocgState.beginOcg(ocDict, ocRef.ID())
	} else {
		ctx.ocgState.beginMarkedContent()
	}
	beginMarkedContent(device, o.V0, properties)
}

// drawXObjectOp runs Do.
func drawXObjectOp(name syntax.Name, resources *syntax.Resources, ctx *Context, device Device) {
	s, ok := resources.GetXObject(name)
	if !ok {
		return
	}
	transferFunction := ctx.compositing().transferFunction
	resolveCS := func(name syntax.Name) (ColorSpace, bool) { return ctx.getColorSpace(resources, name) }
	if x, ok := newXObject(s, resolveCS, &ctx.settings, ctx.interpreterCache.objectCache, transferFunction); ok {
		x.draw(resources, ctx, device)
	}
}

// shadingOp runs sh. It fills the current box, or the bounds of a mesh,
// with the shading.
func shadingOp(name syntax.Name, resources *syntax.Resources, ctx *Context, device Device) {
	if !ctx.ocgState.isVisible() {
		return
	}
	transferFunction := ctx.compositing().transferFunction
	obj, ok := resources.GetShading(name)
	if !ok {
		return
	}
	dict, stream, ok := syntax.DictOrStream(obj)
	if !ok {
		return
	}
	shared, ok := newShading(dict, stream, ctx.interpreterCache.objectCache)
	if !ok {
		return
	}
	shading := *shared
	shading.Background = nil
	bbox := ctx.currentBbox()
	if shading.meshBounds != nil {
		// Upstream snaps the bounds outwards, so that the sampled texture
		// stays aligned with the pixels of the device.
		bounds := ctx.ctm().TransformRectBbox(*shading.meshBounds)
		bbox = kurboext.RectIntersect(bbox, kurboext.RectInflate(kurboext.RectExpand(bounds), 1, 1))
	}
	ctx.saveState()
	ctx.pushRootTransform()
	paint := ctx.fillPaintMut()
	paint.pattern = &ShadingPattern{
		Shading:          &shading,
		Matrix:           kurbo.IdentityAffine,
		Opacity:          1,
		TransferFunction: transferFunction,
		run:              ctx.run,
	}
	paint.colorSpace = patternColorSpace()
	bboxPath := kurboext.RectToPath(bbox, 0.1)
	inverted := kurboext.MulBezPath(ctx.ctm().Inverse(), bboxPath)
	fillPathImpl(ctx, device, FillRuleNonZero, &inverted)
	ctx.popRootTransform()
	ctx.restoreState(device)
}

// glyphRunWritingMode returns the writing mode of the font of the text
// state.
func glyphRunWritingMode(ctx *Context) cmap.WritingMode {
	if ctx.textState().fontHorizontal() {
		return cmap.Horizontal
	}
	return cmap.Vertical
}
