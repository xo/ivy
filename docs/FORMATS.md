# The formats

This document records a survey of the formats that ivy can support, the way
to support each one, its source and its cost. The survey is from 2026-11-16.
It is input for the plan. It is not a decision. The phases in
[`PLAN.md`](PLAN.md) take their order from it, and the maintainers decide
each phase.

## How the survey was made

The survey started from the formats that iv sends to an outside program
today, and from the formats that the maintainers named: the input formats of
`ebook-convert` of calibre, XPS, the documents of LibreOffice and Microsoft
Office, PSD, Adobe Illustrator, PostScript, EPS, PDF, the encrypted forms of
these, and containers that carry an icon or a preview, such as a DMG file.

Three outside models gave a list of formats, a way to support each one, a
source to port and an estimate:

1. Gemini 3.1 Pro, with deep reasoning.
2. GPT-5.6 Luna of OpenAI. GPT-6 Astra, the model that the maintainers asked
   for, did not answer in time on four attempts.
3. DeepSeek V4 Flash. DeepSeek V4 Pro, the model that the maintainers asked
   for, did not answer in time, or spent all of its budget on reasoning.

Each model got the same constraints: pure Go, permissive sources only, okage
and go-text for drawing, and a rendering that is good enough for a quick
review (D5). Each source that a model named was then examined on GitHub, for
its license, size and activity. The examination found two errors. GPT-5.6
named a repository for WMF that does not exist. DeepSeek named libdmg-hfsplus
as MIT, and it is GPL-3.0. The tables below hold only the sources that the
examination found.

## The estimates

The estimates are in months of work for one engineer who uses coding agents,
at the quality of D5. The models disagree by up to six times:

| Model | Total, in months | Its view |
| --- | --- | --- |
| Gemini 3.1 Pro | 14 to 18 | Previews and simple layout. It drops a full interpreter of PostScript, the layout of the binary Office formats, and DjVu |
| GPT-5.6 Luna | 63 to 100 | A limited layout for each format, and ports of most of Apache POI |
| DeepSeek V4 Flash | about 98 | The same scope as GPT-5.6, with more for PostScript and ebooks |
| This survey | 41 to 59 for the core, and 18 to 23 more for the long tail | Previews first, then the ports that have a source, then layout through one shared engine (D10) |

The core is every format in the tables below that is not marked "later" or
"only if a user asks". The long tail is the PostScript interpreter, the
older Illustrator files, GIMP, Microsoft Reader, Sony BBeB, the rare ebooks
and a layout for the binary Office formats.

The column "Survey" in each table below is the estimate of this survey. It
assumes the four shared layers of D10, and it counts each layer once, in the
section "The shared layers". The difference between the models comes from
three choices:

1. Gemini takes the preview or the text of a format where the other two
   build a renderer.
2. GPT-5.6 and DeepSeek count a layout for each Office format. This survey
   counts one layout engine, which the flows of text share.
3. No model knew that a pure Go decoder of DjVu exists, or that okage
   already holds parsers of XML, CSS and HTML. Both cut the cost (D7).

## The shared layers

| Layer | What it holds | Source (license) | Survey |
| --- | --- | --- | --- |
| Document API and options | `ivy.Document`, `ivy.Page`, the options and the sentinel errors | none | 0.5 |
| Display list and okage device | The commands of a page description, and a device that draws them with okage | none. The `Device` of [hayro-interpret][hayro] is the model | 1 |
| Layout of HTML and CSS | A flow of text onto pages | A port of the layout of [WeasyPrint][weasyprint] (BSD-3-Clause, Python), on the parsers and the text API of okage (D7, D15). The work on the parsers of okage is not counted here | 4 to 6 |
| Fonts | Embedded fonts, substitutes for named fonts, Type 1 and Type 3 | go-text for TrueType, OpenType and CFF. [hayro-font][hayro] (Apache-2.0 or MIT) for Type 1 | 1 to 1.5 |
| Containers | Zip with its encryption, the Compound File Binary format of Microsoft, PalmDB, and the plist of Apple | `archive/zip`. [richardlehane/mscfb][mscfb] (Apache-2.0, pure Go) | 0.5 to 1 |
| Encryption | MS-OFFCRYPTO, OpenDocument, zip | the specifications, and [nolze/msoffcrypto-tool][msoffcrypto] (MIT, Python) as a reference. PDF takes its encryption from hayro | 1 to 1.5 |

