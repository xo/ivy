// Command review renders the documents of a corpus with ivy and with
// reference programs, and writes a report in HTML that shows each page of
// ivy beside the pages of the references, so that a maintainer can judge
// the rendering by eye (D5, D13).
//
// Usage:
//
//	review fetch [-cache DIR] [-manifest FILE] [-j N]
//	review render -out DIR [-cache DIR] [-manifest FILE] [-match REGEXP]
//		[-kind LIST] [-pages N] [-dpi N] [-j N] [-timeout DUR]
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
// at its end.
//
// render opens each document with the package of ivy for its kind, and
// renders the first pages and the last page. It renders the same pages
// with the reference programs of the kind, compares each page of ivy with
// each reference, and writes index.html, report.json and the images into
// the folder of -out. It keeps the images of the references in the cache,
// because LibreOffice and calibre are slow. ivy renders each time.
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
		[-kind LIST] [-pages N] [-dpi N] [-j N] [-timeout DUR]

Run "review fetch -h" or "review render -h" for the flags.
`

// run runs the command with the arguments args.
func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "-help" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stderr, usage)
		if len(args) == 0 {
			return errors.New("expected a command: fetch or render")
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
	switch args[0] {
	case "fetch":
		fs.Usage = func() {
			_, _ = fmt.Fprint(fs.Output(), "Usage: review fetch [-cache DIR] [-manifest FILE] [-j N]\n\nDownloads the missing files of the corpus into the cache.\n\n")
			fs.PrintDefaults()
		}
		if err := fs.Parse(args[1:]); err != nil {
			return fmt.Errorf("reading the flags: %w", err)
		}
		entries, err := review.LoadManifest(*manifest)
		if err != nil {
			return err
		}
		return review.Fetch(ctx, entries, review.FetchOptions{
			Cache: *cache, Repo: repo, Jobs: *jobs, Log: os.Stdout,
		})
	case "render":
		out := fs.String("out", "", "the `folder` of the report, which must be new, empty or an earlier report (required)")
		match := fs.String("match", "", "render only the documents whose name matches the `regexp`")
		kinds := fs.String("kind", "", "render only the documents of these kinds, a comma-separated `list` such as pdf,docx")
		pages := fs.Int("pages", 3, "render the first `N` pages and the last page; 0 renders each page")
		dpi := fs.Int("dpi", 96, "the `resolution` in dots per inch")
		timeout := fs.Duration("timeout", 5*time.Minute, "the `time` that ivy, and then the references, can take on one document")
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
			Timeout: *timeout, Log: os.Stdout,
		}
		if *match != "" {
			re, err := regexp.Compile(*match)
			if err != nil {
				return fmt.Errorf("reading -match: %w", err)
			}
			opts.Match = re
		}
		for k := range strings.SplitSeq(*kinds, ",") {
			if k = strings.TrimSpace(strings.ToLower(k)); k != "" {
				opts.Kinds = append(opts.Kinds, k)
			}
		}
		entries, err := review.LoadManifest(*manifest)
		if err != nil {
			return err
		}
		rep, err := review.Render(ctx, entries, opts)
		if rep != nil {
			fmt.Printf("%d documents in %.0f s: %s\n", len(rep.Docs), rep.Elapsed, filepath.Join(*out, "index.html"))
		}
		return err
	}
	return fmt.Errorf("unknown command %q: expected fetch or render", args[0])
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
