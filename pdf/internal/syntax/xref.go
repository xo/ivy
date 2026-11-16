// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/xref.rs.

package syntax

import (
	"encoding/binary"
	"errors"
	"iter"
	"maps"
	"math"
	"sort"
	"sync"
)

// xrefEntryLen is XREF_ENTRY_LEN in hayro-syntax src/xref.rs.
const xrefEntryLen = 20

// xrefError is XRefError in hayro-syntax src/xref.rs. A zero decryption
// stands for the case Unknown.
type xrefError struct {
	decryption DecryptionError
}

// rootXRef is root_xref in hayro-syntax src/xref.rs. It reads the xref table
// that the last startxref points to.
func rootXRef(data []byte, password []byte, env *decodeEnv) (*XRef, *xrefError) {
	m := xrefMap{}
	xrefPos, ok := findLastXRefPos(data)
	if !ok {
		return nil, &xrefError{}
	}
	trailer, ok := populateXRefImpl(data, xrefPos, m, env)
	if !ok {
		return nil, &xrefError{}
	}
	return newXRef(data, m, xrefInput{trailerDictData: trailer}, false, password, env)
}

// fallback is fallback in hayro-syntax src/xref.rs. It rebuilds the xref
// table from a scan of the whole file.
func fallback(data []byte, password []byte, env *decodeEnv) (*XRef, bool) {
	m, input, ok := fallbackXRefMap(data, password, env)
	if !ok {
		return nil, false
	}
	x, err := newXRef(data, m, input, true, password, env)
	if err != nil {
		return nil, false
	}
	return x, true
}

// fallbackXRefMap is fallback_xref_map in hayro-syntax src/xref.rs.
func fallbackXRefMap(data []byte, password []byte, env *decodeEnv) (xrefMap, xrefInput, bool) {
	return fallbackXRefMapInner(data, dummyEnvContext(env), true, password, env)
}

