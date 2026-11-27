# D64. A page gives its text and its places through one capability

Status: Decided.

D37 says that one decision designs one capability for the text of every
format. Item 1 of "Left by phase 6" in `BACKLOG.md` waits for it, for the
text layer of DjVu. D58 names "invisible text for the text layer of DjVu"
as a later item of the PDF writer. On 2026-11-21 the maintainers asked for
a proposal: how a program gets the text of a page, with the place of the
text on the page, from every format that has text. This decision is that
proposal. It adds public API, so it is a decision before any code (D3,
D11). Question 27 of [`../PLAN.md`](../PLAN.md) asked the maintainers to
choose.

This decision is about the text of a page only. The links and the outline
go through the display list in phase 7c of D58. A capability that gives a
program the outline of a document is a decision of its own.

## What exists

1. The display list. A `display.GlyphRun` holds its text, and each
   `display.Glyph` holds the bytes of the text of its cluster (D58, phase
   7b). The layout, the scenes, XPS, the metafiles and the charts fill
   them. pdfenc writes them as text that a reader can search and copy.
2. `pdf` and `ps`. The port of hayro fills the outline of each glyph with
   `display.FillText`, without its text. The interpreter knows the text of
   each glyph through `AsUnicode`, from the ToUnicode map or the encoding of
   the font. It also sends each run of invisible text, the text rendering
   mode 3 that OCR programs write over a scan, with `DrawModeInvisible`.
   The renderer draws nothing for such a run. So no text of a PDF or
   PostScript file reaches a device.
3. `djvu`. The reader of the text layer in `djvu/internal/doc` reads the
   chunks TXTa and TXTz into a tree of zones: page, column, region,
   paragraph, line, word and character. Each zone has a rectangle, in the
   pixels of the page before its rotation, with y up from the bottom. Each
   zone has its part of the text of the page. The page draws no glyph for
   this text, because its picture is one image.
4. The pages of `psd`, `ora`, `comic` and `camera` are images. They have no
   text. A text layer of PSD holds text, but ivy draws the composite of the
   file (D11).
5. `ivy.Page` has only `Size` and `Render`. The root package imports no
   internal package (D6). pdfenc gets the commands of a page from the
   internal interface `display.Drawer` (D58).

## What a program needs

A program uses the text of a page to search a document, to copy a part of a
page, to select text with the mouse, as iv can, and to read a document
aloud. pdfenc uses it to write a PDF file of a scan that a reader can
search. These uses need:

1. The text, in the order in which the page gives it.
2. For each part of the text, its place: four corners in the points of the
   page, with y down, after the rotation of the page. This is the space of
   `Render` with the identity transform, so a program draws a selection
   with the same transform that it renders with. A rotated run has a
   rotated box.
3. The place of each character, where the format knows it, to select a
   part of a word.
4. Whether the page shows the text. The text layer of a scan and the
   invisible text of PDF are text that the page does not show.
5. A break between two parts: none, a space, the end of a line or the end
   of a paragraph. Without it, a program joins the words of two lines.

ivy does not sort the text into reading order. The order is the order of
the page. The layout and DjVu give the order of reading. A PDF file gives
the order in which it draws, which can differ. A program that needs reading
order for a PDF file sorts the parts by their places. Such a sort is a
guess, and each program guesses in another way.

## The public API

The root package gets one new capability of a page. D11 says that a
capability that comes later is a new interface:

