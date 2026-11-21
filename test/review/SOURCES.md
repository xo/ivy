# Sources of the review corpus

`corpus.tsv` lists the documents of the visual review. The repository keeps
only the list. The review command downloads each file into a cache that git
ignores. It compares the SHA-256 and the size of each file with the list.
Nothing in this list is committed to the repository or shipped with ivy.

A row whose URL starts with `repo:` names a file that is already in this
repository. The review command reads that file in place.

The manifest holds 369 rows: 306 files to download, about 169 MB, and 63
files of this repository. 58 of the rows are assets: the style sheets, the
fonts and the images that an HTML file links to, and the figure of a TeX
document. The review command
downloads an asset and does not render it. A note of the form `password=XYZ` gives the
password of an encrypted file.

Some files are under a copyleft license, such as the GPL, the AGPL, the LGPL,
the MPL or the GFDL. The review only reads them as input. Do not commit them,
and do not copy them into ivy (D7). Each section below says which licenses
apply.

## Apache POI

- URL: https://github.com/apache/poi, folder `test-data/`.
- Commit: `8d9b40ac4cb62b1c61759c900fb422abfeac4367`.
- License: Apache-2.0, the license of the repository.
- Rows: 70.

The rows cover docx, doc, xlsx, xls, pptx, ppt, vsdx, vsd, pub, EMF, WMF and
one XPS file. POI took some test files from bug reports and from the web.
The manifest uses the license of the repository for them. The manifest
leaves out the files that a fuzzer made, because they show nothing on a page.

## pdf.js

- URL: https://github.com/mozilla/pdf.js, folder `test/pdfs/`.
- Commit: `60aefc5b51d526e39bbcbae1c6cb760763b65b0a`.
- License: Apache-2.0, the license of the repository.
- Rows: 32.

The manifest takes only files that the repository holds, and no file that
the test manifest of pdf.js links to another site. Many files come from bug
reports. `freeculture.pdf` is a book under CC BY-NC 1.0. The password of
`issue3371.pdf` comes from `test/test_manifest.json` of pdf.js.

## LibreOffice

- URL: https://github.com/LibreOffice/core, the folders `sw/qa`, `sc/qa`,
  `sd/qa` and `chart2/qa`.
- Commit: `7b2a56bf86c19b48ffc46873785bd0e6fefd96cd`.
- License: MPL-2.0, the license of the repository.
- Rows: 40.

The rows cover odt, ods, odp, odg, rtf, docx, xlsx, pptx and doc. MPL-2.0 is
a copyleft license. The review downloads these files, and the repository
never holds them.

## hayro

- URL: https://github.com/LaurenzV/hayro, folder `tests/pdfs/custom/`.
- Commit: `61f50b42a7f7b15764780b9ba534cba91488bb28`, the commit that ivy
  ports (D14).
- License: Apache-2.0 OR MIT, in `tests/LICENSE-APACHE` and
  `tests/LICENSE-MIT`.
- Rows: 18. Of these, 13 are downloads and 5 are copies in this repository.

The passwords come from `tests/manifest_custom.json` of hayro. The manifest
leaves out the files whose names start with `pdftc_`. They come from a corpus
of PDF files from the web, and their license is not known. Most of the other
PDF files of hayro are on a server of its own, not in the repository, so the
manifest leaves them out.

## EPUB 3 Samples

- URL: https://github.com/IDPF/epub3-samples.
- Version: the release `20230704`. The URLs are those of the release assets.
- License: CC-BY-SA-3.0, as `README.md` states, unless the table of samples
  at https://idpf.github.io/epub3-samples/30/samples.html says otherwise.
- Rows: 8.

Three files have another license. `linear-algebra.epub` is under the GNU
FDL 1.2, as the table says. `childrens-literature.epub` is in the public
domain, as its package document says. `jlreq-in-japanese.epub` is a W3C
document under the W3C document license.

## Project Gutenberg

- URL: https://gutenberg.pglaf.org/cache/epub/, a mirror of
  https://www.gutenberg.org. The main site refused the downloads.
- Version: the files of 2026-11-17.
- License: the texts are in the public domain in the United States.
- Rows: 8.

