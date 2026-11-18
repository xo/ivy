// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/formatting_structure/build.py: it turns a tree
// of elements with their styles into a tree of boxes, before the layout.
//
// This includes making the anonymous boxes, and processing the white space
// as necessary.
//
// Port: upstream passes the arguments of the building, such as style_for,
// get_image_from_uri and target_collector, from function to function. The
// port keeps them in a builder, whose methods are the functions of upstream.

package layout

import (
	"fmt"
	"slices"
	"strings"

	"github.com/xo/ivy"
)

// boxKindFromDisplay is BOX_TYPE_FROM_DISPLAY in
// weasyprint/formatting_structure/build.py: the kinds of boxes of the values
// of display, by their first two keywords.
var boxKindFromDisplay = map[string]Kind{
	"block flow":         KindBlock,
	"inline flow":        KindInline,
	"block flow-root":    KindBlock,
	"inline flow-root":   KindInlineBlock,
	"block table":        KindTable,
	"inline table":       KindInlineTable,
	"block flex":         KindFlex,
	"inline flex":        KindInlineFlex,
	"block grid":         KindGrid,
	"inline grid":        KindInlineGrid,
	"table-row":          KindTableRow,
	"table-row-group":    KindTableRowGroup,
	"table-header-group": KindTableRowGroup,
	"table-footer-group": KindTableRowGroup,
	"table-column":       KindTableColumn,
	"table-column-group": KindTableColumnGroup,
	"table-cell":         KindTableCell,
	"table-caption":      KindTableCaption,
}

// maxRowspanCells bounds the cells that the row spans of a table mark as
// occupied.
//
// Port: upstream has no bound. A hostile table of many rows, each with a
// cell that spans all the rows and many columns, takes a time in the cube
// of its size.
const maxRowspanCells = 1 << 24

// PageContext is the context of the layout of a page that the content of
// its margin boxes reads: the named strings of string() and the running
// elements of element(). The layout of the pages gives it.
type PageContext interface {
	// GetStringSetFor is LayoutContext.get_string_set_for: the value of a
	// named string on a page, for a keyword such as "first".
	GetStringSetFor(page *Box, name, keyword string) string
	// GetRunningElementFor is LayoutContext.get_running_element_for: the
	// boxes of a running element on a page, for a keyword such as
	// "first".
	GetRunningElementFor(page *Box, name, keyword string) []*Box
}

// builder holds the arguments of the functions of
// weasyprint/formatting_structure/build.py, and the guards of the port.
type builder struct {
	styleFor        func(element *Element, pseudoType string) *Style
	images          *imageLoader
	fetcher         *URLFetcher
	baseURL         string
	targetCollector *TargetCollector
	counterStyle    CounterStyle
	footnotes       []*Box
	log             *logger
	limits          ivy.Limits

	// boxes counts the boxes that the builder makes, and err holds the
	// error of a limit.
	boxes int
	err   error
}

// BuildOptions holds the arguments of BuildFormattingStructure.
type BuildOptions struct {
	// TargetCollector collects the anchors of the targets. It must be the
	// collector of GetAllComputedStyles, or nil.
	TargetCollector *TargetCollector
	// CounterStyle holds the counter styles of the style sheets, as
	// ComputedStylesOptions.CounterStyle, or nil for those of the user
	// agent.
	CounterStyle CounterStyle
}

// newBuilder returns a builder for a document.
func newBuilder(html *HTML, styleFor *StyleFor, opts BuildOptions) *builder {
	b := &builder{
		styleFor: func(element *Element, pseudoType string) *Style {
			return styleFor.Get(element, pseudoType)
		},
		fetcher:         html.URLFetcher,
		baseURL:         html.BaseURL,
		targetCollector: opts.TargetCollector,
		counterStyle:    opts.CounterStyle,
		log:             html.log,
		limits:          html.Limits.WithDefaults(),
	}
	b.images = newImageLoader(html.URLFetcher, b.limits, b.log)
	if b.targetCollector == nil {
		b.targetCollector = NewTargetCollector(html.log)
	}
	if b.counterStyle == nil {
		b.counterStyle = userAgentStylesheets().counterStyle
	}
	return b
}

// spend counts n boxes, and returns false past the limit.
//
// Port: upstream has no limit. A document makes at most 4 times
// Limits.MaxObjects boxes.
func (b *builder) spend(n int) bool {
	if b.err != nil {
		return false
	}
	b.boxes += n
	if b.boxes > 4*b.limits.MaxObjects {
		b.err = fmt.Errorf("%w: the document makes too many boxes", ivy.ErrLimit)
		return false
	}
	return true
}

// BuildFormattingStructure is build_formatting_structure in
// weasyprint/formatting_structure/build.py. It builds the box tree of a
// document, from its element tree and its styles, and returns the box of
// the root element and the footnotes.
//
// It returns an error that wraps ivy.ErrLimit when the document makes too
// many boxes, and ivy.ErrFormat for a panic.
//
// Port: a panic becomes an error that wraps ivy.ErrFormat.
func BuildFormattingStructure(html *HTML, styleFor *StyleFor, opts BuildOptions) (box *Box, footnotes []*Box, err error) {
	defer func() {
		if r := recover(); r != nil {
			box, footnotes, err = nil, nil, fmt.Errorf("%w: panic in the building of the boxes: %v", ivy.ErrFormat, r)
		}
	}()
	b := newBuilder(html, styleFor, opts)
	box, err = b.buildFormattingStructure(html.EtreeElement)
	return box, b.footnotes, err
}

// buildFormattingStructure is build_formatting_structure.
func (b *builder) buildFormattingStructure(elementTree *Element) (*Box, error) {
	boxList := b.elementToBoxRoot(elementTree)
	var box *Box
	if len(boxList) > 0 {
		box = boxList[0]
	} else {
		// No root element
		styleFor := b.styleFor
		b.styleFor = func(element *Element, pseudoType string) *Style {
			style := styleFor(element, pseudoType)
			if style != nil {
				if element == elementTree {
					style.Set("display", tupleOf("block", "flow"))
				} else {
					style.Set("display", tupleOf("none"))
				}
			}
			return style
		}
		boxList = b.elementToBoxRoot(elementTree)
		if len(boxList) == 0 {
			if b.err != nil {
				return nil, b.err
			}
			// Port: upstream fails to unpack an empty list.
			return nil, fmt.Errorf("%w: the document has no root box", ivy.ErrFormat)
		}
		box = boxList[0]
	}
	if b.err != nil {
		return nil, b.err
	}

	b.targetCollector.CheckPendingTargets()
	box.ProcessWhitespace(false)
	box.ProcessTextTransform()
	// Port: upstream has no tab stops. The tabs of -ivy-tab-stops take
	// text boxes of their own.
	splitTabs(box)

	box.IsForRootElement = true
	// If this is changed, maybe update weasy.layout.page.make_margin_boxes()
	box = createAnonymousBoxes(b, box)
	box = setViewportOverflow(box)
	if b.err != nil {
		return nil, b.err
	}
	return box, nil
}

// createAnonymousBoxes is create_anonymous_boxes in
// weasyprint/formatting_structure/build.py. It makes the anonymous boxes in
// the descendants of a box, as the rules of the layout say.
func createAnonymousBoxes(b *builder, box *Box) *Box {
	box = anonymousTableBoxes(b, box)
	box = flexBoxes(box)
	box = gridBoxes(box)
	box = inlineInBlock(box)
	box = blockInInline(box)
	return box
}

