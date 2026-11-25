package fonts

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"testing"
	"unicode/utf16"
)

// want holds each font file of FS, with its family and the SHA-256 that
// README.md gives.
var want = map[string]struct {
	family string
	sha256 string
}{
	"liberation/LiberationSans-Regular.ttf":    {"Liberation Sans", "76d04c18ea243f426b7de1f3ad208e927008f961dc5945e5aad352d0dfde8ee8"},
	"liberation/LiberationSans-Bold.ttf":       {"Liberation Sans", "788abee4c806d660e8aee46689dd8540cd4bb98da03dcc9d171ce3efd99a9173"},
	"liberation/LiberationSans-Italic.ttf":     {"Liberation Sans", "e5bae5c4cde31f22142753855f4f8fb86da6ff39955ed3c0a11248b0d16948b0"},
	"liberation/LiberationSans-BoldItalic.ttf": {"Liberation Sans", "698da70fc191cc5f33ad4d6d3fe830fe4624b898ea2e3169955928b7c491f1ee"},

	"liberation/LiberationSerif-Regular.ttf":    {"Liberation Serif", "058ea80864aef09a23f45cbec2bb5400bc3dfbdea01c3f10538a21fcb497fb74"},
	"liberation/LiberationSerif-Bold.ttf":       {"Liberation Serif", "d754ba427cfe0bca54ae052384baa8f842da5bd6550ad4da024ac441e7a7d5ce"},
	"liberation/LiberationSerif-Italic.ttf":     {"Liberation Serif", "0e3dea9f8d613e006ccfa62201f33e265d19167bd0907725c3e145368b04fc2e"},
	"liberation/LiberationSerif-BoldItalic.ttf": {"Liberation Serif", "f17db8af71e24d2066b587546021d4f0b296be389512b658dec3c09affeb11a7"},

	"liberation/LiberationMono-Regular.ttf":    {"Liberation Mono", "f2b83c763e8afd21709333370bed4774337fae82267937e2b5aea7e2fbd922c1"},
	"liberation/LiberationMono-Bold.ttf":       {"Liberation Mono", "bd62a0672d0b9b6710b01df434c80ad54fa5f0835207eb7b17b7a761463067bb"},
	"liberation/LiberationMono-Italic.ttf":     {"Liberation Mono", "605c01c711b44480a7508d349dfbf3264e81fa43d69e61cfa7d10b86e764c4d1"},
	"liberation/LiberationMono-BoldItalic.ttf": {"Liberation Mono", "79451f3c09fe25116098853b7a2ca6e2436220ccc11af022979adbcf195be130"},

	"carlito/Carlito-Regular.ttf":    {"Carlito", "f6418f708baede9789daef5d458c0f53d2a888af9820e8062934e504fedc6595"},
	"carlito/Carlito-Bold.ttf":       {"Carlito", "bb5d20f79b82599ec72983597437373a80f2d2085fa91fc144fd74e876a594db"},
	"carlito/Carlito-Italic.ttf":     {"Carlito", "0b019225e58d702bfedcbd35c21696769f8ee115cb6343f84c2f240312450d1c"},
	"carlito/Carlito-BoldItalic.ttf": {"Carlito", "b32928186c119599e03ca6a1ffc680fdcb7fac95772f4b95d989cf6cd3861517"},

	"caladea/Caladea-Regular.ttf":    {"Caladea", "f1e899278b7b4491aba5b6a8253c4b04c050cc59b21865be5c37559a775153cd"},
	"caladea/Caladea-Bold.ttf":       {"Caladea", "ae3cb2dcbc925809dd29d2a44e9802211cab66be541bacbfc9c08c74b27c3742"},
	"caladea/Caladea-Italic.ttf":     {"Caladea", "4359a8e24f748b6447b1ff6d7a174febe70961d29f8bb8634b56dacd740a3deb"},
	"caladea/Caladea-BoldItalic.ttf": {"Caladea", "ccabaa7b7e2fdf253d2b1a5fa699dd8a3df8d835a9eb285ad82631a677eb76c0"},

	"droid/DroidSansFallback.ttf": {"Droid Sans Fallback", "21b96a0377f067833a93af3082eb28d4ffab7a8cd46bfd513286f1d64b7b0949"},

	"noto/NotoNaskhArabic-Regular.ttf":    {"Noto Naskh Arabic", "c34fdbd98af4dbc45ca192a23d2eeb77032add83086f5fe32957d23b3f36b221"},
	"noto/NotoNaskhArabic-Bold.ttf":       {"Noto Naskh Arabic", "380ac34b249d336e1a43c21053df440594e1e3db8f75fabd4d85b649ca1ea4c6"},
	"noto/NotoSansArmenian-Regular.ttf":   {"Noto Sans Armenian", "73bde9f3c63aa5ed39236e8a2837224605210a98b54d65724676c0119fcaa24c"},
	"noto/NotoSansArmenian-Bold.ttf":      {"Noto Sans Armenian", "1dd206046cd3f41efddec93359a8d6590e5e2d5fcb036b9025725b560cefe6c6"},
	"noto/NotoSansBengali-Regular.ttf":    {"Noto Sans Bengali", "5dceda02816fece18ea6796f474f5d3170f1d862f21b1e809cdc94b1caa4b6ec"},
	"noto/NotoSansBengali-Bold.ttf":       {"Noto Sans Bengali", "2fe8d779e0f19576d1fc11e39f9154b4326a17b693215017dd1f5a8898c3f2c3"},
	"noto/NotoSansDevanagari-Regular.ttf": {"Noto Sans Devanagari", "9c7d935139ea6a1e6ad9dbac4f6d27ece1e04bca8123c8888d00a0f9df4724cd"},
	"noto/NotoSansDevanagari-Bold.ttf":    {"Noto Sans Devanagari", "ff2f76a23aad41e0608c2d7dbc4bacd247ff3bec78f0ec2a8fb106b561636e58"},
	"noto/NotoSansEthiopic-Regular.ttf":   {"Noto Sans Ethiopic", "86b9c07c049e68438388d00a34033fa28cabeef91b26e1bbed67512d360166d4"},
	"noto/NotoSansEthiopic-Bold.ttf":      {"Noto Sans Ethiopic", "6b06274ea730962522b1fcbfdcb16cb5a385aa36734f31f5c9d0ca910541e880"},
	"noto/NotoSansGeorgian-Regular.ttf":   {"Noto Sans Georgian", "f4b229b126859725b75031dd8c051335d20c85e4eb6d525af28f3e83e471baa4"},
	"noto/NotoSansGeorgian-Bold.ttf":      {"Noto Sans Georgian Bold", "517722454bbe6587ada79a0509db8c9b292b6a40b025eebe69c151a6508efc7e"},
	"noto/NotoSansGujarati-Regular.ttf":   {"Noto Sans Gujarati", "18727143b8eadade0a59dca80959ef784bf8d55e07285d1b4096594dbf829f0a"},
	"noto/NotoSansGujarati-Bold.ttf":      {"Noto Sans Gujarati", "3ce0cf6e1d0bfe2ef5823772eeb2f25da28ec0c2c1d99c0fce529025723e61a1"},
	"noto/NotoSansGurmukhi-Regular.ttf":   {"Noto Sans Gurmukhi", "2454c944fda799d2861f11c03e492c453e33083dd1e8d99bd6baac8de71ad0f0"},
	"noto/NotoSansGurmukhi-Bold.ttf":      {"Noto Sans Gurmukhi", "d16804da302ecb2a7defe0502e65136870445fd88f9fddaef60b68d5f1ee7386"},
	"noto/NotoSansKannada-Regular.ttf":    {"Noto Sans Kannada", "2870274619c2a4abe5d6b11fcb7c4a372eba765b6b7881d459b0eb0a609a68eb"},
	"noto/NotoSansKannada-Bold.ttf":       {"Noto Sans Kannada", "e989c72f6fec5a0b3366af5f90e3ce8b2b8a350fc5286c6374f0353937e513f5"},
	"noto/NotoSansKhmer-Regular.ttf":      {"Noto Sans Khmer", "60c10dbfae33a44f1897cd939789bd173aedb7c7dc0a9af389a737e33cb7e548"},
	"noto/NotoSansKhmer-Bold.ttf":         {"Noto Sans Khmer", "817382a538f299eb809ee9313ef5c52e0049e3487eb4d02678349755ed607ddf"},
	"noto/NotoSansLao-Regular.ttf":        {"Noto Sans Lao", "d3976b0ac08702c54999ff42eae295d1597fd73b44e0c57d7f4917524e18afbe"},
	"noto/NotoSansLao-Bold.ttf":           {"Noto Sans Lao", "6fea2e9f231a4b9cfdeec90e9afaff8818b3d11f74cdb23559275f71eeff1ffd"},
	"noto/NotoSansMalayalam-Regular.ttf":  {"Noto Sans Malayalam", "cc39bb1f7d63b582a2aec1abf2469a43b805a3523a27e0a0eee8cd32702541d5"},
	"noto/NotoSansMalayalam-Bold.ttf":     {"Noto Sans Malayalam", "165d635d82b0eb0a04275b92b9334c1ece679b99f263727c24c7c92ebcab51e9"},
	"noto/NotoSansMyanmar-Regular.ttf":    {"Noto Sans Myanmar", "f4be6ef43871516d8347c5217a2033991b8ecda2dd1200007c2fb431a66d64db"},
	"noto/NotoSansMyanmar-Bold.ttf":       {"Noto Sans Myanmar", "c40a000f8093de13bdf1379a7c589faa2ea989319f33f244eaaa907033c6889f"},
	"noto/NotoSansOriya-Regular.ttf":      {"Noto Sans Oriya", "a16645d056017927406546aa78e4ce15e782fd8783467267b75450453d007415"},
	"noto/NotoSansOriya-Bold.ttf":         {"Noto Sans Oriya", "1c575e8aba40d6baf200e8abac534e9517f431d38cc8131715643037eec149da"},
	"noto/NotoSansSinhala-Regular.ttf":    {"Noto Sans Sinhala", "46d5b54952a624e6e3981e0968aecc5464f3b1081131dd7a1fcd42fbf7966471"},
	"noto/NotoSansSinhala-Bold.ttf":       {"Noto Sans Sinhala", "438ce1325e788ec0501042dd6e798c0ec02231a42dbf42e72d17b338aa46742a"},
	"noto/NotoSansTamil-Regular.ttf":      {"Noto Sans Tamil", "6634d9cc97a726e670df41281dd32b167a6b9b71f2036e19671ff08fdde0c292"},
	"noto/NotoSansTamil-Bold.ttf":         {"Noto Sans Tamil", "04a472e49fa83b387976756554fe179de631a950d79f652ebba14a39a44f5d71"},
	"noto/NotoSansTelugu-Regular.ttf":     {"Noto Sans Telugu", "dcb86a5da09365a2d73d725e429403dca988e6d50c1b1d564579a25dbc18bd29"},
	"noto/NotoSansTelugu-Bold.ttf":        {"Noto Sans Telugu", "44393b05a863057cc2cbfe86623fc20b906b00cf19a2ae9304cdd0f3608889db"},
	"noto/NotoSansThai-Regular.ttf":       {"Noto Sans Thai", "d4303fe9c63ebb72759ca8b6d2040c8ae81689f7d08d7b91c656154382b49313"},
	"noto/NotoSansThai-Bold.ttf":          {"Noto Sans Thai", "5227602d7f9108252cfb7d75d6f7b2a26bdb2fc2fa89a702f9063758fb2f86c7"},
}

