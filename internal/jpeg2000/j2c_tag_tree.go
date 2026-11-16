// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/tag_tree.rs.

import "math/bits"

// noChild is usize::MAX: the index of a child that does not exist.
const noChild = -1

// tagNode is TagNode in hayro-jpeg2000 src/j2c/tag_tree.rs.
type tagNode struct {
	width       uint32
	height      uint32
	value       uint32
	initialized bool
	level       uint16
	children    [4]int
}

// newTagNode is TagNode::new.
func newTagNode(width, height uint32, level uint16) tagNode {
	return tagNode{
		width:    width,
		height:   height,
		level:    level,
		children: [4]int{noChild, noChild, noChild, noChild},
	}
}

// topLeftWidth is TagNode::top_left_width.
func (n *tagNode) topLeftWidth() uint32 {
	return min(shl32(1, uint32(n.level)-1), n.width)
}

// topLeftHeight is TagNode::top_left_height.
func (n *tagNode) topLeftHeight() uint32 {
	return min(shl32(1, uint32(n.level)-1), n.height)
}

// buildTagNode is TagNode::build. It builds the node and pushes its
// children to nodes.
//
// Port: upstream builds the subtree of a node without area, and then
// discards it. That subtree pushes no node, but it can take 4^level calls,
// so a file can make it hang. The port does not build it. The nodes are the
// same.
func buildTagNode(width, height uint32, level uint16, nodes *[]tagNode) tagNode {
	tag := newTagNode(width, height, level)
	if level == 0 || width == 0 || height == 0 {
		return tag
	}

	tlw := tag.topLeftWidth()
	tlh := tag.topLeftHeight()

	push := func(node tagNode, childIdx int) {
		if node.width > 0 && node.height > 0 {
			tag.children[childIdx] = len(*nodes)
			*nodes = append(*nodes, node)
		}
	}

	push(buildTagNode(tlw, tlh, level-1, nodes), 0)
	push(buildTagNode(width-tlw, tlh, level-1, nodes), 1)
	push(buildTagNode(tlw, height-tlh, level-1, nodes), 2)
	push(buildTagNode(width-tlw, height-tlh, level-1, nodes), 3)
	return tag
}

// readTagNode is read_tag_node in hayro-jpeg2000 src/j2c/tag_tree.rs. It
// returns false when the data ends.
//
// Port: upstream indexes a child that does not exist only for a position
// outside the tree, and panics. The port returns false.
func readTagNode(nodeIdx int, x, y uint32, reader *bitReader, parentVal, maxVal uint32, nodes []tagNode) (uint32, bool) {
	for {
		if nodeIdx < 0 || nodeIdx >= len(nodes) {
			return 0, false
		}
		node := &nodes[nodeIdx]

		if !node.initialized {
			val := max(parentVal, node.value)
			for val < maxVal {
				bit, ok := reader.readBitsWithStuffing(1)
				if !ok {
					return 0, false
				}
				if bit == 1 {
					node.initialized = true
					break
				}
				val++
			}
			node.value = val
		}

		if node.value >= maxVal || node.level == 0 {
			return node.value, true
		}

		tlw := node.topLeftWidth()
		tlh := node.topLeftHeight()
		left := x < tlw
		top := y < tlh

		parentVal = node.value
		switch {
		case left && top:
			nodeIdx = node.children[0]
		case !left && top:
			nodeIdx = node.children[1]
			x -= tlw
		case left && !top:
			nodeIdx = node.children[2]
			y -= tlh
		default:
			nodeIdx = node.children[3]
			x -= tlw
			y -= tlh
		}
	}
}

// tagTree is TagTree in hayro-jpeg2000 src/j2c/tag_tree.rs.
type tagTree struct {
	root   int
	width  uint32
	height uint32
}

// newTagTree is TagTree::new.
func newTagTree(width, height uint32, nodes *[]tagNode) tagTree {
	level := max(ilog2NextPowerOfTwo(width), ilog2NextPowerOfTwo(height))
	node := buildTagNode(width, height, uint16(level), nodes)
	idx := len(*nodes)
	*nodes = append(*nodes, node)
	return tagTree{root: idx, width: width, height: height}
}

// ilog2NextPowerOfTwo is v.next_power_of_two().ilog2() of Rust.
//
// Port: when the power overflows, Rust wraps it to zero in a build for
// release, and ilog2 panics. The limits of the code-blocks keep v small, and
// the port returns 32 for such a value.
func ilog2NextPowerOfTwo(v uint32) uint32 {
	if v <= 1 {
		return 0
	}
	return uint32(bits.Len32(v - 1))
}

// read is TagTree::read.
func (t *tagTree) read(x, y uint32, reader *bitReader, maxVal uint32, nodes []tagNode) (uint32, bool) {
	return readTagNode(t.root, x, y, reader, 0, maxVal, nodes)
}
