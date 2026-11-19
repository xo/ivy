// This file is not a port. It lays out the lines of a large document ahead,
// on goroutines, as a galley: a column of lines without page breaks (D57).
// Upstream lays out each line when the pages reach it.

package layout

import (
	"context"
	"fmt"
	"hash/fnv"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// The bounds of the galley (D57).
const (
	// galleyPartText is the length of the text of a part, in bytes, at
	// least.
	galleyPartText = 64 << 10
	// galleyWorkers bounds the goroutines that lay out the parts.
	galleyWorkers = 8
	// galleyAhead is the number of pages after the pages that the layout
	// knows from which Page starts the galley.
	galleyAhead = 8
	// galleyHeight is the height of the page of a part, in pixels.
	galleyHeight = 1e8
	// galleyBatch is the number of lines that a part gives to the pages
	// at a time.
	galleyBatch = 16
)

// GalleyOptions are the options of the galley for the tests (D57). The zero
// value is the galley of the program.
type GalleyOptions struct {
	// Off turns the galley off.
	Off bool
	// OneBlock makes parts of one block each, so that a document has many
	// cuts. MaxParts bounds the parts, and they are spread over the
	// document.
	OneBlock bool
	// MaxParts bounds the parts of a document with OneBlock, 64 when it is
	// 0.
	MaxParts int
	// Wait makes the pages wait for the lines of a part.
	Wait bool
	// Early starts the galley at the first page that a document lays out,
	// and not only for a count or for a page far ahead.
	Early bool
}

// galleyOptions holds the options of SetGalleyOptions, or nil.
var galleyOptions atomic.Pointer[GalleyOptions]

// SetGalleyOptions sets the options of the galley of the documents that
// start a galley after the call, and returns a function that sets the
// previous options again. It is for the tests.
func SetGalleyOptions(o GalleyOptions) (restore func()) {
	prev := galleyOptions.Swap(&o)
	return func() { galleyOptions.Store(prev) }
}

// currentGalleyOptions returns the options of the galley.
func currentGalleyOptions() GalleyOptions {
	if o := galleyOptions.Load(); o != nil {
		return *o
	}
	return GalleyOptions{}
}

// galleyPlan is what the pages know of the boxes of a document for the
// galley. NewDocument does not make it. Book makes it before the layout
// changes the boxes.
type galleyPlan struct {
	// rebuild reads, styles and builds the document again, with a font
	// configuration.
	rebuild func(fc *FontConfiguration) (*Document, error)
	// tree holds the index of each box in tree order, and the styles and
	// the elements of the boxes by index.
	tree galleyTree
	// blocks are the blocks where a part can start, in tree order.
	blocks []galleyBlock
}

// galleyTree holds the boxes of a document in tree order.
type galleyTree struct {
	index    map[*Box]int
	boxes    []*Box
	styles   []*Style
	elements []*Element
	// end is the index after the last descendant of each box, and text the
	// length of the text of the box and of its descendants.
	end  []int
	text []int
	// sum is a hash of the kinds, the tags and the texts of the boxes.
	sum uint64
}

// galleyBlock is a block that can start a part: the indexes of the block
// and of the box after its descendants, and the length of its text.
type galleyBlock struct {
	lo, hi, text int
}

// treeText returns the length of the text of box and of its descendants.
func treeText(box *Box) int {
	n := len(box.Text)
	for _, child := range box.Children {
		n += treeText(child)
	}
	return n
}

// newGalleyTree returns the boxes of root in tree order.
func newGalleyTree(root *Box) galleyTree {
	t := galleyTree{index: map[*Box]int{}}
	h := fnv.New64a()
	var buf [24]byte
	var walk func(box *Box)
	walk = func(box *Box) {
		i := len(t.boxes)
		t.index[box] = i
		t.boxes = append(t.boxes, box)
		t.styles = append(t.styles, box.Style)
		t.elements = append(t.elements, box.Element)
		t.end = append(t.end, 0)
		t.text = append(t.text, len(box.Text))
		b := strconv.AppendInt(buf[:0], int64(box.Kind), 10)
		b = append(b, ' ')
		b = strconv.AppendInt(b, int64(len(box.Children)), 10)
		b = append(b, ' ')
		_, _ = h.Write(b)
		_, _ = h.Write([]byte(box.ElementTag))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(box.Text))
		_, _ = h.Write([]byte{0})
		for _, child := range box.Children {
			walk(child)
			t.text[i] += t.text[t.index[child]]
		}
		t.end[i] = len(t.boxes)
	}
	walk(root)
	t.sum = h.Sum64()
	return t
}

