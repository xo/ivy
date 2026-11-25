package markdown

import (
	"html"
	"strings"
)

// The HTML of a Markdown text goes through the sanitizer of GitHub, as the
// html-pipeline of GitHub does for a README: only the elements and the
// attributes of its allow list stay. The sanitizer works on the raw HTML of
// the text alone. The HTML that the renderer makes, such as the box of a
// task list, is not raw HTML and stays as it is.
//
// The renderer writes the NUL byte before and after each piece of raw HTML.
// goldmark replaces each NUL byte of the text by U+FFFD, so the marks are
// the only ones in its output. sanitize removes them.

// rawMark is the byte that the renderer writes before and after raw HTML.
const rawMark = "\x00"

// allowedTags are the elements that GitHub allows.
var allowedTags = toSet(`h1 h2 h3 h4 h5 h6 br b i strong em a pre code img tt div ins del sup sub p picture
ol ul table thead tbody tfoot blockquote dl dt dd kbd q samp var hr ruby rt rp li tr td th s strike summary
details caption figure figcaption abbr bdo cite dfn mark small source span time wbr`)

// allowedAttrs are the attributes that GitHub allows on every element.
var allowedAttrs = toSet(`abbr accept accept-charset accesskey action align alt aria-describedby aria-hidden
aria-label aria-labelledby axis border char charoff charset checked clear cols colspan compact coords datetime
dir disabled enctype for frame headers height hreflang hspace id ismap label lang maxlength media method
multiple name nohref noshade nowrap open progress prompt readonly rel rev role rows rowspan rules scope
selected shape size span start summary tabindex title type usemap valign value width itemprop`)

// tagAttrs are the attributes that GitHub allows on one element.
var tagAttrs = map[string]map[string]bool{
	"a":          toSet("href"),
	"img":        toSet("src longdesc loading alt"),
	"div":        toSet("itemscope itemtype"),
	"blockquote": toSet("cite"),
	"del":        toSet("cite"),
	"ins":        toSet("cite"),
	"q":          toSet("cite"),
	"source":     toSet("srcset"),
}

// dropContents are the elements that the sanitizer drops with their
// content. It keeps the content of any other element that it drops.
var dropContents = toSet("script style iframe noscript noembed noframes xmp plaintext")

// linkSchemes are the schemes that GitHub allows in the href of a link.
var linkSchemes = toSet("http https mailto github-windows github-mac")

// srcSchemes are the schemes that GitHub allows in the URL of an image. The
// scheme data stays, as markdown-it shows it, and the review compares ivy
// with it.
var srcSchemes = toSet("http https data")

func toSet(s string) map[string]bool {
	m := make(map[string]bool)
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// sanitize returns s, the HTML that the renderer wrote, without the marks of
// the raw HTML and with the raw HTML cleaned.
func sanitize(s string) string {
	if !strings.Contains(s, rawMark) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	var skip string
	raw := false
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == 0:
			raw = !raw
			i++
		case c != '<':
			j := strings.IndexAny(s[i:], "<\x00")
			if j < 0 {
				j = len(s) - i
			}
			if skip == "" {
				b.WriteString(s[i : i+j])
			}
			i += j
		case !raw:
			// a tag that the renderer made
			j := strings.IndexByte(s[i:], '>')
			if j < 0 {
				j = len(s) - i - 1
			}
			tag := s[i : i+j+1]
			i += j + 1
			if skip != "" {
				if endsBlock(tag) {
					skip = ""
				}
				continue
			}
			b.WriteString(tag)
		default:
			end := rawEnd(s[i:])
			t, ok := parseTag(s[i : i+end])
			if !ok {
				// not a tag, so text
				if skip == "" {
					b.WriteString("&lt;")
				}
				i++
				continue
			}
			i += t.n
			if skip != "" {
				if t.closing && t.name == skip {
					skip = ""
				}
				continue
			}
			if t.comment {
				continue
			}
			if !t.closing && dropContents[t.name] && !t.selfClosing {
				skip = t.name
				continue
			}
			if !allowedTags[t.name] {
				continue
			}
			b.WriteString(t.String())
		}
	}
	return b.String()
}

