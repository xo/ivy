// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-cmap src/parse.rs.

package cmap

import (
	"bytes"
	"fmt"
	"math"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/hayro/postscript"
)

// errFormat returns an error for a malformed CMap. It wraps ivy.ErrFormat.
func errFormat(msg string) error {
	return fmt.Errorf("cmap: %s: %w", msg, ivy.ErrFormat)
}

// errLimit returns an error for a CMap that exceeds a limit. It wraps
// ivy.ErrLimit.
func errLimit(msg string) error {
	return fmt.Errorf("cmap: %s: %w", msg, ivy.ErrLimit)
}

// wrapScan wraps an error of the scanner, which already wraps a sentinel
// error of ivy.
func wrapScan(err error) error {
	return fmt.Errorf("cmap: %w", err)
}

// parseContext is Context in hayro-cmap src/parse.rs. It holds a buffer that
// the parser decodes each string into, and the resolver of usecmap.
//
// Port: budget is the number of CMaps that usecmap can still read. It is the
// limit maxNestedParses.
type parseContext struct {
	buf     []byte
	getCMap Resolver
	budget  *int
}

// parseInner is parse_inner in hayro-cmap src/parse.rs. It parses a CMap at
// depth, the number of usecmap operators that lead to it.
func parseInner(data []byte, getCMap Resolver, depth uint32, budget *int) (*CMap, error) {
	if depth >= maxNestingDepth {
		return nil, errLimit("usecmap nests too deep")
	}

	if bytes.HasPrefix(data, []byte(bcmapMagic)) {
		return parseBCMap(data, getCMap, depth, budget)
	}

	scanner := postscript.NewScanner(data)
	ctx := &parseContext{getCMap: getCMap, budget: budget}
	var codespaceRanges []CodespaceRange
	var ranges, notdefRanges partitionedRanges
	var bfEntries []bfRange
	var base *CMap

	var registry, ordering, cmapName []byte
	var hasRegistry, hasOrdering, hasSupplement, hasName bool
	var supplement int32
	var writingMode *WritingMode
	var last postscript.Token

	for !scanner.AtEnd() {
		obj, err := scanner.ParseToken()
		if err != nil {
			return nil, wrapScan(err)
		}
		prev := last
		last = obj

		name, ok := obj.(postscript.Name)
		if !ok {
			continue
		}
		str, _ := name.AsStr()

		if name.IsLiteral() {
			switch str {
			// Some generators write Registry and Ordering as names, not as
			// strings.
			case "Registry":
				registry, hasRegistry = parseStringOrName(scanner)
			case "Ordering":
				ordering, hasOrdering = parseStringOrName(scanner)
			case "Supplement":
				n, ok := parseInteger(scanner)
				hasSupplement = ok && n >= math.MinInt32 && n <= math.MaxInt32
				supplement = int32(n)
			case "CMapName":
				n, err := scanner.ParseName()
				hasName = err == nil
				cmapName = append([]byte{}, n.AsBytes()...)
			case "WMode":
				writingMode = parseWritingMode(scanner)
			}
			continue
		}

		switch str {
		case "begincodespacerange":
			err = parseCodespaceRange(scanner, &codespaceRanges, ctx)
		case "begincidrange":
			err = parseRange(scanner, &ranges, ctx, "endcidrange")
		case "begincidchar":
			err = parseChar(scanner, &ranges, ctx, "endcidchar")
		case "beginnotdefrange":
			err = parseRange(scanner, &notdefRanges, ctx, "endnotdefrange")
		case "beginnotdefchar":
			err = parseChar(scanner, &notdefRanges, ctx, "endnotdefchar")
		case "beginbfchar":
			err = parseBfChar(scanner, &bfEntries, ctx)
		case "beginbfrange":
			err = parseBfRange(scanner, &bfEntries, ctx)
		case "usecmap":
			base, err = parseUseCMap(prev, ctx, depth)
		}
		if err != nil {
			return nil, err
		}
	}

	ranges.sort()
	notdefRanges.sort()
	sortBfEntries(bfEntries)

	// PDFJS-3323 has an invalid CIDSystemInfo, which the parser ignores.
	var characterCollection *CharacterCollection
	if hasRegistry && hasOrdering && hasSupplement {
		characterCollection = &CharacterCollection{
			Family:     CIDFamilyFromRegistryOrdering(registry, ordering),
			Supplement: supplement,
		}
	}
	if !hasName {
		cmapName = nil
	}

	return &CMap{
		metadata: Metadata{
			CharacterCollection: characterCollection,
			Name:                cmapName,
			WritingMode:         writingMode,
		},
		codespaceRanges: codespaceRanges,
		cidRanges:       ranges,
		notdefRanges:    notdefRanges,
		bfEntries:       bfEntries,
		base:            base,
	}, nil
}

