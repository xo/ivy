# pdffont

The package `pdffont` is a port of the Rust crate `hayro-font`, at the commit
`61f50b42a7f7b15764780b9ba534cba91488bb28` of
[hayro](https://github.com/LaurenzV/hayro) (D14, D16). The crate turns the
font dictionaries of a PDF file, with their embedded font programs, into
fonts. A font maps the character codes of text to glyphs, and it gives the
outline and the advance of each glyph. The package `interpret` will call it.

The code is under the Apache-2.0 or the MIT license, as upstream is. The
files `LICENSE-APACHE` and `LICENSE-MIT` are copies of the files of hayro.
The type `BezPath` of kurbo is in the package `pdf/internal/kurboext`.

## Files

Each Rust file `src/a/b.rs` becomes the Go file `a_b.go`.

| Upstream file | Go file |
| --- | --- |
| `src/lib.rs` | `lib.go`, with its tests in `lib_test.go` |
| `src/cid.rs` | `cid.go` |
| `src/outline.rs` | `outline.go` |
| `src/program/mod.rs` | `program_mod.go` |
| `src/program/pen.rs` | `program_pen.go` |
| `src/standard_font.rs` | `standard_font.go` |
| `src/true_type.rs` | `true_type.go` |
| `src/type1.rs` | `type1.go` |
| `src/type3.rs` | `type3.go` |
| `src/util.rs` | `util.go` |
| `src/generated/mod.rs` | no file. It only declares the modules |
| `src/generated/<name>.rs` | `generated_<name>.go`, made by the script below |
| `assets/*.pfb`, `assets/LICENSE_FOXIT` | `assets/`, byte for byte |
| none | `skrifa_cff.go`, `skrifa_sfnt.go`, `skrifa_type1.go`: the readers of font programs, see below |
| none | `port_test.go`, `skrifa_type1_test.go`, `fuzz_test.go` |

`testdata/` holds eight small files of `tests/pdfs/custom` of hayro, byte for
byte. `testdata/fuzz/FuzzFontProgram` holds an input that made go-text
reserve 7 GB before `sfntTables` existed.

## The readers of font programs

Upstream reads each font program with the crate skrifa. D16 replaces skrifa
with go-text v0.3.5 for TrueType, OpenType and CFF. Three files of this
package are not ports. Each one holds only what go-text does not give. The
header of each file lists the gaps.

1. `skrifa_sfnt.go` reads TrueType and OpenType with go-text. It adds a
   lookup in each subtable of the cmap table without a change of the code,
   the raw fields of the tables OS/2, head and post, a cmap table for a font
   that has none, and a check of the table directory. go-text reserves the
   length that the directory gives for a table before it reads the table.
2. `skrifa_cff.go` reads a bare CFF font (FontFile3 of the subtype Type1C or
   CIDFontType0C, and the substitutes of the standard fonts) with
   `cff.Parse` of go-text. go-text parses a bare CFF font, so no OpenType
   wrapper is needed. go-text skips the built-in encoding, does not export
   the charset, and ignores the FontMatrix. The file reads these three items
   from the Top DICT.
3. `skrifa_type1.go` reads Type 1 fonts (FontFile). go-text has no reader of
   Type 1: no PFA or PFB parser, no eexec, and no Type 1 charstrings. The
   source of skrifa is not in the checkout, so the reader follows the Type 1
   format of Adobe. It reads PFB, PFA with binary or hexadecimal eexec, the
   Subrs and the CharStrings, and it runs flex, the replacement of hints,
   seac and the blend of multiple master fonts.

## The standard fonts

Upstream embeds 14 substitutes of the standard fonts, from PDFium, under the
BSD license of `assets/LICENSE_FOXIT`, "Copyright 2014 PDFium Authors". The
files are 288 KB together. The names end in `.pfb`, but each file holds a
bare CFF font. The package embeds them with `go:embed`, and
`StandardFont.GetFontData` returns them. Upstream embeds them only with its
feature `embed-fonts`. A caller still gives a `FontResolverFn`, as upstream
does. The default resolver of hayro-interpret returns
`GetFontData` of the standard font, or of `PickStandardFont` for a fallback
query.

## The shape of the Go code

1. A Rust enum with data becomes a struct with one field for each case, or
   a sealed interface for `FontGlyph`. A Rust `Option` becomes a second
   result of type `bool`, a nil pointer, or an empty string where the doc
   comment says so.
2. The keys of type `u128` are `Key128`, 16 bytes of FNV-1a. Upstream uses
   SipHash, so the keys differ, but they play the same role.
3. `FontCache` guards its maps with mutexes. Upstream is for one thread.
4. A font is untrusted input. Each error wraps `ivy.ErrFormat` or
   `ivy.ErrLimit`. A call into go-text runs under `safely`, which turns a
   panic into an error. The Type 1 reader bounds the stack (64), the
   nesting of subroutines (16), the operators of one glyph (100,000), the
   glyphs and the subroutines (65,535).

## The differences from upstream

Each difference is marked with `Port:` in the code. These are the main ones:

1. go-text has no hinter. `OutlineGlyph.HintedOutline` always returns
   false, which upstream returns for a font that it cannot hint.
2. go-text picks the subtable of the cmap table for `cmapGlyph` with its
   own rules.
3. The expert encoding of a CFF font maps through the names of
   MacExpertEncoding. The predefined charsets Expert and ExpertSubset are
   not in the package, so the supplements of an encoding cannot use them.
   The FontMatrix of a Font DICT of a CIDFont, and of the CFF table of an
   OpenType font, is not read.
4. The Type 1 reader moves `.notdef` to glyph 0, as FreeType does. A Type 1
   program in a CID font maps a CID to the same glyph index, so the order
   matters there.
5. `readWidths` keeps at most 257 widths, which is all that a code of one
   byte reads. The W array of a CID font is merged by a sweep in n log n
   time, where upstream splits a BTreeMap.
6. `Font.ReadCode` returns the code 0 of length 1 for an offset outside the
   string, where upstream panics. A code that the CMap of a Type 0 font
   does not map takes its length from the codespace ranges of the CMap,
   where upstream reads one byte.
7. A font that the file does not embed first gets a query of the kind
   `Named`, with its name, before the query of a standard font. A resolver
   gives a font of the program for it, such as Liberation Sans for Arial.
   The package `pdf` resolves these queries with `ivy.Config.Fonts` and the
   fonts of the system. The 14 standard fonts, Symbol and ZapfDingbats
   keep their substitutes.
8. A CID font of a CJK collection (Japan1, GB1, CNS1 or Korea1) that the
   file does not embed also holds the substitute of a standard font, its
   latin font. A character that the CJK font of the resolver does not draw,
   such as a Latin letter of Droid Sans Fallback, draws with the latin
   font. A character that neither font draws draws nothing. The code of a
   Unicode CMap, such as UniJIS-UCS2-H, that the collection does not hold
   draws as its character.
9. `sfntTables` cuts the capacity of each table to its end. `ot.WriteTTF`
   of go-text appends the padding of a checksum to a table, and it wrote
   over the start of the next table in the data of the file.

The port keeps the faults of upstream. For example, `newFallbackFontQuery`
reads `FontName` from the font dictionary, and it never sets
`IsFixedPitch`.

## Open problems

1. go-text does not draw the accented glyphs of a Type 2 charstring that
   ends with `endchar` and four arguments (seac). Such a glyph of a CFF font
   draws nothing.
2. `internal/font` of ivy reads fonts with `ParseTTC` of go-text, which
   reads each table with `Loader.RawTable`. It has the memory problem that
   `sfntTables` avoids here.
3. The ledger `docs/upstream/hayro.tsv` does not list this package yet.

## The script of the tables

The script converts the `phf` maps of `src/generated` into Go. Run it from
the root of the repository, then run `gofmt -w pdf/internal/pdffont`:

```sh
python3 -I gentables.py upstream/hayro/hayro-font/src/generated pdf/internal/pdffont
```

```python
#!/usr/bin/env python3
# Converts the phf tables of hayro-font src/generated into Go files.
# Usage: gentables.py <hayro-font/src/generated> <output folder>
import re, sys, os

SRC, OUT = sys.argv[1], sys.argv[2]

HEADER = """// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-font src/generated/{mod}.rs. A script made the
// tables from the Rust file. README.md holds the script.
{see}
package pdffont
"""

# The Go name of each table, by module and Rust name.
NAMES = {
    ("glyph_names", "GLYPH_NAMES"): "glyphNamesTable",
    ("glyph_names", "REVERSE_GLYPH_NAMES"): "glyphNamesReverseTable",
    ("mac_expert", "MAC_EXPERT"): "macExpertTable",
    ("mac_os_roman", "MAC_OS_ROMAN"): "macOSRomanTable",
    ("mac_os_roman", "MAC_OS_ROMAN_INVERSE"): "macOSRomanInverseTable",
    ("mac_roman", "MAC_ROMAN"): "macRomanTable",
    ("mac_roman", "MAC_ROMAN_INVERSE"): "macRomanInverseTable",
    ("standard", "STANDARD"): "standardTable",
    ("symbol", "SYMBOL"): "symbolTable",
    ("tex_glyph_names", "TEX_GLYPH_NAMES"): "texGlyphNamesTable",
    ("win_ansi", "WIN_ANSI"): "winAnsiTable",
    ("zapf_dings", "ZAPF_DING_BATS"): "zapfDingsTable",
    ("zapf_dings", "GLYPH_NAMES"): "zapfDingsGlyphNamesTable",
}

FUNCS = {
    "glyph_names": '''
// glyphNamesGet is get in hayro-font src/generated/glyph_names.rs. It
// returns the Unicode text of a glyph name of the Adobe Glyph List.
func glyphNamesGet(name string) (string, bool) {
	s, ok := glyphNamesTable[name]
	return s, ok
}

// glyphNamesGetReverse is get_reverse in hayro-font
// src/generated/glyph_names.rs. It returns the glyph name of a character.
func glyphNamesGetReverse(unicode rune) (string, bool) {
	s, ok := glyphNamesReverseTable[unicode]
	return s, ok
}
''',
    "mac_expert": '''
// macExpertGet is get in hayro-font src/generated/mac_expert.rs. It returns
// the glyph name of a code in MacExpertEncoding.
func macExpertGet(code uint8) (string, bool) {
	s := macExpertTable[code]
	return s, s != ""
}
''',
    "mac_os_roman": '''
// macOSRomanGet is get in hayro-font src/generated/mac_os_roman.rs. It
// returns the glyph name of a code that Mac OS Roman adds to MacRomanEncoding.
func macOSRomanGet(code uint8) (string, bool) {
	s := macOSRomanTable[code]
	return s, s != ""
}

// macOSRomanGetInverse is get_inverse in hayro-font
// src/generated/mac_os_roman.rs. It returns the code of a glyph name.
func macOSRomanGetInverse(name string) (uint8, bool) {
	c, ok := macOSRomanInverseTable[name]
	return c, ok
}
''',
    "mac_roman": '''
// macRomanGet is get in hayro-font src/generated/mac_roman.rs. It returns the
// glyph name of a code in MacRomanEncoding.
func macRomanGet(code uint8) (string, bool) {
	s := macRomanTable[code]
	return s, s != ""
}

// macRomanGetInverse is get_inverse in hayro-font src/generated/mac_roman.rs.
// It returns the code of a glyph name.
func macRomanGetInverse(name string) (uint8, bool) {
	c, ok := macRomanInverseTable[name]
	return c, ok
}
''',
    "standard": '''
// standardGet is get in hayro-font src/generated/standard.rs. It returns the
// glyph name of a code in StandardEncoding.
func standardGet(code uint8) (string, bool) {
	s := standardTable[code]
	return s, s != ""
}
''',
    "symbol": '''
// symbolGet is get in hayro-font src/generated/symbol.rs. It returns the
// glyph name of a code in the built-in encoding of the font Symbol.
func symbolGet(code uint8) (string, bool) {
	s := symbolTable[code]
	return s, s != ""
}
''',
    "tex_glyph_names": '''
// texGlyphNamesGet is get in hayro-font src/generated/tex_glyph_names.rs. It
// returns the Unicode text of a glyph name of TeX.
func texGlyphNamesGet(name string) (string, bool) {
	s, ok := texGlyphNamesTable[name]
	return s, ok
}
''',
    "win_ansi": '''
// winAnsiGet is get in hayro-font src/generated/win_ansi.rs. It returns the
// glyph name of a code in WinAnsiEncoding. A code over 40 that the table does
// not hold gives "bullet".
func winAnsiGet(code uint8) (string, bool) {
	if s := winAnsiTable[code]; s != "" {
		return s, true
	}
	if code > 40 {
		return "bullet", true
	}
	return "", false
}
''',
    "zapf_dings": '''
// zapfDingsGet is get in hayro-font src/generated/zapf_dings.rs. It returns
// the glyph name of a code in the built-in encoding of the font ZapfDingbats.
func zapfDingsGet(code uint8) (string, bool) {
	s := zapfDingsTable[code]
	return s, s != ""
}

// zapfDingsGlyphNameToUnicode is glyph_name_to_unicode in hayro-font
// src/generated/zapf_dings.rs. It returns the character of a glyph name of
// the font ZapfDingbats.
func zapfDingsGlyphNameToUnicode(name string) (rune, bool) {
	r, ok := zapfDingsGlyphNamesTable[name]
	return r, ok
}
''',
    "metrics": "",
}

def goname(mod, name):
    if (mod, name) in NAMES:
        return NAMES[(mod, name)]
    # metrics: COURIER_BOLD -> metricsCourierBold
    parts = name.lower().split("_")
    return "metrics" + "".join(p.capitalize() for p in parts)

def gostr(s):
    def rep(m):
        v = int(m.group(1), 16)
        return "\\u%04X" % v if v <= 0xFFFF else "\\U%08X" % v
    return '"' + re.sub(r"\\u\{([0-9A-Fa-f]+)\}", rep, s) + '"'

def key(k):
    k = k.strip()
    m = re.fullmatch(r"(0o)?([0-9]+)_u8", k)
    if m:
        return int(m.group(2), 8 if m.group(1) else 10)
    m = re.fullmatch(r"'\\u\{([0-9A-Fa-f]+)\}'", k)
    if m:
        return "0x%04X" % int(m.group(1), 16)
    m = re.fullmatch(r'"(.*)"', k)
    if m:
        return gostr(m.group(1))
    m = re.fullmatch(r"([0-9.]+)_f32", k)
    if m:
        return m.group(1)
    raise ValueError(k)

GOTYPE = {"&'static str": "string", "u8": "uint8", "char": "rune", "f32": "float32"}

for fname in sorted(os.listdir(SRC)):
    if not fname.endswith(".rs") or fname == "mod.rs":
        continue
    mod = fname[:-3]
    text = open(os.path.join(SRC, fname)).read()
    see = re.search(r"^// See <(.*)>", text, re.M)
    seetext = ""
    if see:
        seetext = "//\n// The table follows\n// <%s>.\n" % see.group(1)
    out = [HEADER.format(mod=mod, see=seetext), FUNCS[mod]]
    for m in re.finditer(r"static ([A-Z_]+): phf::Map<([^,]+), ([^>]+)> = phf_map! \{(.*?)\n\};", text, re.S):
        name, kt, vt, body = m.groups()
        entries = []
        for line in body.strip().splitlines():
            line = line.strip().rstrip(",")
            if not line:
                continue
            k, v = line.split(" => ", 1)
            entries.append((key(k), key(v)))
        gn = goname(mod, name)
        comment = "// %s is %s in hayro-font src/generated/%s.rs." % (gn, name, mod)
        if kt == "u8":
            out.append("\n%s\nvar %s = [256]%s{\n" % (comment, gn, GOTYPE[vt]))
            for k, v in sorted(entries):
                out.append("\t%d: %s,\n" % (k, v))
        else:
            out.append("\n%s\nvar %s = map[%s]%s{\n" % (comment, gn, GOTYPE[kt], GOTYPE[vt]))
            for k, v in entries:
                out.append("\t%s: %s,\n" % (k, v))
        out.append("}\n")
    with open(os.path.join(OUT, "generated_%s.go" % mod), "w") as f:
        f.write("".join(out))
```
