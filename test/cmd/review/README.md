# review

`review` renders the documents of a corpus with ivy and with reference
programs. It writes a report in HTML that shows each page of ivy beside the
pages of the references, so that a maintainer can judge the rendering by
eye (D5, D13). The command is in the test module, because only that module
can run outside programs.

## Run it

Run the commands from the folder `test` of the repository. The command
finds the root of the repository from the current folder.

1. Download the files of the corpus, and the files of the page of
   markdown-it, into the cache:

   ```sh
   go run ./cmd/review fetch
   ```

2. Render the corpus, and write the report into a new folder:

   ```sh
   go run ./cmd/review render -out /path/to/report
   ```

3. Open `/path/to/report/index.html` in a browser.

A document of a kind without a paper, such as a text file, renders once on
each paper of `-papers`. See "The papers" below.

The folder of `-out` must be new, empty, or the folder of an earlier report.
The command refuses a folder that holds other files.

The reference of HTML is WeasyPrint 70.0, whose layout ivy ports (D15). If
`weasyprint` is not on the PATH, the command looks for it in a virtual
environment of Python in the folder `weasyprint` of the cache. Make that
environment once, with `uv` and Pango installed:

```sh
uv venv review/cache/weasyprint
```

```sh
uv pip install --python review/cache/weasyprint/bin/python weasyprint==70.0
```

## The flags

`fetch [-cache DIR] [-manifest FILE] [-j N]` downloads each file that is not
in the cache, and checks its size and its sha256. It tries a failed download
once more, goes on with the other files, and lists the failures at the end.
It also downloads the files of `test/review/markdownit.tsv`, the page of
the reference of Markdown, into the folder `markdownit` of the cache. See
"markdown-it".

`bundle -out DIR [-cache DIR] [-manifest FILE] [-match REGEXP] [-kind LIST]`
copies the files of the documents into the folder `DIR`, each at its name
in the manifest, for a program on another machine, such as Microsoft
Office on Windows. It takes the files of the cache and the files of the
repository. It also writes `DIR/passwords.tsv`: one line for each file with
a password, with the name, a tab and the password. It leaves out a file
that is not in the cache, and names it.

`render -out DIR [flags]` takes these flags:

| Flag | Default | What it does |
| --- | --- | --- |
| `-out DIR` | none | the folder of the report. It is required |
| `-cache DIR` | `test/review/cache` | the cache of the files and of the pages of the references |
| `-manifest FILE` | `test/review/corpus.tsv` | the manifest of the corpus |
| `-match REGEXP` | all | render only the documents whose name matches |
| `-kind LIST` | all | render only these kinds, such as `pdf,docx` |
| `-pages N` | 3 | render the first N pages and the last page. 0 renders each page |
| `-dpi N` | 96 | the resolution in dots per inch |
| `-j N` | half the CPUs | the number of documents at the same time |
| `-timeout DUR` | `5m` | the time that ivy can take on one document, and then the time that the references can take on it |
| `-retry` | off | try again each failure of the references that the cache keeps from an earlier run |
| `-papers LIST` | `letter,a4,a4-landscape` | the papers of a document whose file gives none. See "The papers" |
| `-system-fonts` | on | give ivy the fonts of the system. `-system-fonts=false` turns it off. See "The fonts" |
| `-font-dirs LIST` | the font folders of the account | the font folders more for ivy and for the references, separated by commas. An empty list gives none |
| `-faults FILE` | `test/review/reference-faults.tsv` | the known faults of the references. See "Faults of a reference" |
| `-tex TREE` | `base` | the tree of TeX files that ivy gives a TeX document: `base`, `dist` or `none`. See "TeX documents" |
| `-refs CHOICE` | `primary` | the references: `primary`, the primary oracle and at most one secondary oracle of each kind, or `all`, every reference. See "The oracles of a kind" |

## The manifest

The manifest is a file of tab-separated columns, with this header line:

```
name	kind	url	sha256	size	license	source	note
```

- `name` is a unique relative path. A downloaded file goes at this path in
  the folder `files` of the cache.
- `kind` is the format, such as `pdf`, `docx` or `epub`. It selects the
  package of ivy and the references.
- `url` is an https URL, or `repo:` and a path relative to the root of the
  repository.
- `sha256` and `size` are optional. If they are set, `fetch` and `render`
  compare them with the file.
- `note` can give the password of the file as `password=XYZ`.

An empty line and a line that starts with `#` are skipped.

## The references

ivy renders each document again on each run. The references convert some
kinds to PDF first, then draw each page. This table gives every reference,
as `-refs all` shows them. "The oracles of a kind" below says which ones
the report shows by default.

| Kinds | Conversion | Pages | ivy PDF |
| --- | --- | --- | --- |
| pdf, ai | none | MuPDF, Poppler Splash, Poppler Cairo, Chrome | no |
| Office, OpenDocument, rtf, Visio, Publisher, iWork | LibreOffice to PDF | MuPDF, Poppler Splash, Poppler Cairo, Chrome | yes |
| csv, tsv, tab | LibreOffice to PDF through UNO, on the page of ivy | MuPDF, Poppler Splash, Poppler Cairo, Chrome | yes |
| html, htm, xhtml | WeasyPrint to PDF | MuPDF, Poppler Splash, Poppler Cairo, Chrome | yes |
| tex, latex, ltx | pdfLaTeX, or pdfTeX for plain TeX, to PDF | MuPDF, Poppler Splash, Poppler Cairo, Chrome | yes |
| epub, mobi, azw3, azw4, fb2, chm, txt | calibre to PDF | MuPDF, Poppler Splash, Poppler Cairo, Chrome | yes |
| md, markdown | markdown-it in Chrome to PDF, then calibre to PDF | MuPDF, Poppler Splash, Poppler Cairo, Chrome | yes |
| xps, oxps | none | libgxps (`xpstopng`) | no |
| ps, eps | Ghostscript draws each page | Ghostscript | no |
| emf, wmf | none | Inkscape | no |
| djvu | none | DjVuLibre (`ddjvu`) | no |
| psd, ora, camera raw | none | ImageMagick | no |
| kra, cbz | none | no reference | no |

Some kinds also have an external reference, which a program outside this
machine makes, such as Microsoft Office. See "External references".

### The oracles of a kind

By default, `render` shows only the oracles of D69 for each document:
`ivy`, `ivy PDF` when the kind has that column, the primary oracle of the
kind, and at most one secondary oracle.