// fallbackXRefMapInner is fallback_xref_map_inner in hayro-syntax
// src/xref.rs.
func fallbackXRefMapInner(data []byte, dummyCtx *ReaderContext, recurse bool, password []byte, env *decodeEnv) (xrefMap, xrefInput, bool) {
	m := xrefMap{}
	var trailerDicts []Dict
	var rootRef ObjectIdentifier
	hasRootRef := false
	r := NewReader(data)
	var lastObjNum ObjectIdentifier
	hasLastObjNum := false
	for {
		curPos := r.Offset()
		oldR := r.clone()
		if b, ok := r.PeekByte(); ok && isDigit(b) {
			if objID, ok := r.Read[ObjectIdentifier](dummyCtx); ok {
				cloned := r.clone()
				cloned.SkipWhiteSpacesAndComments()
				if _, ok := cloned.Skip[Object](false); ok {
					m[objID] = entryType{offset: curPos}
					lastObjNum, hasLastObjNum = objID, true
					dummyCtx.setObjNumber(objID)
				}
			} else {
				r.ForwardWhile(func(b byte) bool { return !isWhiteSpaceCharacter(b) })
			}
		} else {
			probeReader := r.clone()
			two, isDict := r.PeekBytes(2)
			isDict = isDict && string(two) == "<<"
			var probe dictProbe
			if isDict {
				probe, isDict = probeDict(probeReader, dummyCtx, "<<", ">>")
			}
			if isDict {
				r = probeReader
				if probe.hasRoot || probe.hasType {
					if dict, ok := NewReader(probe.data).ReadWithContext[Dict](dummyCtx); ok {
						if probe.hasRoot && dict.ContainsKey(KeyRoot) {
							trailerDicts = append(trailerDicts, dict)
						}
						typ, hasType := dict.Get[Name](KeyType)
						if hasType && typ.AsStr() == "Catalog" {
							rootRef, hasRootRef = lastObjNum, hasLastObjNum
						}
						if hasType && typ == "ObjStm" {
							fallbackObjStm(r, oldR, dummyCtx, lastObjNum, hasLastObjNum, m)
						}
					}
				}
			} else {
				oldPos := r.Offset()
				r.ForwardWhile(func(b byte) bool { return !isWhiteSpaceCharacter(b) })
				if r.Offset() == oldPos {
					r.ReadU8()
				}
			}
		}
		if r.AtEnd() {
			break
		}
	}
	var trailerDict Dict
	hasTrailer := false
	for _, dict := range trailerDicts {
		rootID, ok := dict.GetRaw[Dict](KeyRoot)
		if !ok {
			continue
		}
		check := func(d Dict) bool { return d.ContainsKey(KeyPages) }
		if !rootID.IsRef {
			if check(rootID.Value) {
				trailerDict, hasTrailer = dict, true
			}
			continue
		}
		entry, ok := m[rootID.Ref.ID()]
		if !ok {
			continue
		}
		if !entry.objStream {
			if entry.offset < 0 || entry.offset > len(data) {
				continue
			}
			obj, ok := NewReader(data[entry.offset:]).ReadWithContext[IndirectObject[Dict]](dummyCtx)
			if ok && check(obj.Get()) {
				trailerDict, hasTrailer = dict, true
			}
			continue
		}
		streamEntry, ok := m[NewObjectIdentifier(entry.objStreamNum, 0)]
		if !ok || streamEntry.objStream || streamEntry.offset < 0 || streamEntry.offset > len(data) {
			continue
		}
		stream, ok := NewReader(data[streamEntry.offset:]).ReadWithContext[IndirectObject[Stream]](dummyCtx)
		if !ok {
			continue
		}
		if objStream, ok := newObjectStream(stream.Get()); ok {
			if obj, ok := objectStreamGet[Dict](objStream, entry.index, dummyCtx); ok && check(obj) {
				trailerDict, hasTrailer = dict, true
			}
		}
	}
	hasEncryption := hasTrailer && trailerDict.ContainsKey(KeyEncrypt)
	if hasEncryption && recurse {
		// Redo the scan with a context that decrypts, so that the scan finds
		// the objects in encrypted object streams.
		if x, err := newXRef(data, maps.Clone(m), xrefInput{trailerDictData: trailerDict.Data()}, true, password, env); err == nil {
			patched, _, _ := fallbackXRefMapInner(data, newReaderContext(x, false), false, password, env)
			m = patched
		}
	}
	switch {
	case hasTrailer:
		return m, xrefInput{trailerDictData: trailerDict.Data()}, true
	case hasRootRef:
		return m, xrefInput{rootRef: rootRef, isRootRef: true}, true
	}
	return m, xrefInput{}, false
}

// fallbackObjStm adds the members of an object stream to the map of the
// fallback scan. It is a part of fallback_xref_map_inner in hayro-syntax
// src/xref.rs.
func fallbackObjStm(r, oldR *Reader, ctx *ReaderContext, lastObjNum ObjectIdentifier, hasLastObjNum bool, m xrefMap) {
	afterDict := r.clone()
	afterDict.SkipWhiteSpacesAndComments()
	if !afterDict.ForwardTag("stream") {
		return
	}
	stream, ok := oldR.ReadIndirect[Stream](ctx)
	if !ok || !hasLastObjNum {
		return
	}
	objStream, ok := newObjectStream(stream)
	if !ok {
		return
	}
	for idx, o := range objStream.offsets {
		if !o.ok {
			continue
		}
		id := NewObjectIdentifier(o.objNum, 0)
		if e, ok := m[id]; !ok || e.objStream {
			m[id] = entryType{objStream: true, objStreamNum: lastObjNum.ObjNumber, index: uint32(idx)}
		}
	}
}

// XRef is XRef in hayro-syntax src/xref.rs. It is the xref table of a PDF
// file.
//
// Port: the case Dummy of upstream is a nil *XRef. Each method accepts a nil
// receiver.
type XRef struct {
	data *pdfData
	// mu guards xrefMap and repaired, as the RwLock of upstream does.
	mu          sync.RWMutex
	xrefMap     xrefMap
	repaired    bool
	metadata    Metadata
	decryptor   *decryptor
	hasOCGs     bool
	password    []byte
	trailerData trailerData
	env         decodeEnv
}

