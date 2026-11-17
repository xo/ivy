// Package graphics is the imaging model of PostScript (D29): the graphics
// state, the paths, the painting, the colors, the images, the patterns,
// the shadings, the fonts and the pages. It adds its operators to an
// interpreter of ps/internal/interp, and it draws each page onto the
// display list of D10.
//
// The device space of a page has resolutionScale pixels for each point,
// with y down from the top left corner. A page records its commands in the
// device space, and a renderer scales them back to points.
package graphics

import (
	"log/slog"
	"math"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/ps/internal/interp"
)

// resolutionScale is the number of the pixels of the device space for each
// point: the device has 300 pixels for each inch, as many printers do.
const resolutionScale = 300.0 / 72

// ResolutionScale is the number of the pixels of the device space for each
// point, which a renderer of the pages scales back.
const ResolutionScale = resolutionScale

// Page is a page that showpage gave.
type Page struct {
	// Width and Height are the size of the page in points.
	Width, Height float64
	// List holds the commands of the page, in the device space.
	List *Recorder
	// Err is the error that ended the page early, or nil.
	Err error
}

// Setup is the setup of the page device at the start of a program.
type Setup struct {
	// Width and Height are the size of the page in points.
	Width, Height float64
	// X and Y are the corner of the page in the default user space. An EPS
	// file draws its bounding box from (X, Y).
	X, Y float64
	// FirstPageOnly stops the program after the first page, as an EPS file
	// needs.
	FirstPageOnly bool
}

// Config is the configuration of a machine.
type Config struct {
	// Setup is the page device at the start.
	Setup Setup
	// Fonts finds the fonts that a program names and does not define.
	Fonts FontSource
	// Log receives the warnings.
	Log *slog.Logger
}

// Machine is the imaging model of an interpreter. It is not safe for
// concurrent use.
type Machine struct {
	in  *interp.Interp
	lim ivy.Limits
	log *slog.Logger
	cfg Config

	gs    *gstate
	stack []*gstate

	// dev is the device of the current page, or of a pattern or a glyph
	// that the machine draws.
	dev *Recorder
	// applied is the clip that the device holds, as dev received it.
	applied *clipNode
	// width and height are the size of the current page in points.
	width, height float64
	// originX and originY are the corner of the page in the default user
	// space.
	originX, originY float64
	// marked is true when the current page holds a mark.
	marked bool
	pages  []Page
	// done is true when the program gave all the pages that the caller
	// wants.
	done bool

	pageDevice interp.Object
	fonts      *fontCache
	// capture is the state of the glyph of a Type 3 font that BuildChar
	// draws, or nil.
	capture *glyphCapture
	// measuring sums the advances of stringwidth in the user space, or is
	// nil.
	measuring *[2]float64
	// type3Depth is the depth of the glyphs of Type 3 fonts that the
	// machine draws.
	type3Depth int
	// cmaps holds the mappings of each CMap dictionary.
	cmaps map[*interp.Dict]*cmapData
	// patternDepth is the depth of the patterns that the machine draws.
	patternDepth int
	// pixels counts the pixels of the images of the document.
	pixels int64
}

// New adds the operators of the imaging model to in, and returns the
// machine. The caller calls in.Finish after New.
func New(in *interp.Interp, cfg Config) *Machine {
	if cfg.Log == nil {
		cfg.Log = in.Log()
	}
	s := cfg.Setup
	if s.Width <= 0 || s.Height <= 0 || math.IsNaN(s.Width+s.Height) || s.Width > maxPageSide || s.Height > maxPageSide {
		s.Width, s.Height = 612, 792
	}
	cfg.Setup = s
	m := &Machine{
		in:      in,
		lim:     in.Limits(),
		log:     cfg.Log,
		cfg:     cfg,
		width:   s.Width,
		height:  s.Height,
		originX: s.X,
		originY: s.Y,
		dev:     newRecorder(in.Limits()),
		cmaps:   map[*interp.Dict]*cmapData{},
	}
	m.fonts = newFontCache(m, cfg.Fonts)
	m.gs = m.initialState()
	in.Hooks = interp.Hooks{
		Save:       m.saveHook,
		Restore:    m.restoreHook,
		CopyGState: m.copyGState,
	}
	m.opsMatrix()
	m.opsPath()
	m.opsState()
	m.opsPaint()
	m.opsColor()
	m.opsPage()
	m.opsImage()
	m.opsFont()
	m.opsPattern()
	m.setupPageDevice()
	return m
}

// maxPageSide bounds the side of a page in points: 200 inches.
const maxPageSide = 14400

// Pages returns the pages that the program gave.
func (m *Machine) Pages() []Page {
	return m.pages
}

// Done reports whether the program gave all the pages that the caller
// wants.
func (m *Machine) Done() bool {
	return m.done
}