// makeBox is make_box in weasyprint/formatting_structure/build.py.
func makeBox(elementTag string, style *Style, content []*Box, element *Element) *Box {
	display, _ := style.Get("display").(Tuple)
	key := make([]string, 0, 2)
	for _, item := range display[:min(len(display), 2)] {
		key = append(key, str(item))
	}
	kind, ok := boxKindFromDisplay[strings.Join(key, " ")]
	if !ok {
		// Port: upstream raises KeyError for a display that has no box,
		// such as none. The box is a block box.
		kind = KindBlock
	}
	return newBox(kind, elementTag, style, element, content)
}

// elementToBoxRoot is element_to_box for the root element, with the
// counters and the quotes of a new document.
func (b *builder) elementToBoxRoot(element *Element) []*Box {
	quoteDepth := 0
	counterValues := map[string][]int{"footnote": {0}, "note": {0}}
	counterScopes := []map[string]bool{{"footnote": true, "note": true}}
	return b.elementToBox(element, &quoteDepth, counterValues, &counterScopes)
}

// elementToBox is element_to_box in weasyprint/formatting_structure/
// build.py. It converts an element and its children into a list of boxes.
// Most of the time the list has one item, but it can have zero or more than
// one.
//
// For example:
//
//	<p>Some <em>emphasised</em> text.</p>
//
// gives (not actual syntax):
//
//	BlockBox[
//	    TextBox['Some '],
//	    InlineBox[
//	        TextBox['emphasised'],
//	    ],
//	    TextBox[' text.'],
//	]
//
// The TextBoxes are anonymous inline boxes. See
// https://www.w3.org/TR/CSS21/visuren.html#anonymous.
func (b *builder) elementToBox(element *Element, quoteDepth *int, counterValues map[string][]int, counterScopes *[]map[string]bool) []*Box {
	if !b.spend(1) {
		return nil
	}
	style := b.styleFor(element, "")
	if style == nil {
		// Port: each element has a style in upstream.
		return nil
	}

	// TODO: should be the used value. When does the used value for
	// `display` differ from the computer value?
	display := style.Get("display")
	if Equal(display, tupleOf("none")) {
		return nil
	}

	if isStr(style.Get("float"), "footnote") {
		if isStr(style.Get("footnote_display"), "block") {
			style.Set("display", tupleOf("block", "flow"))
		} else {
			// TODO: handle compact footnotes
			style.Set("display", tupleOf("inline", "flow"))
		}
	}

	box := makeBox(element.Tag, style, nil, element)
	// Port: the box keeps the styles in boxExtra, which it makes only for
	// a style that is not nil.
	if firstLetter := b.styleFor(element, "first-letter"); firstLetter != nil {
		box.ownExtra().FirstLetterStyle = firstLetter
	}
	if firstLine := b.styleFor(element, "first-line"); firstLine != nil {
		box.ownExtra().FirstLineStyle = firstLine
	}

	updateCounters(counterValues, *counterScopes, style)
	// If this element’s direct children create new scopes, the counter
	// names will be in this new list.
	*counterScopes = append(*counterScopes, map[string]bool{})

	var children []*Box

	var markerBoxes []*Box
	if tupleHas(style.Get("display"), "list-item") {
		markerBoxes = b.markerToBox(element, quoteDepth, counterValues, counterScopes, style)
		children = append(children, markerBoxes...)
	}

	children = append(children, b.beforeAfterToBox(element, "before", quoteDepth, counterValues, counterScopes)...)

	// collect anchor's counter_values, maybe it's a target.
	// to get the spec-conform counter_values we must do it here,
	// after the ::before is parsed and before the ::after is
	if anchor := style.Get("anchor"); truthy(anchor) {
		b.targetCollector.StoreTarget(str(anchor), counterValues, box)
	}

	if text := element.Text; text != "" {
		children = append(children, anonymousTextFrom(box, text))
	}

	for _, childElement := range element.Children {
		childBoxes := b.elementToBox(childElement, quoteDepth, counterValues, counterScopes)

		if len(childBoxes) > 0 && isStr(childBoxes[0].Style.Get("float"), "footnote") {
			footnote := childBoxes[0]
			footnote.Style = footnote.Style.Copy()
			footnote.Style.Set("float", String("none"))
			b.footnotes = append(b.footnotes, footnote)
			callStyle := b.styleFor(footnote.Element, "footnote-call")
			if callStyle == nil {
				// Port: upstream fails when the call has no style.
				callStyle = footnote.Style.AnonymousStyle()
			}
			footnoteCall := makeBox(footnote.Element.Tag+"::footnote-call", callStyle, nil, footnote.Element)
			footnoteCall.Children = b.contentToBoxes(callStyle, footnoteCall, quoteDepth, counterValues, nil, nil)
			footnoteCall.ownExtra().Footnote = footnote
			childBoxes = []*Box{footnoteCall}
		}
		if len(childBoxes) > 0 && childBoxes[0].IsNote() {
			note := childBoxes[0]
			note.Style = note.Style.Copy()
			callStyle := b.styleFor(note.Element, "note-call")
			if callStyle == nil {
				// Port: upstream fails when the call has no style.
				callStyle = note.Style.AnonymousStyle()
			}
			noteCall := makeBox(note.Element.Tag+"::note-call", callStyle, nil, note.Element)
			noteCall.Children = b.contentToBoxes(callStyle, noteCall, quoteDepth, counterValues, nil, nil)
			note.ownExtra().NoteCall, noteCall.ownExtra().Note = noteCall, note
			childBoxes = []*Box{noteCall, note}
		}

		children = append(children, childBoxes...)
		if text := childElement.Tail; text != "" {
			// Port: upstream makes the text box before it checks the last
			// child. The port does not make a box that it drops.
			if len(children) > 0 && children[len(children)-1].Kind == KindText {
				children[len(children)-1].Text += text
			} else {
				children = append(children, anonymousTextFrom(box, text))
			}
		}
	}

	children = append(children, b.beforeAfterToBox(element, "after", quoteDepth, counterValues, counterScopes)...)

	// Scopes created by this element’s children stop here.
	scopes := *counterScopes
	for name := range scopes[len(scopes)-1] {
		if values := counterValues[name]; len(values) > 0 {
			counterValues[name] = values[:len(values)-1]
		}
		if len(counterValues[name]) == 0 {
			delete(counterValues, name)
		}
	}
	*counterScopes = scopes[:len(scopes)-1]

	box.Children = children
	b.setContentLists(element, box, style, counterValues)

	if len(markerBoxes) > 0 && len(box.Children) == 1 {
		// See https://www.w3.org/TR/css-lists-3/#list-style-position-outside
		//
		// "The size or contents of the marker box may affect the height of
		//  the principal block box and/or the height of its first line box,
		//  and in some cases may cause the creation of a new line box; this
		//  interaction is also not defined."
		//
		// We decide here to add a zero-width space to have a minimum
		// height. Adding text boxes is not the best idea, but it's not a
		// good moment to add an empty line box, and the specification lets
		// us do almost what we want, so…
		if isStr(style.Get("list_style_position"), "outside") {
			box.Children = append(box.Children, anonymousTextFrom(box, "\u200b"))
		}
	}

	switch {
	case isStr(style.Get("float"), "footnote"):
		if len(counterValues["footnote"]) == 0 {
			// Port: upstream fails when the counter is missing.
			counterValues["footnote"] = []int{0}
		}
		counterValues["footnote"][len(counterValues["footnote"])-1]++
		if markerStyle := b.styleFor(element, "footnote-marker"); markerStyle != nil {
			marker := makeBox(element.Tag+"::footnote-marker", markerStyle, nil, element)
			marker.Children = b.contentToBoxes(markerStyle, marker, quoteDepth, counterValues, nil, nil)
			box.Children = slices.Insert(box.Children, 0, marker)
		}
	case positionFunction(style.Get("position")) == "note()":
		if len(counterValues["note"]) == 0 {
			// Port: upstream fails when the counter is missing.
			counterValues["note"] = []int{0}
		}
		counterValues["note"][len(counterValues["note"])-1]++
		if markerStyle := b.styleFor(element, "note-marker"); markerStyle != nil {
			marker := makeBox(element.Tag+"::note-marker", markerStyle, nil, element)
			marker.Children = b.contentToBoxes(markerStyle, marker, quoteDepth, counterValues, nil, nil)
			box.Children = slices.Insert(box.Children, 0, marker)
		}
		if callbackStyle := b.styleFor(element, "note-callback"); callbackStyle != nil {
			callback := makeBox(element.Tag+"::note-callback", callbackStyle, nil, element)
			callback.Children = b.contentToBoxes(callbackStyle, callback, quoteDepth, counterValues, nil, nil)
			box.Children = append(box.Children, callback)
			box.ownExtra().NoteCallback = callback
		}
	}

	// Specific handling for the element. (eg. replaced element)
	return handleElement(b, element, box, b.baseURL)
}

