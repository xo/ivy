package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRender renders three small files of the repository with ivy and with
// their references: a PDF file with MuPDF and Poppler, an XPS file with
// libgxps, and a Photoshop file with ImageMagick. As each test of the test
// module, it fails when a program is missing, and names it (D13).
func TestRender(t *testing.T) {
	for _, name := range []string{"mutool", "pdftoppm", "pdftocairo", "pdfinfo", "xpstopng", "magick", "fc-list"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("oracle %s is not installed: %v", name, err)
		}
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	entries := []Entry{
		{Name: "pdf/clip.pdf", Kind: "pdf", URL: "repo:pdf/internal/interpret/testdata/clip_path_nested.pdf", Size: -1},
		{Name: "xps/example.xps", Kind: "xps", URL: "repo:xps/testdata/example.xps", Size: -1},
		{Name: "psd/rgb8bit.psd", Kind: "psd", URL: "repo:psd/testdata/rgb8bit.psd", Size: -1},
		{Name: "missing.pdf", Kind: "pdf", URL: "https://example.com/missing.pdf", Size: -1},
	}
	out := filepath.Join(t.TempDir(), "out")
	opts := RenderOptions{
		Repo: repo, Cache: t.TempDir(), Out: out, Pages: 1, DPI: 72, Jobs: 2, Timeout: time.Minute,
	}
	rep, err := Render(t.Context(), entries, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Docs) != len(entries) {
		t.Fatalf("expected %d documents, got: %d", len(entries), len(rep.Docs))
	}
	refs := map[string][]string{
		"pdf/clip.pdf":    {"MuPDF", "Poppler Splash", "Poppler Cairo"},
		"xps/example.xps": {"libgxps"},
		"psd/rgb8bit.psd": {"ImageMagick"},
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
