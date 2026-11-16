package fonts

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"testing"
	"unicode/utf16"
)

// want holds each font file of FS, with its family and the SHA-256 that
// README.md gives.
var want = map[string]struct {
	family string
	sha256 string
}{
	"liberation/LiberationSans-Regular.ttf":    {"Liberation Sans", "76d04c18ea243f426b7de1f3ad208e927008f961dc5945e5aad352d0dfde8ee8"},
	"liberation/LiberationSans-Bold.ttf":       {"Liberation Sans", "788abee4c806d660e8aee46689dd8540cd4bb98da03dcc9d171ce3efd99a9173"},
	"liberation/LiberationSans-Italic.ttf":     {"Liberation Sans", "e5bae5c4cde31f22142753855f4f8fb86da6ff39955ed3c0a11248b0d16948b0"},
	"liberation/LiberationSans-BoldItalic.ttf": {"Liberation Sans", "698da70fc191cc5f33ad4d6d3fe830fe4624b898ea2e3169955928b7c491f1ee"},

	"liberation/LiberationSerif-Regular.ttf":    {"Liberation Serif", "058ea80864aef09a23f45cbec2bb5400bc3dfbdea01c3f10538a21fcb497fb74"},
	"liberation/LiberationSerif-Bold.ttf":       {"Liberation Serif", "d754ba427cfe0bca54ae052384baa8f842da5bd6550ad4da024ac441e7a7d5ce"},
	"liberation/LiberationSerif-Italic.ttf":     {"Liberation Serif", "0e3dea9f8d613e006ccfa62201f33e265d19167bd0907725c3e145368b04fc2e"},
	"liberation/LiberationSerif-BoldItalic.ttf": {"Liberation Serif", "f17db8af71e24d2066b587546021d4f0b296be389512b658dec3c09affeb11a7"},

	"liberation/LiberationMono-Regular.ttf":    {"Liberation Mono", "f2b83c763e8afd21709333370bed4774337fae82267937e2b5aea7e2fbd922c1"},
	"liberation/LiberationMono-Bold.ttf":       {"Liberation Mono", "bd62a0672d0b9b6710b01df434c80ad54fa5f0835207eb7b17b7a761463067bb"},
	"liberation/LiberationMono-Italic.ttf":     {"Liberation Mono", "605c01c711b44480a7508d349dfbf3264e81fa43d69e61cfa7d10b86e764c4d1"},
	"liberation/LiberationMono-BoldItalic.ttf": {"Liberation Mono", "79451f3c09fe25116098853b7a2ca6e2436220ccc11af022979adbcf195be130"},

	"carlito/Carlito-Regular.ttf":    {"Carlito", "f6418f708baede9789daef5d458c0f53d2a888af9820e8062934e504fedc6595"},
	"carlito/Carlito-Bold.ttf":       {"Carlito", "bb5d20f79b82599ec72983597437373a80f2d2085fa91fc144fd74e876a594db"},
	"carlito/Carlito-Italic.ttf":     {"Carlito", "0b019225e58d702bfedcbd35c21696769f8ee115cb6343f84c2f240312450d1c"},
	"carlito/Carlito-BoldItalic.ttf": {"Carlito", "b32928186c119599e03ca6a1ffc680fdcb7fac95772f4b95d989cf6cd3861517"},

	"caladea/Caladea-Regular.ttf":    {"Caladea", "f1e899278b7b4491aba5b6a8253c4b04c050cc59b21865be5c37559a775153cd"},
	"caladea/Caladea-Bold.ttf":       {"Caladea", "ae3cb2dcbc925809dd29d2a44e9802211cab66be541bacbfc9c08c74b27c3742"},
	"caladea/Caladea-Italic.ttf":     {"Caladea", "4359a8e24f748b6447b1ff6d7a174febe70961d29f8bb8634b56dacd740a3deb"},
	"caladea/Caladea-BoldItalic.ttf": {"Caladea", "ccabaa7b7e2fdf253d2b1a5fa699dd8a3df8d835a9eb285ad82631a677eb76c0"},

	"droid/DroidSansFallback.ttf": {"Droid Sans Fallback", "21b96a0377f067833a93af3082eb28d4ffab7a8cd46bfd513286f1d64b7b0949"},
}