```go
// Texter is a page with text that a program can read (D64).
type Texter interface {
	// Text returns the text of the page, in the order that the page gives
	// it. A page without text returns no span and no error. If Text reads
	// the page only in part, it returns the spans that it read and a
	// *PartialError.
	Text(ctx context.Context) ([]TextSpan, error)
}

// TextSpan is one part of the text of a page, with its place.
type TextSpan struct {
	// Text is the text of the span, in UTF-8.
	Text string
	// Quad holds the corners of the box of the span, in points of the
	// page with y down, after its rotation: the start and the end of the
	// bottom edge, then the end and the start of the top edge.
	Quad [4]okage.Point
	// Chars holds the box of each character or cluster of Text, in
	// order. It is empty when the format knows only the box of the span.
	Chars []TextChar
	// Hidden is true for text that the page does not show, such as the
	// text layer of a scan or the invisible text of a PDF file.
	Hidden bool
	// Break is what comes after the span.
	Break TextBreak
}

// TextChar is the box of one cluster of a TextSpan.
type TextChar struct {
	// Start and End are the bytes of the Text of the span.
	Start, End int
	Quad       [4]okage.Point
}

// TextBreak is what comes after a span of text.
type TextBreak int

const (
	BreakNone      TextBreak = iota // the next span continues the word
	BreakSpace                      // a space
	BreakLine                       // the end of a line
	BreakParagraph                  // the end of a paragraph, a column or a region
)
```

The rules:

1. `Text` is safe for concurrent use, as `Render` is.
2. `Limits.MaxObjects` bounds the number of spans, and the number of
   characters of all spans together. Past the limit, `Text` returns the
   spans before it and an error that wraps `ErrLimit`.
3. A span is a run of glyphs, a word of DjVu, or the glyphs of PDF that
   one text operator draws. A span is not a line, and two spans can share a
   line.
4. A format that has no text has no method `Text`. A program asserts
   `page.(ivy.Texter)` as it asserts every capability.

The choices for the public API:

| Choice | For | Against |
| --- | --- | --- |
| A. `ivy.Texter` on a page, with spans, boxes of characters, `Hidden` and `Break` (recommended) | one API for every format. A program can search, copy and select. It follows D11: a small interface that a program finds by assertion | the root package gets four types |
| B. No public API. Only the internal marks of the next section, for pdfenc | no new public type. pdfenc writes the text of DjVu | a program gets no text. D37 still waits |
| C. The text of the whole document, as one string or one iterator of `Document` | short to call for a search | a flow of text lays out every page first. The places need the page anyway |
| D. A public display list, which a program reads | every command of the page | D58 rejected it. The display list changes with each format (D10) |

The choices for the size of a span:

| Choice | For | Against |
| --- | --- | --- |
| A. A run, a word or a text operator, with the boxes of its characters (recommended) | each format gives what it knows, without a guess. The boxes give a selection inside a word | a program joins the spans of a line itself, with `Break` |
| B. One span for each character | one form for every format | many small values. The words of DjVu lose their boxes, and the text of a cluster splits |
| C. Lines and blocks, as the structured text of MuPDF | close to what a viewer shows | ivy guesses the lines of a PDF file, and its guess differs from the guess of each other program |

## How a page gives its text

The text of most pages already reaches a device through `Draw` (D58). So
the text goes through the same device calls that pdfenc reads.

A new optional interface of `internal/display` takes the text that a page
does not draw as glyphs, as `Linker` and `Bookmarker` of D58 take the
links:

```go
// TextMarker is a device that takes text that a page does not draw as a
// run of glyphs, such as the text layer of a scan.
type TextMarker interface {
	// MarkText marks the text of a span at the quad q, in the user space.
	// chars holds the quad of each cluster, or nil. hidden is true for
	// text that the page does not show.
	MarkText(text string, q [4]okage.Point, chars []TextChar, hidden bool, brk TextBreak)
}
```

`TextChar` and `TextBreak` here are internal types with the fields of the
public ones. The root package does not import `internal/display` (D6).

1. The okage device does not implement `TextMarker`, so the pixels do not
   change. A `display.List` records the marks, as it records the links.
2. A page asserts the device to `TextMarker` before it reads its text. A
   page then reads no text when it renders.
3. `internal/display` gets a text device. It implements `Device` and
   `TextMarker`, and ignores the paths and the images. For
   each `GlyphRun` with text it makes one span. The box of each glyph
   comes from its origin, its advance, and the ascent and the descent of
   its face, through the current transform. `font.Face` has `Advance`
   today, and it gets a method for the ascent and the descent. Each mark
   becomes one span.
4. The function `display.TextOf` calls `Draw` of a page with the text
   device, and returns the spans. Each page type of the table of D58 that
   has text implements `Text` with it.
