// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/idwt.rs.
//
// Port: upstream passes the lifting steps as two closures, one for f32x8
// and one for f32. The port passes a liftOp, and liftSamples runs one
// scalar loop for each operation. The loops are the place for SIMD later
// (D9).

// idwtOutput is IDWTOutput in hayro-jpeg2000 src/j2c/idwt.rs.
type idwtOutput struct {
	coefficients []float32
	rect         intRect
}

// idwtApply is apply in hayro-jpeg2000 src/j2c/idwt.rs. It runs the inverse
// wavelet transform of one component of a tile, and leaves the result in
// the output of the context.
func idwtApply(storage *decompositionStorage, tileCtx *tileDecodeContext, componentIdx int, h *header, transform waveletTransform, c cancel) error {
	td := &storage.tileDecompositions[componentIdx]

	decompositions := storage.decompositions[td.decompositions.start:td.decompositions.end]
	keep := len(decompositions) - int(h.skippedResolutionLevels)
	decompositions = decompositions[:max(keep, 0)]
	ll := &storage.subBands[td.firstLLSubBand]

	out := &tileCtx.idwtOutput

	if len(decompositions) == 0 {
		out.coefficients = append(out.coefficients[:0], storage.coefficients[ll.coefficients.start:ll.coefficients.end]...)
		out.rect = ll.rect
		return nil
	}

	// Start with the buffer that makes the last decomposition land in the
	// output.
	useScratch := len(decompositions)%2 == 0

	input := storage.coefficients[ll.coefficients.start:ll.coefficients.end]
	for i := range decompositions {
		if i > 0 {
			useScratch = !useScratch
			if useScratch {
				input = out.coefficients
			} else {
				input = tileCtx.idwtScratchBuffer
			}
		}
		target := &out.coefficients
		if useScratch {
			target = &tileCtx.idwtScratchBuffer
		}
		if err := filter2D(input, target, &decompositions[i], transform, storage, c); err != nil {
			return err
		}
	}

	out.rect = decompositions[len(decompositions)-1].rect
	return nil
}

// filter2D is filter_2d in hayro-jpeg2000 src/j2c/idwt.rs, with
// filter_2d_inner. It interleaves the four sub-bands of a decomposition
// into coefficients, and filters the rows and the columns.
func filter2D(input []float32, coefficients *[]float32, d *decomposition, transform waveletTransform, storage *decompositionStorage, c cancel) error {
	width := int(d.rect.width())
	height := int(d.rect.height())
	// Port: the limits bound the size of the buffer.
	if err := storage.lim.memory(uint64(width) * uint64(height) * 4); err != nil {
		return err
	}

	// No clearing needed: interleaving overwrites every sample.
	var err error
	if *coefficients, err = c.resize(*coefficients, width*height); err != nil {
		return err
	}

	if width == 0 || height == 0 {
		return nil
	}

	if err := interleaveAndFilterHorizontal(input, *coefficients, d, transform, storage, c); err != nil {
		return err
	}
	return filterVertical(*coefficients, d.rect, transform, c)
}

// interleaveAndFilterHorizontal is interleave_and_filter_horizontal in
// hayro-jpeg2000 src/j2c/idwt.rs.
func interleaveAndFilterHorizontal(input, coefficients []float32, d *decomposition, transform waveletTransform, storage *decompositionStorage, c cancel) error {
	u0, u1, v0 := d.rect.x0, d.rect.x1, d.rect.y0
	width := int(d.rect.width())

	ll := input
	var bands [3][]float32
	for i, idx := range d.subBands {
		sb := &storage.subBands[idx]
		bands[i] = storage.coefficients[sb.coefficients.start:sb.coefficients.end]
	}
	hl, lh, hh := bands[0], bands[1], bands[2]

	numLow := int(divCeil32(u1, 2) - divCeil32(u0, 2))
	numHigh := int(u1/2 - u0/2)
	filter := newHorizontalFilter(transform, u0%2 == 0, numLow, numHigh)

	firstLowRow, firstHighRow := 0, 1
	if v0%2 != 0 {
		firstLowRow, firstHighRow = 1, 0
	}
	if err := filter.filterRows(ll, hl, coefficients, width, firstLowRow, c); err != nil {
		return err
	}
	return filter.filterRows(lh, hh, coefficients, width, firstHighRow, c)
}

// horizontalFilter is HorizontalFilter in hayro-jpeg2000 src/j2c/idwt.rs.
// The bands low and high have one sample of padding on each side.
type horizontalFilter struct {
	transform waveletTransform
	lowFirst  bool
	numLow    int
	numHigh   int
	low       []float32
	high      []float32
}

// newHorizontalFilter is HorizontalFilter::new.
func newHorizontalFilter(transform waveletTransform, lowFirst bool, numLow, numHigh int) *horizontalFilter {
	return &horizontalFilter{
		transform: transform,
		lowFirst:  lowFirst,
		numLow:    numLow,
		numHigh:   numHigh,
		low:       make([]float32, numLow+2),
		high:      make([]float32, numHigh+2),
	}
}

