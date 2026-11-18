package comic

import (
	"bytes"
	"slices"
	"testing"

	"github.com/xo/magic/container"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// TestDetect finds the type of a CBZ file as a program does, with magic and
// its package container (magic D65). Format lists its MIME type, and opens
// it.
func TestDetect(t *testing.T) {
	buf := fixture.Zip(pages()...)
	mime, f := fixture.DetectMIME(t, buf)
	if f != container.CBZ {
		t.Errorf("expected %v, got: %v", container.CBZ, f)
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
}
