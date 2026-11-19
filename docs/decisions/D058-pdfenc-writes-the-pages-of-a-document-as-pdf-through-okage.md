# D58. pdfenc writes the pages of a document as PDF through okage

Status: Decided.

On 2026-11-19 the maintainers asked for a proposal for PDF output. That is
a public API that writes the pages of a document as a PDF file, through the
PDF writer of okage. This decision is that proposal. It holds the API and
the device that maps the display list onto the writer. It also holds what
each format gives, the fonts, the limits, the errors, the tests and the
phases. Where a
choice is real, it names the choices and recommends one. Question 23 at the
end of [`../PLAN.md`](../PLAN.md) asks the maintainers to accept it. No
code is written before they decide (D3), because the change adds a public
package (D11).

## What exists

okage has the package `pdf`, a port of krilla (okage D62), from okage
v0.16.0. A `pdf.Document` starts pages. A `pdf.Page` gives a `pdf.Surface`,
which draws paths, glyphs, images and shadings, and pushes and pops
transforms, clips, masks, opacities and blend modes. A `pdf.Font` is the
bytes of a TrueType, OpenType or collection font. The surface embeds a
subset of each font, with a ToUnicode map from the text that the caller
gives with the glyphs. The document also takes the metadata, an outline,
named destinations and link annotations. `Document.Finish` writes the whole
file to an `io.Writer`. The document holds every object in memory until
then, because the subset of a font needs every glyph of the document
(okage D62 item 6).

okage D62 item 11 says that ivy keeps its own device for PDF output, which
maps the calls of its display list onto a `pdf.Surface`. okage D68 adds what
that device needs and krilla does not have:

| Addition of okage D68 | What it gives |
| --- | --- |
| `Surface.PushGroup(pdf.Group)` | a transparency group with an opacity, a blend mode, a soft mask, and the flags isolated and knockout |
| `Mask.WithBackdrop`, `Mask.WithTransfer` | the backdrop color and the transfer function of a soft mask |
| `Extend` of the gradients, `MeshGradient` | the extend rule of a gradient, and a mesh of triangles as a shading of type 4 |
| `Surface.FillStencil` | a fill through a stencil of 8 bits |
| `Surface.PushStrokeAdjustment` | the stroke adjustment of PDF |
| `Surface.PaintFromShader` | a `pdf.Paint` from an `okage.Shader`, or false |
| `Surface.DrawRaster` | a part that okage draws into a pixmap, which the file holds as an image |

okage D68 is in a worktree of okage on 2026-11-19, and no tag holds it. ivy
requires okage v0.14.0. The work of this decision requires the first tag of
okage that holds okage D68.

ivy draws most pages through the display list of D10. A page description,
such as PDF, XPS, PostScript or EMF, sends its commands to a
`display.Device`. A flow of text, such as EPUB or Word, goes through the
layout, which records each page into a `display.List` (D47) and replays it.
A slide, a drawing of Visio or a chart draws a scene onto a device. A raster
page, such as PSD, OpenRaster, a page of a comic book or a page of DjVu,
draws one image. The display list holds everything that a PDF file
needs, except two things:

1. The text. A `display.GlyphRun` holds the glyphs and their positions, not
   the characters that they stand for.
2. The font files. A `font.Face` holds a parsed face, not the bytes that
   the PDF writer embeds.

`ivy.Page` has only `Size` and `Render`, and `Render` draws onto a pixmap.
So a writer needs another way to get the commands of a page. The root
package cannot give it, because it does not import `internal/display` (D6).

[`../API.md`](../API.md) reserves `pdf/pdfenc` for the PDF writer, as D6
names it, and sketches `Encode` with an iterator of pages.

## The public API

The package `github.com/xo/ivy/pdf/pdfenc` holds the API:

