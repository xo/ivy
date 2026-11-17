package review

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/sheet"
)

// disagreeBlurred is the blurred difference above which two references
// disagree on a page. Two PDF renderers that draw a page the same differ
// by about 1 after the blur.
const disagreeBlurred = 4.0

// refEnv is what the references of a run share.
type refEnv struct {
	cache string
	dpi   int
	// retry makes the references try a failure that the cache keeps again.
	retry bool
	// unoScript is the path of unopdf.py in the folder of the run.
	unoScript string
	// env is the environment of each program: HOME in a temporary folder,
	// and the fonts of ivy for fontconfig.
	env []string
	// fonts is a short hash of the configuration of fontconfig, which the
	// names of the conversions in the cache hold, because the fonts change
	// the pages of a conversion.
	fonts string

	mu       sync.Mutex
	keys     map[string]string
	versions map[string]string

	// cachedFailures counts the failures that came from the cache.
	cachedFailures atomic.Int64
	// pipes numbers the pipes of UNO, so that each soffice has its own.
	pipes atomic.Int64

	unoOnce sync.Once
	uno     unoPython
	unoErr  error
}

// toolError is an error of a reference program: the program failed, did
// not finish in time, or wrote no result. The cache keeps a toolError,
// because the next run gets the same error. It does not keep another
// error, such as an error of the file system, because the next run can
// succeed.
type toolError struct {
	err error
}

func (e *toolError) Error() string {
	return e.err.Error()
}

func (e *toolError) Unwrap() error {
	return e.err
}

// isToolError reports whether the cache keeps err.
func isToolError(err error) bool {
	var te *toolError
	return errors.As(err, &te)
}

// refJob is the work of the references on one document.
type refJob struct {
	*refEnv
	kind     string
	password string
	// slot is the number of the worker, which owns a profile of LibreOffice
	// and of calibre.
	slot int
	// hash is the sha256 of the file.
	hash string
	// tmp is a temporary folder of the document.
	tmp string
	// paper is the paper of a document whose file gives none, or the zero
	// paper for the default of the kind (D32).
	paper ivy.Paper
}

// pageRenderer draws one page of a file to a PNG file.
type pageRenderer struct {
	id   string
	name string
	// tag changes with the options of the programs, to change the key of
	// the cache.
	tag   string
	tools []string
	// render draws page n, from 1, of in to the image out.
	render func(ctx context.Context, j *refJob, in string, n int, out string) error
}

// route says how the references draw a kind: a conversion of the file,
// such as LibreOffice to PDF, then the programs that draw each page.
type route struct {
	id    string
	name  string
	tools []string
	// perDPI makes the conversion depend on the dots per inch.
	perDPI bool
	// prepare converts doc into the folder dir, and returns the name of the
	// result in dir. A route without prepare draws the file itself.
	prepare func(ctx context.Context, j *refJob, doc, dir string) (string, error)
	// count returns the number of pages of in. A nil count means that the
	// number is not known.
	count     func(ctx context.Context, j *refJob, in string) (int, error)
	renderers []pageRenderer
	// native renders ivy at 72 dots per inch, one pixel for each pixel of a
	// raster image, and the references at the size of the image.
	native bool
	// ivyPDF makes ivy render the PDF file of the conversion too, as the
	// column "ivy PDF", and compare it with MuPDF. Only a route that
	// converts the document to PDF sets it.
	ivyPDF bool
	// uno makes the route convert a file with a password through the UNO
	// API of LibreOffice, with unopdf.py and the Python of unoPython.
	uno bool
	// mediaBox makes the route add a row of the MediaBox for a page of a
	// PDF file whose MediaBox differs from its CropBox (D31).
	mediaBox bool
	// variant returns the options of the conversion of a job, such as the
	// options of calibre. The name of the conversion in the cache holds a
	// hash of it, so a change of the options converts the file again. A nil
	// variant or an empty string adds nothing to the name.
	variant func(j *refJob) string
}

// The page renderers of a PDF file. They draw the CropBox, as ivy does.
// MuPDF draws it by default, and Poppler draws it with -cropbox. The tag
// "crop" changes the key of the cache of Poppler, which drew the MediaBox
// before.
var (
	mupdfRenderer = pageRenderer{id: "mupdf", name: "MuPDF", tools: []string{"mutool"},
		render: func(ctx context.Context, j *refJob, in string, n int, out string) error {
			return mutoolDraw(ctx, j, nil, in, n, out)
		}}
	splashRenderer = pageRenderer{id: "splash", name: "Poppler Splash", tag: "crop", tools: []string{"pdftoppm"},
		render: func(ctx context.Context, j *refJob, in string, n int, out string) error {
			return popplerDraw(ctx, j, "pdftoppm", []string{"-aa", "yes", "-aaVector", "yes", "-cropbox"}, in, n, out)
		}}
	// The anti-aliasing "best" of pdftocairo breaks the glyphs of text,
	// and "gray" draws them as "default" does, without subpixels.
	cairoRenderer = pageRenderer{id: "cairo", name: "Poppler Cairo", tag: "graycrop", tools: []string{"pdftocairo"},
		render: func(ctx context.Context, j *refJob, in string, n int, out string) error {
			return popplerDraw(ctx, j, "pdftocairo", []string{"-antialias", "gray", "-cropbox"}, in, n, out)
		}}
	pdfRenderers = []pageRenderer{mupdfRenderer, splashRenderer, cairoRenderer}
)

