package memfs

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestFS(t *testing.T) {
	var m FS
	m.Add("index.xhtml", []byte("<html/>"))
	m.Add("images/a.png", []byte("png"))
	m.Add("images/b/c.jpg", []byte("jpg"))
	m.Add("../escape", []byte("no"))
	m.Add(".", []byte("no"))
	if err := fstest.TestFS(&m, "index.xhtml", "images/a.png", "images/b/c.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Open("../escape"); !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("expected fs.ErrInvalid, got: %v", err)
	}
	if _, err := m.Open("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got: %v", err)
	}
}

func TestEmpty(t *testing.T) {
	var m FS
	if err := fstest.TestFS(&m); err != nil {
		t.Fatal(err)
	}
}

func TestOver(t *testing.T) {
	var m FS
	m.Add("index.html", []byte("top"))
	under := fstest.MapFS{
		"index.html":    {Data: []byte("under")},
		"css/style.css": {Data: []byte("p {}")},
	}
	fsys := m.Over(under)
	for name, exp := range map[string]string{"index.html": "top", "css/style.css": "p {}"} {
		buf, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatal(err)
		}
		if string(buf) != exp {
			t.Errorf("%s: expected %q, got: %q", name, exp, buf)
		}
	}
	if _, err := fs.ReadFile(fsys, "none.png"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got: %v", err)
	}
}