// beforeAfterToBox is before_after_to_box in
// weasyprint/formatting_structure/build.py. It returns the boxes of the
// pseudo-element ::before or ::after.
func (b *builder) beforeAfterToBox(element *Element, pseudoType string, quoteDepth *int, counterValues map[string][]int, counterScopes *[]map[string]bool) []*Box {
	style := b.styleFor(element, pseudoType)
	if pseudoType != "" && style == nil {
		// Pseudo-elements with no style at all do not get a style dict.
		// Their initial content property computes to 'none'.
		return nil
	}

	if Equal(style.Get("display"), tupleOf("none")) {
		return nil
	}
	if isStrIn(style.Get("content"), "normal", "inhibit", "none") {
		return nil
	}
	if !b.spend(1) {
		return nil
	}

	updateCounters(counterValues, *counterScopes, style)

	box := makeBox(element.Tag+"::"+pseudoType, style, nil, element)
	var children []*Box
	if tupleHas(style.Get("display"), "list-item") {
		children = append(children, b.markerToBox(element, quoteDepth, counterValues, counterScopes, style)...)
	}
	children = append(children, b.contentToBoxes(style, box, quoteDepth, counterValues, nil, nil)...)
	box.Children = children

	b.setContentLists(element, box, style, counterValues)

	return []*Box{box}
}

// markerToBox is marker_to_box in weasyprint/formatting_structure/build.py.
// It returns the box of the pseudo-element ::marker, if there is one. See
// https://drafts.csswg.org/css-lists-3/#marker-pseudo.
func (b *builder) markerToBox(element *Element, quoteDepth *int, counterValues map[string][]int, _ *[]map[string]bool, parentStyle *Style) []*Box {
	style := b.styleFor(element, "marker")
	if style == nil {
		style = parentStyle.AnonymousStyle()
	}

	// Port: upstream makes the box before it checks display: none, and
	// fails to make a box of display: none.
	if Equal(style.Get("display"), tupleOf("none")) {
		return nil
	}
	if !b.spend(1) {
		return nil
	}

	var children []*Box

	box := makeBox(element.Tag+"::marker", style, children, element)

	image, _ := style.Get("list_style_image").(Tuple)
	var imageType string
	var imageValue Value
	if len(image) == 2 {
		imageType, imageValue = str(image[0]), image[1]
	}

	if !isStrIn(style.Get("content"), "normal", "inhibit") {
		children = append(children, b.contentToBoxes(style, box, quoteDepth, counterValues, nil, nil)...)
	} else {
		if imageType == "url" {
			// image may be None here too, in case the image is not
			// available.
			if img := b.images.get(str(imageValue), "", style.Get("image_orientation")); img != nil {
				box = anonymousReplacedFrom(KindInlineReplaced, box, img)
				children = append(children, box)
			}
		}

		if len(children) == 0 && !isStr(style.Get("list_style_type"), "none") {
			counterValue := 0
			if values := counterValues["list-item"]; len(values) > 0 {
				counterValue = values[len(values)-1]
			}
			counterType := style.Get("list_style_type")
			if markerText := b.counterStyle.RenderMarker(counterType, counterValue); markerText != "" {
				box = anonymousTextFrom(box, markerText)
				children = append(children, box)
			}
		}
	}

	if len(children) == 0 {
		return nil
	}

	var markerBox *Box
	if isStr(parentStyle.Get("list_style_position"), "outside") {
		markerBox = anonymousFrom(KindBlock, box, children)
		// We can safely edit everything that can't be changed by user style
		// See https://drafts.csswg.org/css-pseudo-4/#marker-pseudo
		markerBox.Style = markerBox.Style.Copy()
		markerBox.Style.Set("position", String("absolute"))
		markerBox.IsOutsideMarker = true
	} else {
		markerBox = anonymousFrom(KindInline, box, children)
	}
	return []*Box{markerBox}
}

// parseAgainFunc is the type of the functions parse_again of upstream,
// which compute a content list again with the counters of the targets.
type parseAgainFunc = func(counters map[string][]int)

