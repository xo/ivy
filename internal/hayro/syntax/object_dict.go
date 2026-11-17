// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/dict.rs.

package syntax

import (
	"bytes"
	"slices"
	"sort"
	"strings"
)

// Dict is Dict in hayro-syntax src/object/dict.rs. It is a PDF dictionary.
// It holds the raw bytes and the offset of each value, and it reads a value
// when a caller asks for it.
//
// Port: the case Empty of upstream is a nil repr.
type Dict struct {
	repr *dictRepr
}

// dictRepr is Repr in hayro-syntax src/object/dict.rs.
type dictRepr struct {
	data    []byte
	offsets []dictOffset
	ctx     ReaderContext
}

// dictOffset is one entry of the offsets of Repr in hayro-syntax
// src/object/dict.rs.
type dictOffset struct {
	name   Name
	offset int
}

// maxUnsortedLen is MAX_UNSORTED_LEN in hayro-syntax src/object/dict.rs.
const maxUnsortedLen = 16

// EmptyDict is Dict::empty in hayro-syntax src/object/dict.rs.
func EmptyDict() Dict {
	return Dict{}
}

// Equal reports whether two dictionaries hold the same raw bytes, as the
// PartialEq of upstream does. White space matters.
func (d Dict) Equal(other Dict) bool {
	return bytes.Equal(d.Data(), other.Data())
}

// Data is Dict::data in hayro-syntax src/object/dict.rs. It returns the raw
// bytes of the dictionary.
func (d Dict) Data() []byte {
	if d.repr == nil {
		return nil
	}
	return d.repr.data
}

// Len is Dict::len in hayro-syntax src/object/dict.rs. It returns the number
// of entries.
func (d Dict) Len() int {
	return len(d.offsets())
}

// IsEmpty is Dict::is_empty in hayro-syntax src/object/dict.rs.
func (d Dict) IsEmpty() bool {
	return len(d.offsets()) == 0
}

// ContainsKey is Dict::contains_key in hayro-syntax src/object/dict.rs.
func (d Dict) ContainsKey(key Name) bool {
	_, ok := d.offset(key)
	return ok
}

// Get is Dict::get in hayro-syntax src/object/dict.rs. It returns the value
// of a key as a T, and it resolves a reference. T is one of the types that
// Reader.Read lists.
func (d Dict) Get[T any](key Name) (T, bool) {
	m, ok := d.GetRaw[T](key)
	if !ok {
		var zero T
		return zero, false
	}
	return m.Resolve(d.ctx())
}

// GetRef is Dict::get_ref in hayro-syntax src/object/dict.rs. It returns the
// reference that a key holds.
func (d Dict) GetRef(key Name) (ObjRef, bool) {
	offset, ok := d.offset(key)
	if !ok {
		return ObjRef{}, false
	}
	return NewReader(d.Data()[offset:]).ReadWithContext[ObjRef](d.ctx())
}

// Keys is Dict::keys in hayro-syntax src/object/dict.rs. It returns the keys
// in no particular order.
func (d Dict) Keys() []Name {
	offsets := d.offsets()
	keys := make([]Name, len(offsets))
	for i, o := range offsets {
		keys[i] = o.name
	}
	return keys
}

// DictEntry is one item of Dict::entries in hayro-syntax
// src/object/dict.rs.
type DictEntry struct {
	// Key is the key of the entry.
	Key Name
	// Value is the value of the entry.
	Value MaybeRef[Object]
}

// Entries is Dict::entries in hayro-syntax src/object/dict.rs. It returns
// the entries sorted by key. An entry whose value cannot be read is left
// out.
func (d Dict) Entries() []DictEntry {
	keys := d.Keys()
	slices.Sort(keys)
	keys = slices.Compact(keys)
	entries := make([]DictEntry, 0, len(keys))
	for _, k := range keys {
		if v, ok := d.GetRaw[Object](k); ok {
			entries = append(entries, DictEntry{Key: k, Value: v})
		}
	}
	return entries
}

// ObjID is Dict::obj_id in hayro-syntax src/object/dict.rs. It returns the
// identifier of the dictionary when it is an indirect object.
func (d Dict) ObjID() (ObjectIdentifier, bool) {
	return d.ctx().objNumberCtx()
}

// GetRaw is Dict::get_raw in hayro-syntax src/object/dict.rs. It returns the
// raw entry of a key, without resolving a reference.
func (d Dict) GetRaw[T any](key Name) (MaybeRef[T], bool) {
	offset, ok := d.offset(key)
	if !ok {
		return MaybeRef[T]{}, false
	}
	return NewReader(d.Data()[offset:]).ReadWithContext[MaybeRef[T]](d.ctx())
}

// ctx is Dict::ctx in hayro-syntax src/object/dict.rs.
func (d Dict) ctx() *ReaderContext {
	if d.repr == nil {
		return dummyRefContext()
	}
	return &d.repr.ctx
}

// offsets is Dict::offsets in hayro-syntax src/object/dict.rs.
func (d Dict) offsets() []dictOffset {
	if d.repr == nil {
		return nil
	}
	return d.repr.offsets
}

// offset is Dict::offset in hayro-syntax src/object/dict.rs.
func (d Dict) offset(key Name) (int, bool) {
	offsets := d.offsets()
	if len(offsets) <= maxUnsortedLen {
		for _, o := range offsets {
			if o.name == key {
				return o.offset, true
			}
		}
		return 0, false
	}
	idx, found := sort.Find(len(offsets), func(i int) int {
		return strings.Compare(string(key), string(offsets[i].name))
	})
	if !found {
		return 0, false
	}
	return offsets[idx].offset, true
}

// GoString returns the Debug text of upstream.
func (d Dict) GoString() string {
	entries := d.Entries()
	if len(entries) == 0 {
		return "Dict"
	}
	var b strings.Builder
	b.WriteString("Dict { ")
	for i, e := range entries {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(rustDebugStr(e.Key.AsStr()))
		b.WriteString(": ")
		b.WriteString(e.Value.GoString())
	}
	b.WriteString(" }")
	return b.String()
}

// String returns the Display text of upstream.
func (d Dict) String() string {
	var b strings.Builder
	b.WriteString("<<")
	for _, e := range d.Entries() {
		b.WriteString(" ")
		b.WriteString(e.Key.String())
		b.WriteString(" ")
		b.WriteString(e.Value.String())
	}
	if !d.IsEmpty() {
		b.WriteString(" ")
	}
	b.WriteString(">>")
	return b.String()
}

