// Command perf measures the time and the memory that ivy takes on the
// documents of the review corpus.
//
// Usage:
//
//	perf [-cache DIR] [-manifest FILE] [-match REGEXP] [-kind LIST]
//		[-pages N] [-dpi N] [-count N] [-timeout DUR]
//		[-cpuprofile FILE] [-memprofile FILE] [-hash]
//
// perf reads each file of the corpus into memory, and then, for each run,
// opens it, renders its first page, counts its pages and renders its first
// pages. It writes one line for each document, separated by tabs: the
// name, the kind, the size of the file, the number of pages, the time to
// open, the time to the first page (open, Page(0) and its render), the
// time to count the pages, the mean time of a page, the total time, the
// bytes that it allocates and the number of allocations. Each time is the
// best of the runs, in milliseconds. A document without pages decodes its
// best preview as its first page. -hash adds a column with a hash of the
// pixels of the pages of the first run, to compare two builds of ivy.
//
// -cpuprofile and -memprofile write one profile that covers every run of
// every document that perf measures. Use -match to profile one document.
//
// perf finds the repository from the current folder, as review does.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"hash"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"slices"
	"strings"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/test/review"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "perf:", err)
		os.Exit(1)
	}
}

// options are the flags of the command.
type options struct {
	repo     string
	cache    string
	manifest string
	match    *regexp.Regexp
	kinds    []string
	pages    int
	dpi      float32
	count    int
	timeout  time.Duration
	cpu      string
	mem      string
	hash     bool
}

func run(ctx context.Context, args []string) error {
	repo, err := findRepo()
	if err != nil {
		return err
	}
	fl := flag.NewFlagSet("perf", flag.ContinueOnError)
	cache := fl.String("cache", filepath.Join(repo, "test", "review", "cache"), "the cache folder of the files")
	manifest := fl.String("manifest", filepath.Join(repo, "test", "review", "corpus.tsv"), "the manifest of the corpus")
	match := fl.String("match", "", "measure only the documents whose name matches this regular expression")
	kinds := fl.String("kind", "", "measure only the documents of these kinds, separated by commas")
	pages := fl.Int("pages", 3, "the number of first pages to render")
	dpi := fl.Int("dpi", 96, "the resolution in dots per inch")
	count := fl.Int("count", 1, "the number of runs of each document")
	timeout := fl.Duration("timeout", 2*time.Minute, "the bound of one run of one document")
	cpu := fl.String("cpuprofile", "", "write a CPU profile to this file")
	mem := fl.String("memprofile", "", "write a profile of the allocations to this file")
	hash := fl.Bool("hash", false, "write a hash of the pixels of the pages")
	if err := fl.Parse(args); err != nil {
		return fmt.Errorf("reading the flags: %w", err)
	}
	opts := options{
		repo: repo, cache: *cache, manifest: *manifest, pages: *pages,
		dpi: float32(*dpi), count: max(*count, 1), timeout: *timeout, cpu: *cpu, mem: *mem, hash: *hash,
	}
	if *match != "" {
		re, err := regexp.Compile(*match)
		if err != nil {
			return fmt.Errorf("compiling -match: %w", err)
		}
		opts.match = re
	}
	if *kinds != "" {
		opts.kinds = strings.Split(*kinds, ",")
	}
	return measure(ctx, opts)
}

// findRepo returns the root of the repository of ivy, the first folder
// from the current folder up that holds go.mod and the folder test/review.
func findRepo() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting the current folder: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "test", "review", "corpus.tsv")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no repository of ivy above the current folder")
		}
		dir = parent
	}
}

// result is the measure of one document.
type result struct {
	pages  int
	open   time.Duration
	first  time.Duration
	count  time.Duration
	page   time.Duration
	total  time.Duration
	bytes  uint64
	allocs uint64
	hash   string
	err    error
}

