# D19. ivy does not hint glyphs, for now

Status: Decided.

Hinting moves the outline of a glyph so that its edges fall on whole pixels.
hayro hints the TrueType and CFF fonts of a PDF with skrifa, and it does so by
default. ivy reads fonts with go-text (D16), which does not hint. A port of
the hinting of skrifa, with the parts of read-fonts that it needs, is about
40,000 to 55,000 lines of Rust.

On 2026-11-16 the maintainers measured what hinting changes. hayro rendered
the first page of 13 PDFs at 72, 96 and 144 dots per inch, with and without
hinting, and a program counted the pixels of ink whose brightness changed by
more than 32 of 255:

| The fonts of the PDF | The pixels of ink that hinting changes |
| --- | --- |
| Type 1, such as the output of LaTeX | none, because skrifa does not hint Type 1 |
| TrueType without instructions | none |
| Most pages from the corpus of hayro, in TrueType, CFF and CID fonts | from 0.6 to 10 percent, mostly from 1 to 5 |
| Body text that LibreOffice exported, with fonts that keep their instructions | 16 percent at 144, 21 at 96, and 28 at 72 dots per inch |

The hinting of hayro moves edges only up and down. It snaps the baselines, the
x-height and the horizontal strokes to the pixels, so the text looks a little
crisper and darker. The share of gray pixels at the edges of the glyphs did
not change. Text that is not hinted is as easy to read (D5).

So ivy does not hint glyphs, and does not port skrifa. The maintainers can
decide again if the users of ivy show documents where hinting matters. The
tool of the measurement and its numbers are described in
[`docs/BACKLOG.md`](../BACKLOG.md).
