// This file of ivy runs the engine for the package latex (D62). It is the
// function compile of api.go, with the context, the fonts, the resolver and
// the math renderer of a run of ivy. A new commit of upstream that changes
// compile can need the same change here.

package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/xo/ivy"
	texmath "github.com/xo/ivy/latex/internal/mathsvg"
)

// ivyState is the state of a run of ivy that the engine reads.
type ivyState struct {
	// done is the channel of the context of the run, and nil outside a run
	// of ivy.
	done <-chan struct{}
	// steps counts the calls of stopped, so that the channel is read once
	// for many steps.
	steps uint32
	// textWidth is \hsize at the end of the document, in sp, and zero
	// before it. The pages are assembled after the group of the document
	// closes, which restores the \hsize of the class (ivy_latex.go).
	textWidth int
	// numbered is true when the document sets the page counter or the style
	// of the page numbers. lastOrdinal, lastNum and numStyle are the place,
	// the number and the style of the last page that was assembled.
	numbered             bool
	lastOrdinal, lastNum int
	numStyle             byte
	// inDocument is true after \begin{document}, and startStyle is the
	// style of the page numbers there.
	inDocument bool
	startStyle byte
	// faces are the faces of the families, the series and the shapes, by
	// fontAxes, which the font switches combine (ivy_fonts.go).
	faces map[fontAxes]fontFace
	// widowAt is the place in the vertical list of the widow penalty of the
	// last paragraph part, or -1, and linesEnd is the length of the list
	// after its last line (ivy_pagebreak.go).
	widowAt, linesEnd int
}

