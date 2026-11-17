package graphics

import (
	"math"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/pdffont"
	"github.com/xo/ivy/ps/internal/interp"
)

// FontProgram is the program of a font that a PostScript program names and
// does not define, such as Helvetica.
type FontProgram struct {
	// Program draws the glyphs. Its outlines are in the units of
	// pdffont.UnitsPerEm.
	Program *pdffont.Program
	// Standard gives the widths of the glyphs when HasStandard is true.
	Standard    pdffont.StandardFont
	HasStandard bool
	// ScaleX scales the glyphs and their widths in x, such as for a narrow
	// face. Zero is 1.
	ScaleX float64
	// Encoding holds the name of each code of the built-in encoding, or
	// nothing for StandardEncoding.
	Encoding []string
}

// FontSource finds the program of a font that a PostScript program names
// and does not define. It returns false when it has none, and the machine
// then takes the substitute of the closest standard font.
type FontSource func(name string) (*FontProgram, bool)

// maxCachedGlyphs bounds the glyphs that a font keeps.
const maxCachedGlyphs = 4096

// fontRec is a font that definefont made. The FID of the dictionary holds
// it.
type fontRec struct {
	dict *interp.Dict
	// typ is the FontType, or 9 for a CIDFont, whose CIDFontType is
	// cidType.
	typ     int
	cidType int
	// sub is the program of a font that the machine found for a name.
	sub *FontProgram
	// prog is the program that the dictionary defines: the charstrings of a
	// Type 1 font, or the sfnts of a Type 42 font.
	prog *pdffont.Program
	// loaded is true when the machine read prog.
	loaded bool
	glyphs map[string]*glyph
}

// glyph is the outline and the width of a glyph, in the glyph space.
type glyph struct {
	path       Path
	wx, wy     float64
	hasWidth   bool
	hasOutline bool
}

// fontCache holds the fonts of a machine.
type fontCache struct {
	m      *Machine
	source FontSource
	// subs holds the substitute of each CharStrings dictionary that the
	// machine made for a font that it found by name.
	subs map[*interp.Dict]*FontProgram
	std  map[pdffont.StandardFont]*pdffont.Program
}

// newFontCache returns the fonts of a machine.
func newFontCache(m *Machine, source FontSource) *fontCache {
	return &fontCache{
		m:      m,
		source: source,
		subs:   map[*interp.Dict]*FontProgram{},
		std:    map[pdffont.StandardFont]*pdffont.Program{},
	}
}

// standardProgram returns the substitute of a standard font.
func (c *fontCache) standardProgram(s pdffont.StandardFont) *pdffont.Program {
	if p, ok := c.std[s]; ok {
		return p
	}
	p, err := pdffont.StandardProgram(s)
	if err != nil {
		p = nil
	}
	c.std[s] = p
	return p
}

// standardFor returns the standard font that is closest to a name, and the
// scale in x of a narrow face.
func standardFor(name string) (pdffont.StandardFont, float64) {
	lower := strings.ToLower(name)
	bold := strings.Contains(lower, "bold") || strings.Contains(lower, "demi") ||
		strings.Contains(lower, "heavy") || strings.Contains(lower, "black")
	italic := strings.Contains(lower, "italic") || strings.Contains(lower, "oblique") ||
		strings.Contains(lower, "slanted") || strings.Contains(lower, "kursiv")
	scale := 1.0
	if strings.Contains(lower, "narrow") || strings.Contains(lower, "condensed") {
		scale = 0.82
	}
	pick := func(regular, b, i, bi pdffont.StandardFont) pdffont.StandardFont {
		switch {
		case bold && italic:
			return bi
		case bold:
			return b
		case italic:
			return i
		}
		return regular
	}
	switch {
	case strings.Contains(lower, "dingbat"):
		return pdffont.ZapfDingBats, 1
	case lower == "symbol" || strings.HasPrefix(lower, "symbol"):
		return pdffont.Symbol, 1
	case strings.Contains(lower, "courier") || strings.Contains(lower, "mono") ||
		strings.Contains(lower, "typewriter") || strings.Contains(lower, "letter gothic"):
		return pick(pdffont.Courier, pdffont.CourierBold, pdffont.CourierOblique, pdffont.CourierBoldOblique), scale
	case strings.Contains(lower, "helvetica") || strings.Contains(lower, "arial") ||
		strings.Contains(lower, "avantgarde") || strings.Contains(lower, "sans") ||
		strings.Contains(lower, "gothic") || strings.Contains(lower, "futura") ||
		strings.Contains(lower, "univers") || strings.Contains(lower, "optima") ||
		strings.Contains(lower, "gill") || strings.Contains(lower, "verdana") ||
		strings.Contains(lower, "tahoma") || strings.Contains(lower, "frutiger"):
		return pick(pdffont.Helvetica, pdffont.HelveticaBold, pdffont.HelveticaOblique, pdffont.HelveticaBoldOblique), scale
	}
	return pick(pdffont.TimesRoman, pdffont.TimesBold, pdffont.TimesItalic, pdffont.TimesBoldItalic), scale
}

