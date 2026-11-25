package odf

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// formulaXML is the content.xml of a formula object of LibreOffice Math:
// x squared over 2, with its annotation of StarMath.
const formulaXML = `<?xml version="1.0" encoding="UTF-8"?>
<math xmlns="http://www.w3.org/1998/Math/MathML" display="block"><semantics><mrow>` +
	`<mfrac><msup><mi>x</mi><mn>2</mn></msup><mn>2</mn></mfrac></mrow>` +
	`<annotation encoding="StarMath 5.0">{x^2} over 2</annotation></semantics></math>`

// formulaSettings is the settings.xml of a formula object, with a base
// font of 20 points.
const formulaSettings = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-settings xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" ` +
	`xmlns:config="urn:oasis:names:tc:opendocument:xmlns:config:1.0"><office:settings>` +
	`<config:config-item-set config:name="ooo:configuration-settings">` +
	`<config:config-item config:name="BaseFontHeight" config:type="short">20</config:config-item>` +
	`</config:config-item-set></office:settings></office:document-settings>`

func TestTextFormula(t *testing.T) {
	body := `<text:p text:style-name="Standard">Let <draw:frame text:anchor-type="as-char" svg:width="30pt" svg:height="30pt">` +
		`<draw:object xlink:href="./Object 1" xlink:type="simple"/><draw:image xlink:href="./ObjectReplacements/Object 1"/>` +
		`</draw:frame> be</text:p>`
	buf := odtFile(body,
		fixture.Entry{Name: "Object 1/content.xml", Data: []byte(formulaXML)},
		fixture.Entry{Name: "Object 1/settings.xml", Data: []byte(formulaSettings)},
	)
	html, _ := odtHTML(t, buf)
	// The formula is an object that sits on the baseline by its depth,
	// whose height is that of a fraction of 20 points.
	re := regexp.MustCompile(`<img src="ivy-object:math1" alt="" style="width: [0-9.]+pt; height: ([0-9.]+)pt; vertical-align: -[0-9.]+pt"/>`)
	m := re.FindStringSubmatch(html)
	if m == nil {
		t.Fatalf("expected the xhtml to match %s, got:\n%s", re, html)
	}
	if h, _ := strconv.ParseFloat(m[1], 64); h < 30 {
		t.Errorf("expected a formula in display style at 20 points, taller than 30 points, got %s", m[1])
	}
	if strings.Contains(html, "ObjectReplacements") {
		t.Errorf("expected no replacement image, got:\n%s", html)
	}
}
