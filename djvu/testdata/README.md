# Test files of djvu

The DjVu files come from the folder `testdata/corpus` of
[sergev/dejapdf](https://github.com/sergev/dejapdf), under the MIT license,
at the commit `2b6ca0c1123cc45f868cca3be99dd8123d5cdc6f`. The license is in
`LICENSE-dejapdf`. `iw44.sha256`, `masks.sha256`, `bundled.dsed` and
`pdf2djvu.dsed` come from the same folder. The two `.sha256` files hold the
hashes of the layers and of the masks that ddjvu of DjVuLibre decodes from
the files. A `.dsed` file holds the output of `djvused -e output-all` for
the DjVu file of the same name. The tests of `djvu/internal/doc` compare the
copy with them. They compare the text layer of each page with its `set-txt`
block.

| File | What it holds |
| --- | --- |
| `bilevel.djvu`, `bilevel-lossy.djvu` | a page of text, a JB2 mask |
| `photo.djvu`, `photo-half.djvu`, `gray.djvu` | a picture, an IW44 background in color, in color with chroma at half the size, and in gray |
| `compound.djvu` | a JB2 mask over an IW44 background, with IW44 foreground colors |
| `palette.djvu` | a JB2 mask with a palette of colors for each blit |
| `jpeg.djvu` | a JB2 mask with a JPEG background and JPEG foreground colors |
| `mmr.djvu`, `mmr-striped.djvu` | a G4 mask, in one stream and in stripes |
| `bundled.djvu` | a bundled document of 4 pages, with shared annotations and a title |
| `pdf2djvu.djvu` | a bundled document of 4 pages that pdf2djvu wrote, with its metadata |
| `shared-dict.djvu` | a bundled document of 2 pages that share a JB2 dictionary |
| `indirect/index.djvu` | the index of an indirect document, without the files that it names. It opens only with `WithDir` (D38) |

`thumbnails.djvu` is `shared-dict.djvu` with a thumbnail for each page. The
command `djvused -e 'set-thumbnails 64' -s thumbnails.djvu` of DjVuLibre
added them.

`text.djvu` is `bilevel.djvu` with a text layer that holds each kind of
zone: page, column, region, paragraph, line, word and character. Its words
are English, German and Russian. The command `djvused -e 'select 1;
set-txt FILE' -s text.djvu` of DjVuLibre added the text of the `set-txt`
block of `text.dsed`, and `djvused -e output-all text.djvu` then wrote
`text.dsed`.

`text.sha256` holds a hash of the text layer of each page of the three books
of `archive-org/djvu` in the corpus of the review, which are too large for
this folder. Each line holds the SHA-256 of the text of one page, as
`djvused print-txt` prints it, then the file and the id of the page.
`TestTextBooks` of `djvu/internal/doc` reads the books from the folder that
`IVY_DJVU_BOOKS` names. On 2026-11-21 the text of each page is the text of
djvused, except two pages of `memoirupontopog00smygoog.djvu`:
`memoirupontopog00smygoog_0080.djvu` and `memoirupontopog00smygoog_0084.djvu`.
Their text layers hold a page zone inside the page zone. djvused stops with
"Text layer hierarchy is corrupt", and ivy reads the words of both pages.
