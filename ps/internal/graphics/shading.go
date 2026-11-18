package graphics

import (
	"image"
	"io"
	"math"
	"slices"
	"sort"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/ps/internal/interp"
)

// maxShadingSide bounds the side of the image of a shading, in pixels.
const maxShadingSide = 1600

// maxMeshVertices bounds the vertices of a mesh.
const maxMeshVertices = 1 << 20

// shading is a shading dictionary (PLRM 4.9.3).
type shading struct {
	typ    int
	space  *colorSpace
	fn     *function
	bg     []float64
	bbox   []float64
	domain []float64
	matrix Matrix
	coords []float64
	extend [2]bool
	// tris holds the triangles of a mesh, three vertices each.
	tris []meshVertex
}

// meshVertex is a vertex of a mesh: its point in the shading space, and
// its color components, or the parameter t of the function.
type meshVertex struct {
	x, y float64
	c    []float64
}

// parseShading reads a shading dictionary.
func (m *Machine) parseShading(o interp.Object) (*shading, error) {
	if o.Type() != interp.TDict {
		return nil, interp.ErrTypecheck
	}
	d := o.Dict()
	s := &shading{matrix: identity}
	v, ok := d.GetName("ShadingType")
	if !ok || !v.IsNumber() {
		return nil, interp.ErrRangecheck
	}
	s.typ = int(v.Float())
	cso, ok := d.GetName("ColorSpace")
	if !ok {
		return nil, interp.ErrRangecheck
	}
	cs, err := m.colorSpace(cso, 0)
	if err != nil {
		return nil, err
	}
	s.space = cs
	if fo, ok := d.GetName("Function"); ok {
		if s.fn, err = m.parseFunction(fo, 0); err != nil {
			return nil, err
		}
	}
	if v, ok := d.GetName("Background"); ok && v.IsArray() {
		s.bg = arrayNumbers(v)
	}
	if v, ok := d.GetName("BBox"); ok && v.IsArray() && v.Len() == 4 {
		s.bbox = arrayNumbers(v)
	}
	if v, ok := d.GetName("Domain"); ok && v.IsArray() {
		s.domain = arrayNumbers(v)
	}
	if v, ok := d.GetName("Coords"); ok && v.IsArray() {
		s.coords = arrayNumbers(v)
	}
	if v, ok := d.GetName("Extend"); ok && v.IsArray() && v.Len() == 2 {
		s.extend = [2]bool{v.Array()[0].BoolVal(), v.Array()[1].BoolVal()}
	}
	if v, ok := d.GetName("Matrix"); ok {
		if mat, err := matrixOf(v); err == nil {
			s.matrix = mat
		}
	}
	switch s.typ {
	case 1:
		if len(s.domain) < 4 {
			s.domain = []float64{0, 1, 0, 1}
		}
		if s.fn == nil {
			return nil, interp.ErrRangecheck
		}
	case 2, 3:
		if len(s.domain) < 2 {
			s.domain = []float64{0, 1}
		}
		want := 4
		if s.typ == 3 {
			want = 6
		}
		if len(s.coords) < want || s.fn == nil {
			return nil, interp.ErrRangecheck
		}
	case 4, 5, 6, 7:
		if err := m.parseMesh(s, d); err != nil {
			return nil, err
		}
	default:
		return nil, interp.ErrRangecheck
	}
	return s, nil
}

// meshReader reads the values of a mesh: packed bits of a string or a
// file, or the numbers of an array.
type meshReader struct {
	nums   []interp.Object
	data   []byte
	bitPos int
}

// bits reads n bits, and false at the end.
func (r *meshReader) bits(n int) (uint64, bool) {
	if r.nums != nil {
		return 0, false
	}
	if r.bitPos+n > len(r.data)*8 {
		return 0, false
	}
	var v uint64
	for range n {
		b := r.data[r.bitPos/8] >> (7 - r.bitPos%8) & 1
		v = v<<1 | uint64(b)
		r.bitPos++
	}
	return v, true
}

// align moves to the next byte.
func (r *meshReader) align() {
	r.bitPos = (r.bitPos + 7) / 8 * 8
}

// value reads one value of n bits decoded to [lo, hi], or one number of an
// array.
func (r *meshReader) value(n int, lo, hi float64) (float64, bool) {
	if r.nums != nil {
		if len(r.nums) == 0 {
			return 0, false
		}
		v := r.nums[0].Float()
		r.nums = r.nums[1:]
		return v, true
	}
	b, ok := r.bits(n)
	if !ok {
		return 0, false
	}
	maxV := math.Pow(2, float64(n)) - 1
	return lo + float64(b)*(hi-lo)/maxV, true
}