// filterRows is HorizontalFilter::filter_rows. It filters every second row
// of coefficients from firstRow, with the samples of the two bands.
//
// Port: upstream slices the bands, and panics when they are too short. The
// port returns an error.
func (f *horizontalFilter) filterRows(lowBand, highBand, coefficients []float32, width, firstRow int, c cancel) error {
	v := 0
	for r := firstRow; (r+1)*width <= len(coefficients); r += 2 {
		if err := c.check(); err != nil {
			return err
		}
		row := coefficients[r*width : (r+1)*width]

		if (v+1)*f.numLow > len(lowBand) || (v+1)*f.numHigh > len(highBand) {
			return errInternal
		}
		copy(f.low[1:1+f.numLow], lowBand[v*f.numLow:])
		copy(f.high[1:1+f.numHigh], highBand[v*f.numHigh:])

		filter1D(f.transform, f)

		low := f.low[1 : 1+f.numLow]
		high := f.high[1 : 1+f.numHigh]
		if f.lowFirst {
			interleaveRow(low, high, row)
		} else {
			interleaveRow(high, low, row)
		}
		v++
	}
	return nil
}

// numSamples is Lifting::num_samples for HorizontalFilter.
func (f *horizontalFilter) numSamples() int {
	return f.numLow + f.numHigh
}

// scaleLow is Lifting::scale_low for HorizontalFilter.
func (f *horizontalFilter) scaleLow(factor float32) {
	scale(f.low[1:1+f.numLow], factor)
}

// scaleHigh is Lifting::scale_high for HorizontalFilter.
func (f *horizontalFilter) scaleHigh(factor float32) {
	scale(f.high[1:1+f.numHigh], factor)
}

// liftLow is Lifting::lift_low for HorizontalFilter.
func (f *horizontalFilter) liftLow(op liftOp) {
	offset := 0
	if f.lowFirst {
		offset = 1
	}
	liftBand(f.low, f.high, offset, op)
}

// liftHigh is Lifting::lift_high for HorizontalFilter.
func (f *horizontalFilter) liftHigh(op liftOp) {
	offset := 1
	if f.lowFirst {
		offset = 0
	}
	liftBand(f.high, f.low, offset, op)
}

// liftBand is lift_band in hayro-jpeg2000 src/j2c/idwt.rs. Both bands have
// one sample of padding on each side.
func liftBand(target, source []float32, offset int, op liftOp) {
	last := len(source) - 1
	source[0] = source[1]
	source[last] = source[last-1]

	n := len(target) - 2
	if 2-offset+n > len(source) {
		// Port: upstream panics here. The geometry of the bands never
		// reaches it.
		n = len(source) - 2 + offset
	}
	liftSamples(target[1:1+n], source[1-offset:1-offset+n], source[2-offset:2-offset+n], op)
}

// interleaveRow is interleave_row in hayro-jpeg2000 src/j2c/idwt.rs. It
// writes the samples of first and second in turn to output.
func interleaveRow(first, second, output []float32) {
	numPairs := min(len(first), len(second))
	numPairs = min(numPairs, len(output)/2)
	for i := range numPairs {
		output[2*i] = first[i]
		output[2*i+1] = second[i]
	}
	if len(first) > numPairs && 2*numPairs < len(output) {
		output[2*numPairs] = first[numPairs]
	}
}

// filterVertical is filter_vertical in hayro-jpeg2000 src/j2c/idwt.rs.
func filterVertical(coefficients []float32, rect intRect, transform waveletTransform, c cancel) error {
	f := &verticalFilter{
		cancel:       c,
		coefficients: coefficients,
		width:        int(rect.width()),
		height:       int(rect.height()),
		lowFirst:     rect.y0%2 == 0,
	}
	filter1D(transform, f)
	return f.err
}

// verticalFilter is VerticalFilter in hayro-jpeg2000 src/j2c/idwt.rs.
//
// Port: the methods of the trait Lifting return no error in the port. The
// filter keeps the first error of a check in err, and skips the remaining
// work.
type verticalFilter struct {
	cancel       cancel
	coefficients []float32
	width        int
	height       int
	lowFirst     bool
	err          error
}

// scaleRows is VerticalFilter::scale_rows.
func (f *verticalFilter) scaleRows(firstRow int, factor float32) {
	for r := firstRow; r < f.height; r += 2 {
		if f.err != nil {
			return
		}
		if f.err = f.cancel.check(); f.err != nil {
			return
		}
		scale(f.coefficients[r*f.width:(r+1)*f.width], factor)
	}
}

