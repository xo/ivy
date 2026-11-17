package fontdb

import (
	"encoding/binary"
	"strings"
	"unicode"

	"github.com/xo/okage/svg"
	gofont "github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype/tables"

	"github.com/xo/ivy/internal/font"
)

// Find returns the face of db for the font that a document names and does
// not embed, such as the BaseFont of a PDF font. name can be a PostScript
// name, such as ArialMT or TimesNewRomanPS-BoldMT, or a family with a
// style, such as Arial,Bold. weight and italic select the face of the
// family.
//
// Find looks for, in this order:
//
//  1. The face with the PostScript name, in any case.
//  2. A family of the name, without its style and its suffix of Monotype
//     (MT, PSMT). Find compares the letters and the digits of the names
//     only, in any case. TimesNewRomanPSMT finds Times New Roman.
//  3. The family of the substitute of such a family, such as Liberation
//     Sans for Arial (font.Substitute).
//  4. For an East Asian font, such as MS Mincho or ＭＳ 明朝, an East Asian
//     face of db of the same kind and language (ClassifyName and
//     EastAsian.Families) that draws the Probe of the language.
//
// The names of a family include the names in other languages that the name
// table of a font holds, such as ＭＳ 明朝 for MS Mincho, and Find compares
// the full-width forms of the Latin letters as their ASCII forms.
//
// It returns false if db holds none of them.
func Find(db *svg.FontDatabase, name string, weight uint16, italic bool) (svg.FontID, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false
	}
	families := map[string]string{}
	for face := range db.Faces() {
		if strings.EqualFold(face.PostScriptName, name) {
			return face.ID, true
		}
		for _, f := range face.Families {
			k := familyKey(f.Name)
			if _, ok := families[k]; !ok && k != "" {
				families[k] = f.Name
			}
		}
	}
	candidates := familyCandidates(name)
	query := func(family string) (svg.FontID, bool) {
		q := svg.FontQuery{
			Families: []svg.FontFamily{{Kind: svg.FontFamilyNamed, Name: family}},
			Weight:   weight,
			Stretch:  svg.DefaultFontStretch(),
		}
		if italic {
			q.Style = svg.FontStyleItalic
		}
		return db.Query(q)
	}
	for _, c := range candidates {
		if family, ok := families[familyKey(c)]; ok {
			if id, ok := query(family); ok {
				return id, true
			}
		}
	}
	for _, c := range candidates {
		sub, ok := font.Substitute(c)
		if !ok {
			continue
		}
		if family, ok := families[familyKey(sub)]; ok {
			if id, ok := query(family); ok {
				return id, true
			}
		}
	}
	kind, lang, ok := ClassifyName(name)
	if !ok {
		return 0, false
	}
	probe := Probe(lang)
	for _, family := range NewEastAsian(db, 0, db.Len()).Families(kind, lang) {
		if id, ok := query(family); ok && draws(db, id, probe) {
			return id, true
		}
	}
	return 0, false
}

// familyKey returns the key of a family name: its letters and digits in
// lower case, with the full-width forms as their ASCII forms.
func familyKey(name string) string {
	var b strings.Builder
	for _, r := range foldWidth(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// familyCandidates returns the families that a font name can name, the
// most exact first: the name, the name without its style after a comma or
// a hyphen, without the suffix MT, PSMT or PS, and with a space before each
// capital letter that follows a small letter.
func familyCandidates(name string) []string {
	out := []string{name}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !strings.EqualFold(s, out[len(out)-1]) {
			out = append(out, s)
		}
	}
	base := name
	if i := strings.IndexByte(base, ','); i > 0 {
		base = base[:i]
		add(base)
	}
	if i := strings.IndexByte(base, '-'); i > 0 {
		base = base[:i]
		add(base)
	}
	for _, suffix := range []string{"PSMT", "MT", "PS"} {
		if s, ok := strings.CutSuffix(base, suffix); ok && s != "" {
			base = s
			add(base)
			break
		}
	}
	add(splitWords(base))
	return out
}

// splitWords puts a space before each capital letter that follows a small
// letter, so that TimesNewRoman becomes Times New Roman.
func splitWords(s string) string {
	var b strings.Builder
	prev := rune(0)
	for _, r := range s {
		if unicode.IsUpper(r) && unicode.IsLower(prev) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

// Covering returns the first face of db that draws r. It looks at the
// faces of the font files first, the first faces of db that Load counts,
// then at the other faces with the fallback selector of okage, which the
// layout uses too. It returns false if no face draws r.
func Covering(db *svg.FontDatabase, first int, r rune) (svg.FontID, bool) {
	var base svg.FontID
	hasBase := false
	i := 0
	for face := range db.Faces() {
		if i < first && draws(db, face.ID, r) {
			return face.ID, true
		}
		if i >= first && !hasBase {
			base, hasBase = face.ID, true
		}
		i++
	}
	if !hasBase {
		return 0, false
	}
	// The selector leaves out the face that it gets, and looks for a face
	// of the same style. base is the first Go font, which is regular and
	// draws no CJK character.
	id, ok := svg.DefaultFallbackSelector(nil)(r, []svg.FontID{base}, db)
	return id, ok
}

// draws reports whether the face id of db draws r. It reads the cmap table
// of the face only, which is faster than to read the whole face.
func draws(db *svg.FontDatabase, id svg.FontID, r rune) (ok bool) {
	data, index, found := db.FaceData(id)
	if !found {
		return false
	}
	// go-text can panic on a malformed font (D5, D18).
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	raw, found := cmapTable(data, index)
	if !found {
		return false
	}
	table, _, err := tables.ParseCmap(raw)
	if err != nil {
		return false
	}
	cmap, _, err := gofont.ProcessCmap(table, 0)
	if err != nil {
		return false
	}
	g, ok := cmap.Lookup(r)
	return ok && g != 0
}

// cmapTable returns the cmap table of the font at index in data, which can
// be a collection, and false if it has none inside data. It reads the table
// directory itself, because the loader of go-text reserves the length that
// the directory gives before it reads a table.
func cmapTable(data []byte, index uint32) ([]byte, bool) {
	off := uint64(0)
	if len(data) >= 12 && string(data[:4]) == "ttcf" {
		pos := 12 + 4*uint64(index)
		if index >= binary.BigEndian.Uint32(data[8:]) || pos+4 > uint64(len(data)) {
			return nil, false
		}
		off = uint64(binary.BigEndian.Uint32(data[pos:]))
	} else if index != 0 {
		return nil, false
	}
	if off+12 > uint64(len(data)) {
		return nil, false
	}
	n := uint64(binary.BigEndian.Uint16(data[off+4:]))
	for i := range n {
		rec := off + 12 + 16*i
		if rec+16 > uint64(len(data)) {
			return nil, false
		}
		if string(data[rec:rec+4]) != "cmap" {
			continue
		}
		start := uint64(binary.BigEndian.Uint32(data[rec+8:]))
		end := start + uint64(binary.BigEndian.Uint32(data[rec+12:]))
		if end > uint64(len(data)) {
			return nil, false
		}
		return data[start:end:end], true
	}
	return nil, false
}