// newXRef is XRef::new in hayro-syntax src/xref.rs.
func newXRef(data []byte, m xrefMap, input xrefInput, repaired bool, password []byte, env *decodeEnv) (*XRef, *xrefError) {
	x := &XRef{
		data:     newPdfData(data),
		xrefMap:  m,
		repaired: repaired,
		password: password,
		env:      *env,
	}
	// The trailer is read twice, so that the catalog can be in an encrypted
	// object stream.
	if !input.isRootRef {
		trailer, ok := NewReader(input.trailerDictData).ReadWithContext[Dict](newReaderContext(x, false))
		if !ok {
			return nil, &xrefError{}
		}
		d, err := getDecryptor(trailer, password)
		if err != nil {
			return nil, err
		}
		x.decryptor = d
	}
	if input.isRootRef {
		root, ok := x.Get[Dict](input.rootRef)
		if !ok {
			return nil, &xrefError{}
		}
		pagesRef, ok := root.GetRef(KeyPages)
		if !ok {
			return nil, &xrefError{}
		}
		x.trailerData = trailerData{pagesRef: pagesRef.ID(), rootRef: input.rootRef}
		return x, nil
	}
	trailer, ok := NewReader(input.trailerDictData).ReadWithContext[Dict](newReaderContext(x, false))
	if !ok {
		return nil, &xrefError{}
	}
	rootRef, ok := trailer.GetRef(KeyRoot)
	if !ok {
		return nil, &xrefError{}
	}
	root, ok := trailer.Get[Dict](KeyRoot)
	if !ok {
		return nil, &xrefError{}
	}
	var metadata Metadata
	if info, ok := trailer.Get[Dict](KeyInfo); ok {
		metadata = parseMetadata(info)
	}
	pagesRef, ok := root.GetRef(KeyPages)
	if !ok {
		return nil, &xrefError{}
	}
	_, hasOCGs := root.Get[Dict](KeyOCProperties)
	var version *PdfVersion
	if v, ok := root.Get[Name](KeyVersion); ok {
		if pv, ok := pdfVersionFromBytes([]byte(v)); ok {
			version = &pv
		}
	}
	x.trailerData = trailerData{pagesRef: pagesRef.ID(), rootRef: rootRef.ID(), version: version}
	x.hasOCGs = hasOCGs
	x.metadata = metadata
	return x, nil
}

// len is XRef::len in hayro-syntax src/xref.rs.
func (x *XRef) len() int {
	if x == nil {
		return 0
	}
	x.mu.RLock()
	defer x.mu.RUnlock()
	return len(x.xrefMap)
}

// RootID is XRef::root_id in hayro-syntax src/xref.rs. It returns the
// identifier of the root dictionary.
func (x *XRef) RootID() ObjectIdentifier {
	if x == nil {
		return ObjectIdentifier{}
	}
	return x.trailerData.rootRef
}

// HasOptionalContentGroups is XRef::has_optional_content_groups in
// hayro-syntax src/xref.rs.
func (x *XRef) HasOptionalContentGroups() bool {
	return x != nil && x.hasOCGs
}

// objects is XRef::objects in hayro-syntax src/xref.rs. It returns each
// object that can be read, in the order of its offset in the file.
//
// Port: upstream sorts by the offset, and a tie keeps the order of a hash
// map. Go breaks a tie by the identifier, so that the order is stable.
func (x *XRef) objects() iter.Seq[Object] {
	return func(yield func(Object) bool) {
		if x == nil {
			return
		}
		type element struct {
			id     ObjectIdentifier
			offset int
			index  uint32
		}
		x.mu.RLock()
		elements := make([]element, 0, len(x.xrefMap))
		for id, e := range x.xrefMap {
			el := element{id: id, offset: e.offset}
			if e.objStream {
				el.offset, el.index = math.MaxInt, 0
				if s, ok := x.xrefMap[NewObjectIdentifier(e.objStreamNum, 0)]; ok && !s.objStream {
					el.offset, el.index = s.offset, e.index
				}
			}
			elements = append(elements, el)
		}
		x.mu.RUnlock()
		sort.Slice(elements, func(i, j int) bool {
			a, b := elements[i], elements[j]
			if a.offset != b.offset {
				return a.offset < b.offset
			}
			if a.index != b.index {
				return a.index < b.index
			}
			return a.id.compare(b.id) < 0
		})
		for _, el := range elements {
			if o, ok := x.getWith[Object](el.id, newReaderContext(x, false)); ok {
				if !yield(o) {
					return
				}
			}
		}
	}
}