// The texts that the licenses hold.
const (
	ofl    = "SIL OPEN FONT LICENSE Version 1.1"
	apache = "Apache License\n                           Version 2.0"
)

// licenses holds the license file of each folder, with a text of its
// license.
var licenses = map[string]string{
	"liberation/LICENSE": ofl,
	"carlito/OFL.txt":    ofl,
	"caladea/OFL.txt":    ofl,
	"droid/NOTICE":       apache,
	"noto/OFL.txt":       ofl,
}

// TestFonts makes sure that each file of FS is a TrueType font of its
// family, with the bytes that README.md gives, and that FS holds no other
// font.
func TestFonts(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	err := fs.WalkDir(FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".ttf" {
			return err
		}
		seen[name] = true
		w, ok := want[name]
		if !ok {
			t.Errorf("%s: unexpected font", name)
			return nil
		}
		data, err := fs.ReadFile(FS(), name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != w.sha256 {
			t.Errorf("%s: SHA-256 %x, expected %s", name, sum, w.sha256)
		}
		family, err := familyOf(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return nil
		}
		if family != w.family {
			t.Errorf("%s: family %q, expected %q", name, family, w.family)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("%s: missing", name)
		}
	}
}

// TestLicenses makes sure that each folder holds its license: the SIL Open
// Font License 1.1, or the Apache License 2.0 for Droid Sans Fallback.
func TestLicenses(t *testing.T) {
	t.Parallel()
	for name, text := range licenses {
		data, err := fs.ReadFile(FS(), name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), text) {
			t.Errorf("%s: does not hold %q", name, text)
		}
	}
}

