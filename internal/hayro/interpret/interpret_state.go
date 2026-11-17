// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/interpret/state.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/pdffont"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// ActiveTransferFunction is ActiveTransferFunction in hayro-interpret
// src/interpret/state.rs. It is one transfer function for all the
// components, or four, one for each component.
//
// Port: upstream is an enum. Go sets Single, or Four.
type ActiveTransferFunction struct {
	// Single is the function of ActiveTransferFunction::Single, or nil.
	Single *TransferFunction
	// Four holds the functions of ActiveTransferFunction::Four, when Single
	// is nil.
	Four [4]*TransferFunction
}

// Apply is ActiveTransferFunction::apply in hayro-interpret
// src/interpret/state.rs. It applies the transfer function to the channels
// of RGB of a color.
func (a *ActiveTransferFunction) Apply(color AlphaColor) AlphaColor {
	rgba := color.Components()
	if a.Single != nil {
		for i := range 3 {
			rgba[i] = a.Single.applyF32(rgba[i])
		}
	} else {
		for i := range 3 {
			rgba[i] = a.Four[i].applyF32(rgba[i])
		}
	}
	return NewAlphaColor(rgba)
}

// applyTo is ActiveTransferFunction::apply_to in hayro-interpret
// src/interpret/state.rs. values holds pixels of RGB.
func (a *ActiveTransferFunction) applyTo(values []uint8) {
	if a.Single != nil {
		a.Single.ApplyTo(values)
		return
	}
	for channel := range 3 {
		if channel < len(values) {
			a.Four[channel].applyToStride(values[channel:], 3)
		}
	}
}

// clipType is ClipType in hayro-interpret src/interpret/state.rs.
type clipType uint8

// The cases of ClipType.
const (
	// clipDummy is ClipType::Dummy: a clip that the device does not see.
	clipDummy clipType = iota
	// clipReal is ClipType::Real: a clip that the device pushed.
	clipReal
)

// state is State in hayro-interpret src/interpret/state.rs. It is the
// graphics state.
type state struct {
	ctm         kurbo.Affine
	stroke      paintState
	fill        paintState
	strokeProps StrokeProps
	textState   textState
	compositing compositingState
}

// newState is State::new in hayro-interpret src/interpret/state.rs, with
// the Default of State.
func newState(initialTransform kurbo.Affine) state {
	return state{
		ctm:         initialTransform,
		stroke:      defaultPaintState(),
		fill:        defaultPaintState(),
		strokeProps: DefaultStrokeProps(),
		textState:   defaultTextState(),
	}
}

// textStateFont is TextStateFont in hayro-interpret src/interpret/state.rs.
// It is a font of the PDF file, or the fallback font when the font of the
// file failed.
type textStateFont struct {
	*pdffont.Font
	fallback bool
}

// textState is TextState in hayro-interpret src/interpret/state.rs.
type textState struct {
	charSpace float32
	wordSpace float32
	// horizontalScaling is in percent, as Tz gives it.
	horizontalScaling float32
	leading           float32
	font              *textStateFont
	fontSize          float32
	rise              float32
	renderMode        textRenderingMode

	textMatrix     kurbo.Affine
	textLineMatrix kurbo.Affine

	clipPaths kurboext.BezPath
}

// defaultTextState is the Default implementation of TextState in
// hayro-interpret src/interpret/state.rs.
func defaultTextState() textState {
	return textState{
		horizontalScaling: 100,
		// Upstream notes that the size is unspecified, and defaults it to
		// avoid an Option.
		fontSize:       1,
		textMatrix:     kurbo.IdentityAffine,
		textLineMatrix: kurbo.IdentityAffine,
	}
}

// clone is the derived Clone of TextState. The paths of the clip are
// copied, because the interpreter adds to them in place.
func (t textState) clone() textState {
	t.clipPaths = t.clipPaths.Clone()
	return t
}

// tempTransform is TextState::temp_transform in hayro-interpret
// src/interpret/state.rs.
func (t *textState) tempTransform() kurbo.Affine {
	return kurbo.NewAffine([6]float64{
		float64(t.fontSize) * float64(t.horizontalScalingFactor()), 0, 0, float64(t.fontSize), 0, float64(t.rise),
	})
}

// horizontalScalingFactor is TextState::horizontal_scaling in
// hayro-interpret src/interpret/state.rs. It returns the scaling as a
// factor.
func (t *textState) horizontalScalingFactor() float32 {
	return t.horizontalScaling / 100
}

// fontHorizontal is TextState::font_horizontal in hayro-interpret
// src/interpret/state.rs.
func (t *textState) fontHorizontal() bool {
	return t.font != nil && t.font.IsHorizontal()
}

