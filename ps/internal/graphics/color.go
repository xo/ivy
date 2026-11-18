package graphics

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/ps/internal/interp"
)

// colorSpace is a color space (PLRM 4.8).
type colorSpace struct {
	family string
	// n is the number of the components of a color.
	n int
	// obj is the object that setcolorspace took, which currentcolorspace
	// gives back.
	obj interp.Object
	// base is the base of Indexed, the alternate of Separation, DeviceN and
	// ICCBased, and the space under an uncolored pattern.
	base *colorSpace
	// hival and lookup are the parameters of Indexed. A procedure of lookup
	// is in proc.
	hival  int
	lookup []byte
	// proc is the tint transform of Separation and DeviceN, or the lookup
	// procedure of Indexed.
	proc interp.Object
	// none is true for the colorant None, which paints nothing. all is true
	// for the colorant All.
	none, all bool
	// cie holds the parameters of CIEBasedA and CIEBasedABC.
	cie *cieSpace
	// ranges holds the range of each component of a CIE space: low and high
	// for each one.
	ranges []float64
	// cache keeps the colors that a procedure computed.
	cache map[[4]int32][3]float64
}

// The device color spaces.
var (
	deviceGray = &colorSpace{family: "DeviceGray", n: 1}
	deviceRGB  = &colorSpace{family: "DeviceRGB", n: 3}
	deviceCMYK = &colorSpace{family: "DeviceCMYK", n: 4}
)

// paint is a color of the graphics state: the components in a color space,
// and for the Pattern space, the pattern.
type paint struct {
	space   *colorSpace
	comps   []float64
	pattern *pattern
}

// maxCachedColors bounds the colors that a color space keeps.
const maxCachedColors = 1 << 16

// rgb returns the color of p in sRGB, and false for a color that paints
// nothing, such as the colorant None.
func (m *Machine) rgb(p paint) ([3]float64, bool) {
	return m.toRGB(p.space, p.comps, 0)
}

// toRGB returns the color of the components c of the space cs.
func (m *Machine) toRGB(cs *colorSpace, c []float64, depth int) ([3]float64, bool) {
	at := func(i int) float64 {
		if i < len(c) {
			return c[i]
		}
		return 0
	}
	if cs == nil || depth > 8 {
		return [3]float64{}, true
	}
	switch cs.family {
	case "DeviceGray", "CIEBasedA":
		if cs.cie != nil {
			return cs.cie.rgb(m, c), true
		}
		g := clamp01(at(0))
		return [3]float64{g, g, g}, true
	case "DeviceRGB":
		return [3]float64{clamp01(at(0)), clamp01(at(1)), clamp01(at(2))}, true
	case "CIEBasedABC", "CIEBasedDEF":
		if cs.cie != nil {
			return cs.cie.rgb(m, c), true
		}
		return [3]float64{clamp01(at(0)), clamp01(at(1)), clamp01(at(2))}, true
	case "DeviceCMYK":
		return cmykToRGB(at(0), at(1), at(2), at(3)), true
	case "CIEBasedDEFG":
		if cs.cie != nil {
			return cs.cie.rgb(m, c), true
		}
		return cmykToRGB(at(0), at(1), at(2), at(3)), true
	case "ICCBased":
		return m.toRGB(cs.base, c, depth+1)
	case "Indexed":
		return m.indexed(cs, at(0), depth)
	case "Separation", "DeviceN":
		if cs.none {
			return [3]float64{}, false
		}
		if cs.all {
			g := 1 - clamp01(at(0))
			return [3]float64{g, g, g}, true
		}
		return m.tint(cs, c, depth)
	case "Pattern":
		if cs.base != nil {
			return m.toRGB(cs.base, c, depth+1)
		}
	}
	return [3]float64{}, true
}

// cmykCache keeps the colors of DeviceCMYK that the profile converted, by
// their components in 8 bits.
var cmykCache sync.Map // [4]uint8 -> [3]float64

// maxCMYKCache bounds the colors of cmykCache.
const maxCMYKCache = 1 << 16

// cmykCount counts the colors of cmykCache.
var cmykCount atomic.Int64

// cmykToRGB converts a color of DeviceCMYK with the CMYK profile of the port
// of hayro, as the PDF reader does, through a cache.
func cmykToRGB(c, mg, y, k float64) [3]float64 {
	key := [4]uint8{to8(c), to8(mg), to8(y), to8(k)}
	if v, ok := cmykCache.Load(key); ok {
		return v.([3]float64)
	}
	col := interpret.DeviceCMYK().ToRGBA([]float32{float32(key[0]) / 255, float32(key[1]) / 255,
		float32(key[2]) / 255, float32(key[3]) / 255}, 1).Components()
	rgb := [3]float64{float64(col[0]), float64(col[1]), float64(col[2])}
	if cmykCount.Add(1) < maxCMYKCache {
		cmykCache.Store(key, rgb)
	}
	return rgb
}