// repair is XRef::repair in hayro-syntax src/xref.rs. It replaces the map
// with the map of a scan of the whole file, once.
func (x *XRef) repair() {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.repaired {
		return
	}
	m, _, _ := fallbackXRefMap(x.data.get(), x.password, &x.env)
	x.xrefMap = m
	x.repaired = true
}

// needsDecryption is XRef::needs_decryption in hayro-syntax src/xref.rs.
func (x *XRef) needsDecryption(ctx *ReaderContext) bool {
	if x == nil || x.decryptor == nil {
		return false
	}
	return !ctx.InContentStream() && !ctx.inObjectStreamCtx()
}

// decrypt is XRef::decrypt in hayro-syntax src/xref.rs.
func (x *XRef) decrypt(id ObjectIdentifier, data []byte, target decryptionTarget) ([]byte, bool) {
	if x == nil || x.decryptor == nil {
		return append([]byte(nil), data...), true
	}
	return x.decryptor.decrypt(id, data, target)
}

// decodeEnv returns the limits and the decoders of the document, or the
// defaults for the dummy xref table.
func (x *XRef) decodeEnv() *decodeEnv {
	if x == nil {
		return defaultDecodeEnv()
	}
	return &x.env
}

// Get is XRef::get in hayro-syntax src/xref.rs. It returns the object with
// the identifier as a T.
func (x *XRef) Get[T any](id ObjectIdentifier) (T, bool) {
	return x.getWith[T](id, newReaderContext(x, false))
}

// getWith is XRef::get_with in hayro-syntax src/xref.rs.
func (x *XRef) getWith[T any](id ObjectIdentifier, ctx *ReaderContext) (T, bool) {
	var zero T
	if x == nil {
		return zero, false
	}
	// The flag must describe the map that the entry came from.
	x.mu.RLock()
	entry, ok := x.xrefMap[id]
	wasRepaired := x.repaired
	x.mu.RUnlock()
	if !ok {
		return zero, false
	}
	data := x.data.get()
	r := NewReader(data)
	callerCtx := ctx
	ctx = ctx.clone()
	ctx.setObjNumber(id)
	ctx.setInContentStream(false)
	if !entry.objStream {
		ctx.setInObjectStream(false)
		r.Jump(entry.offset)
		if obj, ok := r.ReadWithContext[IndirectObject[T]](ctx); ok {
			if obj.ID() == id {
				return obj.Get(), true
			}
		} else if _, ok := r.Skip[IndirectObject[Object]](false); ok {
			// A valid object of another type needs no repair.
			return zero, false
		}
		return x.repairAndGet[T](id, callerCtx, wasRepaired)
	}
	objStreamID := NewObjectIdentifier(entry.objStreamNum, 0)
	if objStreamID == id || ctx.parentChainContains(id) {
		return zero, false
	}
	x.mu.RLock()
	streamEntry, ok := x.xrefMap[objStreamID]
	x.mu.RUnlock()
	if !ok || streamEntry.objStream {
		return zero, false
	}
	ctx.parentChainPush(id)
	objectStream, ok := x.data.objectStream(objStreamID, ctx)
	if !ok {
		return zero, false
	}
	if n, ok := objectStream.objNumber(entry.index); ok && n == id.ObjNumber {
		return objectStreamGet[T](objectStream, entry.index, ctx)
	}
	return x.repairAndGet[T](id, callerCtx, wasRepaired)
}

// repairAndGet is XRef::repair_and_get in hayro-syntax src/xref.rs.
func (x *XRef) repairAndGet[T any](id ObjectIdentifier, ctx *ReaderContext, wasRepaired bool) (T, bool) {
	if wasRepaired {
		var zero T
		return zero, false
	}
	x.repair()
	return x.getWith[T](id, ctx)
}