| Kinds | Primary oracle | Secondary oracle |
| --- | --- | --- |
| Word, Excel, PowerPoint, Visio, Publisher and RTF | Microsoft Office, when its PDF file is in the cache | LibreOffice. It is the primary oracle when the file has no PDF file of Office |
| OpenDocument, csv, tsv and tab | LibreOffice | none |
| Pages, Keynote and Numbers | Apple iWork, when its PDF file is in the cache | none. LibreOffice is the primary oracle when the file has no PDF file of iWork |
| pdf and ai | the PDF file itself | none |
| HTML | WeasyPrint | none |
| ebooks and text | calibre | none |
| Markdown | markdown-it in Chrome (D70) | none. calibre comes after it with `-refs all` |
| TeX | pdfLaTeX or pdfTeX | none |
| xps and oxps | Windows XPS, when its images are in the cache | libgxps. It is the primary oracle when the file has no images of Windows XPS |
| emf, wmf, emz and wmz | GDI+, when its image is in the cache | Inkscape. It is the primary oracle when the file has no image of GDI+ |
| ps and eps | Ghostscript | none |
| djvu | DjVuLibre | none |
| psd, ora and camera raw | ImageMagick | none |

MuPDF and Chrome draw the PDF file of the primary oracle. MuPDF alone draws
the PDF file of the secondary oracle. D69 keeps Chrome next to MuPDF until
the maintainers compare the two renderers. A PDF file of the kind `pdf` is
its own primary oracle, so MuPDF and Chrome draw the file itself. A row of
the MediaBox shows MuPDF alone.

`-refs all` shows every reference of the table above: each conversion,
each external reference, and each PDF file drawn by MuPDF, Poppler Splash,
Poppler Cairo and Chrome. The rows of the MediaBox then show MuPDF, Poppler
Splash and Poppler Cairo. The cache keeps the pages of every reference, so
a run with `-refs all` after a default run draws only the pages that are
not in the cache. The same holds the other way.

A column of a reference names the conversion, then the renderer, such as
"Microsoft Office - MuPDF" or "LibreOffice - Chrome". A reference without a
conversion, such as a file of the kind `pdf`, or a reference whose
renderer has the name of the conversion, such as GDI+, shows the renderer
alone. A row shows ivy, ivy PDF, then the columns of the primary oracle,
then the columns of the secondary oracle.

### Chrome

Chrome draws a page of a PDF file with PDFium, its PDF engine, as a person
sees the file in the browser. The command starts one Chrome process for a
run, at the first page that Chrome draws, and ends it at the end of the
run. It looks for `google-chrome-stable`, `google-chrome`, `chromium` or
`chromium-browser` on the PATH, in this order. It drives Chrome with the
DevTools protocol, through the package `github.com/chromedp/chromedp` (D69).

Chrome runs in its own headless mode, `--headless=new`. This mode shows a
PDF file in the viewer of Chrome. The old headless shell of Chrome does not
show a PDF file, and downloads it instead. So the command needs no virtual
display such as Xvfb, and no pdf.js.

For each page, the command opens a new tab. The tab gets the size of the
page, from `pdfinfo`, with a margin of 40 CSS pixels, and a scale of the
device of DPI/96. Then the tab opens the file with `#toolbar=0&zoom=100`.
At the zoom 100%, a point is 4/3 CSS pixels, so a point is DPI/72 pixels,
as with MuPDF. The tab gets its size before it opens the file, because the
viewer does not draw the page again when the size of the tab changes. The
command waits until the element `pdf-viewer` of the viewer has the pages of
the file, shows the page with `goToPage`, and asks the viewer for the place
of the page on the screen with `getPageScreenRect`, until the place stays
the same. The viewer can place a page at half a CSS pixel. The picture then
starts at the whole device pixel before that place, where Chrome draws the
page. It takes pictures of that part of the screen
every 0.2 seconds, until four pictures in a row are the same, because the
viewer draws a page in steps. The viewer shows its gray background in place
of a page that it did not draw yet, and a heavy page can take seconds
before its first step. So a picture of the gray background alone does not
count, for up to 20 seconds. The picture holds the page alone, without the
toolbar, the gray background or the shadow of the page.

Chrome rounds the size of a page to whole CSS pixels, and MuPDF rounds it
up to whole pixels. So the command gives the picture the size of the page
of MuPDF, from the CropBox and the rotation that `pdfinfo` gives. It drops
the pixels at the edges of the picture of Chrome, which can mix the page
with its shadow, and repeats the pixels next to them up to the size of
MuPDF. The pixels of a page differ from the pixels of MuPDF by less than 3
after the blur, as the test `TestChrome` makes sure.

If the viewer asks for a password, the command gives it the password of the
note of the manifest. The PDF file of a conversion has no password.

Chrome runs with the environment of the other programs, with the
configuration of fontconfig of "The fonts", and with `TMPDIR` in a new
folder of the temporary folder of the system. Chrome makes a socket in
`TMPDIR`, and the path of a socket must be short. The cache key of a page
of Chrome holds the tag `pdfium4` and the key of the program of Chrome.

### markdown-it

The reference of a Markdown file is markdown-it in Chrome (D70). It looks
as GitHub draws the file, on the pages of ivy. A page of HTML holds
markdown-it 15 with these plugins: the alerts of GitHub, the task lists,
the footnotes, the emoji, and @vscode/markdown-it-katex for the math. Shiki
gives the colors of code with the theme `github-light-default`, and KaTeX
draws the math. The style sheet is `github-markdown-light.css` of
github-markdown-css. `test/review/SOURCES.md` gives the versions and the
licenses.

`fetch` downloads the files of the page once, as the files of the corpus,
from the list `test/review/markdownit.tsv`. `render` makes sure that each
file is in the cache, with its size and its sha256, and then runs the page
without the network. If a file is not in the cache, each Markdown file has
no reference, and the error says to run `fetch`.

The command starts a server of HTTP on `127.0.0.1` for the run. It serves
the files of the list at their paths on jsDelivr, so that the imports of a
module find the other modules. It serves the page of each Markdown file at
`/page/ID/`, and the files of the folder of the Markdown file under that
path, but no file out of that folder. So a relative image of the file comes
from its folder. A rule of Content Security Policy in the page lets it load
files of the server alone.

The page holds the text of the file as a string of JSON. Its script does
these steps:

1. It removes the front matter of YAML at the start of the text, as the
   preview of VS Code and ivy do.
