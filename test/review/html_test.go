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
		Counts: map[string]int{StatusOK: 2},
		Docs: []*DocReport{{
			Entry: Entry{Name: "pdf/a<b>.pdf", Kind: "pdf", URL: "https://example.com/a.pdf", License: "MIT", Source: "example"},
			ID:    "001-pdf__a_b_.pdf", Status: StatusOK, Pages: 1, RefPages: 1, Worst: 1.5, IvyPDFWorst: -1,
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
		}, {
			Entry: Entry{Name: "docx/b.docx", Kind: "docx", URL: "repo:docx/testdata/b.docx"},
			ID:    "002-docx__b.docx", Status: StatusOK, Pages: 2, RefPages: 2, Worst: 4.5, IvyPDFWorst: 6.25,
			IvyPDFStatus: StatusPartial, IvyPDFError: "page 2: rendering: a font is missing",
			Reference: "LibreOffice", RefNames: []string{"MuPDF"},
			Rows: []*PageRow{{
				Page: 1, RefsDiff: -1,
				Ivy: &Cell{Name: "ivy", Image: "d/002/p1-ivy.png", Thumb: "d/002/p1-ivy.jpg", Size: image.Pt(816, 1056), Status: StatusOK},
				IvyPDF: &Cell{
					Name: ivyPDFName, Image: "d/002/p1-ivypdf.png", Thumb: "d/002/p1-ivypdf.jpg",
					Diff: "d/002/p1-ivypdf-diff.png", DiffThumb: "d/002/p1-ivypdf-diff.jpg", Size: image.Pt(816, 1056),
					Status: StatusPartial, Error: "rendering: a font is missing",
					Metric: &Metric{Mean: 7.5, Blurred: 6.25, Size: image.Pt(816, 1056), IvySize: image.Pt(816, 1056)},
				},
				Refs: []*Cell{{
					Name: "MuPDF", Image: "d/002/p1-mupdf.png", Thumb: "d/002/p1-mupdf.jpg", Size: image.Pt(816, 1056),
					Metric: &Metric{Mean: 9, Blurred: 4.5, Size: image.Pt(816, 1056), IvySize: image.Pt(816, 1056)},
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
		// the column ivy PDF: in the summary, in the header, and between
		// ivy and the references, with its metric against MuPDF
		`<th data-k="worst" data-num>Worst</th><th data-k="ivypdf" data-num>ivy PDF vs MuPDF</th>`,
		`<td class="n fair">4.50</td><td class="n bad">6.25</td>`,
		`<td class="n good">1.50</td><td class="n none">-</td>`,
		`<p class="err">ivy PDF: page 2: rendering: a font is missing</p>`,
		`<div class="row" style="--cols:3">`,
		`alt="page of ivy"></a>
<div class="m">816×1056</div>
</figure>
<figure class="ref ivypdf"><figcaption>ivy PDF</figcaption>
<a class="plain" href="d/002/p1-ivypdf.png"><img loading="lazy" src="d/002/p1-ivypdf.jpg" width="816" height="1056" alt="page of ivy PDF"></a>
<a class="diff" href="d/002/p1-ivypdf-diff.png"><img loading="lazy" src="d/002/p1-ivypdf-diff.jpg" width="816" height="1056" alt="difference of ivy PDF from MuPDF"></a>
<div class="m"><span class="bad">vs MuPDF: blurred 6.25</span> · mean 7.50 · 816×1056 · <span class="partial">partial</span></div>
<div class="ce">rendering: a font is missing</div></figure>
<figure class="ref"><figcaption>MuPDF</figcaption>
<a class="plain" href="d/002/p1-mupdf.png">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected the html to hold:\n%s", want)
		}
	}
	if strings.Count(out, `<figure class="ref ivypdf">`) != 1 {
		t.Error("expected one cell of ivy PDF, for the docx file only")
	}
	if strings.Contains(out, "<b>") {
		t.Error("expected the name to be escaped")
	}
	if t.Failed() {
		t.Log(out)
	}
}
