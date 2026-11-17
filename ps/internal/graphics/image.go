package graphics

import (
	"errors"
	"image"
	"io"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/ps/internal/interp"
)

// imageParams are the parameters of an image (PLRM 4.10).
type imageParams struct {
	width, height int
	bpc           int
	matrix        Matrix
	decode        []float64
	interpolate   bool
	space         *colorSpace
	// sources holds one source, or one for each component.
	sources []interp.Object
	// mask is true for imagemask, and polarity is its polarity.
	mask, polarity bool
	// maskColor holds the colors of an image of type 4 that are
	// transparent: a value or a range for each component.
	maskColor []int
	// maskImage is the mask of an image of type 3, or nil.
	maskImage *imageParams
}

// errNoData ends the data of an image early.
var errNoData = errors.New("end of the data of an image")

// repeatReader reads a string again and again, as the image operators read
// a string source.
type repeatReader struct {
	b   []byte
	pos int
}

// Read satisfies the io.Reader interface.
func (r *repeatReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) {
		c := copy(p[n:], r.b[r.pos:])
		n += c
		r.pos = (r.pos + c) % len(r.b)
	}
	return n, nil
}

// reader returns the reader of a data source of an image.
func (m *Machine) reader(o interp.Object) (io.Reader, error) {
	if o.Type() == interp.TString {
		return &repeatReader{b: o.Bytes()}, nil
	}
	return m.in.NewDataSource(o)
}

// readRow reads one row of n bytes. A short row is filled with zeros.
func readRow(r io.Reader, row []byte) error {
	n, err := io.ReadFull(r, row)
	if n < len(row) {
		clear(row[n:])
		if n == 0 {
			if err == nil {
				err = errNoData
			}
			return err
		}
	}
	return nil
}

// sample returns the sample i of a row of samples of bpc bits.
func sample(row []byte, i, bpc int) int {
	switch bpc {
	case 8:
		return int(row[i])
	case 16:
		return int(row[2*i])<<8 | int(row[2*i+1])
	case 12:
		bit := i * 12
		b := bit / 8
		if bit%8 == 0 {
			return int(row[b])<<4 | int(row[b+1])>>4
		}
		return int(row[b]&15)<<8 | int(row[b+1])
	}
	bit := i * bpc
	shift := 8 - bpc - bit%8
	return int(row[bit/8]>>shift) & (1<<bpc - 1)
}

// imageDict reads the parameters of an image dictionary.
func (m *Machine) imageDict(d *interp.Dict, mask bool) (*imageParams, error) {
	p := &imageParams{mask: mask, space: m.gs.color.space}
	typ := 1
	if v, ok := d.GetName("ImageType"); ok && v.IsNumber() {
		typ = int(v.Float())
	}
	if typ == 3 {
		return m.imageType3(d)
	}
	geti := func(key string) (int, bool) {
		v, ok := d.GetName(key)
		if !ok || !v.IsNumber() {
			return 0, false
		}
		return int(v.Float()), true
	}
	var ok bool
	if p.width, ok = geti("Width"); !ok {
		return nil, interp.ErrTypecheck
	}
	if p.height, ok = geti("Height"); !ok {
		return nil, interp.ErrTypecheck
	}
	if p.bpc, ok = geti("BitsPerComponent"); !ok {
		if !mask {
			return nil, interp.ErrTypecheck
		}
		p.bpc = 1
	}
	mo, ok := d.GetName("ImageMatrix")
	if !ok {
		return nil, interp.ErrTypecheck
	}
	mat, err := matrixOf(mo)
	if err != nil {
		return nil, err
	}
	p.matrix = mat
	if v, ok := d.GetName("Interpolate"); ok && v.Type() == interp.TBool {
		p.interpolate = v.BoolVal()
	}
	src, ok := d.GetName("DataSource")
	if !ok {
		return nil, interp.ErrTypecheck
	}
	if v, ok := d.GetName("MultipleDataSources"); ok && v.Type() == interp.TBool && v.BoolVal() && src.IsArray() && !src.IsProc() {
		p.sources = append(p.sources, src.Array()...)
	} else {
		p.sources = []interp.Object{src}
	}
	n := 1
	if !mask {
		if p.space.family == "Pattern" {
			return nil, interp.ErrRangecheck
		}
		n = p.space.n
	}
	if v, ok := d.GetName("Decode"); ok && v.IsArray() {
		for _, e := range v.Array() {
			p.decode = append(p.decode, e.Float())
		}
	}
	if len(p.decode) < 2*n {
		p.decode = m.defaultDecode(p.space, p.bpc, n)
	}
	if mask {
		// with Decode [1 0], a sample of 1 paints
		p.polarity = p.decode[0] > p.decode[1]
		p.space = nil
	}
	if typ == 4 {
		if v, ok := d.GetName("MaskColor"); ok && v.IsArray() {
			for _, e := range v.Array() {
				p.maskColor = append(p.maskColor, int(e.Float()))
			}
		}
	}
	return p, p.check(m)
}

