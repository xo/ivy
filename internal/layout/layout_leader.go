// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/leader.py: the management of the
// leaders.

package layout

import "math"

// maxLeaders bounds the copies of the text of a leader in a line.
//
// Port: upstream has no bound. A leader whose text is very narrow in a wide
// line makes millions of boxes.
const maxLeaders = 1 << 16

// leaderIndex is leader_index in weasyprint/layout/leader.py: it returns the
// index of the first leader box in box, and the leader box. The index is
// the path of the indexes of the children from box to the leader. It
// returns nil and nil when box holds no leader.
//
// Port: upstream returns the nested tuples (i, (j, None)). The port returns
// the list [i, j].
func leaderIndex(box *Box) ([]int, *Box) {
	for i, child := range box.Children {
		if child.IsLeader {
			return []int{i}, child
		}
		if child.IsParent() {
			childLeaderIndex, childLeader := leaderIndex(child)
			if childLeaderIndex != nil {
				return append([]int{i}, childLeaderIndex...), childLeader
			}
		}
	}
	return nil, nil
}

// handleLeader is handle_leader in weasyprint/layout/leader.py: it finds a
// leader box in line and handles its text and its position.
func handleLeader(lc *layoutContext, line, containingBlock *Box) {
	index, leaderBox := leaderIndex(line)
	extraWidth := 0.0
	if index != nil && len(leaderBox.Children) > 0 {
		// Port: upstream fails to unpack a leader box with more than one
		// child. The port uses the first one.
		textBox := leaderBox.Children[0]

		// Abort if the leader text has no width
		if textBox.Width <= 0 {
			return
		}

		// Extra width is the additional width taken by the leader box
		extraWidth = containingBlock.Width
		for _, child := range line.InFlowChildren() {
			extraWidth -= child.MarginWidth()
		}

		// Take care of excluded shapes
		for _, shape := range lc.excludedShapes() {
			if shape.PositionY+shape.Height > line.PositionY {
				extraWidth -= shape.Width
			}
		}

		// Available width is the width available for the leader box
		availableWidth := extraWidth + textBox.Width
		line.Width = containingBlock.Width

		// Add text boxes into the leader box
		//
		// Port: upstream fails on a width that is not finite. The port
		// adds no text.
		numberOfLeaders := 0
		if leaders := math.Floor(line.Width / textBox.Width); leaders > 0 && !math.IsInf(leaders, 0) && !math.IsNaN(leaders) {
			numberOfLeaders = int(min(leaders, maxLeaders))
		}
		positionX := line.PositionX + line.Width
		var children []*Box
		for range numberOfLeaders {
			positionX -= textBox.Width
			if positionX < leaderBox.PositionX {
				// Don’t add leaders behind the text on the left
				continue
			} else if positionX+textBox.Width > leaderBox.PositionX+availableWidth {
				// Don’t add leaders behind the text on the right
				continue
			}
			textBox = textBox.Copy()
			textBox.PositionX = positionX
			children = append(children, textBox)
		}
		leaderBox.Children = children

		if isStr(line.Style.Get("direction"), "rtl") {
			leaderBox.Translate(-extraWidth, 0, false)
		}
	}

	// Widen leader parent boxes and translate following boxes
	box := line
	for _, i := range index {
		if i >= len(box.Children) {
			// Port: the children of the line changed under the index.
			break
		}
		for _, child := range box.Children[i+1:] {
			if child.IsInNormalFlow() {
				if isStr(line.Style.Get("direction"), "ltr") {
					child.Translate(extraWidth, 0, false)
				} else {
					child.Translate(-extraWidth, 0, false)
				}
			}
		}
		box = box.Children[i]
		box.Width += extraWidth
	}
}
