// This file of ivy holds the macros that ivy adds to the LaTeX of the
// engine, to set a document closer to pdfLaTeX (D62). Compile loads them
// after the macros of upstream, so that a new commit of upstream keeps
// them.

package engine

import "strings"

// ivyPlain is the format of the macros of ivy for plain TeX and LaTeX.
//
// \TeX is the logo of plain.tex, with the E lowered by half an ex.
const ivyPlain = `\def\TeX{T\kern-.1667em\lower.5ex\hbox{E}\kern-.125emX}
`

// ivyLaTeX is the format of the macros of ivy.
//
// The standard classes define quote, quotation and verse on \list, as
// article.cls does:
//
//	\newenvironment{quote}{\list{}{\rightmargin\leftmargin}\item\relax}{\endlist}
//
// The \list of the engine is a bare trivlist that drops its margins, so the
// three environments were set at the full width of the text. After the class
// loads, ivy sets them as \list does: \@listi gives \leftmargin, \parsep and
// \topsep, the second argument of \list changes them, and the margins go to
// \leftskip and \rightskip. A \\ of verse ends the line as \@centercr does,
// with no \parskip.
//
// \LaTeX is the logo of latex.ltx: an A in the size of a script, the size
// that \sf@size names, raised to the top of the T.
//
// \setcounter{page} and \pagenumbering put the number and the style of the
// page on the vertical list, where the page builder finds them. The engine
// numbered the pages in their order, so the page after a title page was 2
// where LaTeX gives 1, and the front matter of a book was not in roman.
//
// \ps@fancy sets the marks as fancyhdr does: \leftmark is the chapter, or
// the section in a class without chapters. The engine draws the fields of
// fancyhdr in Go, and its \leftmark stayed empty.
//
// The end of a proof is the open box of amsthm, \openbox, set by \qed as
// amsthm sets it: a \quad at least from the text, at the right margin. The
// engine drew a black square of 6pt.
//
// LaTeX does not indent the paragraph that follows a list or a trivlist,
// such as itemize, quote or center, unless a blank line comes first. The
// \endtrivlist of the kernel sets \@endpetrue, and \end runs \@doendpe
// after the group of the environment: the next \everypar takes the box of
// \parindent away with \lastbox, and a \par before it cancels that. The
// engine had the flag and \lastbox, and no \@doendpe. A theorem of amsthm
// still sets \@endpefalse after its \endtrivlist, as in LaTeX.
const ivyLaTeX = `\catcode64=11
\def\gotex@ivydoendpe{\ifx\par\gotex@ivyendpepar\else\let\gotex@ivysavedpar\par\fi
  \let\par\gotex@ivyendpepar
  \everypar{{\setbox\z@\lastbox}\everypar{}\let\par\gotex@ivysavedpar}}
\def\gotex@ivyendpepar{\let\par\gotex@ivysavedpar\everypar{}\par}
\def\end#1{\csname end#1\endcsname\if@endpe\aftergroup\gotex@ivydoendpe\fi\gotex@endenv{#1}}
\let\gotex@ivyendtrivlist\endtrivlist
\def\endtrivlist{\gotex@ivyendtrivlist\@endpetrue}
\let\gotex@ivyenditemize\enditemize
\def\enditemize{\gotex@ivyenditemize\@endpetrue}
\let\gotex@ivyendenumerate\endenumerate
\def\endenumerate{\gotex@ivyendenumerate\@endpetrue}
\let\gotex@ivyenddescription\enddescription
\def\enddescription{\gotex@ivyenddescription\@endpetrue}
\def\gotex@ivylist#1{\rightmargin\z@ \listparindent\z@ \itemindent\z@
  \csname @listi\endcsname
  \ifdim\leftmargin=\z@ \leftmargin2.5em\fi
  #1\relax
  \gotex@listtopspace\begingroup\@listfirsttrue
  \advance\leftskip\leftmargin \advance\rightskip\rightmargin
  \parindent\listparindent \parskip\parsep}
\def\gotex@ivyendlist{\par\endgroup\addvspace\@topsepadd\@endpetrue}
\def\gotex@ivyquote{\gotex@ivylist{\rightmargin\leftmargin}}
\def\gotex@ivyquotation{\gotex@ivylist{\listparindent1.5em\itemindent\listparindent
  \rightmargin\leftmargin \parsep\z@ \@plus\p@}}
\def\gotex@ivyverse{\let\\\gotex@ivyversecr
  \gotex@ivylist{\itemsep\z@ \itemindent-1.5em\listparindent\itemindent
    \rightmargin\leftmargin \advance\leftmargin1.5em}}
\def\gotex@ivyversecr{\ifhmode\unskip\fi\par
  \@ifstar{\nobreak\gotex@ivyxversecr}\gotex@ivyxversecr}
\def\gotex@ivyxversecr{\vskip-\parskip\@ifnextchar[\gotex@ivyiversecr\ignorespaces}
\def\gotex@ivyiversecr[#1]{\vskip#1\relax\ignorespaces}
\def\gotex@ivylists{%
  \let\quote\gotex@ivyquote \let\endquote\gotex@ivyendlist
  \let\quotation\gotex@ivyquotation \let\endquotation\gotex@ivyendlist
  \let\verse\gotex@ivyverse \let\endverse\gotex@ivyendlist}
\def\TeX{T\kern-.1667em\lower.5ex\hbox{E}\kern-.125emX\spacefactor1000 }
\def\LaTeX{L\kern-.36em{\setbox\z@\hbox{T}\vbox to\ht\z@{\hbox{\gotex@ivysfsize A}\vss}}\kern-.15em\TeX}
\def\LaTeXe{\mbox{\LaTeX\kern.15em2$_{\textstyle\varepsilon}$}}
\let\gotex@ivysetcounter\setcounter
\def\setcounter#1#2{\gotex@ivysetcounter{#1}{#2}\gotex@ivypagecounter{#1}{#2}}
\let\gotex@ivypagenumbering\pagenumbering
\def\pagenumbering#1{\gotex@ivypagenumbering{#1}\gotex@ivypagecounter{page}{1}}
\let\gotex@ivydocument\document
\def\document{\gotex@ivybegindocument\gotex@ivydocument}
\let\gotex@ivyenddocument\enddocument
\def\enddocument{\gotex@ivysavetextwidth\gotex@ivyenddocument}
\def\ps@fancy{\let\@mkboth\markboth
  \@ifundefined{chapter}{%
    \def\sectionmark##1{\markboth{\MakeUppercase{\ifnum\c@secnumdepth>\z@
      \thesection\hskip1em\relax\fi##1}}{}}%
    \def\subsectionmark##1{\markright{\ifnum\c@secnumdepth>\@ne
      \thesubsection\hskip1em\relax\fi##1}}}%
  {\def\chaptermark##1{\markboth{\MakeUppercase{\ifnum\c@secnumdepth>\m@ne
      \@chapapp\ \thechapter. \ \fi##1}}{}}%
    \def\sectionmark##1{\markright{\MakeUppercase{\ifnum\c@secnumdepth>\z@
      \thesection. \ \fi##1}}}}}
\def\openbox{\leavevmode\hbox to.77778em{\hfil\vrule
  \vbox to.675em{\hrule width.6em\vfil\hrule}\vrule\hfil}}
\def\qedsymbol{\openbox}
\def\qed{\leavevmode\unskip\penalty9999 \hbox{}\nobreak\hfill\quad\hbox{\qedsymbol}\par}
\AddToHook{class/article/after}{\gotex@ivylists}
\AddToHook{class/report/after}{\gotex@ivylists}
\AddToHook{class/book/after}{\gotex@ivylists}
\catcode64=12\relax
`

