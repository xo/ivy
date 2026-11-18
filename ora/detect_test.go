package ora

import (
	"bytes"
	"slices"
	"testing"

	"github.com/xo/magic/container"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// TestDetect finds the type of OpenRaster and Krita files as a program
// does, with magic and its package container (magic D65). Format lists the
// MIME type of each, and opens each.
func TestDetect(t *testing.T) {
	tests := map[string]container.Format{
		mimeORA:   container.OpenRaster,
		mimeKrita: container.Krita,
	}
	for mimetype, want := range tests {
		t.Run(mimetype, func(t *testing.T) {
			buf := file(mimetype)
			mime, f := fixture.DetectMIME(t, buf)
			if f != want || mime != mimetype {
				t.Errorf("expected %v and %q, got: %v and %q", want, mimetype, f, mime)
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
