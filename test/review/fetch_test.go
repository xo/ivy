package review

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestFetch(t *testing.T) {
	body := []byte("%PDF-1.7 a small file")
	sum := sha256.Sum256(body)
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/gone" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	cache := t.TempDir()
	entries := []Entry{
		{Name: "a/good.pdf", Kind: "pdf", URL: srv.URL + "/good", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))},
		{Name: "bad.pdf", Kind: "pdf", URL: srv.URL + "/bad", SHA256: strings.Repeat("0", 64), Size: -1},
		{Name: "gone.pdf", Kind: "pdf", URL: srv.URL + "/gone", Size: -1},
	}
	opts := FetchOptions{Cache: cache, Jobs: 2, Client: srv.Client()}
	err := Fetch(t.Context(), entries, opts)
	if err == nil || !strings.Contains(err.Error(), "2 of 3 files failed") {
		t.Fatalf("expected 2 failures, got: %v", err)
	}
	// each failure is tried twice
	if n := hits.Load(); n != 5 {
		t.Errorf("expected 5 requests, got: %d", n)
	}
	got, err := os.ReadFile(filepath.Join(cache, "files", "a", "good.pdf"))
	if err != nil || string(got) != string(body) {
		t.Errorf("expected the good file in the cache, got: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(cache, "files", "bad.pdf")); err == nil {
		t.Error("expected no file for a wrong hash")
	}
	// a file in the cache is not fetched again
	hits.Store(0)
	if err := Fetch(t.Context(), entries[:1], opts); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("expected no request, got: %d", n)
	}
}
