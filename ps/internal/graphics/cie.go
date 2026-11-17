package graphics

import (
	"math"

	"github.com/xo/ivy/ps/internal/interp"
)

// cieSpace holds the parameters of a CIE space (PLRM 4.8.3). A color of
// CIEBasedA or CIEBasedABC goes through DecodeABC, MatrixABC, DecodeLMN
// and MatrixLMN to CIE XYZ, and then to sRGB. A color of CIEBasedDEF or
// CIEBasedDEFG first goes through DecodeDEF or DecodeDEFG and the Table to
// the components of CIEBasedABC.
type cieSpace struct {
	// abc is true for CIEBasedABC, CIEBasedDEF and CIEBasedDEFG.
	abc bool
	// table holds the Table of CIEBasedDEF and CIEBasedDEFG, or nil.
	table *cieTable
	// decodeABC holds the procedures of DecodeABC, or DecodeA, or nulls.
	decodeABC [3]interp.Object
	matrixABC [9]float64
	decodeLMN [3]interp.Object
	matrixLMN [9]float64
	white     [3]float64
}

// cieTable is the Table of CIEBasedDEF or CIEBasedDEFG, which maps the
// components to the ones of CIEBasedABC.
type cieTable struct {
	// dims holds m1, m2, m3 and, for DEFG, m4.
	dims []int
	// data holds 3 bytes for each entry, with the last dimension the
	// fastest.
	data []byte
	// decode holds the procedures of DecodeDEF or DecodeDEFG, or nulls.
	decode [4]interp.Object
	// rng holds RangeHIJ or RangeHIJK.
	rng []float64
	// abcRange holds RangeABC.
	abcRange []float64
}

// maxCIETable bounds the bytes of a Table.
const maxCIETable = 1 << 24

// readTable reads the Table of a dictionary of CIEBasedDEF or
// CIEBasedDEFG, and returns nil when it is not valid.
func readTable(d *interp.Dict, n int) *cieTable {
	o, ok := d.GetName("Table")
	if !ok || !o.IsArray() || o.Len() != n+1 {
		return nil
	}
	a := o.Array()
	t := &cieTable{}
	total := 3
	for i := range n {
		if !a[i].IsNumber() {
			return nil
		}
		v := int(a[i].Float())
		if v < 1 || v > 1<<10 {
			return nil
		}
		t.dims = append(t.dims, v)
		total *= v
		if total > maxCIETable {
			return nil
		}
	}
	// DEF: an array of m1 strings. DEFG: an array of m1 arrays of m2
	// strings.
	var add func(o interp.Object, depth int)
	add = func(o interp.Object, depth int) {
		switch {
		case o.Type() == interp.TString:
			t.data = append(t.data, o.Bytes()...)
		case o.IsArray() && depth < 2:
			for _, e := range o.Array() {
				add(e, depth+1)
			}
		}
	}
	add(a[n], 0)
	if len(t.data) < total {
		return nil
	}
	for i := range 4 {
		t.decode[i] = interp.Null
	}
	if n == 3 {
		procs(d, "DecodeDEF", t.decode[:3])
		t.rng, _ = numbers(d, "RangeHIJ", 6)
	} else {
		procs(d, "DecodeDEFG", t.decode[:4])
		t.rng, _ = numbers(d, "RangeHIJK", 8)
	}
	if t.rng == nil {
		// the default of RangeHIJ and RangeHIJK
		for range n {
			t.rng = append(t.rng, 0, 1)
		}
	}
	t.abcRange, ok = numbers(d, "RangeABC", 6)
	if !ok {
		t.abcRange = []float64{0, 1, 0, 1, 0, 1}
	}
	return t
}

// lookup maps the components of DEF or DEFG to the ones of ABC.
func (t *cieTable) lookup(m *Machine, comps []float64) []float64 {
	n := len(t.dims)
	off := 0
	for i := range n {
		v := m.callNum(t.decode[i], at(comps, i))
		lo, hi := t.rng[2*i], t.rng[2*i+1]
		x := 0.0
		if hi != lo {
			x = (clampRange(v, lo, hi) - lo) / (hi - lo)
		}
		idx := int(math.Round(x * float64(t.dims[i]-1)))
		off = off*t.dims[i] + idx
	}
	off *= 3
	abc := make([]float64, 3)
	for i := range 3 {
		b := 0.0
		if off+i < len(t.data) {
			b = float64(t.data[off+i]) / 255
		}
		abc[i] = t.abcRange[2*i] + b*(t.abcRange[2*i+1]-t.abcRange[2*i])
	}
	return abc
}