// xrefInput is XRefInput in hayro-syntax src/xref.rs. It holds the data of
// the trailer dictionary, or the reference of the root when isRootRef is
// true.
type xrefInput struct {
	trailerDictData []byte
	rootRef         ObjectIdentifier
	isRootRef       bool
}

// findLastXRefPos is find_last_xref_pos in hayro-syntax src/xref.rs. It
// returns the offset that the last startxref states.
func findLastXRefPos(data []byte) (int, bool) {
	const needle = "startxref"
	pos, ok := findrNeedle(data, []byte(needle))
	if !ok {
		return 0, false
	}
	finder := NewReader(data)
	finder.Jump(pos)
	if !finder.ForwardTag(needle) {
		return 0, false
	}
	finder.SkipWhiteSpacesAndComments()
	v, ok := finder.ReadWithoutContext[int32]()
	if !ok || v < 0 {
		return 0, false
	}
	return int(v), true
}

// entryType is EntryType in hayro-syntax src/xref.rs.
//
// Port: the Rust enum becomes a struct. objStream selects the case
// ObjStream, with objStreamNum and index. Otherwise offset holds the case
// Normal.
type entryType struct {
	offset       int
	objStream    bool
	objStreamNum uint32
	index        uint32
}

// xrefMap is XrefMap in hayro-syntax src/xref.rs.
type xrefMap map[ObjectIdentifier]entryType

// trailerData is TrailerData in hayro-syntax src/xref.rs.
type trailerData struct {
	pagesRef ObjectIdentifier
	rootRef  ObjectIdentifier
	version  *PdfVersion
}

// xrefEntry is XRefEntry in hayro-syntax src/xref.rs.
type xrefEntry struct {
	offset    int
	genNumber int32
	used      bool
}

// readXRefEntry is XRefEntry::read in hayro-syntax src/xref.rs. data holds
// the 20 bytes of an entry.
func readXRefEntry(data []byte) (xrefEntry, bool) {
	parseU32 := func(data []byte) (uint32, bool) {
		var accum uint64
		for _, b := range data {
			if b < '0' || b > '9' {
				return 0, false
			}
			accum = accum*10 + uint64(b-'0')
			if accum > math.MaxUint32 {
				return 0, false
			}
		}
		return uint32(accum), true
	}
	offset, ok := parseU32(data[0:10])
	if !ok {
		return xrefEntry{}, false
	}
	gen, ok := parseU32(data[11:16])
	if !ok || gen > math.MaxInt32 {
		return xrefEntry{}, false
	}
	return xrefEntry{offset: int(offset), genNumber: int32(gen), used: data[17] == 'n'}, true
}

// populateXRefImpl is populate_xref_impl in hayro-syntax src/xref.rs. It
// reads the xref section at pos and the sections that it chains to. It
// returns the data of the trailer dictionary.
func populateXRefImpl(data []byte, pos int, m xrefMap, env *decodeEnv) ([]byte, bool) {
	visited := map[int]bool{}
	return populateXRefImplInner(data, pos, m, visited, env)
}

// maxXRefChainDepth is MAX_XREF_CHAIN_DEPTH in hayro-syntax src/xref.rs.
const maxXRefChainDepth = 256

// populateXRefImplInner is populate_xref_impl_inner in hayro-syntax
// src/xref.rs.
func populateXRefImplInner(data []byte, pos int, m xrefMap, visited map[int]bool, env *decodeEnv) ([]byte, bool) {
	if visited[pos] {
		return nil, false
	}
	visited[pos] = true
	if len(visited) > maxXRefChainDepth {
		return nil, false
	}
	reader := NewReader(data)
	reader.Jump(pos)
	// The position can point before the object number of an xref stream.
	reader.SkipWhiteSpacesAndComments()
	r2 := reader.clone()
	if _, ok := reader.clone().ReadWithoutContext[ObjectIdentifier](); ok {
		return populateFromXRefStream(data, r2, m, visited, env)
	}
	return populateFromXRefTable(data, r2, m, visited, env)
}

// subsectionHeader is SubsectionHeader in hayro-syntax src/xref.rs.
type subsectionHeader struct {
	start      uint32
	numEntries uint32
}