2. It reads the language of each block of code, and Shiki loads the
   grammar of each one. A block of an unknown language has no colors. A
   block of the language `math` is display math, as on GitHub.
3. markdown-it converts the text to HTML, with the HTML of the file. The
   page puts the HTML into `<article class="markdown-body">`.
4. An image of another site becomes its alternate text, as in ivy.
5. The page waits until each image is decoded and each font is loaded,
   and then sets `window.ivyState` to `done`. An error sets it to the
   error.

The command opens the page in a tab of the Chrome process of the run, and
asks for `window.ivyState` every 0.1 seconds. When the page is done, the
command prints it with `Page.printToPDF`, with `preferCSSPageSize` and the
backgrounds. The first style sheet of the page is `ebook.PaperStyleFor` of
the paper, which the package `markdown` of ivy gives its HTML too: the
size of `@page`, and margins of 0.75 inch. The body has no margin. A line
of a block of code wraps, as in ivy, because a page of paper cannot scroll
as a block on GitHub does. Chrome draws with the fonts that fontconfig
gives it, as in "The fonts". The style sheet of GitHub names Segoe UI for
the text and Consolas for code before Liberation and the generic families.
So the reference draws with them when a person copies the fonts of Windows
into `~/.fonts`, as on the machine of the review on 2026-11-25.

MuPDF and Chrome then draw the pages of the PDF file, as for the other
references of PDF. The name of the conversion in the cache holds a hash of
the style sheet of the paper, of the page and of `markdownit.tsv`, so a
change of one of them converts the files again.

ivy PDF is a column of the report that is not a reference. If LibreOffice,
WeasyPrint, calibre or markdown-it converts a document to PDF, the package
`pdf` of ivy renders that PDF file too, at the same resolution. The command compares
each page with the page of MuPDF. The difference shows the gaps of the PDF
renderer of ivy. A PDF file has no column ivy PDF, because the column
`ivy` already renders the file with the package `pdf`. ivy renders the
pages of ivy PDF again on each run. The PDF file comes from the cache.

These are the command lines, with `DPI` and the page `N`:

```sh
mutool draw -q -r DPI -A 8 -o out.png [-p PASSWORD] file.pdf N
pdftoppm -f N -l N -singlefile -r DPI -png -aa yes -aaVector yes -cropbox [-opw PASSWORD -upw PASSWORD] file.pdf out
pdftocairo -f N -l N -singlefile -r DPI -png -antialias gray -cropbox [-opw PASSWORD -upw PASSWORD] file.pdf out
soffice --headless --norestore --nologo --nodefault --nolockcheck -env:UserInstallation=file://PROFILE --convert-to pdf --outdir DIR file
IVY_REVIEW_PASSWORD=PASSWORD PYTHON unopdf.py SOFFICE file://PROFILE PIPE file out.pdf
ebook-convert file out.pdf OPTIONS
xpstopng -f N -l N -r DPI file.xps prefix
gs -dSAFER -dBATCH -dNOPAUSE -q -sDEVICE=png16m -rDPI -dTextAlphaBits=4 -dGraphicsAlphaBits=4 [-dEPSCrop] -sOutputFile=p-%04d.png file
pdflatex -interaction=nonstopmode -file-line-error -jobname=NAME '\AtBeginDocument{\pdfpagewidth=\paperwidth \pdfpageheight=\paperheight}\input{NAME.tex}'
pdftex -interaction=nonstopmode -file-line-error -jobname=NAME '\pdfpagewidth=8.5in \pdfpageheight=11in \input NAME.tex \end'
inkscape --export-type=png --export-dpi=DPI --export-background=white --export-background-opacity=1 -o out.png file
ddjvu -format=ppm -page=N -scale=DPI file.djvu out.ppm
magick 'file[0]' -colorspace sRGB -background white -alpha remove -alpha off png24:out.png
```

The options of `ebook-convert` give calibre the page and the font of ivy.
calibre converts a Markdown file only with `-refs all`.
An ebook takes margins of 0.6 inch at the top and the bottom and 0.5 inch
at the sides, and a serif font of 11 points. Its default page is 6 by 9
inches. A Markdown or CHM file takes margins of 0.75 inch and a font of 12
points. A text file takes margins of 0.75 inch and a monospace font of 10
points. Their default page is US Letter. On another paper, calibre gets the
size of the paper in points, with `--custom-size WxH --unit point`. The
layout of calibre still differs from the one of ivy, and calibre draws the
cover of an ebook as a page.

calibre gets two more options. `--preserve-cover-aspect-ratio` keeps the
shape of the cover of an ebook, as ivy does. calibre stretches the cover to
the page without it. A Markdown file gets `--markdown-extensions
footnotes,tables,toc,fenced_code,def_list,meta,sane_lists`. Without them,
calibre shows a block of fenced code as text with its backticks.

WeasyPrint gets the user style sheet that the package `html` of ivy gives
an HTML file on the same paper, `ebook.PaperStyleFor`: margins of 0.75
inch, and a serif font of 12 points. The default paper is US Letter. The
links of the file resolve from its folder in the cache, where the manifest
puts its assets.

A CSV file goes to LibreOffice through the script `unopdf.py`, because
the command line of LibreOffice cannot set a page. The script opens the
file with the CSV filter: commas for `.csv`, and tabs for `.tsv` and
`.tab`, in UTF-8. It gives each sheet the page of ivy: the paper, US
Letter by default, the margins of Excel (0.75 inch at the top and the
bottom, 0.7 inch at the sides), no header, no footer, and the grid. These
are the values of `sheet.DefaultPage` of ivy.

LibreOffice 26.8 can crash as it ends, after it wrote the whole PDF file,
as with some files of Excel 97, such as `poi/xls/legal-portrait.xls`. If
`soffice` fails and its PDF file exists, and `pdfinfo` reads the file, the
command keeps the PDF file.

With the anti-aliasing `best`, `pdftocairo` loses parts of the glyphs, so
the command uses `gray`.

A page of PDF has a MediaBox and a CropBox. The CropBox is the part of the
page that a viewer shows, and ivy renders it (D31). MuPDF renders the
CropBox by default. Poppler renders the MediaBox by default, so
`pdftoppm` and `pdftocairo` get `-cropbox`. Then each reference of a PDF
file, and of the PDF file of LibreOffice, calibre, WeasyPrint and
markdown-it, shows the CropBox, as ivy does.