// Finish ends the program: a page that holds marks and that showpage did
// not give becomes the last page. err is the error that ended the program,
// or nil.
func (m *Machine) Finish(err error) {
	if m.done {
		return
	}
	if m.marked || len(m.pages) == 0 {
		m.emitPage(err)
	}
}

// defaultMatrix returns the default matrix of the page: the user space of
// points, with y up from the bottom left corner, onto the device space.
func (m *Machine) defaultMatrix() Matrix {
	s := resolutionScale
	return Matrix{s, 0, 0, -s, -m.originX * s, (m.height + m.originY) * s}
}

// gstate is the graphics state (PLRM 4.2).
type gstate struct {
	ctm   Matrix
	path  Path
	clip  *clipNode
	color paint
	// stroke holds the parameters of the lines.
	lineWidth, miterLimit float64
	lineCap, lineJoin     int
	dash                  []float64
	dashOffset            float64
	flat                  float64
	smoothness            float64
	strokeAdjust          bool
	overprint             bool
	font                  interp.Object
	// The objects that a program sets and reads back, and that the machine
	// does not use.
	transfer, blackGen, ucr, halftone, screen, colorRendering interp.Object
	// save is the save that this entry of the stack of gsave belongs to,
	// or nil.
	save any
}

// initialState returns the graphics state of initgraphics, with the
// parameters that initgraphics does not change at their defaults.
func (m *Machine) initialState() *gstate {
	gs := &gstate{
		smoothness: 0.02,
		font:       interp.Null,
	}
	m.initGraphics(gs)
	return gs
}

// initGraphics sets the parameters of initgraphics (PLRM 8.2).
func (m *Machine) initGraphics(gs *gstate) {
	gs.ctm = m.defaultMatrix()
	gs.path = Path{}
	gs.clip = nil
	gs.color = paint{space: deviceGray, comps: []float64{0}}
	gs.lineWidth = 1
	gs.miterLimit = 10
	gs.lineCap = 0
	gs.lineJoin = 0
	gs.dash = nil
	gs.dashOffset = 0
	gs.flat = 1
	gs.strokeAdjust = false
}

// clone returns a copy of the state that shares no memory that a change of
// one of them changes.
func (gs *gstate) clone() *gstate {
	c := *gs
	c.path = gs.path.clone()
	c.save = nil
	return &c
}

// gsave pushes a copy of the graphics state.
func (m *Machine) gsave() {
	c := m.gs.clone()
	m.stack = append(m.stack, m.gs)
	m.gs = c
}

// grestore pops the graphics state. An entry of a save stays on the stack,
// and the state becomes a copy of it (PLRM 8.2).
func (m *Machine) grestore() {
	if len(m.stack) == 0 {
		return
	}
	top := m.stack[len(m.stack)-1]
	if top.save != nil {
		m.gs = top.clone()
		return
	}
	m.stack = m.stack[:len(m.stack)-1]
	m.gs = top
}

// grestoreAll pops the graphics state down to the entry of the innermost
// save, or to the bottom.
func (m *Machine) grestoreAll() {
	for len(m.stack) > 0 {
		top := m.stack[len(m.stack)-1]
		if top.save != nil {
			m.gs = top.clone()
			return
		}
		m.stack = m.stack[:len(m.stack)-1]
		m.gs = top
	}
}

// saveToken identifies the entry of the stack of gsave of one save.
type saveToken struct{ depth int }

// saveHook is the hook of save: it pushes the graphics state.
func (m *Machine) saveHook() any {
	t := &saveToken{depth: len(m.stack)}
	m.gsave()
	m.stack[len(m.stack)-1].save = t
	return t
}

// restoreHook is the hook of restore: it pops the graphics state to the
// state at the save.
func (m *Machine) restoreHook(state any) {
	t, ok := state.(*saveToken)
	if !ok {
		return
	}
	for len(m.stack) > t.depth {
		top := m.stack[len(m.stack)-1]
		m.stack = m.stack[:len(m.stack)-1]
		m.gs = top
		if top.save == t {
			m.gs.save = nil
			break
		}
	}
}

// copyGState copies a graphics state object into another, for copy and
// currentgstate.
func (m *Machine) copyGState(dst, src any) {
	d, ok1 := dst.(*gstate)
	s, ok2 := src.(*gstate)
	if ok1 && ok2 {
		*d = *s.clone()
	}
}

// clipNode is one clip of the graphics state, in the device space. A clip
// is the intersection of a node and its parents.
type clipNode struct {
	parent *clipNode
	path   *okage.Path
	rule   okage.FillRule
	depth  int
	// bbox is the bounds of the clip in the device space.
	bbox [4]float64
}