// parseMesh reads the vertices of a shading of type 4 to 7.
func (m *Machine) parseMesh(s *shading, d *interp.Dict) error {
	geti := func(key string) int {
		if v, ok := d.GetName(key); ok && v.IsNumber() {
			return int(v.Float())
		}
		return 0
	}
	src, ok := d.GetName("DataSource")
	if !ok {
		return interp.ErrRangecheck
	}
	r := &meshReader{}
	switch {
	case src.IsArray():
		r.nums = src.Array()
	case src.Type() == interp.TString:
		r.data = src.Bytes()
	default:
		rd, err := m.in.NewDataSource(src)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(rd, m.lim.MaxMemory))
		if err != nil {
			return interp.ErrIO
		}
		r.data = data
	}
	bpc, bpcomp, bpf := geti("BitsPerCoordinate"), geti("BitsPerComponent"), geti("BitsPerFlag")
	nc := s.space.n
	if s.fn != nil {
		nc = 1
	}
	var decode []float64
	if v, ok := d.GetName("Decode"); ok && v.IsArray() {
		decode = arrayNumbers(v)
	}
	if r.nums == nil {
		if bpc < 1 || bpc > 32 || bpcomp < 1 || bpcomp > 16 || len(decode) < 4+2*nc {
			return interp.ErrRangecheck
		}
	} else if len(decode) < 4+2*nc {
		decode = make([]float64, 4+2*nc)
	}
	vertex := func() (meshVertex, bool) {
		var v meshVertex
		var ok bool
		if v.x, ok = r.value(bpc, decode[0], decode[1]); !ok {
			return v, false
		}
		if v.y, ok = r.value(bpc, decode[2], decode[3]); !ok {
			return v, false
		}
		v.c = make([]float64, nc)
		for i := range nc {
			if v.c[i], ok = r.value(bpcomp, decode[4+2*i], decode[5+2*i]); !ok {
				return v, false
			}
		}
		return v, true
	}
	flag := func() (int, bool) {
		if r.nums != nil {
			f, ok := r.value(0, 0, 0)
			return int(f), ok
		}
		f, ok := r.bits(bpf)
		return int(f), ok
	}
	add := func(a, b, c meshVertex) bool {
		if len(s.tris) >= maxMeshVertices {
			return false
		}
		s.tris = append(s.tris, a, b, c)
		return true
	}
	switch s.typ {
	case 4:
		var a, b, c meshVertex
		have := false
		for {
			f, ok := flag()
			if !ok {
				break
			}
			v, ok := vertex()
			if !ok {
				break
			}
			if r.nums == nil {
				r.align()
			}
			switch {
			case f == 0 || !have:
				w1, ok1 := vertexAligned(r, flag, vertex)
				w2, ok2 := vertexAligned(r, flag, vertex)
				if !ok1 || !ok2 {
					return nil
				}
				a, b, c, have = v, w1, w2, true
			case f == 1:
				a, b, c = b, c, v
			default:
				b, c = c, v
			}
			if !add(a, b, c) {
				return nil
			}
		}
	case 5:
		per := geti("VerticesPerRow")
		if per < 2 {
			return interp.ErrRangecheck
		}
		var prev []meshVertex
		for {
			row := make([]meshVertex, 0, per)
			for range per {
				v, ok := vertex()
				if !ok {
					return nil
				}
				row = append(row, v)
			}
			if prev != nil {
				for i := 0; i+1 < per; i++ {
					if !add(prev[i], prev[i+1], row[i]) || !add(prev[i+1], row[i+1], row[i]) {
						return nil
					}
				}
			}
			prev = row
			if r.nums == nil {
				r.align()
			}
		}
	case 6, 7:
		npts := 12
		if s.typ == 7 {
			npts = 16
		}
		var last [16]meshVertex
		var lastColors [4][]float64
		have := false
		for {
			f, ok := flag()
			if !ok {
				return nil
			}
			var pts [16]meshVertex
			var colors [4][]float64
			read := npts
			ncol := 4
			start := 0
			if f != 0 && have {
				read, ncol, start = npts-4, 2, 4
			}
			for i := start; i < start+read; i++ {
				var ok bool
				if pts[i].x, ok = r.value(bpc, decode[0], decode[1]); !ok {
					return nil
				}
				if pts[i].y, ok = r.value(bpc, decode[2], decode[3]); !ok {
					return nil
				}
			}
			for i := 4 - ncol; i < 4; i++ {
				colors[i] = make([]float64, nc)
				for k := range nc {
					var ok bool
					if colors[i][k], ok = r.value(bpcomp, decode[4+2*k], decode[5+2*k]); !ok {
						return nil
					}
				}
			}
			if r.nums == nil {
				r.align()
			}
			if f != 0 && have {
				// the shared edge of the last patch (PLRM 4.9.3)
				var idx [4]int
				var cidx [2]int
				switch f {
				case 1:
					idx, cidx = [4]int{3, 4, 5, 6}, [2]int{1, 2}
				case 2:
					idx, cidx = [4]int{6, 7, 8, 9}, [2]int{2, 3}
				default:
					idx, cidx = [4]int{9, 10, 11, 0}, [2]int{3, 0}
				}
				for i := range 4 {
					pts[i] = last[idx[i]]
				}
				colors[0], colors[1] = lastColors[cidx[0]], lastColors[cidx[1]]
			}
			last, lastColors, have = pts, colors, true
			if !m.patchTriangles(pts, colors, npts == 16, add) {
				return nil
			}
		}
	}
	return nil
}

// vertexAligned reads the flag and the vertex of the next vertex of a
// free-form mesh.
func vertexAligned(r *meshReader, flag func() (int, bool), vertex func() (meshVertex, bool)) (meshVertex, bool) {
	if _, ok := flag(); !ok {
		return meshVertex{}, false
	}
	v, ok := vertex()
	if r.nums == nil {
		r.align()
	}
	return v, ok
}