// skipDict is the Skippable implementation of Dict in hayro-syntax
// src/object/dict.rs.
func skipDict(r *Reader, isContentStream bool, depth int) bool {
	if depth >= maxObjectNestingDepth {
		return false
	}
	if !r.ForwardTag("<<") {
		return false
	}
	for {
		r.SkipWhiteSpacesAndComments()
		if r.ForwardTag(">>") {
			return true
		}
		if _, ok := r.Skip[Name](isContentStream); !ok {
			// Lenient: skip garbage between the entries.
			if _, ok := r.SkipWithDepth[Object](isContentStream, depth+1); !ok {
				return false
			}
			continue
		}
		r.SkipWhiteSpacesAndComments()
		var ok bool
		if isContentStream {
			_, ok = r.SkipWithDepth[Object](isContentStream, depth+1)
		} else {
			_, ok = r.SkipWithDepth[MaybeRef[Object]](isContentStream, depth+1)
		}
		if !ok {
			return false
		}
	}
}

// readDict is the Readable implementation of Dict in hayro-syntax
// src/object/dict.rs.
func readDict(r *Reader, ctx *ReaderContext) (Dict, bool) {
	return readInnerDict(r, ctx, "<<", ">>")
}

// readInnerDict is read_inner in hayro-syntax src/object/dict.rs. An empty
// startTag reads a dictionary without delimiters, as an inline image holds
// it.
func readInnerDict(r *Reader, ctx *ReaderContext, startTag, endTag string) (Dict, bool) {
	offsets := make([]dictOffset, 0, 8)
	data, ok := parseDictWith(r, ctx, startTag, endTag, func(name Name, offset int) {
		offsets = append(offsets, dictOffset{name: name, offset: offset})
	})
	if !ok {
		return Dict{}, false
	}
	if len(offsets) > maxUnsortedLen {
		sort.SliceStable(offsets, func(i, j int) bool { return offsets[i].name < offsets[j].name })
		offsets = slices.CompactFunc(offsets, func(a, b dictOffset) bool { return a.name == b.name })
	}
	return Dict{repr: &dictRepr{data: data, offsets: offsets, ctx: *ctx}}, true
}

// dictProbe is DictProbe in hayro-syntax src/object/dict.rs.
type dictProbe struct {
	data    []byte
	hasRoot bool
	hasType bool
}

// probeDict is probe_dict in hayro-syntax src/object/dict.rs. It finds the
// end of a dictionary and whether it holds the keys Root and Type, without
// building the dictionary.
func probeDict(r *Reader, ctx *ReaderContext, startTag, endTag string) (dictProbe, bool) {
	var probe dictProbe
	data, ok := parseDictWith(r, ctx, startTag, endTag, func(name Name, _ int) {
		switch name {
		case KeyRoot:
			probe.hasRoot = true
		case KeyType:
			probe.hasType = true
		}
	})
	if !ok {
		return dictProbe{}, false
	}
	probe.data = data
	return probe, true
}

// parseDictWith is parse_dict_with in hayro-syntax src/object/dict.rs. It
// calls onEntry with the key and the offset of the value of each entry.
func parseDictWith(r *Reader, ctx *ReaderContext, startTag, endTag string, onEntry func(Name, int)) ([]byte, bool) {
	depth := 0
	if startTag != "" {
		depth = 1
	}
	dictData, ok := r.Tail()
	if !ok {
		return nil, false
	}
	startOffset := r.Offset()
	if startTag != "" && !r.ForwardTag(startTag) {
		return nil, false
	}
	for {
		r.SkipWhiteSpacesAndComments()
		if r.PeekTag(endTag) {
			r.ForwardTag(endTag)
			return dictData[:r.Offset()-startOffset], true
		}
		name, ok := r.ReadWithoutContext[Name]()
		if !ok {
			if startTag == "" {
				return nil, false
			}
			// Lenient: skip garbage between the entries, but not in the
			// dictionary of an inline image.
			if _, ok := r.SkipWithDepth[Object](ctx.InContentStream(), depth); !ok {
				return nil, false
			}
			continue
		}
		r.SkipWhiteSpacesAndComments()
		// Null entries stay. A test for them resolves references before
		// the decryption is set up.
		onEntry(name, r.Offset()-startOffset)
		if ctx.InContentStream() {
			_, ok = r.SkipWithDepth[Object](true, depth)
		} else {
			_, ok = r.SkipWithDepth[MaybeRef[Object]](false, depth)
		}
		if !ok {
			return nil, false
		}
	}
}

// InlineImageDict is InlineImageDict in hayro-syntax src/object/dict.rs. It
// is the dictionary of an inline image, which ends with ID.
type InlineImageDict struct {
	dict Dict
}

// GetDict is InlineImageDict::get_dict in hayro-syntax src/object/dict.rs.
func (d InlineImageDict) GetDict() Dict {
	return d.dict
}

// readFrom is the Readable implementation of InlineImageDict in hayro-syntax
// src/object/dict.rs.
func (d *InlineImageDict) readFrom(r *Reader, ctx *ReaderContext) bool {
	dict, ok := readInnerDict(r, ctx, "", "ID")
	if !ok {
		return false
	}
	d.dict = dict
	return true
}