// syncClip makes the device hold the clip c: it restores the clips that c
// lacks, and pushes the ones that the device lacks.
func (m *Machine) syncClip(c *clipNode) {
	a := m.applied
	if a == c {
		return
	}
	// common is the deepest node of both chains
	x, y := a, c
	for x != nil && (y == nil || x.depth > y.depth) {
		x = x.parent
	}
	for y != nil && (x == nil || y.depth > x.depth) {
		y = y.parent
	}
	for x != y {
		x, y = x.parent, y.parent
	}
	for n := a; n != x; n = n.parent {
		m.dev.Restore()
	}
	var push []*clipNode
	for n := c; n != x; n = n.parent {
		push = append(push, n)
	}
	for i := len(push) - 1; i >= 0; i-- {
		m.dev.Save()
		m.dev.ClipPath(push[i].path, push[i].rule)
	}
	m.applied = c
}

// clearClip restores the clips that the device holds.
func (m *Machine) clearClip() {
	m.syncClip(nil)
}

// opsState are the operators of the graphics state (PLRM 8.1).
func (m *Machine) opsState() {
	in := m.in
	in.Define("gsave", func(*interp.Interp) error {
		if len(m.stack) >= m.lim.MaxDepth*16 {
			return interp.ErrLimitcheck
		}
		m.gsave()
		return nil
	})
	in.Define("grestore", func(*interp.Interp) error {
		m.grestore()
		return nil
	})
	in.Define("grestoreall", func(*interp.Interp) error {
		m.grestoreAll()
		return nil
	})
	in.Define("initgraphics", func(*interp.Interp) error {
		m.initGraphics(m.gs)
		return nil
	})
	in.Define("gstate", func(*interp.Interp) error {
		in.Push(interp.GState(m.gs.clone()))
		return nil
	})
	in.Define("currentgstate", func(*interp.Interp) error {
		o, err := in.PopType(interp.TGState)
		if err != nil {
			return err
		}
		m.copyGState(o.Ref(), m.gs)
		in.Push(o)
		return nil
	})
	in.Define("setgstate", func(*interp.Interp) error {
		o, err := in.PopType(interp.TGState)
		if err != nil {
			return err
		}
		if s, ok := o.Ref().(*gstate); ok {
			save := m.gs.save
			m.gs = s.clone()
			m.gs.save = save
		}
		return nil
	})
	num := func(name string, set func(float64) error, get func() float64) {
		in.Define("set"+name, func(*interp.Interp) error {
			v, err := in.PopNum()
			if err != nil {
				return err
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return interp.ErrRangecheck
			}
			return set(v)
		})
		in.Define("current"+name, func(*interp.Interp) error {
			in.PushNum(get())
			return nil
		})
	}
	num("linewidth", func(v float64) error {
		m.gs.lineWidth = math.Abs(v)
		return nil
	}, func() float64 { return m.gs.lineWidth })
	num("miterlimit", func(v float64) error {
		if v < 1 {
			return interp.ErrRangecheck
		}
		m.gs.miterLimit = v
		return nil
	}, func() float64 { return m.gs.miterLimit })
	num("flat", func(v float64) error {
		m.gs.flat = min(max(v, 0.2), 100)
		return nil
	}, func() float64 { return m.gs.flat })
	num("smoothness", func(v float64) error {
		m.gs.smoothness = min(max(v, 0), 1)
		return nil
	}, func() float64 { return m.gs.smoothness })
	integer := func(name string, hi int, field func() *int) {
		in.Define("set"+name, func(*interp.Interp) error {
			v, err := in.PopInt()
			if err != nil {
				return err
			}
			if v < 0 || v > hi {
				return interp.ErrRangecheck
			}
			*field() = v
			return nil
		})
		in.Define("current"+name, func(*interp.Interp) error {
			in.Push(interp.Int(*field()))
			return nil
		})
	}
	integer("linecap", 2, func() *int { return &m.gs.lineCap })
	integer("linejoin", 2, func() *int { return &m.gs.lineJoin })
	boolean := func(name string, field func() *bool) {
		in.Define("set"+name, func(*interp.Interp) error {
			v, err := in.PopBool()
			if err != nil {
				return err
			}
			*field() = v
			return nil
		})
		in.Define("current"+name, func(*interp.Interp) error {
			in.Push(interp.Bool(*field()))
			return nil
		})
	}
	boolean("strokeadjust", func() *bool { return &m.gs.strokeAdjust })
	boolean("overprint", func() *bool { return &m.gs.overprint })
	in.Define("setdash", func(*interp.Interp) error {
		off, err := in.PopNum()
		if err != nil {
			return err
		}
		arr, err := in.PopProc()
		if err != nil {
			return err
		}
		var dash []float64
		total := 0.0
		for _, e := range arr.Array() {
			if !e.IsNumber() {
				return interp.ErrTypecheck
			}
			v := e.Float()
			if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				return interp.ErrRangecheck
			}
			total += v
			dash = append(dash, v)
		}
		if len(dash) > 0 && total == 0 {
			return interp.ErrRangecheck
		}
		if len(dash) > 64 {
			dash = dash[:64]
		}
		m.gs.dash, m.gs.dashOffset = dash, off
		return nil
	})
	in.Define("currentdash", func(*interp.Interp) error {
		elems := make([]interp.Object, len(m.gs.dash))
		for i, v := range m.gs.dash {
			elems[i] = interp.Number(v)
		}
		o, err := in.NewArray(elems)
		if err != nil {
			return err
		}
		in.Push(o)
		in.Push(interp.Number(m.gs.dashOffset))
		return nil
	})
	// the parameters of the device that the machine keeps and does not use
	keep := func(set, current string, n int, field func() *interp.Object) {
		in.Define(set, func(*interp.Interp) error {
			if in.Count() < n {
				return interp.ErrStackUnderflow
			}
			var o interp.Object
			for range n {
				o, _ = in.Pop()
			}
			*field() = o
			return nil
		})
		in.Define(current, func(*interp.Interp) error {
			o := *field()
			if o.Type() == interp.TNull && n > 1 {
				// a program reads back the operands of setscreen
				in.PushNum(60)
				in.PushNum(45)
			}
			if o.Type() == interp.TNull {
				p, err := in.NewArray(nil)
				if err != nil {
					return err
				}
				o = p.Exec()
			}
			in.Push(o)
			return nil
		})
	}
	keep("settransfer", "currenttransfer", 1, func() *interp.Object { return &m.gs.transfer })
	keep("setblackgeneration", "currentblackgeneration", 1, func() *interp.Object { return &m.gs.blackGen })
	keep("setundercolorremoval", "currentundercolorremoval", 1, func() *interp.Object { return &m.gs.ucr })
	keep("setscreen", "currentscreen", 3, func() *interp.Object { return &m.gs.screen })
	in.Define("setcolortransfer", func(*interp.Interp) error {
		for range 4 {
			if _, err := in.Pop(); err != nil {
				return err
			}
		}
		return nil
	})
	in.Define("currentcolortransfer", func(*interp.Interp) error {
		for range 4 {
			p, err := in.NewArray(nil)
			if err != nil {
				return err
			}
			in.Push(p.Exec())
		}
		return nil
	})
	in.Define("setcolorscreen", func(*interp.Interp) error {
		for range 12 {
			if _, err := in.Pop(); err != nil {
				return err
			}
		}
		return nil
	})
	in.Define("currentcolorscreen", func(*interp.Interp) error {
		for range 4 {
			in.PushNum(60)
			in.PushNum(45)
			p, err := in.NewArray(nil)
			if err != nil {
				return err
			}
			in.Push(p.Exec())
		}
		return nil
	})
	in.Define("sethalftone", func(*interp.Interp) error {
		o, err := in.PopType(interp.TDict)
		m.gs.halftone = o
		return err
	})
	in.Define("currenthalftone", func(*interp.Interp) error {
		if m.gs.halftone.Type() == interp.TDict {
			in.Push(m.gs.halftone)
			return nil
		}
		d := in.NewDict(4)
		d.Dict().PutName("HalftoneType", interp.Int(1))
		d.Dict().PutName("Frequency", interp.Int(60))
		d.Dict().PutName("Angle", interp.Int(45))
		in.Push(d)
		return nil
	})
	in.Define("setcolorrendering", func(*interp.Interp) error {
		o, err := in.PopType(interp.TDict)
		m.gs.colorRendering = o
		return err
	})
	in.Define("currentcolorrendering", func(*interp.Interp) error {
		if m.gs.colorRendering.Type() == interp.TDict {
			in.Push(m.gs.colorRendering)
			return nil
		}
		in.Push(in.NewDict(1))
		return nil
	})
	in.Define("sethalftonephase", func(*interp.Interp) error {
		_, err := in.PopNums(2)
		return err
	})
	in.Define("currenthalftonephase", func(*interp.Interp) error {
		in.Push(interp.Int(0))
		in.Push(interp.Int(0))
		return nil
	})
	in.Define("setucacheparams", func(*interp.Interp) error {
		return popToMark(in)
	})
	in.Define("ucachestatus", func(*interp.Interp) error {
		in.Push(interp.Mark)
		for range 5 {
			in.Push(interp.Int(0))
		}
		return nil
	})
	in.Define("wtranslation", func(*interp.Interp) error {
		in.Push(interp.Int(0))
		in.Push(interp.Int(0))
		return nil
	})
}

// popToMark pops the operand stack down to a mark, and the mark.
func popToMark(in *interp.Interp) error {
	for {
		o, err := in.Pop()
		if err != nil {
			return interp.ErrUnmatchedMark
		}
		if o.Type() == interp.TMark {
			return nil
		}
	}
}
