package graphics

import (
	"io"
	"math"

	"github.com/xo/ivy/ps/internal/interp"
)

// function is a function dictionary of PostScript (PLRM 3.10): a sampled
// function, an exponential interpolation, or a stitching function.
type function struct {
	typ    int
	domain []float64
	rng    []float64
	nOut   int

	// type 0
	size    []int
	bps     int
	encode  []float64
	decode  []float64
	samples []float64

	// type 2
	c0, c1 []float64
	n      float64

	// type 3
	funcs  []*function
	bounds []float64
}

// maxFunctionSamples bounds the samples of a sampled function.
const maxFunctionSamples = 1 << 22

// parseFunction reads a function dictionary, or an array of functions of
// one output each.
func (m *Machine) parseFunction(o interp.Object, depth int) (*function, error) {
	if depth > 8 {
		return nil, interp.ErrLimitcheck
	}
	if o.IsArray() && !o.IsProc() {
		f := &function{typ: -1}
		for _, e := range o.Array() {
			g, err := m.parseFunction(e, depth+1)
			if err != nil {
				return nil, err
			}
			f.funcs = append(f.funcs, g)
			f.domain = g.domain
		}
		f.nOut = len(f.funcs)
		return f, nil
	}
	if o.Type() != interp.TDict {
		return nil, interp.ErrTypecheck
	}
	d := o.Dict()
	typ := 0
	if v, ok := d.GetName("FunctionType"); ok && v.IsNumber() {
		typ = int(v.Float())
	}
	f := &function{typ: typ}
	if v, ok := d.GetName("Domain"); ok && v.IsArray() {
		f.domain = arrayNumbers(v)
	}
	if len(f.domain) < 2 {
		f.domain = []float64{0, 1}
	}
	if v, ok := d.GetName("Range"); ok && v.IsArray() {
		f.rng = arrayNumbers(v)
	}
	switch typ {
	case 0:
		return f, m.parseSampled(f, d)
	case 2:
		f.c0, f.c1 = []float64{0}, []float64{1}
		if v, ok := d.GetName("C0"); ok && v.IsArray() {
			f.c0 = arrayNumbers(v)
		}
		if v, ok := d.GetName("C1"); ok && v.IsArray() {
			f.c1 = arrayNumbers(v)
		}
		if len(f.c1) != len(f.c0) {
			return nil, interp.ErrRangecheck
		}
		f.n = 1
		if v, ok := d.GetName("N"); ok && v.IsNumber() {
			f.n = v.Float()
		}
		f.nOut = len(f.c0)
	case 3:
		fs, ok := d.GetName("Functions")
		if !ok || !fs.IsArray() || fs.Len() == 0 || fs.Len() > 256 {
			return nil, interp.ErrRangecheck
		}
		for _, e := range fs.Array() {
			g, err := m.parseFunction(e, depth+1)
			if err != nil {
				return nil, err
			}
			f.funcs = append(f.funcs, g)
		}
		if v, ok := d.GetName("Bounds"); ok && v.IsArray() {
			f.bounds = arrayNumbers(v)
		}
		if v, ok := d.GetName("Encode"); ok && v.IsArray() {
			f.encode = arrayNumbers(v)
		}
		if len(f.bounds) != len(f.funcs)-1 || len(f.encode) < 2*len(f.funcs) {
			return nil, interp.ErrRangecheck
		}
		f.nOut = f.funcs[0].nOut
	default:
		return nil, interp.ErrRangecheck
	}
	if len(f.rng) >= 2 {
		f.nOut = len(f.rng) / 2
	}
	return f, nil
}

// arrayNumbers returns the numbers of an array.
func arrayNumbers(o interp.Object) []float64 {
	a := o.Array()
	v := make([]float64, len(a))
	for i, e := range a {
		v[i] = e.Float()
	}
	return v
}

