package review

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BundleOptions configure Bundle.
type BundleOptions struct {
	// Repo is the root of the repository of ivy.
	Repo string
	// Cache is the cache folder of the files.
	Cache string
	// Out is the folder of the bundle.
	Out string
	// Match and Kinds select the documents, as in RenderOptions.
	Match *regexp.Regexp
	Kinds []string
	// Log receives a line for each missing file. A nil Log writes nothing.
	Log io.Writer
}

// PasswordsFile is the name of the file of the passwords in a bundle.
const PasswordsFile = "passwords.tsv"

// Bundle copies the files of the documents of entries that the options
// select into the folder opts.Out, each at its name in the manifest, for a
// program on another machine, such as Microsoft Office on Windows. It takes
// the files of the cache and of the repository. It also writes
// passwords.tsv: one line for each file with a password, with the name and
// the password separated by a tab. It returns the number of files. A file
// that is not in the cache is left out.
func Bundle(entries []Entry, opts BundleOptions) (int, error) {
	if opts.Out == "" {
		return 0, errors.New("the folder of the bundle is not set")
	}
	if err := os.MkdirAll(opts.Out, 0o755); err != nil {
		return 0, fmt.Errorf("making the folder of the bundle: %w", err)
	}
	var passwords strings.Builder
	n := 0
	for _, e := range selectEntries(entries, opts.Match, opts.Kinds) {
		src := e.Path(opts.Repo, opts.Cache)
		if _, err := os.Stat(src); err != nil {
			if opts.Log != nil {
				_, _ = fmt.Fprintf(opts.Log, "missing %s: %v\n", e.Name, err)
			}
			continue
		}
		dst := filepath.Join(opts.Out, filepath.FromSlash(e.Name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return n, fmt.Errorf("making the folder of %s: %w", e.Name, err)
		}
		_ = os.Remove(dst)
		if err := copyFile(src, dst); err != nil {
			return n, err
		}
		n++
		if p := e.Password(); p != "" {
			if strings.ContainsAny(p, "\t\r\n") {
				return n, fmt.Errorf("the password of %s holds a tab or a line break", e.Name)
			}
			passwords.WriteString(e.Name + "\t" + p + "\n")
		}
	}
	if err := os.WriteFile(filepath.Join(opts.Out, PasswordsFile), []byte(passwords.String()), 0o644); err != nil {
		return n, fmt.Errorf("writing %s: %w", PasswordsFile, err)
	}
	return n, nil
}

// copyFile copies the file src to dst, a new file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("making %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copying %s: %w", src, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", dst, err)
	}
	return nil
}