// applyAdjustment is TextState::apply_adjustment in hayro-interpret
// src/interpret/state.rs. It moves the text by a number of a TJ array.
func (t *textState) applyAdjustment(adjustment float32) {
	horizontal := t.fontHorizontal()
	scaling := float32(1)
	if horizontal {
		scaling = t.horizontalScalingFactor()
	}
	scaled := -adjustment / pdffont.UnitsPerEm * t.fontSize * scaling
	var tx, ty float32
	if horizontal {
		tx = scaled
	} else {
		ty = scaled
	}
	t.textMatrix = t.textMatrix.Mul(kurbo.NewAffine([6]float64{1, 0, 0, 1, float64(tx), float64(ty)}))
}

// applyCodeAdvance is TextState::apply_code_advance in hayro-interpret
// src/interpret/state.rs. It moves the text past a character code.
func (t *textState) applyCodeAdvance(charCode uint32, codeLen int) {
	var glyphAdvance kurbo.Vec2
	if t.font != nil {
		glyphAdvance = t.font.CodeAdvance(charCode)
	}
	horizontal := t.fontHorizontal()
	var wordSpace float32
	if charCode == 32 && codeLen == 1 {
		wordSpace = t.wordSpace
	}
	baseAdvance := func(advance float32) float32 {
		return advance/pdffont.UnitsPerEm*t.fontSize + t.charSpace + wordSpace
	}
	var tx, ty float32
	if horizontal {
		tx = baseAdvance(float32(glyphAdvance.X)) * t.horizontalScalingFactor()
	} else {
		ty = baseAdvance(float32(glyphAdvance.Y))
	}
	t.textMatrix = t.textMatrix.Mul(kurbo.NewAffine([6]float64{1, 0, 0, 1, float64(tx), float64(ty)}))
}

// fullTransform is TextState::full_transform in hayro-interpret
// src/interpret/state.rs.
func (t *textState) fullTransform() kurbo.Affine {
	return t.textMatrix.Mul(t.tempTransform())
}

// paintState is PaintState in hayro-interpret src/interpret/state.rs.
type paintState struct {
	color      ColorComponents
	pattern    Pattern
	colorSpace ColorSpace
	alpha      float32
}

// defaultPaintState is the Default implementation of PaintState in
// hayro-interpret src/interpret/state.rs.
func defaultPaintState() paintState {
	return paintState{color: ColorComponents{0}, colorSpace: DeviceGray(), alpha: 1}
}

// compositingState is CompositingState in hayro-interpret
// src/interpret/state.rs.
type compositingState struct {
	softMask         *SoftMask
	transferFunction *ActiveTransferFunction
	blendMode        BlendMode
}

// paintData is PaintData in hayro-interpret src/interpret/state.rs.
type paintData struct {
	alpha            float32
	color            ColorComponents
	colorSpace       ColorSpace
	pattern          Pattern
	transferFunction *ActiveTransferFunction
}

// handleGs is handle_gs in hayro-interpret src/interpret/state.rs. It
// applies an external graphics state.
func handleGs(dict syntax.Dict, ctx *Context, parentResources *syntax.Resources) {
	for _, key := range dict.Keys() {
		handleGsSingle(dict, key, ctx, parentResources)
	}
}

