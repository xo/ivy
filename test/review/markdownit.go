package review

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cdppage "github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
)

// markdownItManifest lists the files of the page of markdown-it (D70): the
// modules of markdown-it, of its plugins, of Shiki and of KaTeX, from
// jsDelivr, and the style sheets and the fonts. fetch downloads them into
// the folder markdownit of the cache. The name of a file is its path on
// jsDelivr, and the page server serves it at that path.
//
//go:embed markdownit.tsv
var markdownItManifest []byte

// markdownItPage is the page that renders a Markdown file with markdown-it.
// {{PAPER}} takes the style sheet of the paper, and {{SOURCE}} the text of
// the file as a string of JSON.
//
//go:embed markdownit.html
var markdownItPage string

// markdownItFolder is the folder of the cache of the files of the page.
const markdownItFolder = "markdownit"

// markdownItRoute renders a Markdown file with markdown-it in Chrome, with
// the style sheet of GitHub, and prints it to PDF on the paper and with the
// margins of ivy (D70).
var markdownItRoute = &route{
	id: "markdownit", name: "markdown-it", tools: []string{chromeProgram},
	prepare: markdownItPDF, count: pdfCount, renderers: pdfRenderers, ivyPDF: true,
	variant: func(j *refJob) string {
		return markdownItPaper(j.paper) + "\x00" + markdownItPage + "\x00" + string(markdownItManifest)
	},
}

// markdownItPaper returns the style sheet of the paper p of a Markdown
// file: the one that the package markdown of ivy gives its HTML, with the
// size of the page and margins of 0.75 inch.
func markdownItPaper(p ivy.Paper) string {
	return ebook.PaperStyleFor(p)
}

// MarkdownItEntries returns the files of the page of markdown-it, as the
// manifest markdownit.tsv lists them.
func MarkdownItEntries() ([]Entry, error) {
	entries, err := ReadManifest(bytes.NewReader(markdownItManifest))
	if err != nil {
		return nil, fmt.Errorf("reading markdownit.tsv: %w", err)
	}
	return entries, nil
}

// FetchMarkdownIt downloads the files of the page of markdown-it that are
// not in the folder markdownit of the cache of opts, as Fetch does.
func FetchMarkdownIt(ctx context.Context, opts FetchOptions) error {
	entries, err := MarkdownItEntries()
	if err != nil {
		return err
	}
	opts.Cache = filepath.Join(opts.Cache, markdownItFolder)
	return Fetch(ctx, entries, opts)
}

// pageServer serves the page of markdown-it to Chrome, on a port of the
// loopback interface. It serves the files of markdownit.tsv at their names,
// and each page at /page/ID/, with the files of the folder of its Markdown
// file under it.
type pageServer struct {
	once sync.Once
	err  error
	// url is the URL of the server, such as http://127.0.0.1:1234.
	url string
	srv *http.Server
	// files maps the name of each file of markdownit.tsv to its path.
	files map[string]string

	mu    sync.Mutex
	pages map[string]servedPage
}

// servedPage is a page of a Markdown file, and the folder of the file.
type servedPage struct {
	html []byte
	root *os.Root
}

// pages returns the page server of the run. It starts the server the first
// time, after it makes sure that the cache holds each file of the page.
func (e *refEnv) pages() (*pageServer, error) {
	s := &e.pageSrv
	s.once.Do(func() {
		s.err = s.start(filepath.Join(e.cache, markdownItFolder))
	})
	return s, s.err
}

// start checks the files of the page in the folder dir of the cache, and
// starts the server.
func (s *pageServer) start(dir string) error {
	entries, err := MarkdownItEntries()
	if err != nil {
		return err
	}
	s.files = map[string]string{}
	for _, e := range entries {
		p := e.Path("", dir)
		if err := checkFile(p, e); err != nil {
			return fmt.Errorf("the files of markdown-it are not in the cache, run review fetch: %s: %w", e.Name, err)
		}
		s.files[e.Name] = p
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("starting the server of the page of markdown-it: %w", err)
	}
	s.url = "http://" + ln.Addr().String()
	s.pages = map[string]servedPage{}
	s.srv = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = s.srv.Serve(ln) }()
	return nil
}

// close stops the server, if it started.
func (s *pageServer) close() {
	if s.srv != nil {
		_ = s.srv.Close()
	}
}

// add serves the page html at a new URL, with the files of the folder dir
// under it. It returns the URL and the function that stops serving them.
func (s *pageServer) add(html []byte, dir string) (string, func(), error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", nil, fmt.Errorf("opening the folder of the Markdown file: %w", err)
	}
	id := hex.EncodeToString(randomBytes(8))
	s.mu.Lock()
	s.pages[id] = servedPage{html: html, root: root}
	s.mu.Unlock()
	remove := func() {
		s.mu.Lock()
		delete(s.pages, id)
		s.mu.Unlock()
		_ = root.Close()
	}
	return s.url + "/page/" + id + "/", remove, nil
}

// randomBytes returns n random bytes.
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

