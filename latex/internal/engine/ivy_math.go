// This file of ivy holds what ivy adds to the displays and the theorems of
// the engine, to set them closer to pdfLaTeX with amsmath and amsthm (D62).

package engine

import "strings"

// ivyMath is the format of the macros of ivy for the theorems of amsthm.
// Compile loads it after ivyLaTeX.
//
// \theoremstyle names the style of the theorems that \newtheorem defines
// after it, and \newtheorem keeps the style in the environment
// (thmStyleToks). The style gives the font of the head, the font of the
// body and the skip above and below the theorem, as amsthm does:
//
//	plain       a bold head, an italic body and \topsep
//	definition  a bold head, an upright body and \topsep
//	remark      an italic head, an upright body and half of \topsep
//
// \newtheoremstyle gives the font of the body, its fourth argument, and the
// font of the head, its seventh, with \topsep. The engine set every theorem
// in the style plain, with a \medskip above and below it.
//
// The skips go through \addvspace, so the skip between two theorems is the
// larger of the two, as in amsthm. The head ends with its period in the
// font of the head, and with a space of 5pt plus 1pt minus 1pt,
// \thm@headsep of amsthm. The number is upright, as \@upn of amsthm sets
// it, so the number of a remark is in the upright medium font. The note of
// a theorem is in parentheses in the upright medium font, as
// \thm@notefont of amsthm sets it. The body ignores the spaces after the
// head, as amsthm does.
const ivyMath = `\catcode64=11
\newskip\gotex@ivythmskip
\def\theoremstyle#1{\def\gotex@ivythmstyle{#1}}
\def\gotex@ivythmstyle{plain}
\def\gotex@ivythmhead{\bf}
\def\gotex@ivythmbody{\it}
\let\gotex@ivythmnum\relax
\def\gotex@ivythm@plain{\def\gotex@ivythmhead{\bf}\def\gotex@ivythmbody{\it}%
  \let\gotex@ivythmnum\relax \gotex@ivythmskip\topsep}
\def\gotex@ivythm@definition{\def\gotex@ivythmhead{\bf}\def\gotex@ivythmbody{\normalfont}%
  \let\gotex@ivythmnum\relax \gotex@ivythmskip\topsep}
\def\gotex@ivythm@remark{\def\gotex@ivythmhead{\it}\def\gotex@ivythmbody{\normalfont}%
  \let\gotex@ivythmnum\normalfont \gotex@ivythmskip\topsep \divide\gotex@ivythmskip\tw@}
\def\newtheoremstyle#1#2#3#4#5#6#7#8#9{\@namedef{gotex@ivythm@#1}{%
  \def\gotex@ivythmhead{#7}\def\gotex@ivythmbody{#4}\let\gotex@ivythmnum\relax
  \gotex@ivythmskip\topsep}}
\def\@endtheorem{\par\addvspace\gotex@ivythmskip\endgroup}
\def\@begintheorem#1#2{\edef\@currentreftype{\gotex@thmtype}\def\@currentlabelname{}%
  \noindent{\gotex@ivythmhead #1\ {\gotex@ivythmnum #2}}%
  \@ifnextbracket{\@opargbegintheorem}{\@stdbegintheorem}}
\def\@beginthmnonum#1{\edef\@currentreftype{\gotex@thmtype}\def\@currentlabelname{}%
  \noindent{\gotex@ivythmhead #1}\@ifnextbracket{\@opargbegintheorem}{\@stdbegintheorem}}
\def\@stdbegintheorem{{\gotex@ivythmhead .}\hskip5\p@\@plus\p@\@minus\p@\gotex@ivythmbody\ignorespaces}
\def\@opargbegintheorem[#1]{\def\gotex@thmnote{#1}\ifx\gotex@thmnote\@empty\@stdbegintheorem
  \else\ {\normalfont(#1)}\@stdbegintheorem\fi}
\catcode64=12\relax
`

