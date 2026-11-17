package graphics

import (
	"github.com/xo/ivy/internal/hayro/pdffont"
	"github.com/xo/ivy/ps/internal/interp"
)

// maxCIDRanges bounds the ranges and the characters of a CMap.
const maxCIDRanges = 1 << 16

// codeRange is a range of codes of a CMap, of one length in bytes.
type codeRange struct {
	n      int
	lo, hi uint32
	// cid is the CID of lo, for a range of CIDs.
	cid uint32
}

// cmapData holds the mappings of a CMap (PLRM 5.11.4).
type cmapData struct {
	spaces []codeRange
	ranges []codeRange
	// identity maps each code of 2 bytes to the same CID.
	identity bool
	// vertical is true for a CMap of vertical writing.
	vertical bool
}

// bytesValue returns the value of a string of bytes, big-endian.
func bytesValue(b []byte) uint32 {
	var v uint32
	for _, c := range b {
		v = v<<8 | uint32(c)
	}
	return v
}

// next returns the code at the start of s, its length in bytes, and its
// CID.
func (c *cmapData) next(s []byte) (code uint32, n int, cid uint32) {
	n = 0
	for _, sp := range c.spaces {
		if sp.n <= len(s) {
			v := bytesValue(s[:sp.n])
			if v >= sp.lo && v <= sp.hi && (n == 0 || sp.n < n) {
				n = sp.n
			}
		}
	}
	if n == 0 {
		n = 1
		if c.identity || len(c.spaces) == 0 {
			n = min(2, len(s))
		}
	}
	code = bytesValue(s[:n])
	if c.identity {
		return code, n, code
	}
	for i := len(c.ranges) - 1; i >= 0; i-- {
		r := c.ranges[i]
		if r.n == n && code >= r.lo && code <= r.hi {
			return code, n, r.cid + code - r.lo
		}
	}
	return code, n, 0
}

// cmapOf returns the mappings of a CMap dictionary.
func (m *Machine) cmapOf(o interp.Object) *cmapData {
	d := o.Dict()
	if d == nil {
		if o.Type() == interp.TName {
			if v, err := m.in.FindResource("CMap", o); err == nil {
				return m.cmapOf(v)
			}
		}
		return &cmapData{identity: true}
	}
	if c, ok := m.cmaps[d]; ok {
		return c
	}
	return &cmapData{identity: true}
}

// cmapBuilding returns the mappings of the CMap that the current
// dictionary builds.
func (m *Machine) cmapBuilding() *cmapData {
	d := m.in.CurrentDict()
	c, ok := m.cmaps[d]
	if !ok {
		c = &cmapData{}
		m.cmaps[d] = c
	}
	return c
}

// popGroups pops the objects down to a mark in groups of n, and returns
// them from the bottom.
func popGroups(in *interp.Interp, n int) ([][]interp.Object, error) {
	var objs []interp.Object
	for {
		o, err := in.Pop()
		if err != nil {
			return nil, interp.ErrUnmatchedMark
		}
		if o.Type() == interp.TMark {
			break
		}
		objs = append(objs, o)
	}
	var out [][]interp.Object
	for i := len(objs) - 1; i-n+1 >= 0; i -= n {
		g := make([]interp.Object, n)
		for j := range n {
			g[j] = objs[i-j]
		}
		out = append(out, g)
	}
	return out, nil
}