// The keys are the module keys in hayro-syntax src/object/dict.rs, which
// upstream copied and adapted from PDFBox. Each Go name is Key and the value
// of the key. When the value is not a Go name, the Go name is Key and the
// Rust name in MixedCaps. The comment after each key holds the Rust name.
const (
	KeyA                         = "A"                         // A
	KeyAA                        = "AA"                        // AA
	KeyAbsoluteColorimetric      = "AbsoluteColorimetric"      // ABSOLUTE_COLORIMETRIC
	KeyAC                        = "AC"                        // AC
	KeyAcroForm                  = "AcroForm"                  // ACRO_FORM
	KeyActualText                = "ActualText"                // ACTUAL_TEXT
	KeyADBE                      = "ADBE"                      // ADBE
	KeyAdbePkcs7Detached         = "adbe.pkcs7.detached"       // ADBE_PKCS7_DETACHED
	KeyAdbePkcs7Sha1             = "adbe.pkcs7.sha1"           // ADBE_PKCS7_SHA1
	KeyAdbeX509RsaSha1           = "adbe.x509.rsa_sha1"        // ADBE_X509_RSA_SHA1
	KeyAdobePpklite              = "Adobe.PPKLite"             // ADOBE_PPKLITE
	KeyAESV2                     = "AESV2"                     // AESV2
	KeyAESV3                     = "AESV3"                     // AESV3
	KeyAF                        = "AF"                        // AF
	KeyAFRelationship            = "AFRelationship"            // AF_RELATIONSHIP
	KeyAfter                     = "After"                     // AFTER
	KeyAIMetaData                = "AIMetaData"                // AI_META_DATA
	KeyAIS                       = "AIS"                       // AIS
	KeyAllOff                    = "AllOff"                    // ALL_OFF
	KeyAllOn                     = "AllOn"                     // ALL_ON
	KeyAlt                       = "Alt"                       // ALT
	KeyAlpha                     = "Alpha"                     // ALPHA
	KeyAlternate                 = "Alternate"                 // ALTERNATE
	KeyAnnot                     = "Annot"                     // ANNOT
	KeyAnnots                    = "Annots"                    // ANNOTS
	KeyAntiAlias                 = "AntiAlias"                 // ANTI_ALIAS
	KeyAnyOff                    = "AnyOff"                    // ANY_OFF
	KeyAnyOn                     = "AnyOn"                     // ANY_ON
	KeyAP                        = "AP"                        // AP
	KeyAPRef                     = "APRef"                     // AP_REF
	KeyApp                       = "App"                       // APP
	KeyArtBox                    = "ArtBox"                    // ART_BOX
	KeyArtifact                  = "Artifact"                  // ARTIFACT
	KeyAS                        = "AS"                        // AS
	KeyAscent                    = "Ascent"                    // ASCENT
	KeyASCIIHexDecode            = "ASCIIHexDecode"            // ASCII_HEX_DECODE
	KeyAHx                       = "AHx"                       // ASCII_HEX_DECODE_ABBREVIATION
	KeyASCII85Decode             = "ASCII85Decode"             // ASCII85_DECODE
	KeyA85                       = "A85"                       // ASCII85_DECODE_ABBREVIATION
	KeyAttached                  = "Attached"                  // ATTACHED
	KeyAuthor                    = "Author"                    // AUTHOR
	KeyAvgWidth                  = "AvgWidth"                  // AVG_WIDTH
	KeyB                         = "B"                         // B
	KeyBackground                = "Background"                // BACKGROUND
	KeyBaseEncoding              = "BaseEncoding"              // BASE_ENCODING
	KeyBaseFont                  = "BaseFont"                  // BASE_FONT
	KeyBaseState                 = "BaseState"                 // BASE_STATE
	KeyBaseVersion               = "BaseVersion"               // BASE_VERSION
	KeyBBox                      = "BBox"                      // BBOX
	KeyBC                        = "BC"                        // BC
	KeyBE                        = "BE"                        // BE
	KeyBEAD                      = "BEAD"                      // BEAD
	KeyBefore                    = "Before"                    // BEFORE
	KeyBG                        = "BG"                        // BG
	KeyBitsPerComponent          = "BitsPerComponent"          // BITS_PER_COMPONENT
	KeyBitsPerCoordinate         = "BitsPerCoordinate"         // BITS_PER_COORDINATE
	KeyBitsPerFlag               = "BitsPerFlag"               // BITS_PER_FLAG
	KeyBitsPerSample             = "BitsPerSample"             // BITS_PER_SAMPLE
	KeyBl                        = "Bl"                        // BL
	KeyBlackIs1                  = "BlackIs1"                  // BLACK_IS_1
	KeyBlackPoint                = "BlackPoint"                // BLACK_POINT
	KeyBleedBox                  = "BleedBox"                  // BLEED_BOX
	KeyBM                        = "BM"                        // BM
	KeyBorder                    = "Border"                    // BORDER
	KeyBounds                    = "Bounds"                    // BOUNDS
	KeyBPC                       = "BPC"                       // BPC
	KeyBS                        = "BS"                        // BS
	KeyBtn                       = "Btn"                       // BTN
	KeyByteRange                 = "ByteRange"                 // BYTERANGE
	KeyC                         = "C"                         // C
	KeyC0                        = "C0"                        // C0
	KeyC1                        = "C1"                        // C1
	KeyCA                        = "CA"                        // CA
	KeyCaNs                      = "ca"                        // CA_NS
	KeyCalGray                   = "CalGray"                   // CALGRAY
	KeyCalRGB                    = "CalRGB"                    // CALRGB
	KeyCalCMYK                   = "CalCMYK"                   // CALCMYK
	KeyCap                       = "Cap"                       // CAP
	KeyCapHeight                 = "CapHeight"                 // CAP_HEIGHT
	KeyCatalog                   = "Catalog"                   // CATALOG
	KeyCCITTFaxDecode            = "CCITTFaxDecode"            // CCITTFAX_DECODE
	KeyCCF                       = "CCF"                       // CCITTFAX_DECODE_ABBREVIATION
	KeyCenterWindow              = "CenterWindow"              // CENTER_WINDOW
	KeyCert                      = "Cert"                      // CERT
	KeyCerts                     = "Certs"                     // CERTS
	KeyCF                        = "CF"                        // CF
	KeyCFM                       = "CFM"                       // CFM
	KeyCh                        = "Ch"                        // CH
	KeyCharProcs                 = "CharProcs"                 // CHAR_PROCS
	KeyCharSet                   = "CharSet"                   // CHAR_SET
	KeyCheckSum                  = "CheckSum"                  // CHECK_SUM
	KeyCI                        = "CI"                        // CI
	KeyCiciSignit                = "CICI.SignIt"               // CICI_SIGNIT
	KeyCIDFontType0              = "CIDFontType0"              // CID_FONT_TYPE0
	KeyCIDFontType0C             = "CIDFontType0C"             // CID_FONT_TYPE0C
	KeyCIDFontType2              = "CIDFontType2"              // CID_FONT_TYPE2
	KeyCIDToGIDMap               = "CIDToGIDMap"               // CID_TO_GID_MAP
	KeyCIDSet                    = "CIDSet"                    // CID_SET
	KeyCIDSystemInfo             = "CIDSystemInfo"             // CIDSYSTEMINFO
	KeyCL                        = "CL"                        // CL
	KeyClassMap                  = "ClassMap"                  // CLASS_MAP
	KeyClrF                      = "ClrF"                      // CLR_F
	KeyClrFf                     = "ClrFf"                     // CLR_FF
	KeyCMap                      = "CMap"                      // CMAP
	KeyCMapName                  = "CMapName"                  // CMAPNAME
	KeyCMYK                      = "CMYK"                      // CMYK
	KeyCO                        = "CO"                        // CO
	KeyColor                     = "Color"                     // COLOR
	KeyCollection                = "Collection"                // COLLECTION
	KeyCollectionItem            = "CollectionItem"            // COLLECTION_ITEM
	KeyCollectionField           = "CollectionField"           // COLLECTION_FIELD
	KeyCollectionSchema          = "CollectionSchema"          // COLLECTION_SCHEMA
	KeyCollectionSort            = "CollectionSort"            // COLLECTION_SORT
	KeyCollectionSubitem         = "CollectionSubitem"         // COLLECTION_SUBITEM
	KeyColorBurn                 = "ColorBurn"                 // COLOR_BURN
	KeyColorDodge                = "ColorDodge"                // COLOR_DODGE
	KeyColorants                 = "Colorants"                 // COLORANTS
	KeyColors                    = "Colors"                    // COLORS
	KeyColorSpace                = "ColorSpace"                // COLORSPACE
	KeyColorTransform            = "ColorTransform"            // COLOR_TRANSFORM
	KeyColumns                   = "Columns"                   // COLUMNS
	KeyCompatible                = "Compatible"                // COMPATIBLE
	KeyComponents                = "Components"                // COMPONENTS
	KeyContactInfo               = "ContactInfo"               // CONTACT_INFO
	KeyContents                  = "Contents"                  // CONTENTS
	KeyCoords                    = "Coords"                    // COORDS
	KeyCount                     = "Count"                     // COUNT
	KeyCP                        = "CP"                        // CP
	KeyCreationDate              = "CreationDate"              // CREATION_DATE
	KeyCreator                   = "Creator"                   // CREATOR
	KeyCRL                       = "CRL"                       // CRL
	KeyCRLS                      = "CRLS"                      // CRLS
	KeyCropBox                   = "CropBox"                   // CROP_BOX
	KeyCrypt                     = "Crypt"                     // CRYPT
	KeyCS                        = "CS"                        // CS
	KeyCYX                       = "CYX"                       // CYX
	KeyD                         = "D"                         // D
	KeyDA                        = "DA"                        // DA
	KeyDarken                    = "Darken"                    // DARKEN
	KeyDate                      = "Date"                      // DATE
	KeyDCTDecode                 = "DCTDecode"                 // DCT_DECODE
	KeyDCT                       = "DCT"                       // DCT_DECODE_ABBREVIATION
	KeyDecode                    = "Decode"                    // DECODE
	KeyDecodeParms               = "DecodeParms"               // DECODE_PARMS
	KeyDefault                   = "default"                   // DEFAULT
	KeyDefaultCMYK               = "DefaultCMYK"               // DEFAULT_CMYK
	KeyDefaultCryptFilter        = "DefaultCryptFilter"        // DEFAULT_CRYPT_FILTER
	KeyDefaultGray               = "DefaultGray"               // DEFAULT_GRAY
	KeyDefaultRGB                = "DefaultRGB"                // DEFAULT_RGB
	KeyDesc                      = "Desc"                      // DESC
	KeyDescendantFonts           = "DescendantFonts"           // DESCENDANT_FONTS
	KeyDescent                   = "Descent"                   // DESCENT
	KeyDest                      = "Dest"                      // DEST
	KeyDestOutputProfile         = "DestOutputProfile"         // DEST_OUTPUT_PROFILE
	KeyDests                     = "Dests"                     // DESTS
	KeyDeviceCMYK                = "DeviceCMYK"                // DEVICE_CMYK
	KeyDeviceGray                = "DeviceGray"                // DEVICE_GRAY
	KeyDeviceN                   = "DeviceN"                   // DEVICE_N
	KeyDeviceRGB                 = "DeviceRGB"                 // DEVICE_RGB
	KeyDi                        = "Di"                        // DI
	KeyDifference                = "Difference"                // DIFFERENCE
	KeyDifferences               = "Differences"               // DIFFERENCES
	KeyDigestMethod              = "DigestMethod"              // DIGEST_METHOD
	KeyRIPEMD160                 = "RIPEMD160"                 // DIGEST_RIPEMD160
	KeySHA1                      = "SHA1"                      // DIGEST_SHA1
	KeySHA256                    = "SHA256"                    // DIGEST_SHA256
	KeySHA384                    = "SHA384"                    // DIGEST_SHA384
	KeySHA512                    = "SHA512"                    // DIGEST_SHA512
	KeyDirection                 = "Direction"                 // DIRECTION
	KeyDisplayDocTitle           = "DisplayDocTitle"           // DISPLAY_DOC_TITLE
	KeyDL                        = "DL"                        // DL
	KeyDm                        = "Dm"                        // DM
	KeyDoc                       = "Doc"                       // DOC
	KeyDocChecksum               = "DocChecksum"               // DOC_CHECKSUM
	KeyDocTimeStamp              = "DocTimeStamp"              // DOC_TIME_STAMP
	KeyDocMDP                    = "DocMDP"                    // DOCMDP
	KeyDocument                  = "Document"                  // DOCUMENT
	KeyDomain                    = "Domain"                    // DOMAIN
	KeyDOS                       = "DOS"                       // DOS
	KeyDP                        = "DP"                        // DP
	KeyDR                        = "DR"                        // DR
	KeyDS                        = "DS"                        // DS
	KeyDSS                       = "DSS"                       // DSS
	KeyDuplex                    = "Duplex"                    // DUPLEX
	KeyDur                       = "Dur"                       // DUR
	KeyDV                        = "DV"                        // DV
	KeyDW                        = "DW"                        // DW
	KeyDW2                       = "DW2"                       // DW2
	KeyE                         = "E"                         // E
	KeyEarlyChange               = "EarlyChange"               // EARLY_CHANGE
	KeyEF                        = "EF"                        // EF
	KeyEmbeddedFDFs              = "EmbeddedFDFs"              // EMBEDDED_FDFS
	KeyEmbeddedFile              = "EmbeddedFile"              // EMBEDDED_FILE
	KeyEmbeddedFiles             = "EmbeddedFiles"             // EMBEDDED_FILES
	KeyEmpty                     = ""                          // EMPTY
	KeyEncode                    = "Encode"                    // ENCODE
	KeyEncodedByteAlign          = "EncodedByteAlign"          // ENCODED_BYTE_ALIGN
	KeyEncoding                  = "Encoding"                  // ENCODING
	KeyEncoding90msRksjH         = "90ms-RKSJ-H"               // ENCODING_90MS_RKSJ_H
	KeyEncoding90msRksjV         = "90ms-RKSJ-V"               // ENCODING_90MS_RKSJ_V
	KeyEncodingEtenB5H           = "ETen-B5-H"                 // ENCODING_ETEN_B5_H
	KeyEncodingEtenB5V           = "ETen-B5-V"                 // ENCODING_ETEN_B5_V
	KeyEncrypt                   = "Encrypt"                   // ENCRYPT
	KeyEncryptMetadata           = "EncryptMetadata"           // ENCRYPT_META_DATA
	KeyEncryptedPayload          = "EncryptedPayload"          // ENCRYPTED_PAYLOAD
	KeyEndOfBlock                = "EndOfBlock"                // END_OF_BLOCK
	KeyEndOfLine                 = "EndOfLine"                 // END_OF_LINE
	KeyEntrustPpkef              = "Entrust.PPKEF"             // ENTRUST_PPKEF
	KeyExclusion                 = "Exclusion"                 // EXCLUSION
	KeyExtensions                = "Extensions"                // EXTENSIONS
	KeyExtensionLevel            = "ExtensionLevel"            // EXTENSION_LEVEL
	KeyExData                    = "ExData"                    // EX_DATA
	KeyExport                    = "Export"                    // EXPORT
	KeyExportState               = "ExportState"               // EXPORT_STATE
	KeyExtGState                 = "ExtGState"                 // EXT_G_STATE
	KeyExtend                    = "Extend"                    // EXTEND
	KeyExtends                   = "Extends"                   // EXTENDS
	KeyF                         = "F"                         // F
	KeyFDecodeParms              = "FDecodeParms"              // F_DECODE_PARMS
	KeyFFilter                   = "FFilter"                   // F_FILTER
	KeyFB                        = "FB"                        // FB
	KeyFDF                       = "FDF"                       // FDF
	KeyFf                        = "Ff"                        // FF
	KeyFields                    = "Fields"                    // FIELDS
	KeyFilespec                  = "Filespec"                  // FILESPEC
	KeyFilter                    = "Filter"                    // FILTER
	KeyFirst                     = "First"                     // FIRST
	KeyFirstChar                 = "FirstChar"                 // FIRST_CHAR
	KeyFitWindow                 = "FitWindow"                 // FIT_WINDOW
	KeyFL                        = "FL"                        // FL
	KeyFlags                     = "Flags"                     // FLAGS
	KeyFlateDecode               = "FlateDecode"               // FLATE_DECODE
	KeyFl                        = "Fl"                        // FLATE_DECODE_ABBREVIATION
	KeyFo                        = "Fo"                        // FO
	KeyFolders                   = "Folders"                   // FOLDERS
	KeyFont                      = "Font"                      // FONT
	KeyFontBBox                  = "FontBBox"                  // FONT_BBOX
	KeyFontDescriptor            = "FontDescriptor"            // FONT_DESC
	KeyFontFamily                = "FontFamily"                // FONT_FAMILY
	KeyFontFile                  = "FontFile"                  // FONT_FILE
	KeyFontFile2                 = "FontFile2"                 // FONT_FILE2
	KeyFontFile3                 = "FontFile3"                 // FONT_FILE3
	KeyFontMatrix                = "FontMatrix"                // FONT_MATRIX
	KeyFontName                  = "FontName"                  // FONT_NAME
	KeyFontStretch               = "FontStretch"               // FONT_STRETCH
	KeyFontWeight                = "FontWeight"                // FONT_WEIGHT
	KeyForm                      = "Form"                      // FORM
	KeyFormType                  = "FormType"                  // FORMTYPE
	KeyFRM                       = "FRM"                       // FRM
	KeyFS                        = "FS"                        // FS
	KeyFT                        = "FT"                        // FT
	KeyFunction                  = "Function"                  // FUNCTION
	KeyFunctionType              = "FunctionType"              // FUNCTION_TYPE
	KeyFunctions                 = "Functions"                 // FUNCTIONS
	KeyG                         = "G"                         // G
	KeyGamma                     = "Gamma"                     // GAMMA
	KeyGroup                     = "Group"                     // GROUP
	KeyGtsPdfa1                  = "GTS_PDFA1"                 // GTS_PDFA1
	KeyH                         = "H"                         // H
	KeyHardLight                 = "HardLight"                 // HARD_LIGHT
	KeyHeight                    = "Height"                    // HEIGHT
	KeyHelv                      = "Helv"                      // HELV
	KeyHideMenubar               = "HideMenubar"               // HIDE_MENUBAR
	KeyHideToolbar               = "HideToolbar"               // HIDE_TOOLBAR
	KeyHideWindowUI              = "HideWindowUI"              // HIDE_WINDOWUI
	KeyHue                       = "Hue"                       // HUE
	KeyI                         = "I"                         // I
	KeyIC                        = "IC"                        // IC
	KeyICCBased                  = "ICCBased"                  // ICC_BASED
	KeyID                        = "ID"                        // ID
	KeyIDTree                    = "IDTree"                    // ID_TREE
	KeyIdentity                  = "Identity"                  // IDENTITY
	KeyIdentityH                 = "Identity-H"                // IDENTITY_H
	KeyIdentityV                 = "Identity-V"                // IDENTITY_V
	KeyIF                        = "IF"                        // IF
	KeyIllustrator               = "Illustrator"               // ILLUSTRATOR
	KeyIM                        = "IM"                        // IM
	KeyImage                     = "Image"                     // IMAGE
	KeyImageMask                 = "ImageMask"                 // IMAGE_MASK
	KeyIndex                     = "Index"                     // INDEX
	KeyIndexed                   = "Indexed"                   // INDEXED
	KeyInfo                      = "Info"                      // INFO
	KeyInkList                   = "InkList"                   // INKLIST
	KeyIntent                    = "Intent"                    // INTENT
	KeyInterpolate               = "Interpolate"               // INTERPOLATE
	KeyIRT                       = "IRT"                       // IRT
	KeyIT                        = "IT"                        // IT
	KeyItalicAngle               = "ItalicAngle"               // ITALIC_ANGLE
	KeyIssuer                    = "Issuer"                    // ISSUER
	KeyIX                        = "IX"                        // IX
	KeyJavaScript                = "JavaScript"                // JAVA_SCRIPT
	KeyJBIG2Decode               = "JBIG2Decode"               // JBIG2_DECODE
	KeyJBIG2Globals              = "JBIG2Globals"              // JBIG2_GLOBALS
	KeyJPXDecode                 = "JPXDecode"                 // JPX_DECODE
	KeyJS                        = "JS"                        // JS
	KeyK                         = "K"                         // K
	KeyKeywords                  = "Keywords"                  // KEYWORDS
	KeyKeyUsage                  = "KeyUsage"                  // KEY_USAGE
	KeyKids                      = "Kids"                      // KIDS
	KeyL                         = "L"                         // L
	KeyLab                       = "Lab"                       // LAB
	KeyLang                      = "Lang"                      // LANG
	KeyLast                      = "Last"                      // LAST
	KeyLastChar                  = "LastChar"                  // LAST_CHAR
	KeyLastModified              = "LastModified"              // LAST_MODIFIED
	KeyLC                        = "LC"                        // LC
	KeyLE                        = "LE"                        // LE
	KeyLeading                   = "Leading"                   // LEADING
	KeyLegalAttestation          = "LegalAttestation"          // LEGAL_ATTESTATION
	KeyLength                    = "Length"                    // LENGTH
	KeyLength1                   = "Length1"                   // LENGTH1
	KeyLength2                   = "Length2"                   // LENGTH2
	KeyLength3                   = "Length3"                   // LENGTH3
	KeyLighten                   = "Lighten"                   // LIGHTEN
	KeyLimits                    = "Limits"                    // LIMITS
	KeyLinearized                = "Linearized"                // LINEARIZED
	KeyLJ                        = "LJ"                        // LJ
	KeyLL                        = "LL"                        // LL
	KeyLLE                       = "LLE"                       // LLE
	KeyLLO                       = "LLO"                       // LLO
	KeyLocation                  = "Location"                  // LOCATION
	KeyLuminosity                = "Luminosity"                // LUMINOSITY
	KeyLW                        = "LW"                        // LW
	KeyLZWDecode                 = "LZWDecode"                 // LZW_DECODE
	KeyLZW                       = "LZW"                       // LZW_DECODE_ABBREVIATION
	KeyM                         = "M"                         // M
	KeyMac                       = "Mac"                       // MAC
	KeyMacExpertEncoding         = "MacExpertEncoding"         // MAC_EXPERT_ENCODING
	KeyMacRomanEncoding          = "MacRomanEncoding"          // MAC_ROMAN_ENCODING
	KeyMarkInfo                  = "MarkInfo"                  // MARK_INFO
	KeyMask                      = "Mask"                      // MASK
	KeyMatrix                    = "Matrix"                    // MATRIX
	KeyMatte                     = "Matte"                     // MATTE
	KeyMaxLen                    = "MaxLen"                    // MAX_LEN
	KeyMaxWidth                  = "MaxWidth"                  // MAX_WIDTH
	KeyMCID                      = "MCID"                      // MCID
	KeyMDP                       = "MDP"                       // MDP
	KeyMediaBox                  = "MediaBox"                  // MEDIA_BOX
	KeyMeasure                   = "Measure"                   // MEASURE
	KeyMetadata                  = "Metadata"                  // METADATA
	KeyMissingWidth              = "MissingWidth"              // MISSING_WIDTH
	KeyMix                       = "Mix"                       // MIX
	KeyMK                        = "MK"                        // MK
	KeyML                        = "ML"                        // ML
	KeyMMType1                   = "MMType1"                   // MM_TYPE1
	KeyModDate                   = "ModDate"                   // MOD_DATE
	KeyMultiply                  = "Multiply"                  // MULTIPLY
	KeyN                         = "N"                         // N
	KeyName                      = "Name"                      // NAME
	KeyNames                     = "Names"                     // NAMES
	KeyNavigator                 = "Navigator"                 // NAVIGATOR
	KeyNeedAppearances           = "NeedAppearances"           // NEED_APPEARANCES
	KeyNewWindow                 = "NewWindow"                 // NEW_WINDOW
	KeyNext                      = "Next"                      // NEXT
	KeyNM                        = "NM"                        // NM
	KeyNonEFontNoWarn            = "NonEFontNoWarn"            // NON_EFONT_NO_WARN
	KeyNonFullScreenPageMode     = "NonFullScreenPageMode"     // NON_FULL_SCREEN_PAGE_MODE
	KeyNone                      = "None"                      // NONE
	KeyNormal                    = "Normal"                    // NORMAL
	KeyNums                      = "Nums"                      // NUMS
	KeyO                         = "O"                         // O
	KeyObj                       = "Obj"                       // OBJ
	KeyOBJR                      = "OBJR"                      // OBJR
	KeyObjStm                    = "ObjStm"                    // OBJ_STM
	KeyOC                        = "OC"                        // OC
	KeyOCG                       = "OCG"                       // OCG
	KeyOCGs                      = "OCGs"                      // OCGS
	KeyOCMD                      = "OCMD"                      // OCMD
	KeyOCProperties              = "OCProperties"              // OCPROPERTIES
	KeyOCSP                      = "OCSP"                      // OCSP
	KeyOCSPs                     = "OCSPs"                     // OCSPS
	KeyOE                        = "OE"                        // OE
	KeyOID                       = "OID"                       // OID
	KeyOFF                       = "OFF"                       // OFF
	KeyON                        = "ON"                        // ON
	KeyOP                        = "OP"                        // OP
	KeyOpNs                      = "op"                        // OP_NS
	KeyOpenAction                = "OpenAction"                // OPEN_ACTION
	KeyOpenType                  = "OpenType"                  // OPEN_TYPE
	KeyOPI                       = "OPI"                       // OPI
	KeyOPM                       = "OPM"                       // OPM
	KeyOpt                       = "Opt"                       // OPT
	KeyOrder                     = "Order"                     // ORDER
	KeyOrdering                  = "Ordering"                  // ORDERING
	KeyOS                        = "OS"                        // OS
	KeyOutlines                  = "Outlines"                  // OUTLINES
	KeyOutputCondition           = "OutputCondition"           // OUTPUT_CONDITION
	KeyOutputConditionIdentifier = "OutputConditionIdentifier" // OUTPUT_CONDITION_IDENTIFIER
	KeyOutputIntent              = "OutputIntent"              // OUTPUT_INTENT
	KeyOutputIntents             = "OutputIntents"             // OUTPUT_INTENTS
	KeyOverlay                   = "Overlay"                   // OVERLAY
	KeyP                         = "P"                         // P
	KeyPA                        = "PA"                        // PA
	KeyPage                      = "Page"                      // PAGE
	KeyPageLabels                = "PageLabels"                // PAGE_LABELS
	KeyPageLayout                = "PageLayout"                // PAGE_LAYOUT
	KeyPageMode                  = "PageMode"                  // PAGE_MODE
	KeyPages                     = "Pages"                     // PAGES
	KeyPaintType                 = "PaintType"                 // PAINT_TYPE
	KeyPanose                    = "Panose"                    // PANOSE
	KeyParams                    = "Params"                    // PARAMS
	KeyParent                    = "Parent"                    // PARENT
	KeyParentTree                = "ParentTree"                // PARENT_TREE
	KeyParentTreeNextKey         = "ParentTreeNextKey"         // PARENT_TREE_NEXT_KEY
	KeyPart                      = "Part"                      // PART
	KeyPath                      = "Path"                      // PATH
	KeyPattern                   = "Pattern"                   // PATTERN
	KeyPatternType               = "PatternType"               // PATTERN_TYPE
	KeyPC                        = "PC"                        // PC
	KeyPDFDocEncoding            = "PDFDocEncoding"            // PDF_DOC_ENCODING
	KeyPerms                     = "Perms"                     // PERMS
	KeyPerceptual                = "Perceptual"                // PERCEPTUAL
	KeyPieceInfo                 = "PieceInfo"                 // PIECE_INFO
	KeyPg                        = "Pg"                        // PG
	KeyPI                        = "PI"                        // PI
	KeyPO                        = "PO"                        // PO
	KeyPopup                     = "Popup"                     // POPUP
	KeyPreRelease                = "PreRelease"                // PRE_RELEASE
	KeyPredictor                 = "Predictor"                 // PREDICTOR
	KeyPrev                      = "Prev"                      // PREV
	KeyPrint                     = "Print"                     // PRINT
	KeyPrintArea                 = "PrintArea"                 // PRINT_AREA
	KeyPrintClip                 = "PrintClip"                 // PRINT_CLIP
	KeyPrintScaling              = "PrintScaling"              // PRINT_SCALING
	KeyPrintState                = "PrintState"                // PRINT_STATE
	KeyPrivate                   = "Private"                   // PRIVATE
	KeyProcSet                   = "ProcSet"                   // PROC_SET
	KeyProcess                   = "Process"                   // PROCESS
	KeyProducer                  = "Producer"                  // PRODUCER
	KeyPropBuild                 = "Prop_Build"                // PROP_BUILD
	KeyProperties                = "Properties"                // PROPERTIES
	KeyPS                        = "PS"                        // PS
	KeyPtData                    = "PtData"                    // PT_DATA
	KeyPubSec                    = "PubSec"                    // PUB_SEC
	KeyPV                        = "PV"                        // PV
	KeyQ                         = "Q"                         // Q
	KeyQuadPoints                = "QuadPoints"                // QUADPOINTS
	KeyR                         = "R"                         // R
	KeyRange                     = "Range"                     // RANGE
	KeyRC                        = "RC"                        // RC
	KeyRD                        = "RD"                        // RD
	KeyReason                    = "Reason"                    // REASON
	KeyReasons                   = "Reasons"                   // REASONS
	KeyRecipients                = "Recipients"                // RECIPIENTS
	KeyRect                      = "Rect"                      // RECT
	KeyRef                       = "Ref"                       // REF
	KeyReference                 = "Reference"                 // REFERENCE
	KeyRegistry                  = "Registry"                  // REGISTRY
	KeyRegistryName              = "RegistryName"              // REGISTRY_NAME
	KeyRelativeColorimetric      = "RelativeColorimetric"      // RELATIVE_COLORIMETRIC
	KeyRename                    = "Rename"                    // RENAME
	KeyRepeat                    = "Repeat"                    // REPEAT
	KeyResFork                   = "ResFork"                   // RES_FORK
	KeyResources                 = "Resources"                 // RESOURCES
	KeyRGB                       = "RGB"                       // RGB
	KeyRI                        = "RI"                        // RI
	KeyRoleMap                   = "RoleMap"                   // ROLE_MAP
	KeyRoot                      = "Root"                      // ROOT
	KeyRotate                    = "Rotate"                    // ROTATE
	KeyRows                      = "Rows"                      // ROWS
	KeyRT                        = "RT"                        // RT
	KeyRunLengthDecode           = "RunLengthDecode"           // RUN_LENGTH_DECODE
	KeyRL                        = "RL"                        // RUN_LENGTH_DECODE_ABBREVIATION
	KeyRV                        = "RV"                        // RV
	KeyS                         = "S"                         // S
	KeySA                        = "SA"                        // SA
	KeySaturation                = "Saturation"                // SATURATION
	KeySchema                    = "Schema"                    // SCHEMA
	KeyScreen                    = "Screen"                    // SCREEN
	KeySE                        = "SE"                        // SE
	KeySeparation                = "Separation"                // SEPARATION
	KeySetF                      = "SetF"                      // SET_F
	KeySetFf                     = "SetFf"                     // SET_FF
	KeyShading                   = "Shading"                   // SHADING
	KeyShadingType               = "ShadingType"               // SHADING_TYPE
	KeySig                       = "Sig"                       // SIG
	KeySigFlags                  = "SigFlags"                  // SIG_FLAGS
	KeySigRef                    = "SigRef"                    // SIG_REF
	KeySize                      = "Size"                      // SIZE
	KeySM                        = "SM"                        // SM
	KeySMask                     = "SMask"                     // SMASK
	KeySMaskInData               = "SMaskInData"               // SMASK_IN_DATA
	KeySoftLight                 = "SoftLight"                 // SOFT_LIGHT
	KeySort                      = "Sort"                      // SORT
	KeySound                     = "Sound"                     // SOUND
	KeySplit                     = "Split"                     // SPLIT
	KeySS                        = "SS"                        // SS
	KeySt                        = "St"                        // ST
	KeyStandardEncoding          = "StandardEncoding"          // STANDARD_ENCODING
	KeyState                     = "State"                     // STATE
	KeyStateModel                = "StateModel"                // STATE_MODEL
	KeyStatus                    = "Status"                    // STATUS
	KeyStdCF                     = "StdCF"                     // STD_CF
	KeyStemH                     = "StemH"                     // STEM_H
	KeyStemV                     = "StemV"                     // STEM_V
	KeyStmF                      = "StmF"                      // STM_F
	KeyStrF                      = "StrF"                      // STR_F
	KeyStructElem                = "StructElem"                // STRUCT_ELEM
	KeyStructParent              = "StructParent"              // STRUCT_PARENT
	KeyStructParents             = "StructParents"             // STRUCT_PARENTS
	KeyStructTreeRoot            = "StructTreeRoot"            // STRUCT_TREE_ROOT
	KeyStyle                     = "Style"                     // STYLE
	KeySubFilter                 = "SubFilter"                 // SUB_FILTER
	KeySubj                      = "Subj"                      // SUBJ
	KeySubject                   = "Subject"                   // SUBJECT
	KeySubjectDN                 = "SubjectDN"                 // SUBJECT_DN
	KeySubtype                   = "Subtype"                   // SUBTYPE
	KeySupplement                = "Supplement"                // SUPPLEMENT
	KeySV                        = "SV"                        // SV
	KeySVCert                    = "SVCert"                    // SV_CERT
	KeySW                        = "SW"                        // SW
	KeySy                        = "Sy"                        // SY
	KeySynchronous               = "Synchronous"               // SYNCHRONOUS
	KeyT                         = "T"                         // T
	KeyTarget                    = "Target"                    // TARGET
	KeyTemplates                 = "Templates"                 // TEMPLATES
	KeyThread                    = "Thread"                    // THREAD
	KeyThreads                   = "Threads"                   // THREADS
	KeyThreeDd                   = "3DD"                       // THREE_DD
	KeyThumb                     = "Thumb"                     // THUMB
	KeyTI                        = "TI"                        // TI
	KeyTilingType                = "TilingType"                // TILING_TYPE
	KeyTimeStamp                 = "TimeStamp"                 // TIME_STAMP
	KeyTitle                     = "Title"                     // TITLE
	KeyTK                        = "TK"                        // TK
	KeyTM                        = "TM"                        // TM
	KeyToUnicode                 = "ToUnicode"                 // TO_UNICODE
	KeyTR                        = "TR"                        // TR
	KeyTR2                       = "TR2"                       // TR2
	KeyTrapped                   = "Trapped"                   // TRAPPED
	KeyTrans                     = "Trans"                     // TRANS
	KeyTransformMethod           = "TransformMethod"           // TRANSFORM_METHOD
	KeyTransformParams           = "TransformParams"           // TRANSFORM_PARAMS
	KeyTransparency              = "Transparency"              // TRANSPARENCY
	KeyTRef                      = "TRef"                      // TREF
	KeyTrimBox                   = "TrimBox"                   // TRIM_BOX
	KeyTrueType                  = "TrueType"                  // TRUE_TYPE
	KeyTrustedMode               = "TrustedMode"               // TRUSTED_MODE
	KeyTU                        = "TU"                        // TU
	KeyTx                        = "Tx"                        // TX
	KeyType                      = "Type"                      // TYPE
	KeyType0                     = "Type0"                     // TYPE0
	KeyType1                     = "Type1"                     // TYPE1
	KeyType1C                    = "Type1C"                    // TYPE1C
	KeyType3                     = "Type3"                     // TYPE3
	KeyU                         = "U"                         // U
	KeyUE                        = "UE"                        // UE
	KeyUF                        = "UF"                        // UF
	KeyUnchanged                 = "Unchanged"                 // UNCHANGED
	KeyUnix                      = "Unix"                      // UNIX
	KeyURI                       = "URI"                       // URI
	KeyURL                       = "URL"                       // URL
	KeyURLType                   = "URLType"                   // URL_TYPE
	KeyUsage                     = "Usage"                     // USAGE
	KeyUseCMap                   = "UseCMap"                   // USE_CMAP
	KeyUserUnit                  = "UserUnit"                  // USER_UNIT
	KeyV                         = "V"                         // V
	KeyVE                        = "VE"                        // VE
	KeyVerisignPpkvs             = "VeriSign.PPKVS"            // VERISIGN_PPKVS
	KeyVersion                   = "Version"                   // VERSION
	KeyVertices                  = "Vertices"                  // VERTICES
	KeyVerticesPerRow            = "VerticesPerRow"            // VERTICES_PER_ROW
	KeyView                      = "View"                      // VIEW
	KeyViewArea                  = "ViewArea"                  // VIEW_AREA
	KeyViewClip                  = "ViewClip"                  // VIEW_CLIP
	KeyViewState                 = "ViewState"                 // VIEW_STATE
	KeyViewerPreferences         = "ViewerPreferences"         // VIEWER_PREFERENCES
	KeyVolume                    = "Volume"                    // VOLUME
	KeyVP                        = "VP"                        // VP
	KeyVRI                       = "VRI"                       // VRI
	KeyW                         = "W"                         // W
	KeyW2                        = "W2"                        // W2
	KeyWC                        = "WC"                        // WC
	KeyWhitePoint                = "WhitePoint"                // WHITE_POINT
	KeyWidget                    = "Widget"                    // WIDGET
	KeyWidth                     = "Width"                     // WIDTH
	KeyWidths                    = "Widths"                    // WIDTHS
	KeyWin                       = "Win"                       // WIN
	KeyWinAnsiEncoding           = "WinAnsiEncoding"           // WIN_ANSI_ENCODING
	KeyWMode                     = "WMode"                     // WMODE
	KeyWP                        = "WP"                        // WP
	KeyWS                        = "WS"                        // WS
	KeyX                         = "X"                         // X
	KeyXFA                       = "XFA"                       // XFA
	KeyXStep                     = "XStep"                     // X_STEP
	KeyXHeight                   = "XHeight"                   // XHEIGHT
	KeyXObject                   = "XObject"                   // XOBJECT
	KeyXRef                      = "XRef"                      // XREF
	KeyXRefStm                   = "XRefStm"                   // XREF_STM
	KeyY                         = "Y"                         // Y
	KeyYStep                     = "YStep"                     // Y_STEP
	KeyYes                       = "Yes"                       // YES
	KeyZaDb                      = "ZaDb"                      // ZA_DB
)
