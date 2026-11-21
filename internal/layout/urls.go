// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/urls.py, for the files of a Source.

package layout

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/xo/ivy"
)

// unicodeSchemeRE is UNICODE_SCHEME_RE in weasyprint/urls.py.
var unicodeSchemeRE = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9.+-]+):`)

// ErrURLFetching is URLFetchingError in weasyprint/urls.py: a resource that
// the fetcher cannot read. The code that fetches a resource logs it and goes
// on.
var ErrURLFetching = errors.New("cannot fetch url")

// iriSafe holds the bytes that iriToURI does not escape: the reserved
// characters, the unreserved characters and "%".
const iriSafe = "/:?#[]@!$&'()*+,;=~%-._"

// iriToURI is iri_to_uri in weasyprint/urls.py. It turns a Unicode IRI into
// an ASCII URI that conforms to RFC 3986.
func iriToURI(u string) string {
	if strings.HasPrefix(u, "data:") {
		// Data URIs can be huge, but do not need this anyway.
		return u
	}
	var b strings.Builder
	for i := range len(u) {
		c := u[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9',
			strings.IndexByte(iriSafe, c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// pathToURL is path2url in weasyprint/urls.py, for a path of a Source. The
// URL of a path in the FS of a Source is a file URL rooted at "/", such as
// "file:///OEBPS/text/chapter1.xhtml". The fetcher reads such a URL from the
// FS, so that the links of a document resolve as in a file system.
func pathToURL(p string) string {
	u := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(p, "/")}
	return u.String()
}

// urlIsAbsolute is url_is_absolute in weasyprint/urls.py.
func urlIsAbsolute(u string) bool {
	return unicodeSchemeRE.MatchString(u)
}

// urljoin is urljoin of urllib.parse, for the URLs that the port makes. It
// returns ref when base or ref is not a valid URL.
func urljoin(base, ref string) string {
	if base == "" {
		return ref
	}
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return b.ResolveReference(r).String()
}

// unquote is unquote of urllib.parse. It keeps a bad escape as it is.
func unquote(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			b.WriteByte(unhex(s[i+1])<<4 | unhex(s[i+2]))
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// isHex reports whether c is a hexadecimal digit.
func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// unhex returns the value of a hexadecimal digit.
func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	}
	return c - 'A' + 10
}

// URLTuple is the tuple of get_url_tuple in weasyprint/urls.py:
// ('internal', fragment) or ('external', uri).
func urlTuple(kind, value string) Tuple {
	return Tuple{String(kind), String(value)}
}

// getURLTuple is get_url_tuple in weasyprint/urls.py. It returns nil for a
// relative URL without a base URL.
func getURLTuple(u, baseURL string) Value {
	switch {
	case strings.HasPrefix(u, "#"):
		return urlTuple("internal", unquote(u[1:]))
	case urlIsAbsolute(u):
		return urlTuple("external", iriToURI(u))
	case baseURL != "":
		return urlTuple("external", iriToURI(urljoin(baseURL, u)))
	}
	return nil
}

// urlJoin is url_join in weasyprint/urls.py: urljoin, with a log when
// base_url is needed and missing. It returns false for None.
func urlJoin(log *logger, baseURL, u string, allowRelative bool, context string) (string, bool) {
	switch {
	case urlIsAbsolute(u):
		return iriToURI(u), true
	case baseURL != "":
		return iriToURI(urljoin(baseURL, u)), true
	case allowRelative:
		return iriToURI(u), true
	}
	log.error("Relative URI reference without a base URI: %s", context)
	return "", false
}

// GetLink is get_link in weasyprint/urls.py. It returns ('url',
// ('external', absolute_uri)), ('url', ('internal', unquoted_fragment_id)),
// or nil.
func GetLink(log *logger, u, baseURL string) Value {
	if strings.HasPrefix(u, "#") && len(u) > 1 {
		// Do not require a base_url when the value is just a fragment.
		return Tuple{String("url"), urlTuple("internal", unquote(u[1:]))}
	}
	uri, ok := urlJoin(log, baseURL, u, true, u)
	if !ok || uri == "" {
		return nil
	}
	if baseURL != "" {
		parsed, err := url.Parse(uri)
		if err != nil {
			log.warning("Malformed URL: %s", uri)
		} else if parsedBase, err := url.Parse(baseURL); err != nil {
			log.warning("Malformed base URL: %s", baseURL)
		} else if parsed.Fragment != "" && sameURLWithoutFragment(parsed, parsedBase) {
			// Compare with fragments removed.
			return Tuple{String("url"), urlTuple("internal", unquote(parsed.EscapedFragment()))}
		}
	}
	return Tuple{String("url"), urlTuple("external", uri)}
}

// sameURLWithoutFragment compares two URLs without their fragments, as the
// comparison of parsed[:-1] of upstream does.
func sameURLWithoutFragment(a, b *url.URL) bool {
	return a.Scheme == b.Scheme && a.Host == b.Host && a.User.String() == b.User.String() &&
		a.EscapedPath() == b.EscapedPath() && a.RawQuery == b.RawQuery
}

// URLFetcher is URLFetcher in weasyprint/urls.py. It reads the resources of
// a document: the style sheets, the images and the fonts.
//
// ivy never reads the network (D1). The fetcher reads a file URL from FS,
// with the path of the URL as a path in FS, and decodes a data URL. Any other
// URL, such as an http URL, fails with ErrURLFetching.
type URLFetcher struct {
	// FS holds the files. A nil FS reads no file.
	FS fs.FS
	// MaxSize is the size in bytes of the largest resource that the fetcher
	// reads, from ivy.Limits.MaxMemory. Zero or less is the default of
	// ivy.Limits.
	MaxSize int64
	// Objects are the objects of the Source, which the images load by their
	// URL, ObjectURL(name). The fetcher itself reads no object.
	//
	// Port: upstream has no objects.
	Objects map[string]Object
}

// URLFetcherResponse is URLFetcherResponse in weasyprint/urls.py: a resource
// that the fetcher read.
type URLFetcherResponse struct {
	// URL is the URL of the resource.
	URL string
	// Body holds the bytes of the resource.
	Body []byte
	// ContentType is the media type of the resource, such as "text/css",
	// without its parameters.
	ContentType string
	// Charset is the parameter charset of the media type, or "".
	Charset string
}

// Fetch is URLFetcher.fetch and fetch in weasyprint/urls.py. It returns an
// error that wraps ErrURLFetching when it cannot read the URL.
func (f *URLFetcher) Fetch(u string) (*URLFetcherResponse, error) {
	match := unicodeSchemeRE.FindStringSubmatch(u)
	if match == nil {
		return nil, fmt.Errorf("%w: not an absolute URI: %s", ErrURLFetching, u)
	}
	scheme := strings.ToLower(match[1])
	if scheme == "file" {
		// Remove query and fragment parts from file URLs.
		u, _, _ = strings.Cut(u, "?")
		u, _, _ = strings.Cut(u, "#")
	}
	u = iriToURI(u)
	switch scheme {
	case "file":
		return f.fetchFile(u)
	case "data":
		return f.fetchData(u)
	}
	return nil, fmt.Errorf("%w: ivy reads no %s URL: %s", ErrURLFetching, scheme, u)
}

// maxSize returns the largest size of a resource.
func (f *URLFetcher) maxSize() int64 {
	if f.MaxSize > 0 {
		return f.MaxSize
	}
	return ivy.DefaultMaxMemory
}

// fetchFile reads a file URL from FS.
func (f *URLFetcher) fetchFile(u string) (*URLFetcherResponse, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrURLFetching, err)
	}
	if parsed.Host != "" && parsed.Host != "localhost" {
		return nil, fmt.Errorf("%w: file URL with a host: %s", ErrURLFetching, u)
	}
	name := path.Clean(strings.TrimPrefix(strings.ReplaceAll(parsed.Path, `\`, "/"), "/"))
	if f.FS == nil || name == "." || !fs.ValidPath(name) {
		return nil, fmt.Errorf("%w: no such file: %s", ErrURLFetching, u)
	}
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrURLFetching, err)
	}
	defer func() { _ = file.Close() }()
	// Port: the size of a resource is bounded by ivy.Limits.MaxMemory.
	body, err := readAll(file, f.maxSize())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrURLFetching, err)
	}
	if int64(len(body)) > f.maxSize() {
		return nil, fmt.Errorf("%w: %w: file larger than %d bytes: %s", ErrURLFetching, ivy.ErrLimit, f.maxSize(), u)
	}
	return &URLFetcherResponse{URL: u, Body: body, ContentType: guessType(name)}, nil
}

// readAll is io.ReadAll of at most limit+1 bytes of a file.
//
// Port: io.ReadAll grows its buffer as it reads, so it allocates about
// twice the size of a large file. readAll makes the buffer of the size
// that Stat gives, when Stat gives one within the limit.
func readAll(file fs.File, limit int64) ([]byte, error) {
	r := io.LimitReader(file, limit+1)
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		body, err := io.ReadAll(r)
		if err != nil {
			return body, fmt.Errorf("reading the file: %w", err)
		}
		return body, nil
	}
	// The extra byte lets the read that finds the end need no new buffer.
	buf := make([]byte, 0, info.Size()+1)
	for {
		if len(buf) == cap(buf) {
			buf = append(buf, 0)[:len(buf)]
		}
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if errors.Is(err, io.EOF) {
			return buf, nil
		}
		if err != nil {
			return buf, fmt.Errorf("reading the file: %w", err)
		}
	}
}

// fileTypes maps the extensions of files to their media types, as
// mimetypes.guess_type of Python does for the files of a document.
var fileTypes = map[string]string{
	".css":   "text/css",
	".htm":   "text/html",
	".html":  "text/html",
	".xhtml": "application/xhtml+xml",
	".xht":   "application/xhtml+xml",
	".xml":   "text/xml",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".bmp":   "image/bmp",
	".tif":   "image/tiff",
	".tiff":  "image/tiff",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".txt":   "text/plain",
}

// guessType returns the media type of a file from its extension. An unknown
// extension gives text/plain, as the FileHandler of urllib does.
func guessType(name string) string {
	if t, ok := fileTypes[strings.ToLower(path.Ext(name))]; ok {
		return t
	}
	return "text/plain"
}

// fetchData decodes a data URL, as the DataHandler of urllib does.
func (f *URLFetcher) fetchData(u string) (*URLFetcherResponse, error) {
	rest := u[len("data:"):]
	header, data, ok := strings.Cut(rest, ",")
	if !ok {
		return nil, fmt.Errorf("%w: data URL without a comma", ErrURLFetching)
	}
	if int64(len(data)) > 2*f.maxSize() {
		return nil, fmt.Errorf("%w: %w: data URL larger than %d bytes", ErrURLFetching, ivy.ErrLimit, f.maxSize())
	}
	isBase64 := false
	if h, ok := strings.CutSuffix(header, ";base64"); ok {
		header, isBase64 = h, true
	}
	mediaType, params, _ := strings.Cut(header, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	charset := ""
	if mediaType == "" {
		mediaType, charset = "text/plain", "US-ASCII"
	}
	for param := range strings.SplitSeq(params, ";") {
		if k, v, ok := strings.Cut(param, "="); ok && strings.EqualFold(strings.TrimSpace(k), "charset") {
			charset = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	body := []byte(unquote(data))
	if isBase64 {
		body = bytes.Map(func(r rune) rune {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
				return -1
			}
			return r
		}, body)
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(body)))
		n, err := base64.StdEncoding.Decode(decoded, padBase64(body))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrURLFetching, err)
		}
		body = decoded[:n]
	}
	return &URLFetcherResponse{URL: u, Body: body, ContentType: mediaType, Charset: charset}, nil
}

// padBase64 adds the padding that a base64 text lacks.
func padBase64(b []byte) []byte {
	if n := len(b) % 4; n != 0 {
		return append(b, bytes.Repeat([]byte("="), 4-n)...)
	}
	return b
}
