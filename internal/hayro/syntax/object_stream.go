// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/stream.rs.

package syntax

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/xo/ivy"
)

// filtersAndParams is FiltersAndParams in hayro-syntax src/object/stream.rs.
type filtersAndParams struct {
	filters []Filter
	params  []Dict
}

// Stream is Stream in hayro-syntax src/object/stream.rs. It is a stream of
// data with its dictionary.
type Stream struct {
	dict Dict
	data []byte
}

// Equal reports whether two streams hold the same dictionary and data, as
// the PartialEq of upstream does.
func (s Stream) Equal(other Stream) bool {
	return s.dict.Equal(other.dict) && bytes.Equal(s.data, other.data)
}

// ImageDecodeParams is ImageDecodeParams in hayro-syntax
// src/object/stream.rs. It holds more parameters for the decoding of images.
//
// Port: the options of upstream become zero values. A zero Bpc,
// NumComponents or TargetDimension stands for None, because zero is not a
// valid value for any of them.
type ImageDecodeParams struct {
	// IsIndexed is true when the color space of the image is indexed.
	IsIndexed bool
	// Bpc is the number of bits per component of the image, or 0.
	Bpc uint8
	// NumComponents is the number of components of the color space, or 0.
	NumComponents uint8
	// TargetDimension is a hint for the size of the decoded image, or zero.
	TargetDimension [2]uint32
	// Width is the width of the image that the image dictionary states.
	Width uint32
	// Height is the height of the image that the image dictionary states.
	Height uint32
	// Decoders holds the decoders of JBIG2 and JPX for this call. A nil
	// value takes the decoders of the Options of the document.
	//
	// Port: upstream calls its own crates. ivy fills the hooks from outside.
	Decoders *ImageDecoders
}

// newStream is Stream::new in hayro-syntax src/object/stream.rs.
func newStream(data []byte, dict Dict) Stream {
	return Stream{dict: dict, data: data}
}

// filtersAndParams is Stream::filters_and_params in hayro-syntax
// src/object/stream.rs.
func (s Stream) filtersAndParams() filtersAndParams {
	var out filtersAndParams
	name, ok := s.dict.Get[Name](KeyF)
	if !ok {
		name, ok = s.dict.Get[Name](KeyFilter)
	}
	var filter Filter
	if ok {
		filter, ok = filterFromName(name)
	}
	if ok {
		params, ok := s.dict.Get[Dict](KeyDP)
		if !ok {
			params, _ = s.dict.Get[Dict](KeyDecodeParms)
		}
		out.filters = append(out.filters, filter)
		out.params = append(out.params, params)
		return out
	}
	filters, ok := s.dict.Get[Array](KeyF)
	if !ok {
		filters, ok = s.dict.Get[Array](KeyFilter)
	}
	if !ok {
		return out
	}
	paramsArray, hasParams := s.dict.Get[Array](KeyDP)
	if !hasParams {
		paramsArray, hasParams = s.dict.Get[Array](KeyDecodeParms)
	}
	var params *ResolvedArrayIter[Object]
	if hasParams {
		params = paramsArray.Iter[Object]()
	}
	for name := range filters.Iter[Name]().All() {
		var p Dict
		if params != nil {
			if o, ok := params.Next(); ok {
				p, _ = ObjectIntoDict(o)
			}
		}
		if f, ok := filterFromName(name); ok {
			out.filters = append(out.filters, f)
			out.params = append(out.params, p)
		}
	}
	return out
}

