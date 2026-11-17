package review

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRender renders four small files of the repository with ivy and with
// their references: a PDF file with MuPDF and Poppler, a Word file with
// LibreOffice to PDF and then MuPDF and Poppler, an XPS file with libgxps,
// and a Photoshop file with ImageMagick. ivy renders the PDF file of
// LibreOffice too, as the column ivy PDF. As each test of the test module,
// it fails when a program is missing, and names it (D13).
func TestRender(t *testing.T) {
	for _, name := range []string{"mutool", "pdftoppm", "pdftocairo", "pdfinfo", "xpstopng", "magick", "fc-list", "soffice"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("oracle %s is not installed: %v", name, err)
		}
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	entries := []Entry{
		{Name: "pdf/clip.pdf", Kind: "pdf", URL: "repo:internal/hayro/interpret/testdata/clip_path_nested.pdf", Size: -1},
		{Name: "docx/sample.docx", Kind: "docx", URL: "repo:docx/testdata/sample.docx", Size: -1},
		{Name: "xps/example.xps", Kind: "xps", URL: "repo:xps/testdata/example.xps", Size: -1},
		{Name: "psd/rgb8bit.psd", Kind: "psd", URL: "repo:psd/testdata/rgb8bit.psd", Size: -1},
		{Name: "missing.pdf", Kind: "pdf", URL: "https://example.com/missing.pdf", Size: -1},
	}
	out := filepath.Join(t.TempDir(), "out")
	cache := t.TempDir()
	// LibreOffice converts in a temporary folder, and the conversion moves
	// into the cache: TMPDIR on another file system must not break the move
	if other := otherFileSystem(t, cache); other != "" {
		t.Setenv("TMPDIR", other)
	}
	opts := RenderOptions{
		Repo: repo, Cache: cache, Out: out, Pages: 1, DPI: 72, Jobs: 2, Timeout: time.Minute,
	}
	rep, err := Render(t.Context(), entries, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Docs) != len(entries) {
		t.Fatalf("expected %d documents, got: %d", len(entries), len(rep.Docs))
	}
	refs := map[string][]string{
		"pdf/clip.pdf":     {"MuPDF", "Poppler Splash", "Poppler Cairo"},
		"docx/sample.docx": {"MuPDF", "Poppler Splash", "Poppler Cairo"},
		"xps/example.xps":  {"libgxps"},
		"psd/rgb8bit.psd":  {"ImageMagick"},
	}
	for _, d := range rep.Docs {
		if d.Name == "missing.pdf" {
			if d.Status != StatusMissing {
				t.Errorf("expected %s to be missing, got: %s", d.Name, d.Status)
			}
			continue
		}
		if d.Status != StatusOK || d.RefError != "" {
			t.Errorf("%s: expected ok, got: %s: %s %s", d.Name, d.Status, d.Error, d.RefError)
			continue
		}
		if len(d.Rows) != 1 {
			t.Errorf("%s: expected 1 page, got: %d", d.Name, len(d.Rows))
			continue
		}
		row := d.Rows[0]
		for _, p := range []string{row.Ivy.Image, row.Ivy.Thumb} {
			if _, err := os.Stat(filepath.Join(out, p)); err != nil {
				t.Errorf("%s: expected the image of ivy: %v", d.Name, err)
			}
		}
		checkIvyPDF(t, d, out)
		if len(row.Refs) != len(refs[d.Name]) {
			t.Errorf("%s: expected the references %v, got: %d", d.Name, refs[d.Name], len(row.Refs))
			continue
		}
		for i, c := range row.Refs {
			if c.Name != refs[d.Name][i] || c.Error != "" || c.Metric == nil {
				t.Errorf("%s: expected a metric of %s, got: %+v", d.Name, refs[d.Name][i], c)
				continue
			}
			// the pages look the same: TestPDF and TestXPS allow 8
			if c.Metric.Blurred > 8 {
				t.Errorf("%s: expected a blurred difference from %s of at most 8, got: %.2f", d.Name, c.Name, c.Metric.Blurred)
			}
			for _, p := range []string{c.Image, c.Thumb, c.Diff, c.DiffThumb} {
				if _, err := os.Stat(filepath.Join(out, p)); err != nil {
					t.Errorf("%s: expected the image %q of %s: %v", d.Name, p, c.Name, err)
				}
			}
		}
	}
	for _, name := range []string{"index.html", "report.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Error(err)
		}
	}

	// a second run writes over the report, and takes the references from
	// the cache
	opts.Match = nil
	opts.Kinds = []string{"psd"}
	rep, err = Render(t.Context(), entries, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Docs) != 1 || rep.Docs[0].Status != StatusOK {
		t.Errorf("expected the psd file again, got: %+v", rep.Docs)
	}

	// a folder of other files is refused
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "keep.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	opts.Out = other
	if _, err := Render(t.Context(), entries, opts); err == nil || !strings.Contains(err.Error(), "no report.json") {
		t.Errorf("expected a folder of other files to be refused, got: %v", err)
	}
}