// The page renderers of the rows of the MediaBox of a PDF file (D31).
var (
	mupdfMediaRenderer = pageRenderer{id: "mupdf", name: "MuPDF", tag: "media", tools: []string{"mutool"},
		render: func(ctx context.Context, j *refJob, in string, n int, out string) error {
			return mutoolDraw(ctx, j, []string{"-b", "mediabox"}, in, n, out)
		}}
	splashMediaRenderer = pageRenderer{id: "splash", name: "Poppler Splash", tag: "media", tools: []string{"pdftoppm"},
		render: func(ctx context.Context, j *refJob, in string, n int, out string) error {
			return popplerDraw(ctx, j, "pdftoppm", []string{"-aa", "yes", "-aaVector", "yes"}, in, n, out)
		}}
	cairoMediaRenderer = pageRenderer{id: "cairo", name: "Poppler Cairo", tag: "graymedia", tools: []string{"pdftocairo"},
		render: func(ctx context.Context, j *refJob, in string, n int, out string) error {
			return popplerDraw(ctx, j, "pdftocairo", []string{"-antialias", "gray"}, in, n, out)
		}}
	mediaRenderers = []pageRenderer{mupdfMediaRenderer, splashMediaRenderer, cairoMediaRenderer}
)

// The routes.
var (
	pdfRoute     = &route{id: "pdf", count: pdfCount, renderers: pdfRenderers, mediaBox: true}
	sofficeRoute = &route{id: "soffice", name: "LibreOffice", tools: []string{"soffice"},
		prepare: sofficePDF, count: pdfCount, renderers: pdfRenderers, ivyPDF: true, uno: true,
		variant: func(j *refJob) string { return j.unoPage() }}
	calibreRoute = &route{id: "calibre", name: "calibre", tools: []string{"ebook-convert"},
		prepare: calibrePDF, count: pdfCount, renderers: pdfRenderers, ivyPDF: true,
		variant: func(j *refJob) string { return strings.Join(calibreOptions(j.kind, j.paper), "\x00") }}
	// weasyprintRoute lays out HTML with WeasyPrint, whose layout ivy ports
	// (D15), with the style sheet that ivy gives an HTML file.
	weasyprintRoute = &route{id: "weasyprint", name: "WeasyPrint", tools: []string{"weasyprint"},
		prepare: weasyprintPDF, count: pdfCount, renderers: pdfRenderers, ivyPDF: true,
		variant: func(j *refJob) string { return ebook.PaperStyleFor(j.paper) }}
	xpsRoute = &route{id: "xps", name: "libgxps",
		renderers: []pageRenderer{{id: "xpstopng", name: "libgxps", tools: []string{"xpstopng"}, render: xpstopng}}}
	gsRoute = &route{id: "gs", name: "Ghostscript", tools: []string{"gs"}, perDPI: true,
		prepare: ghostscript, count: countPages,
		renderers: []pageRenderer{{id: "gs", name: "Ghostscript", render: pickPage}}}
	inkscapeRoute = &route{id: "inkscape", name: "Inkscape", count: one,
		renderers: []pageRenderer{{id: "inkscape", name: "Inkscape", tools: []string{"inkscape"}, render: inkscape}}}
	djvuRoute = &route{id: "djvu", name: "DjVuLibre", count: djvuCount,
		renderers: []pageRenderer{{id: "ddjvu", name: "DjVuLibre", tools: []string{"ddjvu"}, render: ddjvu}}}
	magickRoute = &route{id: "magick", name: "ImageMagick", count: one, native: true,
		renderers: []pageRenderer{{id: "magick", name: "ImageMagick", tag: "srgb", tools: []string{"magick"}, render: magick}}}
	// noRoute is the route of a kind without a reference.
	noRoute = &route{id: "none"}
	// rasterRoute is the route of an archive of images without a
	// reference, which ivy renders at the size of the images.
	rasterRoute = &route{id: "none", native: true}
)

// routeKinds maps each route to its kinds.
var routeKinds = map[*route][]string{
	pdfRoute: {"pdf", "ai"},
	sofficeRoute: {
		"docx", "dotx", "docm", "dotm", "doc", "dot",
		"xlsx", "xltx", "xlsm", "xltm", "xls", "xlt",
		"pptx", "potx", "ppsx", "pptm", "ppsm", "ppt", "pps", "pot",
		"odt", "ott", "odm", "ods", "ots", "odp", "otp", "odg", "otg",
		"rtf", "csv", "tsv", "tab",
		"vsdx", "vsdm", "vstx", "vstm", "vsd", "vst", "vss",
		"pub", "pages", "key", "numbers",
	},
	calibreRoute:    {"epub", "kepub", "mobi", "prc", "azw", "azw3", "azw4", "fb2", "fbz", "htmlz", "txtz", "chm", "md", "markdown", "txt", "text"},
	weasyprintRoute: {"html", "htm", "xhtml"},
	xpsRoute:        {"xps", "oxps"},
	gsRoute:         {"ps", "eps", "epsi", "epsf", "ept"},
	inkscapeRoute:   {"emf", "wmf", "emz", "wmz"},
	djvuRoute:       {"djvu", "djv"},
	magickRoute:     {"psd", "psb", "ora", "raw", "cr2", "nef", "nrw", "arw", "srf", "sr2", "dng", "orf", "rw2", "pef", "srw", "raf"},
	rasterRoute:     {"kra", "cbz", "cbt"},
}