// setupCIDInit defines the procedure set CIDInit, with the operators that
// build a CMap, and the CMaps Identity-H and Identity-V.
func (m *Machine) setupCIDInit() {
	in := m.in
	old := in.SetGlobal(true)
	defer in.SetGlobal(old)
	ps := in.NewDict(32)
	d := ps.Dict()
	def := func(name string, fn func() error) {
		d.PutName(name, interp.OpObj(&interp.Operator{Name: name, Fn: func(*interp.Interp) error { return fn() }}))
	}
	begin := func() error {
		if _, err := in.PopInt(); err != nil {
			return err
		}
		in.Push(interp.Mark)
		return nil
	}
	for _, name := range []string{"begincodespacerange", "begincidrange", "begincidchar", "beginnotdefrange",
		"beginnotdefchar", "beginbfchar", "beginbfrange", "beginrearrangedfont", "beginusematrix"} {
		def(name, begin)
	}
	def("begincmap", func() error { return nil })
	def("endcmap", func() error { return nil })
	def("endcodespacerange", func() error {
		groups, err := popGroups(in, 2)
		if err != nil {
			return err
		}
		c := m.cmapBuilding()
		for _, g := range groups {
			lo, hi := g[0].Bytes(), g[1].Bytes()
			if len(lo) == 0 || len(lo) > 4 || len(c.spaces) >= maxCIDRanges {
				continue
			}
			c.spaces = append(c.spaces, codeRange{n: len(lo), lo: bytesValue(lo), hi: bytesValue(hi)})
		}
		return nil
	})
	def("endcidrange", func() error {
		groups, err := popGroups(in, 3)
		if err != nil {
			return err
		}
		c := m.cmapBuilding()
		for _, g := range groups {
			lo, hi := g[0].Bytes(), g[1].Bytes()
			if len(lo) == 0 || len(lo) > 4 || !g[2].IsNumber() || len(c.ranges) >= maxCIDRanges {
				continue
			}
			c.ranges = append(c.ranges, codeRange{n: len(lo), lo: bytesValue(lo), hi: bytesValue(hi), cid: uint32(g[2].Float())})
		}
		return nil
	})
	def("endcidchar", func() error {
		groups, err := popGroups(in, 2)
		if err != nil {
			return err
		}
		c := m.cmapBuilding()
		for _, g := range groups {
			b := g[0].Bytes()
			if len(b) == 0 || len(b) > 4 || !g[1].IsNumber() || len(c.ranges) >= maxCIDRanges {
				continue
			}
			v := bytesValue(b)
			c.ranges = append(c.ranges, codeRange{n: len(b), lo: v, hi: v, cid: uint32(g[1].Float())})
		}
		return nil
	})
	for _, name := range []string{"endnotdefrange", "endnotdefchar", "endbfchar", "endbfrange",
		"endrearrangedfont", "endusematrix"} {
		def(name, func() error { return popToMark(in) })
	}
	def("usecmap", func() error {
		key, err := in.Pop()
		if err != nil {
			return err
		}
		src := m.cmapOf(key)
		c := m.cmapBuilding()
		c.spaces = append(append([]codeRange(nil), src.spaces...), c.spaces...)
		c.ranges = append(append([]codeRange(nil), src.ranges...), c.ranges...)
		c.identity = c.identity || src.identity
		return nil
	})
	def("usefont", func() error {
		_, err := in.PopInt()
		return err
	})
	def("StartData", func() error {
		return m.startData()
	})
	in.Category("ProcSet").PutName("CIDInit", ps)
	for _, name := range []string{"Identity-H", "Identity-V"} {
		cm := in.NewDict(8)
		cm.Dict().PutName("CMapName", interp.NameObj(name))
		cm.Dict().PutName("CMapType", interp.Int(1))
		wmode := 0
		if name == "Identity-V" {
			wmode = 1
		}
		cm.Dict().PutName("WMode", interp.Int(wmode))
		m.cmaps[cm.Dict()] = &cmapData{identity: true, vertical: wmode == 1}
		in.Category("CMap").PutName(name, cm)
	}
}

