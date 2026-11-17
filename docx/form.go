package docx

import (
	"strconv"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ooxml"
)

// formField is a legacy form field of Word, from w:ffData (ECMA-376 Part 1,
// 17.16.17): a check box or a drop-down list. A text box needs nothing, as
// the result of its field holds its text.
type formField struct {
	// check is set for a check box, and checked when the box is checked.
	check, checked bool
	// entries are the entries of a drop-down list, and entry is the index
	// of the entry that the list shows.
	entries []string
	entry   int
}

// parseFormField reads a w:ffData element. It returns nil for a form field
// that is not a check box or a drop-down list.
func parseFormField(n roxmltree.Node) *formField {
	on := func(e roxmltree.Node) bool {
		v := ooxml.Attr(e, "val")
		return v == "" || v == "1" || v == "true" || v == "on"
	}
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "checkBox":
			f := &formField{check: true}
			// w:checked is the state of the box, and w:default the state
			// that the box takes when w:checked is missing.
			def, state := false, -1
			for c := range e.Children() {
				if !c.IsElement() {
					continue
				}
				switch c.TagName().Name() {
				case "default":
					def = on(c)
				case "checked":
					state = 0
					if on(c) {
						state = 1
					}
				}
			}
			f.checked = def
			if state >= 0 {
				f.checked = state == 1
			}
			return f
		case "ddList":
			f := &formField{}
			for c := range e.Children() {
				if !c.IsElement() {
					continue
				}
				switch c.TagName().Name() {
				case "listEntry":
					if len(f.entries) < 64 {
						f.entries = append(f.entries, ooxml.Attr(c, "val"))
					}
				case "result", "default":
					if v, err := strconv.Atoi(ooxml.Attr(c, "val")); err == nil && (c.TagName().Name() == "result" || f.entry == 0) {
						f.entry = v
					}
				}
			}
			return f
		}
	}
	return nil
}

// formField writes a check box for the field FORMCHECKBOX, or the entry of
// a drop-down list for the field FORMDROPDOWN, in a run with the properties
// p, in a paragraph whose runs have the properties base. It returns false
// for another field, whose result stays.
func (c *converter) formField(code string, f *formField, p, base props) bool {
	switch {
	case f == nil:
		return false
	case code == "FORMCHECKBOX" && f.check:
		// Word draws a square of the size of the text, with a cross in it
		// when the box is checked.
		color := c.color(p)
		c.w.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" style="width: 0.9em; height: 0.9em; vertical-align: -0.1em">`)
		c.w.WriteString(`<rect x="0.3" y="0.3" width="9.4" height="9.4" fill="none" stroke="` + color + `" stroke-width="0.6"/>`)
		if f.checked {
			c.w.WriteString(`<path d="M0.3 0.3L9.7 9.7M9.7 0.3L0.3 9.7" stroke="` + color + `" stroke-width="0.6"/>`)
		}
		c.w.WriteString("</svg>")
		return true
	case code == "FORMDROPDOWN" && !f.check && len(f.entries) > 0:
		c.text(f.entries[min(max(f.entry, 0), len(f.entries)-1)], p, base)
		return true
	}
	return false
}
