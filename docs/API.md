# The API of ivy

This document holds the design of the public API of ivy and the layout of
its packages, before any code (D3). D6, D10 and D11 hold what the maintainers
decided. The names below are the expected names. If phase 1 finds a reason to
change one, the change is a decision first.

## How the design was made

The plan held a first sketch. Three outside models reviewed it on
2026-11-16: Gemini 3.1 Pro, GPT-5.6 Luna and DeepSeek V4 Flash. GPT-6 Astra
and DeepSeek V4 Pro did not answer in time. The maintainers then chose
between the answers:

| Point | The models | The choice |
| --- | --- | --- |
| The shape of a document | All three: a small `Document`, with capabilities that a program finds by a type assertion | A small `Document` with `Pager`, `Previewer` and `Describer` |
| The number of pages | All three: lazy, with a context and an error | `PageCount(ctx)`, and `ErrPageRange` past the end |
| The size of a preview | Gemini: list the previews. The other two: ask for a size | A list of previews that decode on demand, and a helper that picks the best one |
| The options of a format | All three: an options struct for each format, around a shared configuration | Functional options for each format, around a shared `ivy.Config`, as okage D20 does |
| The shared layers | Gemini: internal. GPT-5.6: internal until a program needs one. DeepSeek: export only the recorder | Internal |
| A page drawn in part | All three: a typed error | `*ivy.PartialError` |
| Concurrency | Gemini: safe. DeepSeek: not safe unless a format says so | A document is safe for concurrent use |
| The input | DeepSeek: a source with its size | `ivy.Source` |
| Generics | All three: not in the public API | Inside the packages only (D9). The API uses the iterators of `iter` |

Two claims of the models were wrong. GPT-5.6 said that a constant of the type
`ivy.Error` cannot be an error. It can, when the type has the method `Error`,
and okage D21 does this. DeepSeek imported a package `github.com/xo/ivy/okage`,
which does not exist.

## The layout of the packages

```text
github.com/xo/ivy                the root package: the document API (D11)
├── pdf/                         PDF, and Illustrator compatible with PDF (D14)
│   └── pdfenc/                  reserved for the PDF writer (D1)
├── xps/  ps/  emf/  djvu/       the other page descriptions. ps holds EPS
├── psd/  ora/                   the images with layers. ora also opens Krita
├── epub/  mobi/  fb2/  chm/     the ebooks. mobi holds MOBI, PRC, AZW, AZW3, AZW4
├── html/  text/  markdown/      HTML and HTMLZ, text and TXTZ, Markdown
├── comic/                       CBZ and CBT, with the standard library
│   ├── cbr/                     a module of its own, for RAR (D6)
│   └── cb7/                     a module of its own, for 7z (D6)
├── docx/  xlsx/  pptx/          Office Open XML
├── odf/  rtf/                   OpenDocument, RTF
├── msdoc/  msxls/  msppt/       the binary Office formats
├── dmg/  ipa/  apk/  msi/       the containers with an icon or a preview
├── camera/  iwork/              camera RAW, iWork
├── drm/                         a module of its own, for DRM that needs a key (D12)
├── internal/
│   ├── display/                 the display list and the okage device (D10)
│   ├── font/                    the fonts (D10)
│   ├── layout/                  the port of WeasyPrint (D15)
│   ├── cfb/  offcrypto/  ...    the code that more than one format shares
│   └── jbig2/  jpx/  ccitt/ ... the codecs of the port of hayro (D14)
└── test/                        a module of its own: the oracles (D13)
```

Each other format of [`FORMATS.md`](FORMATS.md) gets a package named after
its main extension, such as `deb`, `rpm`, `appimage` and `iso`. Code that only
one format uses stays inside that format, such as `pdf/internal/syntax` or the
IW44 wavelets in `djvu`.

The package `html` of ivy has the same name as the package `html` of okage. A
program that imports both renames one.

## The root package

The root package imports the standard library and okage only (D6).

### The input

```go
// Source is the input of a document. *bytes.Reader, *strings.Reader and
// *io.SectionReader satisfy it. For an *os.File, use io.NewSectionReader.
//
// The caller owns the source. A document reads it until Close, and Close
// does not close it.
type Source interface {
	io.ReaderAt
	Size() int64
}
```

`io.ReaderAt` fits the formats that seek, such as zip, PDF and the Compound
File Binary format. A format that reads in order, such as PSD, reads the
source from its start with an `io.SectionReader`.

### Documents and capabilities

```go
// Document is an open file. Each format returns its own type, and the type
// also satisfies the capabilities that the format has. A Document is safe for
// concurrent use.
type Document interface {
	// Close releases the caches of the document. It does not close the
	// source. A second call does nothing.
	Close() error
}

// Pager is a document with pages.
type Pager interface {
	// PageCount returns the number of pages. For a flow of text, such as an
	// EPUB, it lays out the whole document on its first call.
	PageCount(ctx context.Context) (int, error)
	// Page returns page n, from 0. It returns ErrPageRange past the last
	// page. For a flow of text, it lays out only as far as page n.
	Page(ctx context.Context, n int) (Page, error)
}

// Previewer is a document that carries images of its content (D11).
type Previewer interface {
	// Previews lists the previews. Each one decodes only on demand.
	Previews(ctx context.Context) iter.Seq2[Preview, error]
}

// Describer is a document with a title, authors and dates.
type Describer interface {
	Describe(ctx context.Context) (Info, error)
}
```

