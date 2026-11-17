package graphics

import (
	"image"
	"io"
	"math"

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

// shadingImage draws a shading into an image that covers the rectangle r
// of the device space. toDevice maps the shading space onto the device
// space. background paints the Background of the shading outside of it.
// It returns the image and the transform from its pixels to the device
// space.
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
	default:
		m.rasterMesh(s, img, pixToShading)
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

// rasterMesh fills the triangles of a mesh into an image, with the colors
// of their vertices interpolated.
func (m *Machine) rasterMesh(s *shading, img *image.RGBA, pixToShading Matrix) {
	toPix, ok := pixToShading.Invert()
	if !ok {
		return
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	nc := len(s.tris[0].c)
	c := make([]float64, nc)
	for i := 0; i+2 < len(s.tris); i += 3 {
		if i&1023 == 0 && m.in.Context().Err() != nil {
			return
		}
		var px, py [3]float64
		for j := range 3 {
			px[j], py[j] = toPix.Apply(s.tris[i+j].x, s.tris[i+j].y)
		}
		minX := int(math.Floor(math.Min(px[0], math.Min(px[1], px[2]))))
		maxX := int(math.Ceil(math.Max(px[0], math.Max(px[1], px[2]))))
		minY := int(math.Floor(math.Min(py[0], math.Min(py[1], py[2]))))
		maxY := int(math.Ceil(math.Max(py[0], math.Max(py[1], py[2]))))
		minX, minY = max(minX, 0), max(minY, 0)
		maxX, maxY = min(maxX, w-1), min(maxY, h-1)
		den := (py[1]-py[2])*(px[0]-px[2]) + (px[2]-px[1])*(py[0]-py[2])
		if den == 0 {
			continue
		}
		for y := minY; y <= maxY; y++ {
			fy := float64(y) + 0.5
			for x := minX; x <= maxX; x++ {
				fx := float64(x) + 0.5
				l0 := ((py[1]-py[2])*(fx-px[2]) + (px[2]-px[1])*(fy-py[2])) / den
				l1 := ((py[2]-py[0])*(fx-px[2]) + (px[0]-px[2])*(fy-py[2])) / den
				l2 := 1 - l0 - l1
				const e = -1e-6
				if l0 < e || l1 < e || l2 < e {
					continue
				}
				for k := range nc {
					c[k] = l0*s.tris[i].c[k] + l1*s.tris[i+1].c[k] + l2*s.tris[i+2].c[k]
				}
				rgb, ok := m.shadingColor(s, c)
				if !ok {
					continue
				}
				o := y*img.Stride + 4*x
				img.Pix[o] = uint8(math.Round(clamp01(rgb[0]) * 255))
				img.Pix[o+1] = uint8(math.Round(clamp01(rgb[1]) * 255))
				img.Pix[o+2] = uint8(math.Round(clamp01(rgb[2]) * 255))
				img.Pix[o+3] = 255
			}
		}
	}
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

// shfill paints a shading over the clip, as shfill does.
func (m *Machine) shfill(s *shading) error {
	if m.capture != nil && m.capture.intercept() {
		return nil
	}
	toDevice := m.gs.ctm
	area := m.shadingBounds(s, toDevice, m.deviceArea())
	img, t, ok := m.shadingImage(s, toDevice, area, false)
	if !ok {
		return nil
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
	m.dev.DrawImage(img, toTransform(t), display.ImageOptions{Opacity: 1, Smooth: true})
	if pushed {
		m.dev.Restore()
	}
	return nil
}

// shadingShader returns the shader of a shading pattern, over the area of
// the clip.
func (m *Machine) shadingShader(pat *pattern) okage.Shader {
	s := pat.shading
	area := m.shadingBounds(s, pat.matrix, m.deviceArea())
	img, t, ok := m.shadingImage(s, pat.matrix, area, true)
	if !ok {
		return nil
	}
	pix, err := raster.Pixmap(img)
	if err != nil {
		return nil
	}
	return okage.NewPattern(pix.AsRef(), okage.SpreadModePad, okage.FilterQualityBilinear, 1, toTransform(t))
}
