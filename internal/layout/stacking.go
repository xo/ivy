// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/stacking.py: the management of the stacking
// contexts.

package layout

import "slices"

// StackingContext is StackingContext in weasyprint/stacking.py. The stacking
// contexts define the paint order of all the pieces of a document. See
// https://www.w3.org/TR/CSS21/visuren.html#x43 and
// https://www.w3.org/TR/CSS21/zindex.html.
type StackingContext struct {
	Box  *Box
	Page *Box
	// BlockLevelBoxes are the boxes of point 4: in flow, not positioned.
	BlockLevelBoxes []*Box
	// FloatContexts are the contexts of point 5: not positioned.
	FloatContexts []*StackingContext
	// NegativeZContexts are the child contexts of point 3, with a z-index
	// below 0.
	NegativeZContexts []*StackingContext
	// ZeroZContexts are the child contexts of point 8, with a z-index of 0.
	ZeroZContexts []*StackingContext
	// PositiveZContexts are the child contexts of point 9, with a z-index
	// above 0.
	PositiveZContexts []*StackingContext
	// BlocksAndCells are the boxes of point 7: not positioned.
	BlocksAndCells []BlockAndCells
	// ZIndex is the z-index of the box, with 0 for auto.
	ZIndex int

	// inline holds the stacking contexts that stand for a box in the tree
	// of the boxes, by the box that stands in the children of its parent.
	// The contexts of a page share it.
	//
	// Port: upstream puts the context itself in the children of the box.
	inline map[*Box]*StackingContext
}

// BlockAndCells is an item of the dict blocks_and_cells of upstream: a box,
// and the blocks and the cells inside it, in the order of the tree.
type BlockAndCells struct {
	Box      *Box
	Children []BlockAndCells
}

// newStackingContext is StackingContext.__init__ in weasyprint/stacking.py.
func newStackingContext(box *Box, childContexts []*StackingContext, blocks []*Box, floats []*StackingContext,
	blocksAndCells []BlockAndCells, page *Box, inline map[*Box]*StackingContext,
) *StackingContext {
	sc := &StackingContext{
		Box:             box,
		Page:            page,
		BlockLevelBoxes: blocks,
		FloatContexts:   floats,
		BlocksAndCells:  blocksAndCells,
		inline:          inline,
	}
	for _, context := range childContexts {
		switch {
		case context.ZIndex < 0:
			sc.NegativeZContexts = append(sc.NegativeZContexts, context)
		case context.ZIndex == 0:
			sc.ZeroZContexts = append(sc.ZeroZContexts, context)
		default:
			sc.PositiveZContexts = append(sc.PositiveZContexts, context)
		}
	}
	byZ := func(a, b *StackingContext) int { return a.ZIndex - b.ZIndex }
	// The sort is stable, so the lists are now sorted by z-index, then in
	// the order of the tree.
	slices.SortStableFunc(sc.NegativeZContexts, byZ)
	slices.SortStableFunc(sc.PositiveZContexts, byZ)
	if z, ok := box.Style.Get("z_index").(Int); ok {
		sc.ZIndex = int(z)
	}
	return sc
}

// StackingContextFromPage is StackingContext.from_page. The children of the
// page, the box of the root element and the margin boxes, and the page box
// itself are always stacking contexts.
func StackingContextFromPage(page *Box) *StackingContext {
	inline := map[*Box]*StackingContext{}
	childContexts := make([]*StackingContext, 0, len(page.Children))
	for _, child := range page.Children {
		childContexts = append(childContexts, stackingContextFromBox(child, page, nil, inline))
	}
	// The children are sub-contexts: remove them from the "normal" tree.
	page = copyWithChildren(page, nil)
	return newStackingContext(page, childContexts, nil, nil, nil, page, inline)
}

// stackingContextFromBox is StackingContext.from_box. childContexts is
// where the sub-contexts that it finds go, or nil for the children of the
// new context: "treat the element as if it created a new stacking context,
// but any positioned descendants and descendants which actually create a
// new stacking context should be considered part of the parent stacking
// context, not this new one."
func stackingContextFromBox(box, page *Box, childContexts *[]*StackingContext, inline map[*Box]*StackingContext) *StackingContext {
	var children []*StackingContext // What will be passed to this box
	if childContexts == nil {
		childContexts = &children
	}
	var blocks []*Box
	var floats []*StackingContext
	var blocksAndCells []BlockAndCells
	box = dispatchChildren(box, page, childContexts, &blocks, &floats, &blocksAndCells, inline)
	return newStackingContext(box, children, blocks, floats, blocksAndCells, page, inline)
}

