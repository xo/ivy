// This file of ivy holds the changes of ivy to the vertical lists and the
// pages of the engine, so that they follow TeX and LaTeX (D62): the
// penalties between the lines of a paragraph and around a display, the top
// and the end of a page, the footnotes, and the space of the contents. The
// engine put no penalty between two lines, so a page broke after the first
// line of a paragraph or before its last line as easily as between two
// paragraphs.

package engine

// linePenalty appends the penalty that TeX puts after line i of a paragraph
// part of n lines, as post_line_break does (tex.web §890):
//
//	pen:=inter_line_penalty;
//	if cur_line=prev_graf+1 then pen:=pen+club_penalty;
//	if cur_line+2=best_line then pen:=pen+widow_penalty;
//	if disc_break then pen:=pen+broken_penalty;
//	if pen<>0 then append the penalty
//
// broken is true when the line ends at a discretionary. The last line has
// no penalty after it. linePenalty keeps the place of the penalty before
// the last line, so that a display that follows can change the widow
// penalty to \displaywidowpenalty, as TeX does for the lines before a
// display.
func (e *Engine) linePenalty(i, n int, broken bool) {
	if i >= n-1 {
		return
	}
	pen := e.namedInt("interlinepenalty")
	if i == 0 {
		pen += e.namedInt("clubpenalty")
	}
	widow := i == n-2
	if widow {
		pen += e.namedInt("widowpenalty")
	}
	if broken {
		pen += e.namedInt("brokenpenalty")
	}
	if pen == 0 {
		return
	}
	if widow {
		e.ivy.widowAt = len(e.mvl)
	}
	e.mvl = append(e.mvl, penaltyNode{penalty: pen})
}

// markLines starts the lines of a paragraph part: no widow penalty is kept.
func (e *Engine) markLines() {
	e.ivy.widowAt = -1
}

// endLines ends the lines of a paragraph part, and keeps the length of the
// vertical list, so that a display knows that the lines come right before
// it.
func (e *Engine) endLines() {
	e.ivy.linesEnd = len(e.mvl)
}

// displayWidow changes the widow penalty of the lines right before a
// display to \displaywidowpenalty.
func (e *Engine) displayWidow() {
	at := e.ivy.widowAt
	if at < 0 || e.ivy.linesEnd != len(e.mvl) || at >= len(e.mvl) {
		return
	}
	p, ok := e.mvl[at].(penaltyNode)
	if !ok {
		return
	}
	p.penalty += e.namedInt("displaywidowpenalty") - e.namedInt("widowpenalty")
	e.mvl[at] = p
	e.ivy.widowAt = -1
}

// lastVisible returns the length of list without the nodes at its end
// that the engine adds and TeX does not have: the style and the number of
// the page that \thispagestyle and \setcounter{page} leave on the list.
// \lastskip and \addvspace look past them, as TeX finds the glue before
// them. \maketitle ends with \vskip 1.5em and \thispagestyle{plain}, and
// the \section after it added its skip to the 1.5em where LaTeX takes the
// larger one.
func lastVisible(list []node) int {
	n := len(list)
	for n > 0 {
		switch list[n-1].(type) {
		case pageStyleNode, pageCounterNode:
			n--
		default:
			return n
		}
	}
	return n
}

// pageTop returns the start of the page that begins at i, past the glue,
// the kerns and the penalties that TeX discards at the top of a page. TeX
// discards them while the page holds no box, also after a mark or a
// whatsit (tex.web §1000). The engine stopped at the first node that is
// not glue, such as the style of the page that \thispagestyle leaves, and
// kept the \vfil after it: a titlepage that opens with \vfil was centered
// where LaTeX sets it at the top. pageTop moves such glue before the nodes
// that TeX keeps, and skips it.
func pageTop(list []node, i int) int {
	i = skipDiscardable(list, i)
	for {
		j := i
		for j < len(list) && pageInert(list[j]) {
			j++
		}
		if j == i || j >= len(list) {
			return i
		}
		k := skipDiscardable(list, j)
		if k == j {
			return i
		}
		moved := make([]node, 0, k-i)
		moved = append(append(moved, list[j:k]...), list[i:j]...)
		copy(list[i:k], moved)
		i += k - j
	}
}

