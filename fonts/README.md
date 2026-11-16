# fonts

The module `github.com/xo/ivy/fonts` holds fonts with the metrics of the
fonts of Microsoft Office (D24). A font with the same metrics has the same
advance width for each character, so a line breaks at the same word as in
Office. The module is separate from the module `github.com/xo/ivy`, so a
program that does not import it does not hold the fonts (D6).

The module also holds Droid Sans Fallback. It draws the Chinese, Japanese
and Korean (CJK) characters that the other fonts do not draw.

A program gives the fonts to every format with the field `Fonts` of
`ivy.Config`:

```go
cfg := ivy.Config{Fonts: fonts.FS()}
```

| Font | Replaces | Folder |
| --- | --- | --- |
| Liberation Sans | Arial, Helvetica | `liberation/` |
| Liberation Serif | Times New Roman, Times | `liberation/` |
| Liberation Mono | Courier New, Courier | `liberation/` |
| Carlito | Calibri | `carlito/` |
| Caladea | Cambria | `caladea/` |
| Droid Sans Fallback | a CJK font that a document does not embed | `droid/` |

A format that lays out text uses a font of `Config.Fonts` when a document
names it. It also uses it when a document names the font of Office that it
replaces and does not embed that font. The table of the replacements is
`Substitute` in `internal/font` of the main module.

A format uses Droid Sans Fallback for a CJK character that the font of the
text does not draw. A PDF file can name a CJK font, such as KozMinPr6N or
STSong-Light, and not embed it. Then the package `pdf` draws the text with
the first font that draws the characters of the CJK collection of the font:
Japan1, GB1, CNS1 or Korea1. Droid Sans Fallback draws the characters of
the four collections.

## The license

Liberation, Carlito and Caladea are under the SIL Open Font License 1.1.
Droid Sans Fallback is under the Apache License 2.0. Each folder keeps the
license of its fonts, with the copyright notices: `liberation/LICENSE`,
`carlito/OFL.txt`, `caladea/OFL.txt` and `droid/NOTICE`. `FS` gives the
license files with the fonts. The fonts are the files of their projects,
byte for byte, so the names that the licenses reserve are not changed.

## The sources

The files come from the official source of each project. Each font of
Office has four styles: regular, bold, italic and bold italic. Droid Sans
Fallback has the regular style only.

1. Liberation 2.1.5 comes from the release 2.1.5 of
   `github.com/liberationfonts/liberation-fonts`, in the archive
   `https://github.com/liberationfonts/liberation-fonts/files/7261482/liberation-fonts-ttf-2.1.5.tar.gz`.
   The SHA-256 of the archive is
   `7191c669bf38899f73a2094ed00f7b800553364f90e2637010a69c0e268f25d0`.
   `liberation/` holds its 12 TrueType files and its `LICENSE`.
2. Carlito 1.104 comes from `github.com/googlefonts/carlito`, at the commit
   `a46ae30787d4a0c9817afd4056c0042fbcfbafd6`, the last commit that holds
   the built fonts, from the folder `fonts/ttf/`. Google Fonts ships the
   same files in `ofl/carlito/` of `github.com/google/fonts`. `OFL.txt`
   comes from the same commit.
3. Caladea 1.001 comes from `github.com/huertatipografica/Caladea`, at the
   commit `336a529cfad3d103d6527752686f8331d13e820a`, from the folder
   `fonts/ttf/`. Google Fonts ships the same files in `ofl/caladea/` of
   `github.com/google/fonts`. `OFL.txt` comes from the same commit.
4. Droid Sans Fallback 2.55b comes from the Android Open Source Project, in
   the repository `https://android.googlesource.com/platform/frameworks/base`,
   at the commit `1cdfff555f4a21f71ccc978290e2e212e2f8b168`, from the folder
   `data/fonts/`. `NOTICE` comes from the same folder. The folder also holds
   `DroidSansFallbackFull.ttf`. ivy does not use that file, because it has
   no Korean syllables (Hangul).

The URL of each file of Carlito is
`https://raw.githubusercontent.com/googlefonts/carlito/a46ae30787d4a0c9817afd4056c0042fbcfbafd6/fonts/ttf/<file>`.
The URL of each file of Caladea is
`https://raw.githubusercontent.com/huertatipografica/Caladea/336a529cfad3d103d6527752686f8331d13e820a/fonts/ttf/<file>`.
The URL of each file of Droid Sans Fallback is
`https://android.googlesource.com/platform/frameworks/base/+/1cdfff555f4a21f71ccc978290e2e212e2f8b168/data/fonts/<file>?format=TEXT`.
The server gives the file in Base64. Decode it with `base64 -d`. The git
blob of `DroidSansFallback.ttf` is
`1099b177c881c96bf298989dcdca5fda7841133d`.

