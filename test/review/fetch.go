package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxDownload bounds a download whose size the manifest does not give.
const maxDownload = 2 << 30

// FetchOptions configure Fetch.
type FetchOptions struct {
	// Cache is the cache folder. The files go in its folder files.
	Cache string
	// Repo is the root of the repository of ivy, for the entries of repo:.
	Repo string
	// Jobs is the number of downloads at the same time.
	Jobs int
	// Client makes the requests. A nil Client uses http.DefaultClient.
	Client *http.Client
	// Log receives one line for each file. A nil Log writes nothing.
	Log io.Writer
}

// Fetch downloads each file of entries that is not in the cache, or whose
// size or hash differs, and checks its size and its hash. It tries a failed
// download once more. It downloads the other files after a failure, and
// returns an error that lists each failure.
func Fetch(ctx context.Context, entries []Entry, opts FetchOptions) error {
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	jobs := max(opts.Jobs, 1)
	var mu sync.Mutex
	var errs []error
	logf := func(format string, args ...any) {
		if opts.Log == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		_, _ = fmt.Fprintf(opts.Log, format+"\n", args...)
	}
	work := make(chan Entry)
	var wg sync.WaitGroup
	for range jobs {
		wg.Go(func() {
			for e := range work {
				status, err := fetchOne(ctx, client, e, opts)
				if err != nil {
					logf("fail  %s: %v", e.Name, err)
					mu.Lock()
					errs = append(errs, fmt.Errorf("%s: %w", e.Name, err))
					mu.Unlock()
					continue
				}
				logf("%-5s %s", status, e.Name)
			}
		})
	}
	for _, e := range entries {
		select {
		case work <- e:
		case <-ctx.Done():
		}
	}
	close(work)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fetching: %w", err)
	}
	if len(errs) != 0 {
		return fmt.Errorf("%d of %d files failed: %w", len(errs), len(entries), errors.Join(errs...))
	}
	return nil
}

// fetchOne makes sure that the file of e is in place and correct. It
// returns "have", "repo" or "got".
func fetchOne(ctx context.Context, client *http.Client, e Entry, opts FetchOptions) (string, error) {
	dst := e.Path(opts.Repo, opts.Cache)
	if _, ok := e.RepoPath(); ok {
		if err := checkFile(dst, e); err != nil {
			return "", err
		}
		return "repo", nil
	}
	if checkFile(dst, e) == nil {
		return "have", nil
	}
	err := download(ctx, client, e, dst)
	if err != nil && ctx.Err() == nil {
		// one more try, after a short wait
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
		}
		err = download(ctx, client, e, dst)
	}
	if err != nil {
		return "", err
	}
	return "got", nil
}

// checkFile returns an error if the file at path is missing, or if its size
// or its hash differs from the ones of e.
func checkFile(path string, e Entry) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening the file: %w", err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	return checkSum(e, n, hex.EncodeToString(h.Sum(nil)))
}

// checkSum returns an error if the size n or the hash sum differs from the
// ones of e.
func checkSum(e Entry, n int64, sum string) error {
	if e.Size >= 0 && n != e.Size {
		return fmt.Errorf("expected %d bytes, got: %d", e.Size, n)
	}
	if e.SHA256 != "" && sum != e.SHA256 {
		return fmt.Errorf("expected the sha256 %s, got: %s", e.SHA256, sum)
	}
	return nil
}

// download gets the URL of e into dst, through a temporary file in the same
// folder, and checks the size and the hash.
func download(ctx context.Context, client *http.Client, e Entry, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("making the folder of the file: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.URL, nil)
	if err != nil {
		return fmt.Errorf("making the request: %w", err)
	}
	req.Header.Set("User-Agent", "ivy-review")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("getting %s: %w", e.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("getting %s: %s", e.URL, resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".fetch-*")
	if err != nil {
		return fmt.Errorf("making a temporary file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	limit := int64(maxDownload)
	if e.Size >= 0 {
		limit = e.Size + 1
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, limit))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("downloading %s: %w", e.URL, err)
	}
	if err := checkSum(e, n, hex.EncodeToString(h.Sum(nil))); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return fmt.Errorf("moving the file in place: %w", err)
	}
	return nil
}
