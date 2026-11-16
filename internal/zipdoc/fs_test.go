package zipdoc

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

func TestFS(t *testing.T) {
	buf := fixture.Zip(
		fixture.Entry{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true},
		fixture.Entry{Name: "OEBPS/", Data: nil},
		fixture.Entry{Name: "OEBPS/Text/ch1.xhtml", Data: []byte("<html/>")},
		fixture.Entry{Name: "/OEBPS/Images/a.png", Data: []byte("png")},
		fixture.Entry{Name: `OEBPS\Styles\b.css`, Data: []byte("p{}")},
		fixture.Entry{Name: "OEBPS/Text/ch1.xhtml", Data: []byte("second copy")},
		fixture.Entry{Name: "../evil", Data: []byte("no")},
		fixture.Entry{Name: "a/./../../evil", Data: []byte("no")},
	)
	archive, err := Open(bytes.NewReader(buf), ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	fsys := archive.FS()
	if err := fstest.TestFS(fsys, "mimetype", "OEBPS/Text/ch1.xhtml", "OEBPS/Images/a.png", "OEBPS/Styles/b.css"); err != nil {
		t.Fatal(err)
	}
	if data, err := fs.ReadFile(fsys, "OEBPS/Text/ch1.xhtml"); err != nil || string(data) != "<html/>" {
		t.Errorf("expected the first entry of a name, got: %q %v", data, err)
	}
	if data, err := fs.ReadFile(fsys, "oebps/images/A.PNG"); err != nil || string(data) != "png" {
		t.Errorf("expected a fall back on the case, got: %q %v", data, err)
	}
	for _, name := range []string{"evil", "../evil", "missing"} {
		if _, err := fsys.Open(name); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if !fsys.Has("mimetype") || fsys.Has("OEBPS") || fsys.Has("nothing") {
		t.Error("expected Has for files only")
	}

	// Transform and Add
	if !fsys.Transform("mimetype", func(b []byte) ([]byte, error) { return bytes.ToUpper(b), nil }) {
		t.Fatal("expected a transform of a file")
	}
	if fsys.Transform("OEBPS", nil) || fsys.Transform("missing", nil) {
		t.Error("expected no transform of a folder or a missing file")
	}
	if data, _ := fs.ReadFile(fsys, "mimetype"); string(data) != "APPLICATION/EPUB+ZIP" {
		t.Errorf("expected the transform, got: %q", data)
	}
	fsys.Transform("OEBPS/Styles/b.css", func([]byte) ([]byte, error) { return nil, ivy.ErrDRM })
	if _, err := fsys.Open("OEBPS/Styles/b.css"); !errors.Is(err, ivy.ErrDRM) {
		t.Errorf("expected the error of the transform, got: %v", err)
	}
	if !fsys.Add("OEBPS/Text/new.html", []byte("<p>new")) || !fsys.Add("top.html", []byte("top")) {
		t.Fatal("expected Add to add files")
	}
	if fsys.Add("OEBPS/Text", nil) || fsys.Add("mimetype/x", nil) || fsys.Add("../x", nil) || fsys.Add(".", nil) {
		t.Error("expected Add to refuse a folder, a file in the way and a bad name")
	}
	fsys.Transform("OEBPS/Styles/b.css", func(b []byte) ([]byte, error) { return b, nil })
	if err := fstest.TestFS(fsys, "OEBPS/Text/new.html", "top.html", "OEBPS/Text/ch1.xhtml"); err == nil {
		// b.css still fails to open, so the test of the whole tree fails
		t.Error("expected fstest to see the file that fails to open")
	}
}

func TestFSLimit(t *testing.T) {
	buf := fixture.Zip(fixture.Entry{Name: "big.txt", Data: bytes.Repeat([]byte("a"), 1000)})
	archive, err := Open(bytes.NewReader(buf), ivy.Limits{MaxMemory: 100})
	if err != nil {
		t.Fatal(err)
	}
	fsys := archive.FS()
	if _, err := fsys.Open("big.txt"); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
	if info, err := fsys.Stat("big.txt"); err != nil || info.Size() != 1000 {
		t.Errorf("expected the declared size without a load, got: %v %v", info, err)
	}
}

func TestCleanName(t *testing.T) {
	for name, want := range map[string]string{
		"a/b.txt":     "a/b.txt",
		"/a/b.txt":    "a/b.txt",
		`a\b.txt`:     "a/b.txt",
		"./a//b.txt":  "a/b.txt",
		"a/../b.txt":  "b.txt",
		"../b.txt":    "",
		"/":           "",
		"":            "",
		"a/../../b":   "",
		"//server/x":  "server/x",
		"a/b/":        "a/b",
		"a/b/../../.": "",
	} {
		got, ok := CleanName(name)
		if got != want || ok != (want != "") {
			t.Errorf("CleanName(%q): expected %q, got: %q %v", name, want, got, ok)
		}
	}
}

func TestNewFS(t *testing.T) {
	fsys := NewFS()
	loads := 0
	if !fsys.AddFunc("Docs/Page.htm", 4, func() ([]byte, error) {
		loads++
		return []byte("page"), nil
	}) {
		t.Fatal("expected AddFunc to add a file")
	}
	if fsys.AddFunc("../evil", 1, nil) {
		t.Error("expected AddFunc to refuse a path that is not valid")
	}
	if info, err := fsys.Stat("Docs/Page.htm"); err != nil || info.Size() != 4 || loads != 0 {
		t.Errorf("expected the size without a load, got: %v %v %d", info, err, loads)
	}
	if err := fstest.TestFS(fsys, "Docs/Page.htm"); err != nil {
		t.Fatal(err)
	}
	if data, err := fs.ReadFile(fsys, "docs/page.HTM"); err != nil || string(data) != "page" {
		t.Errorf("expected a fall back on the case, got: %q %v", data, err)
	}
}
