package graphics

import (
	"strings"

	"github.com/xo/ivy/internal/hayro/pdffont"
	"github.com/xo/ivy/ps/internal/interp"
)

// isoLatin1High holds the names of the codes 0o220 to 0o377 of
// ISOLatin1Encoding (PLRM E.5).
const isoLatin1High = `dotlessi grave acute circumflex tilde macron breve dotaccent
dieresis .notdef ring cedilla .notdef hungarumlaut ogonek caron
space exclamdown cent sterling currency yen brokenbar section
dieresis copyright ordfeminine guillemotleft logicalnot hyphen registered macron
degree plusminus twosuperior threesuperior acute mu paragraph periodcentered
cedilla onesuperior ordmasculine guillemotright onequarter onehalf threequarters questiondown
Agrave Aacute Acircumflex Atilde Adieresis Aring AE Ccedilla
Egrave Eacute Ecircumflex Edieresis Igrave Iacute Icircumflex Idieresis
Eth Ntilde Ograve Oacute Ocircumflex Otilde Odieresis multiply
Oslash Ugrave Uacute Ucircumflex Udieresis Yacute Thorn germandbls
agrave aacute acircumflex atilde adieresis aring ae ccedilla
egrave eacute ecircumflex edieresis igrave iacute icircumflex idieresis
eth ntilde ograve oacute ocircumflex otilde odieresis divide
oslash ugrave uacute ucircumflex udieresis yacute thorn ydieresis`

// standardNames returns the 256 names of StandardEncoding.
func standardNames() []string {
	names := make([]string, 256)
	for i := range names {
		n, ok := pdffont.StandardEncodingName(uint8(i))
		if !ok || n == "" {
			n = ".notdef"
		}
		names[i] = n
	}
	return names
}

// isoLatin1Names returns the 256 names of ISOLatin1Encoding.
func isoLatin1Names() []string {
	names := standardNames()
	for i := 128; i < 0o220; i++ {
		names[i] = ".notdef"
	}
	names[0o47] = "quoteright"
	names[0o55] = "minus"
	names[0o140] = "quoteleft"
	for i, n := range strings.Fields(isoLatin1High) {
		names[0o220+i] = n
	}
	return names
}

// setupEncodings defines StandardEncoding, ISOLatin1Encoding and the
// encodings of Symbol and ZapfDingbats in systemdict.
func (m *Machine) setupEncodings() {
	in := m.in
	old := in.SetGlobal(true)
	defer in.SetGlobal(old)
	define := func(name string, names []string) {
		elems := make([]interp.Object, len(names))
		for i, n := range names {
			elems[i] = interp.NameObj(n)
		}
		arr, err := in.NewArray(elems)
		if err != nil {
			return
		}
		in.SystemDict.PutName(name, arr)
		in.Category("Encoding").PutName(name, arr)
	}
	define("StandardEncoding", standardNames())
	define("ISOLatin1Encoding", isoLatin1Names())
	builtin := func(get func(uint8) (string, bool)) []string {
		names := make([]string, 256)
		for i := range names {
			n, ok := get(uint8(i))
			if !ok || n == "" {
				n = ".notdef"
			}
			names[i] = n
		}
		return names
	}
	define("SymbolEncoding", builtin(pdffont.SymbolEncodingName))
	define("DingbatsEncoding", builtin(pdffont.DingbatsEncodingName))
}