WeasyPrint paginates with `@page`, so an ebook or a Word document comes out
as pages. [benoitkugler/webrender][webrender] is a Go port of WeasyPrint, and
ivy does not use it, because it requires a port of Pango under the LGPL-2.1
(D15).

## Page descriptions

| Format | Extensions | iv today | Way at review quality | Source (license) | Gemini | GPT-5.6 | DeepSeek | Survey |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| PDF, with encryption | `pdf` | libvips, then `pdftoppm` | A port of hayro onto the display list | [hayro][hayro] (Apache-2.0 or MIT, Rust). Its crates syntax, interpret, font, cmap, ccitt, jbig2 and jpeg2000, and [moxcms][moxcms] (BSD-3-Clause) for ICC profiles (D14). 1,600 test files | 6 to 7.5 | 12 to 18 | 8 | 6.5 to 10 |
| Adobe Illustrator, compatible with PDF | `ai` | none | The PDF reader. Illustrator writes a PDF page next to its own data | the PDF reader | 0.5 | in PostScript | in PostScript | 0.25 |
| PostScript and EPS | `ps`, `eps`, `epsi`, `epsf` | none | First the preview of EPS: the TIFF or WMF image of the DOS header, or the EPSI comments. Then an interpreter of Level 2 onto the display list | [seehuhn/go-postscript][go-postscript] (BSD-3-Clause, "rudimentary"), [llgcode/ps][llgcode-ps] (BSD-2-Clause, from 2021). No full permissive source | 0.5, preview only | 6 to 10 | 15 | 0.5 for the preview, 6 to 10 for the interpreter |
| Adobe Illustrator, older than version 9 | `ai` | none | The PostScript interpreter, with the procedure sets of Illustrator | the PostScript interpreter | dropped | in PostScript | in PostScript | 1 after the interpreter |
| XPS and OpenXPS | `xps`, `oxps` | `xpstopng` | A reader of ECMA-388 onto the display list. Fonts are obfuscated with a simple XOR | the specification. libgxps is LGPL, an oracle only | 1 to 1.5 | 3 to 5 | 4 | 1.5 to 2.5 |
| EMF, EMF+ and WMF | `emf`, `emz`, `wmf`, `wmz` | none | A replay of the records onto the display list. Office files embed these images, and EPS previews use WMF | [mgilbir/gowemf][gowemf] (MIT, pure Go, from 2026-10). Examine it first. The specifications MS-EMF, MS-EMFPLUS and MS-WMF | added | 4 to 6 | 4 | 2 to 4 |
| DjVu | `djvu`, `djv` | none | A copy of the decoders of DjVu from dejapdf: IFF, BZZ, ZP, JB2 and IW44 | [sergev/dejapdf][dejapdf] (MIT, pure Go, Go 1.27). DjVuLibre is GPL, an oracle only | dropped | 6 to 10, with CHM | 8 | 1 to 2 |

## Images with layers

| Format | Extensions | iv today | Way at review quality | Source (license) | Gemini | GPT-5.6 | DeepSeek | Survey |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Photoshop | `psd`, `psb` | `oov/psd`, then libvips | The merged image that the file holds. If the file holds none, a composite of the layers. Fix the 16-bit and 32-bit files that `oov/psd` fails on | [oov/psd][oov-psd] (MIT, pure Go). [psd-tools][psd-tools] (MIT, Python) as the reference for compositing | 0.5 | 2 to 4 | 3 | 0.5 to 1 |
| OpenRaster and Krita | `ora`, `kra` | none | `mergedimage.png` in the zip | none | added | none | none | 0.1 |
| GIMP | `xcf` | none | A composite of the layers. The file holds no merged image | the specification. GIMP is GPL | dropped | none | none | 1, later |

## Ebooks

calibre converts each of these to PDF today. ivy converts each one to HTML
and CSS, and the layout puts it on pages (D10).

