// Package review renders the documents of a corpus with ivy and with
// reference programs, and writes a report in HTML that shows the pages side
// by side, for a maintainer who judges the rendering of ivy by eye (D5,
// D13). The command github.com/xo/ivy/test/cmd/review runs it.
package review

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// KindAsset is the kind of a file that a document links to, such as a
// style sheet, a font or an image of an HTML file. fetch downloads it, and
// render does not render it.
const KindAsset = "asset"

// manifestColumns are the columns of the manifest, in order.
var manifestColumns = []string{"name", "kind", "url", "sha256", "size", "license", "source", "note"}

// Entry is one document of the manifest of the corpus.
type Entry struct {
	// Name is a unique relative path, with slashes. The file of a URL goes
	// at this path in the cache.
	Name string `json:"name"`
	// Kind is the format, such as "pdf" or "docx".
	Kind string `json:"kind"`
	// URL is an https URL, or "repo:" and a path in the repository of ivy.
	URL string `json:"url"`
	// SHA256 is the hash of the file in hexadecimal, or empty.
	SHA256 string `json:"sha256,omitempty"`
	// Size is the size of the file in bytes, or -1 when it is not known.
	Size    int64  `json:"size"`
	License string `json:"license,omitempty"`
	Source  string `json:"source,omitempty"`
	Note    string `json:"note,omitempty"`
	// Line is the line of the entry in the manifest.
	Line int `json:"-"`
}

// repoPrefix starts the URL of a file in the repository of ivy.
const repoPrefix = "repo:"

// RepoPath returns the path of the file in the repository, relative to its
// root, and true, when the URL names a file of the repository.
func (e Entry) RepoPath() (string, bool) {
	p, ok := strings.CutPrefix(e.URL, repoPrefix)
	return p, ok
}

// Password returns the password that the note gives as password=XYZ, or an
// empty string.
func (e Entry) Password() string {
	for f := range strings.FieldsSeq(e.Note) {
		f = strings.TrimRight(f, ";,")
		if p, ok := strings.CutPrefix(f, "password="); ok {
			return p
		}
	}
	return ""
}

// Path returns the path of the file of the entry: in the repository repo,
// or in the cache folder cache.
func (e Entry) Path(repo, cache string) string {
	if p, ok := e.RepoPath(); ok {
		return filepath.Join(repo, filepath.FromSlash(p))
	}
	return filepath.Join(cache, "files", filepath.FromSlash(e.Name))
}

// LoadManifest reads the manifest file at path.
func LoadManifest(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening the manifest: %w", err)
	}
	defer func() { _ = f.Close() }()
	entries, err := ReadManifest(f)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return entries, nil
}

// ReadManifest reads a manifest: a header line, then one line for each
// document, with the columns separated by tabs. An empty line and a line
// that starts with # are skipped. The columns at the end of a line can be
// left out.
func ReadManifest(r io.Reader) ([]Entry, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	var entries []Entry
	header := false
	names := map[string]int{}
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(text) == "" || strings.HasPrefix(text, "#") {
			continue
		}
		cols := strings.Split(text, "\t")
		if !header {
			if err := checkHeader(cols); err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			header = true
			continue
		}
		e, err := parseEntry(cols)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		e.Line = line
		if first, ok := names[e.Name]; ok {
			return nil, fmt.Errorf("line %d: the name %q is also on line %d", line, e.Name, first)
		}
		names[e.Name] = line
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading the manifest: %w", err)
	}
	if !header {
		return nil, errors.New("the manifest has no header line")
	}
	return entries, nil
}

// checkHeader returns an error if cols are not the columns of a manifest.
func checkHeader(cols []string) error {
	if len(cols) != len(manifestColumns) {
		return fmt.Errorf("expected the header %q, got: %q", strings.Join(manifestColumns, "\t"), strings.Join(cols, "\t"))
	}
	for i, c := range cols {
		if strings.TrimSpace(c) != manifestColumns[i] {
			return fmt.Errorf("expected the column %q at %d, got: %q", manifestColumns[i], i+1, c)
		}
	}
	return nil
}

// parseEntry returns the entry of the columns of one line.
func parseEntry(cols []string) (Entry, error) {
	if len(cols) > len(manifestColumns) {
		return Entry{}, fmt.Errorf("expected at most %d columns, got: %d", len(manifestColumns), len(cols))
	}
	if len(cols) < 3 {
		return Entry{}, fmt.Errorf("expected at least the name, the kind and the url, got: %d columns", len(cols))
	}
	get := func(i int) string {
		if i < len(cols) {
			return strings.TrimSpace(cols[i])
		}
		return ""
	}
	e := Entry{
		Name:    get(0),
		Kind:    strings.ToLower(get(1)),
		URL:     get(2),
		SHA256:  strings.ToLower(get(3)),
		Size:    -1,
		License: get(5),
		Source:  get(6),
		Note:    get(7),
	}
	if err := checkName(e.Name); err != nil {
		return Entry{}, err
	}
	if e.Kind == "" {
		return Entry{}, fmt.Errorf("the kind of %s is empty", e.Name)
	}
	switch p, ok := e.RepoPath(); {
	case ok:
		if err := checkName(p); err != nil {
			return Entry{}, fmt.Errorf("the repository path of %s: %w", e.Name, err)
		}
	case !strings.HasPrefix(e.URL, "https://"):
		return Entry{}, fmt.Errorf("expected an https url or %s for %s, got: %q", repoPrefix, e.Name, e.URL)
	}
	if e.SHA256 != "" {
		if len(e.SHA256) != 64 || strings.Trim(e.SHA256, "0123456789abcdef") != "" {
			return Entry{}, fmt.Errorf("expected a sha256 of 64 hexadecimal digits for %s, got: %q", e.Name, e.SHA256)
		}
	}
	if s := get(4); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 0 {
			return Entry{}, fmt.Errorf("expected a size in bytes for %s, got: %q", e.Name, s)
		}
		e.Size = n
	}
	return e, nil
}

// checkName returns an error if name is not a clean relative path with
// slashes that stays inside its folder.
func checkName(name string) error {
	switch {
	case name == "":
		return errors.New("the name is empty")
	case strings.Contains(name, `\`):
		return fmt.Errorf("expected slashes in the name %q", name)
	case path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../"):
		return fmt.Errorf("expected a clean relative path, got: %q", name)
	}
	return nil
}