// checkIvyPDF checks the column ivy PDF of the document d of TestRender: a
// page and a metric against MuPDF for the Word file, which LibreOffice
// converts to PDF, and no column for the other files.
func checkIvyPDF(t *testing.T, d *DocReport, out string) {
	t.Helper()
	c := d.Rows[0].IvyPDF
	if d.Kind != "docx" {
		if c != nil || d.IvyPDFWorst != -1 || d.IvyPDFStatus != "" {
			t.Errorf("%s: expected no column ivy PDF, got: %+v", d.Name, c)
		}
		return
	}
	if c == nil {
		t.Errorf("%s: expected a column ivy PDF", d.Name)
		return
	}
	if c.Name != ivyPDFName || c.Status != StatusOK || c.Error != "" || c.Metric == nil {
		t.Errorf("%s: expected a page of ivy PDF with a metric, got: %+v", d.Name, c)
		return
	}
	if d.IvyPDFStatus != StatusOK || d.IvyPDFError != "" || d.IvyPDFWorst != c.Metric.Blurred {
		t.Errorf("%s: expected the status ok and the worst %.2f of ivy PDF, got: %s %.2f %s",
			d.Name, c.Metric.Blurred, d.IvyPDFStatus, d.IvyPDFWorst, d.IvyPDFError)
	}
	// ivy and MuPDF draw the same PDF file: TestPDF allows 8
	if c.Metric.Blurred > 8 || c.Metric.Scaled() {
		t.Errorf("%s: expected ivy PDF to look as MuPDF, got: %+v", d.Name, c.Metric)
	}
	for _, p := range []string{c.Image, c.Thumb, c.Diff, c.DiffThumb} {
		if _, err := os.Stat(filepath.Join(out, p)); err != nil {
			t.Errorf("%s: expected the image %q of ivy PDF: %v", d.Name, p, err)
		}
	}
}

