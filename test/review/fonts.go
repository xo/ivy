package review

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// userFontDirs are the folders of the fonts of a person, relative to the
// home folder, that fontconfig and okage read on Linux.
var userFontDirs = []string{".fonts", filepath.Join(".local", "share", "fonts")}

// DefaultFontDirs returns the folders of the fonts of the person who runs
// the command, which exist: ~/.fonts and ~/.local/share/fonts of the home
// folder of the account. It takes the home folder from the account, not
// from HOME, because the review can run with another HOME, such as in a
// sandbox. The references and ivy then see the fonts that the person sees
// in a session of their own.
func DefaultFontDirs() []string {
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		return nil
	}
	var dirs []string
	for _, d := range userFontDirs {
		p := filepath.Join(u.HomeDir, d)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			dirs = append(dirs, p)
		}
	}
	return dirs
}

// fontFS is the fs.FS of the fonts that the review gives ivy as
// ivy.Config.Fonts: the font files of the repository at its root, and each
// font folder of -font-dirs in a folder of its own, "dir-1" and on. One
// value serves the whole run, so that the documents share the database of
// its fonts.
type fontFS struct {
	repo fs.FS
	dirs []fs.FS
}

// The interfaces that fontFS implements.
var _ fs.ReadDirFS = (*fontFS)(nil)

// newFontFS returns the fonts of the folder repo, the fonts of the
// repository, and of the folders dirs.
func newFontFS(repo string, dirs []string) *fontFS {
	f := &fontFS{repo: os.DirFS(repo)}
	for _, d := range dirs {
		f.dirs = append(f.dirs, os.DirFS(d))
	}
	return f
}

// split returns the file system and the name in it of name. ok is false
// for the root.
func (f *fontFS) split(name string) (fs.FS, string, bool) {
	first, rest, _ := strings.Cut(name, "/")
	if n, found := strings.CutPrefix(first, "dir-"); found {
		for i := range f.dirs {
			if n == fmt.Sprint(i+1) {
				if rest == "" {
					rest = "."
				}
				return f.dirs[i], rest, true
			}
		}
	}
	return f.repo, name, name != "."
}

// Open satisfies the fs.FS interface.
func (f *fontFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name == "." {
		return &rootDir{f: f}, nil
	}
	fsys, rest, _ := f.split(name)
	file, err := fsys.Open(rest)
	if err != nil {
		return nil, fmt.Errorf("opening the font file %s: %w", name, err)
	}
	return file, nil
}

// ReadDir satisfies the fs.ReadDirFS interface. The root holds the files of
// the repository, and a folder for each font folder.
func (f *fontFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	if name != "." {
		fsys, rest, _ := f.split(name)
		entries, err := fs.ReadDir(fsys, rest)
		if err != nil {
			return nil, fmt.Errorf("reading the font folder %s: %w", name, err)
		}
		return entries, nil
	}
	entries, err := fs.ReadDir(f.repo, ".")
	if err != nil {
		return nil, fmt.Errorf("reading the fonts of the repository: %w", err)
	}
	entries = slices.DeleteFunc(entries, func(e fs.DirEntry) bool { return strings.HasPrefix(e.Name(), "dir-") })
	for i := range f.dirs {
		entries = append(entries, fs.FileInfoToDirEntry(dirInfo(fmt.Sprintf("dir-%d", i+1))))
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

// rootDir is the root folder of a fontFS, as a file.
type rootDir struct {
	f    *fontFS
	read bool
}

func (d *rootDir) Stat() (fs.FileInfo, error) { return dirInfo("."), nil }

func (d *rootDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: ".", Err: errors.New("is a directory")}
}

func (d *rootDir) Close() error { return nil }

// ReadDir satisfies the fs.ReadDirFile interface.
func (d *rootDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.read {
		if n > 0 {
			return nil, io.EOF
		}
		return nil, nil
	}
	d.read = true
	return d.f.ReadDir(".")
}

// dirInfo is the fs.FileInfo of a folder of a fontFS that is not on the
// disk.
type dirInfo string

func (d dirInfo) Name() string       { return path.Base(string(d)) }
func (d dirInfo) Size() int64        { return 0 }
func (d dirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o555 }
func (d dirInfo) ModTime() time.Time { return time.Time{} }
func (d dirInfo) IsDir() bool        { return true }
func (d dirInfo) Sys() any           { return nil }