// computeContentList is compute_content_list in
// weasyprint/formatting_structure/build.py. It computes and returns the
// boxes of a content list, and false for None.
//
// parseAgain computes the content list again when LookupTarget finds a
// pending target. BuildFormattingStructure calls
// TargetCollector.CheckPendingTargets after the first pass, to parse what
// it needs again.
//
// withImages is false for the None of get_image_from_uri, quoteDepth is
// nil for None, and context and page are the layout of the page of a margin
// box, or nil.
func (b *builder) computeContentList(contentList Value, parentBox *Box, counterValues map[string][]int, cssToken string, parseAgain parseAgainFunc, withImages bool, quoteDepth *int, quoteStyle, lang Value, context PageContext, page *Box) ([]*Box, bool) {
	// TODO: Some computation done here may be done in computed_values
	// instead. We currently miss at least style_for, counters and quotes
	// context in computer. Some work will still need to be done here
	// though, like box creation for URIs.

	var contentBoxes []*Box
	hasText := false

	addText := func(text string) {
		hasText = true
		if text != "" {
			if len(contentBoxes) > 0 && contentBoxes[len(contentBoxes)-1].Kind == KindText {
				contentBoxes[len(contentBoxes)-1].Text += text
			} else if b.spend(1) {
				contentBoxes = append(contentBoxes, anonymousTextFrom(parentBox, text))
			}
		}
	}

	// Port: upstream rebinds missing_counters to the list of the missing
	// counters of the anchor of a target-counter(), so that the Counter
	// gets this list as its missing counters, and the counter() items that
	// follow append to it. The page based target counters rely on it: they
	// make the page again when its page counters change. The port keeps
	// pointers to the lists to do the same.
	var localMissingCounters []string
	missingCounters := &localMissingCounters
	missingTargetCounters := map[string]*[]string{}
	var missingTargetOrder []string
	inPageContext := context != nil && page != nil

	if parentBox.extra().CachedCounterValues == nil {
		// Store the counter_values in the parent_box to make them
		// accessible in @page context.
		parentBox.ownExtra().CachedCounterValues = copyCounterValues(counterValues)
	}
	items, _ := contentList.(Tuple)
loop:
	for _, item := range items {
		pair, ok := item.(Tuple)
		if !ok || len(pair) != 2 {
			continue
		}
		typ, value := str(pair[0]), pair[1]
		args, _ := value.(Tuple)
		switch {
		case typ == "string":
			addText(str(value))
		case typ == "url" && withImages:
			if len(args) != 2 {
				continue
			}
			origin, uri := str(args[0]), str(args[1])
			if origin != "external" {
				// Embedding internal references is impossible.
				continue
			}
			if image := b.images.get(uri, "", parentBox.Style.Get("image_orientation")); image != nil && b.spend(1) {
				contentBoxes = append(contentBoxes, anonymousReplacedFrom(KindInlineReplaced, parentBox, image))
			}
		case typ == "attr()":
			if len(args) != 3 {
				continue
			}
			attrName, attrType := str(args[0]), str(args[1])
			if attrType == "string" {
				text := ""
				if parentBox.Element != nil {
					text = parentBox.Element.GetDefault(attrName, "")
				}
				addText(text)
			} else {
				b.log.warning("Only strings are allowed for content attr() functions, not %s", attrType)
			}
		case typ == "content()":
			addText(parentBox.ExtractText(str(value)))
		case typ == "string()":
			if !inPageContext {
				// string() is currently only valid in @page context.
				// See issue #723.
				b.log.warning(`"string(%s)" is only allowed in page margins`, strings.Join(valueStrings(value), " "))
				continue
			}
			if len(args) == 2 {
				addText(context.GetStringSetFor(page, str(args[0]), str(args[1])))
			}
		case typ == "counter()" || typ == "counters()":
			if len(args) < 2 {
				continue
			}
			counterName, counterType := str(args[0]), args[len(args)-1]
			if isStr(counterType, "none") {
				continue
			}
			if !inPageContext {
				if _, ok := counterValues[counterName]; !ok && !slices.Contains(*missingCounters, counterName) {
					*missingCounters = append(*missingCounters, counterName)
				}
			}
			values := counterValues[counterName]
			if len(values) == 0 {
				values = []int{0}
			}
			var text string
			if typ == "counter()" {
				text = b.counterStyle.RenderValue(values[len(values)-1], counterType)
			} else {
				separator := str(args[1])
				parts := make([]string, len(values))
				for i, counterValue := range values {
					parts[i] = b.counterStyle.RenderValue(counterValue, counterType)
				}
				text = strings.Join(parts, separator)
			}
			addText(text)
		case typ == "target-counter()" || typ == "target-counters()":
			if len(args) < 3 {
				continue
			}
			anchorToken, counterName, counterType := args[0], str(args[1]), args[len(args)-1]
			if isStr(counterType, "none") {
				continue
			}
			lookupTarget := b.targetCollector.LookupTarget(anchorToken, parentBox, cssToken, parseAgain)
			if lookupTarget.State != "up-to-date" {
				break loop
			}
			var targetValues map[string][]int
			if lookupTarget.TargetBox != nil {
				targetValues = lookupTarget.TargetBox.extra().CachedCounterValues
			}
			if _, ok := targetValues[counterName]; !inPageContext && !ok {
				anchorName, _ := anchorNameFromToken(anchorToken)
				missing := missingTargetCounters[anchorName]
				if missing == nil {
					missing = &[]string{}
					missingTargetCounters[anchorName] = missing
					missingTargetOrder = append(missingTargetOrder, anchorName)
				}
				missingCounters = missing
				if !slices.Contains(*missing, counterName) {
					*missing = append(*missing, counterName)
				}
			}
			// Mixin target's cached page counters.
			// cached_page_counter_values are empty during layout.
			localCounters := cloneCounterValues(lookupTarget.CachedPageCounterValues)
			for key, value := range targetValues {
				localCounters[key] = value
			}
			values := localCounters[counterName]
			if len(values) == 0 {
				values = []int{0}
			}
			var text string
			if typ == "target-counter()" {
				text = b.counterStyle.RenderValue(values[len(values)-1], counterType)
			} else {
				if len(args) < 4 {
					break loop
				}
				separator, _ := args[2].(Tuple)
				if len(separator) != 2 || !isStr(separator[0], "string") {
					break loop
				}
				separatorString := str(separator[1])
				parts := make([]string, len(values))
				for i, counterValue := range values {
					parts[i] = b.counterStyle.RenderValue(counterValue, counterType)
				}
				text = strings.Join(parts, separatorString)
			}
			addText(text)
		case typ == "target-text()":
			if len(args) != 2 {
				continue
			}
			anchorToken, textStyle := args[0], str(args[1])
			lookupTarget := b.targetCollector.LookupTarget(anchorToken, parentBox, cssToken, parseAgain)
			if lookupTarget.State != "up-to-date" || lookupTarget.TargetBox == nil {
				break loop
			}
			targetBox := lookupTarget.TargetBox
			// TODO: 'before'- and 'after'- content referring missing
			// counters are not properly set.
			addText(targetBox.ExtractText(textStyle))
		case typ == "quote" && quoteDepth != nil && quoteStyle != nil:
			quote := str(value)
			isOpen := strings.Contains(quote, "open")
			insert := !strings.HasPrefix(quote, "no-") && !isStr(quoteStyle, "none")
			if !isOpen {
				*quoteDepth = max(0, *quoteDepth-1)
			}
			if insert {
				var openQuotes, closeQuotes []string
				if isStr(quoteStyle, "auto") {
					quotes := getLangQuotes(str(lang))
					openQuotes, closeQuotes = quotes[0], quotes[1]
				} else if t, ok := quoteStyle.(Tuple); ok && len(t) == 2 {
					openQuotes, closeQuotes = valueStrings(t[0]), valueStrings(t[1])
				}
				quotes := closeQuotes
				if isOpen {
					quotes = openQuotes
				}
				if len(quotes) > 0 {
					addText(quotes[min(*quoteDepth, len(quotes)-1)])
				}
			}
			if isOpen {
				*quoteDepth++
			}
		case typ == "element()":
			if !inPageContext {
				b.log.warning(`"element(%s)" is only allowed in page margins`, strings.Join(valueStrings(value), " "))
				continue
			}
			if len(args) != 2 {
				continue
			}
			for _, newBox := range context.GetRunningElementFor(page, str(args[0]), str(args[1])) {
				newBox = newBox.DeepCopy()
				newBox.Style = newBox.Style.Copy()
				newBox.Style.Set("position", String("relative"))
				if !isStr(args[1], "all-once") {
					// Use page counters instead of box counters.
					for _, child := range newBox.descendantsList() {
						if isStrIn(child.Style.Get("content"), "normal", "none") {
							continue
						}
						child.Children = b.contentToBoxes(child.Style, child, quoteDepth, counterValues, context, page)
					}
				}
				contentBoxes = append(contentBoxes, newBox)
			}
		case typ == "leader()":
			if len(args) != 2 || str(args[1]) == "" {
				continue
			}
			if !b.spend(2) {
				continue
			}
			textBox := anonymousTextFrom(parentBox, str(args[1]))
			leaderBox := anonymousFrom(KindInline, parentBox, []*Box{textBox})
			// Avoid breaks inside the leader box.
			leaderBox.Style.Set("white_space", String("pre"))
			// Prevent whitespaces from being removed from the text box.
			textBox.Style.Set("white_space", String("pre"))
			leaderBox.IsLeader = true
			contentBoxes = append(contentBoxes, leaderBox)
		}
	}

	if hasText || len(contentBoxes) > 0 {
		// Only add Counter if the content_list actually produced text
		var missingTargets map[string][]string
		if len(missingTargetCounters) > 0 {
			missingTargets = make(map[string][]string, len(missingTargetCounters))
			for _, anchorName := range missingTargetOrder {
				missingTargets[anchorName] = *missingTargetCounters[anchorName]
			}
		}
		b.targetCollector.CollectMissingCounters(parentBox, cssToken, parseAgain, *missingCounters, missingTargets)
		return contentBoxes, true
	}
	return nil, false
}