| Format | Extensions | iv today | Way at review quality | Source (license) | Survey |
| --- | --- | --- | --- | --- | --- |
| EPUB 2 and 3 | `epub`, `kepub` | `ebook-convert` | Zip, then the OPF package, then XHTML into the layout. The cover is the preview | the specification | 0.5 |
| FictionBook | `fb2`, `fbz` | `ebook-convert` | XML into HTML, then the layout | the specification | 0.25 |
| MOBI, PRC and AZW | `mobi`, `prc`, `azw` | `ebook-convert` | PalmDB, then the PalmDoc and HUFF/CDIC decompression, then HTML | the MobileRead wiki. [leotaku/mobi][leotaku-mobi] (MIT) writes MOBI and is a reference. KindleUnpack and calibre are GPL. The decompression goes to magic (D8) | 1 to 1.5 |
| AZW3, also called KF8 | `azw3` | `ebook-convert` | The MOBI reader, then the skeleton and the fragments of KF8 into HTML | as MOBI | 0.5 |
| AZW4 | `azw4` | none | A PDF inside a PalmDB file | the PDF reader | 0.1 |
| KFX | `kfx` | none | Almost every KFX file holds DRM. Its decoding goes in the module of DRM (D12) | none | later |
| HTMLZ, TXTZ, HTML, text and Markdown | `htmlz`, `txtz`, `html`, `htm`, `txt`, `md` | `blitz`, through cgo, for HTML, Markdown and text | Into the layout. Markdown goes through a parser into HTML | [yuin/goldmark][goldmark] (MIT, pure Go) for Markdown | 0.5 |
| Comic book archives | `cbz`, `cbr`, `cb7`, `cbt`, `cbc` | the `archives` decoder of iv | The first image in the order of the names. CBR needs RAR, CB7 needs 7z | `archive/zip`, [nwaples/rardecode][rardecode] (BSD-2-Clause), [bodgit/sevenzip][sevenzip] (BSD-3-Clause) | 0.25 |
| CHM | `chm` | none | The ITSF container and LZX, then HTML into the layout | the specification of LZX. LZX goes to magic (D8) | 1 |
| Microsoft Reader | `lit` | none | The LZX of CHM, then HTML. A file with DRM goes to the module of DRM (D12) | none permissive | 1 to 1.5, later |
| Sony BBeB | `lrf` | none | The records into the layout | none permissive | 1, later |
| eReader, Plucker, Rocket, TCR, SNB and PML | `pdb`, `pml`, `rb`, `tcr`, `snb` | none | Each one into HTML | none permissive | 0.25 to 0.5 each, only if a user asks |

Gemini puts all of the ebooks at 2 to 3 months, GPT-5.6 at 5 to 8, and
DeepSeek at 12. This survey puts the core at 4 to 4.5 months, on top of the
layout layer. Microsoft Reader and Sony BBeB add 2 to 2.5 months, and they are
in the long tail.

## Office documents

| Format | Extensions | iv today | Way at review quality | Source (license) | Survey |
| --- | --- | --- | --- | --- | --- |
| The preview of each Office file | all below | `soffice` | `Thumbnails/thumbnail.png` of OpenDocument. `docProps/thumbnail.*` of Office Open XML. The thumbnail in the summary information of a binary Office file (D11) | the specifications | 0.5, and the EMF reader |
| PowerPoint, Impress and Draw | `pptx`, `ppsx`, `potx`, `odp`, `otp`, `odg`, `otg` | `soffice` | Each slide has fixed positions, so it goes onto the display list. The shapes of DrawingML come from the preset definitions of ECMA-376 | the specifications ECMA-376 and ODF 1.3 | 2 to 3 |
| Word, Writer and RTF | `docx`, `dotx`, `odt`, `ott`, `rtf` | `soffice` | Into HTML and CSS, then the layout | the specifications. [Apache POI][poi] (Apache-2.0, Java) as a reference | 2 to 3, and 0.5 to 1 for RTF |
| Excel, Calc and CSV | `xlsx`, `xltx`, `ods`, `ots`, `csv`, `tsv` | `soffice` | A grid of the cells that hold data in the first sheet, with the values that the file holds and the basic styles. No formulas are computed | the specifications | 1 to 1.5 |
| Word, Excel and PowerPoint before 2007 | `doc`, `dot`, `xls`, `xlt`, `ppt`, `pps`, `pot` | `soffice` | First the preview and the text. Then the records of MS-DOC, MS-XLS and MS-PPT into the same paths as the formats above | [mscfb][mscfb] (Apache-2.0, pure Go). Apache POI as a reference | 1 to 2 for the text, 6 or more for a layout |
| Encrypted Office files | all above | `soffice` asks for no password | MS-OFFCRYPTO and the encryption of OpenDocument (D12) | the specifications, and msoffcrypto-tool (MIT) as a reference | in the shared layers |
| Visio | `vsdx`, `vsd` | `soffice` | `docProps/thumbnail.emf`, which needs the EMF reader | the specifications | 0.25, preview only |
| Publisher, WordPerfect, HWP and iWork | `pub`, `wpd`, `hwp`, `hwpx`, `pages`, `key`, `numbers` | `soffice` for `pub` and `wpd` | The preview only. iWork holds a JPEG in `QuickLook/` or in its root | none | 0.25 each, preview only |