// parseUseCMap is the usecmap branch of parse_inner in hayro-cmap
// src/parse.rs. The operand is the token before usecmap, which must be a
// literal name.
//
// Port: upstream writes the branch inside parse_inner. Go also counts the
// CMap against the limit maxNestedParses.
func parseUseCMap(prev postscript.Token, ctx *parseContext, depth uint32) (*CMap, error) {
	operand, ok := prev.(postscript.Name)
	if !ok || !operand.IsLiteral() {
		return nil, errFormat("usecmap needs a literal name")
	}
	nested, err := resolve(ctx.getCMap, NameFromBytes(operand.AsBytes()), ctx.budget)
	if err != nil {
		return nil, err
	}
	return parseInner(nested, ctx.getCMap, depth+1, ctx.budget)
}

// resolve calls getCMap for name, and it counts the call against budget.
func resolve(getCMap Resolver, name Name, budget *int) ([]byte, error) {
	if *budget <= 0 {
		return nil, errLimit("too many usecmap operators")
	}
	*budget--
	if getCMap == nil {
		return nil, errFormat(fmt.Sprintf("usecmap names the unknown CMap %q", string(name)))
	}
	data, ok := getCMap(name)
	if !ok {
		return nil, errFormat(fmt.Sprintf("usecmap names the unknown CMap %q", string(name)))
	}
	return data, nil
}

// parseWritingMode is parse_writing_mode in hayro-cmap src/parse.rs.
func parseWritingMode(scanner *postscript.Scanner) *WritingMode {
	n, ok := parseInteger(scanner)
	if !ok {
		return nil
	}
	var m WritingMode
	switch n {
	case 0:
		m = Horizontal
	case 1:
		m = Vertical
	default:
		return nil
	}
	return &m
}

// parseInteger is parse_integer in hayro-cmap src/parse.rs. It reads the
// next token, which must be an integer.
func parseInteger(scanner *postscript.Scanner) (int64, bool) {
	n, err := scanner.ParseNumber()
	if err != nil || n.IsReal {
		return 0, false
	}
	return n.Integer, true
}

// parseStringOrName is parse_string_or_name in hayro-cmap src/parse.rs. It
// reads the next token, a string or a name, and returns its bytes.
func parseStringOrName(scanner *postscript.Scanner) ([]byte, bool) {
	tok, err := scanner.ParseToken()
	if err != nil {
		return nil, false
	}
	switch tok := tok.(type) {
	case postscript.String:
		b, err := tok.Decode()
		return b, err == nil
	case postscript.Name:
		return append([]byte{}, tok.AsBytes()...), true
	}
	return nil, false
}

// parseCodespaceRange is parse_codespace_range in hayro-cmap src/parse.rs.
// The low and the high code of a range must have the same length.
func parseCodespaceRange(scanner *postscript.Scanner, ranges *[]CodespaceRange, ctx *parseContext) error {
	for {
		obj, err := scanner.ParseToken()
		if err != nil {
			return wrapScan(err)
		}
		if nameMatches(obj, "endcodespacerange") {
			return nil
		}

		low, err := extractU32Code(obj, &ctx.buf)
		if err != nil {
			return err
		}
		nBytes := len(ctx.buf)
		high, err := readU32Code(scanner, &ctx.buf)
		if err != nil {
			return err
		}
		if len(ctx.buf) != nBytes {
			return errFormat("the codes of a codespace range differ in length")
		}

		*ranges = append(*ranges, CodespaceRange{
			numberBytes: uint8(nBytes),
			low:         low,
			high:        high,
		})
	}
}

