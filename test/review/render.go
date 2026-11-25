package review

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xo/ivy"
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
	// Retry makes the references try again each failure that the cache
	// keeps from an earlier run.
	Retry bool
	// Log receives one line for each document. A nil Log writes nothing.
	Log io.Writer
	// Papers are the papers of a document whose file gives none, such as a
	// text file or an ebook (D32). Such a document renders once on each
	// paper, with the paper in its name. An empty Papers renders it once,
	// on the default paper of its format.
	Papers []Paper
	// SystemFonts gives ivy the fonts of the system, which the references
	// use too (D23).
	SystemFonts bool
	// FontDirs are folders of fonts that ivy and the references get more,
	// such as the folders of the fonts of the person, see DefaultFontDirs.
	FontDirs []string
	// Faults are the known faults of the references.
	Faults []Fault
	// TeX is the tree of TeX files of a TeX document, see TeXTree, or nil
	// for none (D65). TeXMode names it in the report.
	TeX     fs.FS
	TeXMode string
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
	// Papers are the papers of the documents without a paper (D32).
	Papers      []Paper  `json:"papers,omitempty"`
	SystemFonts bool     `json:"system_fonts"`
	FontDirs    []string `json:"font_dirs,omitempty"`
	// TeX is the tree of TeX files of a TeX document: TeXBase, TeXDist or
	// TeXNone (D65).
	TeX string `json:"tex,omitempty"`
	// Kinds sums the documents of each kind.
	Kinds []KindSummary `json:"kinds"`
}

// KindSummary sums the documents of one kind.
type KindSummary struct {
	Kind      string `json:"kind"`
	Documents int    `json:"documents"`
	// Pages counts the rows of the pages, without the rows of the MediaBox.
	Pages int `json:"pages"`
	// SizeDiffers counts the pages whose size differs from the page of the
	// primary reference.
	SizeDiffers int `json:"size_differs"`
}

// ExternalRef is an external reference of a document, such as Microsoft
// Office: a file that a program outside this machine made.
type ExternalRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// File is the path of the file in the cache, with slashes.
	File string `json:"file"`
	// Pages is the number of pages of the file, or -1 when it is not known.
	Pages int    `json:"pages"`
	Error string `json:"error,omitempty"`
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
	Disagree int `json:"disagree"`
	// IvyPDFWorst is, over the pages, the largest blurred difference of ivy
	// PDF from MuPDF. It is -1 without a metric. ivy PDF is the PDF file of
	// the conversion of the references, which ivy renders too. It shows the
	// gaps of the package pdf of ivy, and it is not a reference: Worst and
	// Disagree leave it out.
	IvyPDFWorst float64 `json:"ivy_pdf_worst"`
	// IvyPDFStatus is the worst status of ivy PDF over the pages. It is
	// empty when the document has no column ivy PDF, or no PDF file. It
	// does not change Status.
	IvyPDFStatus string `json:"ivy_pdf_status,omitempty"`
	// IvyPDFError is the error of ivy PDF on the PDF file, or on its first
	// page that failed.
	IvyPDFError string `json:"ivy_pdf_error,omitempty"`
	// IvyPDFLog holds the messages of the log of ivy PDF.
	IvyPDFLog []string `json:"ivy_pdf_log,omitempty"`
	// MediaBoxWorst is the worst of the rows of the MediaBox, as Worst is
	// of the other rows, or -1 without a metric. It is nil when the
	// document has no row of the MediaBox. Worst and Disagree leave these
	// rows out.
	MediaBoxWorst *float64 `json:"mediabox_worst,omitempty"`
	// Paper is the paper of a document whose file gives none (D32). The
	// name of the document then ends with the name of the paper, such as
	// "doc.txt [A4 landscape]".
	Paper *Paper `json:"paper,omitempty"`
	// Externals are the external references of the document, such as
	// Microsoft Office.
	Externals []ExternalRef `json:"external_references,omitempty"`
	// WorstBy is Worst for each reference by its id, such as "soffice" and
	// "msoffice". Worst uses the primary reference of each page.
	WorstBy map[string]float64 `json:"worst_by_reference,omitempty"`
	// SizeDiffers counts the pages whose size differs from the page of the
	// primary reference by more than one pixel.
	SizeDiffers int        `json:"size_differs_pages"`
	Rows        []*PageRow `json:"rows"`
}