Gemini puts the Office formats at 3.5 to 4.5 months, with only the preview
and the text of the binary formats. GPT-5.6 puts them at 20 to 31 months, and
DeepSeek at 35, each with a layout for each format and a port of Apache POI.
This survey puts them at 8 to 12 months, because the flows of text share the
layout, the slides share the display list, and the binary formats stop at
their preview and their text until a user needs more. The EMF reader and the
encryption add 3 to 5.5 months.

## Containers with an icon or a preview

These formats are not documents. A program shows them by the icon or the
preview that they carry (D11).

| Format | Extensions | Where the image is | Source (license) | Survey |
| --- | --- | --- | --- | --- |
| Apple disk image | `dmg` | UDIF holds blocks that zlib, bzip2, ADC, LZFSE or LZMA compress. The blocks hold HFS+ or APFS. The image is `.VolumeIcon.icns`, the icon of the application inside, or the background in `.background/` | [blacktop/go-apfs][go-apfs] (Apache-2.0, pure Go). LZFSE from [lzfse][lzfse] (BSD-3-Clause, C). libdmg-hfsplus is GPL-3.0, a reference only. iv decodes ICNS with jackmordaunt/icns (MIT) | 1.5 to 2.5 |
| macOS application and installer | `app`, `pkg` | `Contents/Info.plist` names the ICNS file. A `pkg` is a XAR archive with a compressed cpio | the specifications | 0.5 to 1 |
| iOS application | `ipa` | Zip, then `Info.plist`, then a PNG in the CgBI form of Apple, which needs its channels swapped and its compression fixed | the specification | 0.5 |
| Android package | `apk`, `aab`, `xapk`, `apks` | The binary XML of the manifest names the icon. `resources.arsc` finds the file. The icon is a PNG, a WebP or an adaptive icon in vector XML | AOSP (Apache-2.0) as a reference | 1 to 1.5 |
| Windows installer | `msi`, `msix`, `appx` | MSI is a Compound File with an Icon table. MSIX and APPX are zip files with a manifest | mscfb | 0.5 |
| Linux packages | `AppImage`, `deb`, `rpm`, `snap`, `flatpak` | AppImage holds `.DirIcon` in SquashFS. A deb or an rpm holds icons under `/usr/share/icons` | [diskfs/go-diskfs][go-diskfs] (MIT) for SquashFS and ISO 9660. [sassoftware/go-rpmutils][rpmutils] (Apache-2.0) | 1 |
| Disc images | `iso`, `img`, `udf` | `autorun.inf` names an icon, or `.VolumeIcon.icns` | go-diskfs | 0.5 |
| Extensions and archives of Java | `crx`, `xpi`, `vsix`, `jar` | A manifest in a zip names the icon | none | 0.25 |
| Camera RAW | `cr2`, `cr3`, `nef`, `arw`, `dng`, `orf`, `raf`, `rw2`, `pef`, `srw` | A full size JPEG that the camera wrote. Each format but CR3 is a form of TIFF, and CR3 is a form of ISO BMFF | the specifications | 0.5 to 1 |
| Design and 3D files | `sketch`, `fig`, `afphoto`, `afdesign`, `procreate`, `3mf`, `fcstd`, `blend`, `skp`, `xmind` | A PNG in a zip, or a thumbnail block in the file | none | 0.1 to 0.25 each |

The formats that iv shows already stay in iv: the icons of a Windows
executable, the cover art of audio and video, fonts, and the images that the
standard library of Go and libvips decode.

## Formats that the models added

The models named these formats that the first list did not hold. The tables
above hold the ones that this survey keeps.

| Format | Named by | Kept |
| --- | --- | --- |
| EMF and WMF | Gemini, GPT-5.6, DeepSeek | Yes. Office files and EPS need it |
| iWork | Gemini, GPT-5.6, DeepSeek | Yes, preview only |
| OpenRaster and Krita | Gemini | Yes |
| Comic book archives | Gemini, GPT-5.6, DeepSeek | Yes |
| Visio, Publisher and HWP | GPT-5.6, DeepSeek | Yes, preview only |
| MHTML, EML and MSG | DeepSeek | Later. MHTML and EML go into the layout. MSG is a Compound File |
| OneNote | DeepSeek | No. The format is large and holds no preview |
| HEIC, AVIF, TIFF, JPEG 2000 and DICOM | GPT-5.6 | No. They are images that libvips and other packages decode |
| LaTeX, CAD and GIS formats | GPT-5.6 | No. okage `docs/FORMATS.md` covers DXF and DWG |

