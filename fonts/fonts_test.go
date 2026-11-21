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

// latinModern holds each OpenType file of Latin Modern in FS, with the
// SHA-256 that README.md gives.
var latinModern = map[string]string{
	"lm/latinmodern-math.otf":  "6075562b771f8b82f0c179e363389684f2dd09de30038269e2628e504bd7be0f",
	"lm/lmmono10-regular.otf":  "22deb6d3be3ffcb40b33f0e010afb256c677d97c2a2bdee1f5740cdc97751558",
	"lm/lmmono12-regular.otf":  "b826e490fa671f254840abe88afea0975b606f57fb4a8b93f53787c26d8ac319",
	"lm/lmmono8-regular.otf":   "17319490d20f24e192bd7e7999484b95111a4654ac2620e7aa215591142ae997",
	"lm/lmmono9-regular.otf":   "cdd53c7dc01a1d25f21eaf7cae44bfb8a9665e4cbd60e5f8da14287aca819bb9",
	"lm/lmroman10-bold.otf":    "102fe06c430a8b681b2bf6876b7cd967ae4d47b4b6b41d915eb7913b726d9fb1",
	"lm/lmroman10-italic.otf":  "c1fce25075567bb8dbf2151658c3b442690041db17a2d49fc9e55905ea5b7169",
	"lm/lmroman10-regular.otf": "1aa18cfefa58132c52ce5de70db1fd1154201c19cd2b2cdaffba4906a33e6852",
	"lm/lmroman12-bold.otf":    "28c8782ac2b6486958b5dc7610ada7800c53546ff7f36bc65909a876e1cd338e",
	"lm/lmroman12-italic.otf":  "ada6b3f451238784e5a5ea75d43efa1eb4329cd75ce65bf047ed2f31b7b5e2f3",
	"lm/lmroman12-regular.otf": "e6be218ae83e61aa8a29990d3cdc401c678c1962188cb9a4a8b6359e4f5e5870",
	"lm/lmroman17-regular.otf": "ed048c80fb4b67663b80899b89fe5fba17d7442e1286d61fd08fe5689c6b7c3c",
	"lm/lmroman5-bold.otf":     "6326fb955d07bedcc8f73684e6b6bfa76706d38f3a85bd89c0659504ace366a1",
	"lm/lmroman5-regular.otf":  "954bd9c376597c7043e7b616430163e0158155edf7434fd70082cf8460c08533",
	"lm/lmroman6-bold.otf":     "f1ebd52916182bfc2e3f9d4b5ba847ad6acc6c7761354c0dc052fb01ec14f73c",
	"lm/lmroman6-regular.otf":  "501396c437f76c260b8724265b8495f318a640ee5a4674852ecbb20daf256696",
	"lm/lmroman7-bold.otf":     "063ea9e378b4fca4fd59771cf522faa21eb7c8516c89464745c79725b384fb9a",
	"lm/lmroman7-italic.otf":   "19d20f2bb00fbffe5d3d66e9b6a54f0108289068029e9d88649d981d53613ddf",
	"lm/lmroman7-regular.otf":  "48c4b46ab255eec96588fb992fba1ed6102e53d051da480738a61cc6cdb328c3",
	"lm/lmroman8-bold.otf":     "04d4f633c3da31a169f4c54a7fe8c7e63925b39d030381f8d3eab955f16be7b2",
	"lm/lmroman8-italic.otf":   "fb0536b5c3eee5b5622cd827ceb410e6514e36512ebdc51d2cf98a96c06d44e8",
	"lm/lmroman8-regular.otf":  "f73015c3827596586faa380c398501caf9b5bcbb6f40b46af6a51277ca97c9fc",
	"lm/lmroman9-bold.otf":     "f72feac9ea12eb858d8093d6c0ee710a6c25f59b61278c88ec41c0a680838415",
	"lm/lmroman9-italic.otf":   "2106cffcdc9f9cb411d7c3441e34d77a168b998a58b40e727ab5b815437d1975",
	"lm/lmroman9-regular.otf":  "8ab7b017fc67ea0e30cbf9e7ebd940010442ea67b40cf65f87d73f3ca11b0e6f",
}

// TestLatinModern makes sure that each OpenType file of TeX is a file of
// Latin Modern with the bytes that README.md gives, that the folder lm
// holds the GUST Font License and the notices, and that FS holds no file
// of Latin Modern.
func TestLatinModern(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	err := fs.WalkDir(TeX(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".otf" {
			return err
		}
		seen[name] = true
		want, ok := latinModern[name]
		if !ok {
			t.Errorf("%s: unexpected font", name)
			return nil
		}
		data, err := fs.ReadFile(TeX(), name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		if len(data) < 4 || string(data[:4]) != "OTTO" {
			t.Errorf("%s: not an OpenType font with CFF", name)
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != want {
			t.Errorf("%s: SHA-256 %x, expected %s", name, sum, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name := range latinModern {
		if !seen[name] {
			t.Errorf("%s: missing", name)
		}
	}
	for name, text := range map[string]string{
		"lm/GUST-FONT-LICENSE.txt":          "GUST Font License",
		"lm/README-Latin-Modern.txt":        "Copyright 2003--2021 by B. Jackowski and J.M. Nowacki",
		"lm/README-Latin-Modern-Math.txt":   "Copyright 2012--2014",
		"lm/MANIFEST-Latin-Modern.txt":      "lmroman10-regular.otf",
		"lm/MANIFEST-Latin-Modern-Math.txt": "latinmodern-math.otf",
	} {
		data, err := fs.ReadFile(TeX(), name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), text) {
			t.Errorf("%s: does not hold %q", name, text)
		}
	}
	if _, err := fs.Stat(FS(), "lm"); err == nil {
		t.Error("expected no folder lm in FS")
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