// blocks returns the blocks of the tree where a part can start: the
// children of the root, and the children of a block container that holds
// more text than minText and only blocks.
func (t *galleyTree) blocks(minText int) []galleyBlock {
	var blocks []galleyBlock
	var walk func(box *Box)
	walk = func(box *Box) {
		for _, child := range box.Children {
			i := t.index[child]
			if t.text[i] > minText && splittable(child) {
				walk(child)
				continue
			}
			blocks = append(blocks, galleyBlock{lo: i, hi: t.end[i], text: t.text[i]})
		}
	}
	if len(t.boxes) > 0 {
		walk(t.boxes[0])
	}
	return blocks
}

// splittable reports whether the galley can cut box between its children:
// a block container in the normal flow whose children are blocks.
func splittable(box *Box) bool {
	if box.Kind != KindBlock || box.IsTableWrapper || !box.IsInNormalFlow() || len(box.Children) == 0 {
		return false
	}
	for _, child := range box.Children {
		if child.Kind == KindLine {
			return false
		}
	}
	return true
}

// prepareGalley makes the plan of the galley of the document, before the
// layout changes its boxes. rebuild builds the document again. A document
// with less text than two parts gets no plan.
func (d *Document) prepareGalley(rebuild func(fc *FontConfiguration) (*Document, error)) {
	if d.part != nil || currentGalleyOptions().Off {
		return
	}
	o := currentGalleyOptions()
	if !o.OneBlock && treeText(d.root) < 2*galleyPartText {
		return
	}
	tree := newGalleyTree(d.root)
	minText := galleyPartText
	if o.OneBlock {
		minText = 0
	}
	d.galleyPlan = &galleyPlan{rebuild: rebuild, tree: tree, blocks: tree.blocks(minText)}
}

// parts cuts the blocks into parts of about the same length of text. The
// first part starts at the first block. With OneBlock, each part is a block.
func (p *galleyPlan) parts(o GalleyOptions) []*galleyPart {
	blocks := p.blocks
	var parts []*galleyPart
	if o.OneBlock {
		maxParts := o.MaxParts
		if maxParts <= 0 {
			maxParts = 64
		}
		step := max(1, len(blocks)/maxParts)
		for i := 0; i < len(blocks); i += step {
			parts = append(parts, newGalleyPart(blocks[i].lo, blocks[i].hi))
		}
		return parts
	}
	total := 0
	for _, b := range blocks {
		total += b.text
	}
	n := min(total/galleyPartText, galleyWorkers)
	if n < 2 {
		return nil
	}
	lo, sum := 0, 0
	for i, b := range blocks {
		sum += b.text
		if sum*n >= total*(len(parts)+1) || i == len(blocks)-1 {
			parts = append(parts, newGalleyPart(blocks[lo].lo, b.hi))
			lo = i + 1
		}
	}
	return parts
}

// galleyKey is the key of a line of the galley: the inputs of its layout.
// box is the index of the line box in tree order, skip the resume point
// inside the line box, x, contentX and width the left edges and the width
// of the containing block, indent the text indent and bottom the bottom
// space.
type galleyKey struct {
	box                 int
	skip                string
	x, contentX, width  float64
	indent, bottomSpace float64
}

// newGalleyKey returns the key of the next line of the line box box, or a
// key whose box is -1 when the line box is not a box of the tree.
func newGalleyKey(lc *layoutContext, box *Box, skip skipStack, containingBlock *Box, bottomSpace float64) galleyKey {
	i, ok := lc.lines.index[box]
	if !ok {
		return galleyKey{box: -1}
	}
	return galleyKey{
		box:         i,
		skip:        skipKey(skip),
		x:           containingBlock.PositionX,
		contentX:    containingBlock.ContentBoxX(),
		width:       containingBlock.Width,
		indent:      box.textIndent,
		bottomSpace: bottomSpace,
	}
}

