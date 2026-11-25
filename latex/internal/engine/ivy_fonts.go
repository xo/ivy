// This file of ivy combines the font switches of LaTeX for the family
// sans (D72). The engine binds one face to each switch, such as \sf or
// \it, and a switch selects its face whatever the current font is. NFSS
// changes one axis of three: the family, the series or the shape. So
// \textsf{\textit{x}} is the oblique sans in LaTeX, and it was the italic
// roman in the engine.

package engine

// fontAxes are the family, the series and the shape of a face, as NFSS
// names them: the family rm, sf or tt, the series m or bx, and the shape
// n or it. The slanted shape is the italic shape, as the engine binds \sl
// to the italic face.
type fontAxes struct {
	family string
	bold   bool
	italic bool
}

// bindFaces keeps the faces of the run by their axes. The faces of the
// family sans are new faces of the files of faces. The others are the
// faces that the engine bound to \rm, \bf, \it and \tt.
func (e *Engine) bindFaces(faces Faces) error {
	e.ivy.faces = map[fontAxes]fontFace{}
	for name, ax := range map[string]fontAxes{
		"rm": {family: "rm"},
		"bf": {family: "rm", bold: true},
		"it": {family: "rm", italic: true},
		"tt": {family: "tt"},
		"sf": {family: "sf"},
	} {
		if m := e.eq[name]; m != nil && m.kind == mFont && m.font != nil {
			e.ivy.faces[ax] = m.font
		}
	}
	sf, ok := e.ivy.faces[fontAxes{family: "sf"}]
	if !ok || faces.Sans == nil {
		// without a face of sans, the switches select their faces as the
		// engine binds them
		e.ivy.faces = nil
		return nil
	}
	for ax, data := range map[fontAxes][]byte{
		{family: "sf", bold: true}:               faces.SansBold,
		{family: "sf", italic: true}:             faces.SansItalic,
		{family: "sf", bold: true, italic: true}: faces.SansBoldItalic,
	} {
		if data == nil {
			e.ivy.faces[ax] = sf
			continue
		}
		f, err := NewOpenTypeFont(data, sf.sizePt())
		if err != nil {
			return err
		}
		e.ivy.faces[ax] = f
	}
	return nil
}

// axesOf returns the axes of the face f: the axes of the face of the run
// that has the shape of f, at any size. A face that the run does not know
// is roman.
func (e *Engine) axesOf(f fontFace) fontAxes {
	if f != nil {
		for ax, g := range e.ivy.faces {
			if sameShape(g, f) {
				return ax
			}
		}
	}
	return fontAxes{family: "rm"}
}

// switchAxes returns the axes after the font switch name from the axes
// cur. A switch of LaTeX 2.09, such as \bf, is \normalfont and one axis,
// as \DeclareOldFontCommand of the classes makes it. It reports false for
// a name that is not a font switch of LaTeX.
func switchAxes(name string, cur fontAxes) (fontAxes, bool) {
	switch name {
	case "rm", "normalfont":
		return fontAxes{family: "rm"}, true
	case "bf":
		return fontAxes{family: "rm", bold: true}, true
	case "it", "sl":
		return fontAxes{family: "rm", italic: true}, true
	case "tt":
		return fontAxes{family: "tt"}, true
	case "sf":
		return fontAxes{family: "sf"}, true
	case "rmfamily":
		cur.family = "rm"
	case "sffamily":
		cur.family = "sf"
	case "ttfamily":
		cur.family = "tt"
	case "bfseries":
		cur.bold = true
	case "mdseries":
		cur.bold = false
	case "itshape", "slshape":
		cur.italic = true
	case "upshape":
		cur.italic = false
	default:
		return cur, false
	}
	return cur, true
}

// switchFace returns the face that the font switch m selects. A switch
// that goes into or out of the family sans takes the face of the family,
// the series and the shape that NFSS selects. Any other switch takes the
// face that the engine bound to it, as before: the roman family has no
// bold italic face, and the typewriter family has one face.
func (e *Engine) switchFace(m *meaning) fontFace {
	if e.ivy.faces == nil {
		return m.font
	}
	cur := e.axesOf(e.curFont)
	next, ok := switchAxes(m.name, cur)
	if !ok || (cur.family != "sf" && next.family != "sf") {
		return m.font
	}
	if next.family == "rm" {
		// the roman family has no bold italic face: the shape wins, as the
		// last switch did in the engine
		next.bold = next.bold && !next.italic
	}
	if next.family == "tt" {
		next = fontAxes{family: "tt"}
	}
	if f, ok := e.ivy.faces[next]; ok {
		return f
	}
	return m.font
}