// The texts that the licenses hold.
const (
	ofl    = "SIL OPEN FONT LICENSE Version 1.1"
	apache = "Apache License\n                           Version 2.0"
)

// licenses holds the license file of each folder, with a text of its
// license.
var licenses = map[string]string{
	"liberation/LICENSE": ofl,
	"carlito/OFL.txt":    ofl,
	"caladea/OFL.txt":    ofl,
	"droid/NOTICE":       apache,
}

// TestFonts makes sure that each file of FS is a TrueType font of its
// family, with the bytes that README.md gives, and that FS holds no other
// font.
func TestFonts(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	err := fs.WalkDir(FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".ttf" {
			return err
		}
		seen[name] = true
		w, ok := want[name]
		if !ok {
			t.Errorf("%s: unexpected font", name)
			return nil
		}
		data, err := fs.ReadFile(FS(), name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != w.sha256 {
			t.Errorf("%s: SHA-256 %x, expected %s", name, sum, w.sha256)
		}
		family, err := familyOf(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return nil
		}
		if family != w.family {
			t.Errorf("%s: family %q, expected %q", name, family, w.family)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("%s: missing", name)
		}
	}
}

// TestLicenses makes sure that each folder holds its license: the SIL Open
// Font License 1.1, or the Apache License 2.0 for Droid Sans Fallback.
func TestLicenses(t *testing.T) {
	t.Parallel()
	for name, text := range licenses {
		data, err := fs.ReadFile(FS(), name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), text) {
			t.Errorf("%s: does not hold %q", name, text)
		}
	}
}

// cjk holds a character of each of the four CJK character collections of
// Adobe: Japan1, GB1, CNS1 and Korea1.
var cjk = []rune{'あ', '这', '這', '한'}

// TestCJK makes sure that Droid Sans Fallback draws a character of each
// CJK character collection, so that it can be the fallback of each.
func TestCJK(t *testing.T) {
	t.Parallel()
	data, err := fs.ReadFile(FS(), "droid/DroidSansFallback.ttf")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range cjk {
		if !covers(t, data, r) {
			t.Errorf("Droid Sans Fallback does not draw %q", r)
		}
	}
	if covers(t, data, 'A') {
		t.Error("Droid Sans Fallback draws 'A', expected only CJK characters")
	}
}

// errFont is the error of a file that is not a TrueType font.
var errFont = errors.New("not a TrueType font")