// isStandard35 reports whether a name is one of the 35 fonts of a
// PostScript printer, which the machine always draws with its substitutes.
func isStandard35(name string) bool {
	for _, p := range []string{"Times-", "Helvetica", "Courier", "Symbol", "ZapfDingbats",
		"AvantGarde-", "Bookman-", "NewCenturySchlbk-", "Palatino-", "ZapfChancery-"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// findProgram returns the program of a font that a program names.
func (c *fontCache) findProgram(name string) *FontProgram {
	if c.source != nil && !isStandard35(name) {
		if fp, ok := c.source(name); ok && fp != nil && fp.Program != nil {
			return fp
		}
	}
	s, scale := standardFor(name)
	fp := &FontProgram{Program: c.standardProgram(s), Standard: s, HasStandard: true, ScaleX: scale}
	if s == pdffont.Symbol || s == pdffont.ZapfDingBats {
		fp.Encoding = make([]string, 256)
		for code := range 256 {
			if s == pdffont.Symbol {
				fp.Encoding[code], _ = pdffont.SymbolEncodingName(uint8(code))
			} else {
				fp.Encoding[code], _ = pdffont.DingbatsEncodingName(uint8(code))
			}
		}
	}
	return fp
}

// substituteFont makes the dictionary of a font that a program names and
// does not define, and defines it.
func (m *Machine) substituteFont(key interp.Object) (interp.Object, error) {
	name := key.Text()
	fp := m.fonts.findProgram(name)
	in := m.in
	old := in.SetGlobal(true)
	defer in.SetGlobal(old)
	fo := in.NewDict(16)
	d := fo.Dict()
	d.PutName("FontType", interp.Int(1))
	d.PutName("FontName", interp.NameObj(name))
	d.PutName("PaintType", interp.Int(0))
	mat, err := m.newMatrix(Matrix{0.001, 0, 0, 0.001, 0, 0})
	if err != nil {
		return interp.Null, err
	}
	d.PutName("FontMatrix", mat)
	bbox, err := in.NewArray([]interp.Object{interp.Int(-200), interp.Int(-250), interp.Int(1100), interp.Int(950)})
	if err != nil {
		return interp.Null, err
	}
	d.PutName("FontBBox", bbox)
	var enc interp.Object
	if fp.Encoding != nil {
		elems := make([]interp.Object, 256)
		for i, n := range fp.Encoding {
			if n == "" {
				n = ".notdef"
			}
			elems[i] = interp.NameObj(n)
		}
		if enc, err = in.NewArray(elems); err != nil {
			return interp.Null, err
		}
	} else {
		enc, _ = m.encoding("StandardEncoding")
	}
	d.PutName("Encoding", enc)
	cs := in.NewDict(256)
	names := map[string]bool{".notdef": true}
	for _, e := range enc.Array() {
		names[e.Text()] = true
	}
	if latin, ok := m.encoding("ISOLatin1Encoding"); ok && fp.Encoding == nil {
		for _, e := range latin.Array() {
			names[e.Text()] = true
		}
	}
	for n := range names {
		cs.Dict().PutName(n, interp.Int(0))
	}
	d.PutName("CharStrings", cs)
	m.fonts.subs[cs.Dict()] = fp
	info := in.NewDict(4)
	if full, err := in.NewString([]byte(name)); err == nil {
		info.Dict().PutName("FullName", full)
	}
	d.PutName("FontInfo", info)
	rec := &fontRec{dict: d, typ: 1, sub: fp}
	d.PutName("FID", interp.FontID(rec))
	d.SetAccess(interp.ReadOnly)
	m.in.Category("Font").Put(key, fo)
	return fo, nil
}

// defineFont makes the font of a dictionary, for definefont.
func (m *Machine) defineFont(key, fo interp.Object) (interp.Object, error) {
	if fo.Type() != interp.TDict {
		return interp.Null, interp.ErrTypecheck
	}
	d := fo.Dict()
	typ := 1
	if v, ok := d.GetName("FontType"); ok && v.IsNumber() {
		typ = int(v.Float())
	}
	if v, ok := d.GetName("CIDFontType"); ok && v.IsNumber() {
		typ = 9
	}
	switch typ {
	case 0, 1, 2, 3, 9, 42, 10, 11:
	default:
		return interp.Null, interp.ErrInvalidFont
	}
	if typ != 9 {
		if _, ok := d.GetName("FontMatrix"); !ok {
			return interp.Null, interp.ErrInvalidFont
		}
	}
	rec := &fontRec{dict: d, typ: typ}
	if cs, ok := d.GetName("CharStrings"); ok && cs.Type() == interp.TDict {
		if fp, ok := m.fonts.subs[cs.Dict()]; ok {
			rec.sub = fp
		}
	}
	if typ == 1 && rec.sub == nil && !hasCharstrings(d) {
		// a font without its outlines draws with a substitute
		name := ""
		if n, ok := d.GetName("FontName"); ok {
			name = n.Text()
		}
		if name == "" {
			name = key.Text()
		}
		rec.sub = m.fonts.findProgram(name)
	}
	d.PutName("FID", interp.FontID(rec))
	return fo, nil
}

// hasCharstrings reports whether a Type 1 font holds the charstrings of its
// glyphs as strings.
func hasCharstrings(d *interp.Dict) bool {
	cs, ok := d.GetName("CharStrings")
	if !ok || cs.Type() != interp.TDict {
		return false
	}
	found := false
	cs.Dict().Each(func(_, v interp.Object) bool {
		found = v.Type() == interp.TString
		return !found
	})
	return found
}

// fontOf returns the font of a font dictionary.
func fontOf(fo interp.Object) *fontRec {
	d := fo.Dict()
	if d == nil {
		return nil
	}
	fid, ok := d.GetName("FID")
	if !ok || fid.Type() != interp.TFontID {
		return nil
	}
	rec, _ := fid.Ref().(*fontRec)
	return rec
}

// fontMatrix returns the FontMatrix of a font dictionary.
func fontMatrix(d *interp.Dict) Matrix {
	if o, ok := d.GetName("FontMatrix"); ok {
		if mat, err := matrixOf(o); err == nil {
			return mat
		}
	}
	return Matrix{0.001, 0, 0, 0.001, 0, 0}
}

// load reads the program that the dictionary of a font defines.
func (r *fontRec) load() {
	if r.loaded {
		return
	}
	r.loaded = true
	switch r.typ {
	case 1:
		if r.sub == nil {
			r.prog = type1Program(r.dict)
		}
	case 42:
		r.prog = type42Program(r.dict)
	}
}

// type1Program returns the program of the charstrings of a Type 1 font.
func type1Program(d *interp.Dict) *pdffont.Program {
	cs, ok := d.GetName("CharStrings")
	if !ok || cs.Type() != interp.TDict {
		return nil
	}
	var names []string
	var strs [][]byte
	cs.Dict().Each(func(k, v interp.Object) bool {
		if v.Type() == interp.TString {
			names = append(names, k.Text())
			strs = append(strs, v.Bytes())
		}
		return true
	})
	lenIV := 4
	var subrs [][]byte
	if p, ok := d.GetName("Private"); ok && p.Type() == interp.TDict {
		pd := p.Dict()
		if v, ok := pd.GetName("lenIV"); ok && v.IsNumber() {
			lenIV = int(v.Float())
		}
		if v, ok := pd.GetName("Subrs"); ok && v.IsArray() {
			for _, s := range v.Array() {
				subrs = append(subrs, s.Bytes())
			}
		}
	}
	var weights []float64
	if v, ok := d.GetName("WeightVector"); ok && v.IsArray() {
		for _, e := range v.Array() {
			weights = append(weights, e.Float())
		}
	}
	prog, err := pdffont.NewType1Charstrings(names, strs, subrs, lenIV, weights)
	if err != nil {
		return nil
	}
	return prog
}

// type42Program returns the TrueType program of the sfnts of a Type 42 font.
func type42Program(d *interp.Dict) *pdffont.Program {
	o, ok := d.GetName("sfnts")
	if !ok || !o.IsArray() {
		return nil
	}
	var data []byte
	for _, s := range o.Array() {
		b := s.Bytes()
		if len(b)%2 == 1 {
			// a string of odd length ends with a byte of padding
			b = b[:len(b)-1]
		}
		data = append(data, b...)
	}
	prog, err := pdffont.NewProgram(data, 0)
	if err != nil {
		return nil
	}
	return prog
}

// glyphOf returns the glyph of a name, in the glyph space of the font.
func (m *Machine) glyphOf(r *fontRec, name string) *glyph {
	if g, ok := r.glyphs[name]; ok {
		return g
	}
	r.load()
	g := &glyph{}
	switch {
	case r.sub != nil:
		fp := r.sub
		sx := fp.ScaleX
		if sx == 0 {
			sx = 1
		}
		if fp.Program != nil {
			if gid, ok := fp.Program.GlyphByName(name); ok {
				g.path, g.hasOutline = bezToPath(fp.Program.Outline(gid), Matrix{sx, 0, 0, 1, 0, 0}), true
				if adv, ok := fp.Program.Advance(gid); ok && !fp.HasStandard {
					g.wx, g.hasWidth = float64(adv)*sx, true
				}
			}
		}
		if fp.HasStandard {
			if w, ok := fp.Standard.Width(name); ok {
				g.wx, g.hasWidth = float64(w)*sx, true
			}
		}
	case r.typ == 1 && r.prog != nil:
		if gid, ok := r.prog.GlyphByName(name); ok {
			outline, adv, ok := r.prog.Type1Glyph(gid)
			g.path, g.hasOutline = bezToPath(outline, identity), true
			if ok {
				g.wx, g.wy, g.hasWidth = adv.X, adv.Y, true
			}
		}
	case r.typ == 42 && r.prog != nil:
		gid := pdffont.GlyphID(0)
		found := false
		if cs, ok := r.dict.GetName("CharStrings"); ok && cs.Type() == interp.TDict {
			if v, ok := cs.Dict().GetName(name); ok && v.IsNumber() {
				gid, found = pdffont.GlyphID(v.Float()), true
			}
		}
		if !found {
			gid, found = r.prog.GlyphByName(name)
		}
		if found {
			s := 1 / float64(pdffont.UnitsPerEm)
			g.path, g.hasOutline = bezToPath(r.prog.Outline(gid), Matrix{s, 0, 0, s, 0, 0}), true
			if adv, ok := r.prog.Advance(gid); ok {
				g.wx, g.hasWidth = float64(adv)*s, true
			}
		}
	}
	if r.glyphs == nil || len(r.glyphs) >= maxCachedGlyphs {
		r.glyphs = map[string]*glyph{}
	}
	r.glyphs[name] = g
	return g
}

// bezToPath returns the path of an outline, transformed by t.
func bezToPath(b kurboext.BezPath, t Matrix) Path {
	var p Path
	pt := func(q kurbo.Point) point {
		x, y := t.Apply(q.X, q.Y)
		return point{x, y}
	}
	for _, el := range b.Elements() {
		switch el.Kind {
		case kurbo.PathElMoveTo:
			p.moveTo(pt(el.P[0]))
		case kurbo.PathElLineTo:
			if p.hasCur {
				p.lineTo(pt(el.P[0]))
			}
		case kurbo.PathElQuadTo:
			if p.hasCur {
				c, q, e := p.cur, pt(el.P[0]), pt(el.P[1])
				p.curveTo(point{c.x + 2.0/3*(q.x-c.x), c.y + 2.0/3*(q.y-c.y)},
					point{e.x + 2.0/3*(q.x-e.x), e.y + 2.0/3*(q.y-e.y)}, e)
			}
		case kurbo.PathElCurveTo:
			if p.hasCur {
				p.curveTo(pt(el.P[0]), pt(el.P[1]), pt(el.P[2]))
			}
		case kurbo.PathElClosePath:
			p.closePath()
		}
	}
	return p
}

// glyphName returns the name of a code in the encoding of a font.
func glyphName(d *interp.Dict, code int) string {
	enc, ok := d.GetName("Encoding")
	if !ok || !enc.IsArray() || code < 0 || code >= enc.Len() {
		return ".notdef"
	}
	n := enc.Array()[code]
	switch n.Type() {
	case interp.TName, interp.TString:
		return n.Text()
	}
	return ".notdef"
}

// metricsWidth returns the width of a glyph that the Metrics entry of a
// font gives.
func metricsWidth(d *interp.Dict, name string) (float64, float64, bool) {
	mo, ok := d.GetName("Metrics")
	if !ok || mo.Type() != interp.TDict {
		return 0, 0, false
	}
	v, ok := mo.Dict().GetName(name)
	if !ok {
		return 0, 0, false
	}
	switch {
	case v.IsNumber():
		return v.Float(), 0, true
	case v.IsArray() && v.Len() == 2:
		return v.Array()[1].Float(), 0, true
	case v.IsArray() && v.Len() == 4:
		return v.Array()[2].Float(), v.Array()[3].Float(), true
	}
	return 0, 0, false
}

// showMode is what show does with each glyph.
type showMode int

// The modes of show.
const (
	// modePaint paints the glyphs.
	modePaint showMode = iota
	// modePath adds the outlines to the current path, for charpath.
	modePath
	// modeWidth only measures the advances, for stringwidth.
	modeWidth
)

// showOpts are the options of one of the operators of show.
type showOpts struct {
	mode showMode
	// ax and ay are added to the advance of each glyph, in the user space.
	ax, ay float64
	// cx and cy are added to the advance of the glyph char, or of no
	// glyph when char is -1.
	cx, cy float64
	char   int
	// disp holds the displacements of xshow, xyshow and yshow, and dispX
	// and dispY say which of the axes they give.
	disp         []float64
	dispX, dispY bool
	// kproc is the procedure of kshow, or null.
	kproc interp.Object
	// cproc is the procedure of cshow, or null.
	cproc interp.Object
	// outer is the FontMatrix of a composite font, which applies after the
	// one of a descendant when hasOuter is true.
	outer    Matrix
	hasOuter bool
}

// glyphCapture is the state of a glyph of a Type 3 font that BuildGlyph or
// BuildChar draws.
type glyphCapture struct {
	mode showMode
	// cache is true after setcachedevice: the glyph takes the color of show.
	cache  bool
	color  paint
	wx, wy float64
	// path collects the outlines in the device space for charpath.
	path Path
}

// intercept reports whether the capture takes the marks of the glyph in
// place of the device.
func (c *glyphCapture) intercept() bool {
	return c.cache || c.mode != modePaint
}

// ignoreColor reports whether the marks of the glyph go to fill.
func (c *glyphCapture) ignoreColor() bool {
	return c.intercept()
}

// fill takes a mark of the glyph, in the device space.
func (c *glyphCapture) fill(m *Machine, p *Path, evenOdd bool) error {
	switch c.mode {
	case modeWidth:
		return nil
	case modePath:
		c.path.appendPath(p)
		return nil
	}
	shader := m.shader(c.color)
	op := okagePath(p)
	if shader == nil || op == nil {
		return nil
	}
	rule := okage.FillRuleWinding
	if evenOdd {
		rule = okage.FillRuleEvenOdd
	}
	m.mark()
	m.dev.FillText(op, rule, shader)
	return nil
}

// currentFont returns the font of the graphics state.
func (m *Machine) currentFont() (interp.Object, *fontRec, error) {
	fo := m.gs.font
	rec := fontOf(fo)
	if rec == nil {
		// a program that shows text without setfont gets the default font
		def, err := m.findFont(interp.NameObj("Courier"))
		if err != nil {
			return interp.Null, nil, interp.ErrInvalidFont
		}
		scaled, err := m.scaleFont(def, Matrix{12, 0, 0, 12, 0, 0})
		if err != nil {
			return interp.Null, nil, err
		}
		m.gs.font = scaled
		fo, rec = scaled, fontOf(scaled)
		if rec == nil {
			return interp.Null, nil, interp.ErrInvalidFont
		}
	}
	return fo, rec, nil
}

// findFont returns the font of a name, as findfont does.
func (m *Machine) findFont(key interp.Object) (interp.Object, error) {
	fo, err := m.in.FindResource("Font", key)
	if err == nil && fo.Type() == interp.TDict {
		return fo, nil
	}
	return m.substituteFont(key)
}

// scaleFont returns a copy of a font with its FontMatrix multiplied by mat.
func (m *Machine) scaleFont(fo interp.Object, mat Matrix) (interp.Object, error) {
	d := fo.Dict()
	if d == nil {
		return interp.Null, interp.ErrTypecheck
	}
	if !mat.Finite() {
		return interp.Null, interp.ErrUndefinedResult
	}
	cp := m.in.NewDict(d.Len() + 1)
	c := cp.Dict()
	d.Each(func(k, v interp.Object) bool {
		c.Put(k, v)
		return true
	})
	fm, err := m.newMatrix(fontMatrix(d).Mul(mat))
	if err != nil {
		return interp.Null, err
	}
	c.PutName("FontMatrix", fm)
	if o, ok := d.GetName("OrigFont"); !ok || o.Type() != interp.TDict {
		c.PutName("OrigFont", fo)
	}
	c.SetAccess(interp.ReadOnly)
	return cp, nil
}

// show runs one of the operators of show on a string.
func (m *Machine) show(s []byte, opts showOpts) error {
	fo, rec, err := m.currentFont()
	if err != nil {
		return err
	}
	if opts.mode != modeWidth {
		if err := m.requireCurrent(); err != nil {
			return err
		}
	}
	if rec.typ == 0 {
		return m.showComposite(fo, s, opts)
	}
	di := 0
	if rec.typ == 9 {
		// a CIDFont that cshow makes the current font shows CIDs: of 2
		// bytes, or of 1 byte for a string of odd length
		n := 2
		if len(s)%2 == 1 {
			n = 1
		}
		for i := 0; i+n <= len(s); i += n {
			if err := m.showCID(fo, rec, bytesValue(s[i:i+n]), &opts, &di); err != nil {
				return err
			}
		}
		return nil
	}
	for i, c := range s {
		code := int(c)
		if err := m.showGlyph(fo, rec, code, "", &opts, &di); err != nil {
			return err
		}
		if opts.kproc.Type() != interp.TNull && i+1 < len(s) {
			m.in.Push(interp.Int(code))
			m.in.Push(interp.Int(int(s[i+1])))
			if err := m.in.Call(opts.kproc); err != nil {
				return err
			}
		}
	}
	return nil
}

// showGlyph shows one glyph of a base font at the current point, and moves
// the current point by its advance. A glyph with a name, for glyphshow,
// has the code -1.
func (m *Machine) showGlyph(fo interp.Object, rec *fontRec, code int, name string, opts *showOpts, di *int) error {
	d := fo.Dict()
	if name == "" {
		name = glyphName(d, code)
	}
	fm := fontMatrix(d)
	if opts.hasOuter {
		fm = fm.Mul(opts.outer)
	}
	var wx, wy float64
	if opts.cproc.Type() != interp.TNull {
		wx, wy, err := m.glyphWidth(fo, rec, code, name)
		if err != nil {
			return err
		}
		ux, uy := fm.ApplyDist(wx, wy)
		cur := fo
		if opts.hasOuter {
			// the procedure sees the base font with the matrix of the
			// composite font
			if cur, err = m.scaleFont(fo, opts.outer); err != nil {
				return err
			}
		}
		m.in.Push(interp.Int(code))
		m.in.PushNum(ux)
		m.in.PushNum(uy)
		saved := m.gs.font
		m.gs.font = cur
		err = m.in.Call(opts.cproc)
		m.gs.font = saved
		return err
	}
	if rec.typ == 3 {
		var err error
		if wx, wy, err = m.type3Glyph(fo, code, name, opts.mode); err != nil {
			return err
		}
	} else {
		g := m.glyphOf(rec, name)
		if !g.hasOutline && name != ".notdef" && rec.sub == nil {
			g = m.glyphOf(rec, ".notdef")
		}
		wx, wy = g.wx, g.wy
		if mw, mh, ok := metricsWidth(d, name); ok {
			wx, wy = mw, mh
		}
		if opts.mode != modeWidth && g.hasOutline {
			gm := fm.Mul(m.atCurrent())
			dp := g.path.transform(gm)
			if opts.mode == modePath {
				cur := m.gs.path.cur
				m.gs.path.appendPath(&dp)
				m.gs.path.cur, m.gs.path.hasCur = cur, true
			} else if err := m.fillGlyph(&dp); err != nil {
				return err
			}
		}
	}
	return m.advanceGlyph(fm, wx, wy, code, opts, di)
}

// advanceGlyph moves the current point after a glyph of the code: by its
// width wx, wy in the glyph space of the matrix fm, or by the
// displacement of xshow, plus the additions of ashow and widthshow.
func (m *Machine) advanceGlyph(fm Matrix, wx, wy float64, code int, opts *showOpts, di *int) error {
	ux, uy := fm.ApplyDist(wx, wy)
	if opts.disp != nil {
		ux, uy = 0, 0
		if opts.dispX && *di < len(opts.disp) {
			ux = opts.disp[*di]
			*di++
		}
		if opts.dispY && *di < len(opts.disp) {
			uy = opts.disp[*di]
			*di++
		}
	}
	ux += opts.ax
	uy += opts.ay
	if code == opts.char {
		ux += opts.cx
		uy += opts.cy
	}
	return m.advance(ux, uy)
}

// atCurrent returns the CTM with its origin at the current point.
func (m *Machine) atCurrent() Matrix {
	c := m.gs.ctm
	return Matrix{c[0], c[1], c[2], c[3], m.gs.path.cur.x, m.gs.path.cur.y}
}

// advance moves the current point by a distance of the user space. In the
// mode of stringwidth, the machine keeps the sum in place.
func (m *Machine) advance(ux, uy float64) error {
	if m.measuring != nil {
		m.measuring[0] += ux
		m.measuring[1] += uy
		return nil
	}
	dx, dy := m.gs.ctm.ApplyDist(ux, uy)
	c := m.gs.path.cur
	m.gs.path.cur = point{c.x + dx, c.y + dy}
	if n := len(m.gs.path.segs); n > 0 && m.gs.path.segs[n-1].op == segMove {
		m.gs.path.segs[n-1].p[0] = m.gs.path.cur
	} else {
		m.gs.path.moveTo(m.gs.path.cur)
	}
	return nil
}

// fillGlyph fills the outline of a glyph in the device space with the
// color of the graphics state.
func (m *Machine) fillGlyph(p *Path) error {
	if m.capture != nil && m.capture.intercept() {
		return m.capture.fill(m, p, false)
	}
	shader := m.shader(m.gs.color)
	op := okagePath(p)
	if shader == nil || op == nil {
		return nil
	}
	m.mark()
	m.dev.FillText(op, okage.FillRuleWinding, shader)
	return nil
}

// glyphWidth returns the width of a glyph in the glyph space.
func (m *Machine) glyphWidth(fo interp.Object, rec *fontRec, code int, name string) (float64, float64, error) {
	if mw, mh, ok := metricsWidth(fo.Dict(), name); ok {
		return mw, mh, nil
	}
	if rec.typ == 3 {
		saved := m.measuring
		m.measuring = &[2]float64{}
		defer func() { m.measuring = saved }()
		return m.type3Glyph(fo, code, name, modeWidth)
	}
	g := m.glyphOf(rec, name)
	return g.wx, g.wy, nil
}

// type3Glyph runs the BuildGlyph or BuildChar procedure of a Type 3 font at
// the current point, and returns the width that the procedure gives.
func (m *Machine) type3Glyph(fo interp.Object, code int, name string, mode showMode) (float64, float64, error) {
	d := fo.Dict()
	proc, byName := d.GetName("BuildGlyph")
	if !byName || !proc.IsArray() {
		var ok bool
		if proc, ok = d.GetName("BuildChar"); !ok {
			return 0, 0, interp.ErrInvalidFont
		}
		byName = false
	}
	if m.type3Depth >= 8 {
		return 0, 0, interp.ErrLimitcheck
	}
	m.type3Depth++
	defer func() { m.type3Depth-- }()
	capture := &glyphCapture{mode: mode, color: m.gs.color}
	if m.capture != nil && m.capture.mode != modePaint {
		capture.mode = m.capture.mode
	}
	prev := m.capture
	m.capture = capture
	cur := m.gs.path.cur
	hasCur := m.gs.path.hasCur
	measuring := m.measuring
	m.measuring = nil
	m.gsave()
	if hasCur {
		m.gs.ctm = fontMatrix(d).Mul(m.atCurrent())
	} else {
		m.gs.ctm = fontMatrix(d).Mul(m.gs.ctm)
	}
	m.gs.path.reset()
	m.in.Push(fo)
	if byName && name != "" {
		m.in.Push(interp.NameObj(name))
	} else {
		if code < 0 {
			code = 0
		}
		m.in.Push(interp.Int(code))
	}
	err := m.in.Call(proc)
	m.grestore()
	m.measuring = measuring
	m.capture = prev
	m.gs.path.cur, m.gs.path.hasCur = cur, hasCur
	if err != nil && !interp.IsSignal(err) {
		return 0, 0, err
	}
	if mode == modePath && len(capture.path.segs) > 0 {
		m.gs.path.appendPath(&capture.path)
		m.gs.path.cur, m.gs.path.hasCur = cur, hasCur
	}
	if prev != nil && prev.mode == modePath && mode == modePaint {
		prev.path.appendPath(&capture.path)
	}
	return capture.wx, capture.wy, err
}

// encoding returns an encoding vector of systemdict.
func (m *Machine) encoding(name string) (interp.Object, bool) {
	v, ok := m.in.SystemDict.GetName(name)
	return v, ok && v.IsArray()
}

// opsFont are the operators of fonts and of text (PLRM 8.1).
func (m *Machine) opsFont() {
	in := m.in
	m.setupEncodings()
	fonts := in.Category("Font")
	in.SystemDict.PutName("FontDirectory", interp.DictObj(fonts))
	in.SystemDict.PutName("GlobalFontDirectory", interp.DictObj(fonts))
	in.SystemDict.PutName("SharedFontDirectory", interp.DictObj(fonts))
	in.SetResourceHooks("Font", interp.ResourceHooks{
		Define: m.defineFont,
		Find: func(key interp.Object) (interp.Object, bool, error) {
			fo, err := m.substituteFont(key)
			return fo, err == nil, err
		},
	})
	in.SetResourceHooks("CIDFont", interp.ResourceHooks{Define: m.defineCIDFont})
	m.setupCIDInit()
	in.Define("composefont", func(*interp.Interp) error {
		if in.Count() < 3 {
			return interp.ErrStackUnderflow
		}
		descs, _ := in.Pop()
		cmap, _ := in.Pop()
		key, _ := in.Pop()
		if !descs.IsArray() {
			return interp.ErrTypecheck
		}
		var fonts []interp.Object
		for _, e := range descs.Array() {
			var fo interp.Object
			var err error
			if e.Type() == interp.TDict {
				fo = e
			} else if fo, err = in.FindResource("CIDFont", e); err != nil {
				if fo, err = m.findFont(e); err != nil {
					return err
				}
			}
			fonts = append(fonts, fo)
		}
		if cmap.Type() != interp.TDict {
			v, err := in.FindResource("CMap", cmap)
			if err != nil {
				return err
			}
			cmap = v
		}
		fo := in.NewDict(8)
		d := fo.Dict()
		d.PutName("FontType", interp.Int(0))
		d.PutName("FMapType", interp.Int(9))
		d.PutName("FontName", key)
		fm, err := m.newMatrix(identity)
		if err != nil {
			return err
		}
		d.PutName("FontMatrix", fm)
		d.PutName("CMap", cmap)
		dv, err := in.NewArray(fonts)
		if err != nil {
			return err
		}
		d.PutName("FDepVector", dv)
		enc := make([]interp.Object, len(fonts))
		for i := range enc {
			enc[i] = interp.Int(i)
		}
		ev, err := in.NewArray(enc)
		if err != nil {
			return err
		}
		d.PutName("Encoding", ev)
		if _, err := m.defineFont(key, fo); err != nil {
			return err
		}
		fonts0 := in.Category("Font")
		fonts0.Put(key, fo)
		in.Push(fo)
		return nil
	})
	in.SetResourceHooks("Encoding", interp.ResourceHooks{
		Find: func(key interp.Object) (interp.Object, bool, error) {
			v, ok := m.encoding(key.Text())
			return v, ok, nil
		},
	})
	in.Define("findfont", func(*interp.Interp) error {
		key, err := in.Peek(0)
		if err != nil {
			return err
		}
		fo, err := m.findFont(key)
		if err != nil {
			return err
		}
		_, _ = in.Pop()
		in.Push(fo)
		return nil
	})
	in.Define("definefont", func(*interp.Interp) error {
		fo, err := in.Peek(0)
		if err != nil {
			return err
		}
		key, err := in.Peek(1)
		if err != nil {
			return err
		}
		if _, err := m.defineFont(key, fo); err != nil {
			return err
		}
		fonts.Put(key, fo)
		_, _ = in.Pop()
		_, _ = in.Pop()
		in.Push(fo)
		return nil
	})
	in.Define("undefinefont", func(*interp.Interp) error {
		key, err := in.Pop()
		if err != nil {
			return err
		}
		fonts.Delete(key)
		return nil
	})
	in.Define("scalefont", func(*interp.Interp) error {
		s, err := in.PopNum()
		if err != nil {
			return err
		}
		fo, err := in.PopType(interp.TDict)
		if err != nil {
			return err
		}
		out, err := m.scaleFont(fo, Matrix{s, 0, 0, s, 0, 0})
		if err != nil {
			return err
		}
		in.Push(out)
		return nil
	})
	in.Define("makefont", func(*interp.Interp) error {
		mat, _, err := m.popMatrix()
		if err != nil {
			return err
		}
		fo, err := in.PopType(interp.TDict)
		if err != nil {
			return err
		}
		out, err := m.scaleFont(fo, mat)
		if err != nil {
			return err
		}
		in.Push(out)
		return nil
	})
	in.Define("setfont", func(*interp.Interp) error {
		fo, err := in.PopType(interp.TDict)
		if err != nil {
			return err
		}
		m.gs.font = fo
		return nil
	})
	in.Define("selectfont", func(*interp.Interp) error {
		top, err := in.Pop()
		if err != nil {
			return err
		}
		key, err := in.Pop()
		if err != nil {
			return err
		}
		fo, err := m.findFont(key)
		if err != nil {
			return err
		}
		var mat Matrix
		switch {
		case top.IsNumber():
			mat = Matrix{top.Float(), 0, 0, top.Float(), 0, 0}
		default:
			if mat, err = matrixOf(top); err != nil {
				return err
			}
		}
		out, err := m.scaleFont(fo, mat)
		if err != nil {
			return err
		}
		m.gs.font = out
		return nil
	})
	in.Define("currentfont", func(*interp.Interp) error {
		fo, _, err := m.currentFont()
		if err != nil {
			return err
		}
		in.Push(fo)
		return nil
	})
	in.Define("rootfont", func(*interp.Interp) error {
		fo, _, err := m.currentFont()
		if err != nil {
			return err
		}
		in.Push(fo)
		return nil
	})
	str := func() ([]byte, error) {
		o, err := in.PopType(interp.TString)
		if err != nil {
			return nil, err
		}
		return o.Bytes(), nil
	}
	nullOpts := func() showOpts {
		return showOpts{char: -1, kproc: interp.Null, cproc: interp.Null}
	}
	in.Define("show", func(*interp.Interp) error {
		s, err := str()
		if err != nil {
			return err
		}
		return m.show(s, nullOpts())
	})
	in.Define("ashow", func(*interp.Interp) error {
		s, err := str()
		if err != nil {
			return err
		}
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		o := nullOpts()
		o.ax, o.ay = v[0], v[1]
		return m.show(s, o)
	})
	in.Define("widthshow", func(*interp.Interp) error {
		s, err := str()
		if err != nil {
			return err
		}
		c, err := in.PopInt()
		if err != nil {
			return err
		}
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		o := nullOpts()
		o.cx, o.cy, o.char = v[0], v[1], c
		return m.show(s, o)
	})
	in.Define("awidthshow", func(*interp.Interp) error {
		s, err := str()
		if err != nil {
			return err
		}
		a, err := in.PopNums(2)
		if err != nil {
			return err
		}
		c, err := in.PopInt()
		if err != nil {
			return err
		}
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		o := nullOpts()
		o.ax, o.ay = a[0], a[1]
		o.cx, o.cy, o.char = v[0], v[1], c
		return m.show(s, o)
	})
	disp := func(name string, x, y bool) {
		in.Define(name, func(*interp.Interp) error {
			a, err := in.Pop()
			if err != nil {
				return err
			}
			s, err := str()
			if err != nil {
				return err
			}
			o := nullOpts()
			o.dispX, o.dispY = x, y
			o.disp = []float64{}
			switch {
			case a.IsArray():
				for _, e := range a.Array() {
					o.disp = append(o.disp, e.Float())
				}
			case a.Type() == interp.TString:
				o.disp = decodeNumberString(a.Bytes())
			default:
				return interp.ErrTypecheck
			}
			return m.show(s, o)
		})
	}
	disp("xshow", true, false)
	disp("yshow", false, true)
	disp("xyshow", true, true)
	in.Define("kshow", func(*interp.Interp) error {
		s, err := str()
		if err != nil {
			return err
		}
		p, err := in.PopProc()
		if err != nil {
			return err
		}
		o := nullOpts()
		o.kproc = p
		return m.show(s, o)
	})
	in.Define("cshow", func(*interp.Interp) error {
		s, err := str()
		if err != nil {
			return err
		}
		p, err := in.PopProc()
		if err != nil {
			return err
		}
		o := nullOpts()
		o.cproc = p
		o.mode = modeWidth
		saved := m.measuring
		m.measuring = nil
		defer func() { m.measuring = saved }()
		return m.show(s, o)
	})
	in.Define("glyphshow", func(*interp.Interp) error {
		n, err := in.Pop()
		if err != nil {
			return err
		}
		fo, rec, err := m.currentFont()
		if err != nil {
			return err
		}
		if err := m.requireCurrent(); err != nil {
			return err
		}
		o := nullOpts()
		di := 0
		if rec.typ == 0 {
			return nil
		}
		return m.showGlyph(fo, rec, -1, n.Text(), &o, &di)
	})
	in.Define("stringwidth", func(*interp.Interp) error {
		s, err := str()
		if err != nil {
			return err
		}
		saved := m.measuring
		m.measuring = &[2]float64{}
		o := nullOpts()
		o.mode = modeWidth
		err = m.show(s, o)
		w := *m.measuring
		m.measuring = saved
		if err != nil {
			return err
		}
		in.PushNum(w[0])
		in.PushNum(w[1])
		return nil
	})
	in.Define("charpath", func(*interp.Interp) error {
		if _, err := in.PopBool(); err != nil {
			return err
		}
		s, err := str()
		if err != nil {
			return err
		}
		o := nullOpts()
		o.mode = modePath
		return m.show(s, o)
	})
	in.Define("setcachedevice", func(*interp.Interp) error {
		v, err := in.PopNums(6)
		if err != nil {
			return err
		}
		if m.capture != nil {
			m.capture.wx, m.capture.wy, m.capture.cache = v[0], v[1], true
		}
		return nil
	})
	in.Define("setcachedevice2", func(*interp.Interp) error {
		v, err := in.PopNums(10)
		if err != nil {
			return err
		}
		if m.capture != nil {
			m.capture.wx, m.capture.wy, m.capture.cache = v[0], v[1], true
		}
		return nil
	})
	in.Define("setcharwidth", func(*interp.Interp) error {
		v, err := in.PopNums(2)
		if err != nil {
			return err
		}
		if m.capture != nil {
			m.capture.wx, m.capture.wy = v[0], v[1]
		}
		return nil
	})
	in.Define("setcachelimit", func(*interp.Interp) error {
		_, err := in.PopInt()
		return err
	})
	in.Define("setcacheparams", func(*interp.Interp) error {
		return popToMark(in)
	})
	in.Define("currentcacheparams", func(*interp.Interp) error {
		in.Push(interp.Mark)
		in.Push(interp.Int(1 << 20))
		in.Push(interp.Int(1 << 10))
		return nil
	})
	in.Define("cachestatus", func(*interp.Interp) error {
		for _, v := range []int{0, 1 << 20, 0, 1 << 10, 0, 1 << 10, 1 << 10} {
			in.Push(interp.Int(v))
		}
		return nil
	})
}

// decodeNumberString decodes an encoded number string of xshow (PLRM
// 3.14.5). It reads the form of 16-bit and 32-bit integers and reals.
func decodeNumberString(b []byte) []float64 {
	if len(b) < 4 || b[0] != 149 {
		return nil
	}
	rep := int(b[1])
	n := int(b[2])<<8 | int(b[3])
	if rep >= 128 {
		n = int(b[3])<<8 | int(b[2])
	}
	data := b[4:]
	var out []float64
	for i := 0; i < n; i++ {
		switch {
		case rep%128 < 32:
			if len(data) < 4 {
				return out
			}
			v := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
			if rep >= 128 {
				v = uint32(data[3])<<24 | uint32(data[2])<<16 | uint32(data[1])<<8 | uint32(data[0])
			}
			out = append(out, float64(int32(v))/math.Pow(2, float64(rep%128)))
			data = data[4:]
		case rep%128 < 48:
			if len(data) < 2 {
				return out
			}
			v := uint16(data[0])<<8 | uint16(data[1])
			if rep >= 128 {
				v = uint16(data[1])<<8 | uint16(data[0])
			}
			out = append(out, float64(int16(v))/math.Pow(2, float64(rep%128-32)))
			data = data[2:]
		default:
			if len(data) < 4 {
				return out
			}
			v := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
			if rep >= 128 {
				v = uint32(data[3])<<24 | uint32(data[2])<<16 | uint32(data[1])<<8 | uint32(data[0])
			}
			out = append(out, float64(math.Float32frombits(v)))
			data = data[4:]
		}
	}
	return out
}
