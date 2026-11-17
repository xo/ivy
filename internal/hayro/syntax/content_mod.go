// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/content/mod.rs.

package syntax

import (
	"strings"
)

// operandsThreshold is OPERANDS_THRESHOLD in hayro-syntax
// src/content/mod.rs: scn with 32 DeviceN components and a pattern name.
const operandsThreshold = 33

// maxInlineImageDepth bounds the nesting of the scans of the content after
// an inline image.
//
// Port: upstream scans the content after each candidate end of an inline
// image with a new iterator, and that iterator can meet another inline
// image. Upstream does not bound the nesting, so a crafted stream can take
// exponential time. Past this depth, the scan takes the data as content.
const maxInlineImageDepth = 4

// Operator is Operator in hayro-syntax src/content/mod.rs. It is the
// operator of a content stream.
type Operator Name

// String returns the Display text of upstream.
func (o Operator) String() string {
	return Name(o).AsStr()
}

// skipOperator is the Skippable implementation of Operator in hayro-syntax
// src/content/mod.rs.
func skipOperator(r *Reader) bool {
	return skipNameLike(r, false)
}

// readOperator is the Readable implementation of Operator in hayro-syntax
// src/content/mod.rs.
func readOperator(r *Reader) (Operator, bool) {
	start := r.Offset()
	escaped := false
	for {
		b, ok := r.PeekByte()
		if !ok || !isRegularCharacter(b) {
			break
		}
		escaped = escaped || b == '#'
		r.Forward()
	}
	data, ok := r.Range(start, r.Offset())
	if !ok || len(data) == 0 {
		return "", false
	}
	if escaped {
		name, ok := NewNameEscaped(data)
		if !ok {
			return "", false
		}
		return Operator(name), true
	}
	return Operator(NewNameUnescaped(data)), true
}

// UntypedIter is UntypedIter in hayro-syntax src/content/mod.rs. It
// iterates over the instructions of a content stream, without types.
type UntypedIter struct {
	reader   *Reader
	stack    Stack
	operator Operator
	depth    int
}

// NewUntypedIter is UntypedIter::new in hayro-syntax src/content/mod.rs.
func NewUntypedIter(data []byte) *UntypedIter {
	return &UntypedIter{reader: NewReader(data)}
}

// EmptyUntypedIter is UntypedIter::empty in hayro-syntax
// src/content/mod.rs.
func EmptyUntypedIter() *UntypedIter {
	return &UntypedIter{reader: NewReader(nil)}
}

// Next is UntypedIter::next in hayro-syntax src/content/mod.rs. It returns
// the next instruction. The operands stay valid until the next call.
func (it *UntypedIter) Next() (Instruction, bool) {
	it.stack.clear()
	op, ok := readInstruction(it.reader, &it.stack, it.depth)
	if !ok {
		return Instruction{}, false
	}
	it.operator = op
	return Instruction{Operands: &it.stack, Operator: it.operator}, true
}

// readInstruction is read_instruction in hayro-syntax src/content/mod.rs.
func readInstruction(r *Reader, stack *Stack, depth int) (Operator, bool) {
	cur := r.clone()
	for {
		cur.SkipWhiteSpacesAndComments()
		first, ok := cur.PeekByte()
		if !ok {
			*r = *cur
			return "", false
		}
		isNumberStart := first == '.' || first == '+' || first == '-' || isDigit(first)
		if isNumberStart {
			start := cur.Offset()
			if number, ok := readSimpleNumber(cur); ok {
				if !stack.push(number) {
					*r = *cur
					return "", false
				}
				continue
			}
			cur.Jump(start)
		}
		if isNumberStart || first == '/' || first == '[' || first == '<' || first == '(' {
			*r = *cur
			if !readOperand(r, stack) {
				return "", false
			}
			cur = r.clone()
			continue
		}
		*r = *cur
		operator, ok := r.ReadWithoutContext[Operator]()
		if !ok {
			r.JumpToEnd()
			return "", false
		}
		if operator == "BI" && !readInlineImage(r, stack, depth) {
			return "", false
		}
		return operator, true
	}
}

// readOperand is read_operand in hayro-syntax src/content/mod.rs.
func readOperand(r *Reader, stack *Stack) bool {
	// A number glued to regular characters acts as an unknown operator.
	// Skip it (issue 994).
	if object, ok := r.ReadWithoutContext[Object](); ok {
		return stack.push(object)
	}
	if _, ok := r.ReadWithoutContext[Operator](); ok {
		stack.clear()
		return true
	}
	return false
}

