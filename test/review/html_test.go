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
			}, {
				Page: 1, Box: BoxMedia, RefsDiff: 0.75,
				Ivy:  &Cell{Name: "ivy", Status: StatusUnsupported, Error: "ivy cannot render the MediaBox yet (D31)"},
				Refs: []*Cell{{Name: "MuPDF", Image: "d/001/p1-media-mupdf.png", Thumb: "d/001/p1-media-mupdf.jpg", Size: image.Pt(900, 1100)}},
			}},
			MediaBoxWorst: new(-1.0),
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
		}, {
			// a file of Office with the reference of Microsoft Office, a page
			// of another size, and a known fault of LibreOffice
			Entry: Entry{Name: "vsdx/c.vsdx", Kind: "vsdx", URL: "repo:vsdx/testdata/c.vsdx"},
			ID:    "003-vsdx__c.vsdx", Status: StatusOK, Pages: 1, RefPages: 1, Worst: 1.25, IvyPDFWorst: -1,
			Reference: "LibreOffice", RefNames: []string{"Microsoft Office - MuPDF", "Microsoft Office - Chrome", "LibreOffice - MuPDF"},
			WorstBy:         map[string]float64{"msoffice": 1.25, "soffice": 7},
			WorstByRenderer: map[string]float64{"msoffice/mupdf": 1.25, "msoffice/chrome": 2.5, "soffice/mupdf": 7},
			Externals:       []ExternalRef{{ID: "msoffice", Name: "Microsoft Office", File: "msoffice/vsdx/c.vsdx.pdf", Pages: 1}},
			SizeDiffers:     1,
			Rows: []*PageRow{{
				Page: 1, RefsDiff: 7, Disagrees: true, Primary: "msoffice", SizeDiffers: true,
				Ivy: &Cell{Name: "ivy", Image: "d/003/p1-ivy.png", Thumb: "d/003/p1-ivy.jpg", Size: image.Pt(816, 1056), Status: StatusOK},
				Refs: []*Cell{{
					Name: "MuPDF", Ref: "msoffice", RefName: "Microsoft Office", Renderer: "mupdf", Image: "d/003/p1-msoffice-mupdf.png",
					Thumb: "d/003/p1-msoffice-mupdf.jpg", Size: image.Pt(1056, 816),
					Metric: &Metric{Mean: 2, Blurred: 1.25, Size: image.Pt(1056, 816), IvySize: image.Pt(816, 1056)},
				}, {
					Name: "Chrome", Ref: "msoffice", RefName: "Microsoft Office", Renderer: "chrome", Image: "d/003/p1-msoffice-chrome.png",
					Thumb: "d/003/p1-msoffice-chrome.jpg", Size: image.Pt(1056, 816),
					Metric: &Metric{Mean: 3, Blurred: 2.5, Size: image.Pt(1056, 816), IvySize: image.Pt(816, 1056)},
				}, {
					Name: "MuPDF", Ref: "soffice", Renderer: "mupdf", RefName: "LibreOffice", Image: "d/003/p1-mupdf.png",
					Thumb: "d/003/p1-mupdf.jpg", Size: image.Pt(1056, 816), Fault: "LibreOffice draws square corners.",
					Metric: &Metric{Mean: 9, Blurred: 7, Size: image.Pt(1056, 816), IvySize: image.Pt(816, 1056)},
				}},
			}},
		}, {
			Entry: Entry{Name: "doc/d.doc", Kind: "doc", URL: "repo:msdoc/testdata/d.doc"},
			ID:    "004-doc__d.doc", Status: StatusError, Error: "opening: the file is not a document", Pages: -1, RefPages: -1, Worst: -1, IvyPDFWorst: -1,
		}},
		// a file that ivy did not open counts as a failure of its tab
		Kinds: []KindSummary{{Kind: "docx", Documents: 1, Pages: 1}, {Kind: "vsdx", Documents: 1, Pages: 1, SizeDiffers: 1}},
	}
	var buf bytes.Buffer
	if err := WriteHTML(&buf, rep); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`<td class="name"><a href="#001-pdf__a_b_.pdf">pdf/a&lt;b&gt;.pdf</a></td><td>pdf</td><td><span class="badge ok">ok</span></td><td class="n">1 / 1</td><td class="n good">1.50</td><td></td><td class="n"></td><td class="n none">-</td>`,
		`<h2>pdf/a&lt;b&gt;.pdf <span class="badge ok">ok</span></h2>`,
		`<div class="row" style="--cols:2">`,
		`<a class="plain" href="d/001/p1-ivy.png"><img loading="lazy" src="d/001/p1-ivy.jpg" width="816" height="1056" alt="page of ivy"></a>`,
		`<a class="diff" href="d/001/p1-mupdf-diff.png"><img loading="lazy" src="d/001/p1-mupdf-diff.jpg" width="816" height="1056" alt="difference from MuPDF"></a>`,
		`<div class="m"><span class="good">blurred 1.50</span> · mean 3.25 · 816×1056</div>`,
		`<a href="https://example.com/a.pdf">https://example.com/a.pdf</a>`,
		`<option>pdf</option>`,
		// the row of the MediaBox, and its worst in the header
		`<div class="rowhead">Page 1, MediaBox <span class="m">the references differ by 0.75</span> <button type="button" class="rowdiff">differences</button></div>`,
		`<span class="unsupported">unsupported</span></div>
<div class="ce">ivy cannot render the MediaBox yet (D31)</div>`,
		` · MediaBox: worst <span class="none">-</span></p>`,
		// the column ivy PDF: in the summary, in the header, and between
		// ivy and the references, with its metric against MuPDF
		`<th data-k="worst" data-num>Worst</th><th data-k="byref">By reference</th><th data-k="sizediff" data-num>Size differs</th><th data-k="ivypdf" data-num>ivy PDF vs MuPDF</th>`,
		`<td class="n fair">4.50</td><td></td><td class="n"></td><td class="n bad">6.25</td>`,
		// Microsoft Office: the primary reference, the numbers of each
		// reference, the page of another size and the fault
		`<td class="n good">1.25</td><td>Microsoft Office <span class="good">1.25</span> · LibreOffice <span class="bad">7.00</span></td><td class="n bad">1</td>`,
		`<p class="meta">worst by reference: Microsoft Office <span class="good">1.25</span> · LibreOffice <span class="bad">7.00</span> · Microsoft Office: 1 pages, msoffice/vsdx/c.vsdx.pdf</p>`,
		`<span class="warnbadge">references disagree (7.00)</span> <span class="m">primary: Microsoft Office</span> <span class="bad">the size differs from Microsoft Office</span>`,
		`<figure class="ivy sizediff"><figcaption>ivy</figcaption>`,
		`<figure class="ref"><figcaption>Microsoft Office - MuPDF</figcaption>`,
		`<figure class="ref"><figcaption>Microsoft Office - Chrome</figcaption>`,
		`<figure class="ref fault"><figcaption>LibreOffice - MuPDF</figcaption>`,
		`<p class="meta">worst by column: Microsoft Office - MuPDF <span class="good">1.25</span> · Microsoft Office - Chrome <span class="fair">2.50</span> · LibreOffice - MuPDF <span class="bad">7.00</span></p>`,
		// the tabs: All first, then each group of kinds with documents, with
		// the count of the documents and of the failures
		`<nav class="tabs" role="tablist" aria-label="the kinds of the documents" hidden>
<button type="button" role="tab" data-tab="all" aria-selected="true">All <span class="n">4, 1 failed</span></button>
<button type="button" role="tab" data-tab="pdf" aria-controls="tab-pdf" aria-selected="false">PDF <span class="n">1</span></button>
<button type="button" role="tab" data-tab="word" aria-controls="tab-word" aria-selected="false">Word <span class="n">2, 1 failed</span></button>
<button type="button" role="tab" data-tab="office" aria-controls="tab-office" aria-selected="false">Other Office <span class="n">1</span></button>
</nav>`,
		`<section class="tab" id="tab-word" data-tab="word" role="tabpanel">
<h2 class="tabhead">Word</h2>
<p class="muted">2 documents · <span class="bad">1 failed</span> · mean worst <span class="fair">4.50</span> over 1 documents</p>
<table class="sum">`,
		`<div class="fnote">known fault of the reference, not counted: LibreOffice draws square corners.</div>`,
		`<tr><td>vsdx</td><td>1</td><td>1</td><td class="bad">1</td></tr>`,
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
	// each tab holds the sections of its documents
	for id, docs := range map[string][]string{"pdf": {"001-pdf__a_b_.pdf"}, "word": {"002-docx__b.docx", "004-doc__d.doc"}, "office": {"003-vsdx__c.vsdx"}} {
		start := strings.Index(out, `<section class="tab" id="tab-`+id+`"`)
		if start < 0 {
			t.Errorf("expected the tab %s", id)
			continue
		}
		end := strings.Index(out[start+1:], `<section class="tab" `)
		if end < 0 {
			end = len(out) - start - 1
		}
		tab := out[start : start+1+end]
		for _, d := range docs {
			if !strings.Contains(tab, `<section class="doc" id="`+d+`"`) || !strings.Contains(tab, `<a href="#`+d+`">`) {
				t.Errorf("expected the tab %s to hold the row and the section of %s", id, d)
			}
		}
	}
	if strings.Count(out, `<section class="tab" `) != 3 || strings.Contains(out, `data-tab="excel"`) {
		t.Error("expected 3 tabs, and no tab without documents")
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