// sfSizes are the sizes of a script that LaTeX takes for the sizes of the
// standard classes, \sf@size of \DeclareMathSizes in size10.clo,
// size11.clo, size12.clo and fontmath.ltx.
var sfSizes = [][2]float64{
	{5, 5}, {6, 5}, {7, 5}, {8, 6}, {9, 6}, {10, 7}, {10.95, 8}, {12, 8},
	{14.4, 10}, {17.28, 12}, {20.74, 14.4}, {24.88, 20.74},
}

// sfSize returns the size of a script for the size pt: the size of the
// table within a hundredth of a point, or 0.7 of pt.
func sfSize(pt float64) float64 {
	for _, s := range sfSizes {
		if pt > s[0]-0.01 && pt < s[0]+0.01 {
			return s[1]
		}
	}
	return 0.7 * pt
}

// pageWidth sets \hsize, \headrulewidth and \footrulewidth for the
// assembly of a page, and returns the function that restores them.
//
// The engine assembles the pages after the group of the document closes,
// so \hsize is the width of the class again, and not the width that
// geometry set in the preamble: the head, the foot and their rules were
// 360pt wide on a text of 468pt. ivy keeps \hsize from the end of the
// document. A document in two columns keeps the \hsize of upstream.
//
// fancyhdr reads \headrulewidth and \footrulewidth when it draws a page,
// and a document sets them with \renewcommand. The engine kept a rule of
// 0.4pt under the head and none over the foot.
func (e *Engine) pageWidth() func() {
	hsize, head, foot := e.hsize, e.headRule, e.footRule
	if e.ivy.textWidth > 0 && !e.twoColumn {
		e.hsize = e.ivy.textWidth
	}
	if e.pageStyle == "fancy" {
		if w, ok := e.ruleWidth("headrulewidth"); ok {
			e.headRule = w
		}
		if w, ok := e.ruleWidth("footrulewidth"); ok {
			e.footRule = w
		}
	}
	return func() {
		e.hsize, e.headRule, e.footRule = hsize, head, foot
	}
}

