package test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/sheet"
	"github.com/xo/ivy/test/review"
)

// TestSheetParts lays out each spreadsheet of the corpus of the review
// whole and in parts of 1 page and of 4 pages, and compares the number of
// pages and the pixels of each page (D55). The spreadsheets are the files
// of the kinds xlsx, xls, ods and csv, and a TSV file that the test makes of
// a CSV file. The test reads the corpus from the cache of review, which
// review fetch fills: test/review/cache, or the folder of the variable
// IVY_REVIEW_CACHE. It skips the files that are not in the cache.
func TestSheetParts(t *testing.T) {
	repo := ".."
	cache := os.Getenv("IVY_REVIEW_CACHE")
	if cache == "" {
		cache = filepath.Join("review", "cache")
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
		if !slices.Contains([]string{"xlsx", "xls", "ods", "csv"}, e.Kind) {
			continue
		}
		format, ok := review.FormatFor(e.Kind)
		if !ok {
			t.Fatalf("%s: no format of the kind %s", e.Name, e.Kind)
		}
		data, err := os.ReadFile(e.Path(repo, cache))
		if err != nil {
			t.Logf("%s: not in the cache", e.Name)
			continue
		}
		cfg := cfg
		cfg.Password = e.Password()
		t.Run(e.Name, func(t *testing.T) {
			sheetPartsMatch(t, format, data, cfg)
		})
		if e.Name == "datasets/csv/country-codes.csv" {
			t.Run("country-codes.tsv", func(t *testing.T) {
				sheetPartsMatch(t, format, tsvOf(t, data), cfg)
			})
		}
	}
}

// tsvOf returns the rows of a CSV file as a TSV file. A tab in a field
// becomes a space.
func tsvOf(t *testing.T, data []byte) []byte {
	t.Helper()
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	for _, record := range records {
		for i, field := range record {
			if i > 0 {
				buf.WriteByte('\t')
			}
			buf.Write(bytes.ReplaceAll(bytes.ReplaceAll([]byte(field), []byte("\t"), []byte(" ")), []byte("\n"), []byte(" ")))
		}
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// sheetPartsMatch opens data with format whole and in parts of 1 page and
// of 4 pages, and reports a page that differs.
func sheetPartsMatch(t *testing.T, format ivy.Format, data []byte, cfg ivy.Config) {
	t.Helper()
	want := sheetHashes(t, format, data, cfg, 0)
	if len(want) < 2 {
		return
	}
	for _, n := range []int{1, 4} {
		got := sheetHashes(t, format, data, cfg, n)
		if len(got) != len(want) {
			t.Errorf("parts of %d pages: expected %d pages, got: %d", n, len(want), len(got))
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("parts of %d pages: page %d of %d differs", n, i+1, len(want))
			}
		}
	}
}

// sheetHashes opens data with format, lays it out in parts of n pages, or
// whole when n is 0, and returns the hash of the pixels of each page at 72
// dots per inch.
func sheetHashes(t *testing.T, format ivy.Format, data []byte, cfg ivy.Config, n int) []string {
	t.Helper()
	ctx := sheet.WithPartPages(t.Context(), n)
	doc, err := format.Open(ctx, bytes.NewReader(data), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	pager, ok := doc.(ivy.Pager)
	if !ok {
		t.Fatal("expected pages")
	}
	count, err := pager.PageCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hashes := make([]string, count)
	for i := range count {
		hashes[i] = sheetPageHash(ctx, t, pager, i)
	}
	return hashes
}

// sheetPageHash returns the hash of the pixels of page i of pager.
func sheetPageHash(ctx context.Context, t *testing.T, pager ivy.Pager, i int) string {
	t.Helper()
	page, err := pager.Page(ctx, i)
	if err != nil {
		t.Fatalf("page %d: %v", i+1, err)
	}
	img, err := ivy.RenderImage(ctx, page, 72, nil)
	if _, partial := errors.AsType[*ivy.PartialError](err); err != nil && !partial {
		t.Fatalf("page %d: %v", i+1, err)
	}
	h := sha256.New()
	_, _ = h.Write([]byte(img.Bounds().String()))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			_, _ = h.Write([]byte{byte(r >> 8), byte(g >> 8), byte(bl >> 8), byte(a >> 8)})
		}
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}
