package review

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestMoveFile moves a file across two file systems. rename fails as it
// does between two file systems, so moveFile copies the file.
func TestMoveFile(t *testing.T) {
	saved := rename
	t.Cleanup(func() { rename = saved })
	rename = func(oldpath, newpath string) error {
		if filepath.Dir(oldpath) != filepath.Dir(newpath) {
			return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.EXDEV}
		}
		return os.Rename(oldpath, newpath)
	}
	src := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(src, []byte("%PDF-1.7"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.pdf")
	if err := moveFile(src, dst); err != nil {
		t.Fatal(err)
	}
	checkMoved(t, src, dst, dir)
}

// TestMoveFileOtherFileSystem moves a file to a folder on another file
// system, if the system has one.
func TestMoveFileOtherFileSystem(t *testing.T) {
	dir := t.TempDir()
	other := otherFileSystem(t, dir)
	if other == "" {
		t.Skip("no other file system to write")
	}
	src := filepath.Join(other, "doc.pdf")
	if err := os.WriteFile(src, []byte("%PDF-1.7"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out.pdf")
	if err := moveFile(src, dst); err != nil {
		t.Fatal(err)
	}
	checkMoved(t, src, dst, dir)
}

// checkMoved checks that the file src is now dst, and that the folder dir
// holds no temporary file.
func checkMoved(t *testing.T, src, dst, dir string) {
	t.Helper()
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected %s to be removed, got: %v", src, err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "%PDF-1.7" {
		t.Errorf("expected the file at %s, got: %q %v", dst, data, err)
	}
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 {
		t.Errorf("expected only out.pdf in %s, got: %v", dir, names)
	}
}

// TestPrepareKeepsToolErrors checks that the cache keeps the failure of a
// program, and not another error, and that a run with retry tries a kept
// failure again.
func TestPrepareKeepsToolErrors(t *testing.T) {
	env := &refEnv{cache: t.TempDir(), dpi: 72, fonts: "f", keys: map[string]string{}, versions: map[string]string{}}
	j := &refJob{refEnv: env, kind: "x", hash: strings.Repeat("ab", 32), tmp: t.TempDir()}
	calls := 0
	var fail error
	r := &route{id: "test", prepare: func(_ context.Context, _ *refJob, _, dir string) (string, error) {
		calls++
		if fail != nil {
			return "", fail
		}
		return "out.txt", os.WriteFile(filepath.Join(dir, "out.txt"), nil, 0o644)
	}}

	// an error of the file system: the next run tries again
	fail = errors.New("moving the PDF file: invalid cross-device link")
	for range 2 {
		if _, err := j.prepare(t.Context(), r, "doc"); err == nil {
			t.Fatal("expected an error")
		}
	}
	if calls != 2 {
		t.Errorf("expected 2 conversions of an error that is not of the program, got: %d", calls)
	}

	// an error of the program: the next run takes it from the cache
	fail = &toolError{errors.New("running soffice: exit status 1")}
	for range 2 {
		_, err := j.prepare(t.Context(), r, "doc")
		if err == nil || !strings.Contains(err.Error(), "exit status 1") {
			t.Fatalf("expected the error of the program, got: %v", err)
		}
	}
	if calls != 3 {
		t.Errorf("expected the cache to keep the error of the program, got %d conversions", calls)
	}
	if n := env.cachedFailures.Load(); n != 1 {
		t.Errorf("expected 1 failure from the cache, got: %d", n)
	}

	// retry tries the kept failure again
	fail = nil
	env.retry = true
	p, err := j.prepare(t.Context(), r, "doc")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 || filepath.Base(p.in) != "out.txt" {
		t.Errorf("expected a conversion again, got %d conversions and %s", calls, p.in)
	}
	if _, err := os.Stat(filepath.Join(p.dir, errorFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected no %s after the retry, got: %v", errorFile, err)
	}
}

// TestRenderPassword converts files with a password with LibreOffice,
// through unopdf.py. It renders them from a folder of the cache on one
// file system while TMPDIR is on another, if the system has one.
func TestRenderPassword(t *testing.T) {
	for _, name := range []string{"mutool", "pdftoppm", "pdftocairo", "pdfinfo", "fc-list", "soffice"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("oracle %s is not installed: %v", name, err)
		}
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	if other := otherFileSystem(t, cache); other != "" {
		t.Setenv("TMPDIR", other)
	}
	entries := []Entry{
		{Name: "docx/standard.docx", Kind: "docx", URL: "repo:internal/ooxml/testdata/ecma376standard_password.docx", Size: -1, Note: "password=Password1234_"},
		{Name: "doc/rc4cryptoapi.doc", Kind: "doc", URL: "repo:msdoc/testdata/rc4cryptoapi_password.doc", Size: -1, Note: "password=Password1234_"},
		{Name: "odt/blowfish.odt", Kind: "odt", URL: "repo:odf/testdata/encrypted_blowfish.odt", Size: -1, Note: "password=secret"},
		{Name: "odt/argon2id.odt", Kind: "odt", URL: "repo:odf/testdata/encrypted_argon2id.odt", Size: -1, Note: "password=secret"},
		{Name: "odt/wrong.odt", Kind: "odt", URL: "repo:odf/testdata/encrypted_blowfish.odt", Size: -1, Note: "password=wrong"},
	}
	opts := RenderOptions{
		Repo: repo, Cache: cache, Out: filepath.Join(t.TempDir(), "out"), Pages: 1, DPI: 72, Jobs: 2, Timeout: time.Minute,
	}
	rep, err := Render(t.Context(), entries, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range rep.Docs {
		if d.Name == "odt/wrong.odt" {
			if !strings.Contains(d.RefError, "did not open the file with the password") {
				t.Errorf("%s: expected LibreOffice to refuse the wrong password, got: %q", d.Name, d.RefError)
			}
			continue
		}
		if d.RefError != "" || d.RefPages < 1 {
			t.Errorf("%s: expected a reference, got %d pages: %s", d.Name, d.RefPages, d.RefError)
			continue
		}
		for _, c := range d.Rows[0].Refs {
			if c.Error != "" {
				t.Errorf("%s: expected a page of %s, got: %s", d.Name, c.Name, c.Error)
			}
		}
	}
	// the run removes its temporary folder from the cache
	names, err := os.ReadDir(filepath.Join(cache, "tmp"))
	if err != nil || len(names) != 0 {
		t.Errorf("expected an empty folder tmp in the cache, got: %v %v", names, err)
	}
}

// TestRefGroups checks the references of a document without and with
// -refs all (D69): the primary oracle with MuPDF and Chrome, at most one
// secondary oracle with MuPDF, and the route alone without the file of
// the primary external reference.
func TestRefGroups(t *testing.T) {
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Fatalf("oracle pdfinfo is not installed: %v", err)
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pdf := filepath.Join(repo, "internal", "hayro", "interpret", "testdata", "clip_path_nested.pdf")
	cache := t.TempDir()
	for _, x := range externals {
		if x.id != "msoffice" && x.id != "iwork" {
			continue
		}
		for _, name := range []string{"a.docx", "a.odt", "a.pages"} {
			path := x.file(cache, Entry{Name: name})
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := linkOrCopy(pdf, path); err != nil {
				t.Fatal(err)
			}
		}
	}
	columns := func(groups []*refGroup) string {
		var out []string
		for _, g := range groups {
			for _, pr := range g.renderers {
				out = append(out, g.id+"/"+pr.id)
			}
		}
		return strings.Join(out, " ")
	}
	tests := []struct {
		name, kind string
		all        bool
		want       string
	}{
		{"a.docx", "docx", false, "msoffice/mupdf msoffice/chrome soffice/mupdf"},
		{"b.docx", "docx", false, "soffice/mupdf soffice/chrome"},
		{"a.odt", "odt", false, "soffice/mupdf soffice/chrome"},
		{"a.odt", "odt", true, "soffice/mupdf soffice/splash soffice/cairo soffice/chrome msoffice/mupdf msoffice/splash msoffice/cairo msoffice/chrome"},
		{"a.pages", "pages", false, "iwork/mupdf iwork/chrome"},
		{"b.pages", "pages", false, "soffice/mupdf soffice/chrome"},
		{"a.pages", "pages", true, "iwork/mupdf iwork/splash iwork/cairo iwork/chrome soffice/mupdf soffice/splash soffice/cairo soffice/chrome"},
		{"a.pdf", "pdf", false, "pdf/mupdf pdf/chrome"},
		{"a.xps", "xps", false, "xps/xpstopng"},
		{"a.md", "md", false, "markdownit/mupdf markdownit/chrome"},
		{"a.md", "md", true, "markdownit/mupdf markdownit/splash markdownit/cairo markdownit/chrome calibre/mupdf calibre/splash calibre/cairo calibre/chrome"},
	}
	for _, tt := range tests {
		env := &refEnv{cache: cache, dpi: 96, all: tt.all, keys: map[string]string{}}
		j := &refJob{refEnv: env, kind: tt.kind, hash: strings.Repeat("ab", 32), tmp: t.TempDir()}
		r := routeFor(tt.kind)
		route := &refGroup{id: r.id, name: r.name, job: j, pages: -1, renderers: r.renderers}
		d := &DocReport{Entry: Entry{Name: tt.name, Kind: tt.kind}}
		var extra []*refGroup
		if tt.all {
			for _, x := range extraRoutes[tt.kind] {
				extra = append(extra, &refGroup{id: x.id, name: x.name, job: j, pages: -1, renderers: x.renderers})
			}
		}
		if got := columns(refGroups(t.Context(), d, j, route, extra, nil)); got != tt.want {
			t.Errorf("%s, all %t: expected %q, got: %q", tt.name, tt.all, tt.want, got)
		}
		if len(d.RefNames) != len(strings.Fields(tt.want)) {
			t.Errorf("%s, all %t: expected a name for each column, got: %q", tt.name, tt.all, d.RefNames)
		}
	}
}
