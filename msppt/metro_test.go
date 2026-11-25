package msppt

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/drawingml"
)

// metroBlob returns a metroBlob that holds the shape xml.
func metroBlob(t *testing.T, xml string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(metroShapeXML)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(xml)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestMetroMargins checks that the margins of the paragraphs of a metroBlob
// go over the margins of the text, as page 42 of custom-geometry.ppt shows
// them, and that a metroBlob with another number of paragraphs changes
// nothing.
func TestMetroMargins(t *testing.T) {
	blob := metroBlob(t, `<p:sp xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" `+
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:txBody><a:bodyPr/>`+
		`<a:p><a:pPr marL="342900" indent="-342900"/><a:r><a:t>a</a:t></a:r></a:p>`+
		`<a:p><a:pPr marL="571500" indent="-342900"/><a:r><a:t>b</a:t></a:r></a:p>`+
		`<a:p><a:pPr algn="l"/></a:p></p:txBody></p:sp>`)
	paras := metroParagraphs(blob, ivy.Limits{})
	if len(paras) != 3 {
		t.Fatalf("expected 3 paragraphs, got %d", len(paras))
	}
	tb := &drawingml.TextBody{Paragraphs: make([]drawingml.Paragraph, 3)}
	tb.Paragraphs[2].Style.MarL.Set(27)
	setMetroMargins(tb, paras)
	for i, want := range [][2]float64{{27, -27}, {45, -27}, {27, 0}} {
		st := tb.Paragraphs[i].Style
		if st.MarL.V != want[0] || st.Indent.V != want[1] {
			t.Errorf("paragraph %d: expected %v, got %v and %v", i, want, st.MarL.V, st.Indent.V)
		}
	}
	tb = &drawingml.TextBody{Paragraphs: make([]drawingml.Paragraph, 2)}
	setMetroMargins(tb, paras)
	if tb.Paragraphs[0].Style.MarL.OK {
		t.Error("expected no margin from a metroBlob of another text")
	}
	if metroParagraphs([]byte("PK not a zip"), ivy.Limits{}) != nil {
		t.Error("expected nothing of a broken metroBlob")
	}
}
