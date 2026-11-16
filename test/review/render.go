package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// RenderOptions configure Render.
type RenderOptions struct {
	// Repo is the root of the repository of ivy. ivy reads its fonts from
	// the folder fonts.
	Repo string
	// Cache is the cache folder of the files and of the references.
	Cache string
	// Out is the folder of the report.
	Out string
	// Match selects the documents whose name it matches. A nil Match
	// selects each one.
	Match *regexp.Regexp
	// Kinds selects the documents of these kinds. An empty Kinds selects
	// each kind.
	Kinds []string
	// Pages is the number of first pages to render, with the last page. A
	// Pages of 0 or less renders each page.
	Pages int
	// DPI is the resolution in dots per inch.
	DPI int
	// Jobs is the number of documents at the same time.
	Jobs int
	// Timeout bounds the work of ivy on one document, and the work of the
	// references on it.
	Timeout time.Duration
	// Log receives one line for each document. A nil Log writes nothing.
	Log io.Writer
}

// Report holds the results of a run, as report.json holds them.
type Report struct {
	Created  time.Time      `json:"created"`
	Elapsed  float64        `json:"elapsed_seconds"`
	DPI      int            `json:"dpi"`
	Pages    int            `json:"pages"`
	Timeout  string         `json:"timeout"`
	Programs []Program      `json:"programs"`
	Notes    []string       `json:"notes,omitempty"`
	Docs     []*DocReport   `json:"documents"`
	Counts   map[string]int `json:"counts"`
}

// Program is an outside program of the references.
type Program struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// DocReport is the result of one document.
type DocReport struct {
	Entry
	// ID names the folder of the images and the anchor of the section.
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	// Log holds the messages of the log of ivy.
	Log []string `json:"log,omitempty"`
	// Pages is the number of pages of ivy, 0 for a document without pages,
	// or -1 when ivy did not open the document.
	Pages int `json:"pages"`
	// Preview reports a document without pages, which ivy shows with its
	// best preview.
	Preview  bool    `json:"preview,omitempty"`
	OpenMS   float64 `json:"open_ms"`
	RenderMS float64 `json:"render_ms"`
	TotalMS  float64 `json:"total_ms"`
	// Reference names the conversion of the references, such as
	// "LibreOffice", or is empty.
	Reference string `json:"reference,omitempty"`
	RefError  string `json:"reference_error,omitempty"`
	// RefPages is the number of pages of the references, or -1 when it is
	// not known.
	RefPages int      `json:"reference_pages"`
	RefNames []string `json:"reference_names,omitempty"`
	// Worst is, over the pages, the largest of the smallest blurred
	// difference of ivy from a reference. It is -1 without a metric.
	Worst float64 `json:"worst"`
	// Disagree counts the pages where the references disagree.
	Disagree int        `json:"disagree"`
	Rows     []*PageRow `json:"rows"`
}

// PageRow is one page of a document: the image of ivy and the images of
// the references.
type PageRow struct {
	// Page is the number of the page, from 1.
	Page int     `json:"page"`
	Ivy  *Cell   `json:"ivy"`
	Refs []*Cell `json:"references"`
	// RefsDiff is the largest blurred difference between two references,
	// or -1.
	RefsDiff  float64 `json:"references_diff"`
	Disagrees bool    `json:"references_disagree"`
}

// Cell is one image of a row.
type Cell struct {
	Name string `json:"name"`
	// Image, Thumb, Diff and DiffThumb are paths relative to the folder of
	// the report.
	Image     string      `json:"image,omitempty"`
	Thumb     string      `json:"thumb,omitempty"`
	Diff      string      `json:"diff,omitempty"`
	DiffThumb string      `json:"diff_thumb,omitempty"`
	Size      image.Point `json:"size"`
	MS        float64     `json:"ms"`
	Status    string      `json:"status,omitempty"`
	Error     string      `json:"error,omitempty"`
	Metric    *Metric     `json:"metric,omitempty"`
}

// ms returns d in milliseconds.
func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