// latinModern holds each OpenType file of Latin Modern in FS, with the
// SHA-256 that README.md gives.
var latinModern = map[string]string{
	"lm/latinmodern-math.otf":  "6075562b771f8b82f0c179e363389684f2dd09de30038269e2628e504bd7be0f",
	"lm/lmmono10-regular.otf":  "22deb6d3be3ffcb40b33f0e010afb256c677d97c2a2bdee1f5740cdc97751558",
	"lm/lmmono12-regular.otf":  "b826e490fa671f254840abe88afea0975b606f57fb4a8b93f53787c26d8ac319",
	"lm/lmmono8-regular.otf":   "17319490d20f24e192bd7e7999484b95111a4654ac2620e7aa215591142ae997",
	"lm/lmmono9-regular.otf":   "cdd53c7dc01a1d25f21eaf7cae44bfb8a9665e4cbd60e5f8da14287aca819bb9",
	"lm/lmroman10-bold.otf":    "102fe06c430a8b681b2bf6876b7cd967ae4d47b4b6b41d915eb7913b726d9fb1",
	"lm/lmroman10-italic.otf":  "c1fce25075567bb8dbf2151658c3b442690041db17a2d49fc9e55905ea5b7169",
	"lm/lmroman10-regular.otf": "1aa18cfefa58132c52ce5de70db1fd1154201c19cd2b2cdaffba4906a33e6852",
	"lm/lmroman12-bold.otf":    "28c8782ac2b6486958b5dc7610ada7800c53546ff7f36bc65909a876e1cd338e",
	"lm/lmroman12-italic.otf":  "ada6b3f451238784e5a5ea75d43efa1eb4329cd75ce65bf047ed2f31b7b5e2f3",
	"lm/lmroman12-regular.otf": "e6be218ae83e61aa8a29990d3cdc401c678c1962188cb9a4a8b6359e4f5e5870",
	"lm/lmroman17-regular.otf": "ed048c80fb4b67663b80899b89fe5fba17d7442e1286d61fd08fe5689c6b7c3c",
	"lm/lmroman5-bold.otf":     "6326fb955d07bedcc8f73684e6b6bfa76706d38f3a85bd89c0659504ace366a1",
	"lm/lmroman5-regular.otf":  "954bd9c376597c7043e7b616430163e0158155edf7434fd70082cf8460c08533",
	"lm/lmroman6-bold.otf":     "f1ebd52916182bfc2e3f9d4b5ba847ad6acc6c7761354c0dc052fb01ec14f73c",
	"lm/lmroman6-regular.otf":  "501396c437f76c260b8724265b8495f318a640ee5a4674852ecbb20daf256696",
	"lm/lmroman7-bold.otf":     "063ea9e378b4fca4fd59771cf522faa21eb7c8516c89464745c79725b384fb9a",
	"lm/lmroman7-italic.otf":   "19d20f2bb00fbffe5d3d66e9b6a54f0108289068029e9d88649d981d53613ddf",
	"lm/lmroman7-regular.otf":  "48c4b46ab255eec96588fb992fba1ed6102e53d051da480738a61cc6cdb328c3",
	"lm/lmroman8-bold.otf":     "04d4f633c3da31a169f4c54a7fe8c7e63925b39d030381f8d3eab955f16be7b2",
	"lm/lmroman8-italic.otf":   "fb0536b5c3eee5b5622cd827ceb410e6514e36512ebdc51d2cf98a96c06d44e8",
	"lm/lmroman8-regular.otf":  "f73015c3827596586faa380c398501caf9b5bcbb6f40b46af6a51277ca97c9fc",
	"lm/lmroman9-bold.otf":     "f72feac9ea12eb858d8093d6c0ee710a6c25f59b61278c88ec41c0a680838415",
	"lm/lmroman9-italic.otf":   "2106cffcdc9f9cb411d7c3441e34d77a168b998a58b40e727ab5b815437d1975",
	"lm/lmroman9-regular.otf":  "8ab7b017fc67ea0e30cbf9e7ebd940010442ea67b40cf65f87d73f3ca11b0e6f",

	"lm/lmsans10-bold.otf":        "a597b710326c1a8a2c7238d808e5d38711638a72a32383478db4829d63afd687",
	"lm/lmsans10-boldoblique.otf": "e45673d84754a94f2f053bd4c15950b4b320144fbb622c630a26fd7b68cfabc5",
	"lm/lmsans10-oblique.otf":     "1b38b36efc21f9d3f34edf6a13f9d71f59c41f824e790f5d3ee5ba5b4936f377",
	"lm/lmsans10-regular.otf":     "d431b786b9b603662718e79cfe9b441f47a8b0b3e854dde89d5acb3ed7cfd682",
	"lm/lmsans12-oblique.otf":     "1f138e6203f7092f9a01f6193ed97d971b5045b99969f3a62c7e43a7faabc1c0",
	"lm/lmsans12-regular.otf":     "f87ebdea035a96e8787e2c663cfcc78ce73205d37800eafe7948278e2cc7666a",
	"lm/lmsans17-oblique.otf":     "0f6a4450c5fc9afd3650f24ad9003684fff3252691bbf2a33be94b383db8a77c",
	"lm/lmsans17-regular.otf":     "f6c395a436e4fa7e88bbf1445e64b9c78ce4effba3668b13cc433077fc71a984",
	"lm/lmsans8-oblique.otf":      "7f4227513bfd47b7077bedd125caa4c0a223a2cdad17a1470145cdea83353c88",
	"lm/lmsans8-regular.otf":      "04f0d25ca474494cb707e752170f6c22cdcb8ae2b5fb80872143e18d5cc463f8",
	"lm/lmsans9-oblique.otf":      "ac0535d7d4a8eb1b531c5faeb9060a4b9bac397d72cf54191ef53eea6f66b617",
	"lm/lmsans9-regular.otf":      "80ef596460b7b402621780f7cd73a348efedc4cd2c6deca37da58f07ee61b85d",
}