Each program runs with `HOME` in a temporary folder of the cache, and with
the configuration of fontconfig of "The fonts". Each worker has its own
profile of LibreOffice and of calibre, in the folder `profiles` of the
cache. The end of the time of `-timeout` kills a program and each process
that it started. When a program ends, the command also kills each process
that it started and that still runs.

The name of a conversion in the cache holds a hash of its options, such as
the options of calibre, the style sheet of WeasyPrint or the page of a CSV
file. A change of the options converts the files again.

## The rows of the MediaBox

A file for a printer can draw marks in the margin between the CropBox and
the MediaBox. For a PDF file, of the kind `pdf` or `ai`, the command reads
the boxes of each selected page with `pdfinfo`. If the MediaBox of a page
differs from its CropBox, the page gets a second row, "MediaBox", that
shows the MediaBox:

```sh
pdfinfo -box -f N -l N [-opw PASSWORD -upw PASSWORD] file.pdf
mutool draw -q -r DPI -A 8 -b mediabox -o out.png [-p PASSWORD] file.pdf N
pdftoppm -f N -l N -singlefile -r DPI -png -aa yes -aaVector yes [-opw PASSWORD -upw PASSWORD] file.pdf out
pdftocairo -f N -l N -singlefile -r DPI -png -antialias gray [-opw PASSWORD -upw PASSWORD] file.pdf out
```

ivy renders the cell of this row with the option
`pdf.WithPageBox(pdf.MediaBox)` of D31, and with the configuration of the
row of the CropBox.

MuPDF 1.28 gives the image of `-b mediabox` the size of the MediaBox, but
it clips the drawing to the CropBox. Its image does not show the marks in
the margin. Poppler shows them.

## The fonts

LibreOffice, calibre, WeasyPrint and markdown-it in Chrome draw with the
fonts that fontconfig gives them. Microsoft Office draws with the fonts of
Windows. So ivy gets the fonts of the system too: the command gives ivy
`SystemFonts: true` of `ivy.Config`. `-system-fonts=false` turns it off.
ivy then has the fonts of the folder `fonts` of the repository and the Go
fonts only (D23).

ivy and the references see the same font folders:

- The folder `fonts` of the repository: ivy gets it as `Config.Fonts`, and
  the configuration of fontconfig adds it.
- The folders of the system, such as `/usr/share/fonts`: ivy reads them
  with `SystemFonts`, and fontconfig reads them from
  `/etc/fonts/fonts.conf`.
- The folders of `-font-dirs`. By default, these are `~/.fonts` and
  `~/.local/share/fonts` of the home folder of the account, from the
  database of the accounts, not from `HOME`. The command runs the
  references with `HOME` in a temporary folder, and it can itself run with
  another `HOME`, such as in a sandbox. Without these folders, neither ivy
  nor the references would see the fonts of the person, such as the fonts
  of Windows that a person copies into `~/.fonts`. ivy gets each folder in
  `Config.Fonts`, after the folder `fonts`, and the configuration of
  fontconfig adds each one as a `<dir>`.

The fonts of `Config.Fonts` come before the fonts of the system in ivy, and
fontconfig gives each folder the same rank. So a family that is both in a
folder of `-font-dirs` and in a folder of the system can differ.

The configuration of fontconfig, `profiles/fonts.conf` of the cache, also
makes Liberation Serif, Sans and Mono the generic families serif,
sans-serif and monospace, as in ivy (D28). Without it, fontconfig picks
Noto, whose larger glyphs give the references more pages. The names of the
conversions in the cache hold a hash of the configuration, so a change of
the font folders converts the files again.

To find the font that LibreOffice used, run `pdffonts` on its PDF file in
the cache. For example, LibreOffice draws the Japanese text of
`poi/ppt/chinese.ppt` with MS PGothic from `~/.fonts`, and ivy draws it
with the same face.

## The papers

A text file, a Markdown file, an HTML file, a CSV file and an ebook give no
paper (D32). ivy puts them on the paper of `Config.Paper`. The command
renders each such document once on each paper of `-papers`, and gives the
reference program the same paper. The kinds are those of calibre, of
markdown-it and of WeasyPrint, and csv, tsv and tab.

`-papers` is a list of letter, legal, a4 and a5, each with `-landscape` or
not, separated by commas. The paper `default` gives ivy the zero
`Config.Paper`, the default of the format, and the reference its old
default: 6 by 9 inches for an ebook, and US Letter for the others. An empty
list renders such a document once, on the default.

The report shows each paper as a document of its own, with the paper in
its name, such as `idpf/epub/childrens-literature.epub [A4 landscape]`.
`-match` matches the name in the manifest, without the paper. The key
`paper` of the document in `report.json` holds the paper.

The references get the paper in this way:

| Reference | Paper |
| --- | --- |
| calibre | `--custom-size WxH --unit point`, with the margins of the kind |
| WeasyPrint | the size of `@page` in the style sheet of `ebook.PaperStyleFor` |
| markdown-it | the size of `@page` in the style sheet of `ebook.PaperStyleFor` |
| LibreOffice, for csv, tsv and tab | the page style of each sheet, set by `unopdf.py` |

`pdfinfo` gives the sizes of the PDF files of the cache. calibre rounds a
size in points, such as 594.96 by 840.96 for A4. The difference is less
than one pixel at 96 dots per inch.

## TeX documents

The reference of a TeX document is the TeX distribution of the machine,
such as TeX Live. The report gives the version of `pdflatex` under the
programs, such as `pdfTeX 3.141592653-2.6-1.40.29 (TeX Live 2026/Arch
Linux)`.

A TeX document reads the files of its folder: its figures, the files of
`\input` and its bibliography. The manifest puts each such file in the
folder of the document, as an asset, and ivy reads the folder with
`latex.WithDir` (D65). The reference copies the folder into a temporary
folder of the cache, and runs the program there, so that the files of the
cache do not change. A document of the repository reads its folder in the
repository, such as `latex/testdata/report`.

A document with `\documentclass` or `\documentstyle` goes to `pdflatex`,
and another document goes to `pdftex`, the format of plain TeX. The program
runs twice, for the cross-references and the table of contents, and again
while its log asks for it, up to four times. It runs with
`SOURCE_DATE_EPOCH` of 2026-01-01 and `FORCE_SOURCE_DATE=1`, so that
`\today` does not change. The review runs no BibTeX: a document with a
bibliography holds its `.bbl` file in its folder, as a paper of arXiv does.