// boxPDF returns a PDF file of one page whose MediaBox, 200 by 200 points,
// is larger than its CropBox, from 50 to 150. A blue square fills the
// CropBox, and a black mark from 10 to 30 lies outside it.
func boxPDF() []byte {
	content := "0 0 1 rg 50 50 100 100 re f 0 g 10 10 20 20 re f\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /CropBox [50 50 150 150] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

// TestRenderMediaBox renders a PDF file whose MediaBox is larger than its
// CropBox. The row of the page shows the CropBox with each reference. A
// second row shows the MediaBox, where Poppler draws the mark outside the
// CropBox, and where ivy is unsupported until the package pdf has the
// option of D31.
func TestRenderMediaBox(t *testing.T) {
	for _, name := range []string{"mutool", "pdftoppm", "pdftocairo", "pdfinfo"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("oracle %s is not installed: %v", name, err)
		}
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	e := Entry{Name: "pdf/boxes.pdf", Kind: "pdf", URL: "https://example.com/boxes.pdf", Size: -1}
	path := e.Path(repo, cache)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, boxPDF(), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	rep, err := Render(t.Context(), []Entry{e}, RenderOptions{
		Repo: repo, Cache: cache, Out: out, Pages: 1, DPI: 72, Jobs: 1, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	d := rep.Docs[0]
	if d.Status != StatusOK || d.RefError != "" || len(d.Rows) != 2 {
		t.Fatalf("expected ok and 2 rows, got: %s %s %s, %d rows", d.Status, d.Error, d.RefError, len(d.Rows))
	}
	crop, media := d.Rows[0], d.Rows[1]
	if crop.Box != "" || media.Box != BoxMedia || media.Page != 1 {
		t.Errorf("expected a row of the CropBox, then one of the MediaBox, got: %q %q", crop.Box, media.Box)
	}
	// each reference draws the CropBox: 100 by 100 points at 72 dpi
	for _, c := range crop.Refs {
		if c.Error != "" || c.Size != image.Pt(100, 100) || c.Metric == nil {
			t.Errorf("expected %s at the CropBox, 100×100 with a metric, got: %+v", c.Name, c)
		}
	}
	if d.Worst < 0 || d.Worst > 8 {
		t.Errorf("expected ivy to look as the references at the CropBox, got: %.2f", d.Worst)
	}
	// each reference draws the MediaBox, and ivy cannot yet
	if media.Ivy.Status != StatusUnsupported || media.Ivy.Error != errNoMediaBox.Error() {
		t.Errorf("expected ivy to be unsupported at the MediaBox, got: %+v", media.Ivy)
	}
	if len(media.Refs) != 3 {
		t.Fatalf("expected 3 references at the MediaBox, got: %d", len(media.Refs))
	}
	for _, c := range media.Refs {
		if c.Error != "" || c.Size != image.Pt(200, 200) {
			t.Errorf("expected %s at the MediaBox, 200×200, got: %+v", c.Name, c)
		}
	}
	// Poppler draws the mark, which covers the pixels from 10 to 30, with y
	// down. MuPDF 1.28 gives the image the size of the MediaBox, but clips
	// the page to the CropBox, so the test does not ask it for the mark.
	for _, c := range media.Refs[1:] {
		img, err := readImage(filepath.Join(out, c.Image))
		if err != nil {
			t.Fatal(err)
		}
		if r, g, b, _ := img.At(20, 180).RGBA(); r > 0x2000 || g > 0x2000 || b > 0x2000 {
			t.Errorf("expected %s to draw the mark at the MediaBox, got: %x %x %x", c.Name, r, g, b)
		}
	}
	if d.MediaBoxWorst == nil || *d.MediaBoxWorst != -1 {
		t.Errorf("expected the worst of the MediaBox to be -1 without ivy, got: %v", d.MediaBoxWorst)
	}
	if d.Disagree != 0 || media.Disagrees {
		t.Errorf("expected the row of the MediaBox out of Disagree, got: %d %t", d.Disagree, media.Disagrees)
	}
}

// TestRenderPapersAndOffice renders a CSV file on two papers, with
// LibreOffice on the same paper as ivy (D32). Then it renders a Word file
// with an external reference of Microsoft Office, whose PDF file is a copy
// of the PDF file of LibreOffice, and a known fault of LibreOffice. Office
// is the primary reference, and the fault leaves LibreOffice out of the
// numbers.
func TestRenderPapersAndOffice(t *testing.T) {
	for _, name := range []string{"mutool", "pdftoppm", "pdftocairo", "pdfinfo", "fc-list", "soffice"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("oracle %s is not installed: %v", name, err)
		}
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	csvEntry := Entry{Name: "csv/table.csv", Kind: "csv", URL: "https://example.com/table.csv", Size: -1}
	path := csvEntry.Path(repo, cache)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("name,size\nalpha,1\nbeta,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	papers, err := ParsePapers("default,a4-landscape")
	if err != nil {
		t.Fatal(err)
	}
	opts := RenderOptions{
		Repo: repo, Cache: cache, Out: filepath.Join(t.TempDir(), "out"), Pages: 1, DPI: 72, Jobs: 2,
		Timeout: time.Minute, Papers: papers, SystemFonts: true,
	}
	rep, err := Render(t.Context(), []Entry{csvEntry}, opts)
	if err != nil {
		t.Fatal(err)
	}
	sizes := map[string]image.Point{"csv/table.csv": image.Pt(612, 792), "csv/table.csv [A4 landscape]": image.Pt(842, 596)}
	if len(rep.Docs) != 2 {
		t.Fatalf("expected the CSV file on 2 papers, got: %d documents", len(rep.Docs))
	}
	for _, d := range rep.Docs {
		want, ok := sizes[d.Name]
		if !ok || d.Paper == nil || d.Status != StatusOK || d.RefError != "" || len(d.Rows) != 1 {
			t.Errorf("%s: expected a page on a paper, got: %+v", d.Name, d)
			continue
		}
		row := d.Rows[0]
		if row.Ivy.Size != want {
			t.Errorf("%s: expected ivy at %v, got: %v", d.Name, want, row.Ivy.Size)
		}
		for _, c := range row.Refs {
			if sizeDiffers(c.Size, want) {
				t.Errorf("%s: expected %s at %v, got: %v", d.Name, c.Name, want, c.Size)
			}
		}
		if d.SizeDiffers != 0 || row.SizeDiffers {
			t.Errorf("%s: expected the size of the reference, got %d pages of another size", d.Name, d.SizeDiffers)
		}
	}

	// the reference of Microsoft Office, and a fault of LibreOffice
	docx := Entry{Name: "docx/sample.docx", Kind: "docx", URL: "repo:docx/testdata/sample.docx", Size: -1}
	opts.Out = filepath.Join(t.TempDir(), "out")
	if _, err := Render(t.Context(), []Entry{docx}, opts); err != nil {
		t.Fatal(err)
	}
	pdfs, err := filepath.Glob(filepath.Join(cache, "refs", "*", "soffice-*", "out.pdf"))
	if err != nil || len(pdfs) == 0 {
		t.Fatalf("expected the PDF file of LibreOffice, got: %v %v", pdfs, err)
	}
	var lo string
	for _, p := range pdfs {
		if !strings.Contains(p, "-uno-") {
			lo = p
		}
	}
	office := externals[0].file(cache, docx)
	if err := os.MkdirAll(filepath.Dir(office), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := linkOrCopy(lo, office); err != nil {
		t.Fatal(err)
	}
	opts.Faults = []Fault{{Name: "docx/sample.docx", Ref: "soffice", Note: "a fault"}}
	rep, err = Render(t.Context(), []Entry{docx}, opts)
	if err != nil {
		t.Fatal(err)
	}
	d := rep.Docs[0]
	if len(d.Externals) != 1 || d.Externals[0].ID != "msoffice" || d.Externals[0].Pages < 1 {
		t.Fatalf("expected the reference of Microsoft Office, got: %+v", d.Externals)
	}
	wantNames := []string{"Microsoft Office · MuPDF", "Microsoft Office · Poppler Splash",
		"LibreOffice · MuPDF", "LibreOffice · Poppler Splash", "LibreOffice · Poppler Cairo"}
	if strings.Join(d.RefNames, "|") != strings.Join(wantNames, "|") {
		t.Errorf("expected the references %q, got: %q", wantNames, d.RefNames)
	}
	row := d.Rows[0]
	if row.Primary != "msoffice" || d.Worst != d.WorstBy["msoffice"] || len(d.WorstBy) != 1 {
		t.Errorf("expected Office as the primary reference, and LibreOffice out of the numbers, got: %s %.2f %v",
			row.Primary, d.Worst, d.WorstBy)
	}
	for _, c := range row.Refs {
		if (c.Ref == "soffice") != (c.Fault != "") || c.Error != "" || c.Metric == nil {
			t.Errorf("expected a metric of %s %s, and a fault of LibreOffice only, got: %+v", c.RefName, c.Name, c)
		}
	}
	if !strings.HasPrefix(row.Refs[0].Image, "d/001-docx__sample.docx/p1-msoffice-mupdf") {
		t.Errorf("expected the image of Office to start with msoffice-, got: %q", row.Refs[0].Image)
	}
	if row.Disagrees || row.RefsDiff > 4 {
		t.Errorf("expected the two references of Office to agree, got: %.2f", row.RefsDiff)
	}
}
