# D69. The review shows one or two oracles for each kind

Status: Decided. Amends D13. Amended by D70.

On 2026-11-25 the review of `test/cmd/review` showed, for each document,
ivy, ivy PDF, and each reference program that converts the kind, with each
PDF drawn by MuPDF, Poppler Splash and Poppler Cairo. An Office document
had up to six columns of references. The maintainers found the columns
useful, but too many to review. They asked for one primary oracle for each
kind, two at most, and kept the option to draw every reference.

The flagship models of OpenAI and Google were asked on 2026-11-25 which
oracles fit each kind. Both chose the program that made the file where the
review machine has it, then the program that most people use to view the
kind. Both chose Microsoft Office to PDF, drawn by MuPDF, for the files of
Office, with LibreOffice as the second. Both chose PDFium, the renderer of
Chrome, as the primary renderer of a PDF file, with MuPDF as the second.

## The decision

On 2026-11-25 the maintainers decided:

1. Each row of the review shows `ivy` and `ivy PDF`, as before.
2. By default, the review shows the primary oracle of the kind, drawn by
   MuPDF when it is a PDF file, and at most one secondary oracle. The
   table below gives them.
3. The flag `-refs all` of `render` draws every reference as before: each
   conversion, and each PDF drawn by MuPDF, Poppler Splash and Poppler
   Cairo. The cache keeps the pages of every reference, so a run with
   `-refs all` after a default run draws only what is missing.
4. The test module can import `github.com/chromedp/chromedp` and its
   dependencies, to draw a PDF file in Chrome and take a picture of each
   page (AGENTS.md, hard rule 5). Chrome is not part of the module that a
   user imports (D1).
5. Until the maintainers compare Chrome with MuPDF, Chrome draws each PDF
   file of a primary oracle next to MuPDF, as a second renderer of the
   same file. The comparison decides which one stays primary.

| Kinds | Primary oracle | Secondary oracle |
| --- | --- | --- |
| Word, Excel and PowerPoint, and RTF | Microsoft Office to PDF, drawn by MuPDF | LibreOffice to PDF, drawn by MuPDF. It is the primary when the file has no export of Office |
| Visio and Publisher | Microsoft Office to PDF, drawn by MuPDF, when the file has an export | LibreOffice to PDF, drawn by MuPDF |
| OpenDocument, CSV, TSV and TAB | LibreOffice to PDF, drawn by MuPDF | none |
| Pages, Keynote and Numbers | Apple iWork to PDF, drawn by MuPDF | LibreOffice to PDF, drawn by MuPDF, when the file has no export of iWork |
| PDF and Illustrator | MuPDF | Chrome, until the comparison of point 5 |
| HTML | WeasyPrint to PDF, drawn by MuPDF, because ivy ports its layout (D15) | none |
| The ebooks, text and Markdown | calibre to PDF, drawn by MuPDF | none |
| LaTeX and plain TeX | pdfLaTeX or pdfTeX to PDF, drawn by MuPDF | none |
| XPS and OpenXPS | Windows XPS (WPF), when the file has an export | libgxps |
| PostScript and EPS | Ghostscript | none |
| EMF and WMF | GDI+, when the file has an export | Inkscape |
| DjVu | DjVuLibre | none |
| Photoshop, OpenRaster and camera RAW | ImageMagick | none |

The models also put Chrome first for HTML, with WeasyPrint second. The
review keeps WeasyPrint as the primary, because ivy ports its layout. A
route through the printing of Chrome can come later as a secondary oracle
of HTML, text and Markdown.