// routeFor returns the route of kind.
func routeFor(kind string) *route {
	for r, kinds := range routeKinds {
		for _, k := range kinds {
			if k == kind {
				return r
			}
		}
	}
	return noRoute
}

// calibre styles: the page and the font of ivy for each kind, from
// internal/ebook and text.
var (
	// bookKinds take the page of an ebook: 6 by 9 inches, margins of 0.6
	// inch at the top and the bottom and 0.5 inch at the sides, and a serif
	// font of 11 points.
	bookKinds = []string{"epub", "kepub", "mobi", "prc", "azw", "azw3", "azw4", "fb2", "fbz", "htmlz", "txtz"}
	// textKinds take a page of US Letter, margins of 0.75 inch, and a
	// monospace font of 10 points.
	textKinds = []string{"txt", "text"}
)

// markdownExtensions are the extensions of Python-Markdown that calibre
// turns on for a Markdown file. Without them, calibre shows a block of
// fenced code as text with its backticks.
const markdownExtensions = "footnotes,tables,toc,fenced_code,def_list,meta,sane_lists"

// calibreOptions returns the options of ebook-convert that give the page
// and the font of ivy for kind, on the paper p. The zero p is the default
// paper of the kind. The other kinds take a page of US Letter, margins of
// 0.75 inch and a font of 12 points.
func calibreOptions(kind string, p ivy.Paper) []string {
	opts := []string{
		"--margin-left", "0", "--margin-right", "0", "--margin-top", "0", "--margin-bottom", "0",
		"--disable-font-rescaling", "--minimum-line-height", "0",
		"--chapter-mark", "none", "--page-breaks-before", "/",
		"--pdf-serif-family", "Liberation Serif", "--pdf-sans-family", "Liberation Sans",
		"--pdf-mono-family", "Liberation Mono", "--embed-all-fonts",
	}
	margins := func(tb, lr string) []string {
		return []string{"--pdf-page-margin-top", tb, "--pdf-page-margin-bottom", tb,
			"--pdf-page-margin-left", lr, "--pdf-page-margin-right", lr}
	}
	// size returns the options of the size of the page: def for the zero
	// p, or else the size of p in points
	size := func(def ...string) []string {
		if p.Width <= 0 || p.Height <= 0 {
			return def
		}
		return []string{"--custom-size", points(p.Width) + "x" + points(p.Height), "--unit", "point"}
	}
	switch {
	case contains(bookKinds, kind):
		// ivy keeps the aspect ratio of a cover, and calibre stretches it to
		// the page without this option
		opts = append(opts, size("--custom-size", "6x9", "--unit", "inch")...)
		opts = append(opts, "--pdf-default-font-size", "15", "--pdf-standard-font", "serif", "--preserve-cover-aspect-ratio")
		opts = append(opts, margins("43.2", "36")...)
	case contains(textKinds, kind):
		opts = append(opts, size("--paper-size", "letter")...)
		opts = append(opts, "--pdf-default-font-size", "13", "--pdf-standard-font", "mono", "--formatting-type", "plain")
		opts = append(opts, margins("54", "54")...)
	default:
		opts = append(opts, size("--paper-size", "letter")...)
		opts = append(opts, "--pdf-default-font-size", "16")
		if kind == "md" || kind == "markdown" {
			opts = append(opts, "--formatting-type", "markdown", "--pdf-standard-font", "sans",
				"--markdown-extensions", markdownExtensions)
		} else {
			opts = append(opts, "--pdf-standard-font", "serif")
		}
		opts = append(opts, margins("54", "54")...)
	}
	return opts
}

// contains reports whether list holds s.
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// toolKey returns a short key of the installed program name, from the size
// and the time of its file, so that the cache of the references changes
// when the program changes.
func (e *refEnv) toolKey(name string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if k, ok := e.keys[name]; ok {
		return k, nil
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("the program %s is not installed: %w", name, err)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("reading the program %s: %w", name, err)
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "%s %d %d", path, st.Size(), st.ModTime().UnixNano()))
	k := hex.EncodeToString(sum[:])[:10]
	e.keys[name] = k
	return k, nil
}

// toolsKey returns the key of the programs, joined.
func (e *refEnv) toolsKey(names []string) (string, error) {
	if len(names) == 0 {
		return "0", nil
	}
	keys := make([]string, 0, len(names))
	for _, n := range names {
		k, err := e.toolKey(n)
		if err != nil {
			return "", err
		}
		keys = append(keys, k)
	}
	return strings.Join(keys, "."), nil
}

