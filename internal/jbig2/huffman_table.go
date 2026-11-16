// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/huffman_table.rs.
//
// Port: upstream shares a table with an Arc, and it builds each standard
// table at its first use with Lazy. Go shares a *huffmanTable, and the
// standard tables are package variables over the constant trees of
// huffman_tables_generated.go.

import "math"

// inlineTableSize is INLINE_TABLE_SIZE, the number of nodes of each standard
// table.
const inlineTableSize = 43

// huffmanTable is HuffmanTable in hayro-jbig2 src/huffman_table.rs. It is a
// tree of nodes. The node 0 is the root.
type huffmanTable struct {
	nodes []huffmanNode
}

// huffmanTableFromNodes is HuffmanTable::from_inline and
// HuffmanTable::from_dynamic.
func huffmanTableFromNodes(nodes []huffmanNode) *huffmanTable {
	return &huffmanTable{nodes: nodes}
}

// decode is HuffmanTable::decode. It returns false for the out-of-band
// value.
func (t *huffmanTable) decode(r *reader) (int32, bool, error) {
	return huffmanDecodeFrom(t.nodes, 0, r)
}

// decodeNoOOB is HuffmanTable::decode_no_oob. It returns
// ErrHuffmanUnexpectedOOB for the out-of-band value.
func (t *huffmanTable) decodeNoOOB(r *reader) (int32, error) {
	v, ok, err := t.decode(r)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrHuffmanUnexpectedOOB
	}
	return v, nil
}

// buildHuffmanTable is HuffmanTable::build. It assigns the codes of T.88
// B.3 to the lines and builds the tree.
func buildHuffmanTable(lines []tableLine) (*huffmanTable, error) {
	var maxPrefixLength uint8
	for _, l := range lines {
		maxPrefixLength = max(maxPrefixLength, l.prefixLength)
	}

	nodes := []huffmanNode{newIntermediateNode()}

	// The prefix lengths go up to 255, so a code is a vector of bits. The code
	// is nil once it overflows its length.
	code := make([]bool, 0, maxPrefixLength)
	overflowed := false

	for currentLength := uint8(1); currentLength <= maxPrefixLength && currentLength != 0; currentLength++ {
		if !overflowed {
			code = append(code, false)
		}

		for i := range lines {
			if lines[i].prefixLength != currentLength {
				continue
			}
			if overflowed {
				return nil, ErrHuffmanInvalidCode
			}
			var err error
			nodes, err = insertHuffmanCode(nodes, code, &lines[i])
			if err != nil {
				return nil, err
			}

			// Find the last zero bit, set it, and clear the bits after it.
			last := -1
			for j := len(code) - 1; j >= 0; j-- {
				if !code[j] {
					last = j
					break
				}
			}
			if last < 0 {
				overflowed = true
				continue
			}
			code[last] = true
			for j := last + 1; j < len(code); j++ {
				code[j] = false
			}
		}

		if currentLength == math.MaxUint8 {
			break
		}
	}

	return huffmanTableFromNodes(nodes), nil
}

// buildUniformHuffmanTable is HuffmanTable::build_uniform. It gives each of
// numSymbols symbols a code of codeLength bits.
func buildUniformHuffmanTable(numSymbols, codeLength uint32) (*huffmanTable, error) {
	lines := make([]tableLine, numSymbols)
	for i := range lines {
		lines[i] = newTableLine(int32(i), uint8(codeLength), 0)
	}
	return buildHuffmanTable(lines)
}

// insertHuffmanCode is HuffmanTable::insert_code. It returns the nodes,
// which it can grow.
func insertHuffmanCode(nodes []huffmanNode, code []bool, line *tableLine) ([]huffmanNode, error) {
	var nodeIndex uint32

	for _, bit := range code {
		if idx := nodes[nodeIndex].getChild(!bit); idx != 0 {
			nodeIndex = idx
			continue
		}
		newIdx := uint32(len(nodes))
		nodes = append(nodes, newIntermediateNode())
		if err := nodes[nodeIndex].setChild(!bit, newIdx); err != nil {
			return nodes, err
		}
		nodeIndex = newIdx
	}

	nodes[nodeIndex] = newLeafNode(line.rangeLow, line.rangeLength, line.isLower, line.isOutOfBand)
	return nodes, nil
}

