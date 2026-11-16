package zipdoc

import (
	"bytes"
	"io"
	"io/fs"
	"math"
	"path"
	"slices"
	"strings"
	"time"
)

// FS is the archive as an fs.FS, such as the files of an EPUB file for the
// layout. A file loads when it opens, within the limits of the archive. A
// name of the archive is cleaned: a backslash becomes a slash, and a leading
// slash is dropped. An entry whose name fs.ValidPath refuses after that is
// left out. If two entries have the same name, the first one wins.
//
// Change FS with Add and Transform before the first Open. After that, FS is
// safe for concurrent use.
type FS struct {
	nodes map[string]*node
	// lower maps the lower case of the name of each file to the file. Open
	// falls back on it, because a link in a document can differ in case from
	// the name in the archive.
	lower map[string]*node
}

// node is a file or a folder of an FS.
type node struct {
	name    string
	dir     bool
	size    int64
	modTime time.Time
	load    func() ([]byte, error)
	// children are the names of the entries of a folder.
	children map[string]bool
}

// The interfaces that FS satisfies.
var (
	_ fs.ReadFileFS = (*FS)(nil)
	_ fs.StatFS     = (*FS)(nil)
)

// NewFS returns an empty FS. A format that is not a zip archive, but that
// holds files that load one at a time, such as CHM, adds its files with
// AddFunc.
func NewFS() *FS {
	return &FS{
		nodes: map[string]*node{".": {name: ".", dir: true, children: map[string]bool{}}},
		lower: make(map[string]*node),
	}
}

// FS returns the entries of the archive as an fs.FS.
func (a *Archive) FS() *FS {
	f := NewFS()
	for _, zf := range a.zr.File {
		name, ok := CleanName(zf.Name)
		if !ok || strings.HasSuffix(zf.Name, "/") {
			continue
		}
		if _, exists := f.nodes[name]; exists {
			continue
		}
		size := int64(math.MaxInt64)
		if zf.UncompressedSize64 < math.MaxInt64 {
			size = int64(zf.UncompressedSize64)
		}
		f.add(name, size, zf.Modified, func() ([]byte, error) { return a.Read(zf) })
	}
	return f
}

