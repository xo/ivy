# Test files of djvu

The DjVu files come from the folder `testdata/corpus` of
[sergev/dejapdf](https://github.com/sergev/dejapdf), under the MIT license,
at the commit `2b6ca0c1123cc45f868cca3be99dd8123d5cdc6f`. The license is in
`LICENSE-dejapdf`. `iw44.sha256` and `masks.sha256` come from the same
folder. They hold the hashes of the layers and of the masks that ddjvu of
DjVuLibre decodes from the files, and the tests of `djvu/internal/doc`
compare the copy with them.

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