// pageInert reports whether n is a node that does not end the top of a page
// for TeX: a mark, a whatsit, or the style and the number of the page.
func pageInert(n node) bool {
	switch n.(type) {
	case markNode, specialNode, pageStyleNode, pageCounterNode:
		return true
	}
	return false
}

// footnoteStruts puts the struts of \@footnotetext around the text of a
// note (latex.ltx):
//
//	\@makefntext{\rule\z@\footnotesep\ignorespaces#1\@finalstrut\strutbox}
//
// The first line of a note is then at least \footnotesep high, and its last
// line as deep as the strut of \footnotesize. They space the notes, and the
// first note from the text above. The macros are in ivy_latex.go, and a
// document of plain TeX, which has its own \footstrut, has none.
func (e *Engine) footnoteStruts(text []tok) []tok {
	if e.eq["gotex@ivyfnstrut"] == nil {
		return text
	}
	out := make([]tok, 0, len(text)+2)
	out = append(out, csTok("gotex@ivyfnstrut"))
	out = append(out, text...)
	return append(out, csTok("gotex@ivyfinalstrut"))
}

// footinsGlue is \skip\footins, with its stretch and its shrink.
func (e *Engine) footinsGlue() glueSpec {
	if m := e.eq["footins"]; m != nil && m.kind == mCountRef {
		if g := e.getSkip(e.getCount(m.code)); g.width > 0 {
			return g
		}
	}
	return glueSpec{width: e.footinsSkip()}
}

// backOutDepth ends the text block of a page with the glue
// \vskip -\@outputbox@depth that \@make@normalcolbox puts before
// \@textbottom (ltoutput.dtx): the depth of the last box, at most
// \maxdepth, hangs below the block and is not inside it. Without it, the
// last footnote of a page that \newpage ends stood its depth too high.
func (e *Engine) backOutDepth(vlist []node) []node {
	if len(vlist) == 0 {
		return vlist
	}
	_, d, kind := vHeightDepth(vlist[len(vlist)-1])
	if kind != vBoxLike || d <= 0 {
		return vlist
	}
	if m := e.pageMaxDepth(); m > 0 && d > m {
		d = m
	}
	return append(vlist, glueNode{spec: glueSpec{width: -d}})
}

// tocVskip reports whether the space above the entry en of the contents is
// a \vskip, as \l@chapter of report.cls and book.cls has it, and not the
// \addvspace of \l@part and of \l@section of article.cls.
func tocVskip(en tocEntry) bool {
	return en.kind == "toc" && tocLevelName[en.level] == "chapter"
}

// withHeadBand puts the band of the running head above out, the text block
// of a page of the style empty, as for a page of the style plain. The page
// is lifted by the band of the document (headBand), and an empty page
// without the band rose by \headheight and \headsep: the title page of a
// report with fancyhdr stood 39pt above the place of pdfLaTeX.
func (e *Engine) withHeadBand(out *boxNode) *boxNode {
	if e.headBand() <= 0 {
		return out
	}
	return vpackSP([]node{
		e.headToHeadheight(hpackSP(nil, packNatural, 0)),
		glueNode{spec: glueSpec{width: e.classDimen("headsep", 25*unity)}},
		out,
	}, packNatural, 0)
}

// withLastFil returns page, the material of a page without the glue at its
// end, with the last node of material put back when it is glue of infinite
// stretch, such as the \vfil of \newpage. LaTeX takes that glue off the
// column and puts it back before the footnotes (\@outputbox@removebskip of
// ltoutput.dtx), so the footnotes of a page that \newpage ends stand at the
// bottom, and the \vfil of \newpage shares the space of a title page with
// the two \vfil of titlepage. The engine dropped it with the rest of the
// glue.
func withLastFil(material, page []node) []node {
	if len(material) == 0 || len(page) == len(material) {
		return page
	}
	g, ok := material[len(material)-1].(glueNode)
	if !ok || g.spec.stretchOrder == 0 || g.spec.stretch == 0 {
		return page
	}
	out := make([]node, len(page), len(page)+1)
	copy(out, page)
	return append(out, g)
}