// readFrom is the Readable implementation of SubsectionHeader in
// hayro-syntax src/xref.rs.
func (h *subsectionHeader) readFrom(r *Reader, _ *ReaderContext) bool {
	r.SkipWhiteSpaces()
	start, ok := r.ReadWithoutContext[uint32]()
	if !ok {
		return false
	}
	r.SkipWhiteSpaces()
	numEntries, ok := r.ReadWithoutContext[uint32]()
	if !ok {
		return false
	}
	r.SkipWhiteSpaces()
	*h = subsectionHeader{start: start, numEntries: numEntries}
	return true
}

// populateFromXRefTable is populate_from_xref_table in hayro-syntax
// src/xref.rs.
func populateFromXRefTable(data []byte, reader *Reader, m xrefMap, visited map[int]bool, env *decodeEnv) ([]byte, bool) {
	trailer, ok := readXRefTableTrailer(reader.clone(), DummyReaderContext())
	if !ok {
		return nil, false
	}
	reader.SkipWhiteSpaces()
	if !reader.ForwardTag("xref") {
		return nil, false
	}
	reader.SkipWhiteSpaces()
	if prev, ok := trailer.Get[int32](KeyPrev); ok {
		if _, ok := populateXRefImplInner(data, int(prev), m, visited, env); !ok {
			return nil, false
		}
	}
	// In hybrid files, the entries of XRefStm override those of Prev.
	if xrefStm, ok := trailer.Get[int32](KeyXRefStm); ok {
		if _, ok := populateXRefImplInner(data, int(xrefStm), m, visited, env); !ok {
			return nil, false
		}
	}
	for {
		header, ok := reader.ReadWithoutContext[subsectionHeader]()
		if !ok {
			break
		}
		reader.SkipWhiteSpaces()
		end := uint64(header.start) + uint64(header.numEntries)
		if end > math.MaxUint32 {
			return nil, false
		}
		for objNumber := uint64(header.start); objNumber < end; objNumber++ {
			b, ok := reader.ReadBytes(xrefEntryLen)
			if !ok {
				return nil, false
			}
			entry, ok := readXRefEntry(b)
			if !ok {
				return nil, false
			}
			// Entries past Size are not ignored.
			if entry.used {
				m[NewObjectIdentifier(uint32(objNumber), entry.genNumber)] = entryType{offset: entry.offset}
			}
		}
	}
	return trailer.Data(), true
}

// populateFromXRefStream is populate_from_xref_stream in hayro-syntax
// src/xref.rs.
func populateFromXRefStream(data []byte, reader *Reader, m xrefMap, visited map[int]bool, env *decodeEnv) ([]byte, bool) {
	obj, ok := reader.ReadWithContext[IndirectObject[Stream]](dummyEnvContext(env))
	if !ok {
		return nil, false
	}
	stream := obj.Get()
	dict := stream.Dict()
	if prev, ok := dict.Get[int32](KeyPrev); ok {
		if _, ok := populateXRefImplInner(data, int(prev), m, visited, env); !ok {
			return nil, false
		}
	}
	size, ok := dict.Get[uint32](KeySize)
	if !ok {
		return nil, false
	}
	w, ok := dict.Get[[3]uint8](KeyW)
	if !ok {
		return nil, false
	}
	f1Len, f2Len, f3Len := w[0], w[1], w[2]
	if f2Len == 0 || f2Len > 8 {
		return nil, false
	}
	xrefData, err := stream.DecodedData()
	if err != nil {
		return nil, false
	}
	xrefReader := NewReader(xrefData)
	if arr, ok := dict.Get[Array](KeyIndex); ok {
		for pair := range arr.Iter[Tuple2[uint32, uint32]]().All() {
			if !xrefStreamSubsection(xrefReader, pair.V0, pair.V1, f1Len, f2Len, f3Len, m) {
				return nil, false
			}
		}
	} else if !xrefStreamSubsection(xrefReader, 0, size, f1Len, f2Len, f3Len, m) {
		return nil, false
	}
	return dict.Data(), true
}

