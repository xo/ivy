// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/context.rs.

package interpret

import (
	"errors"
	"fmt"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy"
	"github.com/xo/ivy/pdf/internal/kurboext"
	"github.com/xo/ivy/pdf/internal/pdffont"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// maxNestedInterpretationDepth is MAX_NESTED_INTERPRETATION_DEPTH in
// hayro-interpret src/context.rs.
const maxNestedInterpretationDepth = 50

// InterpreterCache is InterpreterCache in hayro-interpret src/context.rs.
// It is a cache that the interpretations of the pages of one document
// share. It is safe for concurrent use.
//
// Upstream notes that the keys are unique only within a document, so a
// cache that two documents share can mix up their objects.
type InterpreterCache struct {
	fontCache   *pdffont.FontCache
	objectCache *Cache
}

// NewInterpreterCache is InterpreterCache::new in hayro-interpret
// src/context.rs.
func NewInterpreterCache() *InterpreterCache {
	return &InterpreterCache{fontCache: pdffont.NewFontCache(), objectCache: newCache()}
}

// ErrCanceled is the error of an interpretation that the Cancel function
// of InterpreterSettings stopped.
//
// Port: upstream cannot stop an interpretation.
var ErrCanceled = errors.New("interpreting pdf: canceled")

// runState is the state of one interpretation that the nested contexts
// share: the check that stops it, the count of the work, and the first
// error.
//
// Port: upstream has no such state. It bounds the nesting only. The count
// bounds the work that nested forms, patterns, glyphs of Type 3 and soft
// masks can multiply (D18).
type runState struct {
	cancel func() bool
	maxOps int64
	ops    int64
	err    error
}

// newRunState returns the state of an interpretation with the settings.
func newRunState(settings *InterpreterSettings) *runState {
	lim := settings.Limits.WithDefaults()
	return &runState{cancel: settings.Cancel, maxOps: opsPerObject * int64(lim.MaxObjects)}
}

// opsPerObject is the number of operators that the interpretation of a page
// can run for each object of Limits.MaxObjects. The default is 64 million.
const opsPerObject = 64

// cancelInterval is the number of units of work between two calls of the
// Cancel function.
const cancelInterval = 1 << 10

// step counts n units of work. It returns false when the interpretation
// must stop.
func (r *runState) step(n int64) bool {
	if r.err != nil {
		return false
	}
	before := r.ops
	r.ops += n
	if r.ops > r.maxOps {
		r.err = fmt.Errorf("interpreting pdf: more than %d operators: %w", r.maxOps, ivy.ErrLimit)
		return false
	}
	if r.cancel != nil && (before == 0 || before/cancelInterval != r.ops/cancelInterval) && r.cancel() {
		r.err = ErrCanceled
		return false
	}
	return true
}

// canceled reports whether the interpretation must stop, and calls the
// Cancel function. Loops that run no operator, such as the drawing of a
// mesh, call it.
func (r *runState) canceled() bool {
	if r == nil {
		return false
	}
	if r.err != nil {
		return true
	}
	if r.cancel != nil && r.cancel() {
		r.err = ErrCanceled
		return true
	}
	return false
}

// Context is Context in hayro-interpret src/context.rs. It holds the state
// of the interpretation of one page.
type Context struct {
	stack            *stateStack
	path             kurboext.BezPath
	subPathStart     kurbo.Point
	lastPoint        kurbo.Point
	clip             FillRule
	hasClip          bool
	rootTransforms   []kurbo.Affine
	bbox             []kurbo.Rect
	settings         InterpreterSettings
	interpreterCache *InterpreterCache
	xref             *syntax.XRef
	ocgState         *ocgState
	nestingDepth     uint32
	glyphScratch     []PositionedGlyph
	run              *runState
}

// NewContext is Context::new in hayro-interpret src/context.rs. It returns
// the context of a page that draws with the initial transform into the
// box bbox of the device.
func NewContext(initialTransform kurbo.Affine, bbox kurbo.Rect, cache *InterpreterCache, xref *syntax.XRef, settings InterpreterSettings) *Context {
	return newContextWith(initialTransform, bbox, cache, xref, settings, newState(initialTransform), 0, nil)
}

// newContextWith is Context::new_with in hayro-interpret src/context.rs. A
// nil run starts a new count of the work.
func newContextWith(initialTransform kurbo.Affine, bbox kurbo.Rect, cache *InterpreterCache, xref *syntax.XRef, settings InterpreterSettings, s state, nestingDepth uint32, run *runState) *Context {
	ocg := &ocgState{}
	if xref != nil {
		if catalog, ok := xref.Get[syntax.Dict](xref.RootID()); ok {
			ocg = ocgStateFromCatalog(catalog)
		}
	}
	if cache == nil {
		cache = NewInterpreterCache()
	}
	if run == nil {
		run = newRunState(&settings)
	}
	return &Context{
		stack:            newStateStack(s),
		settings:         settings,
		xref:             xref,
		rootTransforms:   []kurbo.Affine{initialTransform},
		bbox:             []kurbo.Rect{bbox},
		interpreterCache: cache,
		ocgState:         ocg,
		nestingDepth:     nestingDepth,
		run:              run,
	}
}

// Err returns the reason why the interpretation stopped early: ErrCanceled,
// an error that wraps ivy.ErrLimit, or an error that wraps ivy.ErrFormat
// for an internal fault. It returns nil when the interpretation ran to the
// end.
//
// Port: upstream cannot fail.
func (ctx *Context) Err() error {
	return ctx.run.err
}

// saveState is Context::save_state in hayro-interpret src/context.rs.
func (ctx *Context) saveState() {
	ctx.stack.save()
}

// currentBbox is Context::bbox in hayro-interpret src/context.rs.
func (ctx *Context) currentBbox() kurbo.Rect {
	if len(ctx.bbox) == 0 {
		return kurbo.NewRect(0, 0, 1, 1)
	}
	return ctx.bbox[len(ctx.bbox)-1]
}

// pushBbox is Context::push_bbox in hayro-interpret src/context.rs.
func (ctx *Context) pushBbox(bbox kurbo.Rect) {
	ctx.bbox = append(ctx.bbox, kurboext.RectIntersect(ctx.currentBbox(), bbox))
}

// pushClipPath is Context::push_clip_path in hayro-interpret
// src/context.rs. A rectangle that holds the current box is a clip that
// the device does not see.
func (ctx *Context) pushClipPath(clipPath kurboext.BezPath, fill FillRule, device Device) {
	if clipRect, ok := pathAsRect(&clipPath); ok {
		cur := ctx.currentBbox()
		if IsNearlyGreaterOrEqual(kurboext.RectMinX(cur), kurboext.RectMinX(clipRect)) &&
			IsNearlyGreaterOrEqual(kurboext.RectMinY(cur), kurboext.RectMinY(clipRect)) &&
			IsNearlyLessOrEqual(kurboext.RectMaxX(cur), kurboext.RectMaxX(clipRect)) &&
			IsNearlyLessOrEqual(kurboext.RectMaxY(cur), kurboext.RectMaxY(clipRect)) {
			ctx.stack.pushClip(clipDummy)
			return
		}
		pushClipRect(device, clipRect)
		ctx.pushBbox(clipRect)
		ctx.stack.pushClip(clipReal)
		return
	}
	bbox := clipPath.BoundingBox()
	device.PushClipPath(&ClipPath{Path: clipPath, Fill: fill})
	ctx.pushBbox(bbox)
	ctx.stack.pushClip(clipReal)
}

// popClip is Context::pop_clip in hayro-interpret src/context.rs.
func (ctx *Context) popClip(device Device) {
	if c, ok := ctx.stack.popClip(); ok && c == clipReal {
		device.PopClip()
		ctx.popBbox()
	}
}

// popBbox is Context::pop_bbox in hayro-interpret src/context.rs.
func (ctx *Context) popBbox() {
	if len(ctx.bbox) > 0 {
		ctx.bbox = ctx.bbox[:len(ctx.bbox)-1]
	}
}

// pushRootTransform is Context::push_root_transform in hayro-interpret
// src/context.rs.
func (ctx *Context) pushRootTransform() {
	ctx.rootTransforms = append(ctx.rootTransforms, ctx.ctm())
}

// popRootTransform is Context::pop_root_transform in hayro-interpret
// src/context.rs.
func (ctx *Context) popRootTransform() {
	if len(ctx.rootTransforms) > 0 {
		ctx.rootTransforms = ctx.rootTransforms[:len(ctx.rootTransforms)-1]
	}
}

// rootTransform is Context::root_transform in hayro-interpret
// src/context.rs.
func (ctx *Context) rootTransform() kurbo.Affine {
	if len(ctx.rootTransforms) == 0 {
		return kurbo.IdentityAffine
	}
	return ctx.rootTransforms[len(ctx.rootTransforms)-1]
}

// restoreState is Context::restore_state in hayro-interpret
// src/context.rs. It pops the clips of the state, then the state.
func (ctx *Context) restoreState(device Device) {
	target, ok := ctx.stack.restoreTargetClips()
	if !ok {
		return
	}
	for ctx.stack.numClips() > target {
		ctx.popClip(device)
	}
	ctx.stack.restore()
}

// drawProps is Context::draw_props in hayro-interpret src/context.rs.
func (ctx *Context) drawProps(isStroke bool) DrawProps {
	comp := ctx.compositing()
	return DrawProps{
		Transform: ctx.ctm(),
		Paint:     ctx.getPaint(isStroke),
		SoftMask:  comp.softMask,
		BlendMode: comp.blendMode,
	}
}

// getPaint is Context::get_paint in hayro-interpret src/context.rs.
func (ctx *Context) getPaint(isStroke bool) Paint {
	paint := ctx.fillPaint()
	if isStroke {
		paint = ctx.strokePaint()
	}
	data := paintData{
		alpha:            paint.alpha,
		color:            paint.color,
		colorSpace:       paint.colorSpace,
		pattern:          paint.pattern,
		transferFunction: ctx.compositing().transferFunction,
	}
	if data.colorSpace.isPattern() || data.pattern != nil {
		if data.pattern == nil {
			return newColor(DeviceGray(), ColorComponents{0}, 0)
		}
		pattern := clonePattern(data.pattern)
		patternSetOpacity(pattern, data.alpha)
		if data.transferFunction != nil {
			patternSetTransferFunction(pattern, data.transferFunction)
		}
		patternPreConcatTransform(pattern, ctx.rootTransform())
		return pattern
	}
	color := newColor(data.colorSpace, data.color, data.alpha)
	if data.transferFunction != nil {
		return ColorFromRGBA(data.transferFunction.Apply(color.ToRGBA()))
	}
	return color
}

// The accessors below are the accessors of the same names in
// hayro-interpret src/context.rs. A pointer of a mut accessor is valid
// until the next call of the same mut accessor.

func (ctx *Context) ctm() kurbo.Affine              { return *ctx.stack.ctmRef() }
func (ctx *Context) ctmMut() *kurbo.Affine          { return ctx.stack.ctmMut() }
func (ctx *Context) strokePaint() *paintState       { return ctx.stack.strokeRef() }
func (ctx *Context) strokePaintMut() *paintState    { return ctx.stack.strokeMut() }
func (ctx *Context) fillPaint() *paintState         { return ctx.stack.fillRef() }
func (ctx *Context) fillPaintMut() *paintState      { return ctx.stack.fillMut() }
func (ctx *Context) strokeProps() *StrokeProps      { return ctx.stack.strokePropsRef() }
func (ctx *Context) strokePropsMut() *StrokeProps   { return ctx.stack.strokePropsMut() }
func (ctx *Context) textState() *textState          { return ctx.stack.textRef() }
func (ctx *Context) textStateMut() *textState       { return ctx.stack.textMut() }
func (ctx *Context) compositing() *compositingState { return ctx.stack.compositingRef() }

func (ctx *Context) compositingMut() *compositingState {
	return ctx.stack.compositingMut()
}

// snapshot is Context::snapshot in hayro-interpret src/context.rs.
//
// Port: the snapshot has the default text state. See stateStack.snapshot.
func (ctx *Context) snapshot() state {
	return ctx.stack.snapshot()
}

// preConcatTransform is Context::pre_concat_transform in hayro-interpret
// src/context.rs.
func (ctx *Context) preConcatTransform(t syntax.TransformOps) {
	ctx.preConcatAffine(convertTransform(t))
}

// preConcatAffine is Context::pre_concat_affine in hayro-interpret
// src/context.rs.
func (ctx *Context) preConcatAffine(t kurbo.Affine) {
	m := ctx.ctmMut()
	*m = m.Mul(t)
}

// getColorSpace is Context::get_color_space in hayro-interpret
// src/context.rs. It reads a color space of the resources, through the
// cache.
func (ctx *Context) getColorSpace(resources *syntax.Resources, name syntax.Name) (ColorSpace, bool) {
	obj, ok := resources.GetColorSpace(name)
	if !ok {
		return ColorSpace{}, false
	}
	cache := ctx.interpreterCache.objectCache
	return cacheGetOrInsertWith(cache, objectCacheKey(obj), func() (ColorSpace, bool) {
		return newColorSpace(obj, cache)
	})
}

// numStates is Context::num_states in hayro-interpret src/context.rs.
func (ctx *Context) numStates() int {
	return ctx.stack.len()
}

// beginNestedInterpretation is Context::begin_nested_interpretation in
// hayro-interpret src/context.rs. It fails past the largest depth.
func (ctx *Context) beginNestedInterpretation() bool {
	if ctx.nestingDepth >= maxNestedInterpretationDepth {
		return false
	}
	ctx.nestingDepth++
	return true
}

// endNestedInterpretation is Context::end_nested_interpretation in
// hayro-interpret src/context.rs.
func (ctx *Context) endNestedInterpretation() {
	if ctx.nestingDepth > 0 {
		ctx.nestingDepth--
	}
}

// fallbackFont is Context::fallback_font in hayro-interpret src/context.rs.
func (ctx *Context) fallbackFont() *textStateFont {
	font, ok := ctx.interpreterCache.fontCache.StandardFont(pdffont.Helvetica, ctx.settings.FontResolver)
	if !ok {
		return nil
	}
	return &textStateFont{Font: font, fallback: true}
}

// resolveFont is Context::resolve_font in hayro-interpret src/context.rs.
// It returns the fallback font, with a warning, when the font fails.
func (ctx *Context) resolveFont(fontDict syntax.Dict) *textStateFont {
	if font, ok := ctx.interpreterCache.fontCache.Font(fontDict, ctx.settings.FontResolver, ctx.settings.CMapResolver); ok {
		return &textStateFont{Font: font}
	}
	ctx.settings.warn(InterpreterWarningUnsupportedFont)
	return ctx.fallbackFont()
}

// pathAsRect is path_as_rect in hayro-interpret src/context.rs. It returns
// the rectangle of a path that is a rectangle aligned to the axes.
func pathAsRect(path *kurboext.BezPath) (kurbo.Rect, bool) {
	els := path.Elements()
	if len(els) != 5 || els[0].Kind != kurbo.PathElMoveTo ||
		els[1].Kind != kurbo.PathElLineTo || els[2].Kind != kurbo.PathElLineTo || els[3].Kind != kurbo.PathElLineTo {
		return kurbo.Rect{}, false
	}
	first := els[0].P[0]
	var points [5]kurbo.Point
	switch els[4].Kind {
	case kurbo.PathElClosePath:
		points = [5]kurbo.Point{first, els[1].P[0], els[2].P[0], els[3].P[0], first}
	case kurbo.PathElLineTo:
		last := els[4].P[0]
		if !IsNearlyEqual(first.X, last.X) || !IsNearlyEqual(first.Y, last.Y) {
			return kurbo.Rect{}, false
		}
		points = [5]kurbo.Point{first, els[1].P[0], els[2].P[0], els[3].P[0], last}
	default:
		return kurbo.Rect{}, false
	}
	previous, hasPrevious := false, false
	for i := range 4 {
		sameX := IsNearlyEqual(points[i].X, points[i+1].X)
		sameY := IsNearlyEqual(points[i].Y, points[i+1].Y)
		if sameX == sameY || (hasPrevious && previous == sameX) {
			return kurbo.Rect{}, false
		}
		previous, hasPrevious = sameX, true
	}
	return fastBoundingBox(path), true
}
