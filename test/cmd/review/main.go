// Command review renders the documents of a corpus with ivy and with
// reference programs, and writes a report in HTML that shows each page of
// ivy beside the pages of the references, so that a maintainer can judge
// the rendering by eye (D5, D13).
//
// Usage:
//
//	review fetch [-cache DIR] [-manifest FILE] [-j N]
//	review render -out DIR [-cache DIR] [-manifest FILE] [-match REGEXP]
//		[-kind LIST] [-pages N] [-dpi N] [-j N] [-timeout DUR] [-retry]
//		[-papers LIST] [-system-fonts=BOOL] [-font-dirs LIST] [-faults FILE]
//		[-tex TREE]
//	review bundle -out DIR [-cache DIR] [-manifest FILE] [-match REGEXP]
//		[-kind LIST]
//
// The manifest is test/review/corpus.tsv of the repository, by default.
// Each line names a document: name, kind, url, sha256, size, license,
// source and note, separated by tabs. The url is an https URL, or "repo:"
// and a path in the repository. A note can give the password of the file
// as password=XYZ.
//
// fetch downloads each file that is not in the cache, into the folder
// files of the cache, and checks its size and its sha256. It tries a failed
// download once more, goes on with the other files, and lists the failures
// at its end. It also downloads the files of the page of markdown-it, the
// reference of Markdown, into the folder markdownit of the cache (D70).
//
// render opens each document with the package of ivy for its kind, and
// renders the first pages and the last page. It renders the same pages
// with the reference programs of the kind, compares each page of ivy with
// each reference, and writes index.html, report.json and the images into
// the folder of -out. It keeps the images of the references in the cache,
// because LibreOffice and calibre are slow, and it keeps the failures of the
// reference programs. -retry tries those failures again. ivy renders each
// time. If the references convert a document to PDF, ivy renders that PDF
// file too, as the column "ivy PDF", and compares it with MuPDF. A
// document of a kind without a paper, such as a text file or an ebook,
// renders once on each paper of -papers (D32).
//
// bundle copies the files of the corpus into a folder, at their names, with
// the file passwords.tsv, for Microsoft Office on Windows.
//
// The cache is test/review/cache of the repository, by default. Git
// ignores it. The command finds the repository from the current folder.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/xo/ivy/test/review"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "review:", err)
		os.Exit(1)
	}
}

// usage is the help of the command.
const usage = `review renders a corpus with ivy and with reference programs, and
writes a report in HTML that shows the pages side by side.

Usage:

	review fetch [-cache DIR] [-manifest FILE] [-j N]
	review render -out DIR [-cache DIR] [-manifest FILE] [-match REGEXP]
		[-kind LIST] [-pages N] [-dpi N] [-j N] [-timeout DUR] [-retry]
		[-papers LIST] [-system-fonts=BOOL] [-font-dirs LIST] [-faults FILE]
		[-tex TREE] [-refs primary|all]
	review bundle -out DIR [-cache DIR] [-manifest FILE] [-match REGEXP]
		[-kind LIST]

Run "review fetch -h", "review render -h" or "review bundle -h" for the
flags.
`