// thmStyleToks returns the tokens that set the style of the last
// \theoremstyle in a theorem, and add the skip of the style above it:
//
//	\gotex@ivythm@plain \csname gotex@ivythm@<style>\endcsname
//	\addvspace\gotex@ivythmskip
//
// A style that no macro defines is \relax, and the theorem keeps the style
// plain.
func (e *Engine) thmStyleToks() []tok {
	style := "plain"
	if m := e.eq["gotex@ivythmstyle"]; m != nil && m.kind == mMacro {
		if s := strings.TrimSpace(e.toksToString(m.body)); s != "" {
			style = s
		}
	}
	t := []tok{csTok("gotex@ivythm@plain"), csTok("csname")}
	t = append(t, stringToToks("gotex@ivythm@"+style)...)
	return append(t, csTok("endcsname"), csTok("addvspace"), csTok("gotex@ivythmskip"))
}

// eqNumberHbox packs the number of an equation in \normalfont, as
// \@eqnnum of LaTeX and \maketag@@@ of amsmath do. The engine set it in
// the current font, which is italic in a theorem.
func (e *Engine) eqNumberHbox(s string) *boxNode {
	e.beginGroup()
	e.execToks([]tok{csTok("normalfont")})
	b := e.textToHbox(s)
	e.endGroup()
	return b
}

// displayRow packs a display of one line, m of the width w, with its number
// num, or with no number when num is nil. TeX centers the display on the
// whole line, and moves it left only when the space between the display
// and the number is less than the width of the number (tex.web §1202):
//
//	d = (z - w) / 2, and if d < 2e, d = (z - w - e) / 2
//
// where z is the width of the line and e the width of the number. The
// engine centered the display in the line less the number, so a numbered
// display was half the width of the number left of the place of pdfLaTeX.
// A display that does not fit beside its number, w + e + 1em > z, keeps the
// row of the engine. placeEquation puts its number on a line of its own.
func (e *Engine) displayRow(m node, w int, num *boxNode) *boxNode {
	z := e.hsize
	if num == nil {
		return hpackSP([]node{e.hfil(), m, e.hfil()}, packTo, z)
	}
	ew := num.width
	if !e.fitsBesideNumber(w, num) {
		return hpackSP([]node{e.hfil(), m, e.hfil(), num}, packTo, z)
	}
	d := (z - w) / 2
	if d < 2*ew {
		d = (z - w - ew) / 2
	}
	return hpackSP([]node{kernNode{width: d}, m, e.hfil(), num}, packTo, z)
}

// fitsBesideNumber reports whether a display of the width w fits on its
// line beside the number num with a quad between them, as tex.web §1199
// asks.
func (e *Engine) fitsBesideNumber(w int, num *boxNode) bool {
	return w+num.width+e.mathSize()*unity <= e.hsize
}

// placeEquation places a display of one line, m of the width w, with its
// number num, or with no number when num is nil. A display that does not
// fit beside its number is centered on its line, and the number goes flush
// right on the line below, after a penalty of 10000 and with no
// \belowdisplayskip, as tex.web §1205 sets it. The engine set the number
// over the end of the display.
func (e *Engine) placeEquation(m node, w int, num *boxNode) {
	if num == nil || e.fitsBesideNumber(w, num) {
		e.placeDisplay([]*boxNode{e.displayRow(m, w, num)})
		return
	}
	e.placeDisplay([]*boxNode{e.displayRow(m, w, nil)})
	// placeDisplay ends with \belowdisplayskip
	if n := len(e.mvl); n > 0 {
		if _, ok := e.mvl[n-1].(glueNode); ok {
			e.mvl = e.mvl[:n-1]
		}
	}
	e.mvl = append(e.mvl, penaltyNode{penalty: 10000})
	e.appendToPage(hpackSP([]node{e.hfil(), num}, packTo, e.hsize))
}
