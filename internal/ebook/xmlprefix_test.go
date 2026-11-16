package ebook

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/ivy"
)

func TestParseXMLUndeclaredPrefix(t *testing.T) {
	for name, tt := range map[string]struct {
		xml string
		// href is the value of the attribute href of the element a, in
		// any namespace.
		href string
	}{
		"prefix l": {`<?xml version="1.0"?><root><a l:href="#x"/></root>`, "#x"},
		"xlink":    {`<root><a xlink:href="#y"/></root>`, "#y"},
		"both": {`<?xml version="1.0"?>
<!-- a comment -->
<!DOCTYPE root [ <!ENTITY e "v"> ]>
<root lang="ru"><a l:href="#z" xlink:title="t"/></root>`, "#z"},
		"other prefix":  {`<root><foo:a foo:href="#w"/></root>`, "#w"},
		"declared":      {`<root xmlns:l="urn:l"><a l:href="#v"/></root>`, "#v"},
		"html entities": {`<root><a l:href="#u">&nbsp;</a></root>`, "#u"},
	} {
		t.Run(name, func(t *testing.T) {
			doc, err := ParseXML([]byte(tt.xml), ivy.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			for n := range doc.RootElement().Descendants() {
				if n.IsElement() && Local(n) == "a" {
					if got := Attr(n, "href"); got != tt.href {
						t.Errorf("expected the href %q, got: %q", tt.href, got)
					}
					return
				}
			}
			t.Error("expected the element a")
		})
	}
}

func TestParseXMLTooManyPrefixes(t *testing.T) {
	var b strings.Builder
	b.WriteString("<root>")
	for i := range maxUndeclared + 1 {
		b.WriteString("<a p" + string(rune('a'+i)) + ":x=\"1\"/>")
	}
	b.WriteString("</root>")
	if _, err := ParseXML([]byte(b.String()), ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
	for _, bad := range []string{`<a l:href="x"`, `<?xml version="1.0"?><!-- open`, `l:href`} {
		if _, err := ParseXML([]byte(bad), ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
			t.Errorf("%q: expected ivy.ErrFormat, got: %v", bad, err)
		}
	}
}

func TestDeclarePrefix(t *testing.T) {
	for _, tt := range []struct {
		text, want string
		ok         bool
	}{
		{`<r>`, `<r xmlns:l="http://www.w3.org/1999/xlink">`, true},
		{`<?xml version="1.0"?><r a="1"/>`, `<?xml version="1.0"?><r xmlns:l="http://www.w3.org/1999/xlink" a="1"/>`, true},
		{`<!DOCTYPE r [<!ENTITY e "<x>">]><r>`, `<!DOCTYPE r [<!ENTITY e "<x>">]><r xmlns:l="http://www.w3.org/1999/xlink">`, true},
		{`<r xmlns:l="u">`, `<r xmlns:l="u">`, false},
		{`text`, `text`, false},
		{`<`, `<`, false},
	} {
		got, ok := declarePrefix(tt.text, "l")
		if got != tt.want || ok != tt.ok {
			t.Errorf("declarePrefix(%q): expected %q %t, got: %q %t", tt.text, tt.want, tt.ok, got, ok)
		}
	}
}