// to8 returns a component from 0 to 1 in 8 bits.
func to8(v float64) uint8 {
	return uint8(math.Round(clamp01(v) * 255))
}

// indexed returns the color of an index of an Indexed space.
func (m *Machine) indexed(cs *colorSpace, v float64, depth int) ([3]float64, bool) {
	i := int(math.Round(v))
	i = min(max(i, 0), cs.hival)
	n := cs.base.n
	if cs.proc.Type() != interp.TNull {
		comps, ok := m.callProc(cs, []float64{float64(i)}, n)
		if !ok {
			return [3]float64{}, true
		}
		return m.toRGB(cs.base, comps, depth+1)
	}
	comps := make([]float64, n)
	for j := range n {
		k := i*n + j
		if k < len(cs.lookup) {
			comps[j] = float64(cs.lookup[k]) / 255
			if cs.base.ranges != nil && 2*j+1 < len(cs.base.ranges) {
				lo, hi := cs.base.ranges[2*j], cs.base.ranges[2*j+1]
				comps[j] = lo + comps[j]*(hi-lo)
			}
		}
	}
	return m.toRGB(cs.base, comps, depth+1)
}

// tint returns the color of the components of a Separation or a DeviceN
// space, through its tint transform.
func (m *Machine) tint(cs *colorSpace, c []float64, depth int) ([3]float64, bool) {
	var key [4]int32
	cacheable := len(c) <= 4
	if cacheable {
		for i, v := range c {
			key[i] = int32(math.Round(v * 4096))
		}
		if rgb, ok := cs.cache[key]; ok {
			return rgb, true
		}
	}
	comps, ok := m.callProc(cs, c, cs.base.n)
	if !ok {
		return [3]float64{}, true
	}
	rgb, ok := m.toRGB(cs.base, comps, depth+1)
	if cacheable && ok {
		if cs.cache == nil || len(cs.cache) >= maxCachedColors {
			cs.cache = map[[4]int32][3]float64{}
		}
		cs.cache[key] = rgb
	}
	return rgb, ok
}

// callProc runs the procedure of a color space on the components in, and
// returns the n numbers that it leaves.
func (m *Machine) callProc(cs *colorSpace, in []float64, n int) ([]float64, bool) {
	ip := m.in
	depth := ip.Count()
	for _, v := range in {
		ip.PushNum(v)
	}
	if err := ip.Call(cs.proc); err != nil {
		m.dropTo(depth)
		return nil, false
	}
	if ip.Count() < depth+n {
		m.dropTo(depth)
		return nil, false
	}
	out := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		o, _ := ip.Pop()
		out[i] = o.Float()
	}
	m.dropTo(depth)
	return out, true
}

// dropTo pops the operand stack down to a depth.
func (m *Machine) dropTo(depth int) {
	for m.in.Count() > depth {
		_, _ = m.in.Pop()
	}
}

// shader returns the shader of a paint, or nil for a paint that paints
// nothing. t is the transform from the space of the shader to the device
// space.
func (m *Machine) shader(p paint) okage.Shader {
	shader, _ := m.paintShader(p)
	return shader
}

// paintShader returns the shader of a paint, as shader does, and the path
// in the device space that a fill with the shader clips to, or nil. The
// path is the outline of a mesh. See meshGradient.
func (m *Machine) paintShader(p paint) (okage.Shader, *okage.Path) {
	if p.pattern != nil {
		return m.patternShader(p)
	}
	rgb, ok := m.rgb(p)
	if !ok {
		return nil, nil
	}
	return okage.SolidColor(okage.ColorFromRGBAUnchecked(float32(rgb[0]), float32(rgb[1]), float32(rgb[2]), 1)), nil
}