// patchTriangles divides a Coons or tensor-product patch into triangles.
// pts holds the points of the boundary in the order of PLRM 4.9.3, and for
// a tensor-product patch, the four inner points.
func (m *Machine) patchTriangles(pts [16]meshVertex, colors [4][]float64, tensor bool, add func(a, b, c meshVertex) bool) bool {
	// the 4x4 grid of control points, p[u][v]
	var p [4][4][2]float64
	set := func(u, v, i int) { p[u][v] = [2]float64{pts[i].x, pts[i].y} }
	// boundary: 0..3 up the left side (u=0, v=0..3), then along the top
	set(0, 0, 0)
	set(0, 1, 1)
	set(0, 2, 2)
	set(0, 3, 3)
	set(1, 3, 4)
	set(2, 3, 5)
	set(3, 3, 6)
	set(3, 2, 7)
	set(3, 1, 8)
	set(3, 0, 9)
	set(2, 0, 10)
	set(1, 0, 11)
	if tensor {
		set(1, 1, 12)
		set(1, 2, 13)
		set(2, 2, 14)
		set(2, 1, 15)
	} else {
		inner := func(u, v int) [2]float64 {
			var out [2]float64
			for k := range 2 {
				out[k] = (-4*p[u][v][k] +
					6*(p[u][map[int]int{0: 1, 3: 2}[v]][k]+p[map[int]int{0: 1, 3: 2}[u]][v][k]) -
					2*(p[u][3-v][k]+p[3-u][v][k]) +
					3*(p[3-u][map[int]int{0: 1, 3: 2}[v]][k]+p[map[int]int{0: 1, 3: 2}[u]][3-v][k]) -
					p[3-u][3-v][k]) / 9
			}
			return out
		}
		p[1][1] = inner(0, 0)
		p[1][2] = inner(0, 3)
		p[2][2] = inner(3, 3)
		p[2][1] = inner(3, 0)
	}
	// the colors of the corners: c00, c01, c11, c10 in the order of the
	// boundary
	c00, c01, c11, c10 := colors[0], colors[1], colors[2], colors[3]
	const n = 12
	bern := func(t float64) [4]float64 {
		u := 1 - t
		return [4]float64{u * u * u, 3 * u * u * t, 3 * u * t * t, t * t * t}
	}
	var grid [n + 1][n + 1]meshVertex
	for i := 0; i <= n; i++ {
		bu := bern(float64(i) / n)
		for j := 0; j <= n; j++ {
			bv := bern(float64(j) / n)
			var x, y float64
			for a := range 4 {
				for b := range 4 {
					w := bu[a] * bv[b]
					x += w * p[a][b][0]
					y += w * p[a][b][1]
				}
			}
			u, v := float64(i)/n, float64(j)/n
			c := make([]float64, len(c00))
			for k := range c {
				c[k] = (1-u)*(1-v)*at(c00, k) + (1-u)*v*at(c01, k) + u*v*at(c11, k) + u*(1-v)*at(c10, k)
			}
			grid[i][j] = meshVertex{x: x, y: y, c: c}
		}
	}
	for i := range n {
		for j := range n {
			if !add(grid[i][j], grid[i+1][j], grid[i+1][j+1]) || !add(grid[i][j], grid[i+1][j+1], grid[i][j+1]) {
				return false
			}
		}
	}
	return true
}

// shadingColor returns the color of the components or of the parameter of the
// function of a shading.
func (m *Machine) shadingColor(s *shading, c []float64) ([3]float64, bool) {
	if s.fn != nil {
		out := make([]float64, max(s.fn.nOut, s.space.n))
		s.fn.eval(c, out)
		return m.toRGB(s.space, out, 0)
	}
	return m.toRGB(s.space, c, 0)
}

// shadingImage draws a shading of type 1, 2 or 3 into an image that covers
// the rectangle r of the device space. toDevice maps the shading space onto
// the device space. background paints the Background of the shading
// outside of it. It returns the image and the transform from its pixels to
// the device space. okage fills a mesh itself, with meshGradient.
func (m *Machine) shadingImage(s *shading, toDevice Matrix, r [4]float64, background bool) (*image.RGBA, Matrix, bool) {
	w, h := r[2]-r[0], r[3]-r[1]
	if !(w > 0 && h > 0) {
		return nil, Matrix{}, false
	}
	k := math.Min(1, maxShadingSide/math.Max(w, h))
	pw, ph := int(math.Ceil(w*k)), int(math.Ceil(h*k))
	if pw < 1 || ph < 1 {
		return nil, Matrix{}, false
	}
	pixToDev := Matrix{1 / k, 0, 0, 1 / k, r[0], r[1]}
	img := image.NewRGBA(image.Rect(0, 0, pw, ph))
	inv, ok := toDevice.Invert()
	if !ok {
		return nil, Matrix{}, false
	}
	pixToShading := pixToDev.Mul(inv)
	var bg [3]float64
	hasBG := false
	if background && len(s.bg) >= s.space.n {
		bg, hasBG = m.toRGB(s.space, s.bg, 0)
	}
	put := func(x, y int, c [3]float64) {
		o := y*img.Stride + 4*x
		img.Pix[o] = uint8(math.Round(clamp01(c[0]) * 255))
		img.Pix[o+1] = uint8(math.Round(clamp01(c[1]) * 255))
		img.Pix[o+2] = uint8(math.Round(clamp01(c[2]) * 255))
		img.Pix[o+3] = 255
	}
	if hasBG {
		for y := range ph {
			for x := range pw {
				put(x, y, bg)
			}
		}
	}
	switch s.typ {
	case 1:
		minv, ok := s.matrix.Invert()
		if !ok {
			return nil, Matrix{}, false
		}
		toDomain := pixToShading.Mul(minv)
		for y := range ph {
			if y&31 == 0 && m.in.Context().Err() != nil {
				return nil, Matrix{}, false
			}
			for x := range pw {
				dx, dy := toDomain.Apply(float64(x)+0.5, float64(y)+0.5)
				if dx < s.domain[0] || dx > s.domain[1] || dy < s.domain[2] || dy > s.domain[3] {
					continue
				}
				if c, ok := m.shadingColor(s, []float64{dx, dy}); ok {
					put(x, y, c)
				}
			}
		}
	case 2, 3:
		lut := m.shadingLUT(s)
		for y := range ph {
			if y&31 == 0 && m.in.Context().Err() != nil {
				return nil, Matrix{}, false
			}
			for x := range pw {
				sx, sy := pixToShading.Apply(float64(x)+0.5, float64(y)+0.5)
				var u float64
				var ok bool
				if s.typ == 2 {
					u, ok = axialParam(s, sx, sy)
				} else {
					u, ok = radialParam(s, sx, sy)
				}
				if !ok {
					continue
				}
				i := int(math.Round(u * float64(len(lut)-1)))
				i = min(max(i, 0), len(lut)-1)
				if lut[i].ok {
					put(x, y, lut[i].c)
				}
			}
		}
	}
	return img, pixToDev, true
}