// RawData is Stream::raw_data in hayro-syntax src/object/stream.rs. It
// returns the data of the stream, decrypted but not decoded.
func (s Stream) RawData() []byte {
	ctx := s.dict.ctx()
	xref := ctx.XRef()
	if !xref.needsDecryption(ctx) {
		return s.data
	}
	if t, ok := s.dict.Get[Name](KeyType); ok && t == KeyXRef {
		return s.data
	}
	target := decryptionTarget{kind: targetStream}
	if name, ok := s.cryptFilter(); ok {
		target = decryptionTarget{kind: targetNamed, name: name}
	}
	id, ok := s.dict.ObjID()
	if !ok {
		return s.data
	}
	d, ok := xref.decrypt(id, s.data, target)
	if !ok {
		// Upstream notes that an error can be better.
		return nil
	}
	return d
}

// Dict is Stream::dict in hayro-syntax src/object/stream.rs. It returns the
// raw dictionary of the stream.
func (s Stream) Dict() Dict {
	return s.dict
}

// ObjID is Stream::obj_id in hayro-syntax src/object/stream.rs. It returns
// the identifier of the stream, or 0 0 when it has none.
func (s Stream) ObjID() ObjectIdentifier {
	id, _ := s.dict.ObjID()
	return id
}

// Filters is Stream::filters in hayro-syntax src/object/stream.rs.
func (s Stream) Filters() []Filter {
	return s.filtersAndParams().filters
}

