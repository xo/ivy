// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-cmap src/bcmap/huffman.rs.

package cmap

// huffmanNode is HuffmanNode in hayro-cmap src/bcmap/huffman.rs. It is a leaf
// with a symbol, or an inner node with two children.
//
// Port: upstream is an enum, and a child is an Option<NonZeroU32>. Go uses
// one struct, and the index 0 for no child. The root has the index 0, so no
// node has it as a child.
type huffmanNode struct {
	leaf   bool
	symbol uint32
	zero   uint32
	one    uint32
}

// huffmanTable is HuffmanTable in hayro-cmap src/bcmap/huffman.rs. It is a
// tree of codes. The node 0 is the root.
type huffmanTable struct {
	nodes []huffmanNode
}

// decode is HuffmanTable::decode in hayro-cmap src/bcmap/huffman.rs. It
// reads bits up to a leaf, and it returns the symbol of the leaf.
//
// Port: upstream indexes the nodes without a test. Go returns false for an
// index out of range.
func (h *huffmanTable) decode(r *reader) (uint32, bool) {
	idx := uint32(0)

	for {
		if int(idx) >= len(h.nodes) {
			return 0, false
		}
		node := h.nodes[idx]
		if node.leaf {
			return node.symbol, true
		}
		bit, ok := r.readBit()
		if !ok {
			return 0, false
		}
		child := node.zero
		if bit != 0 {
			child = node.one
		}
		if child == 0 {
			return 0, false
		}
		idx = child
	}
}

// insertCode is HuffmanTable::insert_code in hayro-cmap
// src/bcmap/huffman.rs. It adds the code of length bits below the node, and
// it makes the inner nodes that it needs. The recursion is as deep as the
// code is long, at most 255.
func insertCode(nodes *[]huffmanNode, nodeIndex, code uint32, length uint8, symbol uint32) {
	if length == 0 {
		(*nodes)[nodeIndex] = huffmanNode{leaf: true, symbol: symbol}
		return
	}

	bit := (code >> (length - 1)) & 1
	remaining := code & ((1 << (length - 1)) - 1)

	node := &(*nodes)[nodeIndex]
	if node.leaf {
		return
	}
	child := node.zero
	if bit != 0 {
		child = node.one
	}
	if child == 0 {
		child = uint32(len(*nodes))
		if bit == 0 {
			node.zero = child
		} else {
			node.one = child
		}
		*nodes = append(*nodes, huffmanNode{})
	}

	insertCode(nodes, child, remaining, length-1, symbol)
}

// buildHuffmanTable is HuffmanTable::build in hayro-cmap
// src/bcmap/huffman.rs. It gives the canonical codes to the symbols, in the
// order of their lengths.
func buildHuffmanTable(codeLengths []uint8, symbols []uint32) huffmanTable {
	if len(symbols) == 0 {
		return huffmanTable{nodes: []huffmanNode{{}}}
	}

	maxLength := uint8(0)
	for _, l := range codeLengths {
		maxLength = max(maxLength, l)
	}

	codes := make([]uint32, len(symbols))
	code := uint32(0)

	for length := 1; length <= int(maxLength); length++ {
		for i, cl := range codeLengths {
			if int(cl) == length {
				codes[i] = code
				code++
			}
		}
		code <<= 1
	}

	nodes := []huffmanNode{{}}

	for i, symbol := range symbols {
		if codeLengths[i] == 0 {
			continue
		}
		insertCode(&nodes, 0, codes[i], codeLengths[i], symbol)
	}

	return huffmanTable{nodes: nodes}
}

// decodeTables is decode_tables in hayro-cmap src/bcmap/huffman.rs. It reads
// the table of the deltas and the table of the counts.
func decodeTables(data []byte) (delta, count huffmanTable, ok bool) {
	r := newBCMapReader(data)
	delta, ok = decodeSingleTable(r, (*reader).readU32)
	if !ok {
		return huffmanTable{}, huffmanTable{}, false
	}
	count, ok = decodeSingleTable(r, func(r *reader) (uint32, bool) {
		v, ok := r.readU8()
		return uint32(v), ok
	})
	if !ok {
		return huffmanTable{}, huffmanTable{}, false
	}
	return delta, count, true
}

// decodeSingleTable is decode_single_table in hayro-cmap
// src/bcmap/huffman.rs. It reads the number of codes of each length, then the
// symbols in the order of their codes.
func decodeSingleTable(r *reader, readSym func(*reader) (uint32, bool)) (huffmanTable, bool) {
	nSymbols, ok := r.readU16()
	if !ok {
		return huffmanTable{}, false
	}

	if nSymbols == 0 {
		if _, ok := r.readU8(); !ok {
			return huffmanTable{}, false
		}
		return huffmanTable{nodes: []huffmanNode{{}}}, true
	}

	maxCodeLength, ok := r.readU8()
	if !ok {
		return huffmanTable{}, false
	}

	counts := make([]uint16, 0, maxCodeLength)
	for range maxCodeLength {
		c, ok := r.readU16()
		if !ok {
			return huffmanTable{}, false
		}
		counts = append(counts, c)
	}

	codeLengths := make([]uint8, 0, nSymbols)
	symbols := make([]uint32, 0, nSymbols)

	for lengthIdx, count := range counts {
		codeLen := uint8(lengthIdx + 1)
		for range count {
			sym, ok := readSym(r)
			if !ok {
				return huffmanTable{}, false
			}
			symbols = append(symbols, sym)
			codeLengths = append(codeLengths, codeLen)
		}
	}

	return buildHuffmanTable(codeLengths, symbols), true
}
