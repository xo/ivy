// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/html.py: the specific handling of some HTML
// elements, especially the replaced elements.
//
// Replaced elements, such as <img> elements, are rendered externally and
// behave as an atomic opaque box in CSS. In general, they may or may not
// have intrinsic dimensions. But the only replaced elements that WeasyPrint
// supports are images with intrinsic dimensions.
//

package layout

import (
	_ "embed" // The style sheets of the user agent.
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage/cssselect2"
	"github.com/xo/okage/tinycss2"
)

// The style sheets of weasyprint/css: UA, UA_FORM and PH in
// weasyprint/html.py.
var (
	//go:embed css/html5_ua.css
	uaCSS string
	//go:embed css/html5_ua_form.css
	uaFormCSS string
	//go:embed css/html5_ph.css
	phCSS string
)

// The characters of weasyprint/html.py.
const (
	// whitespace is WHITESPACE, the ASCII whitespace of
	// https://infra.spec.whatwg.org/#ascii-whitespace.
	whitespace = " \t\n\f\r"
	// tabOrNewline is TAB_OR_NEWLINE.
	tabOrNewline = "\t\n\r"
)

// The regular expressions of weasyprint/html.py.
var (
	// spaceSeparatedTokensRE is SPACE_SEPARATED_TOKENS_RE.
	spaceSeparatedTokensRE = regexp.MustCompile(`[^ \t\n\f\r]+`)
	// integerRE is INTEGER_RE.
	integerRE = regexp.MustCompile(`^[ \t\n\f\r]*([+-]?)([0-9]+)`)
	// dimensionRE is DIMENSION_RE.
	dimensionRE = regexp.MustCompile(`^[ \t\n\f\r]*([0-9]+([.][0-9]*)?)(%)?`)
)

// parseURL is parse_url in weasyprint/html.py. It parses a valid URL that
// spaces can surround. It returns false for None: an empty or missing
// attribute, or a relative URL when the document has no base URL and
// allowRelative is false. Else it returns a URL, absolute if possible.
func parseURL(log *logger, s, baseURL string, allowRelative bool) (string, bool) {
	if s == "" {
		return "", false
	}
	s = strings.Trim(s, whitespace)
	for _, character := range tabOrNewline {
		s = strings.ReplaceAll(s, string(character), "")
	}
	if s == "" {
		return "", false
	}
	return urlJoin(log, baseURL, s, allowRelative, s)
}

// parseInteger is parse_integer in weasyprint/html.py. It parses an integer
// from the value of an HTML attribute, and returns false on error. See
// https://html.spec.whatwg.org/#rules-for-parsing-integers.
func parseInteger(s string) (int, bool) {
	match := integerRE.FindStringSubmatch(s)
	if match == nil {
		return 0, false
	}
	n, err := strconv.Atoi(match[2])
	if err != nil {
		// Port: Python has integers of any size. A larger integer
		// saturates.
		n = math.MaxInt32
	}
	if match[1] == "-" {
		n = -n
	}
	return n, true
}

// parseNonNegativeInteger is parse_non_negative_integer in
// weasyprint/html.py. See
// https://html.spec.whatwg.org/#rules-for-parsing-non-negative-integers.
func parseNonNegativeInteger(s string) (int, bool) {
	n, ok := parseInteger(s)
	return n, ok && n >= 0
}

// parseDimensionValue is parse_dimension_value in weasyprint/html.py. It
// parses a dimension from the value of an HTML attribute, and returns a
// number and a unit, "%" or "px", or false on error. See
// https://html.spec.whatwg.org/#rules-for-parsing-dimension-values.
func parseDimensionValue(s string) (float64, string, bool) {
	match := dimensionRE.FindStringSubmatch(s)
	if match == nil {
		return 0, "", false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, "", false
	}
	if match[3] == "%" {
		return value, "%", true
	}
	return value, "px", true
}

// parseLegacyColor is parse_legacy_color in weasyprint/html.py. It parses a
// legacy color from the value of an HTML attribute, and returns a color
// string that CSS reads, or false on error. See
// https://html.spec.whatwg.org/#rules-for-parsing-a-legacy-colour-value.
func parseLegacyColor(s string) (string, bool) {
	s = strings.Trim(s, whitespace)
	if lower := strings.ToLower(s); lower == "transparent" || lower == "currentcolor" {
		return "", false
	}
	// Use the CSS3 color parser for simplicity.
	color, _ := tinycss2.ParseColor3(tinycss2.ParseOneComponentValue(s, tinycss2.WithSkipComments(true)))
	if color == nil {
		return "", false
	}
	channel := func(v float64) int {
		return int(math.RoundToEven(max(0, min(1, v)) * 255))
	}
	return fmt.Sprintf("#%02x%02x%02x", channel(color.Red), channel(color.Green), channel(color.Blue)), true
}