// cieSpace reads the dictionary of a CIE space. A space DEF or DEFG
// without a valid Table draws as RGB or CMYK.
func (m *Machine) cieSpace(family string, o interp.Object, d *interp.Dict) *colorSpace {
	cs := &colorSpace{family: family, obj: o}
	rangeKey := map[string]string{
		"CIEBasedA": "RangeA", "CIEBasedABC": "RangeABC",
		"CIEBasedDEF": "RangeDEF", "CIEBasedDEFG": "RangeDEFG",
	}[family]
	cs.n = map[string]int{"CIEBasedA": 1, "CIEBasedABC": 3, "CIEBasedDEF": 3, "CIEBasedDEFG": 4}[family]
	cs.ranges = make([]float64, 0, 2*cs.n)
	if r, ok := numbers(d, rangeKey, 2*cs.n); ok {
		cs.ranges = r
	} else {
		for range cs.n {
			cs.ranges = append(cs.ranges, 0, 1)
		}
	}
	var table *cieTable
	switch family {
	case "CIEBasedDEF", "CIEBasedDEFG":
		if table = readTable(d, cs.n); table == nil {
			return cs
		}
	}
	c := &cieSpace{abc: family != "CIEBasedA", table: table}
	c.matrixABC = [9]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}
	c.matrixLMN = c.matrixABC
	c.white = [3]float64{0.9505, 1, 1.089}
	for i := range 3 {
		c.decodeABC[i], c.decodeLMN[i] = interp.Null, interp.Null
	}
	if c.abc {
		if v, ok := numbers(d, "MatrixABC", 9); ok {
			copy(c.matrixABC[:], v)
		}
		procs(d, "DecodeABC", c.decodeABC[:])
	} else {
		// MatrixA has 3 numbers: the A component scales each of L, M and N
		c.matrixABC = [9]float64{1, 1, 1, 0, 0, 0, 0, 0, 0}
		if v, ok := numbers(d, "MatrixA", 3); ok {
			copy(c.matrixABC[:3], v)
		}
		if p, ok := d.GetName("DecodeA"); ok && (p.IsArray() || p.Type() == interp.TOperator) {
			c.decodeABC[0] = p
		}
	}
	if v, ok := numbers(d, "MatrixLMN", 9); ok {
		copy(c.matrixLMN[:], v)
	}
	procs(d, "DecodeLMN", c.decodeLMN[:])
	if v, ok := numbers(d, "WhitePoint", 3); ok && v[1] > 0 {
		copy(c.white[:], v)
	}
	cs.cie = c
	return cs
}

// numbers returns the n numbers of the array of a key.
func numbers(d *interp.Dict, key string, n int) ([]float64, bool) {
	o, ok := d.GetName(key)
	if !ok || !o.IsArray() || o.Len() < n {
		return nil, false
	}
	v := make([]float64, n)
	for i, e := range o.Array()[:n] {
		if !e.IsNumber() {
			return nil, false
		}
		v[i] = e.Float()
	}
	return v, true
}

// procs fills dst with the procedures of the array of a key.
func procs(d *interp.Dict, key string, dst []interp.Object) {
	o, ok := d.GetName(key)
	if !ok || !o.IsArray() {
		return
	}
	for i, e := range o.Array() {
		if i < len(dst) && (e.IsArray() || e.Type() == interp.TOperator) {
			dst[i] = e
		}
	}
}

// rgb converts a color of the space to sRGB.
func (c *cieSpace) rgb(m *Machine, comps []float64) [3]float64 {
	if c.table != nil {
		comps = c.table.lookup(m, comps)
	}
	n := 1
	if c.abc {
		n = 3
	}
	var abc [3]float64
	for i := range n {
		if i < len(comps) {
			abc[i] = comps[i]
		}
		abc[i] = m.callNum(c.decodeABC[i], abc[i])
	}
	var lmn [3]float64
	if c.abc {
		for j := range 3 {
			lmn[j] = abc[0]*c.matrixABC[j] + abc[1]*c.matrixABC[3+j] + abc[2]*c.matrixABC[6+j]
		}
	} else {
		for j := range 3 {
			lmn[j] = abc[0] * c.matrixABC[j]
		}
	}
	for i := range 3 {
		lmn[i] = m.callNum(c.decodeLMN[i], lmn[i])
	}
	var xyz [3]float64
	for j := range 3 {
		xyz[j] = lmn[0]*c.matrixLMN[j] + lmn[1]*c.matrixLMN[3+j] + lmn[2]*c.matrixLMN[6+j]
	}
	// adapt the white point to the white of sRGB, by scaling
	white := [3]float64{0.9505, 1, 1.089}
	for i := range 3 {
		if c.white[i] > 0 {
			xyz[i] *= white[i] / c.white[i]
		}
	}
	return xyzToSRGB(xyz)
}

// callNum runs a procedure on one number, and returns the number that it
// leaves. A null procedure returns v.
func (m *Machine) callNum(proc interp.Object, v float64) float64 {
	if proc.Type() == interp.TNull {
		return v
	}
	cs := &colorSpace{proc: proc}
	out, ok := m.callProc(cs, []float64{v}, 1)
	if !ok {
		return v
	}
	return out[0]
}

// xyzToSRGB converts CIE XYZ under the white of sRGB to sRGB.
func xyzToSRGB(xyz [3]float64) [3]float64 {
	x, y, z := xyz[0], xyz[1], xyz[2]
	lin := [3]float64{
		3.2406*x - 1.5372*y - 0.4986*z,
		-0.9689*x + 1.8758*y + 0.0415*z,
		0.0557*x - 0.2040*y + 1.0570*z,
	}
	var out [3]float64
	for i, v := range lin {
		v = clamp01(v)
		if v <= 0.0031308 {
			out[i] = 12.92 * v
		} else {
			out[i] = 1.055*math.Pow(v, 1/2.4) - 0.055
		}
	}
	return out
}
