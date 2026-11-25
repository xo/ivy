# fonts

The module `github.com/xo/ivy/fonts` holds fonts with the metrics of the
fonts of Microsoft Office (D24). A font with the same metrics has the same
advance width for each character, so a line breaks at the same word as in
Office. The module is separate from the module `github.com/xo/ivy`, so a
program that does not import it does not hold the fonts (D6).

The module also holds Droid Sans Fallback. It draws the Chinese, Japanese
and Korean (CJK) characters that the other fonts do not draw.

The module also holds Latin Modern and Latin Modern Math, the fonts of TeX
(D62). `TeX` gives them apart from `FS`, so that the other formats do not
take them for a font of Office. The package `latex` sets the text and the
math of a LaTeX document with them:

```go
cfg := ivy.Config{Fonts: fonts.FS(), TeX: fonts.TeX()}
```

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
| Latin Modern Roman, Latin Modern Sans, Latin Modern Mono and Latin Modern Math | Computer Modern, the fonts of TeX | `lm/` |

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
Droid Sans Fallback is under the Apache License 2.0. Latin Modern is under the
GUST Font License, which is the LaTeX Project Public License 1.3c with a
request: a font that is changed takes another name. ivy does not change
the fonts. `lm/GUST-FONT-LICENSE.txt` holds the license, and
`lm/README-Latin-Modern.txt` and `lm/README-Latin-Modern-Math.txt` hold the
notices of the authors. The manifests of the two works are
`lm/MANIFEST-Latin-Modern.txt` and `lm/MANIFEST-Latin-Modern-Math.txt`. Each folder keeps the
license of its fonts, with the copyright notices: `liberation/LICENSE`,
`carlito/OFL.txt`, `caladea/OFL.txt`, `droid/NOTICE` and
`lm/GUST-FONT-LICENSE.txt`. `FS` gives the license files with the fonts of
Office and Droid Sans Fallback, and `TeX` with Latin Modern. `FS` gives the
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
5. Latin Modern 2.004 and Latin Modern Math 1.959 come from the folder
   `fonts/lm` of `github.com/go-tex/engine`, at the commit
   `8234996d66f827438f248e8b0059031f52cb9964`. Those are the unchanged
   OpenType files of the distribution Latin Modern 2.005 of GUST, at
   `http://www.gust.org.pl/projects/e-foundry/latin-modern/`. `lm/` holds
   the 24 files of Latin Modern Roman and Latin Modern Mono at their
   optical sizes that the package `latex` uses, Latin Modern Math, the
   license, the two manifests and the two files of notices. The manifests
   list each file of the works, and the folder holds a part of them.