// contentToBoxes is content_to_boxes in weasyprint/formatting_structure/
// build.py. It takes the value of a property content and returns boxes.
// context and page are the layout of the page of a margin box, or nil.
func (b *builder) contentToBoxes(style *Style, parentBox *Box, quoteDepth *int, counterValues map[string][]int, context PageContext, page *Box) []*Box {
	if isStr(style.Get("content"), "inhibit") {
		return nil
	}

	// The functions parse_again share one copy of the depth of the quotes,
	// as the list orig_quote_depth of upstream.
	origQuoteDepth := new(int)
	*origQuoteDepth = *quoteDepth
	parseAgain := func(counters map[string][]int) {
		// Parse the children of the parent_boxes all again.
		localCounters := cloneCounterValues(counters)
		for key, value := range parentBox.extra().CachedCounterValues {
			localCounters[key] = value
		}

		localChildren := b.contentToBoxes(style, parentBox, origQuoteDepth, localCounters, nil, nil)

		// TODO: redo the formatting structure of the parent instead of
		// hacking the already formatted structure. Find why
		// inline_in_blocks has sometimes already been called, and
		// sometimes not.
		if len(parentBox.Children) == 1 && parentBox.Children[0].Kind == KindLine {
			parentBox.Children[0].Children = localChildren
		} else {
			parentBox.Children = localChildren
		}
	}

	cssToken := "content"
	boxList, _ := b.computeContentList(
		style.Get("content"), parentBox, counterValues, cssToken, parseAgain,
		true, quoteDepth, style.Get("quotes"), style.Get("lang"), context, page)
	return boxList
}

// computeStringSet is compute_string_set in weasyprint/formatting_structure/
// build.py. It parses the content list of a named string of string-set.
func (b *builder) computeStringSet(element *Element, box *Box, stringName string, contentList Value, counterValues map[string][]int) {
	parseAgain := func(counters map[string][]int) {
		// Parse the string-set string value all again.
		localCounters := cloneCounterValues(counters)
		for key, value := range box.extra().CachedCounterValues {
			localCounters[key] = value
		}
		b.computeStringSet(element, box, stringName, contentList, localCounters)
	}

	cssToken := "string-set::" + stringName
	boxList, ok := b.computeContentList(contentList, box, counterValues, cssToken, parseAgain, false, nil, nil, nil, nil, nil)
	if ok {
		var text strings.Builder
		for _, child := range boxList {
			if child.Kind == KindText {
				text.WriteString(child.Text)
			}
		}
		// Avoid duplicates, care for parse_again and missing counters,
		// don't change the pointer
		extra := box.ownExtra()
		for i, entry := range extra.StringSet {
			if entry.Name == stringName {
				extra.StringSet = slices.Delete(extra.StringSet, i, i+1)
				break
			}
		}
		extra.StringSet = append(extra.StringSet, StringSetEntry{stringName, text.String()})
	}
}

// computeBookmarkLabel is compute_bookmark_label in
// weasyprint/formatting_structure/build.py. It parses the content list of
// bookmark-label.
func (b *builder) computeBookmarkLabel(element *Element, box *Box, contentList Value, counterValues map[string][]int) {
	parseAgain := func(counters map[string][]int) {
		// Parse the bookmark-label all again.
		localCounters := cloneCounterValues(counters)
		for key, value := range box.extra().CachedCounterValues {
			localCounters[key] = value
		}
		b.computeBookmarkLabel(element, box, contentList, localCounters)
	}

	cssToken := "bookmark-label"
	boxList, _ := b.computeContentList(contentList, box, counterValues, cssToken, parseAgain, false, nil, nil, nil, nil, nil)
	if len(boxList) > 0 {
		var label strings.Builder
		for _, child := range boxList {
			label.WriteString(child.ContentText())
		}
		box.ownExtra().BookmarkLabel, box.ownExtra().HasBookmarkLabel = label.String(), true
	}
}

// setContentLists is set_content_lists in weasyprint/formatting_structure/
// build.py. It sets the values of the content lists of the properties of
// GCPM, such as string-set and bookmark-label.
func (b *builder) setContentLists(element *Element, box *Box, style *Style, counterValues map[string][]int) {
	// Port: upstream sets an empty list. The port sets nil, which the
	// layout reads as an empty list, so that a box without named strings
	// does not make its boxExtra.
	if box.ext != nil {
		box.ext.StringSet = nil
	}
	if stringSet := style.Get("string_set"); !isStr(stringSet, "none") {
		items, _ := stringSet.(Tuple)
		for _, item := range items {
			pair, ok := item.(Tuple)
			if !ok || len(pair) != 2 {
				continue
			}
			b.computeStringSet(element, box, str(pair[0]), pair[1], counterValues)
		}
	}
	if !isStr(style.Get("bookmark_level"), "none") {
		b.computeBookmarkLabel(element, box, style.Get("bookmark_label"), counterValues)
	}
}

// updateCounters is update_counters in weasyprint/formatting_structure/
// build.py. It handles the properties counter-*.
func updateCounters(counterValues map[string][]int, counterScopes []map[string]bool, style *Style) {
	siblingScopes := counterScopes[len(counterScopes)-1]

	pairs := func(v Value) [][2]any {
		t, _ := v.(Tuple)
		var result [][2]any
		for _, item := range t {
			if pair, ok := item.(Tuple); ok && len(pair) == 2 {
				result = append(result, [2]any{str(pair[0]), int(num(pair[1]))})
			}
		}
		return result
	}

	for _, pair := range pairs(style.Get("counter_reset")) {
		name, value := pair[0].(string), pair[1].(int)
		if siblingScopes[name] {
			if values := counterValues[name]; len(values) > 0 {
				counterValues[name] = values[:len(values)-1]
			}
		} else {
			siblingScopes[name] = true
		}
		counterValues[name] = append(counterValues[name], value)
	}

	for _, pair := range pairs(style.Get("counter_set")) {
		name, value := pair[0].(string), pair[1].(int)
		values := counterValues[name]
		if len(values) == 0 {
			// Port: upstream asserts that the name is not in the scopes.
			siblingScopes[name] = true
			values = append(values, 0)
		}
		values[len(values)-1] = value
		counterValues[name] = values
	}

	counterIncrement := style.Get("counter_increment")
	increments := pairs(counterIncrement)
	if isStr(counterIncrement, "auto") {
		// 'auto' is the initial value but is not valid in stylesheet:
		// there was no counter-increment declaration for this element.
		// (Or the winning value was 'initial'.)
		// https://drafts.csswg.org/css-lists-3/#declaring-a-list-item
		increments = nil
		if tupleHas(style.Get("display"), "list-item") {
			increments = [][2]any{{"list-item", 1}}
		}
	}
	for _, pair := range increments {
		name, value := pair[0].(string), pair[1].(int)
		values := counterValues[name]
		if len(values) == 0 {
			// Port: upstream asserts that the name is not in the scopes.
			siblingScopes[name] = true
			values = append(values, 0)
		}
		values[len(values)-1] += value
		counterValues[name] = values
	}
}

// wrapImproper is wrap_improper in weasyprint/formatting_structure/build.py.
// It wraps the consecutive children that do not pass test in a box of a
// kind. A nil test is a test that the child is of that kind.
func wrapImproper(b *builder, box *Box, children []*Box, wrapperKind Kind, test func(*Box) bool) []*Box {
	if test == nil {
		test = func(child *Box) bool { return child.Is(wrapperKind) }
	}
	var result, improper []*Box
	for _, child := range children {
		if test(child) {
			if len(improper) > 0 {
				wrapper := anonymousFrom(wrapperKind, box, nil)
				// Apply the rules again on the new wrapper
				result = append(result, tableBoxesChildren(b, wrapper, improper))
				improper = nil
			}
			result = append(result, child)
		} else {
			improper = append(improper, child)
		}
	}
	if len(improper) > 0 {
		wrapper := anonymousFrom(wrapperKind, box, nil)
		// Apply the rules again on the new wrapper
		result = append(result, tableBoxesChildren(b, wrapper, improper))
	}
	return result
}

