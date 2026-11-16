package ebook

import (
	"io/fs"
	"log/slog"

	"github.com/xo/ivy"
)

// Fonts are the fonts that a program gives a format in its ivy.Config: the
// font files of Config.Fonts and the choice of Config.SystemFonts (D23,
// D24). A format keeps them, so that the layout of its pages uses them.
type Fonts struct {
	files  fs.FS
	system bool
}

// FontsOf returns the fonts of cfg.
func FontsOf(cfg ivy.Config) Fonts {
	return Fonts{files: cfg.Fonts, system: cfg.SystemFonts}
}

// Config returns the configuration of the layout: the limits lim, the
// logger log and the fonts f.
func (f Fonts) Config(lim ivy.Limits, log *slog.Logger) ivy.Config {
	return ivy.Config{Limits: lim, Logger: log, Fonts: f.files, SystemFonts: f.system}
}
