package msole

import (
	"html"
	"strings"
)

// HTML returns an HTML document with the title, the language, the elements
// of the head, such as a style sheet, and the body, for a format that
// converts its text for the layout (D10).
func HTML(title, lang, head, body string) string {
	var b strings.Builder
	b.Grow(len(body) + 256)
	b.WriteString("<!DOCTYPE html>\n<html")
	if lang != "" {
		b.WriteString(` lang="` + html.EscapeString(lang) + `"`)
	}
	b.WriteString(">\n<head>\n<meta charset=\"utf-8\">\n<title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title>\n")
	b.WriteString(head)
	b.WriteString("</head>\n<body>\n")
	b.WriteString(body)
	b.WriteString("</body>\n</html>\n")
	return b.String()
}