The input of the program sets the PDF page. pdfTeX takes the page of
`pdftexconfig.tex` of the distribution, such as A4, when a document loads
neither geometry nor hyperref. The class article sets the text on US
Letter, so the text sits on a page of another size. The input of
`pdflatex` gives the page the paper of the class, at the start of the
document. A package that sets the page, such as geometry, sets it again
after that. The input of `pdftex` gives the page US Letter, the paper of
`plain.tex`. The input of `pdftex` ends with `\end`, because a file of
plain TeX can end without `\bye`, as `story.tex` of the TeXbook does.

An error in the log of the program fails the reference, because a page
with an error is not a reference. The error names the first errors of the
log, such as a package that the distribution does not hold. The cache keeps
the failure. After you install the package, run `render` with `-retry`. On
2026-11-21 the TeX Live of the machine of the review does not hold pgf, so
the documents of beamer and of TikZ have no reference, and it has no
`repstopdf`, so a document with an EPS figure has no reference.

`-tex` gives ivy the tree of TeX files of `ivy.Config.TeX` (D65):

- `base`, the default, gives the standard classes and the packages that
  they load, as `kpsewhich` finds them, and Latin Modern of the folder
  `fonts/lm`. These are the files that the engine of go-tex/engine embeds
  upstream, and the files of `test/latex_test.go`. The engine sets a
  package that it implements itself, such as amsmath.
- `dist` gives the whole tree of the distribution, `TEXMFDIST` of
  `kpsewhich`. The engine then also reads a class such as `ltxguide.cls`,
  and a package such as the `amsmath.sty` of TeX Live.
- `none` gives no tree. The engine sets the text in the Go fonts.

`test/cmd/perf` takes the same flag.

## External references

Some references come from a program that does not run on this machine. A
maintainer runs the program elsewhere, and copies its files into a folder
of the cache. Each file has the name of the document in the manifest, with
an extension more. A reference of pages has one PNG image for each page,
with `.p<N>` before the extension, and N from 1:

| Reference | id | Folder of the cache | File | Kinds | Pages |
| --- | --- | --- | --- | --- | --- |
| Microsoft Office | `msoffice` | `msoffice/` | `<name>.pdf` | Word, Excel, PowerPoint, Visio, Publisher, RTF, OpenDocument, csv | MuPDF and Chrome, or with `-refs all` MuPDF, Poppler Splash, Poppler Cairo and Chrome |
| Windows XPS | `winxps` | `windows/xps/` | `<name>.p<N>.png` | xps, oxps | the images themselves |
| GDI+ | `gdiplus` | `windows/gdiplus/` | `<name>.png` | emf, wmf, emz, wmz | the image itself |
| Apple iWork | `iwork` | `iwork/` | `<name>.pdf` | pages, key, numbers | MuPDF and Chrome, or with `-refs all` MuPDF, Poppler Splash, Poppler Cairo and Chrome |

For example, the PDF file of Microsoft Office of `poi/docx/drawing.docx`
is `test/review/cache/msoffice/poi/docx/drawing.docx.pdf`, and the image of
Windows XPS of page 2 of `microsoft/xps/mb01.xps` is
`test/review/cache/windows/xps/microsoft/xps/mb01.xps.p2.png`. When the
file of a document exists, or the image of its first page, the document
gets the reference. The pages of a reference of pages are the images from
page 1 to the first page without an image. The images of its
cells start with the id, such as `p1-msoffice-mupdf.png`. The pages of the
file go into the cache with a hash of the file in their names, so a new
file draws them again. The cache is out of git, and so are these folders.

An external reference is the primary reference of some kinds: Microsoft
Office of the files of Word, Excel, PowerPoint, Visio and Publisher, and of
RTF, Windows XPS of XPS, GDI+ of the metafiles, and Apple iWork of Pages,
Keynote and Numbers. On each page, `worst` uses the first reference that
has a metric, the primary one first, drawn by its first renderer that is
not Chrome. That is MuPDF for a PDF file. The column "By reference" of the
table, the header of the document and the key `worst_by_reference` of
`report.json` still give the worst against each reference, such as
LibreOffice. For OpenDocument and csv, LibreOffice stays the primary
reference. Microsoft Office comes after it with `-refs all`, and the
default run does not show it.

A program outside this machine makes its file on its own paper. So a
document on a paper of `-papers` gets an external reference only on US
Letter or on the default paper.

The images of GDI+ and of Windows XPS have 96 dots per inch. At another
`-dpi`, their cells say so and have no image.

ivy shows only the preview that an iWork file holds. So Apple iWork
measures that preview against the real pages of the document.

### The programs of Windows

The scripts are in the folder `windows` of this command. They run in
Windows PowerShell 5.1 and later. Each one takes its input from its
parameters, writes a log and a TSV file of the results into its folder
`-Out`, asks nothing, and exits with 0 when it finished, even if some files
failed.

- `office2pdf.ps1 -Source DIR -Out DIR [-Passwords FILE] [-TimeoutSeconds N] [-Force]`
  exports each file of Office to PDF through COM: Word, Excel, PowerPoint,
  Visio and Publisher, by the extension of the file. Office 2024 has no
  Publisher, so the files of Publisher fail with "Class not registered",
  and keep LibreOffice. It writes `export.tsv` and `export.log`.
- `xps2png.ps1 -Source DIR -Out DIR [-Dpi N] [-TimeoutSeconds N] [-Force]`
  draws each page of each XPS file with WPF, the engine of the XPS viewer
  of Windows, to `<name>.p<N>.png`. It writes `xps2png.tsv`, with the
  file, the pages, the result, the seconds and the error, and
  `xps2png.log`.
- `metafile2png.ps1 -Source DIR -Out DIR [-Dpi N] [-TimeoutSeconds N] [-Force]`
  draws each metafile with GDI+, at the size of the page of ivy. It writes
  `metafile2png.tsv` and `metafile2png.log`.

`office2pdf.ps1` opens each file read only, without a window, with
`AutomationSecurity` 3 (`msoAutomationSecurityForceDisable`), which turns
the macros off, and without alerts. It gives each file its password from
`-Passwords`, and a wrong password to a file without one, so that a file
with an unknown password fails and does not ask for it. A watchdog kills
the application at the end of `-TimeoutSeconds`, 180 by default. After a
failure, the script closes the application, releases its objects of COM,
and starts it again for the next file. It skips a file whose PDF file
exists, unless `-Force` is given.

