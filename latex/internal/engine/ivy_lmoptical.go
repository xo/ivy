// This file of ivy replaces lmoptical.go of go-tex/engine (D62). It keeps
// the tables of the optical sizes of Latin Modern and the functions that
// font.go calls, and takes the font files from a program in place of the
// files that upstream embeds. The tables come from lmoptical.go at the
// commit 8234996d66f827438f248e8b0059031f52cb9964, which transcribes
// tulmr.fd and tulmtt.fd: the ranges are half-open, lower <= size < upper.

package engine

import (
	"crypto/sha256"
	"strconv"
	"sync"

	"github.com/xo/ivy/latex/internal/opentype"
)

// opticalRange is one \DeclareFontShape entry: the file to use while the
// requested size is below upTo. upTo == 0 means "and everything above".
type opticalRange struct {
	upTo float64
	data []byte
	name string
}

// opticalFamily is a size→file table, in ascending order of upTo.
type opticalFamily []opticalRange

// pick returns the file for a requested size, following \check@range's
// half-open rule: the first range whose upTo is strictly greater than pt.
func (fam opticalFamily) pick(pt float64) (data []byte, name string) {
	for _, r := range fam {
		if r.upTo == 0 || pt < r.upTo {
			return r.data, r.name
		}
	}
	if len(fam) == 0 {
		return nil, ""
	}
	last := fam[len(fam)-1]
	return last.data, last.name
}

// opticalSpec is a table of the files of one shape, by their names.
type opticalSpec struct {
	upTo float64
	name string
}

// The tables of tulmr.fd and tulmtt.fd, as lmoptical.go has them. The
// third file of each list is the base, the 10 point design.
var (
	lmRegularSpec = []opticalSpec{
		{5.5, "lmroman5-regular"}, {6.5, "lmroman6-regular"}, {7.5, "lmroman7-regular"},
		{8.5, "lmroman8-regular"}, {9.5, "lmroman9-regular"}, {11, "lmroman10-regular"},
		{15, "lmroman12-regular"}, {0, "lmroman17-regular"},
	}
	lmBoldSpec = []opticalSpec{
		{5.5, "lmroman5-bold"}, {6.5, "lmroman6-bold"}, {7.5, "lmroman7-bold"},
		{8.5, "lmroman8-bold"}, {9.5, "lmroman9-bold"}, {11, "lmroman10-bold"},
		{0, "lmroman12-bold"},
	}
	lmItalicSpec = []opticalSpec{
		{7.5, "lmroman7-italic"}, {8.5, "lmroman8-italic"}, {9.5, "lmroman9-italic"},
		{11, "lmroman10-italic"}, {0, "lmroman12-italic"},
	}
	lmMonoSpec = []opticalSpec{
		{8.5, "lmmono8-regular"}, {9.5, "lmmono9-regular"}, {11, "lmmono10-regular"},
		{0, "lmmono12-regular"},
	}
)

// The tables of tulmss.fd for Latin Modern Sans (D72): the medium and the
// oblique shapes at five optical sizes, and the bold extended shapes at
// one. The italic shape is the oblique shape, and the bold series and the
// semibold condensed series take the bold extended series.
var (
	lmSansSpec = []opticalSpec{
		{8.5, "lmsans8-regular"}, {9.5, "lmsans9-regular"}, {11, "lmsans10-regular"},
		{15.5, "lmsans12-regular"}, {0, "lmsans17-regular"},
	}
	lmSansObliqueSpec = []opticalSpec{
		{8.5, "lmsans8-oblique"}, {9.5, "lmsans9-oblique"}, {11, "lmsans10-oblique"},
		{15.5, "lmsans12-oblique"}, {0, "lmsans17-oblique"},
	}
	lmSansBoldSpec        = []opticalSpec{{0, "lmsans10-bold"}}
	lmSansBoldObliqueSpec = []opticalSpec{{0, "lmsans10-boldoblique"}}
)

// LatinModernFiles are the names of the files of Latin Modern that the
// engine uses, without the extension .otf. The math font is
// latinmodern-math.
func LatinModernFiles() []string {
	var names []string
	for _, spec := range [][]opticalSpec{
		lmRegularSpec, lmBoldSpec, lmItalicSpec, lmMonoSpec,
		lmSansSpec, lmSansObliqueSpec, lmSansBoldSpec, lmSansBoldObliqueSpec,
	} {
		for _, s := range spec {
			names = append(names, s.name)
		}
	}
	return append(names, "latinmodern-math")
}

