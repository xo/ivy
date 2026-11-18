package mobi

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xo/magic/container"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// TestDetect finds the type of the books of testdata as a program does,
// with magic and its package container (magic D65). A KF8 book and a book
// with both forms are AZW3. A book of MOBI alone keeps the type of magic.
// Format lists the MIME type of each, and opens each.
func TestDetect(t *testing.T) {
	tests := map[string]struct {
		want container.Format
		// gap is a fault of magic that gives another format, from "Work
		// for magic" in docs/BACKLOG.md. Delete it when magic is fixed.
		gap string
	}{
		"book.azw3":      {want: container.AZW3},
		"book-both.mobi": {want: container.AZW3, gap: "magic v0.9.0 reads the flags of the EXTH header 16 bytes too far"},
		"book-old.mobi":  {want: container.Unknown},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			mime, f := fixture.DetectMIME(t, buf)
			switch {
			case f != tt.want && tt.gap != "":
				t.Logf("expected %v, got: %v, because %s", tt.want, f, tt.gap)
			case f != tt.want:
				t.Errorf("expected %v, got: %v", tt.want, f)
			}
			if !slices.Contains(Format.Mimes, mime) {
				t.Errorf("expected Format to list %q", mime)
			}
			d, err := Format.Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Close(); err != nil {
				t.Error(err)
			}
		})
	}
}