```go
package pdfenc

// Option is an option of Encode and EncodePages.
type Option func(*options)

// WithConfig sets the configuration that every format shares. Encode
// reads its Logger and its Limits.
func WithConfig(cfg ivy.Config) Option

// WithPages writes the pages whose numbers are in pages, from 0, in that
// order. Without it, Encode writes every page. A number past the last page
// gives ErrPageRange before Encode draws a page.
func WithPages(pages ...int) Option

// WithInfo gives the title, the authors and the dates of the file. Without
// it, Encode takes them from Describe of the document, when the document
// is an ivy.Describer.
func WithInfo(info ivy.Info) Option

// WithOutline writes the outline of the document, where its format knows
// one. It is on by default.
func WithOutline(on bool) Option

// WithLinks writes the links of the pages, where their format knows them.
// It is on by default.
func WithLinks(on bool) Option

// WithRasterDPI sets the resolution, in dots per inch, of a part that a
// PDF file cannot hold as vectors, and of a page that ivy can only render.
// The default is 300.
func WithRasterDPI(dpi float32) Option

// Encode writes the pages of d to w as one PDF file. d must be an
// ivy.Pager. Encode writes to w only after it drew every page, so w gets
// no bytes when Encode fails before it writes. When a page draws only in
// part, Encode still writes the file, and returns a *ivy.PartialError
// that names each such page.
func Encode(ctx context.Context, w io.Writer, d ivy.Document, opts ...Option) error

// EncodePages writes pages to w as one PDF file, as Encode does, such as
// pages of more than one document. It writes no outline, and the metadata
// only from WithInfo.
func EncodePages(ctx context.Context, w io.Writer, pages iter.Seq2[ivy.Page, error], opts ...Option) error
```

The rules:

1. Each page of the file has the size of the page in ivy, after its
   rotation, as `Page.Size` gives it. A page of `pdf` has the size of the
   box that `pdf.WithPageBox` chose (D31).
2. The metadata maps `ivy.Info` onto `pdf.Metadata`. `Title`, `Authors`,
   `Subject` and `Language` keep their names. `Subject` is the
   description of the file. `Created` is the creation date. `Producer` of
   `ivy.Info` names the program that made the document, so it becomes the
   creator of the file. `Publisher`, `Identifier` and `Modified` have no
   field in `pdf.Metadata`, and the file leaves them out.
3. `Encode` takes a document and not a pager, so that a later capability,
   such as the outline that D37 waits for, needs no new function.
4. `EncodePages` is the form of the sketch of `API.md`. A program uses it
   to join the pages of several documents into one file.
5. pdfenc imports no format, and no format imports pdfenc (D6). A program
   that writes no PDF does not link pdfenc or the package `pdf` of okage.

The choices for the shape of the API:

| Choice | For | Against |
| --- | --- | --- |
| A. `Encode` of a document and `EncodePages` of pages (recommended) | `Encode` reads the metadata and the outline of the document. `EncodePages` joins documents | two functions |
| B. `EncodePages` only, as `API.md` sketches it | one function | a program builds the metadata itself, and the file gets no outline |
| C. A method of each document, such as `doc.WritePDF(w)` | short to call | each format links the writer, which D6 forbids |

## How pdfenc gets the commands of a page

A page that draws through the display list implements a new interface of
`internal/display`:

```go
// Drawer is a page that draws onto a device of the display list.
type Drawer interface {
	// Draw draws the page onto d, in points with y down, as Render draws
	// it with the identity transform. res is the resolution, in pixels for
	// each point, of a part that the page draws into a pixmap first, such
	// as a tiling pattern of PDF.
	Draw(ctx context.Context, d Device, res float32) error
}
```

pdfenc asserts each `ivy.Page` to a `display.Drawer`. A page that is not a
`Drawer`, such as a page that a program implements itself, goes into the
file as one image. `ivy.RenderImage` draws that image at the resolution of
`WithRasterDPI`.

Most pages already draw through a device: `Render` makes a
`display.OkageDevice` and calls the code of the format with it. The change
moves that code into `Draw`, and `Render` calls `Draw` with an okage device.
The pixels of `Render` do not change. A layout page replays its recording
(D47). The types that implement `Draw` are these:

| Package | Type |
| --- | --- |
| `pdf` | `*pdf.Page`, through `RenderInto` of `internal/hayro/render` |
| `ps` | the page of PostScript and EPS |
| `xps` | `*xps.Page` |
| `emf` | `*emf.Page` |
| `djvu` | `*djvu.Page` |
| `internal/layout` | the page of a book, for every format of the layout |
| `internal/drawingml` | `*Scene`, for `pptx`, `msppt` and the shapes |
| `pptx`, `msppt`, `vsdx`, `odf/internal/drawpage` | the pages of slides and drawings |
| `internal/raster` | `*raster.Page`, for `psd`, `ora` and `comic` |

