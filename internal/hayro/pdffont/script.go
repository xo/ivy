// This file ports no upstream file. It draws a character of a CID font
// that the file does not embed, and that the substitute of a standard font
// does not draw, with a face of the font resolver that draws it (D74).

package pdffont

import "sync"

// The glyphs of the script fonts of a CID font hold the number of their
// program, from 1, in the bits from scriptShift to 30. Bit 31 is latinGlyph.
const (
	scriptShift = 16
	scriptMax   = 1<<(31-scriptShift) - 1
	scriptGlyph = 1<<scriptShift - 1
)

// dataKey identifies the data of a font program that the font resolver
// gives, by the address of its first byte, its length and its index.
type dataKey struct {
	first *byte
	n     int
	index uint32
}

// scriptFonts are the faces that draw the characters of a CID font that the
// file does not embed and that its substitute does not draw. Its methods are
// safe for concurrent use.
//
// Port: upstream draws such a character with the glyph of its CID in the
// substitute of a standard font.
type scriptFonts struct {
	programs *programLoader
	query    FallbackFontQuery

	mu    sync.Mutex
	runes map[rune]GlyphID
	progs []*fontProgram
}

// newScriptFonts returns the script fonts of a font of query.
func newScriptFonts(programs *programLoader, query FallbackFontQuery) *scriptFonts {
	return &scriptFonts{programs: programs, query: query, runes: map[rune]GlyphID{}}
}

// glyph returns the glyph of r in the face that the font resolver gives for
// r, marked with the number of its program, or .notdef.
func (s *scriptFonts) glyph(r rune) GlyphID {
	s.mu.Lock()
	g, ok := s.runes[r]
	s.mu.Unlock()
	if ok {
		return g
	}
	g = GlyphIDNotdef
	q := FontQuery{Fallback: s.query}
	q.Fallback.Char = r
	prog, ok := s.programs.resolveData(&q)
	var id GlyphID
	if ok {
		id, ok = glyphOfChar(prog, r)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ok && id != GlyphIDNotdef && id <= scriptGlyph {
		n := 0
		for i, p := range s.progs {
			if p == prog {
				n = i + 1
				break
			}
		}
		if n == 0 && len(s.progs) < scriptMax {
			s.progs = append(s.progs, prog)
			n = len(s.progs)
		}
		if n != 0 {
			g = id | GlyphID(n)<<scriptShift
		}
	}
	s.runes[r] = g
	return g
}

// programOf returns the program of a glyph that glyph returned, and the ID
// of the glyph in that program. It returns false for a glyph of no script
// font.
func (s *scriptFonts) programOf(glyph GlyphID) (*fontProgram, GlyphID, bool) {
	n := int(glyph>>scriptShift) & scriptMax
	if n == 0 {
		return nil, 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n > len(s.progs) {
		return nil, 0, false
	}
	return s.progs[n-1], glyph & scriptGlyph, true
}

// resolveData returns the font program that the font resolver gives for a
// query. It keeps one program for each face that the resolver gives, so
// that the queries of many characters share the program of a face.
//
// Port: upstream has no such query.
func (l *programLoader) resolveData(query *FontQuery) (*fontProgram, bool) {
	if l.fontResolver == nil {
		return nil, false
	}
	data, index, ok := l.fontResolver(query)
	if !ok || len(data) == 0 {
		return nil, false
	}
	key := dataKey{first: &data[0], n: len(data), index: index}
	c := l.cache
	c.programsMu.Lock()
	p, ok := c.byData[key]
	c.programsMu.Unlock()
	if ok {
		return p, p != nil
	}
	p, err := newFontProgram(data, index)
	if err != nil {
		p = nil
	}
	c.programsMu.Lock()
	if prev, ok := c.byData[key]; ok {
		p = prev
	} else {
		c.byData[key] = p
	}
	c.programsMu.Unlock()
	return p, p != nil
}