// Render renders the documents of entries that the options select, writes
// the images, report.json and index.html into opts.Out, and returns the
// report.
func Render(ctx context.Context, entries []Entry, opts RenderOptions) (*Report, error) {
	if opts.Out == "" {
		return nil, errors.New("the folder of the report is not set")
	}
	if err := checkOut(opts.Out); err != nil {
		return nil, err
	}
	if opts.DPI <= 0 {
		opts.DPI = 96
	}
	if opts.Jobs <= 0 {
		opts.Jobs = max(1, runtime.GOMAXPROCS(0)/2)
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Minute
	}
	selected := selectEntries(entries, opts.Match, opts.Kinds)
	start := time.Now()
	home, err := os.MkdirTemp("", "ivy-review-")
	if err != nil {
		return nil, fmt.Errorf("making a temporary folder: %w", err)
	}
	defer func() { _ = os.RemoveAll(home) }()
	env, notes, err := toolEnv(ctx, opts, home)
	if err != nil {
		return nil, err
	}
	rep := &Report{
		Created: start.UTC(), DPI: opts.DPI, Pages: opts.Pages, Timeout: opts.Timeout.String(),
		Notes: notes, Counts: map[string]int{},
	}
	var mu sync.Mutex
	done := 0
	rep.Docs = make([]*DocReport, len(selected))
	work := make(chan int)
	var wg sync.WaitGroup
	for slot := range opts.Jobs {
		wg.Go(func() {
			for i := range work {
				d := renderDoc(ctx, selected[i], i, slot, env, home, opts)
				mu.Lock()
				rep.Docs[i] = d
				done++
				if opts.Log != nil {
					worst := "-"
					if d.Worst >= 0 {
						worst = fmt.Sprintf("%.2f", d.Worst)
					}
					_, _ = fmt.Fprintf(opts.Log, "[%d/%d] %-8s %-5s pages %3d  worst %6s  %6.1fs  %s\n",
						done, len(selected), d.Status, d.Kind, d.Pages, worst, d.TotalMS/1000, d.Name)
				}
				mu.Unlock()
			}
		})
	}
	for i := range selected {
		select {
		case work <- i:
		case <-ctx.Done():
		}
	}
	close(work)
	wg.Wait()
	docs := rep.Docs[:0]
	for _, d := range rep.Docs {
		if d != nil {
			docs = append(docs, d)
			rep.Counts[d.Status]++
		}
	}
	rep.Docs = docs
	sortDocs(rep.Docs)
	for _, name := range env.used() {
		if v := env.version(ctx, name); v != "" {
			rep.Programs = append(rep.Programs, Program{Name: name, Version: v})
		}
	}
	rep.Elapsed = time.Since(start).Seconds()
	if err := writeReport(opts.Out, rep); err != nil {
		return rep, err
	}
	if err := ctx.Err(); err != nil {
		return rep, fmt.Errorf("rendering: %w", err)
	}
	return rep, nil
}