// readCustomHuffmanTable is HuffmanTable::read_custom. It reads the table of
// a tables segment (T.88 B.2).
func readCustomHuffmanTable(r *reader) (*huffmanTable, error) {
	flags, ok := r.readByte()
	if !ok {
		return nil, ErrUnexpectedEOF
	}

	hasOutOfBand := flags&1 != 0
	prefixLengthBits := (flags>>1)&7 + 1
	rangeLengthBits := (flags>>4)&7 + 1

	minimumValue, ok := r.readI32()
	if !ok {
		return nil, ErrUnexpectedEOF
	}
	maximumValue, ok := r.readI32()
	if !ok {
		return nil, ErrUnexpectedEOF
	}

	var lines []tableLine
	currentRangeLow := minimumValue

	for currentRangeLow < maximumValue {
		prefixLength, ok := r.readBits(prefixLengthBits)
		if !ok {
			return nil, ErrHuffmanInvalidCode
		}
		rangeLength, ok := r.readBits(rangeLengthBits)
		if !ok {
			return nil, ErrHuffmanInvalidCode
		}

		lines = append(lines, newTableLine(currentRangeLow, uint8(prefixLength), uint8(rangeLength)))

		// The shift of 1 by 64 bits or more overflows.
		if uint8(rangeLength) >= 64 {
			return nil, ErrHuffmanInvalidCode
		}
		rangeSize := int64(1) << uint8(rangeLength)
		next, ok := checkedAddInt64(int64(currentRangeLow), rangeSize)
		if !ok || next < math.MinInt32 || next > math.MaxInt32 {
			return nil, ErrHuffmanInvalidCode
		}
		currentRangeLow = int32(next)
	}

	lowerPrefix, ok := r.readBits(prefixLengthBits)
	if !ok {
		return nil, ErrHuffmanInvalidCode
	}
	lines = append(lines, lowerTableLine(int64(minimumValue)-1, uint8(lowerPrefix), 32))

	upperPrefix, ok := r.readBits(prefixLengthBits)
	if !ok {
		return nil, ErrHuffmanInvalidCode
	}
	lines = append(lines, upperTableLine(maximumValue, uint8(upperPrefix), 32))

	if hasOutOfBand {
		oobPrefix, ok := r.readBits(prefixLengthBits)
		if !ok {
			return nil, ErrHuffmanInvalidCode
		}
		lines = append(lines, oobTableLine(uint8(oobPrefix)))
	}

	return buildHuffmanTable(lines)
}

// checkedAddInt64 is i64::checked_add of the Rust core library.
func checkedAddInt64(a, b int64) (int64, bool) {
	s := a + b
	if (b > 0 && s < a) || (b < 0 && s > a) {
		return 0, false
	}
	return s, true
}

// tableLine is TableLine in hayro-jbig2 src/huffman_table.rs, a line of a
// Huffman table of T.88 Annex B.
type tableLine struct {
	rangeLow     int64
	prefixLength uint8
	rangeLength  uint8
	isLower      bool
	isOutOfBand  bool
}

// newTableLine is TableLine::new.
func newTableLine(rangeLow int32, prefixLength, rangeLength uint8) tableLine {
	return tableLine{rangeLow: int64(rangeLow), prefixLength: prefixLength, rangeLength: rangeLength}
}

// lowerTableLine is TableLine::lower, the line of the lower range. Its value
// is the range high minus the offset.
func lowerTableLine(rangeHigh int64, prefixLength, rangeLength uint8) tableLine {
	return tableLine{rangeLow: rangeHigh, prefixLength: prefixLength, rangeLength: rangeLength, isLower: true}
}

// upperTableLine is TableLine::upper, the line of the upper range.
func upperTableLine(rangeLow int32, prefixLength, rangeLength uint8) tableLine {
	return tableLine{rangeLow: int64(rangeLow), prefixLength: prefixLength, rangeLength: rangeLength}
}

// oobTableLine is TableLine::oob, the line of the out-of-band value.
func oobTableLine(prefixLength uint8) tableLine {
	return tableLine{prefixLength: prefixLength, isOutOfBand: true}
}

// The kinds of a huffmanNode.
const (
	nodeEmpty uint8 = iota
	nodeIntermediate
	nodeLeaf
)

// huffmanNode is HuffmanNode in hayro-jbig2 src/huffman_table.rs.
//
// Port: the enum of upstream is a struct with a kind. An index of 0 is the
// None of a child, because the root is never a child. The zero value is the
// node Empty.
type huffmanNode struct {
	kind uint8
	zero uint32
	one  uint32
	leaf leafData
}

// newIntermediateNode is HuffmanNode::new_intermediate.
func newIntermediateNode() huffmanNode {
	return huffmanNode{kind: nodeIntermediate}
}

// newLeafNode is HuffmanNode::new_leaf.
func newLeafNode(rangeLow int64, rangeLength uint8, isLower, isOutOfBand bool) huffmanNode {
	return huffmanNode{kind: nodeLeaf, leaf: leafData{
		rangeLow:    rangeLow,
		rangeLength: rangeLength,
		isLower:     isLower,
		isOutOfBand: isOutOfBand,
	}}
}

