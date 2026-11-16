// Package fb2 opens FictionBook 2 files: an .fb2 file, which is XML, and a
// zip archive that holds one, such as .fb2.zip or .fbz.
//
// The package converts the body of the book into one XHTML document with a
// style sheet, for the layout of ivy (D10, D21). The images of the book are
// in its binary elements, as base64, and the package decodes them into the
// files that the document links to. The cover is the preview. The layout
// puts the document on the pages of an ebook, and Page lays it out only as
// far as the page that a program asks for.
//
// The package reads UTF-8, UTF-16, windows-1251 and windows-1252, which also
// reads ISO-8859-1. It reads a file of another encoding as UTF-8, and logs a
// warning. It does not apply the style sheet that a file can carry, because
// that style sheet names the elements of FictionBook, and not those of the
// XHTML document.
package fb2

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"iter"
	"log/slog"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/layout"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/internal/zipdoc"
	"github.com/xo/okage/roxmltree"
)

// Format describes the package for the registry of a program. A zip archive
// of a FictionBook file often has the extension .fb2.zip, which the registry
// cannot name, so a program finds it by its mime type or by its content.
var Format = ivy.Format{
	Name:  "fb2",
	Mimes: []string{"application/x-fictionbook+xml", "application/x-zip-compressed-fb2"},
	Exts:  []string{"fb2", "fbz"},
	Open: func(ctx context.Context, src ivy.Source, cfg ivy.Config) (ivy.Document, error) {
		return Open(ctx, src, WithConfig(cfg))
	},
}

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

// Document is an open FictionBook file.
type Document struct {
	lim  ivy.Limits
	log  *slog.Logger
	root roxmltree.Node
	info ivy.Info
	// cover is the id of the binary of the cover, or "".
	cover string
	// annotation is the annotation of the title-info, if ok is true.
	annotation struct {
		node roxmltree.Node
		ok   bool
	}
	binaries map[string]binary

	mu     sync.Mutex
	src    *layout.Source
	srcErr error
	// pages holds the book of the layout.
	pages ebook.Pages
}

// binary is one binary element: an image of the book, in base64.
type binary struct {
	contentType string
	data        string
}

// The capabilities of a document.
var (
	_ ivy.Pager     = (*Document)(nil)
	_ ivy.Previewer = (*Document)(nil)
	_ ivy.Describer = (*Document)(nil)
)

// Open opens a FictionBook file, or a zip archive that holds one. It parses
// the XML of the book. It returns an error that wraps ivy.ErrFormat if src
// is not a FictionBook file.
func Open(ctx context.Context, src ivy.Source, opts ...Option) (d *Document, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opening fictionbook: %w", err)
	}
	defer ebook.Recover("opening fictionbook", &err)
	lim := o.cfg.Limits.WithDefaults()
	buf, err := readBook(src, lim)
	if err != nil {
		return nil, err
	}
	text, known := decodeText(buf)
	log := o.cfg.Log()
	if !known {
		log.Warn("fictionbook of an unknown encoding, read as utf-8", "encoding", declaredEncoding(buf))
	}
	doc, err := ebook.ParseXML([]byte(text), lim)
	if err != nil {
		return nil, fmt.Errorf("parsing fictionbook: %w", err)
	}
	root := doc.RootElement()
	if root.TagName().Name() != "FictionBook" {
		return nil, fmt.Errorf("xml of root element %s, not FictionBook: %w", root.TagName().Name(), ivy.ErrFormat)
	}
	d = &Document{lim: lim, log: log, root: root, binaries: make(map[string]binary)}
	for n := range root.Children() {
		switch name(n) {
		case "description":
			d.describe(n)
		case "binary":
			id := ebook.Attr(n, "id")
			if id == "" {
				continue
			}
			if len(d.binaries) >= lim.MaxObjects {
				return nil, fmt.Errorf("fictionbook of more than %d binaries: %w", lim.MaxObjects, ivy.ErrLimit)
			}
			typ := ebook.Attr(n, "content-type")
			data, _ := n.Text()
			d.binaries[id] = binary{contentType: typ, data: data}
		}
	}
	return d, nil
}