// versionArgs are the arguments that make each program print its version.
var versionArgs = map[string][]string{
	"mutool": {"-v"}, "pdftoppm": {"-v"}, "pdftocairo": {"-v"}, "pdfinfo": {"-v"},
	"gs": {"--version"}, "soffice": {"--version"}, "ebook-convert": {"--version"}, "weasyprint": {"--version"},
	"inkscape": {"--version"}, "magick": {"-version"}, "xpstopng": {"--version"},
	"djvused": {"--help"},
}

// version returns the first line of the version of the program name that
// holds a digit, or an empty string.
func (e *refEnv) version(ctx context.Context, name string) string {
	e.mu.Lock()
	v, ok := e.versions[name]
	e.mu.Unlock()
	if ok {
		return v
	}
	args, ok := versionArgs[name]
	if ok {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		out, _ := runTool(ctx, e.env, "", name, args...)
		cancel()
		for line := range strings.SplitSeq(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.ContainsAny(line, "0123456789") && !strings.HasPrefix(line, "No write access") {
				v = line
				break
			}
		}
	}
	e.mu.Lock()
	e.versions[name] = v
	e.mu.Unlock()
	return v
}

// maxToolOutput bounds the output of a program that an error keeps.
const maxToolOutput = 4000

// runTool runs the program name with args in the environment env, in the
// folder dir. The end of ctx kills the program and its children. It
// returns the output. An error holds the end of the output.
func runTool(ctx context.Context, env []string, dir, name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("the program %s is not installed: %w", name, err)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env, cmd.Dir = env, dir
	killGroup(cmd)
	cmd.WaitDelay = 5 * time.Second
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	killRest(cmd)
	text := out.String()
	if err != nil {
		if ctx.Err() != nil {
			return text, &toolError{fmt.Errorf("%s did not finish in time: %w", name, ctx.Err())}
		}
		tail := strings.TrimSpace(text)
		if len(tail) > maxToolOutput {
			tail = "..." + tail[len(tail)-maxToolOutput:]
		}
		return text, &toolError{fmt.Errorf("running %s: %w: %s", name, err, tail)}
	}
	return text, nil
}

// passwordArgs returns the arguments of poppler for the password of j.
func (j *refJob) passwordArgs() []string {
	if j.password == "" {
		return nil
	}
	return []string{"-opw", j.password, "-upw", j.password}
}

// pdfCount returns the number of pages of the PDF file in, from pdfinfo.
func pdfCount(ctx context.Context, j *refJob, in string) (int, error) {
	// the key lists pdfinfo among the programs of the report
	_, _ = j.toolKey("pdfinfo")
	args := append(j.passwordArgs(), in)
	out, err := runTool(ctx, j.env, "", "pdfinfo", args...)
	if err != nil {
		return 0, err
	}
	for line := range strings.SplitSeq(out, "\n") {
		if rest, ok := strings.CutPrefix(line, "Pages:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(rest))
			if err != nil {
				return 0, fmt.Errorf("reading the pages of pdfinfo: %w", err)
			}
			return n, nil
		}
	}
	return 0, &toolError{errors.New("pdfinfo gives no page count")}
}

// pdfBoxesDiffer reports whether the MediaBox of page n, from 1, of the PDF
// file in differs from its CropBox, from pdfinfo.
func pdfBoxesDiffer(ctx context.Context, j *refJob, in string, n int) (bool, error) {
	page := strconv.Itoa(n)
	args := slices.Concat([]string{"-box", "-f", page, "-l", page}, j.passwordArgs(), []string{in})
	out, err := runTool(ctx, j.env, "", "pdfinfo", args...)
	if err != nil {
		return false, err
	}
	boxes := map[string][]float64{}
	for line := range strings.SplitSeq(out, "\n") {
		f := strings.Fields(line)
		// Page 1 MediaBox: 0.00 0.00 612.00 792.00
		if len(f) != 7 || f[0] != "Page" || f[1] != page || !strings.HasSuffix(f[2], "Box:") {
			continue
		}
		var box []float64
		for _, v := range f[3:] {
			x, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return false, fmt.Errorf("reading the boxes of pdfinfo: %w", err)
			}
			box = append(box, x)
		}
		boxes[strings.TrimSuffix(f[2], ":")] = box
	}
	media, crop := boxes["MediaBox"], boxes["CropBox"]
	if media == nil || crop == nil {
		return false, &toolError{fmt.Errorf("pdfinfo gives no MediaBox or no CropBox for page %d", n)}
	}
	for i := range media {
		if math.Abs(media[i]-crop[i]) > 0.01 {
			return true, nil
		}
	}
	return false, nil
}

// djvuCount returns the number of pages of the DjVu file in.
func djvuCount(ctx context.Context, j *refJob, in string) (int, error) {
	_, _ = j.toolKey("djvused")
	out, err := runTool(ctx, j.env, "", "djvused", "-e", "n", in)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("reading the pages of djvused: %w", err)
	}
	return n, nil
}

// one returns 1, the number of pages of a picture.
func one(context.Context, *refJob, string) (int, error) {
	return 1, nil
}

// countPages returns the number of pages that ghostscript drew into the
// folder in.
func countPages(_ context.Context, _ *refJob, in string) (int, error) {
	names, err := filepath.Glob(filepath.Join(in, "p-*.png"))
	if err != nil {
		return 0, fmt.Errorf("listing the pages: %w", err)
	}
	return len(names), nil
}