// startData reads the binary data of a CIDFont of type 0 after
// StartData: (Binary) length StartData, and keeps it in the CIDFont that
// the current dictionary builds (PLRM 5.11.3).
func (m *Machine) startData() error {
	in := m.in
	n, err := in.PopInt()
	if err != nil {
		return err
	}
	if _, err := in.Pop(); err != nil {
		return err
	}
	if n < 0 || int64(n) > m.lim.MaxMemory {
		return interp.ErrLimitcheck
	}
	cur, ok := in.Lookup("currentfile")
	if !ok {
		return interp.ErrUndefined
	}
	if err := in.Call(cur); err != nil {
		return err
	}
	fo, err := in.PopType(interp.TFile)
	if err != nil {
		return err
	}
	f := fo.File()
	data := make([]byte, 0, min(n, 1<<20))
	for len(data) < n {
		b, err := f.ReadByte()
		if err != nil {
			break
		}
		data = append(data, b)
	}
	s, err := in.NewString(data)
	if err != nil {
		return err
	}
	d := in.CurrentDict()
	d.PutName("GlyphData", s)
	if name, ok := d.GetName("CIDFontName"); ok {
		cf := interp.DictObj(d)
		if _, err := m.defineCIDFont(name, cf); err == nil {
			in.Category("CIDFont").Put(name, cf)
		}
	}
	return nil
}

// defineCIDFont makes the font of a CIDFont dictionary, for defineresource.
func (m *Machine) defineCIDFont(_ interp.Object, fo interp.Object) (interp.Object, error) {
	d := fo.Dict()
	if d == nil {
		return interp.Null, interp.ErrTypecheck
	}
	typ := 0
	if v, ok := d.GetName("CIDFontType"); ok && v.IsNumber() {
		typ = int(v.Float())
	}
	rec := &fontRec{dict: d, typ: 9, cidType: typ}
	d.PutName("FID", interp.FontID(rec))
	return fo, nil
}

// cidGlyph returns the glyph of a CID of a CIDFont, in its glyph space.
func (m *Machine) cidGlyph(rec *fontRec, cid uint32) *glyph {
	key := "\x00cid"
	key += string(rune(cid))
	if g, ok := rec.glyphs[key]; ok {
		return g
	}
	g := &glyph{}
	switch rec.cidType {
	case 2:
		if !rec.loaded {
			rec.loaded = true
			rec.prog = type42Program(rec.dict)
		}
		if rec.prog != nil {
			gid := cidToGID(rec.dict, cid)
			s := 1 / float64(pdffont.UnitsPerEm)
			g.path, g.hasOutline = bezToPath(rec.prog.Outline(pdffont.GlyphID(gid)), Matrix{s, 0, 0, s, 0, 0}), true
			if adv, ok := rec.prog.Advance(pdffont.GlyphID(gid)); ok {
				g.wx, g.hasWidth = float64(adv)*s, true
			}
		}
	case 0:
		m.cidType0Glyph(rec, cid, g)
	}
	if !g.hasWidth {
		g.wx, g.hasWidth = 1, true
		if fm := fontMatrix(rec.dict); fm[0] < 0.1 {
			g.wx = 1000
		}
	}
	if rec.glyphs == nil || len(rec.glyphs) >= maxCachedGlyphs {
		rec.glyphs = map[string]*glyph{}
	}
	rec.glyphs[key] = g
	return g
}

// cidToGID returns the glyph of a CID of a CIDFont of type 2, through its
// CIDMap.
func cidToGID(d *interp.Dict, cid uint32) uint32 {
	cm, ok := d.GetName("CIDMap")
	if !ok {
		return cid
	}
	gdBytes := 2
	if v, ok := d.GetName("GDBytes"); ok && v.IsNumber() {
		gdBytes = min(max(int(v.Float()), 1), 4)
	}
	read := func(b []byte, off int) (uint32, bool) {
		if off < 0 || off+gdBytes > len(b) {
			return 0, false
		}
		return bytesValue(b[off : off+gdBytes]), true
	}
	off := int(cid) * gdBytes
	switch {
	case cm.IsNumber():
		return cid + uint32(cm.Float())
	case cm.Type() == interp.TString:
		if g, ok := read(cm.Bytes(), off); ok {
			return g
		}
	case cm.IsArray():
		for _, s := range cm.Array() {
			b := s.Bytes()
			if off < len(b) {
				if g, ok := read(b, off); ok {
					return g
				}
				return 0
			}
			off -= len(b)
		}
	case cm.Type() == interp.TDict:
		if v, ok := cm.Dict().Get(interp.Int(int(cid))); ok && v.IsNumber() {
			return uint32(v.Float())
		}
	}
	return 0
}