// xrefStreamNum is xref_stream_num in hayro-syntax src/xref.rs. It reads a
// big endian number of 1, 2, 3, 4 or 8 bytes.
func xrefStreamNum(data []byte) (uint32, bool) {
	switch len(data) {
	case 1:
		return uint32(data[0]), true
	case 2:
		return uint32(binary.BigEndian.Uint16(data)), true
	case 3:
		return uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2]), true
	case 4:
		return binary.BigEndian.Uint32(data), true
	case 8:
		v := binary.BigEndian.Uint64(data)
		if v > math.MaxUint32 {
			return 0, false
		}
		return uint32(v), true
	}
	return 0, false
}

// xrefStreamSubsection is xref_stream_subsection in hayro-syntax
// src/xref.rs.
func xrefStreamSubsection(xrefReader *Reader, start, numElements uint32, f1Len, f2Len, f3Len uint8, m xrefMap) bool {
	for i := range numElements {
		fType := byte(1)
		if f1Len != 0 {
			// Upstream notes that a first field wider than 1 is not
			// supported.
			b, ok := xrefReader.ReadBytes(1)
			if !ok {
				return false
			}
			fType = b[0]
		}
		if uint64(start)+uint64(i) > math.MaxUint32 {
			return false
		}
		objNumber := start + i
		switch fType {
		case 0:
			if !xrefReader.SkipBytes(int(f2Len) + int(f3Len)) {
				return false
			}
		case 1:
			var offset, genNumber uint32
			if f2Len > 0 {
				b, ok := xrefReader.ReadBytes(int(f2Len))
				if !ok {
					return false
				}
				if offset, ok = xrefStreamNum(b); !ok {
					return false
				}
			}
			if f3Len > 0 {
				b, ok := xrefReader.ReadBytes(int(f3Len))
				if !ok {
					return false
				}
				if genNumber, ok = xrefStreamNum(b); !ok {
					return false
				}
			}
			m[NewObjectIdentifier(objNumber, int32(genNumber))] = entryType{offset: int(offset)}
		case 2:
			b, ok := xrefReader.ReadBytes(int(f2Len))
			if !ok {
				return false
			}
			objStreamNumber, ok := xrefStreamNum(b)
			if !ok {
				return false
			}
			var index uint32
			if f3Len > 0 {
				b, ok := xrefReader.ReadBytes(int(f3Len))
				if !ok {
					return false
				}
				if index, ok = xrefStreamNum(b); !ok {
					return false
				}
			}
			m[NewObjectIdentifier(objNumber, 0)] = entryType{objStream: true, objStreamNum: objStreamNumber, index: index}
		default:
			return false
		}
	}
	return true
}

// readXRefTableTrailer is read_xref_table_trailer in hayro-syntax
// src/xref.rs. It skips the entries of an xref table and reads the trailer
// dictionary after them.
func readXRefTableTrailer(reader *Reader, ctx *ReaderContext) (Dict, bool) {
	reader.SkipWhiteSpaces()
	if !reader.ForwardTag("xref") {
		return Dict{}, false
	}
	reader.SkipWhiteSpaces()
	for {
		header, ok := reader.ReadWithoutContext[subsectionHeader]()
		if !ok {
			break
		}
		// The product fits into an int of 64 bits, where the checked
		// arithmetic of upstream never fails.
		reader.Jump(reader.Offset() + xrefEntryLen*int(header.numEntries))
	}
	reader.SkipWhiteSpaces()
	if !reader.ForwardTag("trailer") {
		return Dict{}, false
	}
	reader.SkipWhiteSpaces()
	return reader.ReadWithContext[Dict](ctx)
}

// getDecryptor is get_decryptor in hayro-syntax src/xref.rs.
func getDecryptor(trailer Dict, password []byte) (*decryptor, *xrefError) {
	encryption, ok := trailer.Get[Dict](KeyEncrypt)
	if !ok {
		return nil, nil
	}
	var id []byte
	if a, ok := trailer.Get[Array](KeyID); ok {
		if s, ok := a.FlexIter().Next[String](); ok {
			id = s
		}
	}
	d, err := getDecryptorFromDict(encryption, id, password)
	if err != nil {
		de := DecryptionInvalidEncryption
		errors.As(err, &de)
		return nil, &xrefError{decryption: de}
	}
	return d, nil
}

