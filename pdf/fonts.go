package pdf

import (
	"io/fs"
	"log/slog"
	"sync"

	"github.com/xo/okage/svg"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fontdb"
	"github.com/xo/ivy/internal/hayro/cmap"
	"github.com/xo/ivy/internal/hayro/interpret"
	"github.com/xo/ivy/internal/hayro/pdffont"
)

// cjkProbes holds a character of each CJK collection of Adobe. A face that
// draws it is the fallback of the fonts of the collection: kana for Japan1,
// a simplified ideograph for GB1, a traditional one for CNS1, and hangul
// for Korea1.
var cjkProbes = map[cmap.CIDFamilyKind]rune{
	cmap.AdobeJapan1: 'あ',
	cmap.AdobeGB1:    '这',
	cmap.AdobeCNS1:   '這',
	cmap.AdobeKorea1: '한',
}

// fontResolver resolves the fonts that a PDF file names and does not embed,
// with the fonts of ivy.Config (D23, D24). It looks for, in this order:
//
//  1. The font that the file names, or its substitute of
//     github.com/xo/ivy/fonts, such as Liberation Sans for Arial, in
//     Config.Fonts and in the fonts of the system when Config.SystemFonts
//     is set.
//  2. For a CID font of a CJK collection, the first face that draws the
//     characters of the collection, such as Droid Sans Fallback of
//     github.com/xo/ivy/fonts.
//  3. The substitute of the standard font that is closest to the font, as
//     interpret.DefaultFontResolver gives.
//
// For a character that the substitute of a CID font does not draw, it gives
// the first face that draws the character, such as a font of Noto of
// github.com/xo/ivy/fonts (D74), and none without the fonts of Config.
//
// A file that names one of the 14 standard fonts gets the substitute of the
// standard font. Its methods are safe for concurrent use.
type fontResolver struct {
	fsys   fs.FS
	system bool
	lim    ivy.Limits
	log    *slog.Logger

	once  sync.Once
	db    *svg.FontDatabase
	first int
}

// newFontResolver returns the font resolver of the fonts of cfg. lim must
// have its defaults.
func newFontResolver(cfg ivy.Config, lim ivy.Limits) *fontResolver {
	return &fontResolver{fsys: cfg.Fonts, system: cfg.SystemFonts, lim: lim, log: cfg.Log()}
}

// resolve is a pdffont.FontResolverFn.
func (r *fontResolver) resolve(q *pdffont.FontQuery) (pdffont.FontData, uint32, bool) {
	if c := q.Fallback.Char; c != 0 && !q.IsStandard {
		if r.fsys == nil && !r.system {
			return nil, 0, false
		}
		return r.styled(c, weightOf(&q.Fallback))
	}
	if q.IsStandard || r.fsys == nil && !r.system {
		return interpret.DefaultFontResolver(q)
	}
	if q.Named {
		return r.named(&q.Fallback)
	}
	if cc := q.Fallback.CharacterCollection; cc != nil {
		if probe, ok := cjkProbes[cc.Family.Kind]; ok {
			if data, index, ok := r.covering(probe); ok {
				return data, index, true
			}
		}
	}
	return interpret.DefaultFontResolver(q)
}

// database returns the database of the fonts, which it reads on first use.
func (r *fontResolver) database() (*svg.FontDatabase, int) {
	r.once.Do(func() {
		r.db, r.first = fontdb.Load(r.fsys, r.system, r.lim, r.log)
	})
	return r.db, r.first
}

// named returns the face of the font that q names, and false if the
// database holds none.
func (r *fontResolver) named(q *pdffont.FallbackFontQuery) (pdffont.FontData, uint32, bool) {
	db, _ := r.database()
	weight := weightOf(q)
	for _, name := range []string{q.PostScriptName, q.FontFamily, q.FontName} {
		if name == "" {
			continue
		}
		if id, ok := fontdb.Find(db, name, weight, q.IsItalic); ok {
			return db.FaceData(id)
		}
	}
	return nil, 0, false
}

// weightOf returns the weight of the font of q.
func weightOf(q *pdffont.FallbackFontQuery) uint16 {
	weight := uint16(min(q.FontWeight, 1000))
	if q.IsBold {
		weight = max(weight, 700)
	}
	return weight
}

// styled returns the face that draws c, with the weight of its family that
// is closest to weight, and false if the database holds none (D74).
func (r *fontResolver) styled(c rune, weight uint16) (pdffont.FontData, uint32, bool) {
	db, first := r.database()
	id, ok := fontdb.Covering(db, first, c)
	if !ok {
		return nil, 0, false
	}
	if s, ok := fontdb.Styled(db, id, weight, false); ok && fontdb.Draws(db, s, c) {
		id = s
	}
	return db.FaceData(id)
}

// covering returns the first face that draws probe, and false if the
// database holds none.
func (r *fontResolver) covering(probe rune) (pdffont.FontData, uint32, bool) {
	db, first := r.database()
	id, ok := fontdb.Covering(db, first, probe)
	if !ok {
		return nil, 0, false
	}
	return db.FaceData(id)
}
