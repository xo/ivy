# D31. pdf can render another page box

Status: Decided.

A page of PDF has five boxes: the MediaBox, the CropBox, the BleedBox, the
TrimBox and the ArtBox. A viewer shows the CropBox, and `pdf` renders it.
A file made for a printer often draws its crop marks, its registration
marks and its color bars in the margin between the CropBox and the
MediaBox, such as `pdfjs/pdf/freeculture.pdf` of the review corpus. A
person who reviews such a file can want to see that margin. Poppler
renders the MediaBox by default, and MuPDF can, with its option `-b`.

On 2026-11-17 the maintainers decided:

1. The package `pdf` has the option `WithPageBox(box Box)`, with the
   constants `CropBox`, `MediaBox`, `BleedBox`, `TrimBox` and `ArtBox`.
   The default stays `CropBox`.
2. A page renders the chosen box: its size and its clip. A box that a page
   does not have takes its default from the PDF specification: the
   CropBox defaults to the MediaBox, and the BleedBox, the TrimBox and the
   ArtBox default to the CropBox. The rotation of the page applies as
   before.
3. The visual review compares the pages at the CropBox by default, with
   every reference. For a file whose MediaBox differs from its CropBox, it
   also shows a row for the MediaBox, with ivy, MuPDF and Poppler.
