// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports part of weasyprint/anchors.py: the transformation matrices
// that gather_anchors sets on the boxes, which the drawing reads. ivy draws
// no links, no bookmarks, no anchors and no forms, so the rest of the module
// is not ported.

package layout

import "math"

// gatherAnchors is gather_anchors in weasyprint/anchors.py, for the
// transformation matrices only. It sets TransformationMatrix on each box
// of the tree that has a transform, and nil on the other boxes.
func gatherAnchors(box *Box, depth int) {
	// Get the transformation matrix of the box.
	// "Transforms apply to block-level and atomic inline-level elements,
	//  but do not apply to elements which may be split into
	//  multiple inline-level boxes."
	// https://www.w3.org/TR/css-transforms-1/#introduction
	if box.ext != nil {
		box.ext.TransformationMatrix = nil
	}
	if transforms, ok := box.Style.Get("transform").(Tuple); ok && len(transforms) > 0 && box.Kind != KindInline {
		borderWidth := box.BorderWidth()
		borderHeight := box.BorderHeight()
		var offsetX, offsetY float64
		if origin, ok := box.Style.Get("transform_origin").(Tuple); ok && len(origin) == 2 {
			offsetX, _ = percentageFloat(origin[0], box.Style, borderWidth)
			offsetY, _ = percentageFloat(origin[1], box.Style, borderHeight)
		}
		originX := box.BorderBoxX() + offsetX
		originY := box.BorderBoxY() + offsetY

		matrix := translation(originX, originY)
		for _, item := range transforms {
			pair, ok := item.(Tuple)
			if !ok || len(pair) != 2 {
				continue
			}
			name, args := str(pair[0]), pair[1]
			m := identity
			t, _ := args.(Tuple)
			switch {
			case name == "scale" && len(t) == 2:
				m.A, m.D = num(t[0]), num(t[1])
			case name == "rotate":
				angle := num(args)
				m.A, m.D = math.Cos(angle), math.Cos(angle)
				m.B = math.Sin(angle)
				m.C = -m.B
			case name == "translate" && len(t) == 2:
				m.E, _ = percentageFloat(t[0], box.Style, borderWidth)
				m.F, _ = percentageFloat(t[1], box.Style, borderHeight)
			case name == "skew" && len(t) == 2:
				m.B, m.C = math.Tan(num(t[1])), math.Tan(num(t[0]))
			case name == "matrix" && len(t) == 6:
				m = Matrix{A: num(t[0]), B: num(t[1]), C: num(t[2]), D: num(t[3]), E: num(t[4]), F: num(t[5])}
			}
			// Port: upstream asserts that the name is matrix for the
			// other names.
			matrix = m.Mul(matrix)
		}
		box.ownExtra().TransformationMatrix = translation(-originX, -originY).Mul(matrix)
	}

	// Port: the depth of the tree is bounded, as the depth of the layout.
	if depth > maxLayoutDepth {
		return
	}
	for _, child := range box.AllChildren() {
		gatherAnchors(child, depth+1)
	}
}