// lutEntry is a color of the table of an axial or a radial shading.
type lutEntry struct {
	c  [3]float64
	ok bool
}

// shadingLUT returns the colors of 1024 values of the parameter of an axial
// or a radial shading, from the start of the domain to its end.
func (m *Machine) shadingLUT(s *shading) []lutEntry {
	lut := make([]lutEntry, 1024)
	for i := range lut {
		t := s.domain[0] + float64(i)/float64(len(lut)-1)*(s.domain[1]-s.domain[0])
		lut[i].c, lut[i].ok = m.shadingColor(s, []float64{t})
	}
	return lut
}

// axialParam returns the parameter, from 0 to 1, of a point of an axial
// shading, and false outside of it.
func axialParam(s *shading, x, y float64) (float64, bool) {
	x0, y0, x1, y1 := s.coords[0], s.coords[1], s.coords[2], s.coords[3]
	dx, dy := x1-x0, y1-y0
	d := dx*dx + dy*dy
	if d == 0 {
		return 0, false
	}
	t := ((x-x0)*dx + (y-y0)*dy) / d
	switch {
	case t < 0:
		return 0, s.extend[0]
	case t > 1:
		return 1, s.extend[1]
	}
	return t, true
}

// radialParam returns the parameter, from 0 to 1, of a point of a radial
// shading, and false outside of it.
func radialParam(s *shading, x, y float64) (float64, bool) {
	cx0, cy0, r0, cx1, cy1, r1 := s.coords[0], s.coords[1], s.coords[2], s.coords[3], s.coords[4], s.coords[5]
	cdx, cdy, dr := cx1-cx0, cy1-cy0, r1-r0
	pdx, pdy := x-cx0, y-cy0
	a := cdx*cdx + cdy*cdy - dr*dr
	b := pdx*cdx + pdy*cdy + r0*dr
	c := pdx*pdx + pdy*pdy - r0*r0
	try := func(t float64) (float64, bool) {
		if r0+t*dr < 0 {
			return 0, false
		}
		switch {
		case t >= 0 && t <= 1:
			return t, true
		case t < 0 && s.extend[0]:
			return 0, true
		case t > 1 && s.extend[1]:
			return 1, true
		}
		return 0, false
	}
	if math.Abs(a) < 1e-12 {
		if b == 0 {
			return 0, false
		}
		return try(c / (2 * b))
	}
	disc := b*b - a*c
	if disc < 0 {
		return 0, false
	}
	sq := math.Sqrt(disc)
	t1, t2 := (b+sq)/a, (b-sq)/a
	if t1 < t2 {
		t1, t2 = t2, t1
	}
	if t, ok := try(t1); ok {
		return t, true
	}
	return try(t2)
}

// deviceArea returns the rectangle of the device space that a mark can
// cover: the page, within the bounds of the clip.
func (m *Machine) deviceArea() [4]float64 {
	r := [4]float64{0, 0, m.width * resolutionScale, m.height * resolutionScale}
	if c := m.gs.clip; c != nil {
		r = [4]float64{math.Max(r[0], c.bbox[0]), math.Max(r[1], c.bbox[1]), math.Min(r[2], c.bbox[2]), math.Min(r[3], c.bbox[3])}
	}
	return r
}

// shadingBounds returns the bounds of a shading of a mesh in the device
// space, or the area for any other.
func (m *Machine) shadingBounds(s *shading, toDevice Matrix, area [4]float64) [4]float64 {
	if len(s.tris) == 0 {
		return area
	}
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, v := range s.tris {
		x, y := toDevice.Apply(v.x, v.y)
		x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x), math.Max(y1, y)
	}
	return [4]float64{math.Max(x0, area[0]), math.Max(y0, area[1]), math.Min(x1, area[2]), math.Min(y1, area[3])}
}

