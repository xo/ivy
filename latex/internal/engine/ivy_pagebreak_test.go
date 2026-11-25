package engine

import (
	"slices"
	"strings"
	"testing"
)

// TestIvyClubPenalty makes sure that the page does not break after the
// first line of a paragraph when a break before the paragraph costs less
// than \clubpenalty, as in TeX.
func TestIvyClubPenalty(t *testing.T) {
	t.Parallel()
	doc := ivyCompile(t, `\hsize=100pt \vsize=36pt \parskip=0pt plus 20pt
Alpha\par Bravo\par
Zulu one two three four five six seven eight nine ten eleven twelve
thirteen fourteen fifteen sixteen.\par\bye`, goFaces)
	first := strings.Join(drawPage(t, doc, 0).text, " ")
	if !strings.Contains(first, "Bravo") || strings.Contains(first, "Zulu") {
		t.Errorf("expected the paragraph of Zulu on the next page, got on the first page: %q", first)
	}
}

// TestIvyPageTop makes sure that the glue after the style of a page, at the
// top of the page, is discarded, and that the style is kept.
func TestIvyPageTop(t *testing.T) {
	t.Parallel()
	box := &boxNode{}
	fil := glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
	list := []node{pageStyleNode{style: "empty"}, fil, penaltyNode{penalty: 0}, box}
	i := pageTop(list, 0)
	if i != 2 || list[i] != node(pageStyleNode{style: "empty"}) || list[3] != node(box) {
		t.Errorf("expected the page to start at the style before the box, got: %d %v", i, list)
	}
}

// TestIvyWithLastFil makes sure that the \vfil of \newpage stays at the end
// of a page, and that other glue does not.
func TestIvyWithLastFil(t *testing.T) {
	t.Parallel()
	box := &boxNode{}
	fil := glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
	got := withLastFil([]node{box, fil}, []node{box})
	if !slices.Equal(got, []node{box, fil}) {
		t.Errorf("expected the box and the fil, got: %v", got)
	}
	skip := glueNode{spec: glueSpec{width: unity}}
	if got := withLastFil([]node{box, skip}, []node{box}); len(got) != 1 {
		t.Errorf("expected the box alone, got: %v", got)
	}
}

// TestIvyLastVisible makes sure that \lastskip finds the glue before the
// style of a page.
func TestIvyLastVisible(t *testing.T) {
	t.Parallel()
	list := []node{glueNode{spec: glueSpec{width: unity}}, pageStyleNode{style: "plain"}, pageCounterNode{value: 1}}
	if n := lastVisible(list); n != 1 {
		t.Errorf("expected 1, got: %d", n)
	}
}