// parseRange is parse_range in hayro-cmap src/parse.rs. It reads the ranges
// of a cidrange or a notdefrange section up to endMarker.
func parseRange(scanner *postscript.Scanner, ranges *partitionedRanges, ctx *parseContext, endMarker string) error {
	for {
		obj, err := scanner.ParseToken()
		if err != nil {
			return wrapScan(err)
		}
		if nameMatches(obj, endMarker) {
			return nil
		}

		start, err := extractU32Code(obj, &ctx.buf)
		if err != nil {
			return err
		}
		byteLen := len(ctx.buf)
		end, err := readU32Code(scanner, &ctx.buf)
		if err != nil {
			return err
		}
		cidStart, err := parseCID(scanner)
		if err != nil {
			return err
		}

		ranges.push(byteLen, CIDRange{rng: codeRange{start: start, end: end}, cidStart: cidStart})
	}
}

// parseChar is parse_char in hayro-cmap src/parse.rs. It reads the codes of
// a cidchar or a notdefchar section up to endMarker.
func parseChar(scanner *postscript.Scanner, ranges *partitionedRanges, ctx *parseContext, endMarker string) error {
	for {
		obj, err := scanner.ParseToken()
		if err != nil {
			return wrapScan(err)
		}
		if nameMatches(obj, endMarker) {
			return nil
		}

		code, err := extractU32Code(obj, &ctx.buf)
		if err != nil {
			return err
		}
		byteLen := len(ctx.buf)
		cidStart, err := parseCID(scanner)
		if err != nil {
			return err
		}

		ranges.push(byteLen, CIDRange{rng: codeRange{start: code, end: code}, cidStart: cidStart})
	}
}

// parseCID is u32::try_from(parse_integer(scanner)?) in hayro-cmap
// src/parse.rs. It reads an integer that fits a CID.
//
// Port: upstream writes the expression twice.
func parseCID(scanner *postscript.Scanner) (CID, error) {
	n, ok := parseInteger(scanner)
	if !ok || n < 0 || n > math.MaxUint32 {
		return 0, errFormat("a CID is not an integer of 32 bits")
	}
	return CID(n), nil
}

// parseBfChar is parse_bf_char in hayro-cmap src/parse.rs. It drops a code
// whose destination is empty.
func parseBfChar(scanner *postscript.Scanner, entries *[]bfRange, ctx *parseContext) error {
	for {
		obj, err := scanner.ParseToken()
		if err != nil {
			return wrapScan(err)
		}
		if nameMatches(obj, "endbfchar") {
			return nil
		}

		code, err := extractU32Code(obj, &ctx.buf)
		if err != nil {
			return err
		}
		dst, err := scanner.ParseString()
		if err != nil {
			return wrapScan(err)
		}
		if ctx.buf, err = dst.DecodeInto(ctx.buf); err != nil {
			return wrapScan(err)
		}

		dstBase := decodeBE(ctx.buf)
		if dstBase == nil {
			continue
		}
		*entries = append(*entries, bfRange{rng: codeRange{start: code, end: code}, dstBase: dstBase})
	}
}