A program finds a capability with a type assertion, such as
`doc.(ivy.Pager)`. Each format package asserts its own capabilities at
compile time, such as `var _ ivy.Pager = (*Document)(nil)`. A capability that
comes later, such as an outline, is a new interface, and no existing format
breaks.

| Kind of format | `Pager` | `Previewer` | `Describer` |
| --- | --- | --- | --- |
| A page description, such as PDF or XPS | yes | where the file holds thumbnails | yes |
| A flow of text, such as EPUB or Word | yes, laid out lazily | the cover or the thumbnail | yes |
| An image with layers, such as PSD | yes, one page | the thumbnail | where the file holds it |
| A container, such as DMG or APK | no | yes, the icons | the name |

### Pages

```go
// Page is one page of a document. Render is safe for concurrent use when
// each call has its own pixmap.
type Page interface {
	// Size returns the size of the page in points, 1/72 of an inch, after
	// its rotation.
	Size() (width, height float32)
	// Render draws the page onto dst. t maps the points of the page onto
	// the pixels of dst. Render draws over what dst holds, so a program
	// fills the background first. If Render draws the page only in part, it
	// returns a *PartialError, and dst holds the part.
	Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error
}
```

### Previews and information

```go
// Preview is one image that a document carries.
type Preview struct {
	Kind PreviewKind
	// Size is the size in pixels, or zero if the file does not say it
	// before the decode.
	Size image.Point
	// Decode decodes the image.
	Decode func(ctx context.Context) (image.Image, error)
}

// PreviewKind is the kind of a preview.
type PreviewKind int

const (
	Thumbnail PreviewKind = iota // a small image of a page
	Icon                         // the icon of an application or a volume
	Cover                        // the cover of a book
	Background                   // the background of a disk image
)

// Info is what a document says about itself. A field is empty when the
// document does not say it.
type Info struct {
	Title    string
	Authors  []string
	Subject  string
	Language string
	Created  time.Time
	Modified time.Time
	Producer string
}
```

### Helpers

```go
// Pages iterates over the pages of p, in order, until the last page.
func Pages(ctx context.Context, p Pager) iter.Seq2[Page, error]

// BestPreview decodes the preview of d that is closest in size to want, and
// prefers one that is larger. It returns ErrNoPreview if d is not a
// Previewer or lists no preview.
func BestPreview(ctx context.Context, d Document, want image.Point) (image.Image, error)

// RenderImage renders p at dpi dots per inch over bg, and returns the image.
// A nil bg leaves the background transparent (D11).
func RenderImage(ctx context.Context, p Page, dpi float32, bg color.Color) (image.Image, error)
```

### The configuration

```go
// Config is the configuration that every format shares. A program gives it
// to a format with the option WithConfig of the format. The zero Config is
// valid.
type Config struct {
	// Password opens a file that a password protects (D12).
	Password string
	// PasswordFunc asks for a password. ivy calls it once for each attempt,
	// from attempt 1, while the password is wrong. It stops when the
	// function returns an error.
	PasswordFunc func(ctx context.Context, attempt int) (string, error)
	// Decrypt decodes a resource that DRM with a key protects. Only the
	// module drm gives such a function (D12). The format passes the
	// resource and its license, and it never decodes the DRM itself.
	Decrypt func(ctx context.Context, scheme string, license, data []byte) ([]byte, error)
	// Logger receives the log of the format. A nil Logger logs nothing.
	Logger *slog.Logger
	// Limits bound the work on one document.
	Limits Limits
	// Cache shares the fonts and the ICC profiles between documents. A nil
	// Cache gives each document a cache of its own.
	Cache *Cache
}

// Log returns the logger of the configuration, or a logger that discards
// each record when Logger is nil.
func (cfg Config) Log() *slog.Logger

// Limits bound the work on one document, because each file is input from a
// stranger (D5). A field that is zero or less takes its default. The context
// bounds the time.
type Limits struct {
	MaxMemory  int64 // the bytes of one decoded part, such as an image or an entry
	MaxPixels  int64 // the pixels of the largest image that it decodes
	MaxObjects int   // the objects, entries or records that it reads
	MaxDepth   int   // the depth of nesting that it follows
}

// The defaults of the limits.
const (
	DefaultMaxMemory  = 512 << 20
	DefaultMaxPixels  = 1 << 28
	DefaultMaxObjects = 1 << 20
	DefaultMaxDepth   = 64
)

// WithDefaults returns the limits, with the default in each field that is
// zero or less.
func (lim Limits) WithDefaults() Limits

// Cache holds the fonts of the system, the fonts that documents embed, and
// the transforms of ICC profiles. It is safe for concurrent use. There is no
// global cache. The formats of phase 1 use no fonts, so it is empty until
// phase 2.
type Cache struct{ /* unexported */ }

// NewCache returns an empty cache.
func NewCache() *Cache
```