// liftRows is VerticalFilter::lift_rows.
func (f *verticalFilter) liftRows(firstRow int, op liftOp) {
	w, h := f.width, f.height
	for r := firstRow; r < h; r += 2 {
		if f.err != nil {
			return
		}
		if f.err = f.cancel.check(); f.err != nil {
			return
		}
		above := r - 1
		if r == 0 {
			above = 1
		}
		below := r + 1
		if r == h-1 {
			below = h - 2
		}
		liftSamples(f.coefficients[r*w:(r+1)*w], f.coefficients[above*w:(above+1)*w], f.coefficients[below*w:(below+1)*w], op)
	}
}

// numSamples is Lifting::num_samples for VerticalFilter.
func (f *verticalFilter) numSamples() int {
	return f.height
}

// scaleLow is Lifting::scale_low for VerticalFilter.
func (f *verticalFilter) scaleLow(factor float32) {
	f.scaleRows(b2i(!f.lowFirst), factor)
}

// scaleHigh is Lifting::scale_high for VerticalFilter.
func (f *verticalFilter) scaleHigh(factor float32) {
	f.scaleRows(b2i(f.lowFirst), factor)
}

// liftLow is Lifting::lift_low for VerticalFilter.
func (f *verticalFilter) liftLow(op liftOp) {
	f.liftRows(b2i(!f.lowFirst), op)
}

// liftHigh is Lifting::lift_high for VerticalFilter.
func (f *verticalFilter) liftHigh(op liftOp) {
	f.liftRows(b2i(f.lowFirst), op)
}

// b2i is usize::from(b) of Rust for a boolean.
func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// lifting is Lifting in hayro-jpeg2000 src/j2c/idwt.rs: a signal that the
// one-dimensional filters lift, either a row or the columns.
type lifting interface {
	numSamples() int
	scaleLow(factor float32)
	scaleHigh(factor float32)
	liftLow(op liftOp)
	liftHigh(op liftOp)
}

// liftOp is one lifting step: the closures f_simd and f_scalar of upstream.
type liftOp struct {
	kind liftKind
	// coef is the coefficient of a step of the 9/7 filter.
	coef float32
}

// liftKind is the kind of a lifting step.
type liftKind uint8

const (
	// lift53Low is the step of the 5/3 filter on the low samples.
	lift53Low liftKind = iota
	// lift53High is the step of the 5/3 filter on the high samples.
	lift53High
	// lift97 is a step of the 9/7 filter.
	lift97
)

// filter1D is filter_1d in hayro-jpeg2000 src/j2c/idwt.rs.
func filter1D(transform waveletTransform, signal lifting) {
	if signal.numSamples() == 1 {
		signal.scaleHigh(0.5)
		return
	}
	switch transform {
	case reversible53:
		filter53R(signal)
	default:
		filter97I(signal)
	}
}

// filter53R is filter_53r in hayro-jpeg2000 src/j2c/idwt.rs: the
// reversible 5/3 filter.
func filter53R(signal lifting) {
	signal.liftLow(liftOp{kind: lift53Low})
	signal.liftHigh(liftOp{kind: lift53High})
}

// The constants of the irreversible 9/7 filter.
const (
	negAlpha float32 = 1.586_134_3
	negBeta  float32 = 0.052_980_117
	negGamma float32 = -0.882_911_1
	negDelta float32 = -0.443_506_87
	kappa    float32 = 1.230_174_1
	invKappa float32 = 1.0 / kappa
)

// filter97I is filter_97i in hayro-jpeg2000 src/j2c/idwt.rs: the
// irreversible 9/7 filter.
func filter97I(signal lifting) {
	signal.scaleLow(kappa)
	signal.scaleHigh(invKappa)
	signal.liftLow(liftOp{kind: lift97, coef: negDelta})
	signal.liftHigh(liftOp{kind: lift97, coef: negGamma})
	signal.liftLow(liftOp{kind: lift97, coef: negBeta})
	signal.liftHigh(liftOp{kind: lift97, coef: negAlpha})
}

// liftSamples is lift_samples in hayro-jpeg2000 src/j2c/idwt.rs, in its
// scalar version. It updates each target sample from its left and right
// neighbors.
func liftSamples(target, left, right []float32, op liftOp) {
	n := min(len(target), len(left), len(right))
	target, left, right = target[:n], left[:n], right[:n]
	switch op.kind {
	case lift53Low:
		for i := range target {
			target[i] -= floorF32(mulAdd(left[i]+right[i], 0.25, 0.5))
		}
	case lift53High:
		for i := range target {
			target[i] += floorF32((left[i] + right[i]) * 0.5)
		}
	case lift97:
		coef := op.coef
		for i := range target {
			target[i] = mulAdd(left[i]+right[i], coef, target[i])
		}
	}
}

// scale is scale in hayro-jpeg2000 src/j2c/idwt.rs.
func scale(samples []float32, factor float32) {
	for i := range samples {
		samples[i] *= factor
	}
}