// endsBlock returns true for the closing tag of a block that the renderer
// made. It ends the content that an unclosed script or style would drop.
func endsBlock(tag string) bool {
	name, ok := strings.CutPrefix(tag, "</")
	if !ok {
		return false
	}
	name = strings.TrimSuffix(name, ">")
	switch name {
	case "p", "li", "td", "th", "dd", "dt", "blockquote", "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	}
	return false
}

// rawEnd returns the length of the piece of s that a tag in the raw HTML can
// span: up to the next mark.
func rawEnd(s string) int {
	if j := strings.IndexByte(s, 0); j >= 0 {
		return j
	}
	return len(s)
}

// attr is an attribute of a tag.
type attr struct {
	name, value string
	// has is true if the attribute has a value.
	has bool
}

// tag is a tag of the raw HTML.
type tag struct {
	name        string
	attrs       []attr
	closing     bool
	selfClosing bool
	comment     bool
	// n is the length of the tag in the text.
	n int
}

// parseTag parses the tag at the start of s, which starts with <. It returns
// false if s does not start with a tag, a comment, or another declaration.
// A comment, a processing instruction and a declaration return a tag whose
// comment is true.
func parseTag(s string) (tag, bool) {
	var t tag
	switch {
	case strings.HasPrefix(s, "<!--"):
		end := strings.Index(s[4:], "-->")
		if end < 0 {
			t.n = len(s)
		} else {
			t.n = 4 + end + 3
		}
		t.comment = true
		return t, true
	case strings.HasPrefix(s, "<?"), strings.HasPrefix(s, "<!"):
		end := strings.IndexByte(s, '>')
		if end < 0 {
			t.n = len(s)
		} else {
			t.n = end + 1
		}
		t.comment = true
		return t, true
	}
	i := 1
	if i < len(s) && s[i] == '/' {
		t.closing = true
		i++
	}
	start := i
	for i < len(s) && (isAlnum(s[i]) || s[i] == '-' || s[i] == ':') {
		i++
	}
	if i == start || !isAlpha(s[start]) {
		return t, false
	}
	t.name = strings.ToLower(s[start:i])
	for {
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if i >= len(s) {
			return tag{}, false
		}
		if s[i] == '>' {
			t.n = i + 1
			return t, true
		}
		if s[i] == '/' {
			t.selfClosing = true
			i++
			continue
		}
		start = i
		for i < len(s) && !isSpace(s[i]) && s[i] != '=' && s[i] != '>' && s[i] != '/' {
			i++
		}
		if i == start {
			i++
			continue
		}
		a := attr{name: strings.ToLower(s[start:i])}
		j := i
		for j < len(s) && isSpace(s[j]) {
			j++
		}
		if j < len(s) && s[j] == '=' {
			j++
			for j < len(s) && isSpace(s[j]) {
				j++
			}
			a.has = true
			switch {
			case j < len(s) && (s[j] == '"' || s[j] == '\''):
				q := s[j]
				end := strings.IndexByte(s[j+1:], q)
				if end < 0 {
					return tag{}, false
				}
				a.value = s[j+1 : j+1+end]
				j += end + 2
			default:
				start = j
				for j < len(s) && !isSpace(s[j]) && s[j] != '>' {
					j++
				}
				a.value = s[start:j]
			}
			i = j
		}
		t.attrs = append(t.attrs, a)
	}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

func isAlpha(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

func isAlnum(c byte) bool { return isAlpha(c) || (c >= '0' && c <= '9') }

// String returns the tag as the sanitizer writes it: with the attributes
// that GitHub allows, and the values of the URLs that it allows.
func (t tag) String() string {
	var b strings.Builder
	b.WriteByte('<')
	if t.closing {
		b.WriteByte('/')
		b.WriteString(t.name)
		b.WriteByte('>')
		return b.String()
	}
	b.WriteString(t.name)
	for _, a := range t.attrs {
		if !allowedAttrs[a.name] && !tagAttrs[t.name][a.name] {
			continue
		}
		if !urlAllowed(t.name, a) {
			continue
		}
		b.WriteByte(' ')
		b.WriteString(a.name)
		if a.has {
			b.WriteString(`="`)
			b.WriteString(strings.ReplaceAll(a.value, `"`, "&quot;"))
			b.WriteByte('"')
		}
	}
	if t.selfClosing {
		b.WriteString(" /")
	}
	b.WriteByte('>')
	return b.String()
}

// urlAllowed returns false if a is an attribute with a URL whose scheme
// GitHub does not allow. A URL without a scheme is relative, and stays.
func urlAllowed(tagName string, a attr) bool {
	var schemes map[string]bool
	switch {
	case tagName == "a" && a.name == "href":
		schemes = linkSchemes
	case tagName == "img" && (a.name == "src" || a.name == "longdesc"):
		schemes = srcSchemes
	case a.name == "cite":
		schemes = toSet("http https")
	default:
		return true
	}
	u := strings.Map(func(r rune) rune {
		if r <= ' ' || r == 0x7f {
			return -1
		}
		return r
	}, html.UnescapeString(a.value))
	i := strings.IndexAny(u, ":/?#")
	if i < 0 || u[i] != ':' {
		return true
	}
	return schemes[strings.ToLower(u[:i])]
}