// mutoolDraw draws page n of the PDF file in with MuPDF, with the options
// opts more.
func mutoolDraw(ctx context.Context, j *refJob, opts []string, in string, n int, out string) error {
	args := slices.Concat([]string{"draw", "-q", "-r", strconv.Itoa(j.dpi), "-A", "8"}, opts, []string{"-o", out})
	if j.password != "" {
		args = append(args, "-p", j.password)
	}
	args = append(args, in, strconv.Itoa(n))
	_, err := runTool(ctx, j.env, "", "mutool", args...)
	return err
}

// popplerDraw draws page n of the PDF file in with pdftoppm or pdftocairo,
// with the options aa of the anti-aliasing.
func popplerDraw(ctx context.Context, j *refJob, name string, aa []string, in string, n int, out string) error {
	prefix := strings.TrimSuffix(out, ".png")
	args := slices.Concat(
		[]string{"-f", strconv.Itoa(n), "-l", strconv.Itoa(n), "-singlefile", "-r", strconv.Itoa(j.dpi), "-png"},
		aa, j.passwordArgs(), []string{in, prefix})
	_, err := runTool(ctx, j.env, "", name, args...)
	return err
}

// xpstopng draws page n of the XPS file in with libgxps.
func xpstopng(ctx context.Context, j *refJob, in string, n int, out string) error {
	dir, err := os.MkdirTemp(j.tmp, "xps-")
	if err != nil {
		return fmt.Errorf("making a folder: %w", err)
	}
	prefix := filepath.Join(dir, "page")
	if _, err := runTool(ctx, j.env, "", "xpstopng", "-f", strconv.Itoa(n), "-l", strconv.Itoa(n),
		"-r", strconv.Itoa(j.dpi), in, prefix); err != nil {
		return err
	}
	names, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return fmt.Errorf("listing the images of xpstopng: %w", err)
	}
	if len(names) != 1 {
		return &toolError{fmt.Errorf("xpstopng wrote %d images for page %d", len(names), n)}
	}
	return linkOrCopy(names[0], out)
}

// ghostscript draws each page of the PostScript file doc into dir, as
// p-0001.png and on. An EPS file takes the size of its bounding box.
func ghostscript(ctx context.Context, j *refJob, doc, dir string) (string, error) {
	args := []string{"-dSAFER", "-dBATCH", "-dNOPAUSE", "-q", "-sDEVICE=png16m",
		"-r" + strconv.Itoa(j.dpi), "-dTextAlphaBits=4", "-dGraphicsAlphaBits=4"}
	if j.kind != "ps" {
		args = append(args, "-dEPSCrop")
	}
	args = append(args, "-sOutputFile="+filepath.Join(dir, "p-%04d.png"), doc)
	if _, err := runTool(ctx, j.env, "", "gs", args...); err != nil {
		return "", err
	}
	return ".", nil
}

// pickPage puts page n that ghostscript drew into the folder in at out.
func pickPage(_ context.Context, _ *refJob, in string, n int, out string) error {
	src := filepath.Join(in, fmt.Sprintf("p-%04d.png", n))
	if _, err := os.Stat(src); err != nil {
		return &toolError{fmt.Errorf("ghostscript drew no page %d", n)}
	}
	return linkOrCopy(src, out)
}

// inkscape draws the metafile in with Inkscape.
func inkscape(ctx context.Context, j *refJob, in string, n int, out string) error {
	if n != 1 {
		return &toolError{fmt.Errorf("a metafile has no page %d", n)}
	}
	_, err := runTool(ctx, j.env, "", "inkscape", "--export-type=png", "--export-dpi="+strconv.Itoa(j.dpi),
		"--export-background=white", "--export-background-opacity=1", "-o", out, in)
	return err
}

// ddjvu draws page n of the DjVu file in with DjVuLibre.
func ddjvu(ctx context.Context, j *refJob, in string, n int, out string) error {
	ppm := strings.TrimSuffix(out, ".png") + ".ppm"
	defer func() { _ = os.Remove(ppm) }()
	if _, err := runTool(ctx, j.env, "", "ddjvu", "-format=ppm", "-page="+strconv.Itoa(n),
		"-scale="+strconv.Itoa(j.dpi), in, ppm); err != nil {
		return err
	}
	img, err := readImage(ppm)
	if err != nil {
		return err
	}
	return writePNG(out, img)
}

// magick draws the first image of in with ImageMagick, over white. The image
// turns into sRGB before it goes over white: in CMYK, the color white is the
// full ink of cyan, magenta and yellow, which is almost black.
func magick(ctx context.Context, j *refJob, in string, n int, out string) error {
	if n != 1 {
		return &toolError{fmt.Errorf("an image has no page %d", n)}
	}
	_, err := runTool(ctx, j.env, "", "magick", in+"[0]", "-colorspace", "sRGB", "-background", "white", "-alpha", "remove", "-alpha", "off", "png24:"+out)
	return err
}

// sofficeProfile returns the profile of LibreOffice of the worker of j,
// without the lock that a killed soffice leaves.
func sofficeProfile(j *refJob) string {
	profile := filepath.Join(j.cache, "profiles", fmt.Sprintf("soffice-%d", j.slot))
	_ = os.Remove(filepath.Join(profile, ".lock"))
	_ = os.Remove(filepath.Join(profile, "user", ".lock"))
	return profile
}