// cidType0Glyph reads the Type 1 charstring of a CID of a CIDFont of type
// 0 from its GlyphData.
func (m *Machine) cidType0Glyph(rec *fontRec, cid uint32, g *glyph) {
	d := rec.dict
	geti := func(dd *interp.Dict, key string, def int) int {
		if v, ok := dd.GetName(key); ok && v.IsNumber() {
			return int(v.Float())
		}
		return def
	}
	gd, ok := d.GetName("GlyphData")
	if !ok || gd.Type() != interp.TString {
		return
	}
	data := gd.Bytes()
	fdBytes, gdBytes := geti(d, "FDBytes", 1), geti(d, "GDBytes", 2)
	if fdBytes < 0 || fdBytes > 4 || gdBytes < 1 || gdBytes > 4 {
		return
	}
	entry := fdBytes + gdBytes
	off := geti(d, "CIDMapOffset", 0) + int(cid)*entry
	if off < 0 || off+2*entry > len(data) {
		return
	}
	fd := int(bytesValue(data[off : off+fdBytes]))
	start := int(bytesValue(data[off+fdBytes : off+entry]))
	end := int(bytesValue(data[off+entry+fdBytes : off+2*entry]))
	if start < 0 || end > len(data) || start >= end {
		return
	}
	fda, ok := d.GetName("FDArray")
	if !ok || !fda.IsArray() || fd >= fda.Len() || fda.Array()[fd].Type() != interp.TDict {
		return
	}
	fdDict := fda.Array()[fd].Dict()
	lenIV := 4
	var subrs [][]byte
	if p, ok := fdDict.GetName("Private"); ok && p.Type() == interp.TDict {
		pd := p.Dict()
		lenIV = geti(pd, "lenIV", 4)
		sOff, sCount, sdBytes := geti(pd, "SubrMapOffset", 0), geti(pd, "SubrCount", 0), geti(pd, "SDBytes", 0)
		if sdBytes > 0 && sdBytes <= 4 && sCount > 0 && sCount < 1<<16 {
			for i := range sCount {
				o := sOff + i*sdBytes
				if o+2*sdBytes > len(data) {
					break
				}
				a, b := int(bytesValue(data[o:o+sdBytes])), int(bytesValue(data[o+sdBytes:o+2*sdBytes]))
				if a < 0 || b > len(data) || a > b {
					subrs = append(subrs, nil)
					continue
				}
				subrs = append(subrs, data[a:b])
			}
		}
	}
	prog, err := pdffont.NewType1Charstrings([]string{"g"}, [][]byte{data[start:end]}, subrs, lenIV, nil)
	if err != nil {
		return
	}
	outline, adv, ok := prog.Type1Glyph(0)
	fm := fontMatrix(fdDict)
	g.path, g.hasOutline = bezToPath(outline, fm), true
	if ok {
		g.wx, g.wy = fm.ApplyDist(adv.X, adv.Y)
		g.hasWidth = true
	}
}

