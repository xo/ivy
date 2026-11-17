// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/content/ops.rs. The generated operators
// are in content_ops_generated.go.

package syntax

import "strings"

// parseNamedColor is parse_named_color in hayro-syntax src/content/ops.rs.
// It takes numbers and at most one name, the last one.
func parseNamedColor(objects []Object) ([]Number, *Name, bool) {
	var nums []Number
	var name *Name
	for _, o := range objects {
		switch v := o.(type) {
		case Number:
			nums = append(nums, v)
		case Name:
			name = &v
		default:
			return nil, nil, false
		}
	}
	return nums, name, true
}

// fmtNamedColor is fmt_named_color in hayro-syntax src/content/ops.rs.
func fmtNamedColor(operator string, numbers []Number, name *Name) string {
	var b strings.Builder
	b.WriteString(operator)
	if len(numbers) == 0 && name == nil {
		return b.String()
	}
	b.WriteString("(")
	for i, n := range numbers {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(n.String())
	}
	if name != nil {
		if len(numbers) > 0 {
			b.WriteString(" ")
		}
		b.WriteString(name.String())
	}
	b.WriteString(")")
	return b.String()
}

// StrokeColorNamed is StrokeColorNamed in hayro-syntax src/content/ops.rs,
// the operator SCN.
type StrokeColorNamed struct {
	// V0 holds the numbers.
	V0 []Number
	// V1 is the name, or nil.
	V1 *Name
}

// String returns the Display text of upstream.
func (o StrokeColorNamed) String() string {
	return fmtNamedColor("StrokeColorNamed", o.V0, o.V1)
}

func (StrokeColorNamed) isTypedInstruction() {}

// strokeColorNamedFromStack is OperatorTrait::from_stack of
// StrokeColorNamed in hayro-syntax src/content/ops.rs.
func strokeColorNamedFromStack(s *Stack) (StrokeColorNamed, bool) {
	nums, name, ok := parseNamedColor(s.asSlice())
	return StrokeColorNamed{V0: nums, V1: name}, ok
}

// NonStrokeColorNamed is NonStrokeColorNamed in hayro-syntax
// src/content/ops.rs, the operator scn.
type NonStrokeColorNamed struct {
	// V0 holds the numbers.
	V0 []Number
	// V1 is the name, or nil.
	V1 *Name
}

// String returns the Display text of upstream.
func (o NonStrokeColorNamed) String() string {
	return fmtNamedColor("NonStrokeColorNamed", o.V0, o.V1)
}

func (NonStrokeColorNamed) isTypedInstruction() {}

// nonStrokeColorNamedFromStack is OperatorTrait::from_stack of
// NonStrokeColorNamed in hayro-syntax src/content/ops.rs.
func nonStrokeColorNamedFromStack(s *Stack) (NonStrokeColorNamed, bool) {
	nums, name, ok := parseNamedColor(s.asSlice())
	return NonStrokeColorNamed{V0: nums, V1: name}, ok
}