// sofficePDF converts doc to PDF with LibreOffice, in the profile of the
// worker. A file with a password, and a CSV file, which needs the page of
// ivy, go through sofficeUNO.
func sofficePDF(ctx context.Context, j *refJob, doc, dir string) (string, error) {
	if j.password != "" || j.unoPage() != "" {
		return sofficeUNO(ctx, j, doc, dir)
	}
	profile := sofficeProfile(j)
	outdir, err := os.MkdirTemp(j.tmp, "soffice-")
	if err != nil {
		return "", fmt.Errorf("making a folder: %w", err)
	}
	env := slices.Concat(j.env, []string{"SAL_USE_VCLPLUGIN=svp"})
	_, err = runTool(ctx, env, "", "soffice", "--headless", "--norestore", "--nologo", "--nodefault",
		"--nolockcheck", "-env:UserInstallation=file://"+profile,
		"--convert-to", "pdf", "--outdir", outdir, doc)
	pdf := filepath.Join(outdir, strings.TrimSuffix(filepath.Base(doc), filepath.Ext(doc))+".pdf")
	if err != nil {
		// LibreOffice 26.8 can crash as it ends, after it wrote the whole PDF
		// file, as with some files of Excel 97. pdfinfo then reads the file.
		if ctx.Err() != nil || !fileExists(pdf) {
			return "", err
		}
		if _, cerr := pdfCount(ctx, j, pdf); cerr != nil {
			return "", err
		}
	}
	if _, err := os.Stat(pdf); err != nil {
		return "", &toolError{errors.New("soffice wrote no PDF file")}
	}
	if err := moveFile(pdf, filepath.Join(dir, "out.pdf")); err != nil {
		return "", fmt.Errorf("moving the PDF file: %w", err)
	}
	return "out.pdf", nil
}

// sofficeUNO converts doc, a file with a password or a CSV file, to PDF
// with LibreOffice. The command line of soffice cannot take a password or a
// page, so the script unopdf.py starts soffice in the profile of the
// worker, opens doc through the UNO API with the password, sets the page of
// a CSV file, and stores it as PDF.
func sofficeUNO(ctx context.Context, j *refJob, doc, dir string) (string, error) {
	py, err := j.unoPython(ctx)
	if err != nil {
		return "", err
	}
	profile := sofficeProfile(j)
	soffice, err := exec.LookPath("soffice")
	if err != nil {
		return "", fmt.Errorf("the program soffice is not installed: %w", err)
	}
	pipe := fmt.Sprintf("ivy-review-%d-%d-%d", os.Getpid(), j.slot, j.pipes.Add(1))
	env := slices.Concat(j.env, py.env, []string{"SAL_USE_VCLPLUGIN=svp", "IVY_REVIEW_PASSWORD=" + j.password})
	if page := j.unoPage(); page != "" {
		env = append(env, "IVY_REVIEW_PAGE="+page, "IVY_REVIEW_CSV="+csvFilterOptions(j.kind))
	}
	out := filepath.Join(dir, "out.pdf")
	if _, err := runTool(ctx, env, j.tmp, py.path, j.unoScript, soffice, "file://"+profile, pipe, doc, out); err != nil {
		return "", err
	}
	if _, err := os.Stat(out); err != nil {
		return "", &toolError{errors.New("LibreOffice wrote no PDF file")}
	}
	return "out.pdf", nil
}

// unoPage returns the page that unopdf.py gives each sheet of a CSV file:
// the width, the height, and the margins at the top, the bottom, the left
// and the right, in hundredths of a millimeter, separated by spaces. It is
// the page of ivy: the paper of j, or US Letter, with the margins of Excel
// and no header or footer (D32). It returns an empty string for a kind that
// is not CSV.
func (j *refJob) unoPage() string {
	if !contains(csvKinds, j.kind) {
		return ""
	}
	p := j.paper
	if p.Width <= 0 || p.Height <= 0 {
		p = ivy.Letter
	}
	m := sheet.DefaultPage
	mm := func(pt float64) int { return int(math.Round(pt * 2540 / 72)) }
	return fmt.Sprintf("%d %d %d %d %d %d", mm(float64(p.Width)), mm(float64(p.Height)), mm(m.Top), mm(m.Bottom), mm(m.Left), mm(m.Right))
}

// csvFilterOptions returns the options of the CSV filter of LibreOffice for
// kind: the separator, the quote, the encoding UTF-8 (76) and the first
// line. A .tsv or a .tab file has tabs (9), and a .csv file has commas (44).
func csvFilterOptions(kind string) string {
	if kind == "csv" {
		return "44,34,76,1"
	}
	return "9,34,76,1"
}

// unoPython is a Python that can import the module uno of LibreOffice.
type unoPython struct {
	path string
	// env is what the environment of Python needs more, such as a
	// PYTHONPATH.
	env []string
}

