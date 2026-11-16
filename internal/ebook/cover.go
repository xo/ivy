package ebook

import (
	"context"
	"iter"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/zipdoc"
)

// CoverNames are the names of a cover image in the root of an HTMLZ or a
// TXTZ file whose package names no cover.
var CoverNames = []string{"cover.jpg", "cover.jpeg", "cover.png", "cover.gif"}

// CoverPreviews lists the cover image of an HTMLZ or a TXTZ file: the first
// of the covers that pkg names and of CoverNames that the archive holds as
// an image. pkg can be nil.
func CoverPreviews(ctx context.Context, archive *zipdoc.Archive, pkg *Package) iter.Seq2[ivy.Preview, error] {
	var names []string
	if pkg != nil {
		names = pkg.Covers()
	}
	names = append(names, CoverNames...)
	candidates := make([]zipdoc.Candidate, 0, len(names))
	for _, name := range names {
		candidates = append(candidates, zipdoc.Candidate{Kind: ivy.Cover, Name: name})
	}
	return func(yield func(ivy.Preview, error) bool) {
		// the archive lists the candidates that it holds, and the first wins
		for preview, err := range archive.Previews(ctx, candidates...) {
			yield(preview, err)
			return
		}
	}
}