Four of these types are exported from a public package: `*pdf.Page`,
`*xps.Page`, `*emf.Page` and `*djvu.Page`. The documentation of their
method `Draw` shows a parameter of an internal type. A program cannot
call it, because it cannot name a `display.Device`.

The choices:

| Choice | For | Against |
| --- | --- | --- |
| A. The internal interface `display.Drawer` (recommended) | no change to the public API of a format. The layers stay internal (D10) | the method `Draw` shows in the documentation of four exported types |
| B. A public device in the root package | a program can draw a page onto its own device | the display list becomes public API, and it still changes with each format (D10) |
| C. Only raster pages | no change to the formats | no vectors, no text to search or copy, and large files |

## The PDF device

The device is an unexported type of pdfenc, in the file `device.go`. It
implements `display.Device`, `display.TextFiller` and
`display.StrokeAdjuster`.

| Choice | For | Against |
| --- | --- | --- |
| A. In pdfenc (recommended) | only a program that writes PDF links okage `pdf`. The rule of D6 for iv holds | the tests of the device are in pdfenc |
| B. In `internal/display`, next to the okage device | the two devices are side by side | every format imports `internal/display`, so iv links the writer. D6 forbids this |

The device keeps a stack of the number of pushes of each `Save`, and
`Restore` pops them. Each call maps onto the surface as this table says:

| Call of the display list | What the device calls |
| --- | --- |
| `Save`, `Restore` | a mark on the stack of pushes, and `Pop` for each push since the mark |
| `Concat` | `PushTransform` |
| `ClipPath` | `PushClipPath` |
| `FillPath` | `PaintFromShader`, then `SetFill` and `DrawPath` |
| `StrokePath` | `PaintFromShader`, then `SetStroke` and `DrawPath`. The dashes, the joins, the caps and the miter limit map one to one |
| `StrokeAdjusted` | `PushStrokeAdjustment(true)`, the stroke, and `Pop` |
| `FillText` | `DrawPath` with the fill, as `FillPath` |
| `DrawImage` | `PushTransform` of the transform of the image, `PushOpacity` when the opacity is below 1, and `DrawImage` of a `pdf.Image`. `Smooth` sets the flag `Interpolate` |
| `FillMask` | `PushTransform`, `SetFill` with the paint of the shader, and `FillStencil` |
| `FillGlyphs` | `DrawGlyphs` with the font of the face, as "Text and fonts" says. A face that cannot go into the file fills its outlines with `DrawPath` |
| `BeginGroup`, `EndGroup` | `PushGroup` with the opacity, the blend mode, `Isolated`, `Knockout` and the mask, and `Pop` |
| `Mask` of a group | `NewMask` of a stream whose surface draws `Content` through a second device, with `WithBackdrop` and `WithTransfer` |

The device keeps the `pdf.Image` of each `image.Image` that it already
converted, by its pointer. A PDF document keeps its decoded images (D49),
and so an image that many pages share becomes one object of the file. The
device keeps the mask of each `Mask.Key` with its transform in the same way.

Some things that ivy draws have no form in PDF. The device draws each one
with okage through `Surface.DrawRaster`, at the resolution of
`WithRasterDPI`:

1. A shader that `PaintFromShader` does not convert. These are a radial
   gradient with the spread mode repeat or reflect, a sweep gradient with
   the decal mode, and a `Pattern` with the spread mode pad or decal. The
   fallback draws the one fill or stroke, inside the bounds of its path.
2. A group with a blend mode that PDF does not have, such as
   `okage.BlendModeDestinationOut` or `PlusLighter`. The fallback records
   the content of the group into a `display.List` and replays it onto an
   okage device, inside the bounds of the clip.

The fallback draws a part without its backdrop. So a part with a blend
mode that needs the backdrop draws as if the backdrop were transparent. The
review sees the shape and the colors of the part (D5). The okage device
does not change.

## What each format gives

