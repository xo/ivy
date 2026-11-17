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
| Office, OpenDocument, rtf, csv, Visio, Publisher, iWork | LibreOffice to PDF | MuPDF, Poppler Splash, Poppler Cairo | yes |
| html, htm, xhtml | WeasyPrint to PDF | MuPDF, Poppler Splash, Poppler Cairo | yes |
| epub, mobi, azw3, azw4, fb2, chm, md, txt | calibre to PDF | MuPDF, Poppler Splash, Poppler Cairo | yes |
| xps, oxps | none | libgxps (`xpstopng`) | no |
| ps, eps | Ghostscript draws each page | Ghostscript | no |
| emf, wmf | none | Inkscape | no |
| djvu | none | DjVuLibre (`ddjvu`) | no |
| psd, ora, camera raw | none | ImageMagick | no |
| kra, cbz | none | no reference | no |

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
pdftoppm -f N -l N -singlefile -r DPI -png -aa yes -aaVector yes [-opw PASSWORD -upw PASSWORD] file.pdf out
pdftocairo -f N -l N -singlefile -r DPI -png -antialias gray [-opw PASSWORD -upw PASSWORD] file.pdf out
soffice --headless --norestore --nologo --nodefault --nolockcheck -env:UserInstallation=file://PROFILE --convert-to pdf --outdir DIR file
ebook-convert file out.pdf OPTIONS
xpstopng -f N -l N -r DPI file.xps prefix
gs -dSAFER -dBATCH -dNOPAUSE -q -sDEVICE=png16m -rDPI -dTextAlphaBits=4 -dGraphicsAlphaBits=4 [-dEPSCrop] -sOutputFile=p-%04d.png file
inkscape --export-type=png --export-dpi=DPI --export-background=white --export-background-opacity=1 -o out.png file
ddjvu -format=ppm -page=N -scale=DPI file.djvu out.ppm
magick 'file[0]' -background white -alpha remove -alpha off png24:out.png
```

The options of `ebook-convert` give calibre the page and the font of ivy.
An ebook takes a page of 6 by 9 inches and a serif font of 11 points. An
Markdown or CHM file takes US Letter and a font of 12 points. A text
file takes US Letter and a monospace font of 10 points. The layout of
calibre still differs from the one of ivy, and calibre draws the cover of
an ebook as a page.

WeasyPrint gets the user style sheet that the package `html` of ivy gives
an HTML file, `ebook.PaperStyle`: US Letter, margins of 0.75 inch, and a
serif font of 12 points. The links of the file resolve from its folder in
the cache, where the manifest puts its assets.

With the anti-aliasing `best`, `pdftocairo` loses parts of the glyphs, so
the command uses `gray`.

Each program runs with `HOME` in a temporary folder, and with a
configuration of fontconfig that adds the folder `fonts` of the
repository, so that LibreOffice, calibre and WeasyPrint have the fonts of
ivy. The configuration also makes Liberation Serif, Sans and Mono the
generic families serif, sans-serif and monospace, as in ivy (D28). Without
it, fontconfig picks Noto, whose larger glyphs give the references more
pages. The names of the conversions in the cache hold a hash of the
configuration, so a change of it converts the files again. Each
worker has its own profile of LibreOffice and of calibre, in the folder
`profiles` of the cache. The end of the time of `-timeout` kills a program
and each process that it started.

LibreOffice and calibre cannot open a file with a password from the command
line, so such a file has no reference. A PDF file with a password takes the
password of the note.

## The cache

The cache holds these folders:

- `files/` holds the files that `fetch` downloads.
- `refs/<hash>/` holds the conversions and the pages of the references of a
  file. The hash is the start of the sha256 of the file. Each name holds a
  key of the program, which changes when the program changes, and the
  resolution.
- `profiles/` holds the profiles of LibreOffice and of calibre, and the
  configuration of fontconfig.

A failed conversion stays in the cache as `error.txt`, because LibreOffice
and calibre are slow. Delete the folder of the conversion to try again.

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
  ivy PDF from MuPDF, `p1-ivypdf-diff.png`.

The top of the report has a table of the documents, from the worst. A click
on a heading sorts the table, and the fields above it filter the documents
by kind, by status and by name. Under the table, each document has a
section: its header, the errors and the log of ivy, and a row for each page.
A row holds the image of ivy, the image of ivy PDF if the kind has that
column, and the images of the references, in this order: ivy, ivy PDF,
MuPDF, Poppler Splash, Poppler Cairo. A click on an image
opens it at its full size. The box "show the differences", or the button of
a row, shows the map of the difference in place of each reference and of ivy
PDF: red where the pages differ, darker where they differ more.

These are the numbers under each image of a reference:

- `blurred` is the mean difference of the grey levels after a blur of 5 by
  5 pixels, from 0 to 255. The blur hides the differences of the
  anti-aliasing. Under 2 is green, and under 5 is amber.
- `mean` is the mean difference of the channels, without the blur.
- `scaled` says that the page of ivy has another size, and that the command
  scaled it to the size of the reference first. It is red when a side
  differs by more than 2 percent.

`worst` in the table is, for each page, the blurred difference from the
closest reference, and the largest of these over the pages. A row says
"references disagree" when two references differ by more than 4 after the
blur. On such a page, the references themselves do not agree.

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

## Add a format

To add a format of ivy, such as DjVu, add its `Format` value to the list
`formats` in `test/review/ivy.go`. The kinds of the manifest are the
extensions of the format. To add a reference, add a route or a renderer in
`test/review/refs.go`.