// handleGsSingle is handle_gs_single in hayro-interpret
// src/interpret/state.rs. It applies one key of an external graphics
// state. A key whose value cannot be read changes nothing.
func handleGsSingle(dict syntax.Dict, key syntax.Name, ctx *Context, parentResources *syntax.Resources) {
	switch key {
	case "LW":
		if v, ok := dict.Get[float32](key); ok {
			ctx.strokePropsMut().LineWidth = v
		}
	case "LC":
		if v, ok := dict.Get[syntax.Number](key); ok {
			ctx.strokePropsMut().LineCap = convertLineCap(syntax.LineCap{V0: v})
		}
	case "LJ":
		if v, ok := dict.Get[syntax.Number](key); ok {
			ctx.strokePropsMut().LineJoin = convertLineJoin(syntax.LineJoin{V0: v})
		}
	case "ML":
		if v, ok := dict.Get[float32](key); ok {
			ctx.strokePropsMut().MiterLimit = v
		}
	case "CA":
		if v, ok := dict.Get[float32](key); ok {
			ctx.strokePaintMut().alpha = v
		}
	case "ca":
		if v, ok := dict.Get[float32](key); ok {
			ctx.fillPaintMut().alpha = v
		}
	case "TR", "TR2":
		obj, ok := dict.Get[syntax.Object](syntax.KeyTR2)
		if !ok {
			if obj, ok = dict.Get[syntax.Object](syntax.KeyTR); !ok {
				return
			}
		}
		var function *ActiveTransferFunction
		switch o := obj.(type) {
		case syntax.Array:
			iter := o.Iter[syntax.Object]()
			var functions [4]*TransferFunction
			for i := range functions {
				fo, ok := iter.Next()
				if !ok {
					return
				}
				f, ok := NewFunction(fo)
				if !ok {
					return
				}
				functions[i] = newTransferFunction(f)
			}
			function = &ActiveTransferFunction{Four: functions}
		case syntax.Name:
			// Only Identity and Default are valid, and both reset it.
		default:
			f, ok := NewFunction(o)
			if !ok {
				return
			}
			function = &ActiveTransferFunction{Single: newTransferFunction(f)}
		}
		ctx.compositingMut().transferFunction = function
	case "SMask":
		if name, ok := dict.Get[syntax.Name](syntax.KeySMask); ok {
			if name == syntax.KeyNone {
				ctx.compositingMut().softMask = nil
			}
			return
		}
		var mask *SoftMask
		if d, ok := dict.Get[syntax.Dict](syntax.KeySMask); ok {
			mask, _ = newSoftMask(d, ctx, parentResources)
		}
		ctx.compositingMut().softMask = mask
	case "BM":
		if name, ok := dict.Get[syntax.Name](key); ok {
			if bm, ok := convertBlendMode(name); ok {
				ctx.compositingMut().blendMode = bm
				return
			}
		} else if arr, ok := dict.Get[syntax.Array](key); ok {
			for name := range arr.Iter[syntax.Name]().All() {
				if bm, ok := convertBlendMode(name); ok {
					ctx.compositingMut().blendMode = bm
					return
				}
			}
		}
		ctx.compositingMut().blendMode = BlendModeNormal
	case "Font":
		arr, ok := dict.Get[syntax.Array](syntax.KeyFont)
		if !ok {
			return
		}
		iter := arr.Iter[syntax.Object]()
		fo, ok := iter.Next()
		if !ok {
			return
		}
		fontDict, ok := syntax.ObjectIntoDict(fo)
		if !ok {
			return
		}
		so, ok := iter.Next()
		if !ok {
			return
		}
		size, ok := syntax.ObjectIntoNumber(so)
		if !ok {
			return
		}
		font := ctx.resolveFont(fontDict)
		ctx.textStateMut().fontSize = size.AsF32()
		ctx.textStateMut().font = font
	case "D":
		arr, ok := dict.Get[syntax.Array](key)
		if !ok {
			return
		}
		iter := arr.Iter[syntax.Object]()
		ao, ok := iter.Next()
		if !ok {
			return
		}
		dashArray, ok := syntax.ObjectIntoArray(ao)
		if !ok {
			return
		}
		po, ok := iter.Next()
		if !ok {
			return
		}
		phase, ok := syntax.ObjectIntoNumber(po)
		if !ok {
			return
		}
		ctx.strokePropsMut().DashOffset = phase.AsF32()
		ctx.strokePropsMut().DashArray = dashArrayOf(dashArray)
	}
}

// dashArrayOf reads a dash array. kurbo cannot deal with a dash of exactly
// 0, as upstream notes, so a 0 becomes 0.01.
func dashArrayOf(a syntax.Array) []float32 {
	var out []float32
	for n := range a.Iter[float32]().All() {
		if n == 0 {
			n = 0.01
		}
		out = append(out, n)
	}
	return out
}

// convertBlendMode is convert_blend_mode in hayro-interpret
// src/interpret/state.rs.
func convertBlendMode(name syntax.Name) (BlendMode, bool) {
	switch name {
	case "Normal":
		return BlendModeNormal, true
	case "Multiply":
		return BlendModeMultiply, true
	case "Screen":
		return BlendModeScreen, true
	case "Overlay":
		return BlendModeOverlay, true
	case "Darken":
		return BlendModeDarken, true
	case "Lighten":
		return BlendModeLighten, true
	case "ColorDodge":
		return BlendModeColorDodge, true
	case "ColorBurn":
		return BlendModeColorBurn, true
	case "HardLight":
		return BlendModeHardLight, true
	case "SoftLight":
		return BlendModeSoftLight, true
	case "Difference":
		return BlendModeDifference, true
	case "Exclusion":
		return BlendModeExclusion, true
	case "Hue":
		return BlendModeHue, true
	case "Saturation":
		return BlendModeSaturation, true
	case "Color":
		return BlendModeColor, true
	case "Luminosity":
		return BlendModeLuminosity, true
	}
	return BlendModeNormal, false
}
