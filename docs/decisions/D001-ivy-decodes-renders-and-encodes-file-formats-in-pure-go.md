# D1. ivy decodes, renders and encodes file formats in pure Go

Status: Decided.

ivy is a set of decoders, renderers and encoders for the file formats that a
program shows as images: documents, ebooks, page descriptions, images with
layers, and containers that carry an icon or a preview. A program that
imports ivy needs the Go toolchain and nothing else.

1. ivy does not use cgo, it does not link a library of C, C++ or Rust, it
   does not run WebAssembly, and it does not run an outside program.
2. ivy is mostly self-contained. It reuses what okage and magic hold, and it
   ports or copies the rest, as D7 says.
3. ivy draws with okage, and the type of a file comes from magic (D8).

The first program that uses ivy is [iv](https://github.com/kenshaw/iv), an
image viewer for the terminal. iv runs outside programs today for many of
these formats: `ebook-convert` of calibre, `xpstopng` of libgxps, `soffice` of
LibreOffice, and `pdftoppm` of poppler. It renders PDF files with libvips and
HTML with the Rust engine Blitz, through cgo. ivy replaces the outside
programs and libvips for these formats with Go code.
[`docs/PLAN.md`](../PLAN.md) lists which package of ivy replaces which
decoder of iv.

## What comes later

1. Encoders. ivy plans no encoder until a program needs one. The name of
   each encoder is ready (D6). The candidates are a PDF writer, as a port of
   `pdf-writer` and krilla, encoders of ICNS and ICO, and encoders of CBZ
   and EPUB.
2. The long tail of [`docs/FORMATS.md`](../FORMATS.md): the PostScript
   interpreter, the older Illustrator files, a layout of the binary Office
   formats, Microsoft Reader, Sony BBeB, the rare ebooks and GIMP. Each one
   waits until a user asks for it, and the maintainers then decide it.