// zipMagic starts a zip archive.
var zipMagic = []byte("PK\x03\x04")

// readBook reads the XML of the book from src. If src is a zip archive, it
// reads the first entry whose name ends in .fb2, or the only entry.
func readBook(src ivy.Source, lim ivy.Limits) ([]byte, error) {
	size := src.Size()
	switch {
	case size <= 0:
		return nil, fmt.Errorf("empty fictionbook: %w", ivy.ErrFormat)
	case size > lim.MaxMemory:
		return nil, fmt.Errorf("fictionbook of %d bytes: %w", size, ivy.ErrLimit)
	}
	head := make([]byte, len(zipMagic))
	if _, err := src.ReadAt(head, 0); err == nil && bytes.Equal(head, zipMagic) {
		return readZip(src, lim)
	}
	buf := make([]byte, size)
	if _, err := src.ReadAt(buf, 0); err != nil {
		return nil, fmt.Errorf("reading fictionbook: %w", err)
	}
	return buf, nil
}

// readZip reads the book from a zip archive.
func readZip(src ivy.Source, lim ivy.Limits) ([]byte, error) {
	archive, err := zipdoc.Open(src, lim)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range archive.Files() {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(path.Ext(f.Name), ".fb2") {
			return archive.Read(f)
		}
		files = append(files, f.Name)
	}
	if len(files) != 1 {
		return nil, fmt.Errorf("zip archive of %d files and no .fb2 file: %w", len(files), ivy.ErrFormat)
	}
	return archive.Read(archive.Find(files[0]))
}

// describe reads the description of the book.
func (d *Document) describe(desc roxmltree.Node) {
	for n := range desc.Children() {
		switch name(n) {
		case "title-info":
			for c := range n.Children() {
				switch name(c) {
				case "book-title":
					d.info.Title = text(c)
				case "author":
					if author := authorName(c); author != "" {
						d.info.Authors = append(d.info.Authors, author)
					}
				case "annotation":
					d.annotation.node, d.annotation.ok = c, true
				case "lang":
					d.info.Language = text(c)
				case "date":
					d.info.Created = date(c)
				case "coverpage":
					for img := range c.Children() {
						if id, ok := strings.CutPrefix(href(img), "#"); ok && name(img) == "image" {
							d.cover = id
							break
						}
					}
				}
			}
		case "document-info":
			for c := range n.Children() {
				if name(c) == "program-used" {
					d.info.Producer = text(c)
				}
			}
		}
	}
}

// authorName returns the name of an author: the first, middle and last
// names, or else the nickname.
func authorName(n roxmltree.Node) string {
	var parts []string
	var nickname string
	for c := range n.Children() {
		switch name(c) {
		case "first-name", "middle-name", "last-name":
			if s := text(c); s != "" {
				parts = append(parts, s)
			}
		case "nickname":
			nickname = text(c)
		}
	}
	if len(parts) == 0 {
		return nickname
	}
	return strings.Join(parts, " ")
}

// date reads a date element. The attribute value holds the date in the form
// of XML Schema, and the text holds it for a person, often as a year.
func date(n roxmltree.Node) time.Time {
	if t := ebook.ParseDate(ebook.Attr(n, "value")); !t.IsZero() {
		return t
	}
	return ebook.ParseDate(text(n))
}

// Close satisfies the ivy.Document interface.
func (d *Document) Close() error {
	return nil
}

// PageCount satisfies the ivy.Pager interface. It lays out the whole book
// on its first call.
func (d *Document) PageCount(ctx context.Context) (int, error) {
	return d.pages.PageCount(ctx, d.source, d.config())
}

// Page satisfies the ivy.Pager interface. It lays out the book only as far
// as page n.
func (d *Document) Page(ctx context.Context, n int) (ivy.Page, error) {
	return d.pages.Page(ctx, d.source, d.config(), n)
}