| Formats | Pages | Text that a reader can search and copy | Links and outline |
| --- | --- | --- | --- |
| `epub`, `mobi`, `fb2`, `chm`, `html`, `markdown`, `text`, `docx`, `odf`, `rtf`, `msdoc` | vectors, through the layout | yes, from the clusters of the shaper | links and bookmarks of the layout, and the table of contents of a book |
| `xlsx`, `msxls`, `csv` and the sheets of `odf` | vectors, through the layout | yes | the links of the layout, where a sheet has them |
| `pptx`, `msppt`, `vsdx`, the drawings of `odf` | vectors, through the scenes | yes, the text bodies go through the layout | the links of the shapes, in a later phase |
| `xps` | vectors | yes, from `UnicodeString` and `Indices` | `NavigateUri` and the document outline |
| `emf` | vectors | yes, from the text of the records (D53) | none |
| `pdf`, `ps` | vectors | in a later phase. The glyphs are paths at first, as "Text and fonts" says | the links and the outline of a PDF file |
| `psd`, `ora`, `comic` | one image of the page, at its own pixels | none | none |
| `djvu` | one image of the page, at its own pixels | in a later phase, from the text layer | the outline and the links of the file |

A raster page draws its image at its own pixels, not at `WithRasterDPI`.
`raster.Page` gives one point for each pixel, so the image fills the page
at one pixel for each point.

## Text and fonts

### The text of a run

`display.GlyphRun` gets the text of its glyphs, and each `display.Glyph`
gets its cluster:

```go
type GlyphRun struct {
	// ... the fields of today
	// Text is the text of the run, or empty when the format does not know
	// it.
	Text string
}

type Glyph struct {
	ID   font.GID
	X, Y float32
	// Start and End are the bytes of Run.Text that the cluster of the glyph
	// covers. The glyphs of one cluster have the same range. Both are 0
	// when the run has no text.
	Start, End int
}
```

The okage device ignores the text. So the pixels do not change. The
`display.List` records it. The PDF device gives the text and the ranges to
`DrawGlyphs` as the `TextRange` of each `pdf.Glyph`, and the writer builds
the ToUnicode map from them. When a cluster does not map to its glyphs one
to one, the writer adds an `ActualText` span (okage D62 item 9). A range
that is not valid for the text drops the text of the run, and the glyphs
stay. A reader then cannot copy that run, but the page looks the same.

The producers of the text:

1. The layout, from the clusters of its shaper, which `textCluster` of
   `internal/layout` already holds.
2. XPS, from `UnicodeString` and the cluster map of `Indices`.
3. The metafiles, from the text of `EMR_EXTTEXTOUTW` and of the records of
   EMF+ and WMF (D53).
4. The charts, from the labels that they lay out.

### The font files

`font.Face` keeps the source of its face. The source is the bytes of the
font file, the index in a collection, and the coordinates of a variable
font. `font.Load`
keeps the bytes that it reads. A face of the font database gets its bytes
from `FaceData` of the font database of okage. A WOFF or WOFF2 font of an
EPUB file keeps the bytes of the TrueType or OpenType font that `webfont`
of okage gives. The device makes one `pdf.Font` for each source, with
`pdf.NewFont` and `WithVariations`, and keeps it for the whole file. The
writer then embeds one subset for each font.

`DrawGlyphs` takes a start and an advance for each glyph, not a position
for each glyph. The device turns the positions of the run into advances
and offsets. The transform of a run can be other than the identity, such as the slant
of a synthetic italic or a sideways glyph of XPS. Then the device draws each
glyph with its own `PushTransform`.

A color glyph needs no work in ivy. The writer reads the `COLR`, `sbix`,
`CBDT` and `SVG` tables from the font file, and makes a Type 3 font (okage
D62 item 9).

A synthetic bold face strokes the outlines with `StrokePath` today. The
stroke stays a path over the text, and the text stays searchable.

### Fonts that the license does not let a program embed

The `OS/2` table of a font has the field `fsType`. When its bit 1 is set,
the license forbids the embedding of the font. Bit 8 forbids a subset, and
bit 9 lets a program embed only the bitmaps. krilla embeds every font, and
only its test of PDF/A reports such a font (okage D62 item 9).

| Choice | For | Against |
| --- | --- | --- |
| A. Embed every font, as krilla does | the text stays searchable | the file holds a font that its license does not let it hold |
| B. Draw the glyphs of such a font as paths (recommended) | the file holds no part of the font program, and the page looks the same | the text of that font cannot be searched or copied |
| C. An option, with B as the default | a program that has the right to embed a font can do it | one more option of the public API |