6. The 12 files of Latin Modern Sans 2.004 come from the folder
   `fonts/opentype/public/lm` of TeX Live 2026 (D72). They are the files
   of the medium, the oblique and the bold extended shapes that
   `tulmss.fd` names. The file `lmroman10-regular.otf` of that folder has
   the same SHA-256 as the file of item 5, so both items give files of the
   same release. The manifest of item 5 lists the 12 files.

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
| `lm/latinmodern-math.otf` | Latin Modern Math | 1.959 | 733736 | `6075562b771f8b82f0c179e363389684f2dd09de30038269e2628e504bd7be0f` |
| `lm/lmmono10-regular.otf` | Latin Modern Mono | 2.004 | 64684 | `22deb6d3be3ffcb40b33f0e010afb256c677d97c2a2bdee1f5740cdc97751558` |
| `lm/lmmono12-regular.otf` | Latin Modern Mono | 2.004 | 63012 | `b826e490fa671f254840abe88afea0975b606f57fb4a8b93f53787c26d8ac319` |
| `lm/lmmono8-regular.otf` | Latin Modern Mono | 2.004 | 66268 | `17319490d20f24e192bd7e7999484b95111a4654ac2620e7aa215591142ae997` |
| `lm/lmmono9-regular.otf` | Latin Modern Mono | 2.004 | 63076 | `cdd53c7dc01a1d25f21eaf7cae44bfb8a9665e4cbd60e5f8da14287aca819bb9` |
| `lm/lmroman10-bold.otf` | Latin Modern Roman | 2.004 | 111240 | `102fe06c430a8b681b2bf6876b7cd967ae4d47b4b6b41d915eb7913b726d9fb1` |
| `lm/lmroman10-italic.otf` | Latin Modern Roman | 2.004 | 118828 | `c1fce25075567bb8dbf2151658c3b442690041db17a2d49fc9e55905ea5b7169` |
| `lm/lmroman10-regular.otf` | Latin Modern Roman | 2.004 | 111536 | `1aa18cfefa58132c52ce5de70db1fd1154201c19cd2b2cdaffba4906a33e6852` |
| `lm/lmroman12-bold.otf` | Latin Modern Roman | 2.004 | 110496 | `28c8782ac2b6486958b5dc7610ada7800c53546ff7f36bc65909a876e1cd338e` |
| `lm/lmroman12-italic.otf` | Latin Modern Roman | 2.004 | 117732 | `ada6b3f451238784e5a5ea75d43efa1eb4329cd75ce65bf047ed2f31b7b5e2f3` |
| `lm/lmroman12-regular.otf` | Latin Modern Roman | 2.004 | 110400 | `e6be218ae83e61aa8a29990d3cdc401c678c1962188cb9a4a8b6359e4f5e5870` |
| `lm/lmroman17-regular.otf` | Latin Modern Roman | 2.004 | 110248 | `ed048c80fb4b67663b80899b89fe5fba17d7442e1286d61fd08fe5689c6b7c3c` |
| `lm/lmroman5-bold.otf` | Latin Modern Roman | 2.004 | 109564 | `6326fb955d07bedcc8f73684e6b6bfa76706d38f3a85bd89c0659504ace366a1` |
| `lm/lmroman5-regular.otf` | Latin Modern Roman | 2.004 | 111872 | `954bd9c376597c7043e7b616430163e0158155edf7434fd70082cf8460c08533` |
| `lm/lmroman6-bold.otf` | Latin Modern Roman | 2.004 | 112452 | `f1ebd52916182bfc2e3f9d4b5ba847ad6acc6c7761354c0dc052fb01ec14f73c` |
| `lm/lmroman6-regular.otf` | Latin Modern Roman | 2.004 | 112212 | `501396c437f76c260b8724265b8495f318a640ee5a4674852ecbb20daf256696` |
| `lm/lmroman7-bold.otf` | Latin Modern Roman | 2.004 | 110212 | `063ea9e378b4fca4fd59771cf522faa21eb7c8516c89464745c79725b384fb9a` |
| `lm/lmroman7-italic.otf` | Latin Modern Roman | 2.004 | 117960 | `19d20f2bb00fbffe5d3d66e9b6a54f0108289068029e9d88649d981d53613ddf` |
| `lm/lmroman7-regular.otf` | Latin Modern Roman | 2.004 | 110792 | `48c4b46ab255eec96588fb992fba1ed6102e53d051da480738a61cc6cdb328c3` |
| `lm/lmroman8-bold.otf` | Latin Modern Roman | 2.004 | 112404 | `04d4f633c3da31a169f4c54a7fe8c7e63925b39d030381f8d3eab955f16be7b2` |
| `lm/lmroman8-italic.otf` | Latin Modern Roman | 2.004 | 117248 | `fb0536b5c3eee5b5622cd827ceb410e6514e36512ebdc51d2cf98a96c06d44e8` |
| `lm/lmroman8-regular.otf` | Latin Modern Roman | 2.004 | 111948 | `f73015c3827596586faa380c398501caf9b5bcbb6f40b46af6a51277ca97c9fc` |
| `lm/lmroman9-bold.otf` | Latin Modern Roman | 2.004 | 113040 | `f72feac9ea12eb858d8093d6c0ee710a6c25f59b61278c88ec41c0a680838415` |
| `lm/lmroman9-italic.otf` | Latin Modern Roman | 2.004 | 118380 | `2106cffcdc9f9cb411d7c3441e34d77a168b998a58b40e727ab5b815437d1975` |
| `lm/lmroman9-regular.otf` | Latin Modern Roman | 2.004 | 112680 | `8ab7b017fc67ea0e30cbf9e7ebd940010442ea67b40cf65f87d73f3ca11b0e6f` |
| `lm/lmsans10-bold.otf` | Latin Modern Sans | 2.004 | 107448 | `a597b710326c1a8a2c7238d808e5d38711638a72a32383478db4829d63afd687` |
| `lm/lmsans10-boldoblique.otf` | Latin Modern Sans | 2.004 | 114984 | `e45673d84754a94f2f053bd4c15950b4b320144fbb622c630a26fd7b68cfabc5` |
| `lm/lmsans10-oblique.otf` | Latin Modern Sans | 2.004 | 101412 | `1b38b36efc21f9d3f34edf6a13f9d71f59c41f824e790f5d3ee5ba5b4936f377` |
| `lm/lmsans10-regular.otf` | Latin Modern Sans | 2.004 | 95128 | `d431b786b9b603662718e79cfe9b441f47a8b0b3e854dde89d5acb3ed7cfd682` |
| `lm/lmsans12-oblique.otf` | Latin Modern Sans | 2.004 | 102056 | `1f138e6203f7092f9a01f6193ed97d971b5045b99969f3a62c7e43a7faabc1c0` |
| `lm/lmsans12-regular.otf` | Latin Modern Sans | 2.004 | 95160 | `f87ebdea035a96e8787e2c663cfcc78ce73205d37800eafe7948278e2cc7666a` |
| `lm/lmsans17-oblique.otf` | Latin Modern Sans | 2.004 | 100500 | `0f6a4450c5fc9afd3650f24ad9003684fff3252691bbf2a33be94b383db8a77c` |
| `lm/lmsans17-regular.otf` | Latin Modern Sans | 2.004 | 94828 | `f6c395a436e4fa7e88bbf1445e64b9c78ce4effba3668b13cc433077fc71a984` |
| `lm/lmsans8-oblique.otf` | Latin Modern Sans | 2.004 | 100084 | `7f4227513bfd47b7077bedd125caa4c0a223a2cdad17a1470145cdea83353c88` |
| `lm/lmsans8-regular.otf` | Latin Modern Sans | 2.004 | 94256 | `04f0d25ca474494cb707e752170f6c22cdcb8ae2b5fb80872143e18d5cc463f8` |
| `lm/lmsans9-oblique.otf` | Latin Modern Sans | 2.004 | 101068 | `ac0535d7d4a8eb1b531c5faeb9060a4b9bac397d72cf54191ef53eea6f66b617` |
| `lm/lmsans9-regular.otf` | Latin Modern Sans | 2.004 | 94552 | `80ef596460b7b402621780f7cd73a348efedc4cd2c6deca37da58f07ee61b85d` |

The fonts hold 15332032 bytes in all, about 15.3 MB. Droid Sans Fallback
holds 3.5 MB of them, and Latin Modern 4.5 MB. `fonts_test.go` makes sure
that each file has this SHA-256 and this family, and that Droid Sans
Fallback draws a character of each CJK collection.

The files are TrueType, and the files of Latin Modern OpenType with CFF, as
their projects release them. WOFF2 would make
them smaller, and ivy can read WOFF2. But no project releases its fonts in
WOFF2, so a WOFF2 file would not be a file of its project.
