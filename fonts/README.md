# fonts

The module `github.com/xo/ivy/fonts` holds fonts with the metrics of the
fonts of Microsoft Office (D24). A font with the same metrics has the same
advance width for each character, so a line breaks at the same word as in
Office. The module is separate from the module `github.com/xo/ivy`, so a
program that does not import it does not hold the fonts (D6).

The module also holds Droid Sans Fallback. It draws the Chinese, Japanese
and Korean (CJK) characters that the other fonts do not draw.

The module also holds the Noto fonts of 18 scripts that the other fonts do
not draw (D74): Arabic, Armenian, Bengali, Devanagari, Ethiopic, Georgian,
Gujarati, Gurmukhi, Kannada, Khmer, Lao, Malayalam, Myanmar, Oriya,
Sinhala, Tamil, Telugu and Thai. Liberation already draws Hebrew.

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
| Noto Naskh Arabic and 17 families of Noto Sans | a font of another script that a document does not embed | `noto/` |
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

A format uses a font of Noto for a character of its script that the font
of the text does not draw. It takes the bold face for bold text. A PDF
file can name a font of no CJK collection, such as Traditional Arabic, and
not embed it. Then the package `pdf` draws a character that the substitute
does not draw with the first face that draws it. The ToUnicode map of the
font gives the character. Without a ToUnicode map, the codes are the
glyphs of the font that the file names, and no other font can draw them.

## The license

Liberation, Carlito, Caladea and Noto are under the SIL Open Font License
1.1.
Droid Sans Fallback is under the Apache License 2.0. Latin Modern is under the
GUST Font License, which is the LaTeX Project Public License 1.3c with a
request: a font that is changed takes another name. ivy does not change
the fonts. `lm/GUST-FONT-LICENSE.txt` holds the license, and
`lm/README-Latin-Modern.txt` and `lm/README-Latin-Modern-Math.txt` hold the
notices of the authors. The manifests of the two works are
`lm/MANIFEST-Latin-Modern.txt` and `lm/MANIFEST-Latin-Modern-Math.txt`. Each folder keeps the
license of its fonts, with the copyright notices: `liberation/LICENSE`,
`carlito/OFL.txt`, `caladea/OFL.txt`, `droid/NOTICE`, `noto/OFL.txt` and
`lm/GUST-FONT-LICENSE.txt`. `noto/OFL.txt` holds the copyright notice of
the repository of each script, such as `github.com/notofonts/arabic`, and
the license, which is the same for each. `FS` gives the license files with the fonts of
Office and Droid Sans Fallback, and `TeX` with Latin Modern. `FS` gives the
license files with the fonts. The fonts are the files of their projects,
byte for byte, so the names that the licenses reserve are not changed.

## The sources

The files come from the official source of each project. Each font of
Office has four styles: regular, bold, italic and bold italic. Droid Sans
Fallback has the regular style only. Each font of Noto has the regular and
the bold styles.

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
7. The 36 files of Noto come from `github.com/notofonts/notofonts.github.io`,
   at the commit `d14cc173a70dc57962ed2d4ad278fc9df83bf9f3`, from the
   folders `fonts/<family>/unhinted/ttf/` (D74). That repository holds the
   files that the repository of each script builds. They are the files for
   documents, not the files for interfaces, such as
   `NotoSansDevanagariUI`, which have shorter ascenders and descenders.
   `OFL.txt` comes from the repository of each script.