// The interned font files. A program can give the same font many times,
// one document at a time. The files are kept once for each content, so
// that the tables and the parsed programs, which are keyed by the address
// of the data, do not grow with the documents.
var (
	internMu sync.Mutex
	interned = map[[sha256.Size]byte][]byte{}
	// families maps the address of a base file to the table of its shape.
	families = map[*byte]opticalFamily{}
)

// intern returns the kept copy of data.
func intern(data []byte) []byte {
	sum := sha256.Sum256(data)
	internMu.Lock()
	defer internMu.Unlock()
	if d, ok := interned[sum]; ok {
		return d
	}
	interned[sum] = data
	return data
}

// LatinModernFaces holds the faces of Latin Modern of a run: the base file
// of each shape, and the math font. A field is nil when the program gave no
// such file.
type LatinModernFaces struct {
	Regular, Bold, Italic, Mono, Math []byte
	// Sans, SansBold, SansItalic and SansBoldItalic are Latin Modern Sans:
	// the medium, the bold extended, the oblique and the bold extended
	// oblique shapes.
	Sans, SansBold, SansItalic, SansBoldItalic []byte
}

// RegisterLatinModern interns the files of Latin Modern, by their names
// without the extension, and returns the base file of each shape. The
// engine then sets a size of a shape with the file that tulmr.fd names for
// it. A shape whose base file is missing is nil. A missing file of
// another size takes the base file.
func RegisterLatinModern(files map[string][]byte) LatinModernFaces {
	reg := func(spec []opticalSpec) []byte {
		base := files[spec[len(spec)-1].name]
		for _, s := range spec {
			if s.upTo == 11 {
				base = files[s.name]
			}
		}
		if len(base) == 0 {
			return nil
		}
		base = intern(base)
		fam := make(opticalFamily, 0, len(spec))
		for _, s := range spec {
			data := base
			if d := files[s.name]; len(d) > 0 {
				data = intern(d)
			}
			fam = append(fam, opticalRange{upTo: s.upTo, data: data, name: s.name})
		}
		internMu.Lock()
		families[&base[0]] = fam
		internMu.Unlock()
		return base
	}
	faces := LatinModernFaces{
		Regular: reg(lmRegularSpec),
		Bold:    reg(lmBoldSpec),
		Italic:  reg(lmItalicSpec),
		Mono:    reg(lmMonoSpec),

		Sans:           reg(lmSansSpec),
		SansBold:       reg(lmSansBoldSpec),
		SansItalic:     reg(lmSansObliqueSpec),
		SansBoldItalic: reg(lmSansBoldObliqueSpec),
	}
	if m := files["latinmodern-math"]; len(m) > 0 {
		faces.Math = intern(m)
	}
	return faces
}

// opticalDesignPt is the size a Latin Modern file was drawn for, read out
// of its own name: lmroman9-regular is a 9pt design.
func opticalDesignPt(name string) float64 {
	digits := ""
	for _, r := range name {
		if r >= '0' && r <= '9' {
			digits += string(r)
			continue
		}
		if digits != "" {
			break
		}
	}
	if digits == "" {
		return 0
	}
	v, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return float64(v)
}

// opticalOf returns the table a set of font bytes belongs to, or nil for
// bytes that RegisterLatinModern did not give. As upstream, it is keyed on
// the identity of the slice: a font that a document names is not re-faced.
func opticalOf(b []byte) opticalFamily {
	if len(b) == 0 {
		return nil
	}
	internMu.Lock()
	defer internMu.Unlock()
	return families[&b[0]]
}

// parsedOptical caches the parsed program per file, so crossing a size
// boundary does not parse a font again.
var (
	parsedOpticalMu sync.Mutex
	parsedOptical   = map[*byte]*opentype.Font{}
)

// opticalProgram parses (once) the file a table names for pt.
func opticalProgram(fam opticalFamily, pt float64) (*opentype.Font, []byte, string) {
	data, name := fam.pick(pt)
	if len(data) == 0 {
		return nil, nil, ""
	}
	key := &data[0]
	parsedOpticalMu.Lock()
	defer parsedOpticalMu.Unlock()
	if f, ok := parsedOptical[key]; ok {
		return f, data, name
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, nil, ""
	}
	parsedOptical[key] = f
	return f, data, name
}