These URLs are not pinned. Project Gutenberg makes its files again from
time to time. If a hash no longer matches, download the file again and
update its row. The rows cover EPUB 3, MOBI, KF8 in a `.mobi` file, HTML and
plain text, in English and in Chinese.

## Aspose.Words for .NET

- URL: https://github.com/aspose-words/Aspose.Words-for-.NET, folder
  `Examples/Data/`.
- Commit: `0dee8aa1aeedf25361244488643dfa5ba4344c10`.
- License: MIT, the license of the repository.
- Rows: 7.

The manifest leaves out the FictionBook and AZW3 files of this folder. They
hold a book that a publisher sells. It also leaves out the MOBI file, which
says that it is a demonstration of another company. The origin of the two CHM
files is not clear, so the manifest leaves them out. `Microsoft Visio drawing.vsd` is a Photoshop file,
so the manifest leaves it out too.

## Ghostscript

- URL: https://github.com/ArtifexSoftware/ghostpdl, folder `examples/`.
- Commit: `5f79c7a87fdeae5d3a70eb9cf9f5bd47d996fa0d`.
- License: AGPL-3.0-or-later. `LICENSE` puts the folder `examples` in GPL
  Ghostscript.
- Rows: 7.

Hard rule 6 applies to these files. The review can read them as input, but
nobody must copy them, or code from Ghostscript, into ivy.

## Windows driver samples

- URL: https://github.com/microsoft/Windows-driver-samples, the folders
  `print/SampleXPS` and `print/SampleOpenXPS`.
- Commit: `2e237713e298495b141560a5e7c76d6b1efa0153`. A later commit deleted
  these folders.
- License: the end-user license agreement of the Microsoft XPS test files,
  in `XPSTestFiles_EndUserLicenseAgreement.txt`. It allows copies for the
  development and the test of software.
- Rows: 6.

The files are the QualityLogic MinBar test pages in XPS and in OpenXPS. The
manifest leaves out `mb03`, because it is 14 MB.

## dejapdf

- URL: https://github.com/sergev/dejapdf, folder `testdata/corpus/`.
- Commit: `2b6ca0c1123cc45f868cca3be99dd8123d5cdc6f`.
- License: MIT, in `LICENSE`.
- Rows: 5.

The files are DjVu documents that `testdata/mkcorpus.sh` made. The manifest
leaves out the indirect document, because it is several files.

## msoffcrypto-tool

- URL: https://github.com/nolze/msoffcrypto-tool, folder `tests/inputs/`.
- Commit: `bdb71ea5dfbdee2f3cd86f8852e74be7a6f15667`.
- License: MIT, in `LICENSE`.
- Rows: 5. Of these, 3 are downloads and 2 are copies in this repository.

The password of each file is `Password1234_`, except the password of
`xor_password_123456789012345.xls`, which is `123456789012345`.

## psd-tools

- URL: https://github.com/psd-tools/psd-tools, folder `tests/psd_files/`.
- Commit: `faac76bfa4532f23bc8674c01ce31f4eb866d182`.
- License: MIT, the license of the repository.
- Rows: 4.

The files test CMYK with spot channels, blend modes, layer effects and
channels of 32 bits.

## Internet Archive

- URL: https://archive.org/download/.
- Version: the files of 2026-11-17.
- License: public domain. Each item has the status `NOT_IN_COPYRIGHT`.
- Rows: 3.

The files are DjVu scans of books from 1829, 1893 and 1899, which the
Internet Archive made from scans of Google Books. These URLs are not pinned.
The Internet Archive can make the DjVu file of an item again.

## Sources without a license

The maintainers allowed these sources on 2026-11-17. Their repositories give
no license, so the review downloads their files and reads them, and the
repository must never hold them. The license column of each row says "none
(no license; download only)".

- CourtBouillon/weasyprint-samples, commit
  `6dc675b05a5cba59576dfaa8881ea6ccdccb13cc`. The book, the report, the
  invoice, the letter and the poster of WeasyPrint, with their style
  sheets, fonts and images as assets. 5 HTML files and 57 assets.
- geometer/FBReaderJ, commit `e83aec9f94084aa59d39e33876bdb6fdc275c95e`.
  FictionBook test files, and help files in Japanese, Thai, Georgian, Greek,
  Chinese and Vietnamese. 11 files.
