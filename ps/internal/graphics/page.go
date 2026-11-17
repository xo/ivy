package graphics

import (
	"fmt"
	"math"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/ps/internal/interp"
)

// maxPages bounds the pages of a document.
const maxPages = 10000

// limitError returns the error of a limit of the machine.
func limitError(what string, n int64) error {
	return fmt.Errorf("postscript %s past %d: %w", what, n, ivy.ErrLimit)
}

// emitPage ends the current page: it keeps the commands as a page, and
// starts an empty one. err is the error that ended the page early, or nil.
func (m *Machine) emitPage(err error) {
	m.clearClip()
	if err == nil {
		err = m.dev.Err()
	}
	if len(m.pages) < maxPages {
		m.pages = append(m.pages, Page{Width: m.width, Height: m.height, List: m.dev, Err: err})
	}
	m.dev = newRecorder(m.lim)
	m.applied = nil
	m.marked = false
	m.in.NewPage()
	if m.cfg.Setup.FirstPageOnly || len(m.pages) >= maxPages {
		m.done = true
	}
}

// erasePage drops the marks of the current page.
func (m *Machine) erasePage() {
	m.dev = newRecorder(m.lim)
	m.applied = nil
	m.marked = false
}

// setupPageDevice makes the dictionary of currentpagedevice.
func (m *Machine) setupPageDevice() {
	d := m.in.NewDict(16)
	m.pageDevice = d
	m.updatePageDevice()
	pd := d.Dict()
	res, _ := m.in.NewArray([]interp.Object{interp.Int(300), interp.Int(300)})
	pd.PutName("HWResolution", res)
	pd.PutName("ImagingBBox", interp.Null)
	pd.PutName("Policies", m.in.NewDict(4))
	pd.PutName("NumCopies", interp.Null)
	pd.PutName("Orientation", interp.Int(0))
	pd.PutName("Duplex", interp.Bool(false))
	pd.PutName("Install", m.emptyProc())
	pd.PutName("BeginPage", m.emptyProc())
	pd.PutName("ProcessColorModel", interp.NameObj("DeviceRGB"))
	pd.PutName("PageCount", interp.Int(0))
}

// emptyProc returns an empty procedure.
func (m *Machine) emptyProc() interp.Object {
	p, err := m.in.NewArray(nil)
	if err != nil {
		return interp.Null
	}
	return p.Exec()
}

// updatePageDevice puts the size of the page into the dictionary of
// currentpagedevice.
func (m *Machine) updatePageDevice() {
	size, err := m.in.NewArray([]interp.Object{interp.Number(m.width), interp.Number(m.height)})
	if err != nil {
		return
	}
	m.pageDevice.Dict().PutName("PageSize", size)
}

// showPage ends the page, and starts the next one as showpage does.
func (m *Machine) showPage() error {
	m.emitPage(nil)
	if pd := m.pageDevice.Dict(); pd != nil {
		if v, ok := pd.GetName("PageCount"); ok && v.Type() == interp.TInt {
			pd.PutName("PageCount", interp.Int(v.IntVal()+1))
		}
	}
	if m.done {
		return interp.Quit()
	}
	m.initGraphics(m.gs)
	return nil
}

// opsPage are the operators of the device and of the pages (PLRM 8.1).
func (m *Machine) opsPage() {
	in := m.in
	in.Define("showpage", func(*interp.Interp) error {
		return m.showPage()
	})
	in.Define("copypage", func(*interp.Interp) error {
		cp := &Recorder{cmds: append([]func(display.Device, float64){}, m.dev.cmds...), max: m.dev.max, depth: m.dev.depth}
		m.clearClip()
		if len(m.pages) < maxPages {
			m.pages = append(m.pages, Page{Width: m.width, Height: m.height, List: cp})
		}
		if m.cfg.Setup.FirstPageOnly {
			m.done = true
			return interp.Quit()
		}
		return nil
	})
	in.Define("erasepage", func(*interp.Interp) error {
		m.erasePage()
		return nil
	})
	in.Define("setpagedevice", func(*interp.Interp) error {
		d, err := in.PopDict()
		if err != nil {
			return err
		}
		if v, ok := d.GetName("PageSize"); ok && v.IsArray() && v.Len() >= 2 {
			a := v.Array()
			w, h := a[0].Float(), a[1].Float()
			if w >= 1 && h >= 1 && w <= maxPageSide && h <= maxPageSide && !math.IsNaN(w+h) {
				m.width, m.height = w, h
				m.originX, m.originY = 0, 0
				m.updatePageDevice()
			}
		}
		pd := m.pageDevice.Dict()
		d.Each(func(k, v interp.Object) bool {
			if k.NameVal() != "PageSize" {
				pd.Put(k, v)
			}
			return true
		})
		if !m.marked {
			m.erasePage()
		}
		m.initGraphics(m.gs)
		return nil
	})
	in.Define("currentpagedevice", func(*interp.Interp) error {
		in.Push(m.pageDevice)
		return nil
	})
	in.Define("nulldevice", func(*interp.Interp) error {
		return nil
	})
	in.Define("framedevice", func(*interp.Interp) error {
		for range 4 {
			if _, err := in.Pop(); err != nil {
				return err
			}
		}
		return nil
	})
	in.Define("renderbands", func(*interp.Interp) error {
		_, err := in.Pop()
		return err
	})
	in.Define("setdevparams", func(*interp.Interp) error {
		return popToMark(in)
	})
	// the sizes of paper of Level 1 in userdict and statusdict
	for name, size := range map[string][2]float64{
		"letter": {612, 792}, "legal": {612, 1008}, "a4": {595, 842},
		"a3": {842, 1191}, "a5": {420, 595}, "b5": {516, 729}, "11x17": {792, 1224},
		"ledger": {1224, 792}, "note": {612, 792}, "lettersmall": {612, 792}, "a4small": {595, 842},
	} {
		op := interp.OpObj(&interp.Operator{Name: name, Fn: func(*interp.Interp) error {
			m.width, m.height = size[0], size[1]
			m.originX, m.originY = 0, 0
			m.updatePageDevice()
			m.erasePage()
			m.initGraphics(m.gs)
			return nil
		}})
		in.UserDict.PutName(name, op)
		in.StatusDict.PutName(name, op)
	}
}
