package review

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/ivy"
)

// Paper is a paper of the review for a document whose file gives none, such
// as a text file, an HTML file, a CSV file or an ebook (D32). ivy gets it as
// ivy.Config.Paper, and each reference program gets the same paper.
type Paper struct {
	// ID names the paper in the flag -papers, such as "a4-landscape".
	ID string `json:"id"`
	// Name names the paper in the report, such as "A4 landscape".
	Name string `json:"name"`
	// Width and Height are the size of the paper in points. They are 0 for
	// the paper "default", the default of the format.
	Width  float32 `json:"width"`
	Height float32 `json:"height"`
}

// Size returns the paper as an ivy.Paper. The paper "default" gives the zero
// ivy.Paper.
func (p Paper) Size() ivy.Paper {
	return ivy.Paper{Width: p.Width, Height: p.Height}
}

// IsDefault reports whether p is the paper "default", which keeps the
// default of the format.
func (p Paper) IsDefault() bool {
	return p.Width <= 0 || p.Height <= 0
}

// DefaultPapers is the default of the flag -papers (D32).
const DefaultPapers = "letter,a4,a4-landscape"

// basePapers are the papers that ParsePapers knows, in portrait.
var basePapers = map[string]struct {
	name string
	size ivy.Paper
}{
	"letter": {"Letter", ivy.Letter},
	"legal":  {"Legal", ivy.Legal},
	"a4":     {"A4", ivy.A4},
	"a5":     {"A5", ivy.A5},
}

// ParsePapers reads a comma-separated list of papers, such as
// "letter,a4,a4-landscape". A paper is letter, legal, a4 or a5, with
// "-landscape" for the paper in landscape, or "default", the default of the
// format. An empty list gives no paper: each document renders once, on the
// default of its format.
func ParsePapers(list string) ([]Paper, error) {
	var papers []Paper
	seen := map[string]bool{}
	for id := range strings.SplitSeq(list, ",") {
		id = strings.TrimSpace(strings.ToLower(id))
		if id == "" {
			continue
		}
		if seen[id] {
			return nil, fmt.Errorf("the paper %q is twice in the list", id)
		}
		seen[id] = true
		if id == "default" {
			papers = append(papers, Paper{ID: id, Name: "default"})
			continue
		}
		base, landscape := strings.CutSuffix(id, "-landscape")
		b, ok := basePapers[base]
		if !ok {
			return nil, fmt.Errorf("expected letter, legal, a4, a5, one of them with -landscape, or default, got: %q", id)
		}
		p := Paper{ID: id, Name: b.name, Width: b.size.Width, Height: b.size.Height}
		if landscape {
			l := b.size.Landscape()
			p.Name += " landscape"
			p.Width, p.Height = l.Width, l.Height
		}
		papers = append(papers, p)
	}
	return papers, nil
}

// csvKinds are the kinds of CSV files. LibreOffice converts them, with the
// page of ivy (D32).
var csvKinds = []string{"csv", "tsv", "tab"}

// paperless reports whether the files of kind give no paper, so that the
// review renders them on each paper of -papers (D32): the kinds of calibre,
// of markdown-it and of WeasyPrint, and the CSV files.
func paperless(kind string) bool {
	r := routeFor(kind)
	return r == calibreRoute || r == markdownItRoute || r == weasyprintRoute || contains(csvKinds, kind)
}

// isLetter reports whether p is the paper "default" or US Letter in
// portrait. A program outside the review, such as Microsoft Office, makes
// its PDF file on such a paper.
func isLetter(p *Paper) bool {
	return p == nil || p.IsDefault() || p.Size() == ivy.Letter
}

// points returns v as a number of points for a command line, such as
// "595.28".
func points(v float32) string {
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}