// readInlineImage is read_inline_image in hayro-syntax src/content/mod.rs.
// It reads the dictionary and the data of an inline image, up to the EI
// that content follows.
func readInlineImage(r *Reader, stack *Stack, depth int) bool {
	inlineDict, ok := r.ReadWithoutContext[InlineImageDict]()
	if !ok {
		return false
	}
	dict := inlineDict.GetDict()
	if !r.ReadWhiteSpace() {
		return false
	}
	streamData, ok := r.Tail()
	if !ok {
		return false
	}
	startOffset := r.Offset()
outer:
	for {
		tail, ok := r.Tail()
		if !ok {
			return false
		}
		pos, ok := findNeedle(tail, []byte("EI"))
		if !ok {
			break
		}
		if _, ok := r.ReadBytes(pos); !ok {
			return false
		}
		if !r.PeekTag("EI") {
			continue
		}
		if b, ok := r.PeekBytes(3); ok && !isWhiteSpaceCharacter(b[2]) {
			r.ReadBytes(3)
			continue
		}
		endOffset := r.Offset() - startOffset
		imageData := streamData[:endOffset]
		stream := newStream(imageData, dict)
		// EI can appear inside the image data, so make sure that content
		// follows.
		rest, ok := r.Tail()
		if !ok {
			return false
		}
		tail = rest[2:]
		findReader := NewReader(tail)
		var binary binaryTracker
		isContent, hasIsContent := false, false
		nextBi, hasNextBi := findNeedle(tail, []byte("BI"))
		for !findReader.AtEnd() {
			offset := findReader.Offset()
			if hasNextBi && nextBi < offset {
				var bi int
				bi, hasNextBi = findNeedle(tail[offset:], []byte("BI"))
				nextBi = offset + bi
			}
			ei, hasEi := findNeedle(tail[offset:], []byte("EI"))
			ei += offset
			if !hasEi && !hasNextBi {
				break
			}
			nextPos, isEi := nextBi, false
			if hasEi && (!hasNextBi || ei <= nextBi) {
				nextPos, isEi = ei, true
			}
			if _, ok := findReader.ReadBytes(nextPos - offset); !ok {
				return false
			}
			if isEi {
				if binary.foundBefore(tail, nextPos) {
					if _, ok := r.ReadBytes(2); !ok {
						return false
					}
					continue outer
				}
				if !hasIsContent {
					isContent, hasIsContent = startsWithContent(tail, depth+1), true
				}
				if !isContent {
					if _, ok := r.ReadBytes(2); !ok {
						return false
					}
					continue outer
				}
			} else {
				cloned := findReader.clone()
				if _, ok := cloned.ReadBytes(2); !ok {
					return false
				}
				if _, ok := cloned.ReadWithoutContext[InlineImageDict](); ok {
					break
				}
			}
			if _, ok := findReader.ReadU8(); !ok {
				return false
			}
		}
		if !stack.push(stream) {
			return false
		}
		if _, ok := r.ReadBytes(2); !ok {
			return false
		}
		r.SkipWhiteSpaces()
		break
	}
	return true
}

// startsWithContent is starts_with_content in hayro-syntax
// src/content/mod.rs. It reports whether data starts with operators of
// content, and not with image data.
func startsWithContent(data []byte, depth int) bool {
	if depth > maxInlineImageDepth {
		return true
	}
	it := NewTypedIter(data)
	it.untyped.depth = depth
	counter := 0
	for {
		op, ok := it.Next()
		if !ok {
			return false
		}
		// 20 valid operators in a row are very unlikely in image data.
		if counter >= 20 {
			return true
		}
		switch op.(type) {
		case NextLineAndShowText, ShowText, ShowTexts, ShowTextWithParameters:
			return true
		}
		counter++
	}
}

// binaryTracker is BinaryTracker in hayro-syntax src/content/mod.rs. A byte
// that is not ASCII outside a string means that the data is still image
// data.
type binaryTracker struct {
	pos         int
	stringDepth int
	escaped     bool
	found       bool
}

// foundBefore is BinaryTracker::found_before in hayro-syntax
// src/content/mod.rs.
func (t *binaryTracker) foundBefore(data []byte, end int) bool {
	if t.found || end <= t.pos {
		return t.found
	}
	for i, b := range data[t.pos:end] {
		switch {
		case t.stringDepth > 0:
			switch {
			case t.escaped:
				t.escaped = false
			case b == '\\':
				t.escaped = true
			case b == '(':
				t.stringDepth++
			case b == ')':
				t.stringDepth--
			}
		case b >= 0x80:
			t.found = true
			t.pos += i
			return true
		case b == '(':
			t.stringDepth = 1
		}
	}
	t.pos = end
	return false
}

// TypedIter is TypedIter in hayro-syntax src/content/mod.rs. It iterates
// over the instructions of a content stream, with types.
type TypedIter struct {
	untyped *UntypedIter
}

// NewTypedIter is TypedIter::new in hayro-syntax src/content/mod.rs.
func NewTypedIter(data []byte) *TypedIter {
	return &TypedIter{untyped: NewUntypedIter(data)}
}

// typedIterFromUntyped is TypedIter::from_untyped in hayro-syntax
// src/content/mod.rs.
func typedIterFromUntyped(untyped *UntypedIter) *TypedIter {
	return &TypedIter{untyped: untyped}
}

