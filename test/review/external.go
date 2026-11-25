package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// external is a reference that a program outside this machine makes, such
// as Microsoft Office on Windows. A maintainer runs the program, and copies
// its files into a folder of the cache, at the name of the document in the
// manifest with an extension more, such as
// cache/msoffice/poi/docx/drawing.docx.pdf, or with one PNG image for each
// page, such as cache/windows/xps/microsoft/xps/mb01.xps.p1.png. When the
// file of a document exists, or the image of its first page, the document
// gets the reference.
type external struct {
	id   string
	name string
	// dir is the folder of the files in the cache, with slashes.
	dir string
	// ext is the extension that the files add to the name of the document:
	// ".pdf" or ".png". A reference of pages adds ".p<N>" before it, with N
	// from 1.
	ext string
	// paged makes the reference one PNG image for each page.
	paged bool
	// kinds are the kinds that the reference draws.
	kinds []string
	// primary are the kinds for which the reference comes before the
	// reference of the route: the worst number of a document uses it first.
	// Without -refs all, a document of another kind does not show the
	// reference (D69).
	primary []string
	// keepRoute keeps the route of the kind as the secondary oracle of a
	// document that has the file of the reference. Without it, such a
	// document shows the reference alone, unless -refs all is given (D69).
	keepRoute bool
	// renderers draw the pages of the file.
	renderers []pageRenderer
	// dpi is the resolution of the images of a reference of PNG files. The
	// review uses them only at this resolution.
	dpi int
}

// officeKinds are the kinds of Microsoft Office, for which Office is the
// primary reference: Word, Excel, PowerPoint, Visio and Publisher, and RTF.
var officeKinds = []string{
	"docx", "dotx", "docm", "dotm", "doc", "dot", "rtf",
	"xlsx", "xltx", "xlsm", "xltm", "xls", "xlt",
	"pptx", "potx", "ppsx", "pptm", "ppsm", "ppt", "pps", "pot",
	"vsdx", "vsdm", "vstx", "vstm", "vsd", "vst",
	"pub",
}

// pngRenderer gives the PNG file of a reference of PNG files as its page.
var pngRenderer = pageRenderer{id: "png", name: "PNG",
	render: func(_ context.Context, _ *refJob, in string, n int, out string) error {
		if n != 1 {
			return &toolError{fmt.Errorf("a picture has no page %d", n)}
		}
		return linkOrCopy(in, out)
	}}

// pagePNGRenderer gives the PNG image of page n of a reference of pages:
// the base name in with ".p<n>.png".
var pagePNGRenderer = pageRenderer{id: "png", name: "PNG",
	render: func(_ context.Context, _ *refJob, in string, n int, out string) error {
		src := pagePNG(in, n)
		if !fileExists(src) {
			return &toolError{fmt.Errorf("the reference has no image of page %d", n)}
		}
		return linkOrCopy(src, out)
	}}

// pagePNG returns the image of page n, from 1, of a reference of pages with
// the base name base.
func pagePNG(base string, n int) string {
	return fmt.Sprintf("%s.p%d.png", base, n)
}

// externals are the references that programs outside this machine make.
// README.md of cmd/review says how to make their files.
var externals = []external{
	{
		id: "msoffice", name: "Microsoft Office", dir: "msoffice", ext: ".pdf",
		// Office also opens OpenDocument and CSV, but LibreOffice stays the
		// primary reference of them
		kinds:     append([]string{"odt", "ott", "ods", "ots", "odp", "otp", "csv"}, officeKinds...),
		primary:   officeKinds,
		keepRoute: true,
		renderers: []pageRenderer{mupdfRenderer, splashRenderer},
	},
	{
		// WPF draws each page, the engine of the XPS viewer of Windows
		id: "winxps", name: "Windows XPS", dir: "windows/xps", ext: ".png", paged: true,
		kinds: []string{"xps", "oxps"}, primary: []string{"xps", "oxps"}, keepRoute: true,
		renderers: []pageRenderer{{id: "wpf", name: "WPF", render: pagePNGRenderer.render}},
		dpi:       96,
	},
	{
		id: "gdiplus", name: "GDI+", dir: "windows/gdiplus", ext: ".png",
		kinds: []string{"emf", "wmf", "emz", "wmz"}, primary: []string{"emf", "wmf", "emz", "wmz"}, keepRoute: true,
		renderers: []pageRenderer{{id: "gdiplus", name: "GDI+", render: pngRenderer.render}},
		dpi:       96,
	},
	{
		id: "iwork", name: "Apple iWork", dir: "iwork", ext: ".pdf",
		kinds: []string{"pages", "key", "numbers"}, primary: []string{"pages", "key", "numbers"},
		renderers: []pageRenderer{mupdfRenderer},
	},
}