// shfill paints a shading over the clip, as shfill does. okage fills a
// mesh, an axial shading and most radial shadings itself. shfill draws any
// other shading as an image. It fills a mesh through the outline of its
// triangles, which gives the antialiasing at the free edges of the mesh.
func (m *Machine) shfill(s *shading) error {
	if m.capture != nil && m.capture.intercept() {
		return nil
	}
	toDevice := m.gs.ctm
	area := m.shadingBounds(s, toDevice, m.deviceArea())
	var paint func()
	if shader, outline, native := m.nativeShader(s, toDevice, false); native {
		// The rectangle covers whole pixels, so that only the clip gives the
		// antialiasing at its edges.
		r, ok := okage.RectFromLTRB(float32(math.Floor(area[0])), float32(math.Floor(area[1])), float32(math.Ceil(area[2])), float32(math.Ceil(area[3])))
		if shader == nil || !ok {
			return nil
		}
		fill := okage.PathFromRect(r)
		if outline != nil {
			fill = outline
		}
		paint = func() { m.dev.FillPath(fill, okage.FillRuleWinding, shader) }
	} else {
		img, t, ok := m.shadingImage(s, toDevice, area, false)
		if !ok {
			return nil
		}
		paint = func() { m.dev.DrawImage(img, toTransform(t), display.ImageOptions{Opacity: 1, Smooth: true}) }
	}
	m.mark()
	pushed := false
	if len(s.bbox) == 4 {
		var p Path
		b := s.bbox
		p.moveTo(m.device(b[0], b[1]))
		p.lineTo(m.device(b[2], b[1]))
		p.lineTo(m.device(b[2], b[3]))
		p.lineTo(m.device(b[0], b[3]))
		p.closePath()
		m.dev.Save()
		m.dev.ClipPath(okagePath(&p), okage.FillRuleWinding)
		pushed = true
	}
	paint()
	if pushed {
		m.dev.Restore()
	}
	return nil
}

// shadingShader returns the shader of a shading pattern, over the area of
// the clip, and the outline of a mesh in the device space, or nil. A fill
// with the shader must clip to the outline. See meshGradient.
func (m *Machine) shadingShader(pat *pattern) (okage.Shader, *okage.Path) {
	s := pat.shading
	if shader, outline, native := m.nativeShader(s, pat.matrix, true); native {
		return shader, outline
	}
	area := m.shadingBounds(s, pat.matrix, m.deviceArea())
	img, t, ok := m.shadingImage(s, pat.matrix, area, true)
	if !ok {
		return nil, nil
	}
	pix, err := raster.Pixmap(img)
	if err != nil {
		return nil, nil
	}
	return okage.NewPattern(pix.AsRef(), okage.SpreadModePad, okage.FilterQualityBilinear, 1, toTransform(t)), nil
}

// maxMeshTriangles bounds the triangles that a mesh gives to okage, with
// the triangles that the stops of its function cut. okage keeps about 250
// bytes for each triangle.
const maxMeshTriangles = 1 << 18

// maxMeshParts bounds the parts that meshBuilder cuts for one mesh.
const maxMeshParts = 1 << 22

// The limits of the stops that functionStops places.
const (
	// stopTolerance is the largest difference of a component, from 0 to 1,
	// between the function and the line between two stops.
	stopTolerance = 0.01
	// maxStopDepth is the number of times that functionStops can halve an
	// interval.
	maxStopDepth = 8
	// stopIntervals is the number of intervals that functionStops starts
	// with, so that it does not miss a narrow change of the function.
	stopIntervals = 4
)

// nativeShader returns the shader of okage of a shading in the device
// space, when okage fills the shading itself: a mesh of type 4 to 7, or an
// axial or a radial shading that gradientShader takes. toDevice maps the
// shading space onto the device space. background paints the Background of
// the shading under a mesh, over the area of the clip. For a mesh, it also
// returns the outline of the triangles in the device space. It returns
// false for a shading that shadingImage draws, and a nil shader for a mesh
// that paints nothing.
func (m *Machine) nativeShader(s *shading, toDevice Matrix, background bool) (okage.Shader, *okage.Path, bool) {
	switch s.typ {
	case 2, 3:
		shader, ok := m.gradientShader(s, toDevice, background)
		return shader, nil, ok
	case 4, 5, 6, 7:
		shader, outline := m.meshGradient(s, toDevice, background)
		return shader, outline, true
	}
	return nil, nil, false
}

