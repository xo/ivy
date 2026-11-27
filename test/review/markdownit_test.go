package review

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/xo/ivy"
)

// TestMarkdownItManifest makes sure that markdownit.tsv names each file of
// jsDelivr at its path, with its hash, its size and its license, and that it
// holds each file that the page loads.
func TestMarkdownItManifest(t *testing.T) {
	entries, err := MarkdownItEntries()
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
		if e.URL != "https://cdn.jsdelivr.net/"+e.Name {
			t.Errorf("%s: expected the URL of jsDelivr at the name, got: %s", e.Name, e.URL)
		}
		if e.SHA256 == "" || e.Size < 0 || e.License == "" || e.Kind != KindAsset {
			t.Errorf("%s: expected an asset with a sha256, a size and a license, got: %+v", e.Name, e)
		}
	}
	for _, m := range regexp.MustCompile(`"/(npm/[^"]+)"`).FindAllStringSubmatch(markdownItPage, -1) {
		if !names[m[1]] {
			t.Errorf("the page loads %s, which markdownit.tsv does not hold", m[1])
		}
	}
}

// TestMarkdownItHTML makes sure that the text of the file cannot end the
// element script of the page, and that the page has the paper of ivy.
func TestMarkdownItHTML(t *testing.T) {
	page, err := markdownItHTML([]byte("a </script><script>alert(1)</script>"), ivy.A4)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page), "</script><script>alert(1)") {
		t.Errorf("expected the text to stay in the element script, got:\n%s", page)
	}
	if !strings.Contains(string(page), "size: 595.28pt 841.89pt;") || !strings.Contains(string(page), "margin: 0.75in;") {
		t.Errorf("expected the paper A4 with margins of 0.75 inch")
	}
	if strings.Contains(string(page), "{{") {
		t.Errorf("expected no placeholder in the page")
	}
}

// TestMarkdownItRoute makes sure that markdown-it is the route of Markdown,
// and that calibre is an extra route of it (D70).
func TestMarkdownItRoute(t *testing.T) {
	for _, kind := range []string{"md", "markdown"} {
		if r := routeFor(kind); r != markdownItRoute {
			t.Errorf("%s: expected the route markdownit, got: %s", kind, r.id)
		}
		if rs := extraRoutes[kind]; len(rs) != 1 || rs[0] != calibreRoute {
			t.Errorf("%s: expected the extra route calibre, got: %v", kind, rs)
		}
		if !paperless(kind) {
			t.Errorf("%s: expected a kind without a paper", kind)
		}
	}
}

// TestPageServer makes sure that the server serves the files of a page,
// and no file out of the folder of the page.
func TestPageServer(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "doc")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"doc/a.png": "png", "secret.txt": "secret"} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &pageServer{files: map[string]string{}, pages: map[string]servedPage{}}
	ts := httptest.NewServer(s)
	defer ts.Close()
	s.url = ts.URL
	u, remove, err := s.add([]byte("<p>page</p>"), sub)
	if err != nil {
		t.Fatal(err)
	}
	get := func(u string) (int, string) {
		t.Helper()
		resp, err := http.Get(u) //nolint:noctx // a test of a local server
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(b)
	}
	if code, body := get(u); code != http.StatusOK || body != "<p>page</p>" {
		t.Errorf("expected the page, got: %d %q", code, body)
	}
	if code, body := get(u + "a.png"); code != http.StatusOK || body != "png" {
		t.Errorf("expected the image, got: %d %q", code, body)
	}
	if code, _ := get(u + "%2e%2e/secret.txt"); code != http.StatusNotFound {
		t.Errorf("expected no file out of the folder, got: %d", code)
	}
	remove()
	if code, _ := get(u); code != http.StatusNotFound {
		t.Errorf("expected no page after remove, got: %d", code)
	}
}

// TestMarkdownIt renders a Markdown file with markdown-it in Chrome, on A4,
// and reads the text of the PDF file. The files of the page are in the
// cache of the review, which test/cmd/review fetches. IVY_REVIEW_CACHE
// names the folder of the cache, by default test/review/cache. Without the
// files of the page, the test is skipped.
func TestMarkdownIt(t *testing.T) {
	cache := os.Getenv("IVY_REVIEW_CACHE")
	if cache == "" {
		cache = "cache"
	}
	cache, err := filepath.Abs(cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, markdownItFolder)); err != nil {
		t.Skipf("no files of markdown-it in %s: fetch them with test/cmd/review, or set IVY_REVIEW_CACHE", cache)
	}
	for _, name := range []string{chromeProgram, "pdfinfo", "pdftotext", "fc-list"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("oracle %s is not installed: %v", name, err)
		}
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	img.Set(1, 1, color.Black)
	f, err := os.Create(filepath.Join(dir, "dot.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "doc.md")
	src := "---\ntitle: hidden front matter\n---\n\n# Title\n\n> [!NOTE]\n> An alert.\n\n" +
		"- [x] done task\n- [ ] open task\n\nA footnote[^1] and :smile:.\n\n[^1]: The note text.\n\n" +
		"```go\nfunc main() {}\n```\n\nInline $x^2$ math.\n\n```math\ny = 2\n```\n\n" +
		"![local dot](dot.png) ![remote alt](https://example.com/a.png)\n\n" +
		"<div style=\"display: none\">A raw div.</div>\n\n<xmp>Raw xmp.</xmp>\n"
	if err := os.WriteFile(doc, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, err := toolEnv(t.Context(), RenderOptions{Repo: repo, Cache: cache, DPI: 96}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer env.closeChrome()
	defer env.pageSrv.close()
	j := &refJob{refEnv: env, kind: "md", tmp: t.TempDir(), paper: ivy.A4}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	out := t.TempDir()
	name, err := markdownItPDF(ctx, j, doc, out)
	if err != nil {
		t.Fatal(err)
	}
	pdf := filepath.Join(out, name)
	// Chrome rounds the size of the page to whole pixels of 1/96 inch
	w, h, err := pageBox(ctx, j, pdf, 1)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(w-595.28) > 1 || math.Abs(h-841.89) > 1 {
		t.Errorf("expected a page of A4, got: %g by %g points", w, h)
	}
	text, err := runTool(ctx, j.env, "", "pdftotext", "-layout", pdf, "-")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Title", "Note", "An alert.", "done task", "The note text.", "func main() {}", "remote alt", "😄", "A raw div."} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in the text of the PDF file, got:\n%s", want, text)
		}
	}
	if strings.Contains(text, "hidden front matter") || strings.Contains(text, "$x") || strings.Contains(text, "[!NOTE]") {
		t.Errorf("expected no front matter, no source of math and no source of an alert, got:\n%s", text)
	}
	// the sanitizer of GitHub drops the style of the div, and the xmp with
	// its content
	if strings.Contains(text, "Raw xmp.") {
		t.Errorf("expected no content of the element xmp, got:\n%s", text)
	}
}
