package xps

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xo/magic/container"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// openXPSFile builds an OpenXPS package with one empty page, with the
// namespaces of ECMA-388.
func openXPSFile() []byte {
	const ns = "http://schemas.openxps.org/oxps/v1.0"
	return fixture.Zip(
		fixture.Entry{Name: "_rels/.rels", Data: []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Type="` + ns + `/fixedrepresentation" Target="/seq.fdseq" Id="R1"/></Relationships>`)},
		fixture.Entry{Name: "seq.fdseq", Data: []byte(`<FixedDocumentSequence xmlns="` + ns + `">` +
			`<DocumentReference Source="Documents/doc.fdoc"/></FixedDocumentSequence>`)},
		fixture.Entry{Name: "Documents/doc.fdoc", Data: []byte(`<FixedDocument xmlns="` + ns + `">` +
			`<PageContent Source="Pages/1.fpage"/></FixedDocument>`)},
		fixture.Entry{Name: "Documents/Pages/1.fpage", Data: []byte(`<FixedPage xmlns="` + ns + `" Width="96" Height="96"/>`)},
	)
}

// TestDetect finds the type of XPS and OpenXPS files as a program does, with
// magic and its package container (magic D65). Format lists the MIME type
// of each, and opens each.
func TestDetect(t *testing.T) {
	tests := map[string]struct {
		buf  []byte
		want container.Format
	}{
		"xps":     {pkgFile(""), container.XPS},
		"openxps": {openXPSFile(), container.OpenXPS},
	}
	for _, name := range []string{"example.xps", "xps-document-example.xps"} {
		buf, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		tests[name] = struct {
			buf  []byte
			want container.Format
		}{buf, container.XPS}
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mime, f := fixture.DetectMIME(t, tt.buf)
			if f != tt.want {
				t.Errorf("expected %v, got: %v", tt.want, f)
			}
			if !slices.Contains(Format.Mimes, mime) {
				t.Errorf("expected Format to list %q", mime)
			}
			d, err := Format.Open(t.Context(), bytes.NewReader(tt.buf), ivy.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Close(); err != nil {
				t.Error(err)
			}
		})
	}
}
