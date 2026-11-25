package review

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// jsDelivr is the server of the files of the page of markdown-it.
const jsDelivr = "https://cdn.jsdelivr.net/"

// maxWalkFile bounds the size of one file of the walk.
const maxWalkFile = 64 << 20

// WalkOptions configure WalkMarkdownIt.
type WalkOptions struct {
	// Base is the URL of the server, with a slash at its end. An empty Base
	// is jsDelivr.
	Base string
	// Jobs is the number of downloads at the same time.
	Jobs int
	// Client makes the requests. A nil Client uses http.DefaultClient.
	Client *http.Client
	// Log receives one line for each file. A nil Log writes nothing.
	Log io.Writer
}

// walkImport finds the paths of jsDelivr in a module: the static and the
// dynamic imports, and the style sheets and the other files that a path
// names, all of them in a string of the module.
var walkImport = regexp.MustCompile(`["'](/npm/[^"'\s]+)["']`)

// walkURL finds the files that a style sheet names with url().
var walkURL = regexp.MustCompile(`url\(\s*["']?([^"')\s]+)["']?\s*\)`)

// walkPackage splits the name of a file of jsDelivr into its package and its
// version.
var walkPackage = regexp.MustCompile(`^npm/((?:@[^/@]+/)?[^/@]+)@([^/]+)/`)

// walkTheme is the one theme of Shiki that the page uses. The module of Shiki
// imports every theme, and the walk keeps this one.
const walkTheme = "github-light-default"

// walkSkip returns true for a name that the walk does not follow: a theme of
// Shiki other than walkTheme.
func walkSkip(name string) bool {
	rest, ok := strings.CutPrefix(name, "npm/@shikijs/themes@")
	if !ok {
		return false
	}
	_, theme, ok := strings.Cut(rest, "/")
	return ok && theme != walkTheme+"/+esm"
}

// WalkMarkdownIt returns the files of the page of markdown-it, as the
// manifest markdownit.tsv lists them, from the imports of the page and of the
// modules that it imports (D70). It starts at each path of jsDelivr in the
// page. It follows each path of jsDelivr in each module, both its static and
// its dynamic imports, except the themes of Shiki other than the theme of the
// page. It adds the fonts that a style sheet names, such as the fonts of
// KaTeX. Then it downloads each file for its size and its hash, and takes its
// license from the package.json of its package. The entries are sorted by
// name.
func WalkMarkdownIt(ctx context.Context, opts WalkOptions) ([]Entry, error) {
	base := opts.Base
	if base == "" {
		base = jsDelivr
	}
	w := &walker{
		base: base, jobs: max(opts.Jobs, 1), client: opts.Client, log: opts.Log,
		seen: map[string]*Entry{}, licenses: map[string]string{},
	}
	if w.client == nil {
		w.client = http.DefaultClient
	}
	found := walkImport.FindAllStringSubmatch(markdownItPage, -1)
	roots := make([]string, 0, len(found))
	for _, m := range found {
		roots = append(roots, strings.TrimPrefix(m[1], "/"))
	}
	if len(roots) == 0 {
		return nil, errors.New("the page of markdown-it imports no file of jsDelivr")
	}
	if err := w.walk(ctx, roots); err != nil {
		return nil, err
	}
	if err := w.fillLicenses(ctx); err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(w.seen))
	for _, e := range w.seen {
		entries = append(entries, *e)
	}
	slices.SortFunc(entries, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return entries, nil
}

// walker holds the state of one walk.
type walker struct {
	base   string
	jobs   int
	client *http.Client
	log    io.Writer

	mu       sync.Mutex
	seen     map[string]*Entry
	licenses map[string]string
}

// logf writes a line to the log.
func (w *walker) logf(format string, args ...any) {
	if w.log == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = fmt.Fprintf(w.log, format+"\n", args...)
}

// walk downloads the files of names and the files that they name, one level
// at a time, with w.jobs downloads at the same time.
func (w *walker) walk(ctx context.Context, names []string) error {
	for len(names) > 0 {
		var level []string
		for _, n := range names {
			if walkSkip(n) {
				continue
			}
			if _, ok := w.seen[n]; ok {
				continue
			}
			w.seen[n] = &Entry{Name: n}
			level = append(level, n)
		}
		slices.Sort(level)
		next := make([][]string, len(level))
		var errs []error
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, w.jobs)
		for i, n := range level {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				refs, err := w.get(ctx, n)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("%s: %w", n, err))
					mu.Unlock()
					return
				}
				next[i] = refs
			}()
		}
		wg.Wait()
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("walking the imports: %w", err)
		}
		if len(errs) != 0 {
			return fmt.Errorf("%d of %d files failed: %w", len(errs), len(level), errors.Join(errs...))
		}
		names = slices.Concat(next...)
	}
	return nil
}

