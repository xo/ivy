// This file of ivy holds the macros that ivy adds to the LaTeX of the
// engine, to set a document closer to pdfLaTeX (D62). Compile loads them
// after the macros of upstream, so that a new commit of upstream keeps
// them.

package engine

import (
	"strconv"
	"strings"
)

// ivyPlain is the format of the macros of ivy for plain TeX and LaTeX.
//
// \TeX is the logo of plain.tex, with the E lowered by half an ex.
const ivyPlain = `\def\newpage{\par\vfil\penalty-10000 }
\def\clearpage{\gotex@clearpagestrut\par\vfil\penalty-10000 \prevdepth\z@}
\def\gotex@ivyfnstrut{\vrule width\z@ height\footnotesep depth\z@\relax\ignorespaces}
\def\gotex@ivyfinalstrut{\unskip\ifhmode\nobreak\fi\vrule width\z@ height\z@ depth\dp\strutbox\relax}
\def\gotex@ivyheadings{\@clubpenalty\clubpenalty
  \def\@nobreakfalse{\global\let\if@nobreak\iffalse}%
  \def\@nobreaktrue{\global\let\if@nobreak\iftrue}%
  \let\gotex@ivyafterheading\@afterheading
  \def\@afterheading{\gotex@ivyafterheading\@nobreaktrue
    \everypar{\if@nobreak\@nobreakfalse\clubpenalty\@M
      \else\clubpenalty\@clubpenalty\everypar{}\fi}}}
\def\TeX{T\kern-.1667em\lower.5ex\hbox{E}\kern-.125emX}
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
// loads, ivy sets them, itemize and enumerate as \list does: \@listdepth
// counts the lists, \@listi, \@listii and the others give \leftmargin,
// \parsep, \topsep and \itemsep of each depth, the second argument of \list
// changes them, and the margins go to \leftskip and \rightskip. The space
// above a list is \topsep, with \partopsep when the list starts a paragraph,
// and the \parskip of the text around it. Inside the list, \parskip is
// \parsep, and \itemsep goes between two items, as \@item does. The
// penalties \@beginparpenalty, \@itempenalty and \@endparpenalty are in
// their places. The engine gave every depth the values of the first one,
// and no \parsep to the paragraphs of an item. A \\ of verse ends the line
// as \@centercr does, with no \parskip.
//
// \@afterheading sets \@nobreaktrue, and the first paragraph after a heading
// has \clubpenalty 10000, as in latex.ltx. A heading right after a heading
// then adds no space of its own. \@nobreaktrue and \@nobreakfalse are
// global, as in latex.ltx.
//
// \newpage and \clearpage end the page with \vfil, as in latex.ltx. The
// page builder keeps it (ivy_pagebreak.go), so the footnotes of the page
// stand at its bottom, and the \vfil shares the space of a title page.
// \clearpage sets \prevdepth to 0, as its \vbox{} does in latex.ltx.
//
// \gotex@ivyfnstrut and \gotex@ivyfinalstrut are the struts of
// \@footnotetext around the text of a footnote: \rule\z@\footnotesep and
// \@finalstrut\strutbox.
//
// \LaTeX is the logo of latex.ltx: an A in the size of a script, the size
// that \sf@size names, raised to the top of the T. latex.ltx puts the A in
// a \vbox. ivy raises an \hbox: under \centering, as in a caption, the
// engine set the \vbox to the width of the line.
//
// \TeX, \LaTeX, \LaTeXe, \em, \emph, \dots and \textsuperscript are robust,
// as in latex.ltx. A \section or a \caption writes its title to the
// contents with its macros expanded, and the contents set it again from
// the text. The logos broke there: "Configuring \LaTeX" came out as
// "Configuring L@T@@ivysfsizeA" in the contents of cfgguide.tex.
//
// \em selects the upright shape in an italic or a slanted font, and the
// italic shape in any other font, as \em of latex.ltx does with
// \fontdimen1. The engine gives \fontdimen1 as 0, so \gotex@ivyem asks
// which face is in use.
//
// \dots, \ldots and \textellipsis set three periods, each with a kern of
// \fontdimen3 after it, as \textellipsis of latex.ltx does. The engine set
// three periods without the kerns. In math, the source of the formula
// keeps \ldots and \dots, and the macros do not run.
//
// \@textsuperscript sets its argument in the size of a script, raised by
// the shift of a superscript of TeX (\fontdimen14 of the font of the
// symbols at the size of the text), as latex.ltx does in math. The engine
// set it at the size of the text on the baseline, so the mark of a
// footnote in its note was large and low. \textsuperscript uses it, as in
// latex.ltx, in place of a formula.
//
// \@makefntext is the one of article.cls, report.cls and book.cls: the mark
// at the right of a box of 1.8 em, and the text after it. A class defines
// its own, and then the class wins. Without it, as in a class that ivy does
// not have, such as ltxguide, the engine set the mark on a line of its own
// above the note.
//
// \section[short]{title}, \subsection[short]{title} and
// \caption[short]{title} write the short title to the contents and to the
// lists of figures and tables, as LaTeX does. The engine took "[" for the
// title, or wrote the long title.
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
\newcount\@listdepth
\newskip\gotex@ivyouterparskip
\def\gotex@ivylist#1{\ifnum\@listdepth>5 \else\global\advance\@listdepth\@ne\fi
  \rightmargin\z@ \listparindent\z@ \itemindent\z@
  \csname @list\romannumeral\the\@listdepth\endcsname
  \ifdim\leftmargin=\z@ \leftmargin2.5em\fi
  #1\relax
  \@topsepadd\topsep
  \ifvmode\advance\@topsepadd\partopsep\else\unskip\par\fi
  \gotex@ivyouterparskip\parskip
  \if@nobreak\else
    \addpenalty\@beginparpenalty
    \@tempskipa\@topsepadd \advance\@tempskipa\parskip
    \addvspace\@tempskipa
  \fi
  \begingroup\@listfirsttrue
  \advance\leftskip\leftmargin \advance\rightskip\rightmargin
  \parindent\listparindent \parskip\parsep}
\def\gotex@ivyitemspace{\parskip\parsep
  \if@listfirst\@listfirstfalse\addvspace{-\parskip}%
  \else\addpenalty\@itempenalty\addvspace\itemsep\fi}
\def\gotex@ivyendlist{\gotex@ivyclose{}}
\def\gotex@ivyclose#1{\par
  \ifdim\lastskip>\z@ \@tempskipa\lastskip \vskip-\lastskip
    \advance\@tempskipa\parskip \advance\@tempskipa-\gotex@ivyouterparskip
    \vskip\@tempskipa\fi
  #1\endgroup\global\advance\@listdepth\m@ne
  \addpenalty\@endparpenalty\addvspace\@topsepadd\@endpetrue}
\def\gotex@ivyitemize{\gotex@ivylist{}\advance\c@itemdepth\@ne
  \ifcase\c@itemdepth\or\gotex@ivybullet\labelitemi\or\gotex@ivybullet\labelitemii
  \or\gotex@ivybullet\labelitemiii\else\gotex@ivybullet\labelitemiv\fi
  \@enumitemopt{itemize}}
\def\gotex@ivybullet#1{\def\item{\par\gotex@ivyitemspace
  \@ifnextbracket{\@itemopt}{\@bulletitem#1}}}
\def\gotex@ivyenumerate{\gotex@ivylist{}\advance\c@enumdepth\@ne
  \ifcase\c@enumdepth\or\gotex@ivynumber\c@enumi\theenumi\labelenumi
  \or\gotex@ivynumber\c@enumii\theenumii\labelenumii
  \or\gotex@ivynumber\c@enumiii\theenumiii\labelenumiii
  \else\gotex@ivynumber\c@enumiv\theenumiv\labelenumiv\fi
  \@enumitemopt{enumerate}}
\def\gotex@ivynumber#1#2#3{#1=0\relax\def\item{\par\gotex@ivyitemspace
  \@ifnextbracket{\@itemopt}{\@listitem#1#2#3}}}
\def\gotex@ivyendenumerate{\gotex@ivyclose\@enumitemrec}
\def\gotex@ivyquote{\gotex@ivylist{\rightmargin\leftmargin}\gotex@ivyitemspace}
\def\gotex@ivyquotation{\gotex@ivylist{\listparindent1.5em\itemindent\listparindent
  \rightmargin\leftmargin \parsep\z@ \@plus\p@}\gotex@ivyitemspace}
\def\gotex@ivyverse{\let\\\gotex@ivyversecr
  \gotex@ivylist{\itemsep\z@ \itemindent-1.5em\listparindent\itemindent
    \rightmargin\leftmargin \advance\leftmargin1.5em}\gotex@ivyitemspace}
\def\gotex@ivyversecr{\ifhmode\unskip\fi\par
  \@ifstar{\nobreak\gotex@ivyxversecr}\gotex@ivyxversecr}
\def\gotex@ivyxversecr{\vskip-\parskip\@ifnextchar[\gotex@ivyiversecr\ignorespaces}
\def\gotex@ivyiversecr[#1]{\vskip#1\relax\ignorespaces}
\def\gotex@ivylists{\gotex@ivyheadings
  \let\itemize\gotex@ivyitemize \let\enditemize\gotex@ivyendlist
  \let\enumerate\gotex@ivyenumerate \let\endenumerate\gotex@ivyendenumerate
  \let\quote\gotex@ivyquote \let\endquote\gotex@ivyendlist
  \let\quotation\gotex@ivyquotation \let\endquotation\gotex@ivyendlist
  \let\verse\gotex@ivyverse \let\endverse\gotex@ivyendlist}
\protected\def\TeX{T\kern-.1667em\lower.5ex\hbox{E}\kern-.125emX\spacefactor1000 }
\protected\def\LaTeX{L\kern-.36em{\setbox\z@\hbox{T}\setbox\tw@\hbox{\gotex@ivysfsize A}%
  \dimen@\ht\z@\advance\dimen@-\ht\tw@\raise\dimen@\box\tw@}\kern-.15em\TeX}
\protected\def\LaTeXe{\mbox{\LaTeX\kern.15em2$_{\textstyle\varepsilon}$}}
\protected\def\em{\gotex@ivyem}
\protected\def\emph#1{{\em #1}}
\protected\def\textellipsis{.\kern\fontdimen3\font.\kern\fontdimen3\font.\kern\fontdimen3\font}
\protected\def\dots{\ifmmode...\else\textellipsis\fi}
\let\ldots\dots
\def\@textsuperscript#1{{\leavevmode\gotex@ivysupraise\hbox{\gotex@ivysfsize#1}}}
\protected\def\textsuperscript#1{\@textsuperscript{#1}}
\long\def\@makefntext#1{\parindent 1em\noindent\hb@xt@1.8em{\hss\@makefnmark}#1}
\let\gotex@ivyshort\relax
\let\gotex@ivytocentry\@tocentry
\def\@tocentry#1#2#3#4{\gotex@ivytocentry{#1}{#2}{#3}{\ifx\gotex@ivyshort\relax#4\else\gotex@ivyshort\fi}}
\def\@gxsecopt#1#2#3#4#5[#6]#7{\gdef\gotex@ivyshort{#6}#5{#1}{#2}{#3}{#4}{#7}\global\let\gotex@ivyshort\relax}
\def\gotex@ivyopt#1{\expandafter\let\csname gotex@ivy#1\expandafter\endcsname\csname #1\endcsname
  \expandafter\def\csname #1\endcsname{\@ifnextchar[{\csname gotex@ivy#1opt\endcsname}{\csname gotex@ivy#1\endcsname}}%
  \expandafter\def\csname gotex@ivy#1opt\endcsname[##1]##2{\gdef\gotex@ivyshort{##1}%
    \csname gotex@ivy#1\endcsname{##2}\global\let\gotex@ivyshort\relax}}
\gotex@ivyopt{@nsection}\gotex@ivyopt{@nsubsection}\gotex@ivyopt{gotex@caption@plain}
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
	for start := pageTop(list, 0); start < len(list); {
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
		next := pageTop(list, end)
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
	// \gotex@ivyem selects the shape of \em: the upright shape in an italic
	// or a slanted face, and the italic shape in any other face.
	e.prim("gotex@ivyem", func(e *Engine) {
		name := "itshape"
		if e.slanted(e.curFont) {
			name = "upshape"
		}
		e.back(csTok(name))
	})
	// \gotex@ivysupraise puts \raise and the shift of a superscript at the
	// size of the current font on the input.
	e.prim("gotex@ivysupraise", func(e *Engine) {
		size := 10.0
		if e.curFont != nil {
			size = e.curFont.sizePt()
		}
		e.push(append([]tok{csTok("raise")}, stringToToks(strconv.Itoa(ptToSP(supShift(size)))+"sp")...))
	})
}

// ivyFootnoteMark puts the mark of a footnote in the text through the
// \@makefnmark of the class, with \@thefnmark as the mark, as
// \@footnotemark of latex.ltx does. It reports false when \@makefnmark is
// not a macro. The engine set the digits at the size of the text, raised
// by 0.4 em, where article.cls sets them at the size of a script.
func (e *Engine) ivyFootnoteMark(mark string) bool {
	if m := e.eq["@makefnmark"]; m == nil || m.kind != mMacro {
		return false
	}
	ts := make([]tok, 0, len(mark)+7)
	ts = append(ts, chTok('{', catBegin), csTok("def"), csTok("@thefnmark"), chTok('{', catBegin))
	ts = append(ts, stringToToks(mark)...)
	ts = append(ts, chTok('}', catEnd), csTok("@makefnmark"), chTok('}', catEnd))
	e.push(ts)
	return true
}

// slanted reports whether f is a face of the italic or the slanted shape:
// the face of \it or \sl, at any size.
func (e *Engine) slanted(f fontFace) bool {
	if f == nil {
		return false
	}
	if e.ivy.faces != nil && e.axesOf(f).italic {
		return true
	}
	for _, name := range []string{"it", "sl"} {
		if m := e.eq[name]; m != nil && m.kind == mFont && sameShape(m.font, f) {
			return true
		}
	}
	return false
}

// sameShape reports whether a and b are faces of one shape at any size.
// The faces of Latin Modern share the table of their optical sizes, and
// another face shares its program.
func sameShape(a, b fontFace) bool {
	oa, ok := a.(*OpenTypeFont)
	ob, ok2 := b.(*OpenTypeFont)
	if !ok || !ok2 {
		return a == b
	}
	if len(oa.optical) > 0 && len(ob.optical) > 0 {
		return &oa.optical[0] == &ob.optical[0]
	}
	return oa.f == ob.f
}

// supShifts are the shifts of a superscript in the text style, \fontdimen14
// of cmsy5 to cmsy10, as pdfLaTeX gives them, by the size of the text. A
// size of more than 10 points takes cmsy10 at the size, as fontmath.ltx
// does.
var supShifts = [][2]float64{
	{5, 2.01776}, {6, 2.5178}, {7, 3.01779}, {8, 2.82333}, {9, 3.82329}, {10, 3.62892},
}

// supShift returns the shift of a superscript, in points, for the text at
// the size pt.
func supShift(pt float64) float64 {
	for i := len(supShifts) - 1; i >= 0; i-- {
		if s := supShifts[i]; pt >= s[0]-0.01 {
			if i == len(supShifts)-1 {
				return s[1] * pt / s[0]
			}
			return s[1]
		}
	}
	return supShifts[0][1] * pt / supShifts[0][0]
}
