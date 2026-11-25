// This file is not a port. It holds the rule of the layout of the text of
// a slide that D71 adds, which CSS cannot give:
//
//   - -ivy-line-baseline puts the baseline of a line at the share of the
//     height of the line that the Windows ascent of its font takes of the
//     Windows ascent and descent, as PowerPoint does, see baselineShift.

package layout

import (
	"bytes"
	"encoding/binary"

	"github.com/xo/okage/svg"
	ot "github.com/xo/okage/typesetting/font/opentype"
)

// baselineShift returns how far -ivy-line-baseline moves the text of a box
// of style up in its line, whose height is lineHeight. baseline is the
// place of the baseline in the line that CSS gives, from the top of the
// line. With 'win', PowerPoint puts the baseline of a line at lineHeight
// times usWinAscent / (usWinAscent + usWinDescent) of the OS/2 table of the
// font, for each spacing of the lines. CSS puts half of the leading above
// the text and half below, with the ascent and the descent of the table
// hhea. The two differ for a font whose Windows metrics hold its line gap,
// such as Calibri: its baseline is 0.086 em lower in PowerPoint. The shift
// is negative when the text moves down. It is 0 for 'normal', and for a
// font without Windows metrics.
func baselineShift(style *Style, lineHeight, baseline float64) float64 {
	if !isStr(style.Get("line_baseline"), "win") || !(lineHeight > 0) {
		return 0
	}
	ratio, ok := winRatio(style)
	if !ok {
		return 0
	}
	return baseline - lineHeight*ratio
}

// winRatio returns the share of usWinAscent in usWinAscent and
// usWinDescent of the first face of the font of style, and false when the
// face has no table OS/2 or its values are 0.
func winRatio(style *Style) (float64, bool) {
	fc := fontConfigOf(style)
	if fc == nil || fc.db == nil {
		return 0, false
	}
	m, _ := fc.styleFontmap(style)
	f, id := m.primary()
	if f == nil {
		return 0, false
	}
	if r, ok := fc.winRatios[id]; ok {
		return r, r > 0
	}
	r := faceWinRatio(fc.db, id)
	if fc.winRatios == nil {
		fc.winRatios = map[svg.FontID]float64{}
	}
	fc.winRatios[id] = r
	return r, r > 0
}

// faceWinRatio reads usWinAscent and usWinDescent of the table OS/2 of the
// face id of db, and returns the share of the ascent, or 0. The two values
// are at the offsets 74 and 76 of each version of the table.
func faceWinRatio(db *svg.FontDatabase, id svg.FontID) (r float64) {
	// go-text can panic on a malformed font (D5, D18).
	defer func() {
		if recover() != nil {
			r = 0
		}
	}()
	data, index, found := db.FaceData(id)
	if !found {
		return 0
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || int(index) >= len(loaders) {
		return 0
	}
	table, err := loaders[index].RawTable(ot.MustNewTag("OS/2"))
	if err != nil || len(table) < 78 {
		return 0
	}
	ascent := float64(binary.BigEndian.Uint16(table[74:]))
	descent := float64(binary.BigEndian.Uint16(table[76:]))
	if ascent <= 0 || ascent+descent <= 0 {
		return 0
	}
	return ascent / (ascent + descent)
}