// showComposite shows a string with a composite font of type 0 (PLRM
// 5.10).
func (m *Machine) showComposite(fo interp.Object, s []byte, opts showOpts) error {
	d := fo.Dict()
	root := fontMatrix(d)
	fmap := 2
	if v, ok := d.GetName("FMapType"); ok && v.IsNumber() {
		fmap = int(v.Float())
	}
	fdep, ok := d.GetName("FDepVector")
	if !ok || !fdep.IsArray() || fdep.Len() == 0 {
		return interp.ErrInvalidFont
	}
	desc := fdep.Array()
	var enc []interp.Object
	if e, ok := d.GetName("Encoding"); ok && e.IsArray() {
		enc = e.Array()
	}
	font := func(i int) (interp.Object, *fontRec) {
		if i >= 0 && i < len(enc) && enc[i].IsNumber() {
			i = int(enc[i].Float())
		}
		if i < 0 || i >= len(desc) {
			return interp.Null, nil
		}
		return desc[i], fontOf(desc[i])
	}
	esc := 255
	if v, ok := d.GetName("EscChar"); ok && v.IsNumber() {
		esc = int(v.Float())
	}
	var cm *cmapData
	if fmap == 9 {
		cmo, _ := d.GetName("CMap")
		cm = m.cmapOf(cmo)
	}
	opts.outer, opts.hasOuter = root, true
	current := 0
	di := 0
	for i := 0; i < len(s); {
		var fi, code int
		var cid uint32
		isCID := false
		switch fmap {
		case 2:
			if i+1 >= len(s) {
				return nil
			}
			fi, code = int(s[i]), int(s[i+1])
			i += 2
		case 4:
			fi, code = int(s[i]>>7), int(s[i]&0x7f)
			i++
		case 5:
			if i+1 >= len(s) {
				return nil
			}
			v := int(s[i])<<8 | int(s[i+1])
			fi, code = v>>7, v&0x7f
			i += 2
		case 3, 7:
			if int(s[i]) == esc && i+1 < len(s) {
				current = int(s[i+1])
				i += 2
				continue
			}
			fi, code = current, int(s[i])
			i++
		case 8:
			switch s[i] {
			case 14:
				current = 1
				i++
				continue
			case 15:
				current = 0
				i++
				continue
			}
			fi, code = current, int(s[i])
			i++
		case 9:
			c, n, id := cm.next(s[i:])
			fi, code, cid, isCID = 0, int(c), id, true
			i += n
		default:
			fi, code = 0, int(s[i])
			i++
		}
		dfo, drec := font(fi)
		if drec == nil {
			continue
		}
		var err error
		switch drec.typ {
		case 9:
			if !isCID {
				cid = uint32(code)
			}
			err = m.showCID(dfo, drec, cid, &opts, &di)
		case 0:
			// a nested composite font draws nothing
			continue
		default:
			err = m.showGlyph(dfo, drec, code, "", &opts, &di)
		}
		if err != nil {
			return err
		}
		if opts.kproc.Type() != interp.TNull && i < len(s) {
			m.in.Push(interp.Int(code))
			m.in.Push(interp.Int(int(s[i])))
			if err := m.in.Call(opts.kproc); err != nil {
				return err
			}
		}
	}
	return nil
}

// showCID shows the glyph of a CID of a CIDFont at the current point.
func (m *Machine) showCID(fo interp.Object, rec *fontRec, cid uint32, opts *showOpts, di *int) error {
	d := fo.Dict()
	fm := fontMatrix(d)
	if opts.hasOuter {
		fm = fm.Mul(opts.outer)
	}
	var wx, wy float64
	if opts.cproc.Type() != interp.TNull {
		g := m.cidGlyph(rec, cid)
		ux, uy := fm.ApplyDist(g.wx, g.wy)
		m.in.Push(interp.Int(int(cid)))
		m.in.PushNum(ux)
		m.in.PushNum(uy)
		saved := m.gs.font
		cur, err := m.scaleFont(fo, opts.outer)
		if err != nil {
			return err
		}
		m.gs.font = cur
		err = m.in.Call(opts.cproc)
		m.gs.font = saved
		return err
	}
	if _, ok := d.GetName("BuildGlyph"); ok && rec.cidType == 1 {
		saved := m.gs.font
		var err error
		wx, wy, err = m.type3Glyph(fo, int(cid), "", opts.mode)
		m.gs.font = saved
		if err != nil {
			return err
		}
	} else {
		g := m.cidGlyph(rec, cid)
		wx, wy = g.wx, g.wy
		if opts.mode != modeWidth && g.hasOutline {
			dp := g.path.transform(fm.Mul(m.atCurrent()))
			if opts.mode == modePath {
				cur := m.gs.path.cur
				m.gs.path.appendPath(&dp)
				m.gs.path.cur, m.gs.path.hasCur = cur, true
			} else if err := m.fillGlyph(&dp); err != nil {
				return err
			}
		}
	}
	return m.advanceGlyph(fm, wx, wy, int(cid), opts, di)
}