// config returns the configuration of the layout.
func (d *Document) config() ivy.Config {
	return ivy.Config{Limits: d.lim, Logger: d.log}
}

// Describe satisfies the ivy.Describer interface. It reads the title-info
// and the document-info of the description. The date of the title-info is
// the creation date. ivy.Info has no field for the annotation, so the
// annotation is the start of the XHTML document instead.
func (d *Document) Describe(ctx context.Context) (ivy.Info, error) {
	if err := ctx.Err(); err != nil {
		return ivy.Info{}, fmt.Errorf("describing fictionbook: %w", err)
	}
	info := d.info
	info.Authors = append([]string(nil), d.info.Authors...)
	return info, nil
}

// Previews satisfies the ivy.Previewer interface. The preview is the image
// of the coverpage.
func (d *Document) Previews(ctx context.Context) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		if d.cover == "" {
			return
		}
		if err := ctx.Err(); err != nil {
			yield(ivy.Preview{}, fmt.Errorf("listing previews: %w", err))
			return
		}
		buf, err := d.binary(d.cover)
		if err != nil {
			yield(ivy.Preview{}, err)
			return
		}
		size, err := raster.Config(buf)
		if err != nil {
			yield(ivy.Preview{}, fmt.Errorf("cover %s: %w", d.cover, err))
			return
		}
		if err := raster.CheckPixels(size, d.lim); err != nil {
			yield(ivy.Preview{}, err)
			return
		}
		yield(ivy.Preview{
			Kind: ivy.Cover,
			Size: size,
			Decode: func(ctx context.Context) (img image.Image, err error) {
				defer ebook.Recover("decoding cover", &err)
				return raster.Decode(ctx, buf, d.lim)
			},
		}, nil)
	}
}

// binary decodes the binary id from base64.
func (d *Document) binary(id string) ([]byte, error) {
	b, ok := d.binaries[id]
	if !ok {
		return nil, fmt.Errorf("binary %s is missing: %w", id, ivy.ErrFormat)
	}
	clean := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, b.data)
	clean = strings.TrimRight(clean, "=")
	if int64(base64.RawStdEncoding.DecodedLen(len(clean))) > d.lim.MaxMemory {
		return nil, fmt.Errorf("binary %s of %d bytes of base64: %w", id, len(clean), ivy.ErrLimit)
	}
	buf, err := base64.RawStdEncoding.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("decoding binary %s: %w: %w", id, ivy.ErrFormat, err)
	}
	return buf, nil
}

// source converts the book into XHTML for the layout. It converts the book
// once, and returns the same result on each later call.
func (d *Document) source(ctx context.Context) (src layout.Source, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.srcErr != nil:
		return layout.Source{}, d.srcErr
	case d.src != nil:
		return *d.src, nil
	}
	defer func() {
		switch {
		case err == nil:
			d.src = &src
		case ctx.Err() == nil:
			// an error of the context is not kept, so that a later call can
			// convert the book
			d.srcErr = err
		}
	}()
	// Recover runs before the function above, which then keeps its error
	defer ebook.Recover("converting fictionbook", &err)
	return d.convert(ctx)
}

// name returns the local name of the element n, or "" if n is not an
// element. A FictionBook file has its elements in the namespace of
// FictionBook, and some files have them in no namespace, so the namespace
// is not compared.
func name(n roxmltree.Node) string {
	if !n.IsElement() {
		return ""
	}
	return ebook.Local(n)
}

// text returns the text of the descendants of n, with each run of white
// space as one space.
func text(n roxmltree.Node) string {
	return ebook.NodeText(n)
}

// href returns the value of the attribute href of n, in any namespace. A
// FictionBook file names it xlink:href, l:href or href.
func href(n roxmltree.Node) string {
	return strings.TrimSpace(ebook.Attr(n, "href"))
}
