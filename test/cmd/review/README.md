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

| Kinds | Conversion | Pages |
| --- | --- | --- |
| pdf, ai | none | MuPDF, Poppler Splash, Poppler Cairo |
| Office, OpenDocument, rtf, csv, Visio, Publisher, iWork | LibreOffice to PDF | MuPDF, Poppler Splash, Poppler Cairo |
| epub, mobi, azw3, azw4, fb2, chm, html, md, txt | calibre to PDF | MuPDF, Poppler Splash, Poppler Cairo |
| xps, oxps | none | libgxps (`xpstopng`) |
| ps, eps | Ghostscript draws each page | Ghostscript |
| emf, wmf | none | Inkscape |
| djvu | none | DjVuLibre (`ddjvu`) |
| psd, ora, camera raw | none | ImageMagick |
| kra, cbz | none | no reference |

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
HTML, Markdown or CHM file takes US Letter and a font of 12 points. A text
file takes US Letter and a monospace font of 10 points. The layout of
calibre still differs from the one of ivy, and calibre draws the cover of
an ebook as a page.

With the anti-aliasing `best`, `pdftocairo` loses parts of the glyphs, so
the command uses `gray`.

Each program runs with `HOME` in a temporary folder, and with a
configuration of fontconfig that adds the folder `fonts` of the
repository, so that LibreOffice and calibre have the fonts of ivy. Each
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
- `d/<id>/` holds the images of each document: `p1-ivy.png` and the image of
  each reference, such as `p1-mupdf.png`, at the full resolution, a JPEG
  file of each one at 480 pixels wide, and a map of the difference from
  each reference, such as `p1-mupdf-diff.png`.

The top of the report has a table of the documents, from the worst. A click
on a heading sorts the table, and the fields above it filter the documents
by kind, by status and by name. Under the table, each document has a
section: its header, the errors and the log of ivy, and a row for each page
with the image of ivy and the images of the references. A click on an image
opens it at its full size. The box "show the differences", or the button of
a row, shows the map of the difference in place of each reference: red
where the pages differ, darker where they differ more.

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

The status of a document is `ok`, `partial` (ivy drew a page only in part),
`error`, `panic` (ivy panicked, which is a bug), `timeout`, `missing` (the
file is not in the cache) or `unsupported` (ivy has no package for the
kind).

## Add a format

To add a format of ivy, such as DjVu, add its `Format` value to the list
`formats` in `test/review/ivy.go`. The kinds of the manifest are the
extensions of the format. To add a reference, add a route or a renderer in
`test/review/refs.go`.