// get downloads the file name, fills its entry, and returns the names of the
// files that it names.
func (w *walker) get(ctx context.Context, name string) ([]string, error) {
	data, err := w.download(ctx, w.base+name)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	e := w.seen[name]
	// each goroutine writes its own entry
	*e = Entry{
		Name: name, Kind: KindAsset, URL: jsDelivr + name, SHA256: hex.EncodeToString(sum[:]),
		Size: int64(len(data)),
	}
	if m := walkPackage.FindStringSubmatch(name); m != nil {
		e.Source = m[1] + "@" + m[2]
	}
	w.logf("got   %s", name)
	var refs []string
	switch {
	case strings.HasSuffix(name, "/+esm"):
		for _, m := range walkImport.FindAllStringSubmatch(string(data), -1) {
			refs = append(refs, strings.TrimPrefix(m[1], "/"))
		}
	case strings.HasSuffix(name, ".css"):
		for _, m := range walkURL.FindAllStringSubmatch(string(data), -1) {
			u, _, _ := strings.Cut(m[1], "?")
			u, _, _ = strings.Cut(u, "#")
			if strings.Contains(u, ":") || !strings.HasSuffix(u, ".woff2") {
				continue
			}
			refs = append(refs, path.Join(path.Dir(name), u))
		}
	}
	return refs, nil
}

// download returns the file at url. It tries a failed request once more.
func (w *walker) download(ctx context.Context, url string) ([]byte, error) {
	data, err := w.fetch(ctx, url)
	if err != nil && ctx.Err() == nil {
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
		}
		data, err = w.fetch(ctx, url)
	}
	return data, err
}

// fetch makes one request.
func (w *walker) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("making the request: %w", err)
	}
	req.Header.Set("User-Agent", "ivy-review")
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getting %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxWalkFile+1))
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	if len(data) > maxWalkFile {
		return nil, fmt.Errorf("%s is larger than %d bytes", url, maxWalkFile)
	}
	return data, nil
}

// fillLicenses sets the license of each entry to the field license of the
// package.json of its package.
func (w *walker) fillLicenses(ctx context.Context) error {
	var pkgs []string
	for _, e := range w.seen {
		if e.Source != "" && !slices.Contains(pkgs, e.Source) {
			pkgs = append(pkgs, e.Source)
		}
	}
	slices.Sort(pkgs)
	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, w.jobs)
	for _, p := range pkgs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := w.download(ctx, w.base+"npm/"+p+"/package.json")
			var lic string
			if err == nil {
				lic, err = packageLicense(data)
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("the license of %s: %w", p, err))
				return
			}
			w.licenses[p] = lic
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("reading the licenses: %w", err)
	}
	if len(errs) != 0 {
		return errors.Join(errs...)
	}
	for _, e := range w.seen {
		e.License = w.licenses[e.Source]
	}
	return nil
}

// packageLicense returns the field license of a package.json: a string, or
// the field type of an object.
func packageLicense(data []byte) (string, error) {
	var p struct {
		License json.RawMessage `json:"license"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return "", fmt.Errorf("reading package.json: %w", err)
	}
	var s string
	if err := json.Unmarshal(p.License, &s); err == nil && s != "" {
		return s, nil
	}
	var o struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(p.License, &o); err == nil && o.Type != "" {
		return o.Type, nil
	}
	return "", errors.New("package.json has no license")
}

// WriteMarkdownItManifest writes entries as the manifest markdownit.tsv: the
// header, then one line for each entry, with the columns separated by tabs.
func WriteMarkdownItManifest(w io.Writer, entries []Entry) error {
	bw := bufio.NewWriter(w)
	_, _ = bw.WriteString(strings.Join(manifestColumns, "\t") + "\n")
	for _, e := range entries {
		size := ""
		if e.Size >= 0 {
			size = fmt.Sprint(e.Size)
		}
		_, _ = bw.WriteString(strings.Join([]string{
			e.Name, e.Kind, e.URL, e.SHA256, size, e.License, e.Source, e.Note,
		}, "\t") + "\n")
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("writing the manifest: %w", err)
	}
	return nil
}