// ruleWidth returns the width of the rule that the macro name holds, such
// as \headrulewidth, and false when it is not a macro.
func (e *Engine) ruleWidth(name string) (int, bool) {
	m := e.eq[name]
	if m == nil || m.kind != mMacro {
		return 0, false
	}
	return max(e.evalDimenTokens(m.body), 0), true
}

// pageCounterNode is a \setcounter{page} or a \pagenumbering on the
// vertical list: the page that holds it takes the number value, in the
// style of \pagenumbering, such as 'r' for roman.
type pageCounterNode struct {
	value int
	style byte
}

func (pageCounterNode) isNode() {}

// pageCounter returns the number and the style of the page that holds
// page, which follows a page of the number prev in the style style. A page
// without a pageCounterNode takes prev+1.
func pageCounter(page []node, prev int, style byte) (int, byte) {
	n := prev + 1
	for _, nd := range page {
		if c, ok := nd.(pageCounterNode); ok {
			n, style = c.value, c.style
		}
	}
	return n, style
}

// pageNumber returns the number of the page page, whose place in the
// document is ordinal, and sets the style of its number. The pages are
// assembled in their order, from the first.
func (e *Engine) pageNumber(page []node, ordinal int) int {
	if !e.ivy.numbered {
		return ordinal
	}
	if ordinal == 1 || ordinal != e.ivy.lastOrdinal+1 {
		e.ivy.lastNum, e.ivy.numStyle = ordinal-1, e.startStyle()
	}
	n, style := pageCounter(page, e.ivy.lastNum, e.ivy.numStyle)
	e.ivy.lastOrdinal, e.ivy.lastNum, e.ivy.numStyle = ordinal, n, style
	e.pageNumStyle = style
	return n
}

// startStyle returns the style of the number of the first page.
func (e *Engine) startStyle() byte {
	if e.ivy.startStyle == 0 {
		return 'a'
	}
	return e.ivy.startStyle
}