// PageRow is one page of a document: the image of ivy and the images of
// the references.
type PageRow struct {
	// Page is the number of the page, from 1.
	Page int `json:"page"`
	// Box is BoxMedia for a row of the MediaBox of a PDF file, and empty
	// for a row of the CropBox, the default view (D31).
	Box string `json:"box,omitempty"`
	Ivy *Cell  `json:"ivy"`
	// IvyPDF is the page of the PDF file of the conversion, which ivy
	// renders. Its metric is its difference from MuPDF. It is nil for a
	// kind that the references do not convert to PDF.
	IvyPDF *Cell   `json:"ivy_pdf,omitempty"`
	Refs   []*Cell `json:"references"`
	// RefsDiff is the largest blurred difference between two references,
	// or -1.
	RefsDiff  float64 `json:"references_diff"`
	Disagrees bool    `json:"references_disagree"`
	// Primary is the id of the reference that Worst uses on the page: the
	// first reference with a metric, Microsoft Office before LibreOffice
	// for a file of Office.
	Primary string `json:"primary_reference,omitempty"`
	// SizeDiffers reports a page of ivy whose size differs from the page of
	// the primary reference by more than one pixel.
	SizeDiffers bool `json:"size_differs,omitempty"`
}

// BoxMedia is the Box of a row of the MediaBox.
const BoxMedia = "media"

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
	// Ref is the id of the reference of the cell, such as "soffice", and
	// RefName its name, when the document has more than one reference.
	Ref     string `json:"reference,omitempty"`
	RefName string `json:"reference_name,omitempty"`
	// Fault is the note of a known fault of the reference on the page. The
	// cell then counts for no number of the document.
	Fault string `json:"fault,omitempty"`
	// renderer is the id of the page renderer of a cell of a reference.
	renderer string
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
	cache, err := filepath.Abs(opts.Cache)
	if err != nil {
		return nil, fmt.Errorf("finding the cache: %w", err)
	}
	opts.Cache = cache
	selected := expandJobs(selectEntries(entries, opts.Match, opts.Kinds), opts.Papers)
	start := time.Now()
	fonts := newFontFS(filepath.Join(opts.Repo, "fonts"), opts.FontDirs)
	// the temporary folder of the run is in the cache, so that a result
	// moves into the cache with a rename on one file system
	runs := filepath.Join(opts.Cache, "tmp")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		return nil, fmt.Errorf("making the temporary folder of the cache: %w", err)
	}
	home, err := os.MkdirTemp(runs, "run-")
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
		Notes: notes, Counts: map[string]int{}, Papers: opts.Papers, SystemFonts: opts.SystemFonts,
		FontDirs: opts.FontDirs, TeX: opts.TeXMode,
	}
	var mu sync.Mutex
	done := 0
	rep.Docs = make([]*DocReport, len(selected))
	work := make(chan int)
	var wg sync.WaitGroup
	for slot := range opts.Jobs {
		wg.Go(func() {
			for i := range work {
				d := renderDoc(ctx, selected[i], i, slot, env, home, opts, fonts)
				mu.Lock()
				rep.Docs[i] = d
				done++
				if opts.Log != nil {
					number := func(v float64) string {
						if v < 0 {
							return "-"
						}
						return fmt.Sprintf("%.2f", v)
					}
					_, _ = fmt.Fprintf(opts.Log, "[%d/%d] %-8s %-5s pages %3d  worst %6s  ivy PDF %6s  %6.1fs  %s\n",
						done, len(selected), d.Status, d.Kind, d.Pages, number(d.Worst), number(d.IvyPDFWorst),
						d.TotalMS/1000, d.Name)
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
	rep.Kinds = sumKinds(rep.Docs)
	sortDocs(rep.Docs)
	if n := env.cachedFailures.Load(); n > 0 {
		note := fmt.Sprintf("%d failures of the references came from the cache of an earlier run. "+
			"To try them again, run render again with -retry.", n)
		rep.Notes = append(rep.Notes, note)
		if opts.Log != nil {
			_, _ = fmt.Fprintln(opts.Log, note)
		}
	}
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
// kind is in kinds, without the assets.
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

// sumKinds returns the sums of the documents of each kind, in the order of
// the kinds.
func sumKinds(docs []*DocReport) []KindSummary {
	index := map[string]int{}
	var out []KindSummary
	for _, d := range docs {
		i, ok := index[d.Kind]
		if !ok {
			i = len(out)
			index[d.Kind] = i
			out = append(out, KindSummary{Kind: d.Kind})
		}
		k := &out[i]
		k.Documents++
		k.SizeDiffers += d.SizeDiffers
		for _, r := range d.Rows {
			if r.Box == "" {
				k.Pages++
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
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

// unoScript is unopdf.py, which converts a file with a password to PDF
// with LibreOffice. toolEnv writes it into the folder of the run.
//
//go:embed unopdf.py
var unoScript []byte

// fontNames are the families that LibreOffice and calibre need to draw with
// the fonts of ivy.
var fontNames = []string{"Carlito", "Caladea", "Liberation Sans", "Liberation Serif", "Liberation Mono"}

// toolEnv returns the shared state of the references: an environment with
// HOME in the folder home, and a configuration of fontconfig that adds the
// fonts of ivy and the folders of opts.FontDirs. It returns a note for each
// family of fontNames that fontconfig does not find.
func toolEnv(ctx context.Context, opts RenderOptions, home string) (*refEnv, []string, error) {
	if err := findWeasyPrint(opts.Cache); err != nil {
		return nil, nil, err
	}
	profiles := filepath.Join(opts.Cache, "profiles")
	if err := os.MkdirAll(profiles, 0o755); err != nil {
		return nil, nil, fmt.Errorf("making the folder of the profiles: %w", err)
	}
	fonts, err := filepath.Abs(filepath.Join(opts.Repo, "fonts"))
	if err != nil {
		return nil, nil, fmt.Errorf("finding the fonts: %w", err)
	}
	conf := filepath.Join(profiles, "fonts.conf")
	var dirs strings.Builder
	for _, d := range opts.FontDirs {
		abs, err := filepath.Abs(d)
		if err != nil {
			return nil, nil, fmt.Errorf("finding the font folder %s: %w", d, err)
		}
		// the escapes of HTML are those of the text of XML
		dirs.WriteString("\n  <dir>" + html.EscapeString(abs) + "</dir>")
	}
	data := `<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <include ignore_missing="yes">/etc/fonts/fonts.conf</include>
  <dir>` + fonts + `</dir>` + dirs.String() + `
  <!-- ivy draws the generic families with Liberation (D28), so the
       references do too. fontconfig prefers other fonts by default. -->
  <alias binding="strong"><family>serif</family><prefer><family>Liberation Serif</family></prefer></alias>
  <alias binding="strong"><family>sans-serif</family><prefer><family>Liberation Sans</family></prefer></alias>
  <alias binding="strong"><family>monospace</family><prefer><family>Liberation Mono</family></prefer></alias>
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
	sum := sha256.Sum256([]byte(strings.Replace(data, filepath.Join(home, "fontconfig"), "", 1)))
	script := filepath.Join(home, "unopdf.py")
	if err := os.WriteFile(script, unoScript, 0o644); err != nil {
		return nil, nil, fmt.Errorf("writing unopdf.py: %w", err)
	}
	e := &refEnv{cache: opts.Cache, dpi: opts.DPI, retry: opts.Retry, unoScript: script, env: env,
		fonts: hex.EncodeToString(sum[:4]), keys: map[string]string{}, versions: map[string]string{}}
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

// docJob is one rendering of a document: the entry of the manifest, and the
// paper of a document whose file gives none, or nil.
type docJob struct {
	e     Entry
	paper *Paper
}

// expandJobs returns the renderings of the entries: one for each entry, and
// one for each paper of papers for an entry of a kind without a paper
// (D32).
func expandJobs(entries []Entry, papers []Paper) []docJob {
	var jobs []docJob
	for _, e := range entries {
		if !paperless(e.Kind) || len(papers) == 0 {
			jobs = append(jobs, docJob{e: e})
			continue
		}
		for _, p := range papers {
			jobs = append(jobs, docJob{e: e, paper: &p})
		}
	}
	return jobs
}

// name returns the name of the document in the report: the name of the
// manifest, with the name of the paper, such as "doc.txt [A4 landscape]".
func (jb docJob) name() string {
	if jb.paper == nil || jb.paper.IsDefault() {
		return jb.e.Name
	}
	return jb.e.Name + " [" + jb.paper.Name + "]"
}

// ivyConfig returns the configuration of ivy for a document with the
// password password and the paper paper.
func ivyConfig(opts RenderOptions, fonts fs.FS, password string, paper *Paper) ivy.Config {
	cfg := ivy.Config{Password: password, Fonts: fonts, SystemFonts: opts.SystemFonts, TeX: opts.TeX}
	if paper != nil {
		cfg.Paper = paper.Size()
	}
	return cfg
}

// renderDoc renders the document of jb with ivy and with its references,
// and writes its images into the folder of the report. fonts are the fonts
// of ivy.
func renderDoc(ctx context.Context, jb docJob, i, slot int, env *refEnv, home string, opts RenderOptions, fonts fs.FS) *DocReport {
	start := time.Now()
	e := jb.e
	d := &DocReport{Entry: e, Pages: -1, RefPages: -1, Worst: -1, IvyPDFWorst: -1, Paper: jb.paper}
	d.Name = jb.name()
	d.ID = fmt.Sprintf("%03d-%s", i+1, slug(d.Name))
	names := []string{e.Name, d.Name}
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
	cfg := ivyConfig(opts, fonts, e.Password(), jb.paper)
	doc, openTime, err := openIvy(ivyCtx, path, e.Kind, cfg)
	d.OpenMS = ms(openTime)
	stuck := errors.Is(err, errStuck)
	// a page can get stuck too, so the close checks stuck at the end
	defer func() {
		if doc != nil && !stuck {
			doc.close()
		}
	}()
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
	if jb.paper != nil {
		job.paper = jb.paper.Size()
	}
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
	}
	routeGroup := &refGroup{id: r.id, name: r.name, job: job, prep: prep, pages: -1, renderers: r.renderers, native: r.native}
	switch {
	case prep == nil || d.RefError != "":
		routeGroup.skip = "no reference"
	default:
		routeGroup.pages = d.RefPages
	}
	groups := refGroups(refCtx, d, job, routeGroup, jb.paper)

	// the time of the references does not count for ivy
	cancelIvy()
	ivyCtx, cancelIvy = context.WithTimeout(ctx, max(opts.Timeout-openTime, time.Millisecond))
	defer cancelIvy()

	ivyPages := d.Pages
	if d.Preview {
		ivyPages = 1
	}
	counts := []int{ivyPages}
	for _, g := range groups {
		if g.prep != nil && g.skip == "" {
			n := g.pages
			if n < 0 && ivyPages < 0 && g == routeGroup {
				// the references do not count their pages: try the first ones
				n = max(opts.Pages, 1)
				g.pages = n
			}
			counts = append(counts, n)
		}
	}
	pages := selectPages(opts.Pages, counts...)
	if d.Preview {
		pages = []int{0}
	}

	// ivy opens the PDF file of the conversion, with a time of its own
	pdfCtx, cancelPDF := context.WithTimeout(ctx, opts.Timeout)
	defer cancelPDF()
	var pdfDoc *ivyPDF
	if r.ivyPDF {
		pdfDoc = &ivyPDF{}
		if prep != nil && d.RefError == "" {
			pdfDoc.open(pdfCtx, d, prep.in, ivyConfig(opts, fonts, "", nil))
		}
		defer pdfDoc.close(d)
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

		// ivy PDF
		var pdfImg image.Image
		if pdfDoc != nil {
			row.IvyPDF, pdfImg = pdfDoc.page(pdfCtx, d, n, float32(opts.DPI), prefix+"-ivypdf", rel)
		}

		// the references
		refImgs := refCells(refCtx, groups, row, n, ivyImg, prefix, rel, opts.Faults, names)

		// ivy PDF compared with MuPDF of the route, which is not a reference
		for k, c := range row.Refs {
			if pdfImg != nil && c.Ref == r.id && c.renderer == mupdfRenderer.id && refImgs[k] != nil {
				compareCell(row.IvyPDF, pdfImg, refImgs[k], prefix+"-ivypdf-diff", rel)
				d.IvyPDFWorst = max(d.IvyPDFWorst, row.IvyPDF.Metric.Blurred)
			}
		}

		// the references compared with ivy and with each other
		m := rowMetrics(row, refImgs, groups)
		row.RefsDiff, row.Primary, row.SizeDiffers = m.refsDiff, m.primary, m.sizeDiffers
		row.Disagrees = row.RefsDiff > disagreeBlurred
		if row.Disagrees {
			d.Disagree++
		}
		if row.SizeDiffers {
			d.SizeDiffers++
		}
		d.Worst = max(d.Worst, m.best)
		for id, v := range m.by {
			if d.WorstBy == nil {
				d.WorstBy = map[string]float64{}
			}
			if old, ok := d.WorstBy[id]; !ok || v > old {
				d.WorstBy[id] = v
			}
		}

		// the MediaBox of a PDF file, if it differs from the CropBox
		if r.mediaBox && routeGroup.skip == "" && (routeGroup.pages < 0 || n < routeGroup.pages) {
			mediaRow(ivyCtx, refCtx, d, routeGroup, path, cfg, n, ivyDPI, filepath.Join(dir, fmt.Sprintf("p%d-media", n+1)), rel, opts, names)
		}
	}
	d.RenderMS = ms(renderTime)
	if doc != nil && doc.log != nil {
		d.Log = append(d.Log, doc.log.lines()...)
	}
	return d
}

// refGroups returns the references of the document d in their order: the
// external references that are primary for the kind, then the route, then
// the other external references. It sets the names of the references of
// d. An external reference that a program made on its own paper joins only
// a document on the default paper or on US Letter.
func refGroups(ctx context.Context, d *DocReport, job *refJob, route *refGroup, paper *Paper) []*refGroup {
	var first, last []*refGroup
	// the files of the external references take the name of the manifest,
	// without the paper
	e := d.Entry
	if isLetter(paper) {
		for _, x := range externals {
			if !contains(x.kinds, job.kind) {
				continue
			}
			g, err := job.prepareExternal(ctx, x, e)
			if g == nil && err == nil {
				continue
			}
			info := ExternalRef{ID: x.id, Name: x.name, File: x.display(e), Pages: -1}
			if g != nil {
				info.Pages = g.pages
			}
			if err != nil {
				info.Error = err.Error()
				if g == nil {
					continue
				}
				g.skip = "no reference: " + err.Error()
			}
			d.Externals = append(d.Externals, info)
			if contains(x.primary, job.kind) {
				first = append(first, g)
			} else {
				last = append(last, g)
			}
		}
	}
	var groups []*refGroup
	if len(route.renderers) != 0 {
		groups = append(groups, route)
	}
	groups = append(first, groups...)
	groups = append(groups, last...)
	for _, g := range groups {
		for _, pr := range g.renderers {
			name := pr.name
			if len(groups) > 1 && g.name != "" {
				name = g.name + " · " + pr.name
			}
			d.RefNames = append(d.RefNames, name)
		}
	}
	return groups
}

// refCells renders page n, from 0, with each page renderer of each group
// into a cell of row, and compares each image with ivyImg if it is not nil.
// It marks a cell that a fault of faults names, for the document of the
// names names. It returns the images of the cells, with nil for a failure.
func refCells(ctx context.Context, groups []*refGroup, row *PageRow, n int, ivyImg image.Image, prefix string,
	rel func(string) string, faults []Fault, names []string,
) []image.Image {
	var refImgs []image.Image
	for _, g := range groups {
		skip := g.skip
		if skip == "" && g.pages >= 0 && n >= g.pages {
			skip = "the reference has no such page"
		}
		for _, pr := range g.renderers {
			c := &Cell{Name: pr.name, Ref: g.id, renderer: pr.id}
			if len(groups) > 1 {
				c.RefName = g.name
			}
			c.Fault = faultNote(faults, names, n+1, g.id)
			row.Refs = append(row.Refs, c)
			refImgs = append(refImgs, nil)
			if skip != "" {
				c.Error = skip
				continue
			}
			t := time.Now()
			png, err := g.job.renderPage(ctx, g.prep, pr, g.native, n+1)
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
			out := prefix + "-" + g.files + pr.id
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
			if ivyImg != nil {
				compareCell(c, ivyImg, img, out+"-diff", rel)
			}
		}
	}
	return refImgs
}

// rowMetric holds the numbers of a row.
type rowMetric struct {
	// refsDiff is the largest blurred difference between two references,
	// or -1.
	refsDiff float64
	// best is the smallest blurred difference of ivy from a cell of the
	// primary reference, or -1.
	best float64
	// by is the smallest blurred difference of ivy from a cell of each
	// reference, by its id.
	by map[string]float64
	// primary is the id of the primary reference of the row: the first
	// group with a metric.
	primary string
	// sizeDiffers reports a page of ivy whose size differs from the page of
	// the primary reference by more than one pixel.
	sizeDiffers bool
}

// rowMetrics returns the numbers of row, whose cells of the references have
// the images refImgs, of the references groups. A cell with a fault counts
// for none of them.
func rowMetrics(row *PageRow, refImgs []image.Image, groups []*refGroup) rowMetric {
	m := rowMetric{refsDiff: -1, best: -1, by: map[string]float64{}}
	for a := range refImgs {
		for b := a + 1; b < len(refImgs); b++ {
			if refImgs[a] == nil || refImgs[b] == nil || row.Refs[a].Fault != "" || row.Refs[b].Fault != "" {
				continue
			}
			cm, _ := Compare(refImgs[b], refImgs[a], false)
			m.refsDiff = max(m.refsDiff, cm.Blurred)
		}
	}
	for _, c := range row.Refs {
		if c.Metric == nil || c.Fault != "" {
			continue
		}
		if v, ok := m.by[c.Ref]; !ok || c.Metric.Blurred < v {
			m.by[c.Ref] = c.Metric.Blurred
		}
	}
	for _, g := range groups {
		if v, ok := m.by[g.id]; ok {
			m.primary, m.best = g.id, v
			break
		}
	}
	if m.primary == "" || row.Ivy == nil || row.Ivy.Image == "" {
		return m
	}
	for _, c := range row.Refs {
		if c.Ref == m.primary && c.Fault == "" && c.Image != "" {
			m.sizeDiffers = sizeDiffers(row.Ivy.Size, c.Size)
			break
		}
	}
	return m
}

// sizeDiffers reports whether a side of the pages of the sizes a and b
// differs by more than one pixel.
func sizeDiffers(a, b image.Point) bool {
	return abs(a.X-b.X) > 1 || abs(a.Y-b.Y) > 1
}

// abs returns |v|.
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// mediaRow adds a row of the MediaBox of page n, from 0, of the PDF file at
// path to d, if the MediaBox of the page differs from its CropBox (D31).
// ivy renders the row with ivyMediaBox and the configuration cfg of the row
// of the CropBox, and the references of route with mediaRenderers, into
// images at prefix. The row does not count for Worst and Disagree of d. It
// counts for MediaBoxWorst.
func mediaRow(ivyCtx, refCtx context.Context, d *DocReport, route *refGroup, path string, cfg ivy.Config, n int, dpi float32,
	prefix string, rel func(string) string, opts RenderOptions, names []string,
) {
	differ, err := pdfBoxesDiffer(refCtx, route.job, route.prep.in, n+1)
	if err != nil {
		d.Log = append(d.Log, fmt.Sprintf("reading the page boxes of page %d: %v", n+1, err))
		return
	}
	if !differ {
		return
	}
	row := &PageRow{Page: n + 1, Box: BoxMedia}
	d.Rows = append(d.Rows, row)
	row.Ivy = &Cell{Name: "ivy"}
	t := time.Now()
	img, err := ivyMediaBox(ivyCtx, path, cfg, n, dpi)
	row.Ivy.MS = ms(time.Since(t))
	row.Ivy.Status = statusOf(ivyCtx, err)
	if err != nil {
		row.Ivy.Error = err.Error()
	}
	var ivyImg image.Image
	if img != nil && (err == nil || row.Ivy.Status == StatusPartial) {
		ivyImg = img
		row.Ivy.Size = img.Bounds().Size()
		if werr := writeCell(row.Ivy, img, prefix+"-ivy", rel); werr != nil {
			row.Ivy.Error = werr.Error()
		}
	}
	g := &refGroup{id: route.id, name: route.name, job: route.job, prep: route.prep, pages: -1, renderers: mediaRenderers}
	refImgs := refCells(refCtx, []*refGroup{g}, row, n, ivyImg, prefix, rel, opts.Faults, names)
	m := rowMetrics(row, refImgs, []*refGroup{g})
	row.RefsDiff = m.refsDiff
	worst := m.best
	if d.MediaBoxWorst != nil {
		worst = max(*d.MediaBoxWorst, m.best)
	}
	d.MediaBoxWorst = &worst
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

// compareCell sets the metric of c to the difference of img from ref, and
// writes the map of the difference as the PNG image and the thumbnail of
// c, at the path base with the extensions .png and .jpg.
func compareCell(c *Cell, img, ref image.Image, base string, rel func(string) string) {
	m, heat := Compare(img, ref, true)
	c.Metric = &m
	if heat == nil {
		return
	}
	if err := writePNG(base+".png", heat); err == nil {
		c.Diff = rel(base + ".png")
	}
	if err := writeThumb(base+".jpg", heat); err == nil {
		c.DiffThumb = rel(base + ".jpg")
	}
}

// ivyPDFName is the name of the column of ivy PDF.
const ivyPDFName = "ivy PDF"

// ivyPDF is the PDF file of the conversion of the references, which ivy
// renders as the column "ivy PDF". The column shows the gaps of the package
// pdf of ivy. It does not change the status of the document.
type ivyPDF struct {
	// path is the PDF file, or empty when the references made none.
	path string
	doc  *ivyDoc
	// ready reports whether ivy opened the file and counted its pages.
	ready bool
	// stuck reports a call of ivy that did not return after the end of its
	// context. ivy then keeps the file open.
	stuck bool
}

// open opens the PDF file at path with the package pdf of ivy, with the
// configuration cfg, and sets the status of ivy PDF in d.
func (p *ivyPDF) open(ctx context.Context, d *DocReport, path string, cfg ivy.Config) {
	p.path = path
	doc, _, err := openIvy(ctx, path, "pdf", cfg)
	p.doc = doc
	if err != nil {
		p.stuck = errors.Is(err, errStuck)
		d.IvyPDFStatus, d.IvyPDFError = statusOf(ctx, err), err.Error()
		return
	}
	p.ready = true
	d.IvyPDFStatus = StatusOK
}

// page renders page n, from 0, of the PDF file at dpi, and writes its
// images at the path base. It returns the cell of the page, and its image
// or nil. An error goes into the cell, and the first one into d.
func (p *ivyPDF) page(ctx context.Context, d *DocReport, n int, dpi float32, base string, rel func(string) string) (*Cell, image.Image) {
	c := &Cell{Name: ivyPDFName}
	switch {
	case p.path == "":
		c.Error = "the references made no PDF file"
		return c, nil
	case !p.ready:
		c.Status, c.Error = d.IvyPDFStatus, "ivy did not open the PDF file"
		return c, nil
	case p.stuck:
		c.Status, c.Error = StatusTimeout, "ivy is stuck on an earlier page"
		return c, nil
	case n >= p.doc.pages:
		c.Error = "the PDF file has no such page"
		return c, nil
	}
	t := time.Now()
	img, err := p.doc.render(ctx, n, dpi)
	c.MS = ms(time.Since(t))
	c.Status = statusOf(ctx, err)
	d.IvyPDFStatus = worse(d.IvyPDFStatus, c.Status)
	if err != nil {
		c.Error = err.Error()
		p.stuck = p.stuck || errors.Is(err, errStuck)
		if d.IvyPDFError == "" {
			d.IvyPDFError = fmt.Sprintf("page %d: %s", n+1, err)
		}
	}
	if img == nil || (err != nil && c.Status != StatusPartial) {
		return c, nil
	}
	c.Size = img.Bounds().Size()
	if werr := writeCell(c, img, base, rel); werr != nil {
		c.Error = werr.Error()
	}
	return c, img
}

// close keeps the log of ivy PDF in d, and closes the PDF file unless ivy
// is stuck on it.
func (p *ivyPDF) close(d *DocReport) {
	if p.doc == nil {
		return
	}
	if p.doc.log != nil {
		d.IvyPDFLog = p.doc.log.lines()
	}
	if !p.stuck {
		p.doc.close()
	}
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
	defer f.Close()
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
	if e.Kind == "html" || e.Kind == "md" || contains(texKinds, e.Kind) {
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
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing index.html: %w", err)
	}
	return nil
}