// parseSampled reads a sampled function.
func (m *Machine) parseSampled(f *function, d *interp.Dict) error {
	v, ok := d.GetName("Size")
	if !ok || !v.IsArray() || v.Len() == 0 || v.Len() > 4 {
		return interp.ErrRangecheck
	}
	total := 1
	for _, e := range v.Array() {
		s := int(e.Float())
		if s < 1 || s > maxFunctionSamples {
			return interp.ErrRangecheck
		}
		f.size = append(f.size, s)
		total *= s
		if total > maxFunctionSamples {
			return limitError("samples of a function", maxFunctionSamples)
		}
	}
	if len(f.domain) < 2*len(f.size) || len(f.rng) < 2 {
		return interp.ErrRangecheck
	}
	f.nOut = len(f.rng) / 2
	bps, ok := d.GetName("BitsPerSample")
	if !ok || !bps.IsNumber() {
		return interp.ErrRangecheck
	}
	f.bps = int(bps.Float())
	switch f.bps {
	case 1, 2, 4, 8, 12, 16, 24, 32:
	default:
		return interp.ErrRangecheck
	}
	if e, ok := d.GetName("Encode"); ok && e.IsArray() {
		f.encode = arrayNumbers(e)
	}
	if len(f.encode) < 2*len(f.size) {
		f.encode = nil
		for _, s := range f.size {
			f.encode = append(f.encode, 0, float64(s-1))
		}
	}
	if e, ok := d.GetName("Decode"); ok && e.IsArray() {
		f.decode = arrayNumbers(e)
	}
	if len(f.decode) < 2*f.nOut {
		f.decode = f.rng
	}
	src, ok := d.GetName("DataSource")
	if !ok {
		return interp.ErrRangecheck
	}
	n := total * f.nOut
	need := (n*f.bps + 7) / 8
	var data []byte
	if src.Type() == interp.TString {
		data = src.Bytes()
	} else {
		r, err := m.in.NewDataSource(src)
		if err != nil {
			return err
		}
		data = make([]byte, need)
		k, _ := io.ReadFull(r, data)
		data = data[:k]
	}
	f.samples = make([]float64, n)
	maxV := math.Pow(2, float64(f.bps)) - 1
	for i := range n {
		bit := i * f.bps
		var s uint64
		for k := 0; k < f.bps; k++ {
			byteIdx := (bit + k) / 8
			if byteIdx >= len(data) {
				break
			}
			s = s<<1 | uint64(data[byteIdx]>>(7-(bit+k)%8)&1)
		}
		j := i % f.nOut
		f.samples[i] = f.decode[2*j] + float64(s)*(f.decode[2*j+1]-f.decode[2*j])/maxV
	}
	return nil
}

// eval evaluates the function on in, and writes the outputs to out.
func (f *function) eval(in []float64, out []float64) {
	switch f.typ {
	case -1:
		for i, g := range f.funcs {
			var o [1]float64
			g.eval(in, o[:])
			if i < len(out) {
				out[i] = o[0]
			}
		}
		return
	case 0:
		f.evalSampled(in, out)
	case 2:
		x := clampRange(at(in, 0), f.domain[0], f.domain[1])
		p := math.Pow(x, f.n)
		if math.IsNaN(p) || math.IsInf(p, 0) {
			p = 0
		}
		for i := range f.c0 {
			if i < len(out) {
				out[i] = f.c0[i] + p*(f.c1[i]-f.c0[i])
			}
		}
	case 3:
		x := clampRange(at(in, 0), f.domain[0], f.domain[1])
		k := 0
		for k < len(f.bounds) && x >= f.bounds[k] {
			k++
		}
		lo, hi := f.domain[0], f.domain[1]
		if k > 0 {
			lo = f.bounds[k-1]
		}
		if k < len(f.bounds) {
			hi = f.bounds[k]
		}
		e0, e1 := f.encode[2*k], f.encode[2*k+1]
		y := e0
		if hi != lo {
			y = e0 + (x-lo)*(e1-e0)/(hi-lo)
		}
		f.funcs[k].eval([]float64{y}, out)
	}
	for i := range out {
		if 2*i+1 < len(f.rng) {
			out[i] = clampRange(out[i], f.rng[2*i], f.rng[2*i+1])
		}
	}
}

// evalSampled evaluates a sampled function, with linear interpolation in
// the first input and the nearest sample in the others.
func (f *function) evalSampled(in []float64, out []float64) {
	m := len(f.size)
	idx := make([]float64, m)
	for i := range m {
		x := clampRange(at(in, i), f.domain[2*i], f.domain[2*i+1])
		d0, d1 := f.domain[2*i], f.domain[2*i+1]
		e := f.encode[2*i]
		if d1 != d0 {
			e += (x - d0) * (f.encode[2*i+1] - f.encode[2*i]) / (d1 - d0)
		}
		idx[i] = clampRange(e, 0, float64(f.size[i]-1))
	}
	offset := func(first int) int {
		o, stride := first, f.size[0]
		for i := 1; i < m; i++ {
			o += int(math.Round(idx[i])) * stride
			stride *= f.size[i]
		}
		return o * f.nOut
	}
	i0 := int(math.Floor(idx[0]))
	frac := idx[0] - float64(i0)
	i1 := min(i0+1, f.size[0]-1)
	a, b := offset(i0), offset(i1)
	for j := 0; j < f.nOut && j < len(out); j++ {
		if a+j >= len(f.samples) || b+j >= len(f.samples) {
			out[j] = 0
			continue
		}
		out[j] = f.samples[a+j] + frac*(f.samples[b+j]-f.samples[a+j])
	}
}

// at returns v[i], or 0.
func at(v []float64, i int) float64 {
	if i < len(v) {
		return v[i]
	}
	return 0
}

// clampRange clamps v to [lo, hi]. NaN becomes lo.
func clampRange(v, lo, hi float64) float64 {
	if lo > hi {
		lo, hi = hi, lo
	}
	if !(v >= lo) {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
