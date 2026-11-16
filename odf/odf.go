// Package odf opens OpenDocument files, such as the documents of LibreOffice
// Writer, Calc, Impress and Draw.
//
// A document gives the thumbnail that every OpenDocument file holds, in
// Thumbnails/thumbnail.png (D11), and the pages of the texts, the
// spreadsheets, the presentations and the drawings (D24):
//
//   - A text, of Writer, goes through the layout of the ebooks, with its
//     styles, its lists, its tables, its pictures and its charts.
//   - A spreadsheet, of Calc, puts each sheet on pages, in order, as
//     printing does, with its pictures and its charts.
//   - A presentation, of Impress, or a drawing, of Draw, gives one page for
//     each page of the file, with its shapes, its pictures, its text and its
//     charts.
//
// internal/chart draws the charts of the embedded objects with zuzu. The
// other kinds of files, such as a chart or a formula on its own, give no
// pages, and Page returns an error that wraps ivy.ErrUnsupported. The title
// and the authors come when okage exports its XML parser (D7).
//
// A file that a password protects opens with the password of ivy.Config, or
// with the function PasswordFunc of ivy.Config (D12). The package reads
// Blowfish, the cipher of OpenOffice, and AES with PBKDF2, the cipher of ODF
// 1.2 and 1.3. It does not read Argon2id, the key derivation that
// LibreOffice writes by default for ODF 1.4.
package odf

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"iter"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/internal/zipdoc"
	"github.com/xo/ivy/odf/internal/odfcrypt"
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

// The names of the entries that the package reads.
const (
	manifestName  = "META-INF/manifest.xml"
	thumbnailName = "Thumbnails/thumbnail.png"
)

// Document is an open OpenDocument file.
type Document struct {
	archive *zipdoc.Archive
	mime    string
	lim     ivy.Limits
	cfg     ivy.Config
	// manifest is nil if the file holds no manifest that the package reads.
	manifest *odfcrypt.Manifest
	// keys is nil if the file is not encrypted.
	keys *odfcrypt.Keyring
	// pages holds the pager of the kind of the document.
	pages pages
}

// The capabilities of a document.
var _ ivy.Previewer = (*Document)(nil)

// Open opens an OpenDocument file. It returns an error that wraps
// ivy.ErrFormat if src is not an OpenDocument file, and one that wraps
// ivy.ErrPassword if the file is encrypted and the configuration does not
// give the right password.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (*Document, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening odf: %w", err)
	}
	lim := o.cfg.Limits.WithDefaults()
	archive, err := zipdoc.Open(src, lim)
	if err != nil {
		return nil, err
	}
	d := &Document{archive: archive, lim: lim, cfg: o.cfg}
	if err := d.readMime(); err != nil {
		return nil, err
	}
	d.readManifest(o.cfg)
	if d.manifest == nil || !d.manifest.Encrypted() {
		return d, nil
	}
	if err := d.unlock(ctx, o.cfg); err != nil {
		return nil, err
	}
	return d, nil
}

// readMime reads the entry mimetype, if the file holds one.
func (d *Document) readMime() error {
	f := d.archive.Find("mimetype")
	if f == nil {
		return nil
	}
	buf, err := d.archive.Read(f)
	if err != nil {
		return err
	}
	d.mime = strings.TrimSpace(string(buf))
	if !strings.HasPrefix(d.mime, mimePrefix) {
		return fmt.Errorf("mime type %q: %w", d.mime, ivy.ErrFormat)
	}
	return nil
}

// readManifest reads the manifest, if the file holds one. A file that is
// not encrypted opens without its manifest, so an error only goes to the
// log.
func (d *Document) readManifest(cfg ivy.Config) {
	d.manifest = nil
	f := d.archive.Find(manifestName)
	if f == nil {
		return
	}
	buf, err := d.archive.Read(f)
	if err == nil {
		d.manifest, err = odfcrypt.ParseManifest(buf, d.lim)
	}
	if err != nil {
		cfg.Log().Debug("odf: reading the manifest", "err", err)
	}
}

