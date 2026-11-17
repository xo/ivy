package msppt

// The records of the headers and the footers (MS-PPT 2.4.15 and 2.4.16).
const (
	rtHeadersFooters     = 0x0FD9
	rtHeadersFootersAtom = 0x0FDA
	rtCString            = 0x0FBA
)

// hfSlides is the instance of the HeadersFootersContainer of the slides.
const hfSlides = 3

// The flags of a HeadersFootersAtom.
const (
	hfHasDate        = 0x0001
	hfHasUserDate    = 0x0004
	hfHasSlideNumber = 0x0008
	hfHasFooter      = 0x0020
)

// The types of the placeholders of a master that the headers and the
// footers show (MS-PPT 2.13.21).
const (
	phMasterDate        = 0x07
	phMasterSlideNumber = 0x08
	phMasterFooter      = 0x09
)

// headersFooters is what a HeadersFootersContainer says of the date, the
// slide number and the footer of a slide: whether each shows, and the text
// of the date and of the footer that a person typed.
type headersFooters struct {
	flags    uint16
	userDate []uint16
	footer   []uint16
	ok       bool
}

// readHeadersFooters reads the HeadersFootersContainer of the slides in the
// body b of a DocumentContainer or of a SlideContainer.
func readHeadersFooters(b []byte) headersFooters {
	var hf headersFooters
	for h, body := range children(b) {
		if h.Type != rtHeadersFooters || h.Inst != hfSlides {
			continue
		}
		for ch, a := range children(body) {
			switch {
			case ch.Type == rtHeadersFootersAtom && len(a) >= 4:
				hf.flags, hf.ok = le.Uint16(a[2:]), true
			case ch.Type == rtCString && ch.Inst == 0:
				hf.userDate = decodeText(rtTextCharsAtom, a[:min(len(a), 512)])
			case ch.Type == rtCString && ch.Inst == 2:
				hf.footer = decodeText(rtTextCharsAtom, a[:min(len(a), 4096)])
			}
		}
		if hf.ok {
			return hf
		}
	}
	return hf
}

// shows reports whether a placeholder of the type ph of the master shows on
// the slide, and returns the text that replaces the text of the
// placeholder, or nil. A date of today does not show, since the date of the
// rendering is not the date of a printing in PowerPoint.
func (hf headersFooters) shows(ph int) (bool, []uint16) {
	switch ph {
	case phMasterDate:
		if hf.flags&hfHasDate == 0 || hf.flags&hfHasUserDate == 0 || len(hf.userDate) == 0 {
			return false, nil
		}
		return true, hf.userDate
	case phMasterSlideNumber:
		return hf.flags&hfHasSlideNumber != 0, nil
	case phMasterFooter:
		if hf.flags&hfHasFooter == 0 {
			return false, nil
		}
		return true, hf.footer
	}
	return false, nil
}

// withUnits returns a copy of the text t with the characters units. The
// characters after the end of the runs of the style of t take its last
// runs.
func (t *text) withUnits(units []uint16) *text {
	c := *t
	c.styleLen = max(len(t.units), 1)
	c.units = units
	c.fields = nil
	c.links = nil
	return &c
}
