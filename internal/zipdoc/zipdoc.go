// Package zipdoc reads the formats that are zip archives, such as
// OpenDocument, Office Open XML, iWork and OpenRaster, within the limits of a
// configuration. It finds the images that they carry, and lists them as
// previews (D11).
package zipdoc

import (
	"archive/zip"
	"context"
	"fmt"
	"image"
	"io"
	"iter"
	"path"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
)

// headerLimit is the number of bytes of an entry that Config reads to find
// the size of an image. A JPEG can hold its EXIF data before its size.
const headerLimit = 256 << 10

// Archive is an open zip archive.
type Archive struct {
	zr    *zip.Reader
	lim   ivy.Limits
	names map[string]*zip.File
	// keyed is true once the format called Unlock, and password is the
	// password of the encrypted entries.
	keyed    bool
	password string
}

// Open opens the zip archive in src. It returns an error that wraps
// ivy.ErrFormat if src is not a zip archive, and ivy.ErrLimit if it holds
// more entries than lim.MaxObjects.
func Open(src ivy.Source, lim ivy.Limits) (*Archive, error) {
	lim = lim.WithDefaults()
	zr, err := zip.NewReader(src, src.Size())
	if err != nil {
		return nil, fmt.Errorf("opening zip archive: %w: %w", ivy.ErrFormat, err)
	}
	if len(zr.File) > lim.MaxObjects {
		return nil, fmt.Errorf("zip archive of %d entries: %w", len(zr.File), ivy.ErrLimit)
	}
	names := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		key := strings.ToLower(f.Name)
		if _, ok := names[key]; !ok {
			names[key] = f
		}
	}
	return &Archive{zr: zr, lim: lim, names: names}, nil
}

// Files returns the entries of the archive, in the order of the archive.
func (a *Archive) Files() []*zip.File {
	return a.zr.File
}

// Find returns the first of names that the archive holds. The names match
// without regard to case. It returns nil if the archive holds none of them.
func (a *Archive) Find(names ...string) *zip.File {
	for _, name := range names {
		if f, ok := a.names[strings.ToLower(name)]; ok {
			return f
		}
	}
	return nil
}

// Read reads the entry f, within the limit of memory.
func (a *Archive) Read(f *zip.File) ([]byte, error) {
	rc, err := a.open(f)
	if err != nil {
		return nil, err
	}
	// a read-only entry has nothing to flush, so its close cannot lose data
	defer rc.Close()
	return raster.ReadAll(rc, a.lim.MaxMemory)
}

// Config returns the size of the image in the entry f, from the first bytes
// of the entry.
func (a *Archive) Config(f *zip.File) (image.Point, error) {
	rc, err := a.open(f)
	if err != nil {
		return image.Point{}, err
	}
	defer rc.Close()
	buf, err := io.ReadAll(io.LimitReader(rc, headerLimit))
	if err != nil {
		return image.Point{}, fmt.Errorf("reading %s: %w: %w", f.Name, ivy.ErrFormat, err)
	}
	return raster.Config(buf)
}

// Decode decodes the image in the entry f.
func (a *Archive) Decode(ctx context.Context, f *zip.File) (image.Image, error) {
	buf, err := a.Read(f)
	if err != nil {
		return nil, err
	}
	return raster.Decode(ctx, buf, a.lim)
}

// open opens the entry f. It decrypts an entry that the archive encrypts
// with the password that Unlock found, and refuses it if the format did not
// call Unlock.
func (a *Archive) open(f *zip.File) (io.ReadCloser, error) {
	if f.UncompressedSize64 > uint64(a.lim.MaxMemory) {
		return nil, fmt.Errorf("zip entry %s of %d bytes: %w", f.Name, f.UncompressedSize64, ivy.ErrLimit)
	}
	if Encrypted(f) {
		if !a.keyed {
			return nil, fmt.Errorf("encrypted zip entry %s: %w", f.Name, ivy.ErrUnsupported)
		}
		return a.decrypt(f, a.password)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w: %w", f.Name, ivy.ErrFormat, err)
	}
	return rc, nil
}

// Preview returns the preview of the image in the entry f. Its size is zero
// if the first bytes of the entry do not give it.
func (a *Archive) Preview(kind ivy.PreviewKind, f *zip.File) ivy.Preview {
	size, _ := a.Config(f)
	return ivy.Preview{
		Kind: kind,
		Size: size,
		Decode: func(ctx context.Context) (image.Image, error) {
			return a.Decode(ctx, f)
		},
	}
}

// Candidate is an entry that can hold a preview.
type Candidate struct {
	Kind ivy.PreviewKind
	Name string
}

// Previews lists the previews of the candidates that the archive holds, in
// the order of the candidates. An entry whose type the package image cannot
// decode, such as a thumbnail in EMF, is left out.
func (a *Archive) Previews(ctx context.Context, candidates ...Candidate) iter.Seq2[ivy.Preview, error] {
	return func(yield func(ivy.Preview, error) bool) {
		for _, c := range candidates {
			if err := ctx.Err(); err != nil {
				yield(ivy.Preview{}, err)
				return
			}
			f := a.Find(c.Name)
			if f == nil || !Decodable(f.Name) {
				continue
			}
			if !yield(a.Preview(c.Kind, f), nil) {
				return
			}
		}
	}
}

// Decodable reports whether the extension of name is a type that the package
// raster decodes.
func Decodable(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	}
	return false
}