// anonymousTableBoxes is anonymous_table_boxes in
// weasyprint/formatting_structure/build.py. It removes and adds boxes as
// the table model says. See
// https://www.w3.org/TR/CSS21/tables.html#anonymous-boxes.
func anonymousTableBoxes(b *builder, box *Box) *Box {
	if !box.IsParent() || box.IsRunning() {
		return box
	}

	// Do recursion.
	children := make([]*Box, len(box.Children))
	for i, child := range box.Children {
		children[i] = anonymousTableBoxes(b, child)
	}
	return tableBoxesChildren(b, box, children)
}

// tableBoxesChildren is table_boxes_children in
// weasyprint/formatting_structure/build.py, the implementation of
// anonymous_table_boxes.
func tableBoxesChildren(b *builder, box *Box, children []*Box) *Box {
	switch box.Kind {
	case KindTableColumn: // rule 1.1
		// Remove all children.
		children = nil
	case KindTableColumnGroup: // rule 1.2
		// Remove children other than table-column.
		var columns []*Box
		for _, child := range children {
			if child.Kind == KindTableColumn {
				columns = append(columns, child)
			}
		}
		children = columns
		// Rule XXX (not in the spec): column groups have at least one
		// column child.
		if len(children) == 0 {
			span := box.Span()
			if b.spend(span) {
				for range span {
					children = append(children, anonymousFrom(KindTableColumn, box, nil))
				}
			}
		}
	}

	// rule 1.3
	if box.tabularContainer() && len(children) >= 2 {
		// TODO: Maybe only remove text if internal is also a proper table
		// descendant of box.
		// This is what the spec says, but maybe not what browsers do:
		// https://lists.w3.org/Archives/Public/www-style/2011Oct/0567

		// Last child
		internal, text := children[len(children)-2], children[len(children)-1]
		if internal.internalTableOrCaption() && text.IsWhitespace() {
			children = children[:len(children)-1]
		}

		// First child
		if len(children) >= 2 {
			text, internal := children[0], children[1]
			if internal.internalTableOrCaption() && text.IsWhitespace() {
				children = children[1:]
			}
		}

		// Children other than first and last that would be removed by rule
		// 1.3 are also removed by rule 1.4 below.
	}

	filtered := make([]*Box, 0, len(children))
	for i, child := range children {
		// Ignore some whitespace: rule 1.4
		if i > 0 && children[i-1].internalTableOrCaption() &&
			i+1 < len(children) && children[i+1].internalTableOrCaption() &&
			child.IsWhitespace() {
			continue
		}
		filtered = append(filtered, child)
	}
	children = filtered

	switch {
	case box.IsTable():
		// Rule 2.1
		children = wrapImproper(b, box, children, KindTableRow, func(child *Box) bool { return child.properTableChild() })
	case box.Kind == KindTableRowGroup:
		// Rule 2.2
		children = wrapImproper(b, box, children, KindTableRow, nil)
	}

	if box.Kind == KindTableRow {
		// Rule 2.3
		children = wrapImproper(b, box, children, KindTableCell, nil)
	} else {
		// Rule 3.1
		children = wrapImproper(b, box, children, KindTableRow, func(child *Box) bool { return child.Kind != KindTableCell })
	}

	// Rule 3.2
	if box.Kind == KindInline {
		children = wrapImproper(b, box, children, KindInlineTable, func(child *Box) bool { return !child.properTableChild() })
	} else {
		parentKind := box.Kind
		children = wrapImproper(b, box, children, KindTable, func(child *Box) bool {
			return !child.properTableChild() || child.hasProperParent(parentKind)
		})
	}

	if box.IsTable() {
		return wrapTable(b, box, children)
	}
	box.Children = children
	return box
}

// wrapTable is wrap_table in weasyprint/formatting_structure/build.py. It
// takes a table box and returns it in its table wrapper box.
//
// It also orders the children again, and gives a position in the grid to
// each column and cell.
//
// Because of the way colspan and rowspan work, grid_y is implicitly the
// index of a row, but grid_x is an explicit attribute of the cells, the
// columns and the column groups.
//
// See https://www.w3.org/TR/CSS21/tables.html#model and
// https://www.w3.org/TR/CSS21/tables.html#table-layout.
func wrapTable(b *builder, box *Box, children []*Box) *Box {
	// Group table children by type
	var columns, rows, allCaptions []*Box
	for _, child := range children {
		switch child.Kind {
		case KindTableColumn, KindTableColumnGroup:
			columns = append(columns, child)
		case KindTableRow, KindTableRowGroup:
			rows = append(rows, child)
		case KindTableCaption:
			allCaptions = append(allCaptions, child)
		}
		// Port: upstream raises KeyError for another kind of child, which
		// the rules of the tables do not leave.
	}

	// Split top and bottom captions
	var topCaptions, bottomCaptions []*Box
	for _, caption := range allCaptions {
		if isStr(caption.Style.Get("caption_side"), "bottom") {
			bottomCaptions = append(bottomCaptions, caption)
		} else {
			topCaptions = append(topCaptions, caption)
		}
	}

	// Assign X positions on the grid to column boxes
	columnGroups := wrapImproper(b, box, columns, KindTableColumnGroup, nil)
	gridX := 0
	for _, group := range columnGroups {
		group.ownTable().GridX = gridX
		if len(group.Children) > 0 {
			for _, column := range group.Children {
				// There's no need to take care of group's span, as "span=x"
				// already generates x TableColumnBox children
				column.ownTable().GridX = gridX
				gridX++
			}
		} else {
			gridX += group.Span()
		}
	}
	gridWidth := gridX

	rowGroups := wrapImproper(b, box, rows, KindTableRowGroup, nil)
	// Extract the optional header and footer groups.
	var bodyRowGroups []*Box
	var header, footer *Box
	for _, group := range rowGroups {
		display := group.Style.Get("display")
		switch {
		case Equal(display, tupleOf("table-header-group")) && header == nil:
			group.ownTable().IsHeader = true
			header = group
		case Equal(display, tupleOf("table-footer-group")) && footer == nil:
			group.ownTable().IsFooter = true
			footer = group
		default:
			bodyRowGroups = append(bodyRowGroups, group)
		}
	}
	rowGroups = nil
	if header != nil {
		rowGroups = append(rowGroups, header)
	}
	rowGroups = append(rowGroups, bodyRowGroups...)
	if footer != nil {
		rowGroups = append(rowGroups, footer)
	}

	// Assign a (x,y) position in the grid to each cell.
	// rowspan can not extend beyond a row group, so each row group
	// is independent.
	// https://www.w3.org/TR/CSS21/tables.html#table-layout
	// Column 0 is on the left if direction is ltr, right if rtl.
	// This algorithm does not change.
	gridHeight := 0
	occupiedBudget := maxRowspanCells
	for _, group := range rowGroups {
		// Indexes: row number in the group.
		// Values: set of cells already occupied by row-spanning cells.
		occupiedCellsByRow := make([]map[int]bool, len(group.Children))
		for i := range occupiedCellsByRow {
			occupiedCellsByRow[i] = map[int]bool{}
		}
		for rowIndex, row := range group.Children {
			occupiedCellsInThisRow := occupiedCellsByRow[rowIndex]
			// The list is now about rows after this one.
			occupiedCellsByRow := occupiedCellsByRow[rowIndex+1:]
			gridX := 0
			for _, cell := range row.Children {
				// Make sure that the first grid cell is free.
				for occupiedCellsInThisRow[gridX] {
					gridX++
				}
				cell.ownTable().GridX = gridX
				newGridX := gridX + cell.table().Colspan
				// https://www.w3.org/TR/html401/struct/tables.html#adef-rowspan
				if cell.table().Rowspan != 1 {
					maxRowspan := len(occupiedCellsByRow) + 1
					var spannedRows []map[int]bool
					if cell.table().Rowspan == 0 {
						// All rows until the end of the group
						spannedRows = occupiedCellsByRow
						cell.ownTable().Rowspan = maxRowspan
					} else {
						cell.ownTable().Rowspan = min(cell.table().Rowspan, maxRowspan)
						spannedRows = occupiedCellsByRow[:cell.table().Rowspan-1]
					}
					for _, occupiedCells := range spannedRows {
						for x := gridX; x < newGridX && occupiedBudget > 0; x++ {
							occupiedCells[x] = true
							occupiedBudget--
						}
					}
				}
				gridX = newGridX
				gridWidth = max(gridWidth, gridX)
			}
		}
		gridHeight += len(group.Children)
	}

	table := box.CopyWithChildren(rowGroups)
	table.Style = table.Style.Copy()
	table.ownTable().ColumnGroups = columnGroups
	if isStr(table.Style.Get("border_collapse"), "collapse") {
		table.ownTable().CollapsedBorderGrid = collapseTableBorders(table, gridWidth, gridHeight)
	}

	wrapperKind := KindBlock
	if box.Kind == KindInlineTable {
		wrapperKind = KindInlineBlock
	}

	wrapperChildren := append(append(slices.Clone(topCaptions), table), bottomCaptions...)
	wrapper := anonymousFrom(wrapperKind, box, wrapperChildren)
	wrapper.Style = wrapper.Style.Copy()
	wrapper.IsTableWrapper = true
	// Non-inherited properties of the table element apply to one of the
	// wrapper and the table. The other get the initial value.
	// TODO: put this in a method of the table object
	for name := range TableWrapperBoxProperties {
		wrapper.Style.Set(name, table.Style.Get(name))
		table.Style.Set(name, InitialValues[name])
	}

	return wrapper
}