// parseBfRange is parse_bf_range in hayro-cmap src/parse.rs. A destination is
// a string, or an array with a string for each code of the range. It drops
// a code whose destination is empty.
func parseBfRange(scanner *postscript.Scanner, entries *[]bfRange, ctx *parseContext) error {
	for {
		obj, err := scanner.ParseToken()
		if err != nil {
			return wrapScan(err)
		}
		if nameMatches(obj, "endbfrange") {
			return nil
		}

		start, err := extractU32Code(obj, &ctx.buf)
		if err != nil {
			return err
		}
		end, err := readU32Code(scanner, &ctx.buf)
		if err != nil {
			return err
		}

		next, err := scanner.ParseToken()
		if err != nil {
			return wrapScan(err)
		}

		if s, ok := next.(postscript.String); ok {
			if ctx.buf, err = s.DecodeInto(ctx.buf); err != nil {
				return wrapScan(err)
			}
			dstBase := decodeBE(ctx.buf)
			if dstBase == nil {
				continue
			}
			*entries = append(*entries, bfRange{rng: codeRange{start: start, end: end}, dstBase: dstBase})
			continue
		}

		if !nameMatches(next, "[") {
			return errFormat("a bfrange destination is not a string or an array")
		}
		// Each step reads a token, so the input bounds the loop. The counter
		// has 64 bits, so that an end of 0xffffffff does not wrap.
		for code := uint64(start); code <= uint64(end); code++ {
			s, err := scanner.ParseString()
			if err != nil {
				return wrapScan(err)
			}
			if ctx.buf, err = s.DecodeInto(ctx.buf); err != nil {
				return wrapScan(err)
			}
			dstBase := decodeBE(ctx.buf)
			if dstBase == nil {
				continue
			}
			c := uint32(code)
			*entries = append(*entries, bfRange{rng: codeRange{start: c, end: c}, dstBase: dstBase})
		}

		closing, err := scanner.ParseToken()
		if err != nil {
			return wrapScan(err)
		}
		if !nameMatches(closing, "]") {
			return errFormat("a bfrange array has no ]")
		}
	}
}

// decodeBE is decode_be in hayro-cmap src/parse.rs. It reads the bytes as
// units of UTF-16, big end first. It returns nil for no bytes.
func decodeBE(b []byte) []uint16 {
	if len(b) == 0 {
		return nil
	}

	out := make([]uint16, 0, (len(b)+1)/2)
	i := 0

	// Some files have bf strings of an odd length. The first byte is then a
	// unit of its own.
	if len(b)%2 != 0 {
		out = append(out, uint16(b[0]))
		i = 1
	}

	for ; i+1 < len(b); i += 2 {
		out = append(out, uint16(b[i])<<8|uint16(b[i+1]))
	}

	return out
}

// readU32Code is read_u32_code in hayro-cmap src/parse.rs. It reads a string
// and returns its bytes as a code.
func readU32Code(scanner *postscript.Scanner, buf *[]byte) (uint32, error) {
	s, err := scanner.ParseString()
	if err != nil {
		return 0, wrapScan(err)
	}
	if *buf, err = s.DecodeInto(*buf); err != nil {
		return 0, wrapScan(err)
	}
	return bytesToU32(*buf)
}

// extractU32Code is extract_u32_code in hayro-cmap src/parse.rs. The token
// must be a string, and the function returns its bytes as a code.
func extractU32Code(obj postscript.Token, buf *[]byte) (uint32, error) {
	s, ok := obj.(postscript.String)
	if !ok {
		return 0, errFormat("a code is not a string")
	}
	var err error
	if *buf, err = s.DecodeInto(*buf); err != nil {
		return 0, wrapScan(err)
	}
	return bytesToU32(*buf)
}

// bytesToU32 is bytes_to_u32 in hayro-cmap src/parse.rs. A code has one to
// four bytes, the big end first.
func bytesToU32(b []byte) (uint32, error) {
	if len(b) == 0 || len(b) > 4 {
		return 0, errFormat("a code does not have one to four bytes")
	}
	var v uint32
	for _, c := range b {
		v = v<<8 | uint32(c)
	}
	return v, nil
}

// nameMatches is name_matches in hayro-cmap src/parse.rs. It reports whether
// obj is the name expected and not a literal name.
func nameMatches(obj postscript.Token, expected string) bool {
	name, ok := obj.(postscript.Name)
	if !ok || name.IsLiteral() {
		return false
	}
	s, ok := name.AsStr()
	return ok && s == expected
}