// file returns the path of the file of the document e in the cache. For a
// reference of pages, it returns the base name of the images, without
// ".p<N>.png".
func (x external) file(cache string, e Entry) string {
	base := filepath.Join(cache, filepath.FromSlash(x.dir), filepath.FromSlash(e.Name))
	if x.paged {
		return base
	}
	return base + x.ext
}

// display returns the file of the document e, relative to the cache, with
// slashes, for the report.
func (x external) display(e Entry) string {
	if x.paged {
		return x.dir + "/" + e.Name + ".p<N>" + x.ext
	}
	return x.dir + "/" + e.Name + x.ext
}

// hashPages returns the sha256 of the images of the pages of the base name
// base, from page 1 to the first page without an image, and the number of
// the pages.
func hashPages(base string) (string, int, error) {
	h := sha256.New()
	n := 0
	for fileExists(pagePNG(base, n+1)) {
		n++
		sum, _, err := hashFile(pagePNG(base, n))
		if err != nil {
			return "", n, err
		}
		h.Write([]byte(sum))
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// refGroup is one reference of a document: a conversion, such as
// LibreOffice to PDF, or the file of an external reference, and the page
// renderers that draw it.
type refGroup struct {
	id   string
	name string
	job  *refJob
	prep *prepared
	// pages is the number of pages, or -1 when it is not known.
	pages     int
	renderers []pageRenderer
	native    bool
	// skip is the error of each cell of the group, such as "no reference",
	// or empty.
	skip string
	// files starts the names of the images of the group, such as
	// "msoffice-". It is empty for the route, whose images keep their names.
	files string
	// dpi is the resolution that the group needs, or 0 for any.
	dpi int
}

// prepareExternal returns the group of the external reference x for the
// document e, or nil when its file is not in the cache. The pages of the
// file go into the folder of the cache of the document, with a hash of the
// file in their names, so a new file of the program draws them again.
func (j *refJob) prepareExternal(ctx context.Context, x external, e Entry) (*refGroup, error) {
	path := x.file(j.cache, e)
	probe := path
	if x.paged {
		probe = pagePNG(path, 1)
	}
	if _, err := os.Stat(probe); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil //nolint:nilnil // the document has no such reference
		}
		return nil, fmt.Errorf("reading the file of %s: %w", x.name, err)
	}
	g := &refGroup{id: x.id, name: x.name, renderers: x.renderers, pages: -1, files: x.id + "-", dpi: x.dpi}
	// the file of the program has no password, and its page is its own
	g.job = &refJob{refEnv: j.refEnv, kind: j.kind, slot: j.slot, hash: j.hash, tmp: j.tmp}
	var hash string
	pages := 1
	var err error
	if x.paged {
		hash, pages, err = hashPages(path)
	} else {
		hash, _, err = hashFile(path)
	}
	if err != nil {
		return g, err
	}
	g.prep = &prepared{in: path, dir: filepath.Join(j.cache, "refs", j.hash[:16], x.id+"-"+hash[:12]), pages: -1}
	if err := os.MkdirAll(g.prep.dir, 0o755); err != nil {
		return g, fmt.Errorf("making the folder of the cache: %w", err)
	}
	if x.dpi != 0 && x.dpi != j.dpi {
		return g, fmt.Errorf("%s drew the file at %d dpi, and the review runs at %d dpi", x.name, x.dpi, j.dpi)
	}
	if strings.HasSuffix(path, ".pdf") {
		n, err := pdfCount(ctx, g.job, path)
		if err != nil {
			return g, fmt.Errorf("counting the pages of %s: %w", x.name, err)
		}
		pages = n
	}
	g.pages, g.prep.pages = pages, pages
	return g, nil
}