`xps2png.ps1` opens each file with `XpsDocument` of WPF, and takes each
page of the `DocumentPaginator` of its `FixedDocumentSequence`. It fills
the page with white, draws a `VisualBrush` of the visual of the page over
the size of the page, and renders that with `RenderTargetBitmap`. A side of
the image is the side of the page in units of 1/96 inch, times `-Dpi` / 96,
rounded up, as the side of a page of ivy. A child process of PowerShell in
a single-threaded apartment draws each file, with a time of
`-TimeoutSeconds`. The script needs no desktop, and it can run as SYSTEM.

"Microsoft Print to PDF" is not the reference of XPS, because it failed for
each job in the virtual machine of the review, also from WPF and on a
printer with a port to a file. WPF reads XPS, and not OpenXPS (ECMA-388),
the content of most `.oxps` files. The line of such a file in
`xps2png.tsv` says why WPF did not open it, and the file keeps libgxps. The
path of Windows that converts OpenXPS to XPS is the XPS Document API:
`IXpsOMObjectFactory1::CreatePackageFromFile1`, then
`IXpsOMPackage1::WriteToFile1` with `XPS_DOCUMENT_TYPE_XPS`. The script
does not hold that conversion yet.

To make the files of the external references of Windows:

1. On this machine, from the folder `test`, copy the files of the corpus
   into a new folder, with the passwords:

   ```sh
   go run ./cmd/review bundle -out /tmp/ivy-bundle -kind docx,dotx,docm,dotm,doc,dot,rtf,odt,ott,xlsx,xltx,xlsm,xltm,xls,xlt,ods,ots,csv,pptx,potx,ppsx,pptm,ppsm,ppt,pps,pot,odp,otp,vsdx,vsdm,vstx,vstm,vsd,vst,pub,xps,oxps,emf,wmf,emz,wmz
   ```

2. Copy the folder `/tmp/ivy-bundle` to the Windows machine, such as to
   `C:\ivy\files`, and the folder `windows` of this command, such as to
   `C:\ivy\scripts`.
3. On Windows, sign in as the person who runs Office, and run the scripts
   in a session of that person. COM of Office needs the desktop of the
   person.

   ```powershell
   powershell -NoProfile -ExecutionPolicy Bypass -File C:\ivy\scripts\office2pdf.ps1 -Source C:\ivy\files -Out C:\ivy\out\msoffice -Passwords C:\ivy\files\passwords.tsv
   powershell -Sta -NoProfile -ExecutionPolicy Bypass -File C:\ivy\scripts\xps2png.ps1 -Source C:\ivy\files -Out C:\ivy\out\xps
   powershell -NoProfile -ExecutionPolicy Bypass -File C:\ivy\scripts\metafile2png.ps1 -Source C:\ivy\files -Out C:\ivy\out\gdiplus
   ```

4. Copy the folders back into the cache, with their relative paths:
   `C:\ivy\out\msoffice` into `test/review/cache/msoffice`,
   `C:\ivy\out\xps` into `test/review/cache/windows/xps`, and
   `C:\ivy\out\gdiplus` into `test/review/cache/windows/gdiplus`. The
   files of the logs and of the results can come too. The command reads
   only the files with the extension of the table.
5. Run `render` again.

### A test plan for the scripts

Nobody ran the scripts before they were written, so a maintainer tests
them once:

1. Run `office2pdf.ps1` on a folder with one small file of each
   application: a docx, an xlsx, a pptx and a vsdx. Make sure that
   `export.tsv` says `ok` for each one, that each PDF file opens, and that
   no window of Office showed. Make sure that no process of Office runs at
   the end, in the Task Manager.
2. Add a docx and an xlsx with a password, and their lines in a
   `passwords.tsv`. Make sure that they export. Remove a line, run with
   `-Force`, and make sure that the file fails at once and asks nothing.
3. Add a file that is not a document, such as a text file renamed to
   `.docx`, and a docm with a macro that runs on open. Make sure that the
   first fails, that the macro does not run, and that the next file still
   exports.
4. Run with `-TimeoutSeconds 1` on a large file. Make sure that its line
   says `timeout`, that the application is killed, and that the next file
   exports.
5. Run again without `-Force`. Make sure that each line says `skipped`.
6. Run `office2pdf.ps1` on a pub file. On Office 2024 the line says that
   Publisher did not start.
7. Run `xps2png.ps1` on `repo/xps/example.xps`, `microsoft/xps/mb01.xps`
   and `microsoft/xps/mb01.oxps`. Make sure that the image of
   `example.xps` is 794 by 1123 pixels, that each page of `mb01.xps` has
   an image, and that the line of `mb01.oxps` gives the error of WPF.
8. Run `metafile2png.ps1` on two EMF files, a WMF file and an EMZ file of
   the corpus. Make sure that the sizes in `metafile2png.tsv` are the sizes
   of the images of ivy in a report at 96 dots per inch.
9. Run each script from a scheduled task as the person, without a console,
   as the host script does. Make sure that it ends with the code 0.
10. Copy the files into the cache, run `render -kind docx,xlsx,pptx,vsdx,xps,emf`,
    and make sure that the report shows the cells of the external
    references and that Microsoft Office is the primary reference.

### Apple iWork on macOS

A maintainer exports each Pages, Keynote and Numbers file to PDF on macOS,
with the applications of Apple, and copies the PDF files into
`test/review/cache/iwork`, as `<name>.pdf`. Pages, Keynote and Numbers
open only the files of 2013 and later.

## Faults of a reference

A reference can be wrong on a page, and ivy right. The file
`test/review/reference-faults.tsv` lists such cases. Its columns are
separated by tabs: the name of the document in the manifest, or that name
with the paper, the page from 1 or `*` for each page, the id of the
reference or `*` for each one, and a note. The ids are `soffice`,
`msoffice`, `calibre`, `markdownit`, `weasyprint`, `pdflatex`, `pdf`, `xps`, `winxps`,
`gs`, `inkscape`, `gdiplus`, `djvu`, `magick` and `iwork`. A line that starts
with `#` is a comment.

The report gives each cell of such a reference a dashed border and the
note. The cell keeps its numbers, but `worst`, "By reference", the primary
reference of the page and "references disagree" leave it out. A document
whose references are all faults on a page has no `worst` for the page.
The key `fault` of the cell in `report.json` holds the note.

Add a case when the file itself and its specification show that the
reference is wrong, with the evidence in the note.

## A page of another size

