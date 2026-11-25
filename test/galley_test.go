package test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/test/review"
)

// galleyKinds are the kinds of the corpus whose pages go through
// internal/layout.
var galleyKinds = []string{
	"epub", "mobi", "azw3", "fb2", "chm", "html", "txt", "md",
	"docx", "doc", "odt", "rtf", "xlsx", "xls", "ods", "csv",
}

// TestGalley lays out each document of the corpus of the review whose kind
// goes through internal/layout without the galley and with a galley of
// parts of one block, and compares the number of pages and the pixels of
// each page (D57). The galley starts at the first page, and the pages wait
// for the lines of each part, so that they take the lines of every part.
// The test reads the corpus from the cache of review that the variable
// IVY_REVIEW_CACHE names, such as test/review/cache after review fetch. It
// takes about 8 minutes, so it skips when the variable is not set. It skips
// the files that are not in the cache.
func TestGalley(t *testing.T) {
	repo := ".."
	cache := os.Getenv("IVY_REVIEW_CACHE")
	if cache == "" {
		t.Skip("IVY_REVIEW_CACHE is not set")
	}
	if _, err := os.Stat(filepath.Join(cache, "files")); err != nil {
		t.Skipf("no corpus in %s: run review fetch, or set IVY_REVIEW_CACHE", cache)
	}
	entries, err := review.LoadManifest(filepath.Join("review", "corpus.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := ivy.Config{Fonts: os.DirFS(filepath.Join(repo, "fonts"))}
	for _, e := range entries {
		if !slices.Contains(galleyKinds, e.Kind) {
			continue
		}
		format, ok := review.FormatFor(e.Kind)
		if !ok {
			t.Fatalf("%s: no format of the kind %s", e.Name, e.Kind)
		}
		path := e.Path(repo, cache)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Logf("%s: not in the cache", e.Name)
			continue
		}
		cfg := cfg
		cfg.Password = e.Password()
		t.Run(e.Name, func(t *testing.T) {
			want := galleyHashes(t, format, data, cfg, path, layout.GalleyOptions{Off: true})
			got := galleyHashes(t, format, data, cfg, path,
				layout.GalleyOptions{OneBlock: true, MaxParts: 16, Wait: true, Early: true})
			if len(got) != len(want) {
				t.Fatalf("expected %d pages, got: %d", len(want), len(got))
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("page %d of %d differs", i+1, len(want))
				}
			}
		})
	}
}

// galleyHashes opens data, the file at path, with format and the galley
// options o, and returns the hash of the pixels of each page at 72 dots
// per inch. A document that fails to open or to count its pages gives no
// hash.
func galleyHashes(t *testing.T, format ivy.Format, data []byte, cfg ivy.Config, path string, o layout.GalleyOptions) []string {
	t.Helper()
	defer layout.SetGalleyOptions(o)()
	ctx := context.Background()
	doc, err := review.OpenIvy(ctx, format, bytes.NewReader(data), cfg, path)
	if err != nil {
		return nil
	}
	defer doc.Close()
	pager, ok := doc.(ivy.Pager)
	if !ok {
		return nil
	}
	count, err := pager.PageCount(ctx)
	if err != nil {
		return nil
	}
	hashes := make([]string, count)
	for i := range count {
		hashes[i] = sheetPageHash(ctx, t, pager, i)
	}
	return hashes
}
