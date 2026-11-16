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
