// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/object/mod.rs.

package syntax

// maxObjectNestingDepth is MAX_OBJECT_NESTING_DEPTH in hayro-syntax
// src/object/mod.rs.
const maxObjectNestingDepth = 512

// Object is Object in hayro-syntax src/object/mod.rs. It is a primitive PDF
// object: Null, Bool, Number, String, Name, Dict, Array or Stream.
//
// Port: the Rust enum becomes a sealed interface. String returns the Display
// text of upstream, and GoString returns its Debug text.
type Object interface {
	String() string
	GoString() string
	isObject()
}

func (Null) isObject()   {}
func (Bool) isObject()   {}
func (Number) isObject() {}
func (String) isObject() {}
func (Name) isObject()   {}
func (Dict) isObject()   {}
func (Array) isObject()  {}
func (Stream) isObject() {}

// ObjectIntoDict is Object::into_dict in hayro-syntax src/object/mod.rs.
func ObjectIntoDict(o Object) (Dict, bool) {
	return castValue[Dict](o)
}

// ObjectIntoName is Object::into_name in hayro-syntax src/object/mod.rs.
func ObjectIntoName(o Object) (Name, bool) {
	return castValue[Name](o)
}

// ObjectIntoStream is Object::into_stream in hayro-syntax src/object/mod.rs.
func ObjectIntoStream(o Object) (Stream, bool) {
	return castValue[Stream](o)
}

// ObjectIntoArray is Object::into_array in hayro-syntax src/object/mod.rs.
func ObjectIntoArray(o Object) (Array, bool) {
	return castValue[Array](o)
}

// ObjectIntoNumber is Object::into_number in hayro-syntax src/object/mod.rs.
func ObjectIntoNumber(o Object) (Number, bool) {
	return castValue[Number](o)
}

// ObjectCast is Object::cast in hayro-syntax src/object/mod.rs. It converts
// the object into a T, for the types that Reader.Read lists. It also stands
// for into_null, into_bool, into_string, into_u8, into_u16, into_f32 and
// into_i32 of upstream.
func ObjectCast[T any](o Object) (T, bool) {
	return castValue[T](o)
}

// skipObject is the Skippable implementation of Object in hayro-syntax
// src/object/mod.rs.
func skipObject(r *Reader, isContentStream bool, depth int) bool {
	b, ok := r.PeekByte()
	if !ok {
		return false
	}
	switch {
	case b == 'n':
		return skipNull(r)
	case b == 't' || b == 'f':
		return skipBool(r)
	case b == '/':
		return skipName(r)
	case b == '<':
		two, ok := r.PeekBytes(2)
		if !ok {
			return false
		}
		if string(two) == "<<" {
			return skipDict(r, isContentStream, depth)
		}
		return skipString(r)
	case b == '(':
		return skipString(r)
	case b == '.' || b == '+' || b == '-' || isDigit(b):
		return skipNumber(r)
	case b == '[':
		return skipArray(r, isContentStream, depth)
	default:
		// Lenient: skip content operators inside arrays.
		return skipNameLike(r, false)
	}
}

// readObject is the Readable implementation of Object in hayro-syntax
// src/object/mod.rs.
func readObject(r *Reader, ctx *ReaderContext) (Object, bool) {
	b, ok := r.PeekByte()
	if !ok {
		return nil, false
	}
	var o Object
	switch {
	case b == 'n':
		if !readNull(r) {
			return nil, false
		}
		o = Null{}
	case b == 't' || b == 'f':
		v, ok := readBool(r)
		if !ok {
			return nil, false
		}
		o = v
	case b == '/':
		v, ok := readName(r)
		if !ok {
			return nil, false
		}
		o = v
	case b == '<':
		two, ok := r.PeekBytes(2)
		if !ok {
			return nil, false
		}
		if string(two) == "<<" {
			v, ok := readDict(r, ctx)
			if !ok {
				return nil, false
			}
			o = v
		} else {
			v, ok := readString(r, ctx)
			if !ok {
				return nil, false
			}
			o = v
		}
	case b == '(':
		v, ok := readString(r, ctx)
		if !ok {
			return nil, false
		}
		o = v
	case b == '.' || b == '+' || b == '-' || isDigit(b):
		v, ok := readNumber(r)
		if !ok {
			return nil, false
		}
		o = v
	case b == '[':
		v, ok := readArray(r, ctx)
		if !ok {
			return nil, false
		}
		o = v
	default:
		// See the comment in skipObject.
		if !skipNameLike(r, false) {
			return nil, false
		}
		o = Null{}
	}
	return o, true
}