// measure measures each document of the manifest that opts selects.
func measure(ctx context.Context, opts options) error {
	entries, err := review.LoadManifest(opts.manifest)
	if err != nil {
		return err
	}
	if opts.cpu != "" {
		f, err := os.Create(opts.cpu)
		if err != nil {
			return fmt.Errorf("creating the CPU profile: %w", err)
		}
		defer func() { _ = f.Close() }()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("starting the CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}
	if opts.mem != "" {
		runtime.MemProfileRate = 64 << 10
	}
	// The folder lm holds the fonts of TeX, which fonts.FS of the module
	// fonts leaves out (D65).
	cfg := ivy.Config{Fonts: withoutTeX{os.DirFS(filepath.Join(opts.repo, "fonts"))}}
	fmt.Println("name\tkind\tsize\tpages\topen_ms\tfirst_ms\tcount_ms\tpage_ms\ttotal_ms\talloc_mb\tallocs\terror\thash")
	for _, e := range entries {
		if e.Kind == "asset" || (opts.match != nil && !opts.match.MatchString(e.Name)) ||
			(len(opts.kinds) > 0 && !slices.Contains(opts.kinds, e.Kind)) {
			continue
		}
		format, ok := review.FormatFor(e.Kind)
		if !ok {
			continue
		}
		path := e.Path(opts.repo, opts.cache)
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "perf: %s: %v\n", e.Name, err)
			continue
		}
		cfg.Password = e.Password()
		var best result
		for i := range opts.count {
			r := measureOne(ctx, opts, format, cfg, filepath.Dir(path), data)
			if i == 0 {
				best = r
				continue
			}
			best.open = min(best.open, r.open)
			best.first = min(best.first, r.first)
			best.count = min(best.count, r.count)
			best.page = min(best.page, r.page)
			best.total = min(best.total, r.total)
		}
		msg := ""
		if best.err != nil {
			msg = strings.ReplaceAll(best.err.Error(), "\n", " ")
			msg = msg[:min(len(msg), 120)]
		}
		fmt.Printf("%s\t%s\t%d\t%d\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%d\t%s\t%s\n",
			e.Name, e.Kind, len(data), best.pages, ms(best.open), ms(best.first), ms(best.count),
			ms(best.page), ms(best.total), float64(best.bytes)/(1<<20), best.allocs, msg, best.hash)
	}
	if opts.mem != "" {
		f, err := os.Create(opts.mem)
		if err != nil {
			return fmt.Errorf("creating the memory profile: %w", err)
		}
		defer func() { _ = f.Close() }()
		if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
			return fmt.Errorf("writing the memory profile: %w", err)
		}
	}
	return nil
}

// measureOne opens data with format once, and measures it.
func measureOne(ctx context.Context, opts options, format ivy.Format, cfg ivy.Config, dir string, data []byte) (r result) {
	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	runtime.GC()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	start := time.Now()
	defer func() {
		r.total = time.Since(start)
		runtime.ReadMemStats(&m1)
		r.bytes = m1.TotalAlloc - m0.TotalAlloc
		r.allocs = m1.Mallocs - m0.Mallocs
	}()
	doc, err := review.OpenIvy(ctx, format, bytes.NewReader(data), cfg, dir)
	r.open = time.Since(start)
	if err != nil {
		r.err = err
		return r
	}
	defer func() { _ = doc.Close() }()
	h := sha256.New()
	defer func() {
		if opts.hash {
			r.hash = hex.EncodeToString(h.Sum(nil)[:8])
		}
	}()
	pager, ok := doc.(ivy.Pager)
	if !ok {
		var img image.Image
		img, r.err = ivy.BestPreview(ctx, doc, image.Point{})
		r.first = time.Since(start)
		if opts.hash && img != nil {
			hashImage(h, img)
		}
		return r
	}
	if _, r.err = renderPage(ctx, pager, 0, opts.dpi); r.err != nil {
		return r
	}
	r.first = time.Since(start)
	t := time.Now()
	r.pages, r.err = pager.PageCount(ctx)
	r.count = time.Since(t)
	if r.err != nil {
		return r
	}
	n := min(opts.pages, r.pages)
	t = time.Now()
	for i := range n {
		var img image.Image
		if img, r.err = renderPage(ctx, pager, i, opts.dpi); r.err != nil {
			return r
		}
		if opts.hash {
			hashImage(h, img)
		}
	}
	if n > 0 {
		r.page = time.Since(t) / time.Duration(n)
	}
	return r
}

// renderPage renders page n of pager at dpi over white.
func renderPage(ctx context.Context, pager ivy.Pager, n int, dpi float32) (image.Image, error) {
	page, err := pager.Page(ctx, n)
	if err != nil {
		return nil, fmt.Errorf("getting page %d: %w", n, err)
	}
	img, err := ivy.RenderImage(ctx, page, dpi, color.White)
	if _, partial := errors.AsType[*ivy.PartialError](err); partial {
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("rendering page %d: %w", n, err)
	}
	return img, nil
}

// hashImage writes the size and the colors of the pixels of img to h.
func hashImage(h hash.Hash, img image.Image) {
	b := img.Bounds()
	_, _ = fmt.Fprintf(h, "%v|", b)
	var buf [16]byte
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			binary.LittleEndian.PutUint32(buf[0:], r)
			binary.LittleEndian.PutUint32(buf[4:], g)
			binary.LittleEndian.PutUint32(buf[8:], bl)
			binary.LittleEndian.PutUint32(buf[12:], a)
			_, _ = h.Write(buf[:])
		}
	}
}

// ms returns d in milliseconds.
func ms(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// withoutTeX is a folder of fonts without its folder lm, as fonts.FS gives
// it.
type withoutTeX struct {
	fs.FS
}

// Open satisfies the fs.FS interface.
func (w withoutTeX) Open(name string) (fs.File, error) {
	if name == "lm" || strings.HasPrefix(name, "lm/") {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return w.FS.Open(name) //nolint:wrapcheck // the error of the folder
}

// ReadDir satisfies the fs.ReadDirFS interface.
func (w withoutTeX) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(w.FS, name)
	if err != nil {
		return nil, err //nolint:wrapcheck // the error of the folder
	}
	if name == "." {
		entries = slices.DeleteFunc(entries, func(e fs.DirEntry) bool { return e.Name() == "lm" })
	}
	return entries, nil
}
