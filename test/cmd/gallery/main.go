// Command gallery draws the pages of the gallery of README.md, from the
// documents of the corpus of the review. Run it from the folder test, after
// `go run ./cmd/review fetch`:
//
//	go run ./cmd/gallery
//
// It writes one PNG file for each page into docs/gallery of the repository.
// Each document is under a license that lets the repository hold a picture
// of it. README.md names the license of each picture.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/test/review"
)

// shot is one page of the gallery.
type shot struct {
	// name is the name of the document in the manifest.
	name string
	// page is the page to draw, from 0.
	page int
	// file is the name of the PNG file in docs/gallery.
	file string
}

// shots are the pages of the gallery, in the order of README.md.
var shots = []shot{
	{"aspose/docx/office-math.docx", 0, "docx-office-math.png"},
	{"poi/doc/floating-pictures.doc", 5, "doc-floating-pictures.png"},
	{"repo/xlsx/chart-treemap.xlsx", 0, "xlsx-treemap.png"},
	{"poi/pptx/bar-chart.pptx", 0, "pptx-bar-chart.png"},
	{"pdfjs/pdf/tracemonkey.pdf", 0, "pdf-tracemonkey.png"},
	{"gutenberg/epub/pg11-images-3.epub", 0, "epub-alice.png"},
	{"latex2e/tex/sample2e.tex", 0, "tex-sample2e.png"},
	{"libreoffice/odp/shapes-test.odp", 0, "odp-shapes.png"},
	{"archive-org/djvu/maudinelriven00thomgoog.djvu", 7, "djvu-title.png"},
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gallery:", err)
		os.Exit(1)
	}
}

// run reads the flags and draws each page of the gallery.
func run(ctx context.Context, args []string) error {
	fl := flag.NewFlagSet("gallery", flag.ContinueOnError)
	width := fl.Int("width", 640, "the width of each picture in pixels")
	if err := fl.Parse(args); err != nil {
		return fmt.Errorf("reading the flags: %w", err)
	}
	repo, err := findRepo()
	if err != nil {
		return err
	}
	entries, err := review.LoadManifest(filepath.Join(repo, "test", "review", "corpus.tsv"))
	if err != nil {
		return err
	}
	byName := make(map[string]review.Entry, len(entries))
	for _, e := range entries {
		byName[e.Name] = e
	}
	tex, err := review.TeXTree(ctx, review.TeXBase, repo)
	if err != nil {
		return err
	}
	cfg := ivy.Config{Fonts: os.DirFS(filepath.Join(repo, "fonts")), TeX: tex}
	out := filepath.Join(repo, "docs", "gallery")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("making the folder of the gallery: %w", err)
	}
	cache := filepath.Join(repo, "test", "review", "cache")
	for _, s := range shots {
		e, ok := byName[s.name]
		if !ok {
			return fmt.Errorf("%s: not in the manifest", s.name)
		}
		if err := draw(ctx, e, s, cfg, repo, cache, out, *width); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
		fmt.Println(filepath.Join(out, s.file))
	}
	return nil
}

// draw draws page s.page of the document of e into the PNG file s.file of
// the folder out, width pixels wide.
func draw(ctx context.Context, e review.Entry, s shot, cfg ivy.Config, repo, cache, out string, width int) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	format, ok := review.FormatFor(e.Kind)
	if !ok {
		return fmt.Errorf("no format for the kind %s", e.Kind)
	}
	path := e.Path(repo, cache)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening the file: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("reading the size of the file: %w", err)
	}
	cfg.Password = e.Password()
	d, err := review.OpenIvy(ctx, format, io.NewSectionReader(f, 0, fi.Size()), cfg, path)
	if err != nil {
		return fmt.Errorf("opening the document: %w", err)
	}
	defer d.Close()
	pager, ok := d.(ivy.Pager)
	if !ok {
		return errors.New("the document has no pages")
	}
	page, err := pager.Page(ctx, s.page)
	if err != nil {
		return fmt.Errorf("getting page %d: %w", s.page, err)
	}
	w, _ := page.Size()
	if w <= 0 {
		return errors.New("the page has no width")
	}
	img, err := ivy.RenderImage(ctx, page, float32(width)*72/w, color.White)
	if err != nil {
		return fmt.Errorf("drawing page %d: %w", s.page, err)
	}
	dst, err := os.Create(filepath.Join(out, s.file))
	if err != nil {
		return fmt.Errorf("creating the picture: %w", err)
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(dst, img); err != nil {
		dst.Close()
		return fmt.Errorf("writing the picture: %w", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("writing the picture: %w", err)
	}
	return nil
}

// findRepo returns the root of the repository of ivy, the first folder
// from the current folder up that holds test/review/corpus.tsv.
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