### Errors

The sentinel errors are constants of the type `ivy.Error`, as okage D21
does. Every format returns them, or wraps them with `%w`, and declares no
sentinel of its own.

```go
// Error is a sentinel error of ivy.
type Error string

func (err Error) Error() string { return string(err) }

const (
	ErrPassword    Error = "password is missing or wrong"
	ErrDRM         Error = "file holds drm that ivy does not decode"
	ErrNoPreview   Error = "file holds no preview"
	ErrPageRange   Error = "page out of range"
	ErrFormat      Error = "malformed file"
	ErrUnsupported Error = "feature not supported"
	ErrLimit       Error = "limit exceeded"
)

// PartialError reports a page that Render drew in part. Err is the cause.
type PartialError struct {
	Err error
}

func (err *PartialError) Error() string { return "partial render: " + err.Err.Error() }
func (err *PartialError) Unwrap() error { return err.Err }
```

### A format for a registry

The root package holds no registry (D6). A program that keeps its own
registry, as iv does, takes the `Format` of each package that it imports:

```go
// Format describes a format package for the registry of a program.
type Format struct {
	Name  string
	Mimes []string
	Exts  []string
	// Open opens a document with the default options of the format and
	// the configuration cfg.
	Open func(ctx context.Context, src Source, cfg Config) (Document, error)
}
```

## A format package

Each format package has the same form. The options are functional options
of the package, and `WithConfig` gives the shared part:

```go
package pdf

// Option is an option of Open.
type Option func(*options)

// WithConfig sets the configuration that every format shares.
func WithConfig(cfg ivy.Config) Option

// WithAnnotations draws the annotations of each page. It is on by default.
func WithAnnotations(on bool) Option

// Open opens a PDF file.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error)

// Document is an open PDF file.
type Document struct{ /* unexported */ }

func (d *Document) Close() error
func (d *Document) PageCount(ctx context.Context) (int, error)
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error)
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error]
func (d *Document) Describe(ctx context.Context) (ivy.Info, error)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name:  "pdf",
	Mimes: []string{"application/pdf"},
	Exts:  []string{"pdf"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}
```

The form of the options repeats in each package. A generic helper in
`internal/` can hold the shared code (D9), but each package keeps its own
`Option` type, so that an option of one format does not compile for another.

A format that holds one image, such as `psd`, also has `Decode` and
`DecodeConfig`, and registers with the package `image` from `init` (D11). A
format can add a method of its own to its `Document`, such as `Mime` of `odf`
and `ora`, which returns the mime type that the file declares.

A container, such as `dmg`, has the same `Open`, and its `Document` is a
`Previewer` and a `Describer`, but not a `Pager`.

## An encoder

No encoder is planned yet (D1). When one comes, it has this form, in its own
package:

```go
package pdfenc

// Encode writes the pages to w as a PDF file.
func Encode(ctx context.Context, w io.Writer, pages iter.Seq2[ivy.Page, error], opts ...Option) error
```

## The internal layers

The layers of D10 are internal, so that their API can change while the
formats grow. Each format and each encoder of the module can import them. If
a program needs one, the maintainers decide to export it.

1. `internal/display` holds the interface `Device`, with the commands of the
   display list (D10). It holds a `List`, which records the commands and
   replays them onto another device, and the device that draws with okage.
   `xps` draws through it, and the port of hayro and the layout will. A path
   of okage changes in place when it transforms, so a shared path, such as
   the outline of a glyph, is cloned first with `display.Clone`.
2. `internal/font` holds the fonts. It reads the fonts of the system and the
   web fonts with okage, the TrueType, OpenType and CFF fonts with go-text,
   and Type 1 and Type 3 with the port of hayro-font. It keeps its fonts in
   the `ivy.Cache`.
3. `internal/layout` holds the port of WeasyPrint (D15). It takes a tree of
   HTML from the parser of okage, the style sheets and an `fs.FS` of the
   resources, such as the files in an EPUB. It returns a document that is a
   `Pager`, and it lays out the pages as a program asks for them.

## How iv uses the API

iv finds the type of a file with magic, and keeps a registry of the `Format`
of each package that it imports:

```go
src := io.NewSectionReader(f, 0, size)
doc, err := format.Open(ctx, src, ivy.Config{
	PasswordFunc: prompt,
	Cache:        cache,
})
if err != nil {
	return err
}
defer doc.Close()
if pager, ok := doc.(ivy.Pager); ok {
	page, err := pager.Page(ctx, n)
	if err != nil {
		return err
	}
	return show(ivy.RenderImage(ctx, page, dpi, bg))
}
return show(ivy.BestPreview(ctx, doc, termSize))
```

A program can open a document once and render its pages in the background,
because a document is safe for concurrent use.