// run runs the command with the arguments args.
func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "-help" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stderr, usage)
		if len(args) == 0 {
			return errors.New("expected a command: fetch, render or bundle")
		}
		return nil
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	repo, err := findRepo()
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("review "+args[0], flag.ContinueOnError)
	cache := fs.String("cache", filepath.Join(repo, "test", "review", "cache"), "the cache `folder` of the files and of the references")
	manifest := fs.String("manifest", filepath.Join(repo, "test", "review", "corpus.tsv"), "the manifest `file` of the corpus")
	jobs := fs.Int("j", max(1, runtime.GOMAXPROCS(0)/2), "the `number` of documents at the same time")
	match := fs.String("match", "", "take only the documents whose name matches the `regexp`")
	kinds := fs.String("kind", "", "take only the documents of these kinds, a comma-separated `list` such as pdf,docx")
	selection := func() (*regexp.Regexp, []string, error) {
		var re *regexp.Regexp
		if *match != "" {
			var err error
			if re, err = regexp.Compile(*match); err != nil {
				return nil, nil, fmt.Errorf("reading -match: %w", err)
			}
		}
		var list []string
		for k := range strings.SplitSeq(*kinds, ",") {
			if k = strings.TrimSpace(strings.ToLower(k)); k != "" {
				list = append(list, k)
			}
		}
		return re, list, nil
	}
	switch args[0] {
	case "fetch":
		fs.Usage = func() {
			_, _ = fmt.Fprint(fs.Output(), "Usage: review fetch [-cache DIR] [-manifest FILE] [-j N]\n\n"+
				"Downloads the missing files of the corpus, and of the page of markdown-it, into the cache.\n\n")
			fs.PrintDefaults()
		}
		if err := fs.Parse(args[1:]); err != nil {
			return fmt.Errorf("reading the flags: %w", err)
		}
		entries, err := review.LoadManifest(*manifest)
		if err != nil {
			return err
		}
		fopts := review.FetchOptions{Cache: *cache, Repo: repo, Jobs: *jobs, Log: os.Stdout}
		return errors.Join(review.Fetch(ctx, entries, fopts), review.FetchMarkdownIt(ctx, fopts))
	case "render":
		out := fs.String("out", "", "the `folder` of the report, which must be new, empty or an earlier report (required)")
		pages := fs.Int("pages", 3, "render the first `N` pages and the last page; 0 renders each page")
		dpi := fs.Int("dpi", 96, "the `resolution` in dots per inch")
		timeout := fs.Duration("timeout", 5*time.Minute, "the `time` that ivy, and then the references, can take on one document")
		retry := fs.Bool("retry", false, "try again each failure of the references that the cache keeps from an earlier run")
		papers := fs.String("papers", review.DefaultPapers,
			"the papers of a document whose file gives none, a comma-separated `list` of letter, legal, a4 and a5, "+
				"each with -landscape or not, and default; an empty list renders such a document once (D32)")
		systemFonts := fs.Bool("system-fonts", true, "give ivy the fonts of the system, as the references have them")
		fontDirs := fs.String("font-dirs", strings.Join(review.DefaultFontDirs(), ","),
			"a comma-separated `list` of font folders more for ivy and for the references; the default is the font folders of the account")
		tex := fs.String("tex", review.TeXBase, "the `tree` of TeX files of a TeX document: base, the standard classes as kpsewhich finds them; "+
			"dist, the whole TeX distribution; or none (D65)")
		refs := fs.String("refs", review.RefsPrimary, "the `references`: primary, the primary oracle of each kind and at most one secondary oracle; "+
			"or all, every conversion drawn by MuPDF, Poppler Splash, Poppler Cairo and Chrome (D69)")
		faults := fs.String("faults", filepath.Join(repo, "test", "review", "reference-faults.tsv"),
			"the `file` of the known faults of the references")
		fs.Usage = func() {
			_, _ = fmt.Fprint(fs.Output(), "Usage: review render -out DIR [flags]\n\nRenders the corpus with ivy and with the references, and writes the report.\n\n")
			fs.PrintDefaults()
		}
		if err := fs.Parse(args[1:]); err != nil {
			return fmt.Errorf("reading the flags: %w", err)
		}
		if *out == "" {
			fs.Usage()
			return errors.New("expected the folder of the report: -out DIR")
		}
		opts := review.RenderOptions{
			Repo: repo, Cache: *cache, Out: *out, Pages: *pages, DPI: *dpi, Jobs: *jobs,
			Timeout: *timeout, Retry: *retry, Log: os.Stdout, SystemFonts: *systemFonts, Refs: *refs,
		}
		var err error
		if opts.Match, opts.Kinds, err = selection(); err != nil {
			return err
		}
		if opts.Papers, err = review.ParsePapers(*papers); err != nil {
			return fmt.Errorf("reading -papers: %w", err)
		}
		for d := range strings.SplitSeq(*fontDirs, ",") {
			if d = strings.TrimSpace(d); d != "" {
				opts.FontDirs = append(opts.FontDirs, d)
			}
		}
		if *faults != "" {
			if opts.Faults, err = review.LoadFaults(*faults); err != nil {
				return err
			}
		}
		if opts.TeX, err = review.TeXTree(ctx, *tex, repo); err != nil {
			return err
		}
		opts.TeXMode = *tex
		entries, err := review.LoadManifest(*manifest)
		if err != nil {
			return err
		}
		rep, err := review.Render(ctx, entries, opts)
		if rep != nil {
			fmt.Printf("%d documents in %.0f s: %s\n", len(rep.Docs), rep.Elapsed, filepath.Join(*out, "index.html"))
		}
		return err
	case "bundle":
		out := fs.String("out", "", "the `folder` of the bundle (required)")
		fs.Usage = func() {
			_, _ = fmt.Fprint(fs.Output(), "Usage: review bundle -out DIR [flags]\n\n"+
				"Copies the files of the corpus into DIR at their names, with passwords.tsv, for Microsoft Office on Windows.\n\n")
			fs.PrintDefaults()
		}
		if err := fs.Parse(args[1:]); err != nil {
			return fmt.Errorf("reading the flags: %w", err)
		}
		if *out == "" {
			fs.Usage()
			return errors.New("expected the folder of the bundle: -out DIR")
		}
		opts := review.BundleOptions{Repo: repo, Cache: *cache, Out: *out, Log: os.Stdout}
		var err error
		if opts.Match, opts.Kinds, err = selection(); err != nil {
			return err
		}
		entries, err := review.LoadManifest(*manifest)
		if err != nil {
			return err
		}
		n, err := review.Bundle(entries, opts)
		fmt.Printf("%d files in %s\n", n, *out)
		return err
	}
	return fmt.Errorf("unknown command %q: expected fetch, render or bundle", args[0])
}

// findRepo returns the root of the repository of ivy: the first folder
// from the current one up whose go.mod declares the module
// github.com/xo/ivy.
func findRepo() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("finding the current folder: %w", err)
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for line := range strings.SplitSeq(string(data), "\n") {
				if strings.TrimSpace(line) == "module github.com/xo/ivy" {
					return dir, nil
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("the current folder is not in the repository of ivy")
		}
		dir = parent
	}
}