5. A page whose `Draw` costs much more than its text implements `Text`
   without `Draw`. A page of DjVu reads its zones and does not composite
   its image.

The choices for the path of the text:

| Choice | For | Against |
| --- | --- | --- |
| A. Through `Draw`, with `GlyphRun.Text` and the marks of `TextMarker` (recommended) | one path for the program and for pdfenc. The text of a page is where its glyphs are, by construction | `Text` of a PDF page interprets the whole page and decodes its images. A later change can skip the decode for a device that wants only text |
| B. A method of each format on its own model, such as the boxes of the layout or the elements of XPS | each format reads only its text | each format computes the places again, apart from the place where it draws. pdfenc still needs the marks for DjVu |
| C. Invisible runs of glyphs: DjVu makes a `GlyphRun` with a face of `internal/fontdb` and a flag `Invisible` | pdfenc writes the run as other runs | `djvu` shapes text and links the font database, and iv links more code. The face must hold every script of the text |

## What each format gives

| Formats | Where the text comes from | Boxes of characters | Hidden text |
| --- | --- | --- | --- |
| `epub`, `mobi`, `fb2`, `chm`, `html`, `markdown`, `text`, `docx`, `odf`, `rtf`, `msdoc` | the runs of the layout | yes, from the clusters of the shaper | no |
| `xlsx`, `msxls`, `csv`, the sheets of `odf` | the runs of the layout | yes | no |
| `pptx`, `msppt`, `vsdx`, the drawings of `odf` | the text bodies through the layout, and the charts | yes | no |
| `xps` | `UnicodeString` and `Indices` | yes | no |
| `emf` | the text of the records (D53) | yes | no |
| `pdf`, `ps` | `AsUnicode` of each glyph, through `MarkText`, for each text operator | yes, one for each glyph | the text rendering mode 3 |
| `djvu` | the zones of the text layer, one span for each word | yes, when the word has zones of characters | all of it |
| `latex` | the runs of the driver of D62, in phase 8 | yes | no |
| `psd`, `ora`, `comic` | none. The page is not a `Texter` | | |

The text of DjVu maps onto spans in this way:

1. Each zone without children is one span: a word, or a line or a
   paragraph that has no words. A word with zones of characters is one
   span, and its characters give `Chars`.
2. The text of a span is the text of its zone without the separator at its
   end. The separator gives `Break`: a space after a word, a line after the
   last word of a line, and a paragraph after the last line of a
   paragraph, a region or a column.
3. A point of the page is the pixel times 72 and divided by the resolution
   of the INFO chunk, with y turned down. The rotation of the INFO chunk
   then turns the quad, as `Draw` turns the image.
4. A zone with a rectangle that is empty, or that is outside the page, is
   kept with its box cut to the page. Its text stays searchable.

## The text of DjVu in a PDF file

The page of DjVu calls `MarkText` with `hidden` true for each word, inside
the transform of its image. The PDF device of pdfenc implements
`TextMarker` and writes each mark as invisible text, over the image. A
reader of the PDF file then searches, selects and copies the words of the
scan. This is what OCR programs write.

1. The device takes a face of the font database of `internal/fontdb` that
   holds the characters of the text, with the fallback that the layout
   uses. A character that no face holds gets the glyph `.notdef`, and the
   writer gives its text with an `ActualText` span (okage D62 item 9).
2. The device shapes the text with that face. It scales the run across so
   that its advance fills the width of the quad, and up so that the
   ascent and the descent fill its height. A reader then selects the word
   where the scan shows it.
3. The device writes the glyphs with the text rendering mode 3, which
   neither fills nor strokes. The page looks the same as without the text.
4. A mark that is not hidden, such as a glyph of PDF that hayro fills as a
   path, goes into the file in the same way, over the path. Then the text
   of a PDF file that pdfenc writes again is searchable, before the Type 3
   fonts of D58 come.

The surface of okage has no invisible text. Its internal writer knows the
mode 3, and `Surface.DrawGlyphs` writes only fill, stroke or both. okage
gives a way to draw glyphs with the mode 3, such as `DrawGlyphs` without a
fill and without a stroke. This is work for okage (D8). okage v0.24.0
gives `Surface.DrawInvisibleGlyphs`, which draws a run in the mode 3 (okage
D106).

