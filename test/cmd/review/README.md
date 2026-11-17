# review

`review` renders the documents of a corpus with ivy and with reference
programs. It writes a report in HTML that shows each page of ivy beside the
pages of the references, so that a maintainer can judge the rendering by
eye (D5, D13). The command is in the test module, because only that module
can run outside programs.

## Run it

Run the commands from the folder `test` of the repository. The command
finds the root of the repository from the current folder.

1. Download the files of the corpus into the cache:

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
kinds to PDF first, then draw each page:

| Kinds | Conversion | Pages | ivy PDF |
| --- | --- | --- | --- |
| pdf, ai | none | MuPDF, Poppler Splash, Poppler Cairo | no |
| Office, OpenDocument, rtf, Visio, Publisher, iWork | LibreOffice to PDF | MuPDF, Poppler Splash, Poppler Cairo | yes |
| csv, tsv, tab | LibreOffice to PDF through UNO, on the page of ivy | MuPDF, Poppler Splash, Poppler Cairo | yes |
| html, htm, xhtml | WeasyPrint to PDF | MuPDF, Poppler Splash, Poppler Cairo | yes |
| epub, mobi, azw3, azw4, fb2, chm, md, txt | calibre to PDF | MuPDF, Poppler Splash, Poppler Cairo | yes |
| xps, oxps | none | libgxps (`xpstopng`) | no |
| ps, eps | Ghostscript draws each page | Ghostscript | no |
| emf, wmf | none | Inkscape | no |
| djvu | none | DjVuLibre (`ddjvu`) | no |
| psd, ora, camera raw | none | ImageMagick | no |
| kra, cbz | none | no reference | no |

Some kinds also have an external reference, which a program outside this
machine makes, such as Microsoft Office. See "External references".

ivy PDF is a column of the report that is not a reference. If LibreOffice,
WeasyPrint or calibre converts a document to PDF, the package `pdf` of ivy
renders that PDF file too, at the same resolution. The command compares
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
inkscape --export-type=png --export-dpi=DPI --export-background=white --export-background-opacity=1 -o out.png file
ddjvu -format=ppm -page=N -scale=DPI file.djvu out.ppm
magick 'file[0]' -colorspace sRGB -background white -alpha remove -alpha off png24:out.png
```

The options of `ebook-convert` give calibre the page and the font of ivy.
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
file, and of the PDF file of LibreOffice, calibre and WeasyPrint, shows
the CropBox, as ivy does.

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

The cell of ivy in this row needs the option `pdf.WithPageBox` of D31.
Until the package `pdf` has it, the cell has the status `unsupported` and
the text "ivy cannot render the MediaBox yet (D31)".

MuPDF 1.28 gives the image of `-b mediabox` the size of the MediaBox, but
it clips the drawing to the CropBox. Its image does not show the marks in
the margin. Poppler shows them.

## The fonts

LibreOffice, calibre and WeasyPrint draw with the fonts that fontconfig
gives them. Microsoft Office draws with the fonts of Windows. So ivy gets
the fonts of the system too: the command gives ivy `SystemFonts: true` of
`ivy.Config`. `-system-fonts=false` turns it off. ivy then has the fonts of
the folder `fonts` of the repository and the Go fonts only (D23).

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
reference program the same paper. The kinds are those of calibre and of
WeasyPrint, and csv, tsv and tab.

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
| LibreOffice, for csv, tsv and tab | the page style of each sheet, set by `unopdf.py` |

`pdfinfo` gives the sizes of the PDF files of the cache. calibre rounds a
size in points, such as 594.96 by 840.96 for A4. The difference is less
than one pixel at 96 dots per inch.

## External references

Some references come from a program that does not run on this machine. A
maintainer runs the program elsewhere, and copies its files into a folder
of the cache. Each file has the name of the document in the manifest, with
an extension more. A reference of pages has one PNG image for each page,
with `.p<N>` before the extension, and N from 1:

| Reference | id | Folder of the cache | File | Kinds | Pages |
| --- | --- | --- | --- | --- | --- |
| Microsoft Office | `msoffice` | `msoffice/` | `<name>.pdf` | Word, Excel, PowerPoint, Visio, Publisher, RTF, OpenDocument, csv | MuPDF, Poppler Splash |
| Windows XPS | `winxps` | `windows/xps/` | `<name>.p<N>.png` | xps, oxps | the images themselves |
| GDI+ | `gdiplus` | `windows/gdiplus/` | `<name>.png` | emf, wmf, emz, wmz | the image itself |
| Apple iWork | `iwork` | `iwork/` | `<name>.pdf` | pages, key, numbers | MuPDF |

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
has a metric, the primary one first. The column "By reference" of the
table, the header of the document and the key `worst_by_reference` of
`report.json` still give the worst against each reference, such as
LibreOffice. For OpenDocument and csv, LibreOffice stays the primary
reference, and Microsoft Office comes after it.

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
`msoffice`, `calibre`, `weasyprint`, `pdf`, `xps`, `winxps`, `gs`,
`inkscape`, `gdiplus`, `djvu`, `magick` and `iwork`. A line that starts
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

The top of the report has a table of the documents, from the worst. A click
on a heading sorts the table, and the fields above it filter the documents
by kind, by status and by name. Under the table, each document has a
section: its header, the errors and the log of ivy, and a row for each page.
A row holds the image of ivy, the image of ivy PDF if the kind has that
column, and the images of the references, in this order: ivy, ivy PDF,
the primary external references, MuPDF, Poppler Splash, Poppler Cairo, and
the other external references. When a document has more than one
reference, the caption of a cell names the reference too, such as
"Microsoft Office · MuPDF", and the header of a row names its primary
reference. A click on an image
opens it at its full size. The box "show the differences", or the button of
a row, shows the map of the difference in place of each reference and of ivy
PDF: red where the pages differ, darker where they differ more.

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
closest cell of the primary reference, and the largest of these over the
pages. A row says "references disagree" when two cells of the references
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
| `documents[].worst_by_reference` | the worst against each reference, by its id, such as `{"msoffice": 1.2, "soffice": 3.4}` |
| `documents[].size_differs_pages` | the number of pages of another size |
| `documents[].rows[].primary_reference` | the id of the reference that `worst` uses on the page |
| `documents[].rows[].size_differs` | `true` for a page of another size |
| `documents[].rows[].references[].reference` | the id of the reference of the cell, such as `soffice` |
| `documents[].rows[].references[].reference_name` | the name of the reference of the cell, when the document has more than one reference |
| `documents[].rows[].references[].fault` | the note of a known fault of the reference on the page |

`report.json` holds these keys for the rows of the MediaBox:

| Key | What it holds |
| --- | --- |
| `documents[].rows[].box` | `media` for a row of the MediaBox. A row of the CropBox, the default view, has no `box` |
| `documents[].mediabox_worst` | the worst of the rows of the MediaBox, as `worst` is of the other rows, or -1 without a metric. It is not there if the document has no row of the MediaBox |

## Add a format

To add a format of ivy, such as DjVu, add its `Format` value to the list
`formats` in `test/review/ivy.go`. The kinds of the manifest are the
extensions of the format. To add a reference, add a route or a renderer in
`test/review/refs.go`.