// Decoded is Stream::decoded in hayro-syntax src/object/stream.rs. It
// returns the decoded data of the stream.
func (s Stream) Decoded() ([]byte, error) {
	res, err := s.DecodedImage(&ImageDecodeParams{})
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

// DecodedData is Stream::decoded_data in hayro-syntax src/object/stream.rs.
// It returns the decoded data of a stream that uses no filter of images.
func (s Stream) DecodedData() ([]byte, error) {
	for _, f := range s.Filters() {
		switch f {
		case FilterDCTDecode, FilterJPXDecode, FilterJBIG2Decode, FilterCCITTFaxDecode:
			return nil, &DecodeFailure{Kind: DecodeFailureImageDecode}
		}
	}
	return s.Decoded()
}

// DecodedImage is Stream::decoded_image in hayro-syntax
// src/object/stream.rs. It returns the decoded data of the stream, with the
// data of the image when a filter of images gives it.
func (s Stream) DecodedImage(imageParams *ImageDecodeParams) (FilterResult, error) {
	if f, ok := s.dict.Get[Object](KeyF); ok {
		switch f.(type) {
		case String, Dict:
			return FilterResult{}, &DecodeFailure{Kind: DecodeFailureStreamDecode}
		}
	}
	data := s.RawData()
	fp := s.filtersAndParams()
	env := s.decodeEnv(imageParams)
	var current *FilterResult
	for i, filter := range fp.filters {
		if i == 0 && filter == FilterCrypt {
			continue
		}
		input := data
		if current != nil {
			input = current.Data
		}
		next, err := filter.apply(input, fp.params[i], imageParams, env)
		if err != nil {
			return FilterResult{}, err
		}
		current = &next
	}
	if current == nil {
		return FilterResult{Data: data}, nil
	}
	return *current, nil
}

// decodeEnv returns the limits and the decoders for the filters of the
// stream.
func (s Stream) decodeEnv(imageParams *ImageDecodeParams) *decodeEnv {
	ctx := s.dict.ctx()
	env := ctx.XRef().decodeEnv()
	if ctx.XRef() == nil && ctx.env != nil {
		env = ctx.env
	}
	if imageParams != nil && imageParams.Decoders != nil {
		e := *env
		e.decoders = *imageParams.Decoders
		env = &e
	}
	return env
}

// cryptFilter is Stream::crypt_filter in hayro-syntax src/object/stream.rs.
func (s Stream) cryptFilter() (Name, bool) {
	fp := s.filtersAndParams()
	if len(fp.filters) == 0 || fp.filters[0] != FilterCrypt {
		return "", false
	}
	if name, ok := fp.params[0].Get[Name](KeyName); ok {
		return name, true
	}
	return KeyIdentity, true
}

// GoString returns the Debug text of upstream.
func (s Stream) GoString() string {
	return s.String()
}

// String returns the Display text of upstream.
func (s Stream) String() string {
	return "Stream (len: " + strconv.Itoa(len(s.data)) + ")"
}

// readIndirectStream is Readable::read_indirect of Stream in hayro-syntax
// src/object/stream.rs. A stream is only read as an indirect object.
func readIndirectStream(r *Reader, ctx *ReaderContext) (Stream, bool) {
	dict, ok := r.ReadWithContext[Dict](ctx)
	if !ok {
		return Stream{}, false
	}
	offset := r.Offset()
	if s, ok := parseProper(r, dict); ok {
		return s, true
	}
	r.Jump(offset)
	return parseFallback(r, dict)
}

// DecodeFailureKind is the kind of a DecodeFailure.
type DecodeFailureKind uint8

// The kinds of DecodeFailure in hayro-syntax src/object/stream.rs.
const (
	// DecodeFailureImageDecode is DecodeFailure::ImageDecode: an image
	// stream failed to decode.
	DecodeFailureImageDecode DecodeFailureKind = iota
	// DecodeFailureStreamDecode is DecodeFailure::StreamDecode: a data stream
	// failed to decode.
	DecodeFailureStreamDecode
	// DecodeFailureDecryption is DecodeFailure::Decryption: the decryption
	// failed.
	DecodeFailureDecryption
	// DecodeFailureUnknown is DecodeFailure::Unknown.
	DecodeFailureUnknown
)

// DecodeFailure is DecodeFailure in hayro-syntax src/object/stream.rs. It
// is a failure of the decoding of a stream.
//
// Port: the Rust enum becomes the field Kind. Err holds the cause when there
// is one, such as ivy.ErrUnsupported for a filter without a decoder, or
// ivy.ErrLimit. The error wraps Err, or ivy.ErrFormat without one.
type DecodeFailure struct {
	// Kind is the case of the Rust enum.
	Kind DecodeFailureKind
	// Err is the cause, or nil.
	Err error
}

// Error returns the message of the failure.
func (f *DecodeFailure) Error() string {
	var kind string
	switch f.Kind {
	case DecodeFailureImageDecode:
		kind = "decoding image stream"
	case DecodeFailureStreamDecode:
		kind = "decoding stream"
	case DecodeFailureDecryption:
		kind = "decrypting stream"
	default:
		kind = "decoding stream"
	}
	return kind + ": " + f.Unwrap().Error()
}

// Unwrap returns the cause.
func (f *DecodeFailure) Unwrap() error {
	if f.Err != nil {
		return f.Err
	}
	return ivy.ErrFormat
}

// ImageColorSpaceKind is the kind of an ImageColorSpace.
type ImageColorSpaceKind uint8

// The kinds of ImageColorSpace in hayro-syntax src/object/stream.rs.
const (
	// ImageColorSpaceGray is ImageColorSpace::Gray.
	ImageColorSpaceGray ImageColorSpaceKind = iota
	// ImageColorSpaceRGB is ImageColorSpace::Rgb.
	ImageColorSpaceRGB
	// ImageColorSpaceCMYK is ImageColorSpace::Cmyk.
	ImageColorSpaceCMYK
	// ImageColorSpaceUnknown is ImageColorSpace::Unknown.
	ImageColorSpaceUnknown
)

// ImageColorSpace is ImageColorSpace in hayro-syntax src/object/stream.rs.
// It is the color space of a decoded image.
//
// Port: the Rust enum becomes a struct. Unknown holds the number of channels
// of the case Unknown.
type ImageColorSpace struct {
	// Kind is the case of the Rust enum.
	Kind ImageColorSpaceKind
	// Unknown is the number of channels of the case Unknown.
	Unknown uint8
}

// String returns the Debug text of upstream.
func (c ImageColorSpace) String() string {
	switch c.Kind {
	case ImageColorSpaceGray:
		return "Gray"
	case ImageColorSpaceRGB:
		return "Rgb"
	case ImageColorSpaceCMYK:
		return "Cmyk"
	}
	return fmt.Sprintf("Unknown(%d)", c.Unknown)
}

// ImageData is ImageData in hayro-syntax src/object/stream.rs. It holds the
// data that some filters of images give.
type ImageData struct {
	// Alpha is the alpha channel of the image, or nil.
	Alpha []byte
	// ColorSpace is the color space of the image, or nil.
	ColorSpace *ImageColorSpace
	// BitsPerComponent is the number of bits per component.
	BitsPerComponent uint8
	// Width is the width of the image.
	Width uint32
	// Height is the height of the image.
	Height uint32
}

// FilterResult is FilterResult in hayro-syntax src/object/stream.rs. It is
// the result of a filter.
type FilterResult struct {
	// Data is the decoded data.
	Data []byte
	// ImageData is the data of the image, or nil.
	ImageData *ImageData
}

// filterResultFromData is FilterResult::from_data in hayro-syntax
// src/object/stream.rs.
func filterResultFromData(data []byte) FilterResult {
	return FilterResult{Data: data}
}

// parseProper is parse_proper in hayro-syntax src/object/stream.rs. It reads
// the data with the length that the dictionary states.
func parseProper(r *Reader, dict Dict) (Stream, bool) {
	length, ok := dict.Get[uint32](KeyLength)
	if !ok {
		return Stream{}, false
	}
	r.SkipWhiteSpacesAndComments()
	if !r.ForwardTag("stream") {
		return Stream{}, false
	}
	if !r.ForwardTag("\n") && !r.ForwardTag("\r\n") && !r.ForwardTag("\r") {
		return Stream{}, false
	}
	data, ok := r.ReadBytes(int(length))
	if !ok {
		return Stream{}, false
	}
	r.SkipWhiteSpaces()
	if !r.ForwardTag("endstream") {
		return Stream{}, false
	}
	return newStream(data, dict), true
}

// parseFallback is parse_fallback in hayro-syntax src/object/stream.rs. It
// searches for endstream when the length is wrong.
func parseFallback(r *Reader, dict Dict) (Stream, bool) {
	r.SkipWhiteSpacesAndComments()
	if !r.ForwardTag("stream") {
		return Stream{}, false
	}
	// A lone carriage return is not allowed, but there is no reason to not
	// try it.
	if !r.ForwardTag("\n") && !r.ForwardTag("\r\n") && !r.ForwardTag("\r") {
		return Stream{}, false
	}
	// Upstream notes that this search can take quadratic time in total, when
	// many streams with a wrong length have no endstream after them, or only
	// a distant one.
	//
	// Port: the data of the document remembers the last search, so that the
	// total time stays linear. The result does not change.
	tail, ok := r.Tail()
	if !ok {
		return Stream{}, false
	}
	var endstreamOffset int
	if x := dict.ctx().XRef(); x != nil && x.data.holds(r.Data) {
		endstreamOffset, ok = x.data.endstream.find(r.Data, r.Offset())
		endstreamOffset -= r.Offset()
	} else {
		endstreamOffset, ok = findNeedle(tail, []byte("endstream"))
	}
	if !ok {
		return Stream{}, false
	}
	dataEnd := trimTrailingASCIIWhitespace(tail[:endstreamOffset])
	data := tail[:dataEnd]
	r.ReadBytes(endstreamOffset)
	r.SkipWhiteSpaces()
	if !r.ForwardTag("endstream") {
		return Stream{}, false
	}
	return newStream(data, dict), true
}

// trimTrailingASCIIWhitespace is trim_trailing_ascii_whitespace in
// hayro-syntax src/object/stream.rs.
func trimTrailingASCIIWhitespace(data []byte) int {
	end := len(data)
	for end > 0 && isWhiteSpaceCharacter(data[end-1]) {
		end--
	}
	return end
}
