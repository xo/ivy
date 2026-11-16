package xps

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
	"github.com/xo/ivy/internal/zipdoc"
	"github.com/xo/okage/roxmltree"
)

// The suffixes of the types of the relationships that the package reads. XPS
// and OpenXPS use other prefixes for the same types.
const (
	relFixedRepresentation = "/fixedrepresentation"
	relThumbnail           = "/metadata/thumbnail"
	relCoreProperties      = "/metadata/core-properties"
)

// pkg is the package of parts of an XPS file, a zip archive.
type pkg struct {
	archive *zipdoc.Archive
	lim     ivy.Limits
}

// partName returns the name of the entry of the archive for the part name,
// without the leading slash and with its escapes decoded.
func partName(name string) string {
	name = strings.TrimPrefix(name, "/")
	if unescaped, err := url.PathUnescape(name); err == nil {
		name = unescaped
	}
	return name
}

// resolve returns the part name that ref names, relative to the part base. A
// fragment of ref is dropped.
func resolve(base, ref string) string {
	ref, _, _ = strings.Cut(strings.TrimSpace(ref), "#")
	if strings.HasPrefix(ref, "/") {
		return path.Clean(ref)
	}
	return path.Clean(path.Join("/", path.Dir(base), ref))
}

// read reads the part name. A part can be split into pieces, as
// name/[0].piece, name/[1].piece and name/[2].last.piece.
func (p *pkg) read(name string) ([]byte, error) {
	if f := p.archive.Find(partName(name)); f != nil {
		return p.archive.Read(f)
	}
	prefix := strings.ToLower(partName(name)) + "/["
	type piece struct {
		n    int
		data []byte
	}
	var pieces []piece
	var total int64
	for _, f := range p.archive.Files() {
		lower := strings.ToLower(f.Name)
		rest, ok := strings.CutPrefix(lower, prefix)
		if !ok {
			continue
		}
		num, _, ok := strings.Cut(rest, "]")
		n, err := strconv.Atoi(num)
		if !ok || err != nil {
			continue
		}
		data, err := p.archive.Read(f)
		if err != nil {
			return nil, err
		}
		if total += int64(len(data)); total > p.lim.MaxMemory {
			return nil, fmt.Errorf("part %s of more than %d bytes: %w", name, p.lim.MaxMemory, ivy.ErrLimit)
		}
		pieces = append(pieces, piece{n, data})
	}
	if len(pieces) == 0 {
		return nil, fmt.Errorf("part %s is missing: %w", name, ivy.ErrFormat)
	}
	slices.SortFunc(pieces, func(a, b piece) int { return a.n - b.n })
	var buf bytes.Buffer
	for _, pc := range pieces {
		buf.Write(pc.data)
	}
	return buf.Bytes(), nil
}

// xml reads the part name and parses it as XML.
func (p *pkg) xml(name string) (*roxmltree.Document, error) {
	buf, err := p.read(name)
	if err != nil {
		return nil, err
	}
	doc, err := roxmltree.Parse(decodeText(buf))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w: %w", name, ivy.ErrFormat, err)
	}
	return doc, nil
}

// decodeText returns the text of an XML part as UTF-8. A part can be UTF-8 or
// UTF-16, with or without a byte order mark.
func decodeText(buf []byte) string {
	switch {
	case bytes.HasPrefix(buf, []byte{0xef, 0xbb, 0xbf}):
		return string(buf[3:])
	case bytes.HasPrefix(buf, []byte{0xff, 0xfe}):
		return utf16Text(buf[2:], false)
	case bytes.HasPrefix(buf, []byte{0xfe, 0xff}):
		return utf16Text(buf[2:], true)
	case len(buf) >= 2 && buf[0] == '<' && buf[1] == 0:
		return utf16Text(buf, false)
	case len(buf) >= 2 && buf[0] == 0 && buf[1] == '<':
		return utf16Text(buf, true)
	}
	if !utf8.Valid(buf) {
		return strings.ToValidUTF8(string(buf), "�")
	}
	return string(buf)
}

// utf16Text decodes UTF-16 text.
func utf16Text(buf []byte, bigEndian bool) string {
	units := make([]uint16, len(buf)/2)
	for i := range units {
		if bigEndian {
			units[i] = uint16(buf[2*i])<<8 | uint16(buf[2*i+1])
		} else {
			units[i] = uint16(buf[2*i+1])<<8 | uint16(buf[2*i])
		}
	}
	return string(utf16.Decode(units))
}

// rel is one relationship of a part.
type rel struct {
	typ, target string
}

// rels reads the relationships of the part name, from _rels/<base>.rels next
// to it. The package itself is the part "/". A part without relationships has
// none, and that is no error.
func (p *pkg) rels(name string) []rel {
	relsName := "/_rels/.rels"
	if name != "/" {
		relsName = path.Join(path.Dir(name), "_rels", path.Base(name)+".rels")
	}
	doc, err := p.xml(relsName)
	if err != nil {
		return nil
	}
	var out []rel
	for n := range doc.Descendants() {
		if !n.IsElement() || local(n) != "Relationship" {
			continue
		}
		if attr(n, "TargetMode") == "External" {
			continue
		}
		out = append(out, rel{typ: attr(n, "Type"), target: resolve(name, attr(n, "Target"))})
	}
	return out
}

// find returns the target of the first relationship whose type ends in
// suffix, and false if there is none.
func find(rels []rel, suffix string) (string, bool) {
	for _, r := range rels {
		if strings.HasSuffix(strings.ToLower(r.typ), suffix) {
			return r.target, true
		}
	}
	return "", false
}

// image decodes the image part name. JPEG XR, which XPS allows, is not
// supported.
func (p *pkg) image(ctx context.Context, name string) (image.Image, error) {
	switch strings.ToLower(path.Ext(name)) {
	case ".wdp", ".jxr", ".hdp":
		return nil, fmt.Errorf("image %s in jpeg xr: %w", name, ivy.ErrUnsupported)
	}
	buf, err := p.read(name)
	if err != nil {
		return nil, err
	}
	img, err := raster.Decode(ctx, buf, p.lim)
	if err != nil {
		return nil, fmt.Errorf("image %s: %w", name, err)
	}
	return img, nil
}

// local returns the local name of the element n, without its namespace.
func local(n roxmltree.Node) string {
	return n.TagName().Name()
}

// attr returns the value of the attribute of n with the local name name, in
// any namespace, or the empty string.
func attr(n roxmltree.Node, name string) string {
	for a := range n.Attributes() {
		if a.Name() == name {
			return a.Value()
		}
	}
	return ""
}

// hasAttr reports whether n has an attribute with the local name name.
func hasAttr(n roxmltree.Node, name string) bool {
	for a := range n.Attributes() {
		if a.Name() == name {
			return true
		}
	}
	return false
}

// elements iterates over the child elements of n.
func elements(n roxmltree.Node) []roxmltree.Node {
	var out []roxmltree.Node
	for c := range n.Children() {
		if c.IsElement() {
			out = append(out, c)
		}
	}
	return out
}

// property returns the first element child of the property element
// <owner.name> of n, such as the brush of <Path.Fill>.
func property(n roxmltree.Node, name string) (roxmltree.Node, bool) {
	want := local(n) + "." + name
	for _, c := range elements(n) {
		if local(c) == want {
			for _, v := range elements(c) {
				return v, true
			}
		}
	}
	return roxmltree.Node{}, false
}