// CleanName returns the name of a zip entry as a path that fs.ValidPath
// accepts, or false if it cannot be one.
func CleanName(name string) (string, bool) {
	name = strings.ReplaceAll(name, `\`, "/")
	name = path.Clean(strings.TrimLeft(name, "/"))
	if name == "." || !fs.ValidPath(name) {
		return "", false
	}
	return name, true
}

// Add adds the file name with the content data, or replaces the file of
// that name. It returns false if name is not a path that fs.ValidPath
// accepts, or if a folder or a file is in the way.
func (f *FS) Add(name string, data []byte) bool {
	return f.add(name, int64(len(data)), time.Time{}, func() ([]byte, error) { return data, nil })
}

// AddFunc adds the file name, or replaces the file of that name. The file
// loads with load each time that it opens. size is the size that Stat
// reports before the load. It returns false as Add does.
func (f *FS) AddFunc(name string, size int64, load func() ([]byte, error)) bool {
	return f.add(name, size, time.Time{}, load)
}

// Transform passes the content of the file name through fn each time the
// file opens. It returns false if FS has no file of that name.
func (f *FS) Transform(name string, fn func([]byte) ([]byte, error)) bool {
	n, ok := f.nodes[name]
	if !ok || n.dir {
		return false
	}
	load := n.load
	n.load = func() ([]byte, error) {
		data, err := load()
		if err != nil {
			return nil, err
		}
		return fn(data)
	}
	return true
}

// Has reports whether FS holds the file name.
func (f *FS) Has(name string) bool {
	n, ok := f.nodes[name]
	return ok && !n.dir
}

// add adds a file, and the folders above it.
func (f *FS) add(name string, size int64, modTime time.Time, load func() ([]byte, error)) bool {
	if name == "." || !fs.ValidPath(name) {
		return false
	}
	if n, ok := f.nodes[name]; ok {
		if n.dir {
			return false
		}
		n.size, n.modTime, n.load = size, modTime, load
		return true
	}
	// make sure that no file is in the place of a folder above name
	for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
		if n, ok := f.nodes[dir]; ok && !n.dir {
			return false
		}
	}
	child := name
	for dir := path.Dir(name); ; dir = path.Dir(dir) {
		parent, ok := f.nodes[dir]
		if !ok {
			parent = &node{name: path.Base(dir), dir: true, children: map[string]bool{}}
			f.nodes[dir] = parent
		}
		parent.children[path.Base(child)] = true
		if dir == "." || ok {
			break
		}
		child = dir
	}
	n := &node{name: path.Base(name), size: size, modTime: modTime, load: load}
	f.nodes[name] = n
	if lower := strings.ToLower(name); f.lower[lower] == nil {
		f.lower[lower] = n
	}
	return true
}

// lookup returns the node of name, and falls back on a file whose name
// differs only in case.
func (f *FS) lookup(name string) (*node, bool) {
	if n, ok := f.nodes[name]; ok {
		return n, true
	}
	n, ok := f.lower[strings.ToLower(name)]
	return n, ok
}

// Open satisfies the fs.FS interface. It reads the whole file.
func (f *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	n, ok := f.lookup(name)
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if n.dir {
		return &dirFile{info: n.info(), entries: f.entries(name, n)}, nil
	}
	data, err := n.load()
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	info := n.info()
	info.size = int64(len(data))
	return &file{info: info, r: bytes.NewReader(data)}, nil
}

// ReadFile satisfies the fs.ReadFileFS interface.
func (f *FS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	}
	n, ok := f.lookup(name)
	switch {
	case !ok:
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	case n.dir:
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	}
	data, err := n.load()
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: err}
	}
	// a file that Add holds is shared, so the caller gets a copy
	return slices.Clone(data), nil
}

// Stat satisfies the fs.StatFS interface. It does not load the file, so the
// size is the size that the archive declares.
func (f *FS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	n, ok := f.lookup(name)
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return n.info(), nil
}

// entries returns the entries of the folder n, sorted by name.
func (f *FS) entries(name string, n *node) []fs.DirEntry {
	names := make([]string, 0, len(n.children))
	for child := range n.children {
		names = append(names, child)
	}
	slices.Sort(names)
	entries := make([]fs.DirEntry, 0, len(names))
	for _, child := range names {
		full := child
		if name != "." {
			full = name + "/" + child
		}
		entries = append(entries, f.nodes[full].info())
	}
	return entries
}

// info returns the information of n.
func (n *node) info() info {
	return info{name: n.name, size: n.size, modTime: n.modTime, dir: n.dir}
}

// info is the information of a file or of a folder. It is both an
// fs.FileInfo and an fs.DirEntry.
type info struct {
	name    string
	size    int64
	modTime time.Time
	dir     bool
}

// Name satisfies the fs.FileInfo interface.
func (i info) Name() string { return i.name }

// Size satisfies the fs.FileInfo interface.
func (i info) Size() int64 {
	if i.dir {
		return 0
	}
	return i.size
}

// Mode satisfies the fs.FileInfo interface.
func (i info) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}

// ModTime satisfies the fs.FileInfo interface.
func (i info) ModTime() time.Time { return i.modTime }

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

// dirFile is an open folder.
type dirFile struct {
	info    info
	entries []fs.DirEntry
	off     int
}

// Stat satisfies the fs.File interface.
func (d *dirFile) Stat() (fs.FileInfo, error) { return d.info, nil }

// Read satisfies the fs.File interface. A folder has no content.
func (d *dirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.name, Err: fs.ErrInvalid}
}

// Close satisfies the fs.File interface.
func (d *dirFile) Close() error { return nil }

// ReadDir satisfies the fs.ReadDirFile interface.
func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
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
