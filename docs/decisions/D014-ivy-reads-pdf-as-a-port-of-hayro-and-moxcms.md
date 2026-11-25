# D14. ivy reads PDF as a port of hayro and moxcms

Status: Decided. Amended by D16. Amended by D20.

okage `docs/FORMATS.md` surveyed a PDF reader, EPS and Adobe Illustrator as
work for okage. They belong in ivy, and okage draws their pages:

1. The PDF reader is the package `pdf`. It also opens an Illustrator file
   that is compatible with PDF.
2. PostScript and EPS are the package `ps`. It also opens an older
   Illustrator file, after the PostScript interpreter exists (D1).
3. A PDF reader needs decoders of images, such as JBIG2 and JPEG 2000, and
   Type 1 fonts. okage D7 and okage D9 forbid both in okage. ivy holds them in
   `internal/` and in the font layer (D10).

okage `docs/FORMATS.md` gets a note that points to this decision. The
backlog holds that request.

## The port

The package `pdf` is a port of [hayro](https://github.com/LaurenzV/hayro),
under Apache-2.0 or MIT. hayro splits the work of a PDF reader into crates:

1. `hayro-syntax` reads the file, its objects, its streams and its
   encryption.
2. `hayro-interpret` runs the content of a page and sends each mark to an
   abstract `Device`. It does not draw.
3. `hayro-font`, `hayro-cmap` and `hayro-postscript` read the fonts and the
   character maps.
4. `hayro-ccitt`, `hayro-jbig2` and `hayro-jpeg2000` decode images.
5. The crate `hayro` is one `Device`, which draws with `vello_cpu`.

ivy ports the crates of items 1 to 4, and it does not port the crate
`hayro`. In its place, the device of the display list draws with okage
(D10). The codecs of the Go project dejapdf (MIT) can replace a codec of
hayro where they decode, after the backlog examines them.

ivy also ports [moxcms](https://github.com/awxkee/moxcms) (BSD-3-Clause),
the color management of ICC profiles that `hayro-interpret` uses, so that a
PDF made for print shows its colors. hayro uses the geometry crate kurbo,
which okage already ports (D7).

The port follows its sources file by file, with a ledger, as okage follows
resvg. A later encoder `pdf/pdfenc` can port `pdf-writer` and krilla, from
the author of hayro, and take its input from the same display list (D1).