// legacyColorCSS returns the color of parseLegacyColor as the f-string of
// upstream formats it, with "None" on error.
func legacyColorCSS(s string) string {
	if color, ok := parseLegacyColor(s); ok {
		return color
	}
	return "None"
}

// mapToPixelLength is map_to_pixel_length in weasyprint/html.py. It maps the
// value of an HTML attribute to a length in pixels, and returns false on
// error.
func mapToPixelLength(s string) (string, bool) {
	if value, ok := parseNonNegativeInteger(s); ok {
		return strconv.Itoa(value) + "px", true
	}
	return "", false
}

// mapToDimensionProperty is map_to_dimension_property in weasyprint/html.py.
// It maps the value of an HTML attribute to a dimension, and returns false
// on error.
func mapToDimensionProperty(s string) (string, bool) {
	value, unit, ok := parseDimensionValue(s)
	if !ok {
		return "", false
	}
	return pyStr(value) + unit, true
}

// mapToDimensionPropertyIgnoringZero is
// map_to_dimension_property_ignoring_zero in weasyprint/html.py. It maps the
// value of an HTML attribute to a dimension, ignoring a zero value, and
// returns false on error.
func mapToDimensionPropertyIgnoringZero(s string) (string, bool) {
	value, unit, ok := parseDimensionValue(s)
	if !ok || value == 0 {
		return "", false
	}
	return pyStr(value) + unit, true
}

// asciiLower is ascii_lower in weasyprint/html.py. It turns only the ASCII
// letters into lower case, for the ASCII case-insensitive matching of
// https://whatwg.org/C#ascii-case-insensitive.
func asciiLower(s string) string {
	return tinycss2.ASCIILower(s)
}

// elementHasLinkType is element_has_link_type in weasyprint/html.py. It
// reports whether the element has a rel attribute with the link type.
func elementHasLinkType(element *Element, linkType string) bool {
	for _, token := range spaceSeparatedTokensRE.FindAllString(element.GetDefault("rel", ""), -1) {
		if asciiLower(token) == linkType {
			return true
		}
	}
	return false
}

// htmlHandler is a handler of HTML_HANDLERS in weasyprint/html.py: a
// function that takes an HTML element and its box, and returns its boxes.
type htmlHandler func(b *builder, element *Element, box *Box, baseURL string) []*Box

// htmlHandlers is HTML_HANDLERS in weasyprint/html.py: the handlers of the
// HTML elements that need special care, by tag.
var htmlHandlers map[string]htmlHandler

func init() {
	htmlHandlers = map[string]htmlHandler{
		"img":                handleImg,
		"a":                  handleA,
		"embed":              handleEmbed,
		"object":             handleObject,
		"colgroup":           handleColgroup,
		"col":                handleCol,
		"{" + nsSVG + "}svg": handleSVG,
	}
}

// handleElement is handle_element in weasyprint/html.py. It handles the
// HTML elements that need special care, and returns a list of boxes, which
// can be empty.
func handleElement(b *builder, element *Element, box *Box, baseURL string) []*Box {
	if handler, ok := htmlHandlers[box.ElementTag]; ok {
		return handler(b, element, box, baseURL)
	}
	// Port: see builder.one.
	b.one[0] = box
	return b.one[:]
}

// makeReplacedBox is make_replaced_box in weasyprint/html.py. It wraps an
// image in a replaced box, which is block-level or inline-level as the
// element is.
func makeReplacedBox(element *Element, box *Box, image Image) *Box {
	kind := KindInlineReplaced
	if tupleHas(box.Style.Get("display"), "block") {
		kind = KindBlockReplaced
	}
	newBox := newReplacedBox(kind, element.Tag, box.Style, element, image)
	// TODO: check other attributes that need to be copied
	// TODO: find another solution
	from, to := box.extra(), newBox.ownExtra()
	to.StringSet = from.StringSet
	to.BookmarkLabel, to.HasBookmarkLabel = from.BookmarkLabel, from.HasBookmarkLabel
	return newBox
}