// logicalPageRefs gives the contents, the index and \pageref the numbers of
// the pages, in place of their places in the document.
func (e *Engine) logicalPageRefs() {
	if !e.ivy.numbered {
		return
	}
	var nums []int
	var carry []node
	list := e.mvl
	num, style := 0, e.startStyle()
	for start := 0; start < len(list); {
		end := e.findPageBreak(list, start)
		// a page that ships nothing gives its nodes to the next page, as in
		// paginateSingleList
		switch page := trimTrailingGlue(list[start:end]); {
		case pageIsUnshippable(page):
			carry = append(carry, page...)
		case len(page) > 0:
			num, style = pageCounter(append(carry, page...), num, style)
			carry = carry[:0]
			nums = append(nums, num)
		}
		next := skipDiscardable(list, end)
		if end > next-1 {
			next = end + 1
		}
		start = next
		if end == len(list) {
			break
		}
	}
	number := func(ordinal int) int {
		if ordinal >= 1 && ordinal <= len(nums) {
			return nums[ordinal-1]
		}
		return ordinal
	}
	for i := range e.tocEntries {
		e.tocEntries[i].page = number(e.tocEntries[i].page)
	}
	for i := range e.indexEntries {
		e.indexEntries[i].page = number(e.indexEntries[i].page)
	}
	for k, v := range e.labelPages {
		e.labelPages[k] = number(v)
	}
}

// carried reports whether the page builder keeps n for the next page when
// the page of n ships nothing: a \special, a mark or a \thispagestyle. TeX
// ships no page for them, and keeps them on the list. A \thispagestyle
// after the \newpage of a title page gave a blank page before the contents.
func carried(n node) bool {
	switch n.(type) {
	case specialNode, markNode, pageStyleNode, pageCounterNode:
		return true
	}
	return false
}

// unbracedInt reports whether t, the token after the name of a counter,
// is a number without braces: a digit, or a register or a \chardef such as
// \@ne. LaTeX takes the one token as the argument. The \end{titlepage} of
// the standard classes says \setcounter{page}\@ne, and the engine set the
// \@ne as a character, which started a paragraph and a blank page.
func (e *Engine) unbracedInt(t tok) bool {
	if !t.cs_ {
		return t.ch >= '0' && t.ch <= '9'
	}
	m := e.eq[t.cs]
	return m != nil && (m.kind == mCharDef || m.kind == mCountRef)
}

// loadIvyPrims installs the primitives of ivy.
func (e *Engine) loadIvyPrims() {
	// \gotex@ivypagecounter{name}{value} puts the value of the page counter
	// on the vertical list when name is page. The engine has no \c@page.
	e.prim("gotex@ivypagecounter", func(e *Engine) {
		name := e.readBraceName()
		v := e.readBraceInt()
		// a class says \pagenumbering{arabic} in the preamble, where the
		// style of the first page is set without a node
		if name != "page" || e.inPar || !e.ivy.inDocument {
			return
		}
		e.ivy.numbered = true
		e.contribute(pageCounterNode{value: v, style: e.pageNumStyle})
	})
	// \babelprovide[options]{language} of babel loads a language from its
	// .ini file. With the option main, the language is the main language of
	// the document, as \usepackage[language]{babel} makes it: French takes
	// its names, such as "Chapitre" and "Table des matières", and its
	// spaces. A document that loads babel.sty keeps its own \babelprovide.
	e.prim("babelprovide", func(e *Engine) {
		opts := e.scanBracketList()
		name := strings.TrimSpace(e.readBraceNameX())
		for _, o := range opts {
			if strings.TrimSpace(o) == "main" && name != "" {
				e.applyBabel([]string{name})
				return
			}
		}
	})
	// \gotex@ivybegindocument marks the start of the document, and keeps
	// the style of the page numbers of the preamble.
	e.prim("gotex@ivybegindocument", func(e *Engine) {
		e.ivy.inDocument, e.ivy.startStyle = true, e.pageNumStyle
	})
	// \gotex@ivysavetextwidth keeps \hsize at the end of the document.
	e.prim("gotex@ivysavetextwidth", func(e *Engine) {
		e.ivy.textWidth = e.hsize
	})
	// \gotex@ivysfsize selects the current font at the size of a script.
	e.prim("gotex@ivysfsize", func(e *Engine) {
		f := e.curFont
		if f == nil {
			f = e.baseFont
		}
		if f == nil {
			return
		}
		sf, ok := f.(scalableFont)
		if !ok {
			return
		}
		e.selectFont(sf.atSize(sfSize(f.sizePt())))
	})
}
