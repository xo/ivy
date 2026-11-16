// Package odf opens OpenDocument files, such as the documents of LibreOffice
// Writer, Calc, Impress and Draw.
//
// On 2026-11-16 a document gives the thumbnail that every OpenDocument file
// holds, in Thumbnails/thumbnail.png (D11). The pages come in phase 5, and the
// title and the authors come when okage exports its XML parser (D7).
package odf

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/zipdoc"
)

// Format describes the package for the registry of a program.
var Format = ivy.Format{
	Name: "odf",
	Mimes: []string{
		"application/vnd.oasis.opendocument.text",
		"application/vnd.oasis.opendocument.text-template",
		"application/vnd.oasis.opendocument.text-master",
		"application/vnd.oasis.opendocument.spreadsheet",
		"application/vnd.oasis.opendocument.spreadsheet-template",
		"application/vnd.oasis.opendocument.presentation",
		"application/vnd.oasis.opendocument.presentation-template",
		"application/vnd.oasis.opendocument.graphics",
		"application/vnd.oasis.opendocument.graphics-template",
		"application/vnd.oasis.opendocument.chart",
		"application/vnd.oasis.opendocument.formula",
	},
	Exts: []string{"odt", "ott", "odm", "ods", "ots", "odp", "otp", "odg", "otg", "odc", "odf"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

// mimePrefix starts the mime type of every OpenDocument file.
const mimePrefix = "application/vnd.oasis.opendocument."

// Option is an option of Open.
type Option func(*options)

type options struct {
	cfg ivy.Config
}

// WithConfig sets the configuration that every format shares.
func WithConfig(cfg ivy.Config) Option {
	return func(o *options) {
		o.cfg = cfg
	}
}

// Document is an open OpenDocument file.
type Document struct {
	archive *zipdoc.Archive
	mime    string
}

// The capabilities of a document.
var _ ivy.Previewer = (*Document)(nil)

// Open opens an OpenDocument file. It returns an error that wraps
// ivy.ErrFormat if src is not an OpenDocument file.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening odf: %w", err)
	}
	archive, err := zipdoc.Open(src, o.cfg.Limits)
	if err != nil {
		return nil, err
	}
	d := &Document{archive: archive}
	if f := archive.Find("mimetype"); f != nil {
		buf, err := archive.Read(f)
		if err != nil {
			return nil, err
		}
		d.mime = strings.TrimSpace(string(buf))
		if !strings.HasPrefix(d.mime, mimePrefix) {
			return nil, fmt.Errorf("mime type %q: %w", d.mime, ivy.ErrFormat)
		}
	}
	return d, nil
}

// Mime returns the mime type that the file declares, such as
// "application/vnd.oasis.opendocument.text". It is empty if the file does not
// declare one.
func (d *Document) Mime() string {
	return d.mime
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// Previews satisfies the ivy.Previewer interface. It lists the thumbnail.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return d.archive.Previews(ctx, zipdoc.Candidate{Kind: ivy.Thumbnail, Name: "Thumbnails/thumbnail.png"})
}