// readIndirectObject is Readable::read_indirect of Object in hayro-syntax
// src/object/mod.rs.
func readIndirectObject(r *Reader, ctx *ReaderContext) (Object, bool) {
	if two, ok := r.PeekBytes(2); ok && string(two) == "<<" {
		cloned := r.clone()
		dict, ok := readDict(cloned, ctx)
		if !ok {
			return nil, false
		}
		cloned.SkipWhiteSpacesAndComments()
		if cloned.ForwardTag("stream") {
			s, ok := readIndirectStream(r, ctx)
			if !ok {
				return nil, false
			}
			return s, true
		}
		r.Jump(cloned.Offset())
		return dict, true
	}
	return readObject(r, ctx)
}

// FromBytes is FromBytes::from_bytes in hayro-syntax src/object/mod.rs. It
// reads a T, as an indirect object, from b with a dummy context.
func FromBytes[T any](b []byte) (T, bool) {
	return readIndirectValue[T](NewReader(b), DummyReaderContext())
}

// ObjectIdentifier is ObjectIdentifier in hayro-syntax src/object/mod.rs. It
// identifies a PDF object.
type ObjectIdentifier struct {
	// ObjNumber is the object number.
	ObjNumber uint32
	// GenNumber is the generation number.
	GenNumber int32
}

// NewObjectIdentifier is ObjectIdentifier::new in hayro-syntax
// src/object/mod.rs.
func NewObjectIdentifier(objNumber uint32, genNumber int32) ObjectIdentifier {
	return ObjectIdentifier{ObjNumber: objNumber, GenNumber: genNumber}
}

// compare orders identifiers as the derived Ord of upstream does.
func (id ObjectIdentifier) compare(other ObjectIdentifier) int {
	switch {
	case id.ObjNumber < other.ObjNumber:
		return -1
	case id.ObjNumber > other.ObjNumber:
		return 1
	case id.GenNumber < other.GenNumber:
		return -1
	case id.GenNumber > other.GenNumber:
		return 1
	}
	return 0
}

// readObjectIdentifier is the Readable implementation of ObjectIdentifier
// in hayro-syntax src/object/mod.rs.
func readObjectIdentifier(r *Reader) (ObjectIdentifier, bool) {
	objNumber, ok := r.ReadWithoutContext[uint32]()
	if !ok {
		return ObjectIdentifier{}, false
	}
	r.SkipWhiteSpacesAndComments()
	genNumber, ok := r.ReadWithoutContext[int32]()
	if !ok {
		return ObjectIdentifier{}, false
	}
	r.SkipWhiteSpacesAndComments()
	if !r.ForwardTag("obj") {
		return ObjectIdentifier{}, false
	}
	return ObjectIdentifier{ObjNumber: objNumber, GenNumber: genNumber}, true
}

// skipObjectIdentifier is the Skippable implementation of ObjectIdentifier
// in hayro-syntax src/object/mod.rs.
func skipObjectIdentifier(r *Reader) bool {
	if _, ok := r.Skip[uint32](false); !ok {
		return false
	}
	r.SkipWhiteSpacesAndComments()
	if _, ok := r.Skip[int32](false); !ok {
		return false
	}
	r.SkipWhiteSpacesAndComments()
	return r.ForwardTag("obj")
}

// DictOrStream is dict_or_stream in hayro-syntax src/object/mod.rs. It
// returns the dictionary of an object, and the stream if it is one.
func DictOrStream(o Object) (Dict, *Stream, bool) {
	switch v := o.(type) {
	case Stream:
		return v.Dict(), &v, true
	case Dict:
		return v, nil, true
	}
	return Dict{}, nil, false
}

// objectDebug formats o as the derived Debug text of Object in upstream,
// such as Number(Number(Integer(34))).
func objectDebug(o Object) string {
	switch v := o.(type) {
	case Null:
		return "Null(Null)"
	case Bool:
		return "Boolean(" + v.GoString() + ")"
	case Number:
		return "Number(" + v.GoString() + ")"
	case String:
		return "String(" + v.GoString() + ")"
	case Name:
		return "Name(" + v.GoString() + ")"
	case Dict:
		return "Dict(" + v.GoString() + ")"
	case Array:
		return "Array(" + v.GoString() + ")"
	case Stream:
		return "Stream(" + v.GoString() + ")"
	}
	return "None"
}