// TestLatinModern makes sure that each OpenType file of TeX is a file of
// Latin Modern with the bytes that README.md gives, that the folder lm
// holds the GUST Font License and the notices, and that FS holds no file
// of Latin Modern.
func TestLatinModern(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	err := fs.WalkDir(TeX(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".otf" {
			return err
		}
		seen[name] = true
		want, ok := latinModern[name]
		if !ok {
			t.Errorf("%s: unexpected font", name)
			return nil
		}
		data, err := fs.ReadFile(TeX(), name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		if len(data) < 4 || string(data[:4]) != "OTTO" {
			t.Errorf("%s: not an OpenType font with CFF", name)
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != want {
			t.Errorf("%s: SHA-256 %x, expected %s", name, sum, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for name := range latinModern {
		if !seen[name] {
			t.Errorf("%s: missing", name)
		}
	}
	for name, text := range map[string]string{
		"lm/GUST-FONT-LICENSE.txt":          "GUST Font License",
		"lm/README-Latin-Modern.txt":        "Copyright 2003--2021 by B. Jackowski and J.M. Nowacki",
		"lm/README-Latin-Modern-Math.txt":   "Copyright 2012--2014",
		"lm/MANIFEST-Latin-Modern.txt":      "lmroman10-regular.otf",
		"lm/MANIFEST-Latin-Modern-Math.txt": "latinmodern-math.otf",
	} {
		data, err := fs.ReadFile(TeX(), name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), text) {
			t.Errorf("%s: does not hold %q", name, text)
		}
	}
	if _, err := fs.Stat(FS(), "lm"); err == nil {
		t.Error("expected no folder lm in FS")
	}
}

// cjk holds a character of each of the four CJK character collections of
// Adobe: Japan1, GB1, CNS1 and Korea1.
var cjk = []rune{'あ', '这', '這', '한'}

// TestCJK makes sure that Droid Sans Fallback draws a character of each
// CJK character collection, so that it can be the fallback of each.
func TestCJK(t *testing.T) {
	t.Parallel()
	data, err := fs.ReadFile(FS(), "droid/DroidSansFallback.ttf")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range cjk {
		if !covers(t, data, r) {
			t.Errorf("Droid Sans Fallback does not draw %q", r)
		}
	}
	if covers(t, data, 'A') {
		t.Error("Droid Sans Fallback draws 'A', expected only CJK characters")
	}
}

// scripts holds a letter of the script of each family of Noto (D74).
var scripts = map[string]rune{
	"NotoNaskhArabic":    'ب',
	"NotoSansArmenian":   'Ա',
	"NotoSansBengali":    'ক',
	"NotoSansDevanagari": 'क',
	"NotoSansEthiopic":   'ሀ',
	"NotoSansGeorgian":   'ა',
	"NotoSansGujarati":   'ક',
	"NotoSansGurmukhi":   'ਕ',
	"NotoSansKannada":    'ಕ',
	"NotoSansKhmer":      'ក',
	"NotoSansLao":        'ກ',
	"NotoSansMalayalam":  'ക',
	"NotoSansMyanmar":    'က',
	"NotoSansOriya":      'କ',
	"NotoSansSinhala":    'ක',
	"NotoSansTamil":      'க',
	"NotoSansTelugu":     'క',
	"NotoSansThai":       'ก',
}

// TestScripts makes sure that the regular and the bold face of each family
// of Noto draw a letter of their script, so that each can be the fallback
// of its script (D74).
func TestScripts(t *testing.T) {
	t.Parallel()
	for family, r := range scripts {
		for _, style := range []string{"Regular", "Bold"} {
			name := "noto/" + family + "-" + style + ".ttf"
			data, err := fs.ReadFile(FS(), name)
			if err != nil {
				t.Fatal(err)
			}
			if !covers(t, data, r) {
				t.Errorf("%s does not draw %q", name, r)
			}
		}
	}
}

// errFont is the error of a file that is not a TrueType font.
var errFont = errors.New("not a TrueType font")

// tablesOf returns the tables of a TrueType font by their tags. It reads
// the table directory, and makes sure that the table head has its magic
// number.
func tablesOf(data []byte) (map[string][]byte, error) {
	if len(data) < 12 || binary.BigEndian.Uint32(data) != 0x00010000 {
		return nil, errFont
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	if len(data) < 12+16*n {
		return nil, fmt.Errorf("table directory: %w", errFont)
	}
	tables := map[string][]byte{}
	for i := range n {
		rec := data[12+16*i:]
		off, size := int(binary.BigEndian.Uint32(rec[8:])), int(binary.BigEndian.Uint32(rec[12:]))
		if off < 0 || size < 0 || off > len(data) || size > len(data)-off {
			return nil, fmt.Errorf("table %q: %w", rec[:4], errFont)
		}
		tables[string(rec[:4])] = data[off : off+size]
	}
	head := tables["head"]
	if len(head) < 16 || binary.BigEndian.Uint32(head[12:]) != 0x5F0F3CF5 {
		return nil, fmt.Errorf("table head: %w", errFont)
	}
	return tables, nil
}

// familyOf returns the family of a TrueType font: the name with the ID 1
// for Windows and Unicode in its table name.
func familyOf(data []byte) (string, error) {
	tables, err := tablesOf(data)
	if err != nil {
		return "", err
	}
	name := tables["name"]
	if len(name) < 6 {
		return "", fmt.Errorf("table name: %w", errFont)
	}
	count, strs := int(binary.BigEndian.Uint16(name[2:])), int(binary.BigEndian.Uint16(name[4:]))
	if len(name) < 6+12*count {
		return "", fmt.Errorf("table name: %w", errFont)
	}
	for i := range count {
		rec := name[6+12*i:]
		platform, encoding := binary.BigEndian.Uint16(rec), binary.BigEndian.Uint16(rec[2:])
		id := binary.BigEndian.Uint16(rec[6:])
		size, off := int(binary.BigEndian.Uint16(rec[8:])), int(binary.BigEndian.Uint16(rec[10:]))
		if platform != 3 || encoding != 1 || id != 1 {
			continue
		}
		if strs+off+size > len(name) || size%2 != 0 {
			return "", fmt.Errorf("table name: %w", errFont)
		}
		b := name[strs+off : strs+off+size]
		u := make([]uint16, size/2)
		for j := range u {
			u[j] = binary.BigEndian.Uint16(b[2*j:])
		}
		return string(utf16.Decode(u)), nil
	}
	return "", fmt.Errorf("no family: %w", errFont)
}

// covers reports whether the subtable of the cmap table of a TrueType font
// for Windows and Unicode maps r to a glyph other than 0. It reads the
// formats 4 and 12.
func covers(t *testing.T, data []byte, r rune) bool {
	t.Helper()
	tables, err := tablesOf(data)
	if err != nil {
		t.Fatal(err)
	}
	cmap := tables["cmap"]
	if len(cmap) < 4 {
		t.Fatal("no table cmap")
	}
	n := int(binary.BigEndian.Uint16(cmap[2:]))
	for i := range n {
		rec := 4 + 8*i
		if rec+8 > len(cmap) {
			break
		}
		platform, encoding := binary.BigEndian.Uint16(cmap[rec:]), binary.BigEndian.Uint16(cmap[rec+2:])
		off := int(binary.BigEndian.Uint32(cmap[rec+4:]))
		if platform != 3 || (encoding != 1 && encoding != 10) || off+4 > len(cmap) {
			continue
		}
		sub := cmap[off:]
		switch binary.BigEndian.Uint16(sub) {
		case 4:
			if covers4(sub, r) {
				return true
			}
		case 12:
			if covers12(sub, r) {
				return true
			}
		}
	}
	return false
}

// covers4 reports whether a subtable of the format 4 maps r to a glyph.
func covers4(sub []byte, r rune) bool {
	if r > 0xFFFF || len(sub) < 14 {
		return false
	}
	segs := int(binary.BigEndian.Uint16(sub[6:])) / 2
	if len(sub) < 16+8*segs {
		return false
	}
	c := uint16(r)
	ends, starts := sub[14:], sub[16+2*segs:]
	deltas, offsets := sub[16+4*segs:], sub[16+6*segs:]
	for i := range segs {
		end, start := binary.BigEndian.Uint16(ends[2*i:]), binary.BigEndian.Uint16(starts[2*i:])
		if c > end || c < start {
			continue
		}
		delta, ro := binary.BigEndian.Uint16(deltas[2*i:]), int(binary.BigEndian.Uint16(offsets[2*i:]))
		if ro == 0 {
			return c+delta != 0
		}
		at := 16 + 6*segs + 2*i + ro + 2*int(c-start)
		if at+2 > len(sub) {
			return false
		}
		g := binary.BigEndian.Uint16(sub[at:])
		return g != 0 && g+delta != 0
	}
	return false
}

// covers12 reports whether a subtable of the format 12 maps r to a glyph.
func covers12(sub []byte, r rune) bool {
	if len(sub) < 16 {
		return false
	}
	n := int(binary.BigEndian.Uint32(sub[12:]))
	for i := range n {
		g := 16 + 12*i
		if g+12 > len(sub) {
			return false
		}
		start, end := rune(binary.BigEndian.Uint32(sub[g:])), rune(binary.BigEndian.Uint32(sub[g+4:]))
		if r >= start && r <= end {
			return binary.BigEndian.Uint32(sub[g+8:])+uint32(r-start) != 0
		}
	}
	return false
}