// checkOut makes the folder of the report. It refuses a folder that holds
// files and no report.json, so that a wrong path does not mix the report
// with other files.
func checkOut(out string) error {
	entries, err := os.ReadDir(out)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(out, 0o755); err != nil {
			return fmt.Errorf("making the folder of the report: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading the folder of the report: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(out, "report.json")); err != nil {
		return fmt.Errorf("the folder %s holds files and no report.json: give an empty or a new folder", out)
	}
	return nil
}

// selectEntries returns the entries whose name match matches and whose
// kind is in kinds.
func selectEntries(entries []Entry, match *regexp.Regexp, kinds []string) []Entry {
	var out []Entry
	for _, e := range entries {
		if match != nil && !match.MatchString(e.Name) {
			continue
		}
		if e.Kind == KindAsset || (len(kinds) != 0 && !contains(kinds, e.Kind)) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// sortDocs sorts the documents from the worst: by status, then by the
// worst difference, then by name.
func sortDocs(docs []*DocReport) {
	sort.SliceStable(docs, func(i, j int) bool {
		a, b := docs[i], docs[j]
		if ra, rb := statusRank[a.Status], statusRank[b.Status]; ra != rb {
			return ra < rb
		}
		if a.Worst != b.Worst {
			return a.Worst > b.Worst
		}
		return a.Name < b.Name
	})
}

// fontNames are the families that LibreOffice and calibre need to draw with
// the fonts of ivy.
var fontNames = []string{"Carlito", "Caladea", "Liberation Sans", "Liberation Serif", "Liberation Mono"}

// toolEnv returns the shared state of the references: an environment with
// HOME in the folder home, and a configuration of fontconfig that adds the
// fonts of ivy. It returns a note for each family of fontNames that
// fontconfig does not find.
func toolEnv(ctx context.Context, opts RenderOptions, home string) (*refEnv, []string, error) {
	profiles := filepath.Join(opts.Cache, "profiles")
	if err := os.MkdirAll(profiles, 0o755); err != nil {
		return nil, nil, fmt.Errorf("making the folder of the profiles: %w", err)
	}
	fonts, err := filepath.Abs(filepath.Join(opts.Repo, "fonts"))
	if err != nil {
		return nil, nil, fmt.Errorf("finding the fonts: %w", err)
	}
	conf := filepath.Join(profiles, "fonts.conf")
	data := `<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <include ignore_missing="yes">/etc/fonts/fonts.conf</include>
  <dir>` + fonts + `</dir>
  <cachedir>` + filepath.Join(home, "fontconfig") + `</cachedir>
</fontconfig>
`
	if err := os.WriteFile(conf, []byte(data), 0o644); err != nil {
		return nil, nil, fmt.Errorf("writing the configuration of fontconfig: %w", err)
	}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "HOME", "FONTCONFIG_FILE", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "TMPDIR":
			continue
		}
		env = append(env, kv)
	}
	tmp := filepath.Join(home, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return nil, nil, fmt.Errorf("making a temporary folder: %w", err)
	}
	env = append(env, "HOME="+home, "FONTCONFIG_FILE="+conf, "TMPDIR="+tmp,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_CACHE_HOME="+filepath.Join(home, ".cache"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"))
	e := &refEnv{cache: opts.Cache, dpi: opts.DPI, env: env, keys: map[string]string{}, versions: map[string]string{}}
	var notes []string
	out, err := runTool(ctx, env, "", "fc-list", ":", "family")
	if err != nil {
		notes = append(notes, "fc-list did not run, so the fonts of LibreOffice and calibre are not known: "+err.Error())
	} else {
		for _, name := range fontNames {
			if !strings.Contains(out, name) {
				notes = append(notes, "fontconfig has no "+name+": LibreOffice and calibre draw it with another font")
			}
		}
	}
	return e, notes, nil
}