// gradientShader returns the gradient of okage of an axial or a radial
// shading. A shading with no extend has the spread mode
// okage.SpreadModeDecal, which paints nothing past its ends (okage D55). Any
// other has the spread mode okage.SpreadModePad, and an end with no extend
// gets a transparent stop at the same place as its last color, so that the
// gradient paints nothing past that end.
//
// gradientShader returns false for a pattern with a Background, and for
// some radial shadings whose circles do not nest. PostScript paints a point
// with the largest circle that holds it in the domain and its extends.
// okage paints it with the largest circle that holds it. The two agree when
// the circles nest, because then one circle holds the point, and when the
// shading extends past its end circle.
func (m *Machine) gradientShader(s *shading, toDevice Matrix, background bool) (okage.Shader, bool) {
	if background && len(s.bg) >= s.space.n {
		return nil, false
	}
	c := s.coords
	if s.typ == 3 {
		nested := math.Abs(c[5]-c[2])-math.Hypot(c[3]-c[0], c[4]-c[1]) > 1e-6
		if !(c[2] >= 0 && c[5] >= 0) || !nested && !s.extend[1] {
			return nil, false
		}
	}
	stops := m.functionStops(s, s.domain[0], s.domain[1])
	gradientStops := make([]okage.GradientStop, 0, len(stops)+2)
	if s.extend[1] && !s.extend[0] {
		gradientStops = append(gradientStops, okage.NewGradientStop(float32(stops[0].u), okage.Color{}))
	}
	for _, st := range stops {
		gradientStops = append(gradientStops, okage.NewGradientStop(float32(st.u), okageColor(st.c)))
	}
	if s.extend[0] && !s.extend[1] {
		gradientStops = append(gradientStops, okage.NewGradientStop(float32(stops[len(stops)-1].u), okage.Color{}))
	}
	spread := okage.SpreadModePad
	if !s.extend[0] && !s.extend[1] {
		spread = okage.SpreadModeDecal
	}
	pt := func(x, y float64) okage.Point { return okage.PointFromXY(float32(x), float32(y)) }
	if s.typ == 2 {
		return okage.NewLinearGradient(pt(c[0], c[1]), pt(c[2], c[3]), gradientStops, spread, toTransform(toDevice))
	}
	return okage.NewRadialGradient(pt(c[0], c[1]), float32(c[2]), pt(c[3], c[4]), float32(c[5]), gradientStops, spread, toTransform(toDevice))
}

// meshGradient returns the MeshGradient of okage of a mesh, a shading of
// type 4 to 7 (okage D55), and the outline of its triangles in the device
// space. It returns nil when the mesh holds no triangle. toDevice and
// background are as for nativeShader.
//
// A function of the colors is not linear in a triangle. meshGradient thus
// cuts each triangle at the stops of the function, where the color goes
// from one linear segment to the next.
//
// okage paints a pixel at a free edge of a mesh in full. A fill of the mesh
// must thus clip to the outline, which gives the antialiasing at that edge,
// so that the mesh does not grow by a pixel.
func (m *Machine) meshGradient(s *shading, toDevice Matrix, background bool) (okage.Shader, *okage.Path) {
	inv, ok := toDevice.Invert()
	if !ok {
		return nil, nil
	}
	b := &meshBuilder{}
	if background && len(s.bg) >= s.space.n {
		if c, ok := m.toRGB(s.space, s.bg, 0); ok {
			// the corners of the area of the clip, in the shading space
			area := m.deviceArea()
			x0, y0 := inv.Apply(area[0], area[1])
			x1, y1 := inv.Apply(area[2], area[1])
			x2, y2 := inv.Apply(area[0], area[3])
			x3, y3 := inv.Apply(area[2], area[3])
			bg := [4]float64{c[0], c[1], c[2], 1}
			b.add([3]meshCut{{x0, y0, 0}, {x1, y1, 0}, {x2, y2, 0}}, func(float64) [4]float64 { return bg })
			b.add([3]meshCut{{x1, y1, 0}, {x3, y3, 0}, {x2, y2, 0}}, func(float64) [4]float64 { return bg })
		}
	}
	if s.fn == nil {
		linear := s.space.cie == nil && (s.space.family == "DeviceGray" || s.space.family == "DeviceRGB")
		for i := 0; i+2 < len(s.tris) && len(b.tris) < maxMeshTriangles; i += 3 {
			if i&1023 == 0 && m.in.Context().Err() != nil {
				return nil, nil
			}
			n := 1
			if !linear {
				n = splits(s.tris[i:i+3], toDevice)
			}
			m.splitTriangle(b, s, [3]meshVertex(s.tris[i:i+3]), n)
		}
	} else if len(s.tris) > 0 {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, v := range s.tris {
			lo, hi = math.Min(lo, at(v.c, 0)), math.Max(hi, at(v.c, 0))
		}
		stops := m.functionStops(s, lo, hi)
		scale := 0.0
		if hi > lo {
			scale = 1 / (hi - lo)
		}
		for i := 0; i+2 < len(s.tris) && len(b.tris) < maxMeshTriangles; i += 3 {
			if i&1023 == 0 && m.in.Context().Err() != nil {
				return nil, nil
			}
			var tri [3]meshCut
			for j := range tri {
				v := s.tris[i+j]
				tri[j] = meshCut{x: v.x, y: v.y, u: (at(v.c, 0) - lo) * scale}
			}
			b.cut(tri, stops)
		}
	}
	shader, ok := okage.NewMeshGradient(b.tris, toTransform(toDevice))
	if !ok {
		return nil, nil
	}
	return shader, meshOutline(b.tris, toDevice)
}

