// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports part of weasyprint/__init__.py: the class HTML and
// _find_base_url. The class CSS is in css.go, with the parsing of the style
// sheets.

package layout

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/xo/okage/cssselect2"
	"github.com/xo/okage/html"
	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/charset"
)

// DefaultMaxTreeDepth is the default depth of the elements that a tree
// keeps.
const DefaultMaxTreeDepth = html.DefaultMaxDepth

// findBaseURL is _find_base_url in weasyprint/__init__.py. It returns the
// base URL of the document. See
// https://www.w3.org/TR/html5/urls.html#document-base-url.
func findBaseURL(root *Element, fallbackBaseURL string) string {
	for base := range root.IterTag("base") {
		if href := strings.TrimSpace(base.GetDefault("href", "")); href != "" {
			return urljoin(fallbackBaseURL, href)
		}
		break
	}
	return fallbackBaseURL
}

// HTML is the class HTML of weasyprint/__init__.py: a parsed document.
type HTML struct {
	// BaseURL is the base URL of the document, from its URL and its <base>
	// element.
	BaseURL string
	// URLFetcher reads the resources of the document.
	URLFetcher *URLFetcher
	// MediaType is the media type of @media, "print" by default.
	MediaType string
	// WrapperElement is the wrapper of cssselect2 of the root element.
	WrapperElement *cssselect2.ElementWrapper
	// EtreeElement is the root element.
	EtreeElement *Element
	// Limits bound the work on the document.
	Limits ivy.Limits

	log *logger
}

// HTMLOptions holds the options of NewHTML and LoadHTML.
type HTMLOptions struct {
	// MediaType is the media type of @media. "" is "print".
	MediaType string
	// Limits bound the work on the document. A zero field takes its
	// default.
	Limits ivy.Limits
	// Logger receives the log of the document, or nil.
	Logger *slog.Logger
}

// NewHTML is HTML.__init__ in weasyprint/__init__.py, for a tree that is
// already built. url is the URL of the document, such as the URL of
// pathToURL for a file of a Source.
func NewHTML(root *Element, url string, fetcher *URLFetcher, opts HTMLOptions) *HTML {
	mediaType := opts.MediaType
	if mediaType == "" {
		mediaType = "print"
	}
	return &HTML{
		BaseURL:        findBaseURL(root, url),
		URLFetcher:     fetcher,
		MediaType:      mediaType,
		WrapperElement: cssselect2.FromHTMLRoot(selectorElement{root}, ""),
		EtreeElement:   root,
		Limits:         opts.Limits.WithDefaults(),
		log:            newLogger(opts.Logger),
	}
}

// LoadHTML reads and parses a document of a Source: a path in its FS that
// ends in .xhtml, .xht or .xml is read as XHTML with the XML parser of
// okage, and any other one as HTML. It returns an error that wraps
// ivy.ErrFormat for a document that cannot be parsed.
func LoadHTML(src Source, path string, opts HTMLOptions) (*HTML, error) {
	limits := opts.Limits.WithDefaults()
	fetcher := &URLFetcher{FS: src.FS, MaxSize: limits.MaxMemory, Objects: src.Objects}
	url := pathToURL(path)
	response, err := fetcher.Fetch(url)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var root *Element
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".xht") || strings.HasSuffix(lower, ".xml") {
		options := roxmltree.DefaultParsingOptions()
		options.AllowDTD = true
		options.NodesLimit = uint32(min(limits.MaxObjects, 1<<31))
		doc, err := roxmltree.ParseWithOptions(string(response.Body), options)
		if err != nil {
			return nil, fmt.Errorf("%w: parsing %s: %w", ivy.ErrFormat, path, err)
		}
		root = TreeFromXML(doc, limits.MaxObjects, DefaultMaxTreeDepth)
	} else {
		// Port: html5lib of upstream decodes every encoding of the Encoding
		// Standard. The parser of okage decodes UTF-8, UTF-16 and
		// windows-1252, and charset.Reader decodes the others, such as the
		// code pages of a CHM file.
		parseOpts := []html.DocumentOption{html.WithMaxNodes(limits.MaxObjects), html.WithCharsetReader(charset.Reader)}
		if response.Charset != "" {
			parseOpts = append(parseOpts, html.WithCharset(response.Charset))
		}
		doc, err := html.ParseDocument(bytes.NewReader(response.Body), parseOpts...)
		if err != nil {
			return nil, fmt.Errorf("%w: parsing %s: %w", ivy.ErrFormat, path, err)
		}
		root = TreeFromHTML(doc, limits.MaxObjects, DefaultMaxTreeDepth)
	}
	if root == nil {
		return nil, fmt.Errorf("%w: %s has no root element", ivy.ErrFormat, path)
	}
	opts.Limits = limits
	return NewHTML(root, url, fetcher, opts), nil
}

// userAgent holds the style sheets and the counter styles of the user agent:
// UA_STYLESHEET, UA_FORM_STYLESHEET, PH_STYLESHEET and UA_COUNTER_STYLE in
// weasyprint/html.py. They are parsed once, and shared by the documents.
type userAgent struct {
	stylesheet     *CSS
	formStylesheet *CSS
	phStylesheet   *CSS
	counterStyle   CounterStyle
}

// userAgentStylesheets returns the style sheets of the user agent, and
// parses them the first time.
var userAgentStylesheets = sync.OnceValue(func() *userAgent {
	ua := &userAgent{counterStyle: CounterStyle{}}
	ctx := &cssContext{
		fetcher:       &URLFetcher{},
		counterStyle:  ua.counterStyle,
		colorProfiles: map[string]*ColorProfile{},
	}
	ua.stylesheet = newCSS(ctx, CSSSource{Text: uaCSS}, "print", nil)
	ua.formStylesheet = newCSS(ctx, CSSSource{Text: uaFormCSS}, "print", nil)
	ctx.counterStyle = CounterStyle{}
	ua.phStylesheet = newCSS(ctx, CSSSource{Text: phCSS}, "print", nil)
	return ua
})