The URL of each file of Carlito is
`https://raw.githubusercontent.com/googlefonts/carlito/a46ae30787d4a0c9817afd4056c0042fbcfbafd6/fonts/ttf/<file>`.
The URL of each file of Caladea is
`https://raw.githubusercontent.com/huertatipografica/Caladea/336a529cfad3d103d6527752686f8331d13e820a/fonts/ttf/<file>`.
The URL of each file of Droid Sans Fallback is
`https://android.googlesource.com/platform/frameworks/base/+/1cdfff555f4a21f71ccc978290e2e212e2f8b168/data/fonts/<file>?format=TEXT`.
The server gives the file in Base64. Decode it with `base64 -d`. The git
blob of `DroidSansFallback.ttf` is
`1099b177c881c96bf298989dcdca5fda7841133d`.
The URL of each file of Noto is
`https://raw.githubusercontent.com/notofonts/notofonts.github.io/d14cc173a70dc57962ed2d4ad278fc9df83bf9f3/fonts/<family>/unhinted/ttf/<file>`.

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
| `noto/NotoNaskhArabic-Regular.ttf` | Noto Naskh Arabic | 2.021 | 158580 | `c34fdbd98af4dbc45ca192a23d2eeb77032add83086f5fe32957d23b3f36b221` |
| `noto/NotoNaskhArabic-Bold.ttf` | Noto Naskh Arabic | 2.021 | 158172 | `380ac34b249d336e1a43c21053df440594e1e3db8f75fabd4d85b649ca1ea4c6` |
| `noto/NotoSansArmenian-Regular.ttf` | Noto Sans Armenian | 2.008 | 18772 | `73bde9f3c63aa5ed39236e8a2837224605210a98b54d65724676c0119fcaa24c` |
| `noto/NotoSansArmenian-Bold.ttf` | Noto Sans Armenian | 2.008 | 18752 | `1dd206046cd3f41efddec93359a8d6590e5e2d5fcb036b9025725b560cefe6c6` |
| `noto/NotoSansBengali-Regular.ttf` | Noto Sans Bengali | 3.011 | 103704 | `5dceda02816fece18ea6796f474f5d3170f1d862f21b1e809cdc94b1caa4b6ec` |
| `noto/NotoSansBengali-Bold.ttf` | Noto Sans Bengali | 3.011 | 102440 | `2fe8d779e0f19576d1fc11e39f9154b4326a17b693215017dd1f5a8898c3f2c3` |
| `noto/NotoSansDevanagari-Regular.ttf` | Noto Sans Devanagari | 2.007 | 184228 | `9c7d935139ea6a1e6ad9dbac4f6d27ece1e04bca8123c8888d00a0f9df4724cd` |
| `noto/NotoSansDevanagari-Bold.ttf` | Noto Sans Devanagari | 2.007 | 183412 | `ff2f76a23aad41e0608c2d7dbc4bacd247ff3bec78f0ec2a8fb106b561636e58` |
| `noto/NotoSansEthiopic-Regular.ttf` | Noto Sans Ethiopic | 2.102 | 289244 | `86b9c07c049e68438388d00a34033fa28cabeef91b26e1bbed67512d360166d4` |
| `noto/NotoSansEthiopic-Bold.ttf` | Noto Sans Ethiopic | 2.102 | 288828 | `6b06274ea730962522b1fcbfdcb16cb5a385aa36734f31f5c9d0ca910541e880` |
| `noto/NotoSansGeorgian-Regular.ttf` | Noto Sans Georgian | 2.005 | 35300 | `f4b229b126859725b75031dd8c051335d20c85e4eb6d525af28f3e83e471baa4` |
| `noto/NotoSansGeorgian-Bold.ttf` | Noto Sans Georgian Bold | 2.005 | 35820 | `517722454bbe6587ada79a0509db8c9b292b6a40b025eebe69c151a6508efc7e` |
| `noto/NotoSansGujarati-Regular.ttf` | Noto Sans Gujarati | 2.106 | 142760 | `18727143b8eadade0a59dca80959ef784bf8d55e07285d1b4096594dbf829f0a` |
| `noto/NotoSansGujarati-Bold.ttf` | Noto Sans Gujarati | 2.106 | 140812 | `3ce0cf6e1d0bfe2ef5823772eeb2f25da28ec0c2c1d99c0fce529025723e61a1` |
| `noto/NotoSansGurmukhi-Regular.ttf` | Noto Sans Gurmukhi | 2.004 | 35956 | `2454c944fda799d2861f11c03e492c453e33083dd1e8d99bd6baac8de71ad0f0` |
| `noto/NotoSansGurmukhi-Bold.ttf` | Noto Sans Gurmukhi | 2.004 | 35712 | `d16804da302ecb2a7defe0502e65136870445fd88f9fddaef60b68d5f1ee7386` |
| `noto/NotoSansKannada-Regular.ttf` | Noto Sans Kannada | 2.006 | 135488 | `2870274619c2a4abe5d6b11fcb7c4a372eba765b6b7881d459b0eb0a609a68eb` |
| `noto/NotoSansKannada-Bold.ttf` | Noto Sans Kannada | 2.006 | 133868 | `e989c72f6fec5a0b3366af5f90e3ce8b2b8a350fc5286c6374f0353937e513f5` |
| `noto/NotoSansKhmer-Regular.ttf` | Noto Sans Khmer | 2.004 | 67784 | `60c10dbfae33a44f1897cd939789bd173aedb7c7dc0a9af389a737e33cb7e548` |
| `noto/NotoSansKhmer-Bold.ttf` | Noto Sans Khmer | 2.004 | 67976 | `817382a538f299eb809ee9313ef5c52e0049e3487eb4d02678349755ed607ddf` |
| `noto/NotoSansLao-Regular.ttf` | Noto Sans Lao | 2.003 | 21720 | `d3976b0ac08702c54999ff42eae295d1597fd73b44e0c57d7f4917524e18afbe` |
| `noto/NotoSansLao-Bold.ttf` | Noto Sans Lao | 2.003 | 21508 | `6fea2e9f231a4b9cfdeec90e9afaff8818b3d11f74cdb23559275f71eeff1ffd` |
| `noto/NotoSansMalayalam-Regular.ttf` | Noto Sans Malayalam | 2.104 | 77312 | `cc39bb1f7d63b582a2aec1abf2469a43b805a3523a27e0a0eee8cd32702541d5` |
| `noto/NotoSansMalayalam-Bold.ttf` | Noto Sans Malayalam | 2.104 | 75916 | `165d635d82b0eb0a04275b92b9334c1ece679b99f263727c24c7c92ebcab51e9` |
| `noto/NotoSansMyanmar-Regular.ttf` | Noto Sans Myanmar | 2.107 | 148628 | `f4be6ef43871516d8347c5217a2033991b8ecda2dd1200007c2fb431a66d64db` |
| `noto/NotoSansMyanmar-Bold.ttf` | Noto Sans Myanmar | 2.107 | 146412 | `c40a000f8093de13bdf1379a7c589faa2ea989319f33f244eaaa907033c6889f` |
| `noto/NotoSansOriya-Regular.ttf` | Noto Sans Oriya | 2.007 | 131216 | `a16645d056017927406546aa78e4ce15e782fd8783467267b75450453d007415` |
| `noto/NotoSansOriya-Bold.ttf` | Noto Sans Oriya | 2.007 | 130104 | `1c575e8aba40d6baf200e8abac534e9517f431d38cc8131715643037eec149da` |
| `noto/NotoSansSinhala-Regular.ttf` | Noto Sans Sinhala | 3.000 | 90296 | `46d5b54952a624e6e3981e0968aecc5464f3b1081131dd7a1fcd42fbf7966471` |
| `noto/NotoSansSinhala-Bold.ttf` | Noto Sans Sinhala | 3.000 | 88508 | `438ce1325e788ec0501042dd6e798c0ec02231a42dbf42e72d17b338aa46742a` |
| `noto/NotoSansTamil-Regular.ttf` | Noto Sans Tamil | 2.004 | 46600 | `6634d9cc97a726e670df41281dd32b167a6b9b71f2036e19671ff08fdde0c292` |
| `noto/NotoSansTamil-Bold.ttf` | Noto Sans Tamil | 2.004 | 46428 | `04a472e49fa83b387976756554fe179de631a950d79f652ebba14a39a44f5d71` |
| `noto/NotoSansTelugu-Regular.ttf` | Noto Sans Telugu | 2.005 | 165212 | `dcb86a5da09365a2d73d725e429403dca988e6d50c1b1d564579a25dbc18bd29` |
| `noto/NotoSansTelugu-Bold.ttf` | Noto Sans Telugu | 2.005 | 165564 | `44393b05a863057cc2cbfe86623fc20b906b00cf19a2ae9304cdd0f3608889db` |
| `noto/NotoSansThai-Regular.ttf` | Noto Sans Thai | 2.002 | 20960 | `d4303fe9c63ebb72759ca8b6d2040c8ae81689f7d08d7b91c656154382b49313` |
| `noto/NotoSansThai-Bold.ttf` | Noto Sans Thai | 2.002 | 20600 | `5227602d7f9108252cfb7d75d6f7b2a26bdb2fc2fa89a702f9063758fb2f86c7` |
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

The fonts hold 19066624 bytes in all, about 19.1 MB. Droid Sans Fallback
holds 3.5 MB of them, Noto 3.7 MB, and Latin Modern 4.5 MB. `fonts_test.go`
makes sure that each file has this SHA-256 and this family, that Droid Sans
Fallback draws a character of each CJK collection, and that each face of
Noto draws a letter of its script.

The files are TrueType, and the files of Latin Modern OpenType with CFF, as
their projects release them. WOFF2 would make
them smaller, and ivy can read WOFF2. But no project releases its fonts in
WOFF2, so a WOFF2 file would not be a file of its project.