// copyWithChildren returns a copy of box with other children.
//
// Port: copy_with_children of upstream also clears the removed decorations
// of the original box. The drawing of the port does not change the boxes of
// the layout, which a later pass of the layout of a document can read.
func copyWithChildren(box *Box, children []*Box) *Box {
	c := box.Copy()
	c.Children = children
	return c
}

// dispatch is _dispatch in weasyprint/stacking.py. It returns the box that
// stays in the children of its parent, or nil.
func dispatch(box, page *Box, childContexts *[]*StackingContext, blocks *[]*Box, floats *[]*StackingContext,
	blocksAndCells *[]BlockAndCells, inline map[*Box]*StackingContext,
) *Box {
	if box.IsPlaceholder() {
		box = box.placeholderBox()
	}
	style := box.Style

	// Remove the boxes that define a new stacking context from the
	// children.
	zAuto := isStr(style.Get("z_index"), "auto")
	definesStackingContext := (!isStr(style.Get("position"), "static") && !zAuto) ||
		(box.IsGridItem && !zAuto) ||
		num(style.Get("opacity")) < 1 ||
		hasTransform(style) || // 'transform: none' gives a "falsy" empty list
		!isStr(style.Get("overflow"), "visible")
	if definesStackingContext {
		*childContexts = append(*childContexts, stackingContextFromBox(box, page, nil, inline))
		return nil
	}

	switch {
	case !isStr(style.Get("position"), "static"):
		// "Fake" context: the sub-contexts go in this childContexts list.
		// Insert at the position before the sub-context is created.
		index := len(*childContexts)
		stackingContext := stackingContextFromBox(box, page, childContexts, inline)
		*childContexts = slices.Insert(*childContexts, index, stackingContext)
	case box.IsFloated():
		*floats = append(*floats, stackingContextFromBox(box, page, childContexts, inline))
	case box.Is(KindInlineBlock) || box.Is(KindInlineFlex) || box.Is(KindInlineGrid):
		// Have this fake stacking context be part of the "normal" box
		// tree, because its position is needed in the middle of a tree of
		// inline boxes.
		stackingContext := stackingContextFromBox(box, page, childContexts, inline)
		inline[stackingContext.Box] = stackingContext
		return stackingContext.Box
	default:
		switch {
		case box.IsBlockLevel():
			blocksIndex := len(*blocks)
			var boxBlocksAndCells []BlockAndCells
			box = dispatchChildren(box, page, childContexts, blocks, floats, &boxBlocksAndCells, inline)
			*blocks = slices.Insert(*blocks, blocksIndex, box)
			*blocksAndCells = append(*blocksAndCells, BlockAndCells{Box: box, Children: boxBlocksAndCells})
		case box.Is(KindTableCell):
			var boxBlocksAndCells []BlockAndCells
			box = dispatchChildren(box, page, childContexts, blocks, floats, &boxBlocksAndCells, inline)
			*blocksAndCells = append(*blocksAndCells, BlockAndCells{Box: box, Children: boxBlocksAndCells})
		default:
			box = dispatchChildren(box, page, childContexts, blocks, floats, blocksAndCells, inline)
		}
		return box
	}
	return nil
}

// dispatchChildren is _dispatch_children in weasyprint/stacking.py.
func dispatchChildren(box, page *Box, childContexts *[]*StackingContext, blocks *[]*Box, floats *[]*StackingContext,
	blocksAndCells *[]BlockAndCells, inline map[*Box]*StackingContext,
) *Box {
	if !box.IsParent() {
		return box
	}
	newChildren := make([]*Box, 0, len(box.Children))
	for _, child := range box.Children {
		if result := dispatch(child, page, childContexts, blocks, floats, blocksAndCells, inline); result != nil {
			newChildren = append(newChildren, result)
		}
	}
	return copyWithChildren(box, newChildren)
}

// hasTransform reports whether the style has a transform: the value of
// transform is not the empty list of none.
func hasTransform(style *Style) bool {
	t, ok := style.Get("transform").(Tuple)
	return ok && len(t) > 0
}