## What okage lacks

A model listed each feature below as a gap of tiny-skia. At the quality of
D5, most of them do not matter.

| Feature | Needed by | At review quality |
| --- | --- | --- |
| CMYK and ICC profiles | PDF, PSD, EPS, JPEG | ivy converts to RGB before it draws, with the port of moxcms where the file holds an ICC profile (D14). okage needs no change |
| Knockout groups and the blending of CMYK | PDF | Ignored, as hayro ignores them |
| Isolated groups and soft masks | PDF, XPS | A group draws to a pixmap of its own, then blends. okage has masks of alpha and of luminance. To be confirmed in okage |
| Shadings of types 4 to 7, meshes of triangles and of patches | PDF, PostScript | A request to okage for a fill of triangles with colors that the fill interpolates (D8). Until then, the mean color of the mesh |
| 16-bit and float channels | PSD, TIFF, PDF images | Reduced to 8 bits before the draw |
| Type 1 and Type 3 fonts | PDF, PostScript | Not okage. The font layer of ivy gives outlines, and okage fills them as paths |
| Hinting of fonts | all | Ignored |

## The suggested order

All three models put the previews and the quick wins first. Gemini put
ebooks before PDF, and the other two did not order the work. This survey
suggests this order. Each step gives iv a result that it can use, and each
step builds a layer that a later step needs:

1. The previews (D11): OpenDocument, Office Open XML, iWork, OpenRaster,
   Krita, EPS, camera RAW, comic book archives, and PSD. The containers with
   icons, such as DMG, IPA and APK, run beside the other steps.
2. The display list and XPS. XPS is small and replaces `xpstopng`.
3. The PDF reader, a port of hayro, with its encryption, and Illustrator on
   top of it. It replaces libvips and `pdftoppm` for PDF in iv.
4. The layout and the ebooks: EPUB, FictionBook, Markdown and HTML, then
   MOBI, AZW3 and CHM. They replace `ebook-convert`.
5. The Office formats: EMF and WMF, the slides, the documents, the
   spreadsheets and their encryption. They replace `soffice`.
6. DjVu. The long tail waits until a user asks for it (D1).

## The risks

1. The models disagree by six times on the cost. The survey estimate is a
   guess until phase 1 measures one format of each kind.
2. Fonts decide most of what a person sees. A PDF that embeds no font, or a
   Word document that names a font that the system lacks, needs a good
   substitute.
3. The binary Office formats have much behavior that no specification
   holds. A test corpus matters more than the specification.
4. Each format is input from strangers. A zip bomb, a loop of references in
   a PDF, or a huge image can stop a program. Each package needs limits on
   memory and on time, and fuzz tests.
5. hayro is still at work on its robustness and its API, so its port needs a
   ledger that follows its commits, as okage follows resvg.
6. A source that a model names can be wrong. Examine each source before a
   decision names it.

[hayro]: https://github.com/LaurenzV/hayro
[webrender]: https://github.com/benoitkugler/webrender
[weasyprint]: https://github.com/Kozea/WeasyPrint
[moxcms]: https://github.com/awxkee/moxcms
[mscfb]: https://github.com/richardlehane/mscfb
[msoffcrypto]: https://github.com/nolze/msoffcrypto-tool
[go-postscript]: https://github.com/seehuhn/go-postscript
[llgcode-ps]: https://github.com/llgcode/ps
[gowemf]: https://github.com/mgilbir/gowemf
[dejapdf]: https://github.com/sergev/dejapdf
[oov-psd]: https://github.com/oov/psd
[psd-tools]: https://github.com/psd-tools/psd-tools
[leotaku-mobi]: https://github.com/leotaku/mobi
[goldmark]: https://github.com/yuin/goldmark
[rardecode]: https://github.com/nwaples/rardecode
[sevenzip]: https://github.com/bodgit/sevenzip
[poi]: https://github.com/apache/poi
[go-apfs]: https://github.com/blacktop/go-apfs
[lzfse]: https://github.com/lzfse/lzfse
[go-diskfs]: https://github.com/diskfs/go-diskfs
[rpmutils]: https://github.com/sassoftware/go-rpmutils