- datasets/country-codes, commit `2d84034a0cae19ea5444b78c2131304a5c7f4bb9`.
  A CSV file of 56 columns, with names in many scripts. 1 file.

## Apache Tika

- URL: https://github.com/apache/tika, folder
  `tika-parsers/tika-parsers-standard/tika-parsers-standard-modules/tika-parser-apple-module/src/test/resources/test-documents/`.
- Commit: `3756dbd60d2e8552fd6c721d13f651cab07a5d12`.
- License: Apache-2.0, in `LICENSE.txt`. `NOTICE.txt` says nothing about
  the test documents.
- Rows: 4.

The rows cover Pages, Keynote and Numbers. Three files have the form of
2013 and later: a zip of `Index/*.iwa` with `preview.jpg`. The Keynote file
of 2018 is a zip of a folder `.key` with `Index.zip`, and it holds no
preview image. The manifest leaves out the files of iWork '09, because the
applications of Apple do not open them now.

## TeX and LaTeX

The rows of the kind `tex` are the LaTeX and plain TeX documents of the
package `latex` (D62). A document reads the files of its folder, such as
its figures, so an asset of a TeX document has a name in the folder of the
document. Six rows are files of this repository, see "Files of this
repository".

| Source | Commit | License | Files |
| --- | --- | --- | --- |
| https://github.com/latex3/latex2e | `db6df6380f8660758d26b9666d0e70600a3d7be9` | LPPL-1.3c, the license of the repository and of the headers of the files | `base/sample2e.tex`, `required/amsmath/testmath.tex`, `base/doc/cfgguide.tex`, `required/graphics/grfguide.tex`, `required/graphics/rotex.tex` and its figure `required/graphics/cat.eps` |
| https://github.com/josephwright/beamer | `e9e99b8fdc3084d9f118a761264be37184899e4b` | LPPL-1.3c or GFDL-1.3-or-later, the two licenses of the folder `doc` in `LICENSE.md` | `doc/examples/a-conference-talk/beamerexample-conference-talk.tex` |
| https://github.com/janosh/diagrams | `3bb036835ba754c46ce1ac5b694fa11db5d89b43` | MIT, in `license` | `assets/multilayer-perceptron/multilayer-perceptron.tex` |
| CTAN, `systems/knuth/dist/lib` | none, the files of Knuth do not change | Knuth, the license that CTAN gives the package knuth-lib | `story.tex` |

The manifest leaves out `subeqn.tex` of amsmath, whose license allows only
copies without changes or with a new name, and `lppl.tex`, the text of the
license itself.

## Other sources

Each of these sources gives one row:

| Source | Commit | License | File |
| --- | --- | --- | --- |
| https://github.com/phiresky/ripgrep-all | `fce1a28683142b3dbd6ea2f7b719d89c90aee227` | AGPL-3.0-or-later, in `LICENSE.md`. The poem itself is in the public domain | `exampledir/wasteland.fb2` |
| https://github.com/onekey-sec/unblob | `d057b3a0435c61b91cc03bed1933499c14f06735` | LGPL-2.1-or-later, the license of 7-Zip, whose help file it is | `_7zip.chm`, from Git LFS through `media.githubusercontent.com` |
| https://github.com/mdn/content | `e976b4bf6b9e0c68b4321904560df47804d8cc48` | CC-BY-SA-2.5, in `LICENSE.md` | `files/en-us/web/html/reference/elements/table/index.md` |
| https://github.com/yuin/goldmark | `d473f9516b6bba29f524dc0a1798890b8263674b` | MIT, the commit that ivy copies | `README.md` |
| https://github.com/datasets/world-cities | `30b0719b393270002b578e98120f7f230f200903` | CC-BY-4.0 for the data of GeoNames, as `README.md` states | `data/world-cities.csv` |

## Files of this repository

The rows whose URL starts with `repo:` name 63 files of this repository. The
`README.md` of each `testdata` folder says where its files come from. The
source column names the project that made the file, or `xo/ivy` with the
commit `81c4da1a35591008a63a05cb42fea70c31dc11c0` for a file that the
maintainers made.

## Formats without a good sample

The manifest holds no file of the kinds azw4, ora, kra and cbz. No source with
a clear license was easy to find for them. It holds one CHM file and one AZW3
file, the last from this repository.