The command compares the size of each page of ivy with the size of the
page of the primary reference, in pixels at `-dpi`. If a side differs by
more than one pixel, the page of ivy gets a red border, the row says "the
size differs", and the key `size_differs` of the row in `report.json` is
`true`. The table of the kinds at the top of the report counts these pages
for each kind, and the column "Size differs" of the table of the documents
counts them for each document.

## Files with a password

A PDF file with a password takes the password of the note. MuPDF and
Poppler read it from their command lines.

The command line of LibreOffice cannot take a password. For a kind of
LibreOffice, the command runs the script `test/review/unopdf.py` instead.
The script starts `soffice` in the profile of the worker, opens the file
through the UNO API of LibreOffice with the password, and stores it as PDF.
The filter of the PDF file comes from the type of the document:
`writer_pdf_Export`, `calc_pdf_Export`, `impress_pdf_Export` or
`draw_pdf_Export`. The script gets the password in the environment
variable `IVY_REVIEW_PASSWORD`, so that the list of processes does not show
it.

The script needs a Python that can import the module `uno` of LibreOffice.
The command tries these programs, in this order:

1. `python` in the folder of `soffice.bin`, which some packages of
   LibreOffice bundle.
2. `python3` of the PATH.
3. `python3` of the PATH, with the folder of `soffice.bin` in its
   `PYTHONPATH`.

If no Python can import `uno`, install the Python bridge of LibreOffice,
such as the package `python3-uno` of Debian. Without it, a file with a
password of a kind of LibreOffice has no reference, and the error says why.
If the password is wrong, or if LibreOffice cannot read the encryption, the
error says "LibreOffice did not open the file with the password".

LibreOffice 26.8 opens each file with a password of the corpus: the RC4
CryptoAPI encryption of `.doc`, the XOR obfuscation of `.xls`, the Standard
and Agile encryption of `.docx` and `.xlsx`, and the Blowfish and the
Argon2id encryption of `.odt`.

calibre cannot open a file with a password, so an ebook with a password
has no reference.

## The cache

The cache holds these folders:

- `files/` holds the files that `fetch` downloads.
- `markdownit/files/` holds the files of the page of markdown-it that
  `fetch` downloads, at their paths on jsDelivr.
- `refs/<hash>/` holds the conversions and the pages of the references of a
  file. The hash is the start of the sha256 of the file. Each name holds a
  key of the program, which changes when the program changes, and the
  resolution. The name of a file with a password also holds a hash of the
  password.
- `profiles/` holds the profiles of LibreOffice and of calibre, and the
  configuration of fontconfig.
- `msoffice/`, `windows/xps/`, `windows/gdiplus/` and `iwork/` hold the
  files of the external references, which a maintainer copies there. See
  "External references".
- `tmp/` holds a folder for each run that has not ended: the `HOME` and the
  `TMPDIR` of the programs, and the temporary files of the conversions. A
  result moves from it into `refs/` with a rename on one file system, so
  the cache never holds a part of a result. The run deletes its folder at
  its end. If a run was killed, delete its folder when no run is active.

If a reference program fails, the cache keeps the failure, because
LibreOffice and calibre are slow. A failed conversion stays as `error.txt`
in the folder of the conversion, and a failed page stays as a file `.err`
beside its image. The cache keeps only an error of the program: it failed,
it did not finish in time, or it wrote no result. It does not keep another
error, such as an error of the file system, because the next run can
succeed.

If a failure comes from the cache, the error says so. The command also
counts these failures at its end, in its output and at the top of the
report. To try them again, run `render` again with `-retry`. To try one
conversion again, delete its folder.

An earlier version of the command made its temporary folders in `TMPDIR`.
If `TMPDIR` and the cache were on different file systems, LibreOffice
conversions failed with "invalid cross-device link", and the cache kept the
failure. Run `render` with `-retry` once to try these files again.

## The report

The folder of `-out` holds these files:

- `index.html` is the report. It needs no network.
- `report.json` holds the numbers of the report, so that a later run can
  compare them.
- `d/<id>/` holds the images of each document: `p1-ivy.png`, the image of
  ivy PDF `p1-ivypdf.png`, and the image of each reference, such as
  `p1-mupdf.png`, at the full resolution. It also holds a JPEG file of each
  one at 480 pixels wide, a map of the difference of ivy from each
  reference, such as `p1-mupdf-diff.png`, and a map of the difference of
  ivy PDF from MuPDF, `p1-ivypdf-diff.png`. The images of a row of the
  MediaBox start with `p1-media-`, such as `p1-media-mupdf.png`.

The top of the report gives the run: the counts of the statuses, the
programs, the time and the kinds. Under it, a tab for each group of kinds
holds the documents of the group: PDF, Word, Excel, PowerPoint,
OpenDocument, Other Office (Visio, Publisher, iWork, RTF and CSV), Page
descriptions (XPS, PostScript, EPS, the metafiles and DjVu), Ebooks and
text, HTML, LaTeX, Images, and Other for a kind of no group. The tab "All"
comes first and shows every group. A tab without documents is not there.
Each tab gives the number of its documents, and the number of the
documents that ivy did not render, such as "Word 28, 2 failed". The status
of such a document is not `ok` and not `partial`. The hash of the URL keeps
the tab, such as `#tab=word`, so that a reload or a link opens the same
tab. Without JavaScript, the report shows every group, one after the
other.

Each group starts with its name and a summary: the number of its
documents, of the failures, and the mean `worst` of its documents that
have one. Then comes a table of its documents, from the worst. A click on
a heading sorts the table, and the fields above the tabs filter the
documents by kind, by status and by name. Under the table, each document
has a section: its header, the errors and the log of ivy, and a row for
each page. A row holds the image of ivy, the image of ivy PDF if the kind
has that column, and the images of the references, in this order: ivy, ivy
PDF, the primary oracle, and the secondary oracle. With `-refs all`, the
other references come after them. The caption of a cell names the column of
the reference, such as "Microsoft Office - MuPDF", and the header of a row
names its primary reference. The header of a document gives the worst
against each column, as "worst by column", when it has more than one
column. The box "show the differences", or the button of a row, shows
the map of the difference in place of each reference and of ivy PDF: red
where the pages differ, darker where they differ more.

A click on an image opens the gallery of its document. The gallery holds
the images of that document only, in the order of the rows: for each page,
ivy, ivy PDF and each reference. These keys and controls work in the
gallery:

- The left and right arrows, the buttons at the sides, or a swipe show the
  image before or after. Home and End show the first and the last image.