| File | Family | Version | Bytes | SHA-256 |
| --- | --- | --- | ---: | --- |
| `liberation/LiberationSans-Regular.ttf` | Liberation Sans | 2.1.5 | 410712 | `76d04c18ea243f426b7de1f3ad208e927008f961dc5945e5aad352d0dfde8ee8` |
| `liberation/LiberationSans-Bold.ttf` | Liberation Sans | 2.1.5 | 414456 | `788abee4c806d660e8aee46689dd8540cd4bb98da03dcc9d171ce3efd99a9173` |
| `liberation/LiberationSans-Italic.ttf` | Liberation Sans | 2.1.5 | 415816 | `e5bae5c4cde31f22142753855f4f8fb86da6ff39955ed3c0a11248b0d16948b0` |
| `liberation/LiberationSans-BoldItalic.ttf` | Liberation Sans | 2.1.5 | 408996 | `698da70fc191cc5f33ad4d6d3fe830fe4624b898ea2e3169955928b7c491f1ee` |
| `liberation/LiberationSerif-Regular.ttf` | Liberation Serif | 2.1.5 | 393576 | `058ea80864aef09a23f45cbec2bb5400bc3dfbdea01c3f10538a21fcb497fb74` |
| `liberation/LiberationSerif-Bold.ttf` | Liberation Serif | 2.1.5 | 370096 | `d754ba427cfe0bca54ae052384baa8f842da5bd6550ad4da024ac441e7a7d5ce` |
| `liberation/LiberationSerif-Italic.ttf` | Liberation Serif | 2.1.5 | 375632 | `0e3dea9f8d613e006ccfa62201f33e265d19167bd0907725c3e145368b04fc2e` |
| `liberation/LiberationSerif-BoldItalic.ttf` | Liberation Serif | 2.1.5 | 376772 | `f17db8af71e24d2066b587546021d4f0b296be389512b658dec3c09affeb11a7` |
| `liberation/LiberationMono-Regular.ttf` | Liberation Mono | 2.1.5 | 319508 | `f2b83c763e8afd21709333370bed4774337fae82267937e2b5aea7e2fbd922c1` |
| `liberation/LiberationMono-Bold.ttf` | Liberation Mono | 2.1.5 | 307996 | `bd62a0672d0b9b6710b01df434c80ad54fa5f0835207eb7b17b7a761463067bb` |
| `liberation/LiberationMono-Italic.ttf` | Liberation Mono | 2.1.5 | 281536 | `605c01c711b44480a7508d349dfbf3264e81fa43d69e61cfa7d10b86e764c4d1` |
| `liberation/LiberationMono-BoldItalic.ttf` | Liberation Mono | 2.1.5 | 284068 | `79451f3c09fe25116098853b7a2ca6e2436220ccc11af022979adbcf195be130` |
| `carlito/Carlito-Regular.ttf` | Carlito | 1.104 | 628032 | `f6418f708baede9789daef5d458c0f53d2a888af9820e8062934e504fedc6595` |
| `carlito/Carlito-Bold.ttf` | Carlito | 1.104 | 682468 | `bb5d20f79b82599ec72983597437373a80f2d2085fa91fc144fd74e876a594db` |
| `carlito/Carlito-Italic.ttf` | Carlito | 1.104 | 615236 | `0b019225e58d702bfedcbd35c21696769f8ee115cb6343f84c2f240312450d1c` |
| `carlito/Carlito-BoldItalic.ttf` | Carlito | 1.104 | 808508 | `b32928186c119599e03ca6a1ffc680fdcb7fac95772f4b95d989cf6cd3861517` |
| `caladea/Caladea-Regular.ttf` | Caladea | 1.001 | 81600 | `f1e899278b7b4491aba5b6a8253c4b04c050cc59b21865be5c37559a775153cd` |
| `caladea/Caladea-Bold.ttf` | Caladea | 1.001 | 84492 | `ae3cb2dcbc925809dd29d2a44e9802211cab66be541bacbfc9c08c74b27c3742` |
| `caladea/Caladea-Italic.ttf` | Caladea | 1.001 | 83780 | `4359a8e24f748b6447b1ff6d7a174febe70961d29f8bb8634b56dacd740a3deb` |
| `caladea/Caladea-BoldItalic.ttf` | Caladea | 1.001 | 83356 | `ccabaa7b7e2fdf253d2b1a5fa699dd8a3df8d835a9eb285ad82631a677eb76c0` |
| `droid/DroidSansFallback.ttf` | Droid Sans Fallback | 2.55b | 3451900 | `21b96a0377f067833a93af3082eb28d4ffab7a8cd46bfd513286f1d64b7b0949` |

The fonts hold 10878536 bytes in all, about 10.9 MB. Droid Sans Fallback
holds 3.5 MB of them. `fonts_test.go` makes sure that each file has this
SHA-256 and this family, and that Droid Sans Fallback draws a character of
each CJK collection.

The files are TrueType, as their projects release them. WOFF2 would make
them smaller, and ivy can read WOFF2. But no project releases its fonts in
WOFF2, so a WOFF2 file would not be a file of its project.
