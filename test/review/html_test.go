package review

import (
	"bytes"
	"image"
	"strings"
	"testing"
	"time"
)

func TestWriteHTML(t *testing.T) {
	rep := &Report{
		Created: time.Date(2026, 11, 17, 8, 30, 0, 0, time.UTC), DPI: 96, Pages: 3, Timeout: "5m0s",
		Counts: map[string]int{StatusOK: 1},
		Docs: []*DocReport{{
			Entry: Entry{Name: "pdf/a<b>.pdf", Kind: "pdf", URL: "https://example.com/a.pdf", License: "MIT", Source: "example"},
			ID:    "001-pdf__a_b_.pdf", Status: StatusOK, Pages: 1, RefPages: 1, Worst: 1.5,
			RefNames: []string{"MuPDF"},
			Rows: []*PageRow{{
				Page: 1, RefsDiff: -1,
				Ivy: &Cell{Name: "ivy", Image: "d/001/p1-ivy.png", Thumb: "d/001/p1-ivy.jpg", Size: image.Pt(816, 1056), MS: 12, Status: StatusOK},
				Refs: []*Cell{{
					Name: "MuPDF", Image: "d/001/p1-mupdf.png", Thumb: "d/001/p1-mupdf.jpg",
					Diff: "d/001/p1-mupdf-diff.png", DiffThumb: "d/001/p1-mupdf-diff.jpg", Size: image.Pt(816, 1056),
					Metric: &Metric{Mean: 3.25, Blurred: 1.5, Size: image.Pt(816, 1056), IvySize: image.Pt(816, 1056)},
				}},
			}},
		}},
	}
	var buf bytes.Buffer
	if err := WriteHTML(&buf, rep); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`<td class="name"><a href="#001-pdf__a_b_.pdf">pdf/a&lt;b&gt;.pdf</a></td><td>pdf</td><td><span class="badge ok">ok</span></td><td class="n">1 / 1</td><td class="n good">1.50</td>`,
		`<h2>pdf/a&lt;b&gt;.pdf <span class="badge ok">ok</span></h2>`,
		`<div class="row" style="--cols:2">`,
		`<a class="plain" href="d/001/p1-ivy.png"><img loading="lazy" src="d/001/p1-ivy.jpg" width="816" height="1056" alt="page of ivy"></a>`,
		`<a class="diff" href="d/001/p1-mupdf-diff.png"><img loading="lazy" src="d/001/p1-mupdf-diff.jpg" width="816" height="1056" alt="difference from MuPDF"></a>`,
		`<div class="m"><span class="good">blurred 1.50</span> · mean 3.25 · 816×1056</div>`,
		`<a href="https://example.com/a.pdf">https://example.com/a.pdf</a>`,
		`<option>pdf</option>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected the html to hold:\n%s", want)
		}
	}
	if strings.Contains(out, "<b>") {
		t.Error("expected the name to be escaped")
	}
	if t.Failed() {
		t.Log(out)
	}
}