// unoPython finds a Python that can import the module uno, once for the
// run. It tries the Python that LibreOffice bundles in the folder of
// soffice.bin, then python3 of the PATH, then python3 with that folder in
// its PYTHONPATH.
func (e *refEnv) unoPython(ctx context.Context) (unoPython, error) {
	e.unoOnce.Do(func() {
		// the end of the time of one document must not end the search for
		// the whole run
		e.uno, e.unoErr = findUNOPython(context.WithoutCancel(ctx), e.env)
	})
	return e.uno, e.unoErr
}

// findUNOPython returns the first Python that unoPython tries that can
// import uno.
func findUNOPython(ctx context.Context, env []string) (unoPython, error) {
	soffice, err := exec.LookPath("soffice")
	if err != nil {
		return unoPython{}, fmt.Errorf("the program soffice is not installed: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(soffice); err == nil {
		soffice = resolved
	}
	program := filepath.Dir(soffice)
	var tries []unoPython
	if p := filepath.Join(program, "python"); fileExists(p) {
		tries = append(tries, unoPython{path: p})
	}
	if p, err := exec.LookPath("python3"); err == nil {
		tries = append(tries, unoPython{path: p}, unoPython{path: p, env: []string{"PYTHONPATH=" + program}})
	}
	for _, py := range tries {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, err := runTool(ctx, slices.Concat(env, py.env), "", py.path, "-c", "import uno")
		cancel()
		if err == nil {
			return py, nil
		}
	}
	return unoPython{}, fmt.Errorf("no Python can import the module uno of LibreOffice, so a file with a password has no reference: "+
		"install the package of the Python bridge of LibreOffice, such as python3-uno, or give python3 the folder %s", program)
}

// fileExists reports whether the file path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// calibrePDF converts doc to PDF with ebook-convert of calibre, with the
// page and the font of ivy.
func calibrePDF(ctx context.Context, j *refJob, doc, dir string) (string, error) {
	if j.password != "" {
		return "", errors.New("calibre cannot open a file with a password")
	}
	env := slices.Concat(j.env, []string{
		"CALIBRE_CONFIG_DIRECTORY=" + filepath.Join(j.cache, "profiles", fmt.Sprintf("calibre-%d", j.slot)),
		"QT_QPA_PLATFORM=offscreen",
		"QTWEBENGINE_CHROMIUM_FLAGS=--no-sandbox",
	})
	args := append([]string{doc, filepath.Join(dir, "out.pdf")}, calibreOptions(j.kind, j.paper)...)
	if _, err := runTool(ctx, env, "", "ebook-convert", args...); err != nil {
		return "", err
	}
	return "out.pdf", nil
}

// weasyprintPDF lays out the HTML file doc with WeasyPrint into a PDF file.
// It gives WeasyPrint the user style sheet that the package html of ivy
// gives an HTML file on the paper of j, ebook.PaperStyleFor, so that both
// lay out the same input. The links of doc resolve from its folder.
func weasyprintPDF(ctx context.Context, j *refJob, doc, dir string) (string, error) {
	style := filepath.Join(dir, "paper.css")
	if err := os.WriteFile(style, []byte(ebook.PaperStyleFor(j.paper)), 0o644); err != nil {
		return "", fmt.Errorf("writing the style sheet: %w", err)
	}
	if _, err := runTool(ctx, j.env, "", "weasyprint", "--quiet", "-s", style, doc, filepath.Join(dir, "out.pdf")); err != nil {
		return "", err
	}
	return "out.pdf", nil
}

// findWeasyPrint adds the folder bin of a virtual environment of Python in
// cache/weasyprint to the PATH, when the PATH has no weasyprint. README.md
// of cmd/review says how to make that environment.
func findWeasyPrint(cache string) error {
	if _, err := exec.LookPath("weasyprint"); err == nil {
		return nil
	}
	bin, err := filepath.Abs(filepath.Join(cache, "weasyprint", "bin"))
	if err != nil {
		return fmt.Errorf("finding the folder of weasyprint: %w", err)
	}
	switch _, err := os.Stat(filepath.Join(bin, "weasyprint")); {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("finding weasyprint: %w", err)
	}
	if err := os.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return fmt.Errorf("adding weasyprint to the PATH: %w", err)
	}
	return nil
}

// prepared is a reference that is ready to draw pages.
type prepared struct {
	// in is the file that the page renderers read.
	in string
	// dir is the folder of the cache of the pages.
	dir string
	// pages is the number of pages, or -1 when it is not known.
	pages int
}

// errorFile holds the error of a conversion in the cache.
const errorFile = "error.txt"

