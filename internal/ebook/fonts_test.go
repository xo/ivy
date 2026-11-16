package ebook

import (
	"log/slog"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
)

// TestFontsConfig checks that the fonts of a configuration reach the
// configuration of the layout, with the limits and the logger of the
// format.
func TestFontsConfig(t *testing.T) {
	files := fstest.MapFS{"a.ttf": {Data: []byte("font")}}
	log := slog.New(slog.DiscardHandler)
	lim := ivy.Limits{MaxObjects: 7}
	cfg := FontsOf(ivy.Config{Fonts: files, SystemFonts: true}).Config(lim, log)
	if cfg.Fonts == nil || !cfg.SystemFonts || cfg.Limits != lim || cfg.Logger != log {
		t.Errorf("expected the fonts, the limits and the logger, got: %+v", cfg)
	}
	if cfg := (Fonts{}).Config(lim, log); cfg.Fonts != nil || cfg.SystemFonts {
		t.Errorf("expected no fonts, got: %+v", cfg)
	}
}
