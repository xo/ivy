# D72. The module fonts holds Latin Modern Sans

Status: Decided.

LaTeX sets `\textsf` and `\sffamily` in Latin Modern Sans when a document
uses the fonts of Latin Modern. The module `fonts` holds 25 files of Latin
Modern: the roman, the bold, the italic, the typewriter and the math (D65).
It holds no file of Latin Modern Sans. The engine of LaTeX binds `\sf` to
the field `Sans` of its faces, and the package `latex` gives no face there.
So ivy sets the text of `\textsf` in the roman face.

The review of LaTeX of 2026-11-25 found this. `latex2e/tex/cfgguide.tex`
sets the names of its files in sans, such as `texsys.cfg` in its contents.
pdfLaTeX sets them in Latin Modern Sans, and ivy sets them in the roman
face.

## The facts

1. `tulmss.fd` of TeX Live names 14 files of Latin Modern Sans for the
   family `lmss`:
   - the medium upright shape at five optical sizes: `lmsans8-regular`,
     `lmsans9-regular`, `lmsans10-regular`, `lmsans12-regular` and
     `lmsans17-regular`;
   - the oblique shape at the same five sizes: `lmsans8-oblique` to
     `lmsans17-oblique`. The italic shape is the oblique shape;
   - the bold extended shape at one size: `lmsans10-bold` and
     `lmsans10-boldoblique`;
   - the semibold condensed shape at one size: `lmsansdemicond10-regular`
     and `lmsansdemicond10-oblique`.
2. Each file has about 95 KB. The 12 files of the first three items have
   about 1.1 MB.
3. The files are under the GUST Font License, as the 25 files of D65 are.
   The folder `fonts/lm` holds the license and the MANIFEST of Latin Modern.
4. The engine already chooses a file of Latin Modern by its optical size
   (`latex/internal/engine/ivy_lmoptical.go`). A table for the sans takes
   the sizes of `tulmss.fd`.

## The options

1. Add the 12 files of the medium, the oblique and the bold extended
   shapes. `\textsf`, `\textsf{\textit{}}` and `\textsf{\textbf{}}` then
   take the file of pdfLaTeX. The semibold condensed shape takes the bold
   extended shape.
2. Add the five files of the medium shape only. The oblique and the bold
   sans take the medium shape, as the engine does today for a shape that
   has no file.
3. Add `lmsans10-regular` only, scaled to each size.
4. Add no file. A program that wants sans gives the files in its tree of
   TeX files (`ivy.Config.TeX`), and the package `latex` takes them by
   their names when the tree holds them.

## The proposal

Option 1. Each option that adds files also does option 4: the package
`latex` finds the files of Latin Modern Sans by their names in
`Config.Fonts` or `Config.TeX`, as it finds the files of D65, and gives the
engine the face `Sans` with its table of optical sizes.

On 2026-11-25 the maintainers chose option 1, with option 4.

## The build

The build of 2026-11-25 adds the 12 files to `fonts/lm`, with 1201476
bytes. They come from TeX Live 2026, and `fonts/README.md` gives their
sizes and their SHA-256. The engine combines the family, the series and
the shape of a font switch when a switch goes into or out of the family
sans (`latex/internal/engine/ivy_fonts.go`). So `\textsf{\textit{x}}` takes
the oblique sans, and `\textsf{\textbf{x}}` takes the bold extended sans.
The other switches select one face each, as before. The roman family has
no bold italic face.
