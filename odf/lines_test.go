package odf

import (
	"strings"
	"testing"
)

// TestTextLineNumbers checks that the text gives the numbers of its lines
// to the layout from text:linenumbering-configuration, in the font of the
// text style of the numbers, that a paragraph style can leave its lines
// out, and that a proportional line spacing puts the extra space below the
// text (libreoffice/odt/ooo96040-2.odt, D56).
func TestTextLineNumbers(t *testing.T) {
	const numbers = `<style:style style:name="LineNumbering" style:family="text"><style:text-properties fo:color="#5382a1" fo:font-size="10pt"/></style:style>`
	const auto = `<style:style style:name="PN" style:family="paragraph" style:parent-style-name="Body"><style:paragraph-properties text:number-lines="false"/></style:style>`
	for _, tt := range []struct {
		name, config, want string
	}{
		{"continuous", `<text:linenumbering-configuration text:style-name="LineNumbering" text:offset="0.5in" text:increment="5" text:number-position="left"/>`, `-ivy-line-numbers: 36pt 5 1 continuous`},
		{"page", `<text:linenumbering-configuration text:restart-on-page="true"/>`, `-ivy-line-numbers: 14.15pt 1 1 page`},
		{"off", `<text:linenumbering-configuration text:number-lines="false" text:increment="5"/>`, ""},
		// The layout draws the numbers on the left only.
		{"right", `<text:linenumbering-configuration text:number-position="right"/>`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			edit := func(s string) string {
				return strings.Replace(s, "</office:styles>", numbers+tt.config+"</office:styles>", 1)
			}
			body := `<text:p text:style-name="Body">Numbered</text:p><text:p text:style-name="PN">Left out</text:p>`
			html, _ := odtHTML(t, odtWith(body, auto, edit))
			_, body0, _ := strings.Cut(html, "<body")
			body0, _, _ = strings.Cut(body0, ">")
			switch {
			case tt.want == "" && strings.Contains(body0, "-ivy-line-numbers"):
				t.Errorf("expected no numbers, got: %s", body0)
			case tt.want != "" && !strings.Contains(body0, tt.want):
				t.Errorf("expected %q, got: %s", tt.want, body0)
			}
			if tt.name == "continuous" && (!strings.Contains(body0, "color: #5382a1") || !strings.Contains(body0, "font-size: 10pt")) {
				t.Errorf("expected the font of the text style of the numbers, got: %s", body0)
			}
			if p := odtParagraph(t, html, "Left out"); !strings.Contains(p, "-ivy-line-numbers: none") {
				t.Errorf("expected a paragraph without numbers, got: %s", p)
			}
			if p := odtParagraph(t, html, "Numbered"); !strings.Contains(p, "-ivy-line-multiple: 1.15") {
				t.Errorf("expected the multiple 1.15, got: %s", p)
			}
		})
	}
	if !strings.Contains(odtStyleSheet, "-ivy-page-top-space: discard") {
		t.Error("expected the style sheet to leave out the space before a paragraph at the top of a page")
	}
}

// TestTextLineMultiple checks that a single line and a proportional line
// spacing put the line gap above their text, and that a line of a fixed
// height does not (D56).
func TestTextLineMultiple(t *testing.T) {
	const auto = `<style:style style:name="P1" style:family="paragraph"><style:paragraph-properties fo:line-height="100%"/></style:style>` +
		`<style:style style:name="P2" style:family="paragraph"><style:paragraph-properties fo:line-height="0.5in"/></style:style>` +
		`<style:style style:name="P3" style:family="paragraph"><style:paragraph-properties fo:line-height="200%"/></style:style>`
	body := `<text:p text:style-name="P1">Single</text:p><text:p text:style-name="P2">Fixed</text:p><text:p text:style-name="P3">Double</text:p>`
	html, _ := odtHTML(t, odtWith(body, auto, nil))
	for text, want := range map[string]string{"Single": "-ivy-line-multiple: 1;", "Double": "-ivy-line-multiple: 2;"} {
		if p := odtParagraph(t, html, text); !strings.Contains(p, want) {
			t.Errorf("expected %q in the paragraph %s, got: %s", want, text, p)
		}
	}
	if p := odtParagraph(t, html, "Fixed"); strings.Contains(p, "-ivy-line-multiple") {
		t.Errorf("expected no multiple for a line of a fixed height, got: %s", p)
	}
}
