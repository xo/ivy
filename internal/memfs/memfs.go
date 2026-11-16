// Package memfs holds files in memory as an fs.FS. A format that converts
// its own structure into HTML, such as FictionBook or Markdown, keeps the
// document that it makes and the images of the document in one, and gives
// it to the layout in a layout.Source.
package memfs

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"
)

// FS is a set of files in memory. The zero FS is empty. Add files with Add
// before the first read. After that, FS is safe for concurrent use.
type FS struct {
	files map[string][]byte
}

// The interfaces that FS satisfies.
var (
	_ fs.ReadFileFS = (*FS)(nil)
	_ fs.ReadDirFS  = (*FS)(nil)
	_ fs.StatFS     = (*FS)(nil)
)

// Add adds the file name with the content data. The name is a path that
// fs.ValidPath accepts, and Add does nothing for a name that it refuses. A
// second Add of the same name replaces the content. FS keeps data, so the
// caller does not change data after the call.
func (m *FS) Add(name string, data []byte) {
	if !fs.ValidPath(name) || name == "." {
		return
	}
	if m.files == nil {
		m.files = make(map[string][]byte)
	}
	m.files[name] = data
}

// Open satisfies the fs.FS interface.
func (m *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if data, ok := m.files[name]; ok {
		return &file{info: info{name: path.Base(name), size: int64(len(data))}, r: bytes.NewReader(data)}, nil
	}
	entries := m.entries(name)
	if entries == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &dir{info: info{name: path.Base(name), dir: true}, entries: entries}, nil
}

// ReadFile satisfies the fs.ReadFileFS interface. It returns a copy of the
// content.
func (m *FS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	}
	data, ok := m.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return slices.Clone(data), nil
}

// Stat satisfies the fs.StatFS interface.
func (m *FS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	if data, ok := m.files[name]; ok {
		return info{name: path.Base(name), size: int64(len(data))}, nil
	}
	if m.entries(name) == nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return info{name: path.Base(name), dir: true}, nil
}

// ReadDir satisfies the fs.ReadDirFS interface.
func (m *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	if _, ok := m.files[name]; ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	entries := m.entries(name)
	if entries == nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	return entries, nil
}

// entries returns the entries of the folder name, sorted by name. It returns
// nil if no file is in the folder. The root "." always exists.
func (m *FS) entries(name string) []fs.DirEntry {
	prefix := name + "/"
	if name == "." {
		prefix = ""
	}
	seen := make(map[string]bool)
	entries := []fs.DirEntry{}
	for file, data := range m.files {
		rest, ok := strings.CutPrefix(file, prefix)
		if !ok {
			continue
		}
		child, _, isDir := strings.Cut(rest, "/")
		if seen[child] {
			continue
		}
		seen[child] = true
		e := info{name: child, dir: isDir}
		if !isDir {
			e.size = int64(len(data))
		}
		entries = append(entries, e)
	}
	if len(entries) == 0 && name != "." {
		return nil
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries
}

// info is the information of a file or of a folder. It is both an
// fs.FileInfo and an fs.DirEntry.
type info struct {
	name string
	size int64
	dir  bool
}

// Name satisfies the fs.FileInfo interface.
func (i info) Name() string { return i.name }

// Size satisfies the fs.FileInfo interface.
func (i info) Size() int64 { return i.size }

// Mode satisfies the fs.FileInfo interface.
func (i info) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}

// ModTime satisfies the fs.FileInfo interface. It is the zero time.
func (i info) ModTime() time.Time { return time.Time{} }

// IsDir satisfies the fs.FileInfo interface.
func (i info) IsDir() bool { return i.dir }

// Sys satisfies the fs.FileInfo interface.
func (i info) Sys() any { return nil }

// Type satisfies the fs.DirEntry interface.
func (i info) Type() fs.FileMode { return i.Mode().Type() }

// Info satisfies the fs.DirEntry interface.
func (i info) Info() (fs.FileInfo, error) { return i, nil }

// file is an open file.
type file struct {
	info info
	r    *bytes.Reader
}

// Stat satisfies the fs.File interface.
func (f *file) Stat() (fs.FileInfo, error) { return f.info, nil }

// Read satisfies the fs.File interface.
func (f *file) Read(p []byte) (int, error) {
	// a reader returns io.EOF as it is, so that its caller sees the end
	return f.r.Read(p) //nolint:wrapcheck
}

// Seek satisfies the io.Seeker interface.
func (f *file) Seek(offset int64, whence int) (int64, error) {
	return f.r.Seek(offset, whence) //nolint:wrapcheck
}

// ReadAt satisfies the io.ReaderAt interface.
func (f *file) ReadAt(p []byte, off int64) (int, error) {
	return f.r.ReadAt(p, off) //nolint:wrapcheck
}

// Close satisfies the fs.File interface.
func (f *file) Close() error { return nil }

// dir is an open folder.
type dir struct {
	info    info
	entries []fs.DirEntry
	off     int
}

// Stat satisfies the fs.File interface.
func (d *dir) Stat() (fs.FileInfo, error) { return d.info, nil }

// Read satisfies the fs.File interface. A folder has no content.
func (d *dir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.name, Err: fs.ErrInvalid}
}

// Close satisfies the fs.File interface.
func (d *dir) Close() error { return nil }

// ReadDir satisfies the fs.ReadDirFile interface.
func (d *dir) ReadDir(n int) ([]fs.DirEntry, error) {
	rest := d.entries[d.off:]
	if n <= 0 {
		d.off = len(d.entries)
		return slices.Clone(rest), nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	n = min(n, len(rest))
	d.off += n
	return slices.Clone(rest[:n]), nil
}