// pageTypes are the types of the files of the page and of the folder of a
// Markdown file, by their extension. A module of jsDelivr has no
// extension, so a file without one of these is JavaScript.
var pageTypes = map[string]string{
	".css": "text/css", ".woff2": "font/woff2", ".woff": "font/woff", ".ttf": "font/ttf",
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".svg": "image/svg+xml", ".webp": "image/webp", ".avif": "image/avif", ".bmp": "image/bmp",
}

// ServeHTTP serves a file of the page, a page, or a file of the folder of a
// page.
func (s *pageServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if p, ok := s.files[name]; ok {
		typ, ok := pageTypes[path.Ext(name)]
		if !ok {
			typ = "text/javascript; charset=utf-8"
		}
		w.Header().Set("Content-Type", typ)
		http.ServeFile(w, r, p)
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/page/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	id, file, _ := strings.Cut(rest, "/")
	s.mu.Lock()
	page, ok := s.pages[id]
	s.mu.Unlock()
	switch {
	case !ok:
		http.NotFound(w, r)
	case file == "":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page.html)
	default:
		s.serveFile(w, r, page.root, file)
	}
}

// serveFile serves the file name of the folder root.
func (s *pageServer) serveFile(w http.ResponseWriter, r *http.Request, root *os.Root, name string) {
	name = path.Clean(name)
	f, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	if typ, ok := pageTypes[strings.ToLower(path.Ext(name))]; ok {
		w.Header().Set("Content-Type", typ)
	}
	http.ServeContent(w, r, name, st.ModTime(), f)
}

// markdownItHTML returns the page of the Markdown text src on the paper p.
func markdownItHTML(src []byte, p ivy.Paper) ([]byte, error) {
	// json.Marshal writes < as <, so the text cannot end the element
	// script
	text, err := json.Marshal(string(src))
	if err != nil {
		return nil, fmt.Errorf("encoding the Markdown text: %w", err)
	}
	r := strings.NewReplacer("{{PAPER}}", markdownItPaper(p), "{{SOURCE}}", string(text))
	return []byte(r.Replace(markdownItPage)), nil
}

// markdownItPDF renders the Markdown file doc with markdown-it in a tab of
// Chrome, waits until the page ends, and prints the page to PDF into the
// folder dir. The style sheet of the paper of ivy sets the size and the
// margins of the pages with @page.
func markdownItPDF(ctx context.Context, j *refJob, doc, dir string) (string, error) {
	srv, err := j.pages()
	if err != nil {
		return "", err
	}
	src, err := os.ReadFile(doc)
	if err != nil {
		return "", fmt.Errorf("reading the Markdown file: %w", err)
	}
	page, err := markdownItHTML(src, j.paper)
	if err != nil {
		return "", err
	}
	u, remove, err := srv.add(page, filepath.Dir(doc))
	if err != nil {
		return "", err
	}
	defer remove()
	bctx, err := j.browser()
	if err != nil {
		return "", err
	}
	tab, cancel := chromedp.NewContext(bctx)
	defer cancel()
	if d, ok := ctx.Deadline(); ok {
		var cancelTime context.CancelFunc
		tab, cancelTime = context.WithDeadline(tab, d)
		defer cancelTime()
	}
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	data, err := markdownItPrint(tab, u)
	if err != nil {
		if ctx.Err() != nil {
			return "", &toolError{fmt.Errorf("markdown-it did not finish in time: %w", ctx.Err())}
		}
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "out.pdf"), data, 0o644); err != nil {
		return "", fmt.Errorf("writing the PDF file: %w", err)
	}
	return "out.pdf", nil
}

// markdownItPrint opens the page at u in the tab of ctx, waits until the
// page says that it is done, and returns the page printed to PDF.
func markdownItPrint(ctx context.Context, u string) ([]byte, error) {
	if err := chromedp.Do(ctx, chromedp.Navigate(u)); err != nil {
		return nil, fmt.Errorf("opening the page of markdown-it in Chrome: %w", err)
	}
	for {
		state, err := chromedp.Run(ctx, chromedp.Evaluate[string](`window.ivyState || ""`))
		if err != nil {
			return nil, fmt.Errorf("reading the state of the page of markdown-it: %w", err)
		}
		if msg, ok := strings.CutPrefix(state, "error: "); ok {
			return nil, &toolError{fmt.Errorf("markdown-it: %s", msg)}
		}
		if state == "done" {
			break
		}
		if err := sleep(ctx, 100*time.Millisecond); err != nil {
			return nil, err
		}
	}
	yes := true
	res, err := chromedp.Call(ctx, cdppage.PrintToPDF, cdppage.PrintToPDFParams{PreferCSSPageSize: &yes, PrintBackground: &yes})
	if err != nil {
		return nil, fmt.Errorf("printing the page of markdown-it to PDF: %w", err)
	}
	if len(res.Data) == 0 {
		return nil, &toolError{errors.New("chrome printed an empty PDF file")}
	}
	return res.Data, nil
}