// Next is TypedIter::next in hayro-syntax src/content/mod.rs. It returns the
// next typed instruction.
func (it *TypedIter) Next() (TypedInstruction, bool) {
	op, ok := it.untyped.Next()
	if !ok {
		return nil, false
	}
	// Upstream notes that the dispatch can be more efficient.
	if typed, ok := dispatchTypedInstruction(op); ok {
		return typed, true
	}
	// Stop on an invalid operator of a path, so that no garbage gets drawn,
	// and skip other operators.
	switch op.Operator {
	case "m", "l", "c", "v", "y", "h", "re":
		return nil, false
	}
	return Fallback{V0: op.Operator}, true
}

// Instruction is Instruction in hayro-syntax src/content/mod.rs. It is an
// operator with its operands.
type Instruction struct {
	// Operands is the stack with the operands.
	Operands *Stack
	// Operator is the operator.
	Operator Operator
}

// OperandIterator is Instruction::operands in hayro-syntax
// src/content/mod.rs. It returns the operands.
func (i Instruction) OperandIterator() []Object {
	return i.Operands.asSlice()
}

// Stack is Stack in hayro-syntax src/content/mod.rs. It holds the operands
// of an operator.
type Stack struct {
	// Upstream notes that a pool of objects can avoid the repeated
	// allocation of objects.
	data []Object
}

// NewStack is Stack::new in hayro-syntax src/content/mod.rs.
func NewStack() *Stack {
	return &Stack{}
}

// push is Stack::push in hayro-syntax src/content/mod.rs.
func (s *Stack) push(operand Object) bool {
	if len(s.data) >= operandsThreshold {
		return false
	}
	s.data = append(s.data, operand)
	return true
}

// clear is Stack::clear in hayro-syntax src/content/mod.rs.
func (s *Stack) clear() {
	clear(s.data)
	s.data = s.data[:0]
}

// len is Stack::len in hayro-syntax src/content/mod.rs.
func (s *Stack) len() int {
	return len(s.data)
}

// asSlice is Stack::as_slice in hayro-syntax src/content/mod.rs.
func (s *Stack) asSlice() []Object {
	return s.data
}

// GoString returns the Debug text of upstream.
func (s *Stack) GoString() string {
	var b strings.Builder
	b.WriteString("[")
	for i, o := range s.data {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(objectDebug(o))
	}
	b.WriteString("]")
	return b.String()
}

// stackGet is Stack::get in hayro-syntax src/content/mod.rs. It returns the
// operand at index as a T, which is Number, String, Name, Array, Stream or
// Object, as Operand of upstream lists.
func stackGet[T any](s *Stack, index int) (T, bool) {
	if index < 0 || index >= len(s.data) {
		var zero T
		return zero, false
	}
	return castValue[T](s.data[index])
}

// stackGetAll is Stack::get_all in hayro-syntax src/content/mod.rs.
func stackGetAll[T any](s *Stack) ([]T, bool) {
	out := make([]T, 0, len(s.data))
	for _, o := range s.data {
		v, ok := castValue[T](o)
		if !ok {
			return nil, false
		}
		out = append(out, v)
	}
	return out, true
}

// TypedInstruction is TypedInstruction in hayro-syntax
// src/content/ops_generated.rs. It is an operator with operands of the
// right types.
//
// Port: the Rust enum becomes a sealed interface. Each case is a type of
// its own, such as MoveTo, and String returns the Display text of upstream.
type TypedInstruction interface {
	String() string
	isTypedInstruction()
}

// Fallback is TypedInstruction::Fallback in hayro-syntax
// src/content/ops_generated.rs. It is an operator that the dispatch does
// not know.
type Fallback struct {
	// V0 is the operator.
	V0 Operator
}

// String returns the Display text of upstream.
func (f Fallback) String() string {
	return "Fallback(" + f.V0.String() + ")"
}

func (Fallback) isTypedInstruction() {}

// dispatchOp converts the result of a from_stack function into a
// TypedInstruction.
func dispatchOp[T TypedInstruction](v T, ok bool) (TypedInstruction, bool) {
	if !ok {
		return nil, false
	}
	return v, true
}

// displayOp is the macro display_op in hayro-syntax src/content/mod.rs. It
// formats an operator with its operands.
func displayOp(name string, fields ...any) string {
	var b strings.Builder
	b.WriteString(name)
	b.WriteString("(")
	for i, f := range fields {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(displayAny(f))
	}
	b.WriteString(")")
	return b.String()
}

// displayVariadicOp is the macro display_variadic_op in hayro-syntax
// src/content/mod.rs.
func displayVariadicOp(name string, values []Number) string {
	if len(values) == 0 {
		return name
	}
	fields := make([]any, len(values))
	for i, v := range values {
		fields[i] = v
	}
	return displayOp(name, fields...)
}
