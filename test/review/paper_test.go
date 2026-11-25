package review

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
)

func TestParsePapers(t *testing.T) {
	papers, err := ParsePapers(DefaultPapers)
	if err != nil {
		t.Fatal(err)
	}
	want := []Paper{
		{ID: "letter", Name: "Letter", Width: 612, Height: 792},
		{ID: "a4", Name: "A4", Width: 595.28, Height: 841.89},
		{ID: "a4-landscape", Name: "A4 landscape", Width: 841.89, Height: 595.28},
	}
	if !slices.Equal(papers, want) {
		t.Errorf("expected %+v, got: %+v", want, papers)
	}
	papers, err = ParsePapers(" default , Legal-landscape,")
	if err != nil {
		t.Fatal(err)
	}
	if len(papers) != 2 || !papers[0].IsDefault() || papers[1].Size() != ivy.Legal.Landscape() || papers[1].Name != "Legal landscape" {
		t.Errorf("expected default and Legal landscape, got: %+v", papers)
	}
	if papers, err := ParsePapers(""); err != nil || len(papers) != 0 {
		t.Errorf("expected no paper, got: %+v, %v", papers, err)
	}
	for _, bad := range []string{"b5", "a4,a4", "a4-portrait"} {
		if _, err := ParsePapers(bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestExpandJobs(t *testing.T) {
	papers, err := ParsePapers("default,a4-landscape")
	if err != nil {
		t.Fatal(err)
	}
	entries := []Entry{{Name: "a.txt", Kind: "txt"}, {Name: "b.docx", Kind: "docx"}, {Name: "c.csv", Kind: "csv"}, {Name: "d.epub", Kind: "epub"}}
	jobs := expandJobs(entries, papers)
	names := make([]string, 0, len(jobs))
	for _, jb := range jobs {
		names = append(names, jb.name())
	}
	want := []string{"a.txt", "a.txt [A4 landscape]", "b.docx", "c.csv", "c.csv [A4 landscape]", "d.epub", "d.epub [A4 landscape]"}
	if !slices.Equal(names, want) {
		t.Errorf("expected %q, got: %q", want, names)
	}
	if jobs := expandJobs(entries, nil); len(jobs) != len(entries) {
		t.Errorf("expected one job for each entry without papers, got: %d", len(jobs))
	}
}

func TestPaperOptions(t *testing.T) {
	a4l := ivy.A4.Landscape()
	opts := strings.Join(calibreOptions("epub", a4l), " ")
	for _, want := range []string{"--custom-size 841.89x595.28 --unit point", "--preserve-cover-aspect-ratio", "--pdf-page-margin-top 43.2"} {
		if !strings.Contains(opts, want) {
			t.Errorf("epub on A4 landscape: expected %q in %q", want, opts)
		}
	}
	if opts := strings.Join(calibreOptions("epub", ivy.Paper{}), " "); !strings.Contains(opts, "--custom-size 6x9 --unit inch") {
		t.Errorf("epub on the default paper: expected 6x9 inches in %q", opts)
	}
	if opts := strings.Join(calibreOptions("md", ivy.A4), " "); !strings.Contains(opts, "--custom-size 595.28x841.89 --unit point") ||
		!strings.Contains(opts, "--markdown-extensions "+markdownExtensions) || !strings.Contains(opts, "--pdf-page-margin-left 54") {
		t.Errorf("md on A4: expected the size, the extensions and the margins in %q", opts)
	}
	if opts := strings.Join(calibreOptions("txt", ivy.Paper{}), " "); !strings.Contains(opts, "--paper-size letter") {
		t.Errorf("txt on the default paper: expected letter in %q", opts)
	}
	j := &refJob{kind: "csv", paper: a4l}
	if got, want := j.unoPage(), "29700 21000 1905 1905 1778 1778"; got != want {
		t.Errorf("csv on A4 landscape: expected the page %q, got: %q", want, got)
	}
	j.paper = ivy.Paper{}
	if got, want := j.unoPage(), "21590 27940 1905 1905 1778 1778"; got != want {
		t.Errorf("csv on the default paper: expected the page %q, got: %q", want, got)
	}
	if j := (&refJob{kind: "docx", paper: a4l}); j.unoPage() != "" {
		t.Error("docx: expected no page")
	}
}

func TestReadFaults(t *testing.T) {
	faults, err := ReadFaults(strings.NewReader("# a comment\nname\tpage\treference\tnote\n" +
		"a.vsdx\t*\tsoffice\tsquare corners\nb.docx\t2\t*\ta wrong font\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(faults) != 2 || faults[0].Page != 0 || faults[1].Page != 2 || faults[1].Ref != "*" {
		t.Fatalf("expected two faults, got: %+v", faults)
	}
	for _, c := range []struct {
		name string
		page int
		ref  string
		want string
	}{
		{"a.vsdx", 3, "soffice", "square corners"},
		{"a.vsdx", 3, "msoffice", ""},
		{"b.docx", 2, "msoffice", "a wrong font"},
		{"b.docx", 1, "msoffice", ""},
		{"b.docx [A4]", 2, "soffice", ""},
	} {
		if got := faultNote(faults, []string{c.name}, c.page, c.ref); got != c.want {
			t.Errorf("%s page %d %s: expected %q, got: %q", c.name, c.page, c.ref, c.want, got)
		}
	}
	for _, bad := range []string{"", "name\tpage\treference\tnote\na\t0\tsoffice\tx\n", "name\tpage\treference\tnote\na\t1\tsoffice\n"} {
		if _, err := ReadFaults(strings.NewReader(bad)); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
	// the file of the repository reads
	if _, err := LoadFaults("reference-faults.tsv"); err != nil {
		t.Error(err)
	}
}

// page returns a white page of the size w by h with a black square at x.
func page(w, h, x int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for yy := 10; yy < 40; yy++ {
		for xx := x; xx < x+30; xx++ {
			img.Set(xx, yy, color.Black)
		}
	}
	return img
}

func TestRowMetrics(t *testing.T) {
	ivyImg := page(100, 100, 10)
	office, lo := page(100, 100, 12), page(100, 100, 50)
	cell := func(ref string, img image.Image, fault string) *Cell {
		m, _ := Compare(ivyImg, img, false)
		return &Cell{Ref: ref, Image: "x.png", Size: img.Bounds().Size(), Metric: &m, Fault: fault}
	}
	groups := []*refGroup{{id: "msoffice"}, {id: "soffice"}}
	row := &PageRow{
		Ivy:  &Cell{Image: "ivy.png", Size: image.Pt(100, 100)},
		Refs: []*Cell{cell("msoffice", office, ""), cell("soffice", lo, "")},
	}
	m := rowMetrics(row, []image.Image{office, lo}, groups)
	if m.primary != "msoffice" || m.best != row.Refs[0].Metric.Blurred || m.by["soffice"] != row.Refs[1].Metric.Blurred {
		t.Errorf("expected Office as the primary reference, got: %+v", m)
	}
	if m.refsDiff <= disagreeBlurred || m.sizeDiffers {
		t.Errorf("expected the references to disagree, and the same size, got: %+v", m)
	}
	// a fault of Office leaves LibreOffice as the primary reference, and
	// out of the disagreement
	row.Refs[0].Fault = "a fault"
	m = rowMetrics(row, []image.Image{office, lo}, groups)
	if m.primary != "soffice" || m.refsDiff != -1 || len(m.by) != 1 {
		t.Errorf("expected LibreOffice as the primary reference without Office, got: %+v", m)
	}
	// a page of another size
	wide := page(140, 100, 10)
	row = &PageRow{Ivy: &Cell{Image: "ivy.png", Size: image.Pt(100, 100)}, Refs: []*Cell{cell("soffice", wide, "")}}
	if m := rowMetrics(row, []image.Image{wide}, groups); !m.sizeDiffers {
		t.Errorf("expected a page of another size, got: %+v", m)
	}
	if sizeDiffers(image.Pt(816, 1056), image.Pt(817, 1055)) {
		t.Error("expected one pixel to be the same size")
	}
}

func TestFontFS(t *testing.T) {
	repo, user := t.TempDir(), t.TempDir()
	for _, f := range []string{filepath.Join(repo, "a.ttf"), filepath.Join(user, "sub", "b.otf")} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("font"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fsys := newFontFS(repo, []string{user})
	var files []string
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := fsys.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			p := e.Name()
			if dir != "." {
				p = dir + "/" + p
			}
			if e.IsDir() {
				walk(p)
				continue
			}
			files = append(files, p)
		}
	}
	walk(".")
	if want := []string{"a.ttf", "dir-1/sub/b.otf"}; !slices.Equal(files, want) {
		t.Errorf("expected %q, got: %q", want, files)
	}
	for _, name := range files {
		f, err := fsys.Open(name)
		if err != nil {
			t.Error(err)
			continue
		}
		f.Close()
	}
	if _, err := fsys.Open("../x"); err == nil {
		t.Error("expected an error for an invalid name")
	}
}

func TestBundle(t *testing.T) {
	repo, cache, out := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "bundle")
	if err := os.MkdirAll(filepath.Join(repo, "docx"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "docx", "a.docx"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []Entry{
		{Name: "repo/docx/a.docx", Kind: "docx", URL: "repo:docx/a.docx", Note: "password=x y"},
		{Name: "poi/b.xls", Kind: "xls", URL: "https://example.com/b.xls"},
		{Name: "style.css", Kind: KindAsset, URL: "https://example.com/style.css"},
	}
	n, err := Bundle(entries, BundleOptions{Repo: repo, Cache: cache, Out: out})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 file, got: %d", n)
	}
	if data, err := os.ReadFile(filepath.Join(out, "repo", "docx", "a.docx")); err != nil || string(data) != "a" {
		t.Errorf("expected the copy of the file, got: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(out, PasswordsFile)); err != nil || string(data) != "repo/docx/a.docx\tx\n" {
		t.Errorf("expected the password, got: %q, %v", data, err)
	}
}

func TestExternalPages(t *testing.T) {
	cache := t.TempDir()
	e := Entry{Name: "xps/a.xps", Kind: "xps"}
	x := externals[1]
	if x.id != "winxps" || !x.paged {
		t.Fatalf("expected the reference of Windows XPS, got: %+v", x)
	}
	base := x.file(cache, e)
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		t.Fatal(err)
	}
	for n, w := range []int{20, 30} {
		if err := writePNG(pagePNG(base, n+1), page(w, 40, 0)); err != nil {
			t.Fatal(err)
		}
	}
	env := &refEnv{cache: cache, dpi: 96}
	j := &refJob{refEnv: env, kind: "xps", hash: strings.Repeat("ab", 32), tmp: t.TempDir()}
	g, err := j.prepareExternal(t.Context(), x, e)
	if err != nil || g == nil {
		t.Fatalf("expected the reference, got: %+v %v", g, err)
	}
	if g.pages != 2 || x.display(e) != "windows/xps/xps/a.xps.p<N>.png" {
		t.Errorf("expected 2 pages, got: %d %s", g.pages, x.display(e))
	}
	png, err := g.job.renderPage(t.Context(), g.prep, g.renderers[0], false, 2)
	if err != nil {
		t.Fatal(err)
	}
	img, err := readImage(png)
	if err != nil || img.Bounds().Dx() != 30 {
		t.Errorf("expected the image of page 2, got: %v %v", img, err)
	}
	if _, err := g.job.renderPage(t.Context(), g.prep, g.renderers[0], false, 3); err == nil {
		t.Error("expected no page 3")
	}
	// another resolution
	env.dpi = 72
	if g, err := j.prepareExternal(t.Context(), x, e); err == nil || g == nil {
		t.Errorf("expected an error at 72 dpi, got: %v", err)
	}
	// no image of page 1: no reference
	if g, err := j.prepareExternal(t.Context(), x, Entry{Name: "xps/b.xps", Kind: "xps"}); g != nil || err != nil {
		t.Errorf("expected no reference, got: %+v %v", g, err)
	}
}