// tablesOf returns the tables of a TrueType font by their tags. It reads
// the table directory, and makes sure that the table head has its magic
// number.
func tablesOf(data []byte) (map[string][]byte, error) {
	if len(data) < 12 || binary.BigEndian.Uint32(data) != 0x00010000 {
		return nil, errFont
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	if len(data) < 12+16*n {
		return nil, fmt.Errorf("table directory: %w", errFont)
	}
	tables := map[string][]byte{}
	for i := range n {
		rec := data[12+16*i:]
		off, size := int(binary.BigEndian.Uint32(rec[8:])), int(binary.BigEndian.Uint32(rec[12:]))
		if off < 0 || size < 0 || off > len(data) || size > len(data)-off {
			return nil, fmt.Errorf("table %q: %w", rec[:4], errFont)
		}
		tables[string(rec[:4])] = data[off : off+size]
	}
	head := tables["head"]
	if len(head) < 16 || binary.BigEndian.Uint32(head[12:]) != 0x5F0F3CF5 {
		return nil, fmt.Errorf("table head: %w", errFont)
	}
	return tables, nil
}

// familyOf returns the family of a TrueType font: the name with the ID 1
// for Windows and Unicode in its table name.
func familyOf(data []byte) (string, error) {
	tables, err := tablesOf(data)
	if err != nil {
		return "", err
	}
	name := tables["name"]
	if len(name) < 6 {
		return "", fmt.Errorf("table name: %w", errFont)
	}
	count, strs := int(binary.BigEndian.Uint16(name[2:])), int(binary.BigEndian.Uint16(name[4:]))
	if len(name) < 6+12*count {
		return "", fmt.Errorf("table name: %w", errFont)
	}
	for i := range count {
		rec := name[6+12*i:]
		platform, encoding := binary.BigEndian.Uint16(rec), binary.BigEndian.Uint16(rec[2:])
		id := binary.BigEndian.Uint16(rec[6:])
		size, off := int(binary.BigEndian.Uint16(rec[8:])), int(binary.BigEndian.Uint16(rec[10:]))
		if platform != 3 || encoding != 1 || id != 1 {
			continue
		}
		if strs+off+size > len(name) || size%2 != 0 {
			return "", fmt.Errorf("table name: %w", errFont)
		}
		b := name[strs+off : strs+off+size]
		u := make([]uint16, size/2)
		for j := range u {
			u[j] = binary.BigEndian.Uint16(b[2*j:])
		}
		return string(utf16.Decode(u)), nil
	}
	return "", fmt.Errorf("no family: %w", errFont)
}

// covers reports whether the subtable of the cmap table of a TrueType font
// for Windows and Unicode maps r to a glyph other than 0. It reads the
// formats 4 and 12.
func covers(t *testing.T, data []byte, r rune) bool {
	t.Helper()
	tables, err := tablesOf(data)
	if err != nil {
		t.Fatal(err)
	}
	cmap := tables["cmap"]
	if len(cmap) < 4 {
		t.Fatal("no table cmap")
	}
	n := int(binary.BigEndian.Uint16(cmap[2:]))
	for i := range n {
		rec := 4 + 8*i
		if rec+8 > len(cmap) {
			break
		}
		platform, encoding := binary.BigEndian.Uint16(cmap[rec:]), binary.BigEndian.Uint16(cmap[rec+2:])
		off := int(binary.BigEndian.Uint32(cmap[rec+4:]))
		if platform != 3 || (encoding != 1 && encoding != 10) || off+4 > len(cmap) {
			continue
		}
		sub := cmap[off:]
		switch binary.BigEndian.Uint16(sub) {
		case 4:
			if covers4(sub, r) {
				return true
			}
		case 12:
			if covers12(sub, r) {
				return true
			}
		}
	}
	return false
}

// covers4 reports whether a subtable of the format 4 maps r to a glyph.
func covers4(sub []byte, r rune) bool {
	if r > 0xFFFF || len(sub) < 14 {
		return false
	}
	segs := int(binary.BigEndian.Uint16(sub[6:])) / 2
	if len(sub) < 16+8*segs {
		return false
	}
	c := uint16(r)
	ends, starts := sub[14:], sub[16+2*segs:]
	deltas, offsets := sub[16+4*segs:], sub[16+6*segs:]
	for i := range segs {
		end, start := binary.BigEndian.Uint16(ends[2*i:]), binary.BigEndian.Uint16(starts[2*i:])
		if c > end || c < start {
			continue
		}
		delta, ro := binary.BigEndian.Uint16(deltas[2*i:]), int(binary.BigEndian.Uint16(offsets[2*i:]))
		if ro == 0 {
			return c+delta != 0
		}
		at := 16 + 6*segs + 2*i + ro + 2*int(c-start)
		if at+2 > len(sub) {
			return false
		}
		g := binary.BigEndian.Uint16(sub[at:])
		return g != 0 && g+delta != 0
	}
	return false
}

// covers12 reports whether a subtable of the format 12 maps r to a glyph.
func covers12(sub []byte, r rune) bool {
	if len(sub) < 16 {
		return false
	}
	n := int(binary.BigEndian.Uint32(sub[12:]))
	for i := range n {
		g := 16 + 12*i
		if g+12 > len(sub) {
			return false
		}
		start, end := rune(binary.BigEndian.Uint32(sub[g:])), rune(binary.BigEndian.Uint32(sub[g+4:]))
		if r >= start && r <= end {
			return binary.BigEndian.Uint32(sub[g+8:])+uint32(r-start) != 0
		}
	}
	return false
}