With B, `font.Face` reads `fsType`, and the device draws the glyphs of a
face with bit 1, bit 8 or bit 9 set as paths. The fonts that ivy bundles
let a program embed them: the Go fonts, and Liberation, Carlito and Caladea
of the module `fonts` (D24). So the choice changes only the fonts that a
document embeds and the fonts of the system (D23).

### Fonts that are not TrueType or OpenType

The fonts of `pdf` and `ps` come from the port of hayro. They are Type 1,
Type 3, bare CFF, TrueType and OpenType fonts. The renderer of hayro fills the outline of
each glyph with `display.FillText`. No `GlyphRun` holds them, and the
writer of okage reads only TrueType and OpenType. The choices:

| Choice | For | Against |
| --- | --- | --- |
| A. Paths, with no text (recommended for the first phases) | no change to hayro or to okage. The page looks the same | a reader cannot search or copy the text of a PDF or PostScript file |
| B. A Type 3 font from the outlines, with the text of the glyphs (recommended for a later phase) | the text is searchable for every kind of font. Each glyph is one outline in the file, not one path for each use | okage adds a font made of outlines, which okage question 25 asks for. hayro sends a run with the text from `AsUnicode` of its glyphs |
| C. Wrap a bare CFF font in OpenType, and embed the TrueType fonts of the file | the real fonts of the file | Type 1 and Type 3 still need A or B. A wrapper is a font writer of its own |

The text of a glyph of hayro comes from the ToUnicode map of the source
file, through `AsUnicode`. A glyph without it has no text, as in the
source file.

## Links and the outline

D37 says that one decision designs one capability for the text and the
outline of every format. This decision does not design that capability.
The marks that a page draws carry its links and its bookmarks, inside the
display list. A device that wants them implements two optional interfaces
of `internal/display`, as `TextFiller` is optional:

```go
// Linker is a device that takes the links and the anchors of a page.
type Linker interface {
	// LinkURI makes r, in the user space, a link to uri.
	LinkURI(r okage.Rect, uri string)
	// LinkAnchor makes r a link to the anchor name of the document.
	LinkAnchor(r okage.Rect, name string)
	// Anchor names the point x, y of the page.
	Anchor(name string, x, y float32)
}

// Bookmarker is a device that takes the bookmarks of a page.
type Bookmarker interface {
	// Bookmark adds an entry of the outline at x, y of the page, at
	// level, from 1. open shows its children.
	Bookmark(level int, label string, x, y float32, open bool)
}
```

The okage device implements neither, so the pixels do not change. A
`display.List` and the recorder of `ps` record them. The PDF device maps
them onto the surface:

1. A link to a URI becomes a link annotation with a `LinkAction`.
2. A link to an anchor becomes a link annotation to a named destination,
   and each anchor becomes a `NamedDestination`. The device resolves the
   names after the last page. A link to an anchor on a page that the file
   does not hold, because `WithPages` left the page out, is dropped.
3. A rectangle that the transform turns becomes quad points.
4. The bookmarks of all pages, in the order of the pages, become the
   outline. A bookmark of level n is a child of the last bookmark of a
   lower level, as `make_bookmark_tree` of WeasyPrint builds it.

The sources of the links and the bookmarks:

1. The layout ports the rest of `weasyprint/anchors.py`, which
   `internal/layout/anchors.go` leaves out today. It gives the links of
   `<a href>`, the anchors of `id`, and the bookmarks of the property
   `bookmark-level`. The style sheet of the user agent of WeasyPrint gives
   `h1` to `h6` a bookmark level. Each format of the layout gets them.
2. A book of `epub`, `mobi`, `fb2` or `chm` adds a bookmark at the target
   of each entry of its table of contents, in place of the headings.
3. `pdf` gives the link annotations of each page, and puts each entry of
   the outline of the file on the page of its destination.
4. `xps` gives `NavigateUri`, and `djvu` gives the links and the outline
   of the file, as `pdf` does.

The choices:

| Choice | For | Against |
| --- | --- | --- |
| A. Links and bookmarks inside the display list (recommended) | no public API. The layout gets them as WeasyPrint does. Each format adds them in its own unit | an outline of a PDF file whose entries are not in the order of the pages comes out in the order of the pages |
| B. Wait for the capability of D37 | one design for the outline of every format, for a viewer and for the writer | no outline and no links until that decision, and the links still need a way through the page |