// skipKey returns a skip stack as a string.
func skipKey(s skipStack) string {
	if s == nil {
		return "-"
	}
	keys := make([]int, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var b strings.Builder
	b.WriteByte('{')
	for _, k := range keys {
		b.WriteString(strconv.Itoa(k))
		b.WriteByte(':')
		b.WriteString(skipKey(s[k]))
		b.WriteByte(',')
	}
	b.WriteByte('}')
	return b.String()
}

// galleyLine is a line of the galley, laid out at a height of 0, with the
// pointers of the boxes of the pages, and where the next line resumes.
type galleyLine struct {
	line     *Box
	resumeAt skipStack
}

// galleyPart is a part of the galley: the boxes of the indexes from lo to
// hi, and the lines that a goroutine laid out for them.
type galleyPart struct {
	lo, hi int
	// done is closed when the goroutine of the part ends.
	done chan struct{}
	// stopped is set when the pages passed the part.
	stopped atomic.Bool
	// mu guards cancel and lines. cancel stops the layout of the part, or
	// is nil before it starts. lines are the lines of the part, or nil
	// when the pages passed the part.
	mu     sync.Mutex
	cancel context.CancelFunc
	lines  map[galleyKey]*galleyLine
}

// newGalleyPart returns a part of the boxes from lo to hi.
func newGalleyPart(lo, hi int) *galleyPart {
	return &galleyPart{lo: lo, hi: hi, done: make(chan struct{}), lines: map[galleyKey]*galleyLine{}}
}

// stop stops the layout of the part. Unless keep is set, it drops its
// lines.
func (p *galleyPart) stop(keep bool) {
	p.stopped.Store(true)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
	if !keep {
		p.lines = nil
	}
}

// galleyReader is the galley of the pages of a document. The goroutines of
// the parts lay out the lines, and the pages take them.
type galleyReader struct {
	plan  *galleyPlan
	parts []*galleyPart
	// faces is the number of faces of the font configuration of the
	// document, and facesKey the key of its faces, when the galley
	// started. A part whose configuration has other faces gives no lines.
	faces    int
	facesKey string
	// keep is set when the pages can lay out the document again, for the
	// counter pages. The parts that the pages passed then keep their
	// lines.
	keep bool
	wait bool
	// reached is the index of the last part from which the pages took a
	// line, and passed the index of the first part that the pages did not
	// stop.
	reached, passed int
	cancel          context.CancelFunc
	// lines counts the lines of all the parts, which Limits.MaxObjects
	// bounds.
	lines    atomic.Int64
	maxLines int64
	wg       sync.WaitGroup
}

// galleyFaces returns a key of the faces of a font configuration: the
// number of the faces of the program and of all faces, and the @font-face
// rules that loaded faces.
func galleyFaces(fc *FontConfiguration) string {
	return fmt.Sprint(fc.base, fc.db.Len(), fc.loaded)
}

// startGalley starts the galley of the document, when it has a plan and
// its galley is not started yet. The cancellation of ctx stops the galley.
func (d *Document) startGalley(ctx context.Context) {
	plan := d.galleyPlan
	if plan == nil || d.lc.lines.galley != nil || d.complete {
		return
	}
	o := currentGalleyOptions()
	workers := min(runtime.GOMAXPROCS(0), galleyWorkers)
	if o.Off || workers < 2 {
		return
	}
	parts := plan.parts(o)
	if len(parts) < 2 {
		return
	}
	fc := d.lc.fontConfig
	gctx, cancel := context.WithCancel(context.Background())
	stop := context.AfterFunc(ctx, cancel)
	gr := &galleyReader{
		plan:     plan,
		parts:    parts,
		faces:    fc.db.Len(),
		facesKey: galleyFaces(fc),
		keep:     d.whole,
		wait:     o.Wait,
		maxLines: int64(d.lc.limits.MaxObjects),
		cancel: func() {
			stop()
			cancel()
		},
	}
	d.lc.lines.galley = gr
	d.lc.lines.index = plan.tree.index
	// The first part has no galley: the pages lay out its lines first.
	parts[0].stop(false)
	close(parts[0].done)
	var next atomic.Int64
	next.Store(1)
	for range min(workers, len(parts)-1) {
		gr.wg.Go(func() {
			fc := NewFontConfiguration(fc.prog, FontConfigurationOptions{
				Limits:     d.lc.limits,
				FirstFaces: fc.first,
			})
			for {
				i := int(next.Add(1)) - 1
				if i >= len(parts) {
					return
				}
				gr.layOut(gctx, parts[i], fc)
			}
		})
	}
}

// stopGalley stops the galley of the document and drops its lines. A nil
// document has no galley.
func (d *Document) stopGalley() {
	if d == nil {
		return
	}
	gr := d.lc.lines.galley
	if gr == nil {
		return
	}
	gr.cancel()
	for _, p := range gr.parts {
		p.stop(false)
	}
	d.lc.lines.galley = nil
}

// layOut lays out the lines of a part. It reads, styles and builds the
// document again with fc, keeps the boxes of the part, and lays them out on
// a page of galleyHeight. A part whose layout fails or panics gives no
// lines.
func (gr *galleyReader) layOut(ctx context.Context, p *galleyPart, fc *FontConfiguration) {
	defer close(p.done)
	defer func() {
		if recover() != nil {
			p.stop(false)
		}
	}()
	if p.stopped.Load() || ctx.Err() != nil {
		return
	}
	pctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p.mu.Lock()
	p.cancel = cancel
	p.mu.Unlock()
	if p.stopped.Load() {
		return
	}
	doc, err := gr.plan.rebuild(fc)
	if err != nil {
		return
	}
	tree := newGalleyTree(doc.root)
	if tree.sum != gr.plan.tree.sum || len(tree.boxes) != len(gr.plan.tree.boxes) || galleyFaces(fc) != gr.facesKey {
		return
	}
	r := &galleyRecorder{reader: gr, part: p, styles: map[*Style]int{}, elements: map[*Element]int{}}
	for i := len(tree.boxes) - 1; i >= 0; i-- {
		r.styles[tree.styles[i]] = i
		if e := tree.elements[i]; e != nil {
			r.elements[e] = i
		}
	}
	tree.keep(0, p.lo, p.hi)
	doc.lc.lines.index = tree.index
	doc.lc.lines.recorder = r
	_ = doc.run(pctx, func() {
		doc.startLoop()
		for doc.iter != nil {
			doc.step()
		}
	})
	r.flush()
}

// keep removes from the box of index i the descendants that are out of the
// part from lo to hi, and keeps their ancestors.
func (t *galleyTree) keep(i, lo, hi int) {
	box := t.boxes[i]
	if i >= lo && t.end[i] <= hi {
		return
	}
	children := make([]*Box, 0, len(box.Children))
	for _, child := range box.Children {
		j := t.index[child]
		if t.end[j] <= lo || j >= hi {
			continue
		}
		t.keep(j, lo, hi)
		children = append(children, child)
	}
	box.Children = children
}

// galleyPage gives the page of a part of the galley the height of the
// galley.
func (lc *layoutContext) galleyPage(page *Box) {
	if lc.lines.recorder != nil {
		page.Height = galleyHeight
	}
}

// partOf returns the index of the part of the box of index i, or -1.
func (gr *galleyReader) partOf(i int) int {
	k, found := slices.BinarySearchFunc(gr.parts, i, func(p *galleyPart, i int) int {
		switch {
		case p.hi <= i:
			return -1
		case p.lo > i:
			return 1
		}
		return 0
	})
	if !found {
		return -1
	}
	return k
}

// take returns a copy of the line of the galley of the key, and where the
// next line resumes. When the page is complete, the pages stop the parts
// before the part of the line, see galleyPageDone.
func (gr *galleyReader) take(lc *layoutContext, key galleyKey) (*Box, skipStack, bool) {
	if key.box < 0 {
		return nil, nil, false
	}
	k := gr.partOf(key.box)
	if k < 0 {
		return nil, nil, false
	}
	gr.reached = max(gr.reached, k)
	p := gr.parts[k]
	if gr.wait {
		<-p.done
	}
	if lc.fontConfig.db.Len() != gr.faces {
		return nil, nil, false
	}
	p.mu.Lock()
	gl := p.lines[key]
	p.mu.Unlock()
	if gl == nil {
		return nil, nil, false
	}
	lc.lines.taken++
	return copyGalleyBox(gl.line, lc.fontConfig), gl.resumeAt.clone(), true
}

// galleyPageDone stops the parts before the part of the last line that the
// pages took, when the page maker completes a page. A page that the page
// maker makes again, for a page float of -ivy-float-position, can take the
// lines of the parts that its first layout passed (D60).
func (lc *layoutContext) galleyPageDone() {
	gr := lc.lines.galley
	if gr == nil {
		return
	}
	for ; gr.passed < gr.reached; gr.passed++ {
		gr.parts[gr.passed].stop(gr.keep)
	}
}

// galleyRecorder keeps the lines of a part of the galley. It maps the
// pointers of the lines onto the boxes of the pages by their index.
type galleyRecorder struct {
	reader *galleyReader
	part   *galleyPart
	// styles and elements hold the first index of each style and element
	// of the boxes of the part.
	styles   map[*Style]int
	elements map[*Element]int
	batch    []galleyBatchLine
}

// galleyBatchLine is a line of a batch of a part, with its key.
type galleyBatchLine struct {
	key  galleyKey
	line galleyLine
}

// record keeps a line of the part, laid out at a height of 0. It keeps no
// line of a box out of the part, and no line that holds a box other than
// an inline box or a text box, a note or a footnote call, or a style that
// the boxes of the pages do not have.
func (r *galleyRecorder) record(key galleyKey, line *Box, resumeAt skipStack) {
	if key.box < r.part.lo || key.box >= r.part.hi || r.part.stopped.Load() {
		return
	}
	if r.reader.lines.Load() >= r.reader.maxLines {
		return
	}
	c, ok := r.mapBox(line)
	if !ok {
		return
	}
	r.reader.lines.Add(1)
	r.batch = append(r.batch, galleyBatchLine{key: key, line: galleyLine{line: c, resumeAt: resumeAt.clone()}})
	if len(r.batch) >= galleyBatch {
		r.flush()
	}
}

// flush gives the lines of the batch to the pages.
func (r *galleyRecorder) flush() {
	p := r.part
	p.mu.Lock()
	if p.lines != nil {
		for _, b := range r.batch {
			p.lines[b.key] = &b.line
		}
	}
	p.mu.Unlock()
	r.batch = r.batch[:0]
}

// mapBox returns a deep copy of a box of a line of the part, with the
// styles and the elements of the boxes of the pages. It returns false for
// a box that the pages cannot take.
func (r *galleyRecorder) mapBox(g *Box) (*Box, bool) {
	switch g.Kind {
	case KindLine, KindInline, KindText:
	default:
		return nil, false
	}
	if g.Background != nil || g.tbl != nil || g.flx != nil || g.placeholder != notPlaceholder {
		return nil, false
	}
	plan := &r.reader.plan.tree
	c := new(Box)
	*c = *g
	var ok bool
	if c.Style, ok = r.style(g.Style); !ok {
		return nil, false
	}
	if g.Element != nil {
		i, ok := r.elements[g.Element]
		if !ok {
			return nil, false
		}
		c.Element = plan.elements[i]
	}
	if g.ext != nil {
		e := *g.ext
		if e.Replacement != nil || e.Footnote != nil || e.Note != nil || e.NoteCall != nil ||
			e.NoteCallback != nil || e.CachedCounterValues != nil || e.MissingLink != nil ||
			e.TransformationMatrix != nil || e.Advancements != nil || e.Page != nil ||
			e.CanvasBackground != nil || e.fixedBoxes != nil {
			return nil, false
		}
		if e.FirstLetterStyle, ok = r.style(e.FirstLetterStyle); !ok {
			return nil, false
		}
		if e.FirstLineStyle, ok = r.style(e.FirstLineStyle); !ok {
			return nil, false
		}
		e.StringSet = slices.Clone(e.StringSet)
		c.ext = &e
	}
	if g.TextLayout != nil {
		l := *g.TextLayout
		if l.Style, ok = r.style(l.Style); !ok {
			return nil, false
		}
		l.fc = nil
		c.TextLayout = &l
	}
	if c.Children != nil {
		c.Children = make([]*Box, len(g.Children))
		for i, child := range g.Children {
			if c.Children[i], ok = r.mapBox(child); !ok {
				return nil, false
			}
		}
	}
	copyGalleyFields(c, g)
	return c, true
}

// style returns the style of the pages of a style of the part. nil stays
// nil.
func (r *galleyRecorder) style(s *Style) (*Style, bool) {
	if s == nil {
		return nil, true
	}
	i, ok := r.styles[s]
	if !ok {
		return nil, false
	}
	return r.reader.plan.tree.styles[i], true
}

// copyGalleyFields gives c its own copies of the fields of g that the
// layout can change in place.
func copyGalleyFields(c, g *Box) {
	if g.removedSides != nil {
		s := *g.removedSides
		c.removedSides = &s
	}
	if g.radii != nil {
		r := *g.radii
		c.radii = &r
	}
	if g.TextLayout != nil && g.TextLayout.MaxWidth != nil {
		w := *g.TextLayout.MaxWidth
		c.TextLayout.MaxWidth = &w
	}
	c.resumeAt = g.resumeAt.clone()
}

// copyGalleyBox returns a deep copy of a line of the galley for the pages,
// whose text layouts measure with fc.
func copyGalleyBox(g *Box, fc *FontConfiguration) *Box {
	c := new(Box)
	*c = *g
	if g.ext != nil {
		e := *g.ext
		e.StringSet = slices.Clone(e.StringSet)
		c.ext = &e
	}
	if g.TextLayout != nil {
		l := *g.TextLayout
		l.fc = fc
		if l.Style != nil {
			l.fc = fontConfigOf(l.Style)
		}
		c.TextLayout = &l
	}
	if g.Children != nil {
		c.Children = make([]*Box, len(g.Children))
		for i, child := range g.Children {
			c.Children[i] = copyGalleyBox(child, fc)
		}
	}
	copyGalleyFields(c, g)
	return c
}