// imageType3 reads an image of type 3: a data dictionary with a mask
// dictionary. The machine reads InterleaveType 3, with a source for each.
func (m *Machine) imageType3(d *interp.Dict) (*imageParams, error) {
	dd, ok1 := d.GetName("DataDict")
	md, ok2 := d.GetName("MaskDict")
	if !ok1 || !ok2 || dd.Type() != interp.TDict || md.Type() != interp.TDict {
		return nil, interp.ErrTypecheck
	}
	p, err := m.imageDict(dd.Dict(), false)
	if err != nil {
		return nil, err
	}
	if v, ok := d.GetName("InterleaveType"); ok && v.IsNumber() && int(v.Float()) == 3 {
		mask, err := m.imageDict(md.Dict(), true)
		if err == nil {
			p.maskImage = mask
		}
	}
	return p, nil
}

// defaultDecode returns the default decode array of a color space.
func (m *Machine) defaultDecode(cs *colorSpace, bpc, n int) []float64 {
	d := make([]float64, 0, 2*n)
	if cs != nil && cs.family == "Indexed" {
		return append(d, 0, float64(int(1)<<bpc-1))
	}
	for i := range n {
		lo, hi := 0.0, 1.0
		if cs != nil && 2*i+1 < len(cs.ranges) {
			lo, hi = cs.ranges[2*i], cs.ranges[2*i+1]
		}
		d = append(d, lo, hi)
	}
	return d
}

// check checks the parameters against the limits.
func (p *imageParams) check(m *Machine) error {
	switch p.bpc {
	case 1, 2, 4, 8, 12, 16:
	default:
		return interp.ErrRangecheck
	}
	if p.width <= 0 || p.height <= 0 {
		return interp.ErrRangecheck
	}
	px := int64(p.width) * int64(p.height)
	if px > m.lim.MaxPixels || p.width > 1<<20 || p.height > 1<<20 {
		return limitError("pixels of an image", m.lim.MaxPixels)
	}
	m.pixels += px
	if m.pixels > 4*m.lim.MaxPixels {
		return limitError("pixels of the images", 4*m.lim.MaxPixels)
	}
	return nil
}