// prepare runs the conversion of route r on doc, or takes it from the
// cache. A failed conversion is kept in the cache too, because LibreOffice
// and calibre are slow. Delete its folder to try again.
func (j *refJob) prepare(ctx context.Context, r *route, doc string) (*prepared, error) {
	id, tools := r.id, r.tools
	if r.uno && (j.password != "" || j.unoPage() != "") {
		// the conversion through UNO depends on Python too
		py, err := j.unoPython(ctx)
		if err != nil {
			return nil, err
		}
		id, tools = id+"-uno", slices.Concat(tools, []string{py.path})
	}
	key, err := j.toolsKey(tools)
	if err != nil {
		return nil, err
	}
	name := id + "-" + key + "-" + j.fonts
	if r.perDPI {
		name += fmt.Sprintf("-%ddpi", j.dpi)
	}
	if r.variant != nil {
		if v := r.variant(j); v != "" {
			sum := sha256.Sum256([]byte(v))
			name += "-v" + hex.EncodeToString(sum[:4])
		}
	}
	if j.password != "" {
		// a wrong password gives another result
		sum := sha256.Sum256([]byte(j.password))
		name += "-pw" + hex.EncodeToString(sum[:4])
	}
	dir := filepath.Join(j.cache, "refs", j.hash[:16], name)
	p := &prepared{in: doc, dir: dir, pages: -1}
	if r.prepare != nil {
		if msg, err := os.ReadFile(filepath.Join(dir, errorFile)); err == nil && !j.retry {
			j.cachedFailures.Add(1)
			return nil, fmt.Errorf("%s (kept in the cache: run render with -retry, or delete %s, to try again)",
				strings.TrimSpace(string(msg)), dir)
		}
		out, err := os.ReadFile(filepath.Join(dir, "done"))
		if err != nil {
			// a folder without the mark of a result is a failure that -retry
			// tries again, or a folder that a person emptied
			if err := os.RemoveAll(dir); err != nil {
				return nil, fmt.Errorf("removing the folder of the cache: %w", err)
			}
			out, err = j.convert(ctx, r, doc, dir)
			if err != nil {
				return nil, err
			}
		}
		p.in = filepath.Join(dir, strings.TrimSpace(string(out)))
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("making the folder of the cache: %w", err)
	}
	if r.count != nil {
		n, err := r.count(ctx, j, p.in)
		if err != nil {
			return p, fmt.Errorf("counting the pages: %w", err)
		}
		p.pages = n
	}
	return p, nil
}

// convert runs the conversion of route r on doc in a temporary folder, then
// moves the folder to dir. The temporary folder is in the folder of dir, so
// that the move is one rename on one file system. The cache keeps a failure
// only if it is a toolError.
func (j *refJob) convert(ctx context.Context, r *route, doc, dir string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, fmt.Errorf("making the folder of the cache: %w", err)
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), ".tmp-")
	if err != nil {
		return nil, fmt.Errorf("making a folder in the cache: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	out, cerr := r.prepare(ctx, j, doc, tmp)
	if cerr != nil {
		if errors.Is(ctx.Err(), context.Canceled) || !isToolError(cerr) {
			// the run stopped, or the error is not one of the program: keep
			// nothing
			return nil, cerr
		}
		if err := os.WriteFile(filepath.Join(tmp, errorFile), []byte(cerr.Error()+"\n"), 0o644); err != nil {
			return nil, fmt.Errorf("keeping the error: %w", err)
		}
	} else if err := os.WriteFile(filepath.Join(tmp, "done"), []byte(out+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("marking the conversion: %w", err)
	}
	if err := os.Rename(tmp, dir); err != nil {
		// another run can put the same conversion into the cache first
		if !fileExists(filepath.Join(dir, "done")) && !fileExists(filepath.Join(dir, errorFile)) {
			return nil, fmt.Errorf("moving the conversion into the cache: %w", err)
		}
	}
	if cerr != nil {
		return nil, cerr
	}
	return []byte(out), nil
}

// renderPage returns the path of the image of page n, from 1, that pr draws
// from p, from the cache when it holds it. The cache keeps a failure that is
// a toolError and not the end of ctx.
func (j *refJob) renderPage(ctx context.Context, p *prepared, pr pageRenderer, native bool, n int) (string, error) {
	key, err := j.toolsKey(pr.tools)
	if err != nil {
		return "", err
	}
	dpi := strconv.Itoa(j.dpi)
	if native {
		dpi = "native"
	}
	base := filepath.Join(p.dir, fmt.Sprintf("%s%s-%s-%s-p%d", pr.id, pr.tag, key, dpi, n))
	png := base + ".png"
	if _, err := os.Stat(png); err == nil {
		return png, nil
	}
	if msg, err := os.ReadFile(base + ".err"); err == nil {
		if !j.retry {
			j.cachedFailures.Add(1)
			return "", fmt.Errorf("%s (kept in the cache: run render with -retry to try again)", strings.TrimSpace(string(msg)))
		}
		if err := os.Remove(base + ".err"); err != nil {
			return "", fmt.Errorf("removing the failure from the cache: %w", err)
		}
	}
	tmp := base + ".tmp.png"
	err = pr.render(ctx, j, p.in, n, tmp)
	if err == nil {
		if _, serr := os.Stat(tmp); serr != nil {
			err = &toolError{fmt.Errorf("%s wrote no image", pr.name)}
		}
	}
	if err != nil {
		_ = os.Remove(tmp)
		if ctx.Err() == nil && isToolError(err) {
			_ = os.WriteFile(base+".err", []byte(err.Error()+"\n"), 0o644)
		}
		return "", err
	}
	if err := os.Rename(tmp, png); err != nil {
		return "", fmt.Errorf("moving the image into the cache: %w", err)
	}
	return png, nil
}

// used returns the programs that the run used, sorted.
func (e *refEnv) used() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	names := make([]string, 0, len(e.keys))
	for n := range e.keys {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