// handleImg is handle_img in weasyprint/html.py. It handles an <img>
// element, and returns an image or the alternative text. See
// https://www.w3.org/TR/html5/embedded-content-1.html#the-img-element.
func handleImg(b *builder, element *Element, box *Box, baseURL string) []*Box {
	if src, ok := parseURL(b.log, element.GetDefault("src", ""), baseURL, false); ok {
		orientation := box.Style.Get("image_orientation")
		if image := b.images.get(src, "", orientation); image != nil {
			return []*Box{makeReplacedBox(element, box, image)}
		}
	}
	// Invalid image, use the alt-text.
	if alt := element.GetDefault("alt", ""); alt != "" {
		box.Children = []*Box{anonymousTextFrom(box, alt)}
		return []*Box{box}
	}
	// TODO: find some indicator that an image is missing.
	return nil
}

// handleA is handle_a in weasyprint/html.py. It sets the link of the box of
// an <a> element.
func handleA(b *builder, element *Element, box *Box, baseURL string) []*Box {
	if href, ok := parseURL(b.log, element.GetDefault("href", ""), baseURL, true); ok {
		box.ownExtra().Link = GetLink(b.log, href, baseURL)
	}
	return []*Box{box}
}

// handleEmbed is handle_embed in weasyprint/html.py. It handles an <embed>
// element, and returns an image or nothing. See
// https://www.w3.org/TR/html5/embedded-content-0.html#the-embed-element.
func handleEmbed(b *builder, element *Element, box *Box, baseURL string) []*Box {
	if src, ok := parseURL(b.log, element.GetDefault("src", ""), baseURL, false); ok {
		mimeType := strings.TrimSpace(element.GetDefault("type", ""))
		orientation := box.Style.Get("image_orientation")
		if image := b.images.get(src, mimeType, orientation); image != nil {
			return []*Box{makeReplacedBox(element, box, image)}
		}
	}
	// No fallback.
	return nil
}

// handleObject is handle_object in weasyprint/html.py. It handles an
// <object> element, and returns an image or the fallback. See
// https://www.w3.org/TR/html5/embedded-content-0.html#the-object-element.
func handleObject(b *builder, element *Element, box *Box, baseURL string) []*Box {
	if data, ok := parseURL(b.log, element.GetDefault("data", ""), baseURL, false); ok {
		mimeType := strings.TrimSpace(element.GetDefault("type", ""))
		orientation := box.Style.Get("image_orientation")
		if image := b.images.get(data, mimeType, orientation); image != nil {
			return []*Box{makeReplacedBox(element, box, image)}
		}
	}
	// The element’s children are the fallback.
	return []*Box{box}
}

// handleColgroup is handle_colgroup in weasyprint/html.py. It handles the
// attribute span of a <colgroup> element.
func handleColgroup(_ *builder, element *Element, box *Box, _ string) []*Box {
	if box.Kind == KindTableColumnGroup {
		hasCol := false
		for _, child := range element.Children {
			if child.Tag == "col" {
				hasCol = true
				break
			}
		}
		if !hasCol {
			span := box.Span()
			box.Children = make([]*Box, span)
			for i := range span {
				box.Children[i] = anonymousFrom(KindTableColumn, box, nil)
			}
		}
	}
	return []*Box{box}
}

// handleCol is handle_col in weasyprint/html.py. It handles the attribute
// span of a <col> element.
func handleCol(_ *builder, _ *Element, box *Box, _ string) []*Box {
	if box.Kind == KindTableColumn {
		if span := box.Span(); span > 1 {
			// Generate multiple boxes.
			// https://lists.w3.org/Archives/Public/www-style/2011Nov/0293.html
			boxes := make([]*Box, span)
			for i := range boxes {
				boxes[i] = box.Copy()
			}
			return boxes
		}
	}
	return []*Box{box}
}

// handleSVG is handle_svg in weasyprint/html.py. It handles an <svg>
// element, and returns an image.
//
// Port: upstream returns no box when the SVG cannot be loaded. The port
// keeps the tree of the element, and okage parses it when it is drawn.
func handleSVG(b *builder, element *Element, box *Box, baseURL string) []*Box {
	// TODO: handle href base for inline svg tags.
	image := newInlineSVGImage(element, baseURL, b.fetcher, b.limits)
	return []*Box{makeReplacedBox(element, box, image)}
}

// Metadata is the dictionary of get_html_metadata in weasyprint/html.py: the
// metadata of a document.
type Metadata struct {
	// Title is the text of the first <title>, and HasTitle is false when the
	// document has none.
	Title    string
	HasTitle bool
	// Description and Generator are the first <meta> of their name, or "".
	Description string
	Generator   string
	// Keywords and Authors are the values of the <meta> of their name.
	Keywords []string
	Authors  []string
	// Created and Modified are the dates of dcterms.created and
	// dcterms.modified, or "".
	Created  string
	Modified string
	// Attachments are the URLs of the <link rel="attachment">, with their
	// titles. Upstream fetches them, and the port keeps the URLs.
	Attachments []Attachment
	// Lang is the attribute lang of the root element, or "".
	Lang string
	// Custom holds the other <meta>, by name.
	Custom map[string]string
}

