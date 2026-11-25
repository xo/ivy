package review

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// TestWalkMarkdownIt walks the page against a server that imports one leaf
// module and two themes from each module, and checks the rules of the walk.
func TestWalkMarkdownIt(t *testing.T) {
	const module = `import"/npm/leaf@1.0.0/+esm";` +
		`const a=()=>import("/npm/@shikijs/themes@1.0.0/other/+esm"),` +
		`b=()=>import("/npm/@shikijs/themes@1.0.0/github-light-default/+esm");`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		switch {
		case strings.HasSuffix(p, "/package.json"):
			if strings.Contains(p, "leaf@") {
				_, _ = w.Write([]byte(`{"license":{"type":"ISC"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"license":"MIT"}`))
		case strings.HasSuffix(p, ".css"):
			_, _ = w.Write([]byte(`@font-face{src:url(fonts/F.woff2) format("woff2"),url(fonts/F.woff) format("woff"),url(data:x)}`))
		case strings.Contains(p, "other"), strings.Contains(p, "leaf@"), strings.Contains(p, "github-light-default"), strings.HasSuffix(p, ".woff2"):
			_, _ = w.Write([]byte(`export{}`))
		default:
			_, _ = w.Write([]byte(module))
		}
	}))
	defer srv.Close()
	entries, err := WalkMarkdownIt(t.Context(), WalkOptions{Base: srv.URL + "/", Jobs: 4})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
		if !strings.HasPrefix(e.URL, jsDelivr) || e.Kind != KindAsset || e.License == "" || e.Size <= 0 {
			t.Errorf("%s: expected an asset on jsDelivr with a license and a size, got: %+v", e.Name, e)
		}
	}
	if !slices.IsSorted(names) {
		t.Errorf("expected the entries in the order of their names, got: %v", names)
	}
	for _, want := range []string{
		"npm/leaf@1.0.0/+esm",
		"npm/@shikijs/themes@1.0.0/github-light-default/+esm",
		"npm/katex@0.16.22/dist/fonts/F.woff2",
	} {
		if !slices.Contains(names, want) {
			t.Errorf("expected the walk to find %s, got: %v", want, names)
		}
	}
	for _, n := range names {
		if strings.Contains(n, "other") || strings.HasSuffix(n, ".woff") {
			t.Errorf("expected the walk to leave out %s", n)
		}
	}
	for _, e := range entries {
		switch {
		case e.Name == "npm/leaf@1.0.0/+esm":
			sum := sha256.Sum256([]byte("export{}"))
			if e.SHA256 != hex.EncodeToString(sum[:]) || e.License != "ISC" || e.Source != "leaf@1.0.0" || e.Size != 8 {
				t.Errorf("expected the hash, the license ISC and the source of the leaf, got: %+v", e)
			}
		case strings.HasPrefix(e.Name, "npm/markdown-it@"):
			if e.License != "MIT" || e.Source != "markdown-it@15.0.3" {
				t.Errorf("expected the license MIT and the source of markdown-it, got: %+v", e)
			}
		}
	}
	// the manifest that the walk writes is a manifest that the review reads
	var b bytes.Buffer
	if err := WriteMarkdownItManifest(&b, entries); err != nil {
		t.Fatal(err)
	}
	back, err := ReadManifest(&b)
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != len(entries) {
		t.Errorf("expected %d entries back, got: %d", len(entries), len(back))
	}
}
