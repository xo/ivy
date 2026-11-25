package markdown

import (
	"encoding/base64"
	"strings"

	"github.com/xo/ivy/internal/goldmark/ast"
	"github.com/xo/ivy/internal/goldmark/text"
)

// The paths of the icons of the alerts are the octicons of GitHub at 16
// pixels: info, light-bulb, report, alert and stop. They are under this
// license:
//
//	MIT License
//
//	Copyright (c) 2026 GitHub Inc.
//
//	Permission is hereby granted, free of charge, to any person obtaining a
//	copy of this software and associated documentation files (the
//	"Software"), to deal in the Software without restriction, including
//	without limitation the rights to use, copy, modify, merge, publish,
//	distribute, sublicense, and/or sell copies of the Software, and to
//	permit persons to whom the Software is furnished to do so, subject to
//	the following conditions:
//
//	The above copyright notice and this permission notice shall be included
//	in all copies or substantial portions of the Software.
//
//	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
//	OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
//	MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
//	IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
//	CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
//	TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
//	SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
//
// alertIcons maps the kind of an alert to its color and the path of its
// icon.
var alertIcons = map[string][2]string{
	"note":      {"#0969da", "M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.5 7.75A.75.75 0 0 1 7.25 7h1a.75.75 0 0 1 .75.75v2.75h.25a.75.75 0 0 1 0 1.5h-2a.75.75 0 0 1 0-1.5h.25v-2h-.25a.75.75 0 0 1-.75-.75ZM8 6a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"},
	"tip":       {"#1a7f37", "M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 0 1-1.484.211c-.04-.282-.163-.547-.37-.847a8.456 8.456 0 0 0-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.751.751 0 0 1-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75ZM5.75 12h4.5a.75.75 0 0 1 0 1.5h-4.5a.75.75 0 0 1 0-1.5ZM6 15.25a.75.75 0 0 1 .75-.75h2.5a.75.75 0 0 1 0 1.5h-2.5a.75.75 0 0 1-.75-.75Z"},
	"important": {"#8250df", "M0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0 1 14.25 13H8.06l-2.573 2.573A1.458 1.458 0 0 1 3 14.543V13H1.75A1.75 1.75 0 0 1 0 11.25Zm1.75-.25a.25.25 0 0 0-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 0 1 .75.75v2.19l2.72-2.72a.749.749 0 0 1 .53-.22h6.5a.25.25 0 0 0 .25-.25v-9.5a.25.25 0 0 0-.25-.25Zm7 2.25v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"},
	"warning":   {"#9a6700", "M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0 1 14.082 15H1.918a1.75 1.75 0 0 1-1.543-2.575Zm1.763.707a.25.25 0 0 0-.44 0L1.698 13.132a.25.25 0 0 0 .22.368h12.164a.25.25 0 0 0 .22-.368Zm.53 3.996v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z"},
	"caution":   {"#d1242f", "M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25c.141.14.22.331.22.53v6a.749.749 0 0 1-.22.53l-4.25 4.25A.749.749 0 0 1 11 16H5a.749.749 0 0 1-.53-.22L.22 11.53A.749.749 0 0 1 0 11V5c0-.199.079-.389.22-.53Zm.84 1.28L1.5 5.31v5.38l3.81 3.81h5.38l3.81-3.81V5.31L10.69 1.5ZM8 4a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-3.5A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z"},
}

// alertIcon returns the octicon of the alert kind as an image, in the color
// of the kind.
func alertIcon(kind string) *ast.Image {
	icon := alertIcons[kind]
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16"><path fill="` +
		icon[0] + `" d="` + icon[1] + `"/></svg>`
	dest := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
	img := ast.NewImage(text.NewSingleLineValueFromString(dest, nil))
	img.SetAttribute("class", text.NewMultiLineValueFromString("octicon", nil))
	img.SetAttribute("width", text.NewMultiLineValueFromString("16", nil))
	img.SetAttribute("height", text.NewMultiLineValueFromString("16", nil))
	return img
}

// alertTitles are the kinds of the alerts of GitHub, in lower case, and
// their titles.
var alertTitles = map[string]string{
	"note":      "Note",
	"tip":       "Tip",
	"important": "Important",
	"warning":   "Warning",
	"caution":   "Caution",
}

// alerts makes an alert of GitHub of each block quote of doc whose first
// line is a marker such as "[!NOTE]". goldmark does not read alerts. As on
// GitHub, an alert is a block quote at the top level of the document, its
// marker is alone on its line, and text follows the marker. The block quote
// takes the classes "markdown-alert" and "markdown-alert-<kind>", and a
// first paragraph of the class "markdown-alert-title" with the title of the
// kind, such as "Note", in place of the marker. The title starts with the octicon of the
// kind, as GitHub draws it. The style sheet gives the colors of each kind.
func alerts(doc ast.Node, src []byte) {
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		q, ok := n.(*ast.Blockquote)
		if !ok {
			continue
		}
		p, ok := q.FirstChild().(*ast.Paragraph)
		if !ok {
			continue
		}
		lines := p.Source()
		if len(lines) == 0 {
			continue
		}
		first := lines[0]
		marker := strings.TrimSpace(first.Str(src))
		kind, ok := strings.CutPrefix(marker, "[!")
		if !ok {
			continue
		}
		kind, ok = strings.CutSuffix(kind, "]")
		title := alertTitles[strings.ToLower(kind)]
		if !ok || title == "" {
			continue
		}
		// the text of the alert follows the marker, in the same paragraph or
		// in a later block
		if len(lines) == 1 && p.NextSibling() == nil {
			continue
		}
		// the marker is the inline nodes of the first line
		for c := p.FirstChild(); c != nil; {
			next := c.NextSibling()
			if pos := c.Pos(); pos < 0 || pos >= first.Stop {
				break
			}
			p.RemoveChild(c)
			c = next
		}
		if p.FirstChild() == nil {
			q.RemoveChild(p)
		}
		h := ast.NewParagraph()
		h.SetAttribute("class", text.NewMultiLineValueFromString("markdown-alert-title", nil))
		h.AppendChild(alertIcon(strings.ToLower(kind)))
		h.AppendChild(ast.NewText(text.NewSingleLineValueFromString(title, nil)))
		if c := q.FirstChild(); c != nil {
			q.InsertBefore(c, h)
		} else {
			q.AppendChild(h)
		}
		q.SetAttribute("class", text.NewMultiLineValueFromString("markdown-alert markdown-alert-"+strings.ToLower(kind), nil))
	}
}