- The box "differences", or the key `d`, shows the map of the difference
  in place of each image that has one. The choice stays as you go to the
  next image. The box starts checked when you click a map of the
  difference, or when the page shows the differences.
- Esc, the button ✕ or a click outside the image closes the gallery.

The top of the gallery gives the name and the status of the document, the
page and the column of the image, and its place in the gallery, such as
"3 / 29". Under the image are the numbers of the cell, then the summary of
the document: its source, its license, its URL, the times, the worst by
reference and the errors. "open the file" opens the image at its full size.
A click with Ctrl, Cmd or Shift opens the image without the gallery.

A row of the MediaBox has the same cells and the same numbers as the other
rows, at the MediaBox. Its header gives the blurred difference between its
references. `worst`, the column "Refs disagree" and the warning
"references disagree" leave these rows out. The header of the document
gives their worst as "MediaBox: worst".

These are the numbers under each image of a reference:

- `blurred` is the mean difference of the grey levels after a blur of 5 by
  5 pixels, from 0 to 255. The blur hides the differences of the
  anti-aliasing. Under 2 is green, and under 5 is amber.
- `mean` is the mean difference of the channels, without the blur.
- `scaled` says that the page of ivy has another size, and that the command
  scaled it to the size of the reference first. It is red when a side
  differs by more than 2 percent.

`worst` in the table is, for each page, the blurred difference from the
cell of the primary reference drawn by its first renderer that is not
Chrome, and the largest of these over the pages. For a PDF file, that
renderer is MuPDF. Before D69, `worst` took the closest of the cells of
MuPDF, Poppler Splash and Poppler Cairo of the primary reference. So a new
number is never smaller than the old one. On 2026-11-25, on the kinds pdf
and docx, 64 of 78 documents changed by less than 0.5. A PDF file that
MuPDF draws in another way than Poppler changed more, such as
`repo/pdf/pattern-shading-type6-2.pdf`, from 0.59 to 29.93, and
`pdfjs/pdf/forms.pdf`, from 1.93 to 11.57. A row says "references disagree" when two cells of the references
differ by more than 4 after the blur, such as Microsoft Office and
LibreOffice. On such a page, the references themselves do not agree.

Under the image of ivy PDF, `vs MuPDF` gives the blurred and the mean
difference of ivy PDF from MuPDF on the same page. The column "ivy PDF vs
MuPDF" of the table gives the largest of these over the pages. A sort on
that column shows where the PDF renderer of ivy has gaps. ivy PDF is not a
reference, so `worst` and "references disagree" do not count it.

The status of a document is `ok`, `partial` (ivy drew a page only in part),
`error`, `panic` (ivy panicked, which is a bug), `timeout`, `missing` (the
file is not in the cache) or `unsupported` (ivy has no package for the
kind). The status comes from the column `ivy` only. An error, a panic or a
partial page of ivy PDF shows under its image, and the header of the document
gives the first one as "ivy PDF: " and the error.

`report.json` holds these keys for ivy PDF. The other keys do not change.

| Key | What it holds |
| --- | --- |
| `documents[].ivy_pdf_worst` | the largest blurred difference of ivy PDF from MuPDF over the pages, or -1 without a metric |
| `documents[].ivy_pdf_status` | the worst status of ivy PDF over the pages. It is not there if the kind has no column ivy PDF, or if the references made no PDF file |
| `documents[].ivy_pdf_error` | the error of ivy PDF on the PDF file, or on its first page that failed |
| `documents[].ivy_pdf_log` | the messages of the log of ivy PDF |
| `documents[].rows[].ivy_pdf` | the image of ivy PDF on the page, with the keys of `ivy`. Its `metric` is the difference from MuPDF. It is not there for a kind without the column |

`report.json` holds these keys for the papers, the references, the faults
and the sizes:

| Key | What it holds |
| --- | --- |
| `papers` | the papers of `-papers`, each with `id`, `name`, `width` and `height` in points |
| `system_fonts`, `font_dirs` | the fonts of the system for ivy, and the font folders more |
| `kinds[]` | for each kind: `kind`, `documents`, `pages` and `size_differs`, the pages of another size |
| `documents[].paper` | the paper of a document without a paper. Its `name` also ends with the paper |
| `documents[].external_references[]` | each external reference of the document: `id`, `name`, `file` in the cache, `pages`, and `error` |
| `refs` | `primary` or `all`, the choice of `-refs` |
| `documents[].worst_by_reference` | the worst against each reference, by its id, such as `{"msoffice": 1.2, "soffice": 3.4}`. It comes from the first renderer that is not Chrome |
| `documents[].worst_by_renderer` | the worst against each column, by the id of the reference and the id of the renderer, such as `{"msoffice/mupdf": 1.2, "msoffice/chrome": 1.5}` |
| `documents[].primary_reference`, `documents[].primary_renderer` | the ids of the primary oracle of the document and of the renderer that `worst` uses, such as `msoffice` and `mupdf`. A page where the primary oracle has no metric uses the next reference, as `rows[].primary_reference` says |
| `documents[].size_differs_pages` | the number of pages of another size |
| `documents[].rows[].primary_reference`, `documents[].rows[].primary_renderer` | the ids of the reference and of the renderer that `worst` uses on the page |
| `documents[].rows[].size_differs` | `true` for a page of another size |
| `documents[].rows[].references[].reference` | the id of the reference of the cell, such as `soffice` |
| `documents[].rows[].references[].reference_name` | the name of the conversion of the reference of the cell, such as `LibreOffice`. It is not there for a file of the kind `pdf` |
| `documents[].rows[].references[].renderer` | the id of the renderer of the cell, such as `mupdf` or `chrome` |
| `documents[].rows[].references[].fault` | the note of a known fault of the reference on the page |

`report.json` holds these keys for the rows of the MediaBox:

| Key | What it holds |
| --- | --- |
| `documents[].rows[].box` | `media` for a row of the MediaBox. A row of the CropBox, the default view, has no `box` |
| `documents[].mediabox_worst` | the worst of the rows of the MediaBox, as `worst` is of the other rows, or -1 without a metric. It is not there if the document has no row of the MediaBox |

## Add a format

To add a format of ivy, such as DjVu, add its `Format` value to the list
`formats` in `test/review/ivy.go`. A format that reads the files of its
folder, such as HTML, Markdown and LaTeX, also gets the folder in
`OpenIvy` of that file. The kinds of the manifest are the
extensions of the format. To add a reference, add a route or a renderer in
`test/review/refs.go`.