// blockify is blockify in weasyprint/formatting_structure/build.py. It turns
// an inline box into a block box. See
// https://drafts.csswg.org/css-display-4/#blockify.
func blockify(box *Box) *Box {
	var anonymous *Box
	switch {
	case box.Kind == KindInlineBlock:
		anonymous = anonymousFrom(KindBlock, box, box.Children)
	case box.Kind == KindInlineReplaced:
		anonymous = anonymousReplacedFrom(KindBlockReplaced, box, box.extra().Replacement)
	case box.Kind == KindText:
		anonymous = anonymousFrom(KindBlock, box, []*Box{box})
	case box.Kind == KindInlineFlex:
		anonymous = anonymousFrom(KindFlex, box, box.Children)
	case box.Kind == KindInlineGrid:
		anonymous = anonymousFrom(KindGrid, box, box.Children)
	case box.IsInlineLevel():
		anonymous = anonymousFrom(KindBlock, box, box.Children)
	default:
		return box
	}
	anonymous.Style = box.Style
	anonymous.IsTableWrapper = box.IsTableWrapper
	anonymous.IsFlexItem = box.IsFlexItem
	anonymous.IsGridItem = box.IsGridItem
	if link := box.extra().Link; link != nil {
		anonymous.ownExtra().Link = link
	}
	return anonymous
}

// flexBoxes is flex_boxes in weasyprint/formatting_structure/build.py. It
// removes and adds boxes as the flex model says. See
// https://www.w3.org/TR/css-flexbox-1/#flex-items.
func flexBoxes(box *Box) *Box {
	if !box.IsParent() || box.IsRunning() {
		return box
	}

	// Do recursion.
	children := make([]*Box, len(box.Children))
	for i, child := range box.Children {
		children[i] = flexBoxes(child)
	}
	box.Children = flexChildren(box, children)
	return box
}

// flexChildren is flex_children in weasyprint/formatting_structure/build.py.
func flexChildren(box *Box, children []*Box) []*Box {
	if !box.IsFlexContainer() {
		return children
	}
	var flexChildren []*Box
	for _, child := range children {
		child.notFloated = true
		if child.IsInNormalFlow() {
			child.IsFlexItem = true
		}
		if child.Kind == KindText && strings.Trim(child.Text, " ") == "" {
			// TODO: ignore texts only containing "characters that can be
			// affected by the white-space property"
			// https://www.w3.org/TR/css-flexbox-1/#flex-items
			continue
		}
		flexChildren = append(flexChildren, blockify(child))
	}
	return flexChildren
}

// gridBoxes is grid_boxes in weasyprint/formatting_structure/build.py. It
// removes and adds boxes as the grid model says. See
// https://drafts.csswg.org/css-grid-2/#grid-item.
func gridBoxes(box *Box) *Box {
	if !box.IsParent() || box.IsRunning() {
		return box
	}

	// Do recursion.
	children := make([]*Box, len(box.Children))
	for i, child := range box.Children {
		children[i] = gridBoxes(child)
	}
	box.Children = gridChildren(box, children)
	return box
}

// gridChildren is grid_children in weasyprint/formatting_structure/build.py.
func gridChildren(box *Box, children []*Box) []*Box {
	if !box.IsGridContainer() {
		return children
	}
	var gridChildren []*Box
	for _, child := range children {
		child.notFloated = true
		if child.IsInNormalFlow() {
			child.IsGridItem = true
		}
		if child.Kind == KindText && strings.Trim(child.Text, " ") == "" {
			// TODO: ignore texts only containing "characters that can be
			// affected by the white-space property"
			// https://drafts.csswg.org/css-grid-2/#grid-item
			continue
		}
		gridChildren = append(gridChildren, blockify(child))
	}
	return gridChildren
}

// inlineInBlock is inline_in_block in weasyprint/formatting_structure/
// build.py. It builds the structure of the lines inside the blocks and
// returns a new box tree.
//
// The consecutive inline-level boxes of a block container box are wrapped
// in a line box, itself wrapped in an anonymous block box. The layout later
// breaks this line box into one line box for each line.
//
// This is the first case in
// https://www.w3.org/TR/CSS21/visuren.html#anonymous-block-level.
//
// For example:
//
//	BlockBox[
//	    TextBox['Some '],
//	    InlineBox[TextBox['text']],
//	    BlockBox[
//	        TextBox['More text'],
//	    ]
//	]
//
// is turned into:
//
//	BlockBox[
//	    AnonymousBlockBox[
//	        LineBox[
//	            TextBox['Some '],
//	            InlineBox[TextBox['text']],
//	        ]
//	    ]
//	    BlockBox[
//	        LineBox[
//	            TextBox['More text'],
//	        ]
//	    ]
//	]
func inlineInBlock(box *Box) *Box {
	if len(box.Children) == 0 || box.IsRunning() || box.IsNote() {
		return box
	}

	boxChildren := box.Children

	if len(boxChildren) > 0 && !box.LeadingCollapsibleSpace {
		box.LeadingCollapsibleSpace = boxChildren[0].LeadingCollapsibleSpace
	}

	var children []*Box
	trailingCollapsibleSpace := false
	for _, child := range boxChildren {
		// Keep track of removed collapsing spaces for wrap opportunities,
		// and remove empty text boxes.
		// (They may have been emptied by process_whitespace().)

		if trailingCollapsibleSpace {
			child.LeadingCollapsibleSpace = true
		}

		if child.Kind == KindText && child.Text == "" {
			trailingCollapsibleSpace = child.LeadingCollapsibleSpace
		} else {
			trailingCollapsibleSpace = false
			children = append(children, inlineInBlock(child))
		}
	}

	if !box.TrailingCollapsibleSpace {
		box.TrailingCollapsibleSpace = trailingCollapsibleSpace
	}

	if !box.IsBlockContainer() {
		box.Children = children
		return box
	}

	var newLineChildren, newChildren []*Box

	for _, childBox := range children {
		// Port: upstream asserts that the child is not a line box.
		switch {
		case len(newLineChildren) > 0 && childBox.IsAbsolutelyPositioned():
			newLineChildren = append(newLineChildren, childBox)
		case childBox.IsInlineLevel() || (len(newLineChildren) > 0 && !childBox.IsInNormalFlow()):
			// Do not append white space at the start of a line:
			// It would be removed during layout.
			// A sequence of white-space was collapsed to a single space by
			// process_whitespace().
			collapsedSpace := childBox.Kind == KindText && childBox.Text == " " &&
				isStrIn(childBox.Style.Get("white_space"), "normal", "nowrap", "pre-line")
			if len(newLineChildren) > 0 || !collapsedSpace {
				newLineChildren = append(newLineChildren, childBox)
			}
		default:
			if len(newLineChildren) > 0 {
				// Inlines are consecutive no more: add this line box and
				// create a new one.
				lineBox := anonymousFrom(KindLine, box, newLineChildren)
				anonymous := anonymousFrom(KindBlock, box, []*Box{lineBox})
				newChildren = append(newChildren, anonymous)
				newLineChildren = nil
			}
			newChildren = append(newChildren, childBox)
		}
	}
	if len(newLineChildren) > 0 {
		// There were inlines at the end
		lineBox := anonymousFrom(KindLine, box, newLineChildren)
		if len(newChildren) > 0 {
			anonymous := anonymousFrom(KindBlock, box, []*Box{lineBox})
			newChildren = append(newChildren, anonymous)
		} else {
			// Only inline-level children: one line box
			newChildren = append(newChildren, lineBox)
		}
	}

	box.Children = newChildren
	return box
}