// drawImage reads the samples of an image and draws it.
func (m *Machine) drawImage(p *imageParams) error {
	n := 1
	if !p.mask {
		n = p.space.n
	}
	readers := make([]io.Reader, len(p.sources))
	for i, s := range p.sources {
		r, err := m.reader(s)
		if err != nil {
			return err
		}
		readers[i] = r
	}
	var maskRead func(y int) []bool
	if p.maskImage != nil {
		mr, err := m.reader(p.maskImage.sources[0])
		if err != nil {
			return err
		}
		mrow := make([]byte, (p.maskImage.width*p.maskImage.bpc+7)/8)
		mask := make([][]bool, 0, p.maskImage.height)
		for range p.maskImage.height {
			if err := readRow(mr, mrow); err != nil {
				break
			}
			bits := make([]bool, p.maskImage.width)
			for x := range bits {
				bits[x] = (sample(mrow, x, p.maskImage.bpc) != 0) == p.maskImage.polarity
			}
			mask = append(mask, bits)
		}
		maskRead = func(y int) []bool {
			my := y * p.maskImage.height / p.height
			if my < len(mask) {
				return mask[my]
			}
			return nil
		}
	}
	var img image.Image
	var alpha *image.Alpha
	var rgba *image.NRGBA
	if p.mask {
		alpha = image.NewAlpha(image.Rect(0, 0, p.width, p.height))
		img = alpha
	} else {
		rgba = image.NewNRGBA(image.Rect(0, 0, p.width, p.height))
		img = rgba
	}
	conv := m.pixelConverter(p, n)
	maxV := float64(int(1)<<p.bpc - 1)
	single := len(readers) == 1
	var row []byte
	var rows [][]byte
	if single {
		row = make([]byte, (p.width*n*p.bpc+7)/8)
	} else {
		rows = make([][]byte, len(readers))
		for i := range rows {
			rows[i] = make([]byte, (p.width*p.bpc+7)/8)
		}
	}
	comps := make([]int, n)
	var err error
rowsLoop:
	for y := range p.height {
		if y&63 == 0 {
			if cerr := m.in.Context().Err(); cerr != nil {
				return cerr
			}
		}
		if single {
			if err = readRow(readers[0], row); err != nil {
				break
			}
		} else {
			for i, r := range readers {
				if err = readRow(r, rows[i]); err != nil {
					break rowsLoop
				}
			}
		}
		var mrow []bool
		if maskRead != nil {
			mrow = maskRead(y)
		}
		for x := range p.width {
			if single {
				for c := range n {
					comps[c] = sample(row, x*n+c, p.bpc)
				}
			} else {
				for c := range n {
					if c < len(rows) {
						comps[c] = sample(rows[c], x, p.bpc)
					}
				}
			}
			if p.mask {
				if (comps[0] != 0) == p.polarity {
					alpha.Pix[y*alpha.Stride+x] = 255
				}
				continue
			}
			o := y*rgba.Stride + 4*x
			if p.maskColor != nil && maskedColor(comps, p.maskColor) {
				continue
			}
			if mrow != nil && x*len(mrow)/p.width < len(mrow) && !mrow[x*len(mrow)/p.width] {
				continue
			}
			c := conv(comps, maxV)
			rgba.Pix[o], rgba.Pix[o+1], rgba.Pix[o+2], rgba.Pix[o+3] = c[0], c[1], c[2], 255
		}
	}
	if errors.Is(err, errNoData) || errors.Is(err, io.EOF) {
		err = nil
	}
	if err != nil && (interp.IsSignal(err) || isFatal(err)) {
		return err
	}
	inv, ok := p.matrix.Invert()
	if !ok {
		return nil
	}
	t := inv.Mul(m.gs.ctm)
	if !t.Finite() || t.Det() == 0 {
		return nil
	}
	if m.capture != nil && (m.capture.mode != modePaint || (!p.mask && m.capture.cache)) {
		return nil
	}
	opts := display.ImageOptions{Opacity: 1, Smooth: p.interpolate || t.Scale() < 1}
	if p.mask {
		col := m.gs.color
		if m.capture != nil && m.capture.cache {
			col = m.capture.color
		}
		shader := m.shader(col)
		if shader == nil {
			return nil
		}
		m.mark()
		m.dev.FillMask(alpha, toTransform(t), opts, shader)
		return nil
	}
	m.mark()
	m.dev.DrawImage(img, toTransform(t), opts)
	return nil
}

// isFatal reports whether an error ends the program.
func isFatal(err error) bool {
	var pe *interp.Error
	return !errors.As(err, &pe)
}

// maskedColor reports whether the samples of a pixel match MaskColor.
func maskedColor(comps, mask []int) bool {
	if len(mask) == len(comps) {
		for i, c := range comps {
			if c != mask[i] {
				return false
			}
		}
		return true
	}
	if len(mask) == 2*len(comps) {
		for i, c := range comps {
			if c < mask[2*i] || c > mask[2*i+1] {
				return false
			}
		}
		return true
	}
	return false
}