// meshOutline returns the outline of the triangles of a mesh in the device
// space, or nil when no triangle has an area. Each triangle turns the same
// way, so that the nonzero rule fills the union of the triangles.
func meshOutline(tris []okage.MeshTriangle, toDevice Matrix) *okage.Path {
	pb := okage.NewPathBuilder()
	for _, t := range tris {
		var x, y [3]float64
		for i, v := range t {
			x[i], y[i] = toDevice.Apply(float64(v.Point.X), float64(v.Point.Y))
		}
		cross := (x[1]-x[0])*(y[2]-y[0]) - (y[1]-y[0])*(x[2]-x[0])
		if cross == 0 || math.IsNaN(cross) || math.IsInf(cross, 0) {
			continue
		}
		if cross < 0 {
			x[1], x[2] = x[2], x[1]
			y[1], y[2] = y[2], y[1]
		}
		pb.MoveTo(float32(x[0]), float32(y[0]))
		pb.LineTo(float32(x[1]), float32(y[1]))
		pb.LineTo(float32(x[2]), float32(y[2]))
		pb.Close()
	}
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// meshSplitSide is the length in pixels of the side of a part of a triangle
// that splitTriangle makes.
const meshSplitSide = 16

// maxMeshSplits is the largest number of parts of a side that splits
// returns.
const maxMeshSplits = 64

// splits returns the number of parts of each side of a triangle of a mesh
// whose color space is not linear in RGB, such as DeviceCMYK. Each part is
// about meshSplitSide pixels long in the device space, so that the color in
// each part is close to linear.
func splits(tri []meshVertex, toDevice Matrix) int {
	var longest float64
	for j := range 3 {
		a, b := tri[j], tri[(j+1)%3]
		dx, dy := toDevice.ApplyDist(b.x-a.x, b.y-a.y)
		longest = math.Max(longest, math.Hypot(dx, dy))
	}
	if !(longest > meshSplitSide) {
		return 1
	}
	return int(math.Min(math.Ceil(longest/meshSplitSide), maxMeshSplits))
}

// splitTriangle appends a triangle of a mesh to b, split into n*n parts.
// The components vary linearly in the triangle, and each corner of a part
// takes the color of its components.
func (m *Machine) splitTriangle(b *meshBuilder, s *shading, tri [3]meshVertex, n int) {
	if len(b.tris)+n*n > maxMeshTriangles {
		n = max(int(math.Sqrt(float64(maxMeshTriangles-len(b.tris)))), 1)
	}
	nc := len(tri[0].c)
	comps := make([]float64, nc)
	// grid holds the corners of the parts, row j after row j-1, with n+1-j
	// corners in row j.
	grid := make([]okage.MeshVertex, 0, (n+1)*(n+2)/2)
	for j := 0; j <= n; j++ {
		for i := 0; i+j <= n; i++ {
			u, v := float64(i)/float64(n), float64(j)/float64(n)
			w := 1 - u - v
			x := w*tri[0].x + u*tri[1].x + v*tri[2].x
			y := w*tri[0].y + u*tri[1].y + v*tri[2].y
			for k := range comps {
				comps[k] = w*at(tri[0].c, k) + u*at(tri[1].c, k) + v*at(tri[2].c, k)
			}
			var c [4]float64
			if rgb, ok := m.toRGB(s.space, comps, 0); ok {
				c = [4]float64{rgb[0], rgb[1], rgb[2], 1}
			}
			grid = append(grid, okage.MeshVertex{Point: okage.PointFromXY(float32(x), float32(y)), Color: okageColor(c)})
		}
	}
	// row returns the index of the first corner of row j.
	row := func(j int) int { return j*(n+1) - j*(j-1)/2 }
	for j := range n {
		for i := 0; i+j < n; i++ {
			a, c, d := row(j)+i, row(j+1)+i, row(j)+i+1
			b.tris = append(b.tris, okage.MeshTriangle{grid[a], grid[d], grid[c]})
			if i+j+1 < n {
				b.tris = append(b.tris, okage.MeshTriangle{grid[d], grid[c+1], grid[c]})
			}
		}
	}
}

// okageColor returns the color of okage of the components of RGBA, which
// it clamps to 0..=1.
func okageColor(c [4]float64) okage.Color {
	color, _ := okage.ColorFromRGBA(float32(clamp01(c[0])), float32(clamp01(c[1])), float32(clamp01(c[2])), float32(clamp01(c[3])))
	return color
}

// shadingStop is a stop of the function of a shading: the parameter u of
// the stop, from 0 to 1, and the color of RGBA at u.
type shadingStop struct {
	u float64
	c [4]float64
}

// functionStops returns stops that approximate the colors of the function
// of a shading from t0, at the parameter 0, to t1, at the parameter 1. The
// colors between two stops are close to linear. The stops start at the
// bounds of a stitching function and at stopIntervals equal intervals, and
// functionStops halves an interval until the colors in it are close to
// linear. A color that the color space cannot give is transparent.
func (m *Machine) functionStops(s *shading, t0, t1 float64) []shadingStop {
	sample := func(u float64) shadingStop {
		st := shadingStop{u: u}
		if c, ok := m.shadingColor(s, []float64{t0 + u*(t1-t0)}); ok {
			st.c = [4]float64{c[0], c[1], c[2], 1}
		}
		return st
	}
	starts := []float64{0}
	for i := 1; i < stopIntervals; i++ {
		starts = append(starts, float64(i)/stopIntervals)
	}
	if s.fn.typ == 3 && t1 != t0 {
		for _, b := range s.fn.bounds {
			if u := (b - t0) / (t1 - t0); u > 0 && u < 1 {
				starts = append(starts, u)
			}
		}
	}
	starts = append(starts, 1)
	slices.Sort(starts)
	starts = slices.Compact(starts)
	stops := []shadingStop{sample(0)}
	for _, u := range starts[1:] {
		stops = refineStops(stops, stops[len(stops)-1], sample(u), sample, 0)
	}
	return stops
}

// refineStops appends the stops between a and b, and b, to stops. It
// halves the interval until the colors in it are close to linear.
func refineStops(stops []shadingStop, a, b shadingStop, sample func(float64) shadingStop, depth int) []shadingStop {
	if depth >= maxStopDepth {
		return append(stops, b)
	}
	mid := sample(0.5 * (a.u + b.u))
	linear := true
	for _, w := range [3]float64{0.25, 0.5, 0.75} {
		got := mid
		if w != 0.5 {
			got = sample(a.u + w*(b.u-a.u))
		}
		for k := range got.c {
			if math.Abs(got.c[k]-(a.c[k]+w*(b.c[k]-a.c[k]))) > stopTolerance {
				linear = false
			}
		}
	}
	if linear {
		return append(stops, b)
	}
	stops = refineStops(stops, a, mid, sample, depth+1)
	return refineStops(stops, mid, b, sample, depth+1)
}

// stopsColor returns the color at u on the segment from the stop i to the
// stop i+1. Out of the stops, it returns the color of the nearest stop.
func stopsColor(stops []shadingStop, i int, u float64) [4]float64 {
	switch {
	case i < 0:
		return stops[0].c
	case i+1 >= len(stops):
		return stops[len(stops)-1].c
	}
	a, b := stops[i], stops[i+1]
	w := b.u - a.u
	if !(w > 0) {
		return a.c
	}
	f := min(max((u-a.u)/w, 0), 1)
	var c [4]float64
	for k := range c {
		c[k] = a.c[k] + f*(b.c[k]-a.c[k])
	}
	return c
}

// meshBuilder collects the triangles of a MeshGradient.
type meshBuilder struct {
	tris []okage.MeshTriangle
	// cuts holds the values at which cut cuts a triangle, and parts counts
	// the parts that it cut.
	cuts  []float64
	parts int
}

// meshCut is a corner of a part of a triangle that meshBuilder cuts: its
// point, and its parameter u.
type meshCut struct {
	x, y, u float64
}

// add appends the triangle of the corners, with the color of u at each
// corner.
func (b *meshBuilder) add(tri [3]meshCut, color func(u float64) [4]float64) {
	if len(b.tris) >= maxMeshTriangles {
		return
	}
	var t okage.MeshTriangle
	for j, v := range tri {
		t[j] = okage.MeshVertex{Point: okage.PointFromXY(float32(v.x), float32(v.y)), Color: okageColor(color(v.u))}
	}
	b.tris = append(b.tris, t)
}

// cut appends a triangle whose parameter u varies linearly in it. The color
// is linear between two stops, so cut cuts the triangle along the lines
// where u is at a stop, and appends the triangles of each part.
func (b *meshBuilder) cut(tri [3]meshCut, stops []shadingStop) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := range tri {
		tri[i].u = min(max(tri[i].u, 0), 1)
		if math.IsNaN(tri[i].u) {
			tri[i].u = 0
		}
		lo, hi = math.Min(lo, tri[i].u), math.Max(hi, tri[i].u)
	}
	after := func(u float64) int {
		return sort.Search(len(stops), func(i int) bool { return stops[i].u > u })
	}
	b.cuts = append(b.cuts[:0], lo)
	for i := after(lo); i < len(stops) && stops[i].u < hi; i++ {
		if stops[i].u > b.cuts[len(b.cuts)-1] {
			b.cuts = append(b.cuts, stops[i].u)
		}
	}
	b.cuts = append(b.cuts, hi)
	cuts := b.cuts
	var buf, scratch [8]meshCut
	for k := 0; k+1 < len(cuts) && b.parts < maxMeshParts && len(b.tris) < maxMeshTriangles; k++ {
		b.parts++
		lower, upper := cuts[k], cuts[k+1]
		poly, other := append(buf[:0], tri[:]...), scratch[:0]
		if k > 0 {
			poly, other = cutPolygon(poly, other, lower, false), poly[:0]
		}
		if k+2 < len(cuts) {
			poly = cutPolygon(poly, other, upper, true)
		}
		i := after(0.5*(lower+upper)) - 1
		color := func(u float64) [4]float64 { return stopsColor(stops, i, u) }
		for j := 1; j+1 < len(poly); j++ {
			b.add([3]meshCut{poly[0], poly[j], poly[j+1]}, color)
		}
	}
}

// cutPolygon appends to out the part of the convex polygon in where u is
// level or more, or where u is level or less when below is true, and
// returns it. out must not share memory with in.
//
// A point on an edge depends only on the two ends of the edge, and not on
// their order. Two triangles that share an edge thus cut it at the same
// point, and the triangles meet with no gap.
func cutPolygon(in, out []meshCut, level float64, below bool) []meshCut {
	if len(in) == 0 {
		return out
	}
	f := func(v meshCut) float64 {
		if below {
			return level - v.u
		}
		return v.u - level
	}
	prev := in[len(in)-1]
	fPrev := f(prev)
	for _, cur := range in {
		fCur := f(cur)
		if (fCur >= 0) != (fPrev >= 0) {
			a, b, fa, fb := prev, cur, fPrev, fCur
			if b.x < a.x || (b.x == a.x && b.y < a.y) {
				a, b, fa, fb = b, a, fb, fa
			}
			t := fa / (fa - fb)
			out = append(out, meshCut{x: a.x + t*(b.x-a.x), y: a.y + t*(b.y-a.y), u: level})
		}
		if fCur >= 0 {
			out = append(out, cur)
		}
		prev, fPrev = cur, fCur
	}
	return out
}