// blockInInline is block_in_inline in weasyprint/formatting_structure/
// build.py. It builds the structure of the blocks inside the lines.
//
// The inline boxes that hold block-level boxes are broken in two boxes on
// each side of the consecutive block-level boxes, each side wrapped in an
// anonymous block-level box.
//
// This is the second case in
// https://www.w3.org/TR/CSS21/visuren.html#anonymous-block-level.
//
// For example, if this is given:
//
//	BlockBox[
//	    LineBox[
//	        InlineBox[
//	            TextBox['Hello.'],
//	        ],
//	        InlineBox[
//	            TextBox['Some '],
//	            InlineBox[
//	                TextBox['text']
//	                BlockBox[LineBox[TextBox['More text']]],
//	                BlockBox[LineBox[TextBox['More text again']]],
//	            ],
//	            BlockBox[LineBox[TextBox['And again.']]],
//	        ]
//	    ]
//	]
//
// this is returned:
//
//	BlockBox[
//	    AnonymousBlockBox[
//	        LineBox[
//	            InlineBox[
//	                TextBox['Hello.'],
//	            ],
//	            InlineBox[
//	                TextBox['Some '],
//	                InlineBox[TextBox['text']],
//	            ]
//	        ]
//	    ],
//	    BlockBox[LineBox[TextBox['More text']]],
//	    BlockBox[LineBox[TextBox['More text again']]],
//	    AnonymousBlockBox[
//	        LineBox[
//	            InlineBox[
//	            ]
//	        ]
//	    ],
//	    BlockBox[LineBox[TextBox['And again.']]],
//	    AnonymousBlockBox[
//	        LineBox[
//	            InlineBox[
//	            ]
//	        ]
//	    ],
//	]
func blockInInline(box *Box) *Box {
	if len(box.Children) == 0 || box.IsRunning() || box.IsNote() {
		return box
	}

	var newChildren []*Box
	changed := false

	for _, child := range box.Children {
		var newChild *Box
		if child.Kind == KindLine {
			// Port: upstream asserts that line boxes have no siblings at
			// this stage.
			var stack *buildSkipStack
			var newLine *Box
			for {
				var block *Box
				newLine, block, stack = innerBlockInInline(child, stack)
				if block == nil {
					break
				}
				anon := anonymousFrom(KindBlock, box, []*Box{newLine})
				newChildren = append(newChildren, anon)
				newChildren = append(newChildren, blockInInline(block))
				// Loop with the same child and the new stack.
			}
			if len(newChildren) > 0 {
				// Some children were already added, this became a block
				// context.
				newChild = anonymousFrom(KindBlock, box, []*Box{newLine})
			} else {
				// Keep the single line box as-is, without anonymous
				// blocks.
				newChild = newLine
			}
		} else {
			// Not in an inline formatting context.
			newChild = blockInInline(child)
		}

		if newChild != child {
			changed = true
		}
		newChildren = append(newChildren, newChild)
	}

	if changed {
		box.Children = newChildren
	}
	return box
}

// buildSkipStack is a skip stack of upstream: the dict {index: skip_stack} of
// one item, where a nil skip stack is None.
type buildSkipStack struct {
	index int
	next  *buildSkipStack
}

// innerBlockInInline is _inner_block_in_inline in
// weasyprint/formatting_structure/build.py. It finds a block-level box in
// an inline formatting context.
//
// If it finds one, it returns (newBox, blockLevelBox, resumeAt). newBox
// holds all of the content of box before the block-level box. resumeAt can
// be passed as stack in a new call to this function to resume the
// search just after the block-level box.
//
// If it finds no block-level box after the position of stack, it
// returns (newBox, nil, nil).
func innerBlockInInline(box *Box, stack *buildSkipStack) (*Box, *Box, *buildSkipStack) {
	var newChildren []*Box
	var blockLevelBox *Box
	var resumeAt *buildSkipStack
	changed := false

	skip := 0
	if stack != nil {
		skip, stack = stack.index, stack.next
	}

	found := false
	for i, child := range box.Children[min(skip, len(box.Children)):] {
		index := i + skip
		if child.IsBlockLevel() && child.IsInNormalFlow() {
			// Port: upstream asserts that the stack is empty here.
			blockLevelBox = child
			index++ // Resume *after* the block
		} else {
			var newChild *Box
			if child.Kind == KindInline {
				newChild, blockLevelBox, resumeAt = innerBlockInInline(child, stack)
				stack = nil
			} else {
				// Port: upstream asserts that the stack is empty here.
				newChild = blockInInline(child)
				// block_level_box is still None.
			}
			if newChild != child {
				changed = true
			}
			newChildren = append(newChildren, newChild)
		}
		if blockLevelBox != nil {
			resumeAt = &buildSkipStack{index, resumeAt}
			box = box.CopyWithChildren(newChildren)
			found = true
			break
		}
	}
	if !found && (changed || skip != 0) {
		box = box.CopyWithChildren(newChildren)
	}

	return box, blockLevelBox, resumeAt
}

// setViewportOverflow is set_viewport_overflow in
// weasyprint/formatting_structure/build.py. It sets the attribute
// ViewportOverflow of the box of the root element.
//
// Like the backgrounds, the overflow of the root element is propagated to
// the viewport. See https://www.w3.org/TR/CSS21/visufx.html#overflow.
func setViewportOverflow(rootBox *Box) *Box {
	chosenBox := rootBox
	if strings.ToLower(rootBox.ElementTag) == "html" && isStr(rootBox.Style.Get("overflow"), "visible") {
		for _, child := range rootBox.Children {
			if strings.ToLower(child.ElementTag) == "body" {
				chosenBox = child
				break
			}
		}
	}

	rootBox.ownExtra().ViewportOverflow = str(chosenBox.Style.Get("overflow"))
	chosenBox.Style.Set("overflow", String("visible"))
	return rootBox
}