// renderDoc renders the document e with ivy and with its references, and
// writes its images into the folder of the report.
func renderDoc(ctx context.Context, e Entry, i, slot int, env *refEnv, home string, opts RenderOptions) *DocReport {
	start := time.Now()
	d := &DocReport{Entry: e, ID: fmt.Sprintf("%03d-%s", i+1, slug(e.Name)), Pages: -1, RefPages: -1, Worst: -1}
	defer func() { d.TotalMS = ms(time.Since(start)) }()
	dir := filepath.Join(opts.Out, "d", d.ID)
	if err := os.RemoveAll(dir); err != nil {
		d.Status, d.Error = StatusError, err.Error()
		return d
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		d.Status, d.Error = StatusError, err.Error()
		return d
	}
	path := e.Path(opts.Repo, opts.Cache)
	hash, size, err := hashFile(path)
	if err != nil {
		d.Status, d.Error = StatusMissing, err.Error()
		if _, ok := e.RepoPath(); !ok {
			d.Error += " (run review fetch)"
		}
		return d
	}
	if err := checkSum(e, size, hash); err != nil {
		d.Log = append(d.Log, "the file differs from the manifest: "+err.Error())
	}
	tmp, err := os.MkdirTemp(home, "doc-")
	if err != nil {
		d.Status, d.Error = StatusError, err.Error()
		return d
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	r := routeFor(e.Kind)
	ivyDPI := float32(opts.DPI)
	if r.native {
		ivyDPI = 72
	}

	// ivy opens the document
	ivyCtx, cancelIvy := context.WithTimeout(ctx, opts.Timeout)
	defer cancelIvy()
	doc, openTime, err := openIvy(ivyCtx, path, e.Kind, e.Password(), filepath.Join(opts.Repo, "fonts"))
	d.OpenMS = ms(openTime)
	stuck := errors.Is(err, errStuck)
	if doc != nil && !stuck {
		defer doc.close()
	}
	switch {
	case err != nil:
		d.Status = statusOf(ivyCtx, err)
		if _, ok := formatFor(e.Kind); !ok {
			d.Status = StatusUnsupported
		}
		d.Error = err.Error()
	default:
		d.Status = StatusOK
		d.Pages = doc.pages
		d.Preview = doc.pager == nil
	}

	// the references prepare
	refCtx, cancelRefs := context.WithTimeout(ctx, opts.Timeout)
	defer cancelRefs()
	job := &refJob{refEnv: env, kind: e.Kind, password: e.Password(), slot: slot, hash: hash, tmp: tmp}
	d.Reference = r.name
	var prep *prepared
	if len(r.renderers) != 0 {
		in, err := linkInput(tmp, path, e)
		if err == nil {
			prep, err = job.prepare(refCtx, r, in)
		}
		if err != nil {
			d.RefError = err.Error()
		}
		if prep != nil {
			d.RefPages = prep.pages
		}
		for _, pr := range r.renderers {
			d.RefNames = append(d.RefNames, pr.name)
		}
	}

	// the time of the references does not count for ivy
	cancelIvy()
	ivyCtx, cancelIvy = context.WithTimeout(ctx, max(opts.Timeout-openTime, time.Millisecond))
	defer cancelIvy()

	ivyPages := d.Pages
	if d.Preview {
		ivyPages = 1
	}
	refPages := -1
	if prep != nil && d.RefError == "" {
		refPages = d.RefPages
		if refPages < 0 && ivyPages < 0 {
			// the references do not count their pages: try the first ones
			refPages = max(opts.Pages, 1)
		}
	}
	pages := selectPages(opts.Pages, ivyPages, refPages)
	if d.Preview {
		pages = []int{0}
	}

	var renderTime time.Duration
	for _, n := range pages {
		row := &PageRow{Page: n + 1, RefsDiff: -1}
		d.Rows = append(d.Rows, row)
		prefix := filepath.Join(dir, fmt.Sprintf("p%d", n+1))
		rel := func(p string) string {
			r, _ := filepath.Rel(opts.Out, p)
			return filepath.ToSlash(r)
		}

		// ivy
		row.Ivy = &Cell{Name: "ivy"}
		var ivyImg image.Image
		switch {
		case doc == nil || doc.doc == nil:
			row.Ivy.Status, row.Ivy.Error = d.Status, "ivy did not open the document"
		case stuck:
			row.Ivy.Status, row.Ivy.Error = StatusTimeout, "ivy is stuck on an earlier page"
		case n >= ivyPages:
			row.Ivy.Error = "ivy has no such page"
		default:
			t := time.Now()
			img, err := doc.render(ivyCtx, n, ivyDPI)
			el := time.Since(t)
			renderTime += el
			row.Ivy.MS = ms(el)
			row.Ivy.Status = statusOf(ivyCtx, err)
			if err != nil {
				row.Ivy.Error = err.Error()
				stuck = stuck || errors.Is(err, errStuck)
			}
			d.Status = worse(d.Status, row.Ivy.Status)
			if img != nil && (err == nil || row.Ivy.Status == StatusPartial) {
				ivyImg = img
				row.Ivy.Size = img.Bounds().Size()
				if werr := writeCell(row.Ivy, img, prefix+"-ivy", rel); werr != nil {
					row.Ivy.Error = werr.Error()
				}
			}
		}

		// the references
		var refImgs []image.Image
		for _, pr := range r.renderers {
			c := &Cell{Name: pr.name}
			row.Refs = append(row.Refs, c)
			refImgs = append(refImgs, nil)
			switch {
			case prep == nil || d.RefError != "":
				c.Error = "no reference"
				continue
			case refPages >= 0 && n >= refPages:
				c.Error = "the reference has no such page"
				continue
			}
			t := time.Now()
			png, err := job.renderPage(refCtx, prep, pr, r.native, n+1)
			c.MS = ms(time.Since(t))
			if err != nil {
				c.Error = err.Error()
				continue
			}
			img, err := readImage(png)
			if err != nil {
				c.Error = err.Error()
				continue
			}
			refImgs[len(refImgs)-1] = img
			c.Size = img.Bounds().Size()
			out := prefix + "-" + pr.id
			if err := linkOrCopy(png, out+".png"); err != nil {
				c.Error = err.Error()
				continue
			}
			c.Image = rel(out + ".png")
			if err := writeThumb(out+".jpg", img); err != nil {
				c.Error = err.Error()
				continue
			}
			c.Thumb = rel(out + ".jpg")
			if ivyImg == nil {
				continue
			}
			m, heat := Compare(ivyImg, img, true)
			c.Metric = &m
			if err := writePNG(out+"-diff.png", heat); err == nil {
				c.Diff = rel(out + "-diff.png")
			}
			if err := writeThumb(out+"-diff.jpg", heat); err == nil {
				c.DiffThumb = rel(out + "-diff.jpg")
			}
		}

		// the references compared with each other
		for a := range refImgs {
			for b := a + 1; b < len(refImgs); b++ {
				if refImgs[a] == nil || refImgs[b] == nil {
					continue
				}
				m, _ := Compare(refImgs[b], refImgs[a], false)
				row.RefsDiff = max(row.RefsDiff, m.Blurred)
			}
		}
		row.Disagrees = row.RefsDiff > disagreeBlurred
		if row.Disagrees {
			d.Disagree++
		}
		best := -1.0
		for _, c := range row.Refs {
			if c.Metric != nil && (best < 0 || c.Metric.Blurred < best) {
				best = c.Metric.Blurred
			}
		}
		d.Worst = max(d.Worst, best)
	}
	d.RenderMS = ms(renderTime)
	if doc != nil && doc.log != nil {
		d.Log = append(d.Log, doc.log.lines()...)
	}
	return d
}

// writeCell writes img as the PNG image and the thumbnail of c, at the path
// base with the extensions .png and .jpg.
func writeCell(c *Cell, img image.Image, base string, rel func(string) string) error {
	if err := writePNG(base+".png", img); err != nil {
		return err
	}
	c.Image = rel(base + ".png")
	if err := writeThumb(base+".jpg", img); err != nil {
		return err
	}
	c.Thumb = rel(base + ".jpg")
	return nil
}

// selectPages returns the pages to render, from 0: the first n pages and
// the last page of each count that is not negative, or each page when n is
// 0 or less. It renders at most maxPages pages.
func selectPages(n int, counts ...int) []int {
	const maxPages = 1000
	set := map[int]bool{}
	for _, c := range counts {
		if c <= 0 {
			continue
		}
		first := n
		if n <= 0 {
			first = c
		}
		for p := range min(first, c, maxPages) {
			set[p] = true
		}
		set[c-1] = true
	}
	pages := make([]int, 0, len(set))
	for p := range set {
		pages = append(pages, p)
	}
	sort.Ints(pages)
	return pages
}

// hashFile returns the sha256 of the file at path, in hexadecimal, and its
// size.
func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("opening the file: %w", err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("reading the file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// linkInput makes a symbolic link to the file path in the folder tmp, with
// the extension of the kind, because some programs read the extension.
func linkInput(tmp, path string, e Entry) (string, error) {
	if e.Kind == "html" || e.Kind == "md" {
		// A document that links to other files must stay in its folder.
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("finding the file: %w", err)
		}
		return abs, nil
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(e.Name)), ".")
	if ext == "" || routeFor(ext) != routeFor(e.Kind) {
		ext = e.Kind
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("finding the file: %w", err)
	}
	link := filepath.Join(tmp, "doc."+ext)
	if err := os.Symlink(abs, link); err != nil {
		if err := linkOrCopy(abs, link); err != nil {
			return "", err
		}
	}
	return link, nil
}

// slugChars are the characters that a slug keeps.
var slugChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// slug returns name as the name of a folder.
func slug(name string) string {
	s := slugChars.ReplaceAllString(strings.ReplaceAll(name, "/", "__"), "_")
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// writeReport writes report.json and index.html into out.
func writeReport(out string, rep *Report) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the report: %w", err)
	}
	if err := os.WriteFile(filepath.Join(out, "report.json"), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing report.json: %w", err)
	}
	f, err := os.Create(filepath.Join(out, "index.html"))
	if err != nil {
		return fmt.Errorf("making index.html: %w", err)
	}
	if err := WriteHTML(f, rep); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing index.html: %w", err)
	}
	return nil
}
