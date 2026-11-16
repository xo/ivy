# D11. The document API is a small document with capabilities

Status: Decided.

The root package `ivy` holds the document API that every format shares.
[`docs/API.md`](../API.md) holds its design and the expected names. This
decision holds what the maintainers chose.

## Documents

1. A format returns its own type of document. The interface
   `ivy.Document` asks only for `Close`.
2. A document has capabilities, which a program finds with a type
   assertion: `Pager` for pages, `Previewer` for the images that the file
   carries, and `Describer` for the title, the authors and the dates. A new
   capability is a new interface, so no format breaks.
3. The number of pages is lazy. `PageCount` takes a context and returns an
   error, and it can lay out a whole book. `Page` returns `ErrPageRange` past
   the last page, so a program can read pages in order without the count.
4. A document is safe for concurrent use. `Render` is safe for concurrent
   use when each call has its own pixmap.
5. The input is an `ivy.Source`, an `io.ReaderAt` with its size. The caller
   owns it, and `Close` does not close it.

## Pages

1. A page has a size in points, 1/72 of an inch.
2. A page renders onto an `*okage.Pixmap`, with an `okage.Transform` that maps
   the points of the page onto the pixels of the pixmap. A program can draw
   several pages onto one pixmap, or draw a page at any scale and position.
3. A page that ivy draws only in part returns a `*PartialError`, and the
   pixmap holds the part (D5).
4. A helper in the root package renders a page at a resolution in dots per
   inch, over a background, and returns an `image.Image`, for a program that
   does not use okage. A pixmap works with the package `image` (okage D19).

## Previews

Many formats carry a small image of their content next to the content:

- An OpenDocument file always holds `Thumbnails/thumbnail.png`.
- An Office Open XML file can hold `docProps/thumbnail.jpeg`.
- An EPS file can hold a TIFF or a WMF image in its DOS header, or an EPSI
  preview in its comments.
- An Apple iWork file holds a preview in `QuickLook/` or in its root.
- An OpenRaster or a Krita file holds `mergedimage.png`.
- A camera RAW file holds a full size JPEG.
- A disk image, an application bundle or an Android package holds icons in
  several sizes.

A `Previewer` lists its previews, and each one decodes only on demand. A
helper in the root package decodes the preview that is closest to a wanted
size, or returns `ErrNoPreview`. A container, such as a disk image, is a
`Previewer` and not a `Pager`. For a format that ivy can render, a program
chooses between the preview, which is fast, and the rendering, which is
complete.

The previews come first in the phases. They give a useful result for many
formats in weeks, before the large renderers are ready (D5).

## Options

1. Each format package has its own functional options, whose names start
   with `With`, as okage D20 does. An option of one format does not compile
   for another.
2. The configuration that every format shares, such as the password, the
   logger, the limits and the cache, is the struct `ivy.Config`. A format
   takes it with its option `WithConfig`.
3. The cache of fonts and of ICC profiles is a value that a program makes and
   shares between documents. ivy has no global cache.
4. Each format package exports a `Format` value, with its name, its types and
   a function that opens a document with an `ivy.Config`. A program builds its
   own registry from them. ivy has no registry (D6).

## Errors

The sentinel errors are constants of the type `ivy.Error`, as okage D21
does: `ErrPassword`, `ErrDRM`, `ErrNoPreview`, `ErrPageRange`, `ErrFormat`,
`ErrUnsupported` and `ErrLimit`. Every format returns them, or wraps them
with `%w`, and declares no sentinel of its own.

## Formats that hold one image

A decoder of a format that holds one image, such as PSD, OpenRaster, Krita
or the preview of a camera RAW file, also exports `Decode` and
`DecodeConfig`, and it calls `image.RegisterFormat` from `init`, as
`image/png` does. `image.Decode` then reads that format in each program that
imports the package. iv uses this registry to show the images inside an
archive. The package registers only its own format, so D6 still holds.

## Generics

Generics stay inside the packages (D9). The public API uses the iterators of
`iter`, and no other generic type.
