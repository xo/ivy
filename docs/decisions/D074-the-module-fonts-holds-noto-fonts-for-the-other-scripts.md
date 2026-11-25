# D74. The module fonts holds Noto fonts for the other scripts

Status: Decided.

The module `fonts` draws Latin, Greek, Cyrillic and Hebrew with Liberation,
Carlito and Caladea (D24). It draws Chinese, Japanese and Korean (CJK) with
Droid Sans Fallback. It draws no other script. If a program does not give
ivy the fonts of the system (D23), the text of a document in Arabic, Thai or
Devanagari falls to a font that does not hold its letters. The page then
shows empty boxes or nothing. iv on a machine with few fonts and a program
in WebAssembly are in this case.

On 2026-11-26 the maintainers asked for Noto fonts as the fallback of these
scripts. Droid Sans Fallback stays the font of CJK.

## The facts

1. Liberation Sans and Liberation Serif draw Hebrew and Yiddish, as
   `fc-query` reports. Hebrew needs no new font.
2. The Noto project gives one family for each script. The repository
   `notofonts/notofonts.github.io` holds the built files. Its own `LICENSE`
   is the Apache License 2.0 of its scripts. Each repository of a script,
   such as `notofonts/arabic`, holds `OFL.txt`. The fonts are under the SIL
   Open Font License 1.1, which D7 allows.
3. Each family has a file for documents and a file for interfaces, such as
   `NotoSansDevanagari` and `NotoSansDevanagariUI`. The file for
   interfaces has shorter ascenders and descenders, so that it fits a line
   of a menu. The file for documents is the one that ivy needs.
4. These are the families, with the size of the regular file and the bold
   file of `unhinted/ttf` at commit `d14cc173a70dc57962ed2d4ad278fc9df83bf9f3`
   of `notofonts/notofonts.github.io`:

   | Script | Family | Regular and bold |
   | --- | --- | --- |
   | Arabic | Noto Naskh Arabic | 317 KB |
   | Devanagari | Noto Sans Devanagari | 368 KB |
   | Bengali | Noto Sans Bengali | 206 KB |
   | Tamil | Noto Sans Tamil | 93 KB |
   | Telugu | Noto Sans Telugu | 331 KB |
   | Kannada | Noto Sans Kannada | 269 KB |
   | Malayalam | Noto Sans Malayalam | 153 KB |
   | Gujarati | Noto Sans Gujarati | 284 KB |
   | Gurmukhi | Noto Sans Gurmukhi | 72 KB |
   | Oriya | Noto Sans Oriya | 261 KB |
   | Sinhala | Noto Sans Sinhala | 179 KB |
   | Thai | Noto Sans Thai | 42 KB |
   | Lao | Noto Sans Lao | 43 KB |
   | Khmer | Noto Sans Khmer | 136 KB |
   | Myanmar | Noto Sans Myanmar | 295 KB |
   | Ethiopic | Noto Sans Ethiopic | 578 KB |
   | Armenian | Noto Sans Armenian | 38 KB |
   | Georgian | Noto Sans Georgian | 71 KB |

5. The 36 files have 3.7 MB. The 18 regular files have 1.9 MB. Today the
   module makes a program larger by about 15.3 MB.
6. Noto Naskh Arabic is in the style of the text of books and letters. The
   Arabic letters of Times New Roman and Arial in Office are in that style.
   Noto Sans Arabic is in a Kufi style for headings and screens.
7. okage shapes these scripts. Its port of HarfBuzz has the shapers of
   Arabic, Hebrew, the Indic scripts, Khmer and Myanmar, and the Universal
   Shaping Engine (okage D101).
8. The layout draws a character that the fonts of a style do not draw with
   the fallback selector of okage, over each face of the font database
   (okage D39). A new face of `fonts.FS` reaches the ebooks, the Office
   formats and the formats of text without a change of code. The package
   `pdf` looks for a fallback face only for a CJK collection
   (`pdf/fonts.go`).
9. The review corpus holds `poi/docx/rtl.docx`,
   `pdfjs/pdf/arabic-cid-truetype.pdf`,
   `idpf/epub/regime-anticancer-arabic.epub` and
   `fbreaderj/fb2/minihelp.th.fb2`. The review gives ivy the fonts of the
   system (`-system-fonts`), and the machine of the review has 2033 faces of
   Noto. So the review shows the change only with `-system-fonts=false`.
10. These fonts do not fix the three lines in the box of
    `pdfjs/pdf/thuluth-features.pdf`. That file does not embed Traditional
    Arabic and has no ToUnicode. Its codes are the glyph numbers of that
    font, not characters.

## The options

1. Add the regular file and the bold file of the 18 families: 36 files,
   3.7 MB.
2. Add the regular files only: 18 files, 1.9 MB. The bold text of these
   scripts takes the regular face.
3. Add Arabic, Devanagari and Thai only, regular and bold: 6 files,
   0.7 MB. A program gives the fonts of the other scripts.
4. Add no file. A program gives Noto fonts in `Config.Fonts`, or with
   `Config.SystemFonts`.

## The proposal

Option 1, with these changes:

- The folder `fonts/noto` holds the 36 files and `OFL.txt`. `fonts.FS`
  gives the folder, and the doc comment of the package lists the families.
- A test of the module `fonts` makes sure that each face draws a letter of
  its script, as `TestCJK` does for Droid Sans Fallback.
- For a font that a PDF file does not embed and that is not of a CJK
  collection, the package `pdf` draws a character that the substitute does
  not draw with the face of `fonts` that draws it. The character comes from
  the ToUnicode map of the font.
- `docs/UPSTREAM.md` and `docs/upstream/upstreams.tsv` record the commit
  of each file.

## The decision

On 2026-11-26 the maintainers answered question 31 and chose option 1, as
the proposal says.