With A, the capability of D37 can come later. When it comes, pdfenc can
take the outline from it, and the links stay in the display list.

## Limits and errors

The input of pdfenc is the pages of a decoder, and each decoder already
takes its limits from `ivy.Config` (D5). pdfenc adds these limits, from
`Limits` of `WithConfig`:

1. The writer holds the whole file in memory until `Finish` (okage D62
   item 6). The device counts the bytes that it gives to the writer. These
   are the pixels of each new image, at 4 bytes each, the bytes of each new
   font file, and the commands of the pages. When the count passes
   `MaxMemory`, `Encode` stops with an error that wraps `ivy.ErrLimit`.
   The file is one part, as an image is one part, so `MaxMemory` bounds it.
2. A pixmap of the raster fallback has at most 5000 pixels on a side in
   okage D68, and at most `MaxPixels` pixels. The device lowers the
   resolution of a larger part until it fits.
3. `MaxObjects` bounds the number of pages, links, anchors and bookmarks.
   `MaxDepth` bounds the depth of the outline. The device drops a link or
   a bookmark past a limit and logs it, and the pages stay.
4. The context bounds the time. `Encode` passes it to each `Draw`, and
   stops between the pages when the context ends.

The errors:

1. A page that `Draw` draws in part, with a `*ivy.PartialError`, stays in
   the file as it is. `Encode` writes the file, then returns a
   `*ivy.PartialError` whose `Err` joins the error of each such page, with
   its number.
2. Any other error of a page stops `Encode`, before it writes a byte. The
   error names the page.
3. An error of `pdf.NewFont` for one face makes that face draw its glyphs
   as paths. It is not an error of `Encode`.
4. An error of `Document.Finish` that a limit of the version of PDF gives,
   such as `okage.ErrPDFTooLargeFloat`, wraps `ivy.ErrLimit`. Any other
   error of the writer wraps `ivy.ErrUnsupported`. Both also wrap the error
   of okage.
5. An error of `w` is returned as it is, wrapped with the context of
   `Encode`. `w` can then hold a part of the file.

pdfenc gets a fuzz test, `FuzzEncode`, as each decoder has one (D5, D41).
It turns its input into a sequence of commands of the display list, and
writes them as a file. The commands hold paths, groups, masks, images,
glyphs, links and bookmarks. The file must open with the package `pdf` of ivy.

## Tests

okage D62 measured that MuPDF draws the edges of shapes with another
anti-aliasing than okage. So a comparison with the pixels of ivy needs a
tolerance. The tests have three levels:

1. Unit tests in pdfenc, in CI. Each call of the device gets a scene of the
   display list, and the test reads the objects of the written file. A
   round trip opens the written file with the package `pdf` of ivy, renders
   each page, and compares it with `ivy.RenderImage` of the source page.
   Both sides draw with okage, so the tolerance is small: a mean difference
   of 2 at most, as `meanDiff` of the test module computes it. A scene of
   the raster fallback has its own tolerance.
2. The oracles, in the test module (D13). For each document of the corpus
   of the review, the test writes the first 3 pages and the last page. It
   runs `qpdf --check` on the file. MuPDF `mutool draw` and Poppler
   `pdftoppm` render each page at 72 dots for each inch. The test compares
   each one with `ivy.RenderImage` of the source page, within a mean
   difference of 8, as `test/pdf_test.go` does for `pdf`. okage D68 lists
   known faults of the viewers, such as the knockout groups of Poppler and
   the stroke adjustment of MuPDF. The test logs a page of such a fault, and
   the page does not fail. For a source that is a PDF file, the test also
   compares the two files through the same viewer, as okage D62 item 12
   does.
3. The text, in the test module. `pdftotext` of Poppler and `mutool draw -F
   txt` extract the text of each written page. The words of the page, which
   the test takes from the source text, must be in the extracted text, in
   order. `pdffonts` must show each font as embedded and as a subset.

The visual review gets a column for the written file through MuPDF, next to
the columns of ivy, MuPDF and Poppler of D31.

## Phases

The estimates are for one agent with a maintainer, as the phases of
`PLAN.md` are. They assume the tag of okage that holds okage D68.