// pixelConverter returns the function that converts the samples of a
// pixel to sRGB.
func (m *Machine) pixelConverter(p *imageParams, n int) func(comps []int, maxV float64) [3]uint8 {
	if p.mask {
		return nil
	}
	dec := p.decode
	val := func(c, s int, maxV float64) float64 {
		return dec[2*c] + float64(s)*(dec[2*c+1]-dec[2*c])/maxV
	}
	cs := p.space
	switch cs.family {
	case "DeviceGray":
		return func(comps []int, maxV float64) [3]uint8 {
			g := to8(val(0, comps[0], maxV))
			return [3]uint8{g, g, g}
		}
	case "DeviceRGB":
		return func(comps []int, maxV float64) [3]uint8 {
			return [3]uint8{to8(val(0, comps[0], maxV)), to8(val(1, comps[1], maxV)), to8(val(2, comps[2], maxV))}
		}
	}
	// the other spaces go through a cache of the colors of the samples
	cache := map[[4]int][3]uint8{}
	values := make([]float64, n)
	return func(comps []int, maxV float64) [3]uint8 {
		var key [4]int
		if n <= 4 {
			copy(key[:], comps)
			if c, ok := cache[key]; ok {
				return c
			}
		}
		for i := range n {
			values[i] = val(i, comps[i], maxV)
		}
		rgb, _ := m.toRGB(cs, values, 0)
		c := [3]uint8{to8(rgb[0]), to8(rgb[1]), to8(rgb[2])}
		if n <= 4 && len(cache) < maxCachedColors {
			cache[key] = c
		}
		return c
	}
}

// opsImage are the operators of images (PLRM 8.1).
func (m *Machine) opsImage() {
	in := m.in
	in.Define("image", func(*interp.Interp) error {
		top, err := in.Peek(0)
		if err != nil {
			return err
		}
		if top.Type() == interp.TDict {
			p, err := m.imageDict(top.Dict(), false)
			if err != nil {
				return err
			}
			_, _ = in.Pop()
			return m.drawImage(p)
		}
		p, err := m.level1Image(1, false)
		if err != nil {
			return err
		}
		p.space = deviceGray
		p.decode = []float64{0, 1}
		return m.drawImage(p)
	})
	in.Define("imagemask", func(*interp.Interp) error {
		top, err := in.Peek(0)
		if err != nil {
			return err
		}
		if top.Type() == interp.TDict {
			p, err := m.imageDict(top.Dict(), true)
			if err != nil {
				return err
			}
			_, _ = in.Pop()
			return m.drawImage(p)
		}
		p, err := m.level1Image(1, true)
		if err != nil {
			return err
		}
		return m.drawImage(p)
	})
	in.Define("colorimage", func(*interp.Interp) error {
		ncomp, err := in.PopInt()
		if err != nil {
			return err
		}
		multi, err := in.PopBool()
		if err != nil {
			return err
		}
		var cs *colorSpace
		switch ncomp {
		case 1:
			cs = deviceGray
		case 3:
			cs = deviceRGB
		case 4:
			cs = deviceCMYK
		default:
			return interp.ErrRangecheck
		}
		nsrc := 1
		if multi {
			nsrc = ncomp
		}
		p, err := m.level1Image(nsrc, false)
		if err != nil {
			return err
		}
		p.space = cs
		p.decode = m.defaultDecode(cs, p.bpc, ncomp)
		return m.drawImage(p)
	})
}

// level1Image pops the operands of image, imagemask and colorimage of
// Level 1: width, height, bits or polarity, matrix, and nsrc sources.
func (m *Machine) level1Image(nsrc int, mask bool) (*imageParams, error) {
	in := m.in
	if in.Count() < 4+nsrc {
		return nil, interp.ErrStackUnderflow
	}
	srcs := make([]interp.Object, nsrc)
	for i := nsrc - 1; i >= 0; i-- {
		srcs[i], _ = in.Pop()
	}
	mat, _, err := m.popMatrix()
	if err != nil {
		return nil, err
	}
	third, err := in.Pop()
	if err != nil {
		return nil, err
	}
	v, err := in.PopNums(2)
	if err != nil {
		return nil, err
	}
	p := &imageParams{width: int(v[0]), height: int(v[1]), matrix: mat, sources: srcs, mask: mask, bpc: 1}
	if mask {
		if third.Type() != interp.TBool {
			return nil, interp.ErrTypecheck
		}
		p.polarity = third.BoolVal()
	} else {
		if !third.IsNumber() {
			return nil, interp.ErrTypecheck
		}
		p.bpc = int(third.Float())
	}
	return p, p.check(m)
}
