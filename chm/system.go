package chm

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/xo/ivy/internal/ebook"
)

// The codes of the entries of #SYSTEM that the package reads.
const (
	systemContents = 0
	systemDefault  = 2
	systemTitle    = 3
	systemLCID     = 4
)

// The offsets in an entry of #WINDOWS of the strings that the package
// reads, as offsets in #STRINGS: the title, the table of contents and the
// default topic.
const (
	windowTitle    = 0x14
	windowContents = 0x60
	windowDefault  = 0x68
	// windowMinSize is the smallest entry that holds the three.
	windowMinSize = windowDefault + 4
)

// system is what #SYSTEM and #WINDOWS give: the title, the table of
// contents, the default topic and the LCID of the help file. The strings
// are in the code page of the LCID.
type system struct {
	title, contents, topic []byte
	lcid                   uint32
	hasLCID                bool
}

// readSystem reads #SYSTEM. It starts with its version in 4 bytes, then
// holds entries to its end. Each entry is its code and the length of its
// data, in 2 bytes each, then its data. A string ends with a NUL. Then it
// reads the first entry of #WINDOWS, for the strings that #SYSTEM does not
// give.
func (a *archive) readSystem() system {
	var s system
	if buf, err := a.read("/#SYSTEM"); err == nil {
		for p := 4; p+4 <= len(buf); {
			code, n := binary.LittleEndian.Uint16(buf[p:]), int(binary.LittleEndian.Uint16(buf[p+2:]))
			p += 4
			if n > len(buf)-p {
				break
			}
			data := buf[p : p+n]
			p += n
			switch code {
			case systemContents:
				s.contents = cString(data)
			case systemDefault:
				s.topic = cString(data)
			case systemTitle:
				s.title = cString(data)
			case systemLCID:
				if len(data) >= 4 {
					s.lcid, s.hasLCID = binary.LittleEndian.Uint32(data), true
				}
			}
		}
	}
	if len(s.title) > 0 && len(s.contents) > 0 && len(s.topic) > 0 {
		return s
	}
	windows, err := a.read("/#WINDOWS")
	if err != nil || len(windows) < 8 {
		return s
	}
	count, size := binary.LittleEndian.Uint32(windows), int(binary.LittleEndian.Uint32(windows[4:]))
	if count == 0 || size < windowMinSize || size > len(windows)-8 {
		return s
	}
	strs, err := a.read("/#STRINGS")
	if err != nil {
		return s
	}
	window := windows[8 : 8+size]
	str := func(off int) []byte {
		at := int64(binary.LittleEndian.Uint32(window[off:]))
		if at <= 0 || at >= int64(len(strs)) {
			return nil
		}
		return cString(strs[at:])
	}
	if len(s.title) == 0 {
		s.title = str(windowTitle)
	}
	if len(s.contents) == 0 {
		s.contents = str(windowContents)
	}
	if len(s.topic) == 0 {
		s.topic = str(windowDefault)
	}
	return s
}

// cString returns the bytes of b up to its first NUL.
func cString(b []byte) []byte {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return b[:i]
	}
	return b
}

// contentsOrder returns the links of the local files of a table of
// contents, in its order, resolved from the folder dir. A table of contents
// is a sitemap of HTML: each entry is an object element whose param element
// with the name Local gives the file. Text that is not UTF-8 is in the
// encoding charset.
func contentsOrder(hhc []byte, dir, charset string) []string {
	var out []string
	for tag := range ebook.Tags(ebook.TextIn(hhc, charset)) {
		if tag.Name != "param" {
			continue
		}
		name, _ := tag.Attr("name")
		if !strings.EqualFold(strings.TrimSpace(name), "local") {
			continue
		}
		value, _ := tag.Attr("value")
		if name, ok := resolveLink(dir, value); ok {
			out = append(out, name)
		}
	}
	return out
}

// resolveLink returns the path of a link of a help file, relative to the
// folder dir. A link can name a file of a help file, as
// mk:@MSITStore:name.chm::/a.htm or ms-its:name.chm::/a.htm, and the part
// after "::" is the path in that file.
func resolveLink(dir, link string) (string, bool) {
	if i := strings.LastIndex(link, "::"); i >= 0 {
		link = link[i+2:]
	}
	return ebook.Resolve(dir, link)
}