// clamp01 clamps v to [0, 1]. NaN becomes 0.
func clamp01(v float64) float64 {
	if !(v > 0) {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// hsbToRGB converts a color of HSB (PLRM 7.2.1).
func hsbToRGB(h, s, b float64) [3]float64 {
	h, s, b = clamp01(h), clamp01(s), clamp01(b)
	h6 := h * 6
	if h6 >= 6 {
		h6 = 0
	}
	i := math.Floor(h6)
	f := h6 - i
	p := b * (1 - s)
	q := b * (1 - s*f)
	t := b * (1 - s*(1-f))
	switch int(i) {
	case 0:
		return [3]float64{b, t, p}
	case 1:
		return [3]float64{q, b, p}
	case 2:
		return [3]float64{p, b, t}
	case 3:
		return [3]float64{p, q, b}
	case 4:
		return [3]float64{t, p, b}
	}
	return [3]float64{b, p, q}
}

// rgbToHSB converts a color of RGB to HSB.
func rgbToHSB(c [3]float64) [3]float64 {
	r, g, b := c[0], c[1], c[2]
	hi := math.Max(r, math.Max(g, b))
	lo := math.Min(r, math.Min(g, b))
	if hi == 0 {
		return [3]float64{0, 0, 0}
	}
	s := (hi - lo) / hi
	if s == 0 {
		return [3]float64{0, 0, hi}
	}
	var h float64
	d := hi - lo
	switch hi {
	case r:
		h = (g - b) / d
	case g:
		h = 2 + (b-r)/d
	default:
		h = 4 + (r-g)/d
	}
	h /= 6
	if h < 0 {
		h++
	}
	return [3]float64{h, s, hi}
}

// setPaint sets the color of the graphics state.
func (m *Machine) setPaint(cs *colorSpace, comps ...float64) {
	m.gs.color = paint{space: cs, comps: comps}
}

// opsColor are the operators of colors (PLRM 8.1).
func (m *Machine) opsColor() {
	in := m.in
	set := func(name string, cs *colorSpace) {
		in.Define(name, func(*interp.Interp) error {
			v, err := in.PopNums(cs.n)
			if err != nil {
				return err
			}
			for i := range v {
				v[i] = clamp01(v[i])
			}
			m.setPaint(cs, v...)
			return nil
		})
	}
	set("setgray", deviceGray)
	set("setrgbcolor", deviceRGB)
	set("setcmykcolor", deviceCMYK)
	in.Define("sethsbcolor", func(*interp.Interp) error {
		v, err := in.PopNums(3)
		if err != nil {
			return err
		}
		c := hsbToRGB(v[0], v[1], v[2])
		m.setPaint(deviceRGB, c[0], c[1], c[2])
		return nil
	})
	in.Define("currentgray", func(*interp.Interp) error {
		c := m.gs.color
		if c.space == deviceGray && len(c.comps) == 1 {
			in.PushNum(c.comps[0])
			return nil
		}
		rgb, _ := m.rgb(c)
		in.PushNum(0.3*rgb[0] + 0.59*rgb[1] + 0.11*rgb[2])
		return nil
	})
	in.Define("currentrgbcolor", func(*interp.Interp) error {
		c := m.gs.color
		if c.space == deviceRGB && len(c.comps) == 3 {
			for _, v := range c.comps {
				in.PushNum(v)
			}
			return nil
		}
		rgb, _ := m.rgb(c)
		for _, v := range rgb {
			in.PushNum(v)
		}
		return nil
	})
	in.Define("currenthsbcolor", func(*interp.Interp) error {
		rgb, _ := m.rgb(m.gs.color)
		for _, v := range rgbToHSB(rgb) {
			in.PushNum(v)
		}
		return nil
	})
	in.Define("currentcmykcolor", func(*interp.Interp) error {
		c := m.gs.color
		if c.space == deviceCMYK && len(c.comps) == 4 {
			for _, v := range c.comps {
				in.PushNum(v)
			}
			return nil
		}
		if c.space == deviceGray && len(c.comps) == 1 {
			for _, v := range []float64{0, 0, 0, 1 - c.comps[0]} {
				in.PushNum(v)
			}
			return nil
		}
		rgb, _ := m.rgb(c)
		for _, v := range []float64{1 - rgb[0], 1 - rgb[1], 1 - rgb[2], 0} {
			in.PushNum(v)
		}
		return nil
	})
	in.Define("setcolorspace", func(*interp.Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		cs, err := m.colorSpace(o, 0)
		if err != nil {
			return err
		}
		_, _ = in.Pop()
		m.gs.color = paint{space: cs, comps: initialColor(cs)}
		return nil
	})
	in.Define("currentcolorspace", func(*interp.Interp) error {
		cs := m.gs.color.space
		o := cs.obj
		if o.Type() == interp.TNull {
			n, err := in.NewArray([]interp.Object{interp.NameObj(cs.family)})
			if err != nil {
				return err
			}
			o = n
		}
		in.Push(o)
		return nil
	})
	in.Define("setcolor", func(*interp.Interp) error {
		cs := m.gs.color.space
		if cs.family == "Pattern" {
			return m.setPatternColor(cs)
		}
		v, err := in.PopNums(cs.n)
		if err != nil {
			return err
		}
		m.gs.color = paint{space: cs, comps: v}
		return nil
	})
	in.Define("currentcolor", func(*interp.Interp) error {
		c := m.gs.color
		for _, v := range c.comps {
			in.PushNum(v)
		}
		if c.pattern != nil {
			in.Push(c.pattern.dict)
		}
		return nil
	})
}

// initialColor returns the initial color of a color space (PLRM 4.8).
func initialColor(cs *colorSpace) []float64 {
	switch cs.family {
	case "DeviceCMYK":
		return []float64{0, 0, 0, 1}
	case "Separation", "DeviceN":
		c := make([]float64, cs.n)
		for i := range c {
			c[i] = 1
		}
		return c
	case "Pattern":
		return nil
	}
	c := make([]float64, cs.n)
	for i := range c {
		if 2*i < len(cs.ranges) {
			c[i] = max(0, cs.ranges[2*i])
			c[i] = min(c[i], cs.ranges[2*i+1])
		}
	}
	return c
}

// colorSpace reads a color space: a name or an array (PLRM 4.8).
func (m *Machine) colorSpace(o interp.Object, depth int) (*colorSpace, error) {
	if depth > 8 {
		return nil, interp.ErrLimitcheck
	}
	var family string
	var params []interp.Object
	switch {
	case o.Type() == interp.TName:
		family = o.NameVal()
	case o.IsArray() && o.Len() > 0:
		a := o.Array()
		if a[0].Type() != interp.TName {
			return nil, interp.ErrTypecheck
		}
		family, params = a[0].NameVal(), a[1:]
	default:
		return nil, interp.ErrTypecheck
	}
	switch family {
	case "DeviceGray", "G":
		return deviceGray, nil
	case "DeviceRGB", "RGB":
		return deviceRGB, nil
	case "DeviceCMYK", "CMYK":
		return deviceCMYK, nil
	case "Pattern":
		cs := &colorSpace{family: family, obj: o}
		if len(params) > 0 {
			base, err := m.colorSpace(params[0], depth+1)
			if err != nil {
				return nil, err
			}
			cs.base, cs.n = base, base.n
		}
		return cs, nil
	case "Indexed", "I":
		if len(params) < 3 {
			return nil, interp.ErrRangecheck
		}
		base, err := m.colorSpace(params[0], depth+1)
		if err != nil {
			return nil, err
		}
		if !params[1].IsNumber() {
			return nil, interp.ErrTypecheck
		}
		hival := int(params[1].Float())
		if hival < 0 || hival > 4095 {
			return nil, interp.ErrRangecheck
		}
		cs := &colorSpace{family: "Indexed", n: 1, obj: o, base: base, hival: hival, proc: interp.Null}
		switch l := params[2]; {
		case l.Type() == interp.TString:
			cs.lookup = l.Bytes()
		case l.IsArray():
			cs.proc = l
		default:
			return nil, interp.ErrTypecheck
		}
		return cs, nil
	case "Separation", "DeviceN":
		if len(params) < 3 {
			return nil, interp.ErrRangecheck
		}
		base, err := m.colorSpace(params[1], depth+1)
		if err != nil {
			return nil, err
		}
		cs := &colorSpace{family: family, n: 1, obj: o, base: base, proc: params[2]}
		if family == "DeviceN" {
			if !params[0].IsArray() {
				return nil, interp.ErrTypecheck
			}
			cs.n = params[0].Len()
			if cs.n < 1 || cs.n > 32 {
				return nil, interp.ErrRangecheck
			}
			none := true
			for _, c := range params[0].Array() {
				none = none && c.Text() == "None"
			}
			cs.none = none
		} else {
			switch params[0].Text() {
			case "None":
				cs.none = true
			case "All":
				cs.all = true
			}
		}
		if !cs.proc.IsArray() && cs.proc.Type() != interp.TOperator && !cs.none && !cs.all {
			return nil, interp.ErrTypecheck
		}
		return cs, nil
	case "ICCBased":
		if len(params) < 1 || params[0].Type() != interp.TDict {
			return nil, interp.ErrTypecheck
		}
		d := params[0].Dict()
		cs := &colorSpace{family: family, obj: o}
		if alt, ok := d.GetName("Alternate"); ok {
			if base, err := m.colorSpace(alt, depth+1); err == nil {
				cs.base = base
			}
		}
		n := 3
		if v, ok := d.GetName("N"); ok && v.IsNumber() {
			n = int(v.Float())
		}
		if cs.base == nil {
			switch n {
			case 1:
				cs.base = deviceGray
			case 4:
				cs.base = deviceCMYK
			default:
				cs.base = deviceRGB
			}
		}
		cs.n = cs.base.n
		return cs, nil
	case "CIEBasedA", "CIEBasedABC", "CIEBasedDEF", "CIEBasedDEFG":
		if len(params) < 1 || params[0].Type() != interp.TDict {
			return nil, interp.ErrTypecheck
		}
		return m.cieSpace(family, o, params[0].Dict()), nil
	case "DevicePixel":
		return deviceGray, nil
	}
	return nil, interp.ErrUndefined
}