// unlock finds the password of an encrypted file. It decrypts the smallest
// encrypted entry with the password of the configuration, and while the
// password is wrong, it asks PasswordFunc for another one. If the file
// encrypts the whole package, unlock opens the package that it decrypts.
func (d *Document) unlock(ctx context.Context, cfg ivy.Config) error {
	f, enc, err := d.probe()
	if err != nil {
		return err
	}
	var plain []byte
	try := func(password string) error {
		d.keys = odfcrypt.NewKeyring(password)
		plain, err = d.decrypt(ctx, f, enc)
		return err
	}
	err = try(cfg.Password)
	for attempt := 1; err != nil && errors.Is(err, ivy.ErrPassword) && cfg.PasswordFunc != nil; attempt++ {
		password, perr := cfg.PasswordFunc(ctx, attempt)
		if perr != nil {
			return fmt.Errorf("opening odf: %w: %w", ivy.ErrPassword, perr)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("opening odf: %w", err)
		}
		err = try(password)
	}
	if err != nil {
		return fmt.Errorf("opening odf: %w", err)
	}
	if f.Name != odfcrypt.PackageName {
		return nil
	}
	// The whole package is encrypted. The package that it decrypts is an
	// OpenDocument file that is not encrypted.
	if d.archive, err = zipdoc.Open(bytes.NewReader(plain), d.lim); err != nil {
		return fmt.Errorf("opening the encrypted package: %w", err)
	}
	d.keys = nil
	if err := d.readMime(); err != nil {
		return err
	}
	d.readManifest(cfg)
	if d.manifest != nil && d.manifest.Encrypted() {
		return fmt.Errorf("encrypted package in an encrypted package: %w", ivy.ErrFormat)
	}
	return nil
}

// probe returns the entry that unlock decrypts to test a password: the
// encrypted package, or else the smallest encrypted entry. It returns an
// error that wraps ivy.ErrUnsupported if the package cannot decrypt the
// entry.
func (d *Document) probe() (*zip.File, *odfcrypt.Encryption, error) {
	var best *zip.File
	var enc *odfcrypt.Encryption
	for _, f := range d.archive.Files() {
		e := d.manifest.Encryption(f.Name)
		if e == nil {
			continue
		}
		if f.Name == odfcrypt.PackageName {
			best, enc = f, e
			break
		}
		if best == nil || f.CompressedSize64 < best.CompressedSize64 {
			best, enc = f, e
		}
	}
	if best == nil {
		return nil, nil, fmt.Errorf("odf: the manifest encrypts no entry of the file: %w", ivy.ErrFormat)
	}
	if err := enc.Supported(); err != nil {
		return nil, nil, fmt.Errorf("opening odf: %w", err)
	}
	return best, enc, nil
}

// decrypt reads and decrypts the entry f.
func (d *Document) decrypt(ctx context.Context, f *zip.File, enc *odfcrypt.Encryption) ([]byte, error) {
	buf, err := d.archive.Read(f)
	if err != nil {
		return nil, err
	}
	plain, err := d.keys.Decrypt(ctx, enc, buf, d.lim)
	if err != nil {
		return nil, fmt.Errorf("decrypting %s: %w", f.Name, err)
	}
	return plain, nil
}

// encryption returns the encryption of the entry f, or nil if it is not
// encrypted.
func (d *Document) encryption(f *zip.File) *odfcrypt.Encryption {
	if d.keys == nil || d.manifest == nil {
		return nil
	}
	return d.manifest.Encryption(f.Name)
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
// The thumbnail of an encrypted file is decrypted when the list reaches it.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	candidate := zipdoc.Candidate{Kind: ivy.Thumbnail, Name: thumbnailName}
	f := d.archive.Find(thumbnailName)
	if f == nil || d.encryption(f) == nil {
		return d.archive.Previews(ctx, candidate)
	}
	return func(yield func(ivy.Preview, error) bool) {
		buf, err := d.decrypt(ctx, f, d.encryption(f))
		if err != nil {
			yield(ivy.Preview{}, err)
			return
		}
		size, _ := raster.Config(buf)
		yield(ivy.Preview{
			Kind: ivy.Thumbnail,
			Size: size,
			Decode: func(ctx context.Context) (image.Image, error) {
				return raster.Decode(ctx, buf, d.lim)
			},
		}, nil)
	}
}