The choices for the font of invisible text:

| Choice | For | Against |
| --- | --- | --- |
| A. A face of the font database, with its fallback (recommended) | no new font. The faces that pdfenc embeds already go through it | a subset of a real font goes into the file for each script |
| B. A font without glyphs, made for this use, as the GlyphLessFont of Tesseract | a small file, one font for every script | ivy makes and embeds a font of its own. okage embeds TrueType and OpenType files only |
| C. Paths with no fill and with text through `ActualText` | no font at all | readers find `ActualText` around paths in fewer ways than they find text |

## Limits, errors and tests

The limits of D5 hold. `MaxObjects` bounds the spans and the characters of
`Text`, and the reader of the zones of DjVu, which also takes `MaxDepth`
and `MaxMemory`. The context bounds the time. The text device draws no
pixel, so its memory is the text and the boxes.

The tests:

1. Unit tests in each format compare the spans of a small file with the
   text and the boxes that the test expects.
2. The text of `djvu` matches `djvused print-txt` of DjVuLibre in the test
   module (D13), for the test files of `djvu` and the books of the corpus.
   The words and their boxes must be equal, after the conversion to points.
3. The text of `pdf` matches `pdftotext -bbox-layout` of Poppler and
   `mutool draw -F stext` of MuPDF. Each word of the oracle must be in the
   spans, in order, with a box that overlaps the box of the oracle.
4. The text of the formats of the display list matches the text of the
   runs that the text test of D58 already reads.
5. The PDF file that pdfenc writes from a DjVu file gives the words of
   `djvutxt` through `pdftotext`, in order. Each page looks as before, by
   the comparisons of D58.
6. A fuzz test of each format calls `Text` on the pages that it opens.

## Phases

The estimates are for one agent with a maintainer.

| Phase | What it gives | Estimate |
| --- | --- | --- |
| 64a. The text of DjVu in PDF | `display.TextMarker`, its recording in `display.List`, the marks of the zones of DjVu in `Draw`, the invisible text of the PDF device, and the work for okage | 1 to 2 weeks |
| 64b. The capability | `ivy.Texter`, `TextSpan`, the text device and `display.TextOf`, `Text` of each page type of the display list and of DjVu, the oracles and the fuzz tests | 2 to 3 weeks |
| 64c. The text of PDF and PostScript | the marks of hayro with `AsUnicode`, the invisible runs, the oracles of Poppler and MuPDF, and the text of a PDF file that pdfenc writes again | 1 to 2 weeks |

Phase 64a needs no public API, and it can go first. Phase 64b gives the
capability for every format except `pdf` and `ps`, and phase 64c adds
them.

## The choices in short

| Point | Recommendation |
| --- | --- |
| The public API | `ivy.Texter` on a page, with `TextSpan`, `TextChar` and `TextBreak` |
| The size of a span | a run, a word or a text operator, with the boxes of its characters |
| The path of the text | through `Draw`, with `GlyphRun.Text` and the new internal `display.TextMarker` |
| A page whose `Draw` costs much | `Text` reads the text without `Draw`, as `djvu` does |
| The text of DjVu in a PDF file | invisible text, the mode 3, with a face of the font database, scaled to the box of each word |
| The order of the text | the order of the page. ivy does not sort it |

## What changes when the maintainers decide

1. `API.md` adds `Texter` to "Documents and capabilities", and its item on
   `internal/display` names `TextMarker` and the text device.
2. `PLAN.md` gets the phases above.
3. "Work for okage" in `BACKLOG.md` gets the invisible text of the surface.
4. Item 1 of "Left by phase 6" in `BACKLOG.md` loses the text layer.
   Phases 64a and 64b do it.

## The decision

On 2026-11-21 the maintainers answered question 27 with choice 1: the public
capability `ivy.Texter` and the internal `display.TextMarker`, built in the
phases 64a, 64b and 64c. The public API is built as this decision gives it,
and `docs/API.md` gets it with phase 64b.