// ObjectStream is ObjectStream in hayro-syntax src/xref.rs. It holds the
// decoded data of an object stream and the offset of each member.
type ObjectStream struct {
	data    []byte
	offsets []objectStreamOffset
}

// objectStreamOffset is one entry of the offsets of ObjectStream in
// hayro-syntax src/xref.rs. ok is false for a member without a valid
// offset.
type objectStreamOffset struct {
	objNum uint32
	offset int
	ok     bool
}

// newObjectStream is ObjectStream::new in hayro-syntax src/xref.rs.
func newObjectStream(stream Stream) (*ObjectStream, bool) {
	data, err := stream.DecodedData()
	if err != nil {
		return nil, false
	}
	offsets, _ := readObjectStreamOffsets(stream, data)
	return &ObjectStream{data: data, offsets: offsets}, true
}

// readObjectStreamOffsets is ObjectStream::read_offsets in hayro-syntax
// src/xref.rs.
func readObjectStreamOffsets(stream Stream, data []byte) ([]objectStreamOffset, bool) {
	numObjects, ok := stream.Dict().Get[int](KeyN)
	if !ok {
		return nil, false
	}
	firstOffset, ok := stream.Dict().Get[int](KeyFirst)
	if !ok {
		return nil, false
	}
	r := NewReader(data)
	var offsets []objectStreamOffset
	lastOffset := -1
	for range numObjects {
		r.SkipWhiteSpacesAndComments()
		objNum, ok := r.ReadWithoutContext[uint32]()
		if !ok {
			return nil, false
		}
		r.SkipWhiteSpacesAndComments()
		relative, ok := r.ReadWithoutContext[int]()
		if !ok {
			return nil, false
		}
		// Offsets must increase, or members can overlap.
		if relative > math.MaxInt-firstOffset || firstOffset+relative >= len(data) || (lastOffset >= 0 && firstOffset+relative <= lastOffset) {
			offsets = append(offsets, objectStreamOffset{objNum: objNum})
			continue
		}
		member := NewReader(data)
		member.Jump(firstOffset + relative)
		member.SkipWhiteSpacesAndComments()
		lastOffset = member.Offset()
		offsets = append(offsets, objectStreamOffset{objNum: objNum, offset: member.Offset(), ok: true})
	}
	return offsets, true
}

// objNumber is ObjectStream::obj_number in hayro-syntax src/xref.rs.
func (s *ObjectStream) objNumber(index uint32) (uint32, bool) {
	if uint64(index) >= uint64(len(s.offsets)) {
		return 0, false
	}
	return s.offsets[index].objNum, true
}

// objectStreamGet is ObjectStream::get in hayro-syntax src/xref.rs. It reads
// the member at index as a T.
func objectStreamGet[T any](s *ObjectStream, index uint32, ctx *ReaderContext) (T, bool) {
	var zero T
	if uint64(index) >= uint64(len(s.offsets)) || !s.offsets[index].ok {
		return zero, false
	}
	r := NewReader(s.data)
	r.Jump(s.offsets[index].offset)
	ctx = ctx.clone()
	ctx.setInObjectStream(true)
	return r.ReadWithContext[T](ctx)
}

// parseMetadata is parse_metadata in hayro-syntax src/xref.rs.
func parseMetadata(info Dict) Metadata {
	var m Metadata
	date := func(key Name) *DateTime {
		s, ok := info.Get[String](key)
		if !ok {
			return nil
		}
		d, ok := DateTimeFromBytes(s)
		if !ok {
			return nil
		}
		return &d
	}
	text := func(key Name) []byte {
		s, ok := info.Get[String](key)
		if !ok {
			return nil
		}
		return append([]byte{}, s...)
	}
	m.CreationDate = date(KeyCreationDate)
	m.ModificationDate = date(KeyModDate)
	m.Title = text(KeyTitle)
	m.Author = text(KeyAuthor)
	m.Subject = text(KeySubject)
	m.Keywords = text(KeyKeywords)
	m.Creator = text(KeyCreator)
	m.Producer = text(KeyProducer)
	return m
}