// Attachment is an attachment of the metadata: a URL and its description.
type Attachment struct {
	URL         string
	Description string
}

// GetHTMLMetadata is get_html_metadata in weasyprint/html.py. It returns the
// metadata of a document. See https://www.whatwg.org/html#the-title-element,
// https://www.whatwg.org/html#standard-metadata-names,
// https://wiki.whatwg.org/wiki/MetaExtensions and
// https://microformats.org/wiki/existing-rel-values#link_type_extensions.
func GetHTMLMetadata(html *HTML) Metadata {
	m := Metadata{Custom: map[string]string{}}
	m.Lang = html.EtreeElement.GetDefault("lang", "")
	selectors, _ := cssselect2.CompileSelectorList("title, meta, link", nil)
	createdSet, modifiedSet, descriptionSet, generatorSet := false, false, false, false
	for wrapper := range html.WrapperElement.QueryAll(selectors...) {
		element := wrapperElement(wrapper)
		switch {
		case element.Tag == "title" && !m.HasTitle:
			m.Title, m.HasTitle = getChildText(element), true
		case element.Tag == "meta":
			name := asciiLower(element.GetDefault("name", ""))
			content := element.GetDefault("content", "")
			switch name {
			case "keywords":
				for keyword := range strings.SplitSeq(content, ",") {
					keyword = strings.Trim(keyword, whitespace)
					if !slices.Contains(m.Keywords, keyword) {
						m.Keywords = append(m.Keywords, keyword)
					}
				}
			case "author":
				m.Authors = append(m.Authors, content)
			case "description":
				if !descriptionSet {
					m.Description, descriptionSet = content, true
				}
			case "generator":
				if !generatorSet {
					m.Generator, generatorSet = content, true
				}
			case "dcterms.created":
				if !createdSet {
					m.Created, createdSet = parseW3CDate(html.log, name, content)
				}
			case "dcterms.modified":
				if !modifiedSet {
					m.Modified, modifiedSet = parseW3CDate(html.log, name, content)
				}
			default:
				if _, ok := m.Custom[name]; name != "" && !ok {
					m.Custom[name] = content
				}
			}
		case element.Tag == "link" && elementHasLinkType(element, "attachment"):
			u, ok := parseURL(html.log, element.GetDefault("href", ""), html.BaseURL, false)
			if !ok {
				html.log.error(`Missing href in <link rel="attachment">`)
				continue
			}
			m.Attachments = append(m.Attachments, Attachment{URL: u, Description: element.GetDefault("title", "")})
		}
	}
	return m
}

// w3cDateRE is W3C_DATE_RE in weasyprint/html.py. It matches:
//
//	YYYY (eg 1997)
//	YYYY-MM (eg 1997-07)
//	YYYY-MM-DD (eg 1997-07-16)
//	YYYY-MM-DDThh:mmTZD (eg 1997-07-16T19:20+01:00)
//	YYYY-MM-DDThh:mm:ssTZD (eg 1997-07-16T19:20:30+01:00)
//	YYYY-MM-DDThh:mm:ss.sTZD (eg 1997-07-16T19:20:30.45+01:00)
var w3cDateRE = regexp.MustCompile(`^[ \t\n\f\r]*` +
	`(?P<year>\d\d\d\d)` +
	`(?:-(?P<month>0\d|1[012])` +
	`(?:-(?P<day>[012]\d|3[01])` +
	`(?:T(?P<hour>[01]\d|2[0-3]):(?P<minute>[0-5]\d)` +
	`(?::(?P<second>[0-5]\d)(?:\.\d+)?)?` +
	`(?:Z|(?P<tz_hour>[+-](?:[01]\d|2[0-3])):(?P<tz_minute>[0-5]\d)))?)?)?` +
	`[ \t\n\f\r]*$`)

// parseW3CDate is parse_w3c_date in weasyprint/html.py. It returns the
// string of a date as the W3C defines it, and false with a warning for any
// other string. See https://www.w3.org/TR/NOTE-datetime.
func parseW3CDate(log *logger, metaName, s string) (string, bool) {
	if w3cDateRE.MatchString(s) {
		return s, true
	}
	log.warning(`Invalid date in <meta name="%s"> %q`, metaName, s)
	return "", false
}