// stopped reports whether the context of the run is done. The step counter
// of the engine calls it on each step of expansion.
func (s *ivyState) stopped() bool {
	if s.done == nil {
		return false
	}
	s.steps++
	if s.steps&1023 != 0 {
		return false
	}
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// Faces are the font files of a run. A text face that is nil takes the
// regular face, and a nil Math takes STIX Two Math of okage.
type Faces struct {
	Regular, Bold, Italic, Mono, Sans, Math []byte
	// SansBold, SansItalic and SansBoldItalic are the other shapes of the
	// family of Sans. A nil shape takes Sans.
	SansBold, SansItalic, SansBoldItalic []byte
}

// RunOptions are the options of a run of ivy.
type RunOptions struct {
	// Faces are the fonts. Regular is required.
	Faces Faces
	// Resolve gives the files that a document reads by name: the classes,
	// the packages, the files of \input and the figures. It returns false
	// for a file it does not have.
	Resolve func(name string) ([]byte, bool)
	// Date is the text of \today, which is empty without it.
	Date string
	// JobName is \jobname, "texput" when it is empty.
	JobName string
	// StepLimit and NoProgressLimit replace the limits of the steps of
	// expansion of the engine when they are more than zero.
	StepLimit, NoProgressLimit int
	// MaxPages is the most pages of a document. Zero is the limit of the
	// engine.
	MaxPages int
}

// Doc is a document that a run put on pages.
type Doc struct {
	e       *Engine
	pages   []*boxNode
	margin  float64
	vmargin float64
	paperW  float64
	paperH  float64
	math    *texmath.Renderer
	skipped int
	// specials holds the parsed specials of each page.
	specials specialCache
	// outline is the outline of the document. Compile makes it, because
	// the page builder that finds the page of a heading is not safe for
	// concurrent use, and DrawPage is.
	outline []Outline
}

// The margin of a page of a document that names no paper, as the default
// of Options.Margin.
const defaultMargin = 72

// running holds one token while a run uses the engine. The engine of
// upstream changes maps of the package when it builds the primitives of a
// run, such as expandableSet, so two runs at the same time race. A run
// waits for the token, or for its context.
var running = make(chan struct{}, 1)

// Compile runs the source src and returns its pages. The run is lenient: a
// command that the engine does not know is skipped, as an editor preview of
// upstream does. It returns an error that wraps ivy.ErrFormat when the
// engine fails, the error of the context when the context is done, and one
// that wraps ivy.ErrLimit when the document has too many pages.
func Compile(ctx context.Context, src []byte, ro RunOptions) (doc *Doc, err error) {
	if len(ro.Faces.Regular) == 0 {
		return nil, fmt.Errorf("latex: no text font: %w", ivy.ErrUnsupported)
	}
	select {
	case running <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-running }()
	// The engine is input from a stranger at each step (D5).
	defer func() {
		if r := recover(); r != nil {
			doc, err = nil, fmt.Errorf("latex: panic: %v: %w", r, ivy.ErrFormat)
		}
	}()
	opt := Options{
		Font:            ro.Faces.Regular,
		BoldFont:        ro.Faces.Bold,
		ItalicFont:      ro.Faces.Italic,
		MonoFont:        ro.Faces.Mono,
		SansFont:        ro.Faces.Sans,
		Lenient:         true,
		Resolve:         ro.Resolve,
		Date:            ro.Date,
		JobName:         ro.JobName,
		StepLimit:       ro.StepLimit,
		NoProgressLimit: ro.NoProgressLimit,
	}
	build := func(latex bool) (*Engine, error) {
		e, err := buildEngine(opt, latex)
		if err != nil {
			return nil, fmt.Errorf("latex: %w: %w", ivy.ErrFormat, err)
		}
		e.loadIvyPrims()
		if err := e.bindFaces(ro.Faces); err != nil {
			return nil, fmt.Errorf("latex: %w: %w", ivy.ErrFormat, err)
		}
		if err := e.LoadFormat(ivyPlain); err != nil {
			return nil, fmt.Errorf("latex: %w: %w", ivy.ErrFormat, err)
		}
		if latex {
			if err := e.LoadFormat(ivyLaTeX); err != nil {
				return nil, fmt.Errorf("latex: %w: %w", ivy.ErrFormat, err)
			}
			if err := e.LoadFormat(ivyMath); err != nil {
				return nil, fmt.Errorf("latex: %w: %w", ivy.ErrFormat, err)
			}
		}
		e.ivy.done = ctx.Done()
		r, err := texmath.New(ro.Faces.Math)
		if err != nil {
			return nil, err
		}
		e.mathR = r
		return e, nil
	}
	run := func(e *Engine) error {
		_, err := e.Run(string(src))
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if err != nil {
			return fmt.Errorf("latex: %w: %w", ivy.ErrFormat, err)
		}
		return nil
	}
	e, err := compileWith(src, build, run)
	if err != nil {
		return nil, err
	}
	pages := e.Pages()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ro.MaxPages > 0 && len(pages) > ro.MaxPages {
		return nil, fmt.Errorf("latex: %d pages: %w", len(pages), ivy.ErrLimit)
	}
	doc = &Doc{e: e, pages: pages, math: e.mathR}
	var ok bool
	doc.paperW, doc.paperH, ok = e.paperSizePt()
	margin := float64(defaultMargin)
	if !ok && !isLaTeX(src) {
		// pdfTeX puts a document of plain TeX on US Letter, with the origin
		// of the text one inch from the top and the left side.
		doc.paperW, doc.paperH = 8.5*72.27, 11*72.27
		margin = 72.27
	}
	doc.margin = e.renderMargin(margin)
	doc.vmargin = e.renderVMargin(doc.margin)
	for _, n := range e.SkippedCommands() {
		doc.skipped += n
	}
	doc.outline = doc.makeOutline()
	return doc, nil
}

// compileWith is compile of api.go, with the functions of ivy that build an
// engine and run the source.
func compileWith(src []byte, build func(latex bool) (*Engine, error), run func(*Engine) error) (*Engine, error) {
	src = []byte(normalizeEOL(string(src)))
	latex := isLaTeX(src)
	if latex && needsTwoPass(src) {
		aux, err := build(true)
		if err != nil {
			return nil, err
		}
		if err := run(aux); err != nil {
			return nil, err
		}
		aux.finalizePages()
		prev := aux
		for rerun := 0; ; rerun++ {
			e, err := build(true)
			if err != nil {
				return nil, err
			}
			e.carryCrossRefs(prev)
			if err := run(e); err != nil {
				return nil, err
			}
			was := e.labelPages
			e.finalizePages()
			if rerun == maxReruns || e.crossRefsAgree(was) {
				return e, nil
			}
			prev = e
		}
	}
	e, err := build(latex)
	if err != nil {
		return nil, err
	}
	if err := run(e); err != nil {
		return nil, err
	}
	return e, nil
}

// PageCount returns the number of pages.
func (d *Doc) PageCount() int {
	return len(d.pages)
}

// bigPointsPerTeXPoint converts a TeX point (1/72.27in, what the layout
// computes in) to a big point (1/72in, the unit of a page of ivy).
const bigPointsPerTeXPoint = 72.0 / 72.27

// pageSizePt returns the size of page n in TeX points.
func (d *Doc) pageSizePt(n int) (w, h float64) {
	if d.paperW > 0 && d.paperH > 0 {
		return d.paperW, d.paperH
	}
	p := d.pages[n]
	return spToPt(p.width) + 2*d.margin, spToPt(p.height+p.depth) + 2*d.vmargin
}

// PageSize returns the size of page n in big points, 1/72 inch.
func (d *Doc) PageSize(n int) (w, h float32) {
	pw, ph := d.pageSizePt(n)
	return float32(pw * bigPointsPerTeXPoint), float32(ph * bigPointsPerTeXPoint)
}

// Info is the metadata of a document: the fields of \hypersetup, or the
// title and the author of the document under pdfusetitle.
type Info struct {
	Title, Author, Subject, Keywords string
}

// Info returns the metadata of the document.
func (d *Doc) Info() Info {
	e := d.e
	info := Info{Title: e.pdfTitle, Author: e.pdfAuthor, Subject: e.pdfSubject, Keywords: e.pdfKeywords}
	if info.Title == "" {
		info.Title = e.macroPlainText("@title")
	}
	if info.Author == "" {
		info.Author = e.macroPlainText("@author")
	}
	return info
}

// Skipped returns the number of commands that the run did not know and
// skipped.
func (d *Doc) Skipped() int {
	return d.skipped
}

// ErrNoPages is the error of a document that gives no page.
var ErrNoPages = errors.New("latex: the document has no pages")