// getChild is HuffmanNode::get_child. It returns 0 for no child.
func (n *huffmanNode) getChild(childZero bool) uint32 {
	if n.kind != nodeIntermediate {
		return 0
	}
	if childZero {
		return n.zero
	}
	return n.one
}

// setChild is HuffmanNode::set_child.
func (n *huffmanNode) setChild(childZero bool, index uint32) error {
	if n.kind != nodeIntermediate {
		return ErrHuffmanConflictingCodes
	}
	if childZero {
		n.zero = index
	} else {
		n.one = index
	}
	return nil
}

// huffmanDecodeFrom is HuffmanNode::decode_from. It follows the bits from
// the node nodeIndex to a leaf.
func huffmanDecodeFrom(nodes []huffmanNode, nodeIndex uint32, r *reader) (int32, bool, error) {
	for {
		if int(nodeIndex) >= len(nodes) {
			return 0, false, ErrHuffmanInvalidCode
		}
		n := &nodes[nodeIndex]
		switch n.kind {
		case nodeIntermediate:
			bit, ok := r.readBit()
			if !ok {
				return 0, false, ErrUnexpectedEOF
			}
			child := n.one
			if bit == 0 {
				child = n.zero
			}
			if child == 0 {
				return 0, false, ErrHuffmanInvalidCode
			}
			nodeIndex = child
		case nodeLeaf:
			leaf := &n.leaf
			if leaf.isOutOfBand {
				return 0, false, nil
			}

			offset, ok := r.readBits(leaf.rangeLength)
			if !ok {
				return 0, false, ErrHuffmanInvalidCode
			}
			rangeOffset := int64(offset)

			value := leaf.rangeLow + rangeOffset
			if leaf.isLower {
				value = leaf.rangeLow - rangeOffset
			}

			if value < math.MinInt32 || value > math.MaxInt32 {
				return 0, false, ErrHuffmanInvalidCode
			}
			return int32(value), true, nil
		default:
			return 0, false, ErrHuffmanInvalidCode
		}
	}
}

// leafData is LeafData in hayro-jbig2 src/huffman_table.rs.
type leafData struct {
	rangeLow    int64
	rangeLength uint8
	isLower     bool
	isOutOfBand bool
}

// The standard tables B.1 to B.15 of T.88 Annex B.
var (
	standardTableA = huffmanTableFromNodes(tableANodes[:])
	standardTableB = huffmanTableFromNodes(tableBNodes[:])
	standardTableC = huffmanTableFromNodes(tableCNodes[:])
	standardTableD = huffmanTableFromNodes(tableDNodes[:])
	standardTableE = huffmanTableFromNodes(tableENodes[:])
	standardTableF = huffmanTableFromNodes(tableFNodes[:])
	standardTableG = huffmanTableFromNodes(tableGNodes[:])
	standardTableH = huffmanTableFromNodes(tableHNodes[:])
	standardTableI = huffmanTableFromNodes(tableINodes[:])
	standardTableJ = huffmanTableFromNodes(tableJNodes[:])
	standardTableK = huffmanTableFromNodes(tableKNodes[:])
	standardTableL = huffmanTableFromNodes(tableLNodes[:])
	standardTableM = huffmanTableFromNodes(tableMNodes[:])
	standardTableN = huffmanTableFromNodes(tableNNodes[:])
	standardTableO = huffmanTableFromNodes(tableONodes[:])
)

// standardHuffmanTables is StandardHuffmanTables in hayro-jbig2
// src/huffman_table.rs. It gives the standard tables. The tables never
// change, so a decoder can share them.
type standardHuffmanTables struct{}

func (standardHuffmanTables) tableA() *huffmanTable { return standardTableA }
func (standardHuffmanTables) tableB() *huffmanTable { return standardTableB }
func (standardHuffmanTables) tableC() *huffmanTable { return standardTableC }
func (standardHuffmanTables) tableD() *huffmanTable { return standardTableD }
func (standardHuffmanTables) tableE() *huffmanTable { return standardTableE }
func (standardHuffmanTables) tableF() *huffmanTable { return standardTableF }
func (standardHuffmanTables) tableG() *huffmanTable { return standardTableG }
func (standardHuffmanTables) tableH() *huffmanTable { return standardTableH }
func (standardHuffmanTables) tableI() *huffmanTable { return standardTableI }
func (standardHuffmanTables) tableJ() *huffmanTable { return standardTableJ }
func (standardHuffmanTables) tableK() *huffmanTable { return standardTableK }
func (standardHuffmanTables) tableL() *huffmanTable { return standardTableL }
func (standardHuffmanTables) tableM() *huffmanTable { return standardTableM }
func (standardHuffmanTables) tableN() *huffmanTable { return standardTableN }
func (standardHuffmanTables) tableO() *huffmanTable { return standardTableO }