| Phase | What it gives | Estimate |
| --- | --- | --- |
| 7a. The API and the device | pdfenc with `Encode`, `EncodePages` and the options, the PDF device with each call of the table, the raster fallback, `display.Drawer` and `Draw` for each page type, the raster pages as images, the metadata, the limits and the errors, the fuzz test, the round trip in CI, and the oracles of MuPDF, Poppler and qpdf. The glyphs are paths | 3 to 5 weeks |
| 7b. Fonts and text | `GlyphRun.Text` and the clusters, the source of `font.Face`, the font database with the bytes of its faces, the text of the layout, XPS, the metafiles and the charts, `fsType`, and the tests of the text | 3 to 4 weeks |
| 7c. Links and the outline | `Linker` and `Bookmarker`, the port of the rest of `weasyprint/anchors.py`, the tables of contents of the books, the links and the outline of `pdf`, `xps` and `djvu`, and the named destinations | 3 to 5 weeks |
| Later, each by a decision | the Type 3 fonts from outlines for `pdf` and `ps` (okage question 25), invisible text for the text layer of DjVu, a JPEG of a raster page or a picture that goes into the file without a new encode, pages that draw on several goroutines into recordings before the writer takes them, the page labels of a PDF file, and a writer that streams its pages (okage question 24) | not estimated |

Phase 7a gives a correct file for every format on its own. Each later phase
adds to the same file and does not change the API.

## The choices in short

| Point | Recommendation |
| --- | --- |
| The API | `pdfenc.Encode` of a document and `pdfenc.EncodePages` of pages, with the options above |
| How pdfenc gets the commands of a page | the internal interface `display.Drawer` |
| Where the device is | in pdfenc, so that iv does not link the writer (D6) |
| A part that PDF cannot hold | `Surface.DrawRaster` at 300 dots for each inch, which `WithRasterDPI` changes |
| A font whose license forbids embedding | its glyphs as paths |
| A font that is not TrueType or OpenType | paths first, then a Type 3 font from the outlines |
| Links and the outline | marks in the display list, without public API |
| The memory of the writer | `MaxMemory` bounds the bytes that the writer holds |
| A page drawn in part | the file is written, and `Encode` returns a `*ivy.PartialError` |

## The decision

On 2026-11-19 the maintainers accepted this proposal with each
recommendation of the table above. The method `Draw` of the pages of `pdf`,
`xps`, `emf` and `djvu` takes the internal `display.Device`, so it shows an
internal type in their public documentation. This decision answers question
23.

## What changes when the maintainers decide

1. `API.md` replaces its section "An encoder" with the API of this
   decision, and its item on `internal/display` names `Drawer`, `Linker`
   and `Bookmarker`.
2. `PLAN.md` gets phase 7 with the phases above.
3. The layout table of `AGENTS.md` gets `pdf/pdfenc`.
4. `go.mod` requires the tag of okage that holds okage D68.

## The build of phase 7a

The build of 2026-11-19 differs from this decision in these places:

1. `*raster.Page` is not a `display.Drawer`. `display` imports `raster`,
   so the method makes an import cycle. A move of the conversion of the
   pixmap into `display` adds the font code of okage to the imports of
   `camera`, and that code registers TIFF, which then decodes the CR2
   files in `image.Decode`. So `raster.Page` gets the method
   `Image(ctx) (*image.RGBA, error)`, and pdfenc draws that image at its own
   pixels, as the table above says.
2. The test of the corpus also opens each written file with the package
   `pdf` of ivy, within a mean difference of 8, as for the viewers. The
   limit of 2 stays for the scenes of the unit tests. On the corpus, 44 pages
   are between 2 and 8, because the glyphs are paths and a thin line
   depends on the resolution.
3. The test logs 17 documents of the corpus and does not fail them. Each
   one has a reason in the test: the viewers scale 6 DjVu scans and 4 fine
   pictures with another filter, Poppler smooths the glyph paths of 5 pages
   of small text in another way, Poppler draws the tiles of
   `poi/ppt/backgrounds.ppt` at another phase, and the stencils of a Type 3
   font of bitmaps become soft masks.
4. `StrokeDash` of okage has no method that gives the dash array, so the
   device cuts a dashed path with `Path.Dash` before it strokes it.

The test of the corpus passes on 246 documents and 527 pages. `qpdf
--check` finds no error and no warning. The mean difference from the
source page at 72 dots for each inch is 0.77 through the package `pdf` of
ivy, 0.94 through MuPDF and 2.19 through Poppler.
