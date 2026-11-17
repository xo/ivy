// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/content/ops_generated.rs, which upstream
// generates with src/content/gen_ops.py. A script made this file from the
// Rust file.

package syntax

// BeginCompatibility is BeginCompatibility in hayro-syntax
// src/content/ops_generated.rs, the operator BX.
type BeginCompatibility struct{}

// String returns the Display text of upstream.
func (BeginCompatibility) String() string {
	return "BeginCompatibility"
}

func (BeginCompatibility) isTypedInstruction() {}

// beginCompatibilityFromStack is OperatorTrait::from_stack of
// BeginCompatibility in hayro-syntax src/content/ops_generated.rs.
func beginCompatibilityFromStack(*Stack) (BeginCompatibility, bool) {
	return BeginCompatibility{}, true
}

// EndCompatibility is EndCompatibility in hayro-syntax
// src/content/ops_generated.rs, the operator EX.
type EndCompatibility struct{}

// String returns the Display text of upstream.
func (EndCompatibility) String() string {
	return "EndCompatibility"
}

func (EndCompatibility) isTypedInstruction() {}

// endCompatibilityFromStack is OperatorTrait::from_stack of
// EndCompatibility in hayro-syntax src/content/ops_generated.rs.
func endCompatibilityFromStack(*Stack) (EndCompatibility, bool) {
	return EndCompatibility{}, true
}

// SaveState is SaveState in hayro-syntax
// src/content/ops_generated.rs, the operator q.
type SaveState struct{}

// String returns the Display text of upstream.
func (SaveState) String() string {
	return "SaveState"
}

func (SaveState) isTypedInstruction() {}

// saveStateFromStack is OperatorTrait::from_stack of
// SaveState in hayro-syntax src/content/ops_generated.rs.
func saveStateFromStack(*Stack) (SaveState, bool) {
	return SaveState{}, true
}

// RestoreState is RestoreState in hayro-syntax
// src/content/ops_generated.rs, the operator Q.
type RestoreState struct{}

// String returns the Display text of upstream.
func (RestoreState) String() string {
	return "RestoreState"
}

func (RestoreState) isTypedInstruction() {}

// restoreStateFromStack is OperatorTrait::from_stack of
// RestoreState in hayro-syntax src/content/ops_generated.rs.
func restoreStateFromStack(*Stack) (RestoreState, bool) {
	return RestoreState{}, true
}

// TransformOps is Transform in hayro-syntax
// src/content/ops_generated.rs, the operator cm.
//
// Port: the name takes the suffix of the module ops, because the type
// Transform of src/transform.rs has the name Transform (okage D10).
type TransformOps struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
	// V3 is the operand 3.
	V3 Number
	// V4 is the operand 4.
	V4 Number
	// V5 is the operand 5.
	V5 Number
}

// String returns the Display text of upstream.
func (o TransformOps) String() string {
	return displayOp("Transform", o.V0, o.V1, o.V2, o.V3, o.V4, o.V5)
}

func (TransformOps) isTypedInstruction() {}

// transformOpsFromStack is OperatorTrait::from_stack of
// Transform in hayro-syntax src/content/ops_generated.rs.
func transformOpsFromStack(s *Stack) (TransformOps, bool) {
	shift := max(s.len()-6, 0)
	var o TransformOps
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return TransformOps{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return TransformOps{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return TransformOps{}, false
	}
	if o.V3, ok = stackGet[Number](s, 3+shift); !ok {
		return TransformOps{}, false
	}
	if o.V4, ok = stackGet[Number](s, 4+shift); !ok {
		return TransformOps{}, false
	}
	if o.V5, ok = stackGet[Number](s, 5+shift); !ok {
		return TransformOps{}, false
	}
	return o, true
}

// LineWidth is LineWidth in hayro-syntax
// src/content/ops_generated.rs, the operator w.
type LineWidth struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o LineWidth) String() string {
	return displayOp("LineWidth", o.V0)
}

func (LineWidth) isTypedInstruction() {}

// lineWidthFromStack is OperatorTrait::from_stack of
// LineWidth in hayro-syntax src/content/ops_generated.rs.
func lineWidthFromStack(s *Stack) (LineWidth, bool) {
	shift := max(s.len()-1, 0)
	var o LineWidth
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return LineWidth{}, false
	}
	return o, true
}

// LineCap is LineCap in hayro-syntax
// src/content/ops_generated.rs, the operator J.
type LineCap struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o LineCap) String() string {
	return displayOp("LineCap", o.V0)
}

func (LineCap) isTypedInstruction() {}

// lineCapFromStack is OperatorTrait::from_stack of
// LineCap in hayro-syntax src/content/ops_generated.rs.
func lineCapFromStack(s *Stack) (LineCap, bool) {
	shift := max(s.len()-1, 0)
	var o LineCap
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return LineCap{}, false
	}
	return o, true
}

// LineJoin is LineJoin in hayro-syntax
// src/content/ops_generated.rs, the operator j.
type LineJoin struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o LineJoin) String() string {
	return displayOp("LineJoin", o.V0)
}

func (LineJoin) isTypedInstruction() {}

// lineJoinFromStack is OperatorTrait::from_stack of
// LineJoin in hayro-syntax src/content/ops_generated.rs.
func lineJoinFromStack(s *Stack) (LineJoin, bool) {
	shift := max(s.len()-1, 0)
	var o LineJoin
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return LineJoin{}, false
	}
	return o, true
}

// MiterLimit is MiterLimit in hayro-syntax
// src/content/ops_generated.rs, the operator M.
type MiterLimit struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o MiterLimit) String() string {
	return displayOp("MiterLimit", o.V0)
}

func (MiterLimit) isTypedInstruction() {}

// miterLimitFromStack is OperatorTrait::from_stack of
// MiterLimit in hayro-syntax src/content/ops_generated.rs.
func miterLimitFromStack(s *Stack) (MiterLimit, bool) {
	shift := max(s.len()-1, 0)
	var o MiterLimit
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return MiterLimit{}, false
	}
	return o, true
}

// DashPattern is DashPattern in hayro-syntax
// src/content/ops_generated.rs, the operator d.
type DashPattern struct {
	// V0 is the operand 0.
	V0 Array
	// V1 is the operand 1.
	V1 Number
}

// String returns the Display text of upstream.
func (o DashPattern) String() string {
	return displayOp("DashPattern", o.V0, o.V1)
}

func (DashPattern) isTypedInstruction() {}

// dashPatternFromStack is OperatorTrait::from_stack of
// DashPattern in hayro-syntax src/content/ops_generated.rs.
func dashPatternFromStack(s *Stack) (DashPattern, bool) {
	shift := max(s.len()-2, 0)
	var o DashPattern
	var ok bool
	if o.V0, ok = stackGet[Array](s, 0+shift); !ok {
		return DashPattern{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return DashPattern{}, false
	}
	return o, true
}

// RenderingIntent is RenderingIntent in hayro-syntax
// src/content/ops_generated.rs, the operator ri.
type RenderingIntent struct {
	// V0 is the operand 0.
	V0 Name
}

// String returns the Display text of upstream.
func (o RenderingIntent) String() string {
	return displayOp("RenderingIntent", o.V0)
}

func (RenderingIntent) isTypedInstruction() {}

// renderingIntentFromStack is OperatorTrait::from_stack of
// RenderingIntent in hayro-syntax src/content/ops_generated.rs.
func renderingIntentFromStack(s *Stack) (RenderingIntent, bool) {
	shift := max(s.len()-1, 0)
	var o RenderingIntent
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return RenderingIntent{}, false
	}
	return o, true
}

// FlatnessTolerance is FlatnessTolerance in hayro-syntax
// src/content/ops_generated.rs, the operator i.
type FlatnessTolerance struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o FlatnessTolerance) String() string {
	return displayOp("FlatnessTolerance", o.V0)
}

func (FlatnessTolerance) isTypedInstruction() {}

// flatnessToleranceFromStack is OperatorTrait::from_stack of
// FlatnessTolerance in hayro-syntax src/content/ops_generated.rs.
func flatnessToleranceFromStack(s *Stack) (FlatnessTolerance, bool) {
	shift := max(s.len()-1, 0)
	var o FlatnessTolerance
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return FlatnessTolerance{}, false
	}
	return o, true
}

// SetGraphicsState is SetGraphicsState in hayro-syntax
// src/content/ops_generated.rs, the operator gs.
type SetGraphicsState struct {
	// V0 is the operand 0.
	V0 Name
}

// String returns the Display text of upstream.
func (o SetGraphicsState) String() string {
	return displayOp("SetGraphicsState", o.V0)
}

func (SetGraphicsState) isTypedInstruction() {}

// setGraphicsStateFromStack is OperatorTrait::from_stack of
// SetGraphicsState in hayro-syntax src/content/ops_generated.rs.
func setGraphicsStateFromStack(s *Stack) (SetGraphicsState, bool) {
	shift := max(s.len()-1, 0)
	var o SetGraphicsState
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return SetGraphicsState{}, false
	}
	return o, true
}

// MoveTo is MoveTo in hayro-syntax
// src/content/ops_generated.rs, the operator m.
type MoveTo struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
}

// String returns the Display text of upstream.
func (o MoveTo) String() string {
	return displayOp("MoveTo", o.V0, o.V1)
}

func (MoveTo) isTypedInstruction() {}

// moveToFromStack is OperatorTrait::from_stack of
// MoveTo in hayro-syntax src/content/ops_generated.rs.
func moveToFromStack(s *Stack) (MoveTo, bool) {
	shift := max(s.len()-2, 0)
	var o MoveTo
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return MoveTo{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return MoveTo{}, false
	}
	return o, true
}

// LineTo is LineTo in hayro-syntax
// src/content/ops_generated.rs, the operator l.
type LineTo struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
}

// String returns the Display text of upstream.
func (o LineTo) String() string {
	return displayOp("LineTo", o.V0, o.V1)
}

func (LineTo) isTypedInstruction() {}

// lineToFromStack is OperatorTrait::from_stack of
// LineTo in hayro-syntax src/content/ops_generated.rs.
func lineToFromStack(s *Stack) (LineTo, bool) {
	shift := max(s.len()-2, 0)
	var o LineTo
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return LineTo{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return LineTo{}, false
	}
	return o, true
}

// CubicTo is CubicTo in hayro-syntax
// src/content/ops_generated.rs, the operator c.
type CubicTo struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
	// V3 is the operand 3.
	V3 Number
	// V4 is the operand 4.
	V4 Number
	// V5 is the operand 5.
	V5 Number
}

// String returns the Display text of upstream.
func (o CubicTo) String() string {
	return displayOp("CubicTo", o.V0, o.V1, o.V2, o.V3, o.V4, o.V5)
}

func (CubicTo) isTypedInstruction() {}

// cubicToFromStack is OperatorTrait::from_stack of
// CubicTo in hayro-syntax src/content/ops_generated.rs.
func cubicToFromStack(s *Stack) (CubicTo, bool) {
	shift := max(s.len()-6, 0)
	var o CubicTo
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return CubicTo{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return CubicTo{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return CubicTo{}, false
	}
	if o.V3, ok = stackGet[Number](s, 3+shift); !ok {
		return CubicTo{}, false
	}
	if o.V4, ok = stackGet[Number](s, 4+shift); !ok {
		return CubicTo{}, false
	}
	if o.V5, ok = stackGet[Number](s, 5+shift); !ok {
		return CubicTo{}, false
	}
	return o, true
}

// CubicStartTo is CubicStartTo in hayro-syntax
// src/content/ops_generated.rs, the operator v.
type CubicStartTo struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
	// V3 is the operand 3.
	V3 Number
}

// String returns the Display text of upstream.
func (o CubicStartTo) String() string {
	return displayOp("CubicStartTo", o.V0, o.V1, o.V2, o.V3)
}

func (CubicStartTo) isTypedInstruction() {}

// cubicStartToFromStack is OperatorTrait::from_stack of
// CubicStartTo in hayro-syntax src/content/ops_generated.rs.
func cubicStartToFromStack(s *Stack) (CubicStartTo, bool) {
	shift := max(s.len()-4, 0)
	var o CubicStartTo
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return CubicStartTo{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return CubicStartTo{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return CubicStartTo{}, false
	}
	if o.V3, ok = stackGet[Number](s, 3+shift); !ok {
		return CubicStartTo{}, false
	}
	return o, true
}

// CubicEndTo is CubicEndTo in hayro-syntax
// src/content/ops_generated.rs, the operator y.
type CubicEndTo struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
	// V3 is the operand 3.
	V3 Number
}

// String returns the Display text of upstream.
func (o CubicEndTo) String() string {
	return displayOp("CubicEndTo", o.V0, o.V1, o.V2, o.V3)
}

func (CubicEndTo) isTypedInstruction() {}

// cubicEndToFromStack is OperatorTrait::from_stack of
// CubicEndTo in hayro-syntax src/content/ops_generated.rs.
func cubicEndToFromStack(s *Stack) (CubicEndTo, bool) {
	shift := max(s.len()-4, 0)
	var o CubicEndTo
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return CubicEndTo{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return CubicEndTo{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return CubicEndTo{}, false
	}
	if o.V3, ok = stackGet[Number](s, 3+shift); !ok {
		return CubicEndTo{}, false
	}
	return o, true
}

// ClosePath is ClosePath in hayro-syntax
// src/content/ops_generated.rs, the operator h.
type ClosePath struct{}

// String returns the Display text of upstream.
func (ClosePath) String() string {
	return "ClosePath"
}

func (ClosePath) isTypedInstruction() {}

// closePathFromStack is OperatorTrait::from_stack of
// ClosePath in hayro-syntax src/content/ops_generated.rs.
func closePathFromStack(*Stack) (ClosePath, bool) {
	return ClosePath{}, true
}

// RectPath is RectPath in hayro-syntax
// src/content/ops_generated.rs, the operator re.
type RectPath struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
	// V3 is the operand 3.
	V3 Number
}

// String returns the Display text of upstream.
func (o RectPath) String() string {
	return displayOp("RectPath", o.V0, o.V1, o.V2, o.V3)
}

func (RectPath) isTypedInstruction() {}

// rectPathFromStack is OperatorTrait::from_stack of
// RectPath in hayro-syntax src/content/ops_generated.rs.
func rectPathFromStack(s *Stack) (RectPath, bool) {
	shift := max(s.len()-4, 0)
	var o RectPath
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return RectPath{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return RectPath{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return RectPath{}, false
	}
	if o.V3, ok = stackGet[Number](s, 3+shift); !ok {
		return RectPath{}, false
	}
	return o, true
}

// StrokePath is StrokePath in hayro-syntax
// src/content/ops_generated.rs, the operator S.
type StrokePath struct{}

// String returns the Display text of upstream.
func (StrokePath) String() string {
	return "StrokePath"
}

func (StrokePath) isTypedInstruction() {}

// strokePathFromStack is OperatorTrait::from_stack of
// StrokePath in hayro-syntax src/content/ops_generated.rs.
func strokePathFromStack(*Stack) (StrokePath, bool) {
	return StrokePath{}, true
}

// CloseAndStrokePath is CloseAndStrokePath in hayro-syntax
// src/content/ops_generated.rs, the operator s.
type CloseAndStrokePath struct{}

// String returns the Display text of upstream.
func (CloseAndStrokePath) String() string {
	return "CloseAndStrokePath"
}

func (CloseAndStrokePath) isTypedInstruction() {}

// closeAndStrokePathFromStack is OperatorTrait::from_stack of
// CloseAndStrokePath in hayro-syntax src/content/ops_generated.rs.
func closeAndStrokePathFromStack(*Stack) (CloseAndStrokePath, bool) {
	return CloseAndStrokePath{}, true
}

// FillPathNonZero is FillPathNonZero in hayro-syntax
// src/content/ops_generated.rs, the operator f.
type FillPathNonZero struct{}

// String returns the Display text of upstream.
func (FillPathNonZero) String() string {
	return "FillPathNonZero"
}

func (FillPathNonZero) isTypedInstruction() {}

// fillPathNonZeroFromStack is OperatorTrait::from_stack of
// FillPathNonZero in hayro-syntax src/content/ops_generated.rs.
func fillPathNonZeroFromStack(*Stack) (FillPathNonZero, bool) {
	return FillPathNonZero{}, true
}

// FillPathNonZeroCompatibility is FillPathNonZeroCompatibility in hayro-syntax
// src/content/ops_generated.rs, the operator F.
type FillPathNonZeroCompatibility struct{}

// String returns the Display text of upstream.
func (FillPathNonZeroCompatibility) String() string {
	return "FillPathNonZeroCompatibility"
}

func (FillPathNonZeroCompatibility) isTypedInstruction() {}

// fillPathNonZeroCompatibilityFromStack is OperatorTrait::from_stack of
// FillPathNonZeroCompatibility in hayro-syntax src/content/ops_generated.rs.
func fillPathNonZeroCompatibilityFromStack(*Stack) (FillPathNonZeroCompatibility, bool) {
	return FillPathNonZeroCompatibility{}, true
}

// FillPathEvenOdd is FillPathEvenOdd in hayro-syntax
// src/content/ops_generated.rs, the operator f*.
type FillPathEvenOdd struct{}

// String returns the Display text of upstream.
func (FillPathEvenOdd) String() string {
	return "FillPathEvenOdd"
}

func (FillPathEvenOdd) isTypedInstruction() {}

// fillPathEvenOddFromStack is OperatorTrait::from_stack of
// FillPathEvenOdd in hayro-syntax src/content/ops_generated.rs.
func fillPathEvenOddFromStack(*Stack) (FillPathEvenOdd, bool) {
	return FillPathEvenOdd{}, true
}

// FillAndStrokeNonZero is FillAndStrokeNonZero in hayro-syntax
// src/content/ops_generated.rs, the operator B.
type FillAndStrokeNonZero struct{}

// String returns the Display text of upstream.
func (FillAndStrokeNonZero) String() string {
	return "FillAndStrokeNonZero"
}

func (FillAndStrokeNonZero) isTypedInstruction() {}

// fillAndStrokeNonZeroFromStack is OperatorTrait::from_stack of
// FillAndStrokeNonZero in hayro-syntax src/content/ops_generated.rs.
func fillAndStrokeNonZeroFromStack(*Stack) (FillAndStrokeNonZero, bool) {
	return FillAndStrokeNonZero{}, true
}

// FillAndStrokeEvenOdd is FillAndStrokeEvenOdd in hayro-syntax
// src/content/ops_generated.rs, the operator B*.
type FillAndStrokeEvenOdd struct{}

// String returns the Display text of upstream.
func (FillAndStrokeEvenOdd) String() string {
	return "FillAndStrokeEvenOdd"
}

func (FillAndStrokeEvenOdd) isTypedInstruction() {}

// fillAndStrokeEvenOddFromStack is OperatorTrait::from_stack of
// FillAndStrokeEvenOdd in hayro-syntax src/content/ops_generated.rs.
func fillAndStrokeEvenOddFromStack(*Stack) (FillAndStrokeEvenOdd, bool) {
	return FillAndStrokeEvenOdd{}, true
}

// CloseFillAndStrokeNonZero is CloseFillAndStrokeNonZero in hayro-syntax
// src/content/ops_generated.rs, the operator b.
type CloseFillAndStrokeNonZero struct{}

// String returns the Display text of upstream.
func (CloseFillAndStrokeNonZero) String() string {
	return "CloseFillAndStrokeNonZero"
}

func (CloseFillAndStrokeNonZero) isTypedInstruction() {}

// closeFillAndStrokeNonZeroFromStack is OperatorTrait::from_stack of
// CloseFillAndStrokeNonZero in hayro-syntax src/content/ops_generated.rs.
func closeFillAndStrokeNonZeroFromStack(*Stack) (CloseFillAndStrokeNonZero, bool) {
	return CloseFillAndStrokeNonZero{}, true
}

// CloseFillAndStrokeEvenOdd is CloseFillAndStrokeEvenOdd in hayro-syntax
// src/content/ops_generated.rs, the operator b*.
type CloseFillAndStrokeEvenOdd struct{}

// String returns the Display text of upstream.
func (CloseFillAndStrokeEvenOdd) String() string {
	return "CloseFillAndStrokeEvenOdd"
}

func (CloseFillAndStrokeEvenOdd) isTypedInstruction() {}

// closeFillAndStrokeEvenOddFromStack is OperatorTrait::from_stack of
// CloseFillAndStrokeEvenOdd in hayro-syntax src/content/ops_generated.rs.
func closeFillAndStrokeEvenOddFromStack(*Stack) (CloseFillAndStrokeEvenOdd, bool) {
	return CloseFillAndStrokeEvenOdd{}, true
}

// EndPath is EndPath in hayro-syntax
// src/content/ops_generated.rs, the operator n.
type EndPath struct{}

// String returns the Display text of upstream.
func (EndPath) String() string {
	return "EndPath"
}

func (EndPath) isTypedInstruction() {}

// endPathFromStack is OperatorTrait::from_stack of
// EndPath in hayro-syntax src/content/ops_generated.rs.
func endPathFromStack(*Stack) (EndPath, bool) {
	return EndPath{}, true
}

// ClipNonZero is ClipNonZero in hayro-syntax
// src/content/ops_generated.rs, the operator W.
type ClipNonZero struct{}

// String returns the Display text of upstream.
func (ClipNonZero) String() string {
	return "ClipNonZero"
}

func (ClipNonZero) isTypedInstruction() {}

// clipNonZeroFromStack is OperatorTrait::from_stack of
// ClipNonZero in hayro-syntax src/content/ops_generated.rs.
func clipNonZeroFromStack(*Stack) (ClipNonZero, bool) {
	return ClipNonZero{}, true
}

// ClipEvenOdd is ClipEvenOdd in hayro-syntax
// src/content/ops_generated.rs, the operator W*.
type ClipEvenOdd struct{}

// String returns the Display text of upstream.
func (ClipEvenOdd) String() string {
	return "ClipEvenOdd"
}

func (ClipEvenOdd) isTypedInstruction() {}

// clipEvenOddFromStack is OperatorTrait::from_stack of
// ClipEvenOdd in hayro-syntax src/content/ops_generated.rs.
func clipEvenOddFromStack(*Stack) (ClipEvenOdd, bool) {
	return ClipEvenOdd{}, true
}

// ColorSpaceStroke is ColorSpaceStroke in hayro-syntax
// src/content/ops_generated.rs, the operator CS.
type ColorSpaceStroke struct {
	// V0 is the operand 0.
	V0 Name
}

// String returns the Display text of upstream.
func (o ColorSpaceStroke) String() string {
	return displayOp("ColorSpaceStroke", o.V0)
}

func (ColorSpaceStroke) isTypedInstruction() {}

// colorSpaceStrokeFromStack is OperatorTrait::from_stack of
// ColorSpaceStroke in hayro-syntax src/content/ops_generated.rs.
func colorSpaceStrokeFromStack(s *Stack) (ColorSpaceStroke, bool) {
	shift := max(s.len()-1, 0)
	var o ColorSpaceStroke
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return ColorSpaceStroke{}, false
	}
	return o, true
}

// ColorSpaceNonStroke is ColorSpaceNonStroke in hayro-syntax
// src/content/ops_generated.rs, the operator cs.
type ColorSpaceNonStroke struct {
	// V0 is the operand 0.
	V0 Name
}

// String returns the Display text of upstream.
func (o ColorSpaceNonStroke) String() string {
	return displayOp("ColorSpaceNonStroke", o.V0)
}

func (ColorSpaceNonStroke) isTypedInstruction() {}

// colorSpaceNonStrokeFromStack is OperatorTrait::from_stack of
// ColorSpaceNonStroke in hayro-syntax src/content/ops_generated.rs.
func colorSpaceNonStrokeFromStack(s *Stack) (ColorSpaceNonStroke, bool) {
	shift := max(s.len()-1, 0)
	var o ColorSpaceNonStroke
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return ColorSpaceNonStroke{}, false
	}
	return o, true
}

// StrokeColor is StrokeColor in hayro-syntax
// src/content/ops_generated.rs, the operator SC.
type StrokeColor struct {
	// V0 is the operand 0.
	V0 []Number
}

// String returns the Display text of upstream.
func (o StrokeColor) String() string {
	return displayVariadicOp("StrokeColor", o.V0)
}

func (StrokeColor) isTypedInstruction() {}

// strokeColorFromStack is OperatorTrait::from_stack of
// StrokeColor in hayro-syntax src/content/ops_generated.rs.
func strokeColorFromStack(s *Stack) (StrokeColor, bool) {
	all, ok := stackGetAll[Number](s)
	return StrokeColor{V0: all}, ok
}

// NonStrokeColor is NonStrokeColor in hayro-syntax
// src/content/ops_generated.rs, the operator sc.
type NonStrokeColor struct {
	// V0 is the operand 0.
	V0 []Number
}

// String returns the Display text of upstream.
func (o NonStrokeColor) String() string {
	return displayVariadicOp("NonStrokeColor", o.V0)
}

func (NonStrokeColor) isTypedInstruction() {}

// nonStrokeColorFromStack is OperatorTrait::from_stack of
// NonStrokeColor in hayro-syntax src/content/ops_generated.rs.
func nonStrokeColorFromStack(s *Stack) (NonStrokeColor, bool) {
	all, ok := stackGetAll[Number](s)
	return NonStrokeColor{V0: all}, ok
}

// StrokeColorDeviceGray is StrokeColorDeviceGray in hayro-syntax
// src/content/ops_generated.rs, the operator G.
type StrokeColorDeviceGray struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o StrokeColorDeviceGray) String() string {
	return displayOp("StrokeColorDeviceGray", o.V0)
}

func (StrokeColorDeviceGray) isTypedInstruction() {}

// strokeColorDeviceGrayFromStack is OperatorTrait::from_stack of
// StrokeColorDeviceGray in hayro-syntax src/content/ops_generated.rs.
func strokeColorDeviceGrayFromStack(s *Stack) (StrokeColorDeviceGray, bool) {
	shift := max(s.len()-1, 0)
	var o StrokeColorDeviceGray
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return StrokeColorDeviceGray{}, false
	}
	return o, true
}

// NonStrokeColorDeviceGray is NonStrokeColorDeviceGray in hayro-syntax
// src/content/ops_generated.rs, the operator g.
type NonStrokeColorDeviceGray struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o NonStrokeColorDeviceGray) String() string {
	return displayOp("NonStrokeColorDeviceGray", o.V0)
}

func (NonStrokeColorDeviceGray) isTypedInstruction() {}

// nonStrokeColorDeviceGrayFromStack is OperatorTrait::from_stack of
// NonStrokeColorDeviceGray in hayro-syntax src/content/ops_generated.rs.
func nonStrokeColorDeviceGrayFromStack(s *Stack) (NonStrokeColorDeviceGray, bool) {
	shift := max(s.len()-1, 0)
	var o NonStrokeColorDeviceGray
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return NonStrokeColorDeviceGray{}, false
	}
	return o, true
}

// StrokeColorDeviceRgb is StrokeColorDeviceRgb in hayro-syntax
// src/content/ops_generated.rs, the operator RG.
type StrokeColorDeviceRgb struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
}

// String returns the Display text of upstream.
func (o StrokeColorDeviceRgb) String() string {
	return displayOp("StrokeColorDeviceRgb", o.V0, o.V1, o.V2)
}

func (StrokeColorDeviceRgb) isTypedInstruction() {}

// strokeColorDeviceRgbFromStack is OperatorTrait::from_stack of
// StrokeColorDeviceRgb in hayro-syntax src/content/ops_generated.rs.
func strokeColorDeviceRgbFromStack(s *Stack) (StrokeColorDeviceRgb, bool) {
	shift := max(s.len()-3, 0)
	var o StrokeColorDeviceRgb
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return StrokeColorDeviceRgb{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return StrokeColorDeviceRgb{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return StrokeColorDeviceRgb{}, false
	}
	return o, true
}

// NonStrokeColorDeviceRgb is NonStrokeColorDeviceRgb in hayro-syntax
// src/content/ops_generated.rs, the operator rg.
type NonStrokeColorDeviceRgb struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
}

// String returns the Display text of upstream.
func (o NonStrokeColorDeviceRgb) String() string {
	return displayOp("NonStrokeColorDeviceRgb", o.V0, o.V1, o.V2)
}

func (NonStrokeColorDeviceRgb) isTypedInstruction() {}

// nonStrokeColorDeviceRgbFromStack is OperatorTrait::from_stack of
// NonStrokeColorDeviceRgb in hayro-syntax src/content/ops_generated.rs.
func nonStrokeColorDeviceRgbFromStack(s *Stack) (NonStrokeColorDeviceRgb, bool) {
	shift := max(s.len()-3, 0)
	var o NonStrokeColorDeviceRgb
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return NonStrokeColorDeviceRgb{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return NonStrokeColorDeviceRgb{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return NonStrokeColorDeviceRgb{}, false
	}
	return o, true
}

// StrokeColorCmyk is StrokeColorCmyk in hayro-syntax
// src/content/ops_generated.rs, the operator K.
type StrokeColorCmyk struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
	// V3 is the operand 3.
	V3 Number
}

// String returns the Display text of upstream.
func (o StrokeColorCmyk) String() string {
	return displayOp("StrokeColorCmyk", o.V0, o.V1, o.V2, o.V3)
}

func (StrokeColorCmyk) isTypedInstruction() {}

// strokeColorCmykFromStack is OperatorTrait::from_stack of
// StrokeColorCmyk in hayro-syntax src/content/ops_generated.rs.
func strokeColorCmykFromStack(s *Stack) (StrokeColorCmyk, bool) {
	shift := max(s.len()-4, 0)
	var o StrokeColorCmyk
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return StrokeColorCmyk{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return StrokeColorCmyk{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return StrokeColorCmyk{}, false
	}
	if o.V3, ok = stackGet[Number](s, 3+shift); !ok {
		return StrokeColorCmyk{}, false
	}
	return o, true
}

// NonStrokeColorCmyk is NonStrokeColorCmyk in hayro-syntax
// src/content/ops_generated.rs, the operator k.
type NonStrokeColorCmyk struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
	// V3 is the operand 3.
	V3 Number
}

// String returns the Display text of upstream.
func (o NonStrokeColorCmyk) String() string {
	return displayOp("NonStrokeColorCmyk", o.V0, o.V1, o.V2, o.V3)
}

func (NonStrokeColorCmyk) isTypedInstruction() {}

// nonStrokeColorCmykFromStack is OperatorTrait::from_stack of
// NonStrokeColorCmyk in hayro-syntax src/content/ops_generated.rs.
func nonStrokeColorCmykFromStack(s *Stack) (NonStrokeColorCmyk, bool) {
	shift := max(s.len()-4, 0)
	var o NonStrokeColorCmyk
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return NonStrokeColorCmyk{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return NonStrokeColorCmyk{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return NonStrokeColorCmyk{}, false
	}
	if o.V3, ok = stackGet[Number](s, 3+shift); !ok {
		return NonStrokeColorCmyk{}, false
	}
	return o, true
}

// Shading is Shading in hayro-syntax
// src/content/ops_generated.rs, the operator sh.
type Shading struct {
	// V0 is the operand 0.
	V0 Name
}

// String returns the Display text of upstream.
func (o Shading) String() string {
	return displayOp("Shading", o.V0)
}

func (Shading) isTypedInstruction() {}

// shadingFromStack is OperatorTrait::from_stack of
// Shading in hayro-syntax src/content/ops_generated.rs.
func shadingFromStack(s *Stack) (Shading, bool) {
	shift := max(s.len()-1, 0)
	var o Shading
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return Shading{}, false
	}
	return o, true
}

// XObject is XObject in hayro-syntax
// src/content/ops_generated.rs, the operator Do.
type XObject struct {
	// V0 is the operand 0.
	V0 Name
}

// String returns the Display text of upstream.
func (o XObject) String() string {
	return displayOp("XObject", o.V0)
}

func (XObject) isTypedInstruction() {}

// xObjectFromStack is OperatorTrait::from_stack of
// XObject in hayro-syntax src/content/ops_generated.rs.
func xObjectFromStack(s *Stack) (XObject, bool) {
	shift := max(s.len()-1, 0)
	var o XObject
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return XObject{}, false
	}
	return o, true
}

// InlineImage is InlineImage in hayro-syntax
// src/content/ops_generated.rs, the operator BI.
type InlineImage struct {
	// V0 is the operand 0.
	V0 Stream
}

// String returns the Display text of upstream.
func (o InlineImage) String() string {
	return displayOp("InlineImage", o.V0)
}

func (InlineImage) isTypedInstruction() {}

// inlineImageFromStack is OperatorTrait::from_stack of
// InlineImage in hayro-syntax src/content/ops_generated.rs.
func inlineImageFromStack(s *Stack) (InlineImage, bool) {
	shift := max(s.len()-1, 0)
	var o InlineImage
	var ok bool
	if o.V0, ok = stackGet[Stream](s, 0+shift); !ok {
		return InlineImage{}, false
	}
	return o, true
}

// CharacterSpacing is CharacterSpacing in hayro-syntax
// src/content/ops_generated.rs, the operator Tc.
type CharacterSpacing struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o CharacterSpacing) String() string {
	return displayOp("CharacterSpacing", o.V0)
}

func (CharacterSpacing) isTypedInstruction() {}

// characterSpacingFromStack is OperatorTrait::from_stack of
// CharacterSpacing in hayro-syntax src/content/ops_generated.rs.
func characterSpacingFromStack(s *Stack) (CharacterSpacing, bool) {
	shift := max(s.len()-1, 0)
	var o CharacterSpacing
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return CharacterSpacing{}, false
	}
	return o, true
}

// WordSpacing is WordSpacing in hayro-syntax
// src/content/ops_generated.rs, the operator Tw.
type WordSpacing struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o WordSpacing) String() string {
	return displayOp("WordSpacing", o.V0)
}

func (WordSpacing) isTypedInstruction() {}

// wordSpacingFromStack is OperatorTrait::from_stack of
// WordSpacing in hayro-syntax src/content/ops_generated.rs.
func wordSpacingFromStack(s *Stack) (WordSpacing, bool) {
	shift := max(s.len()-1, 0)
	var o WordSpacing
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return WordSpacing{}, false
	}
	return o, true
}

// HorizontalScaling is HorizontalScaling in hayro-syntax
// src/content/ops_generated.rs, the operator Tz.
type HorizontalScaling struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o HorizontalScaling) String() string {
	return displayOp("HorizontalScaling", o.V0)
}

func (HorizontalScaling) isTypedInstruction() {}

// horizontalScalingFromStack is OperatorTrait::from_stack of
// HorizontalScaling in hayro-syntax src/content/ops_generated.rs.
func horizontalScalingFromStack(s *Stack) (HorizontalScaling, bool) {
	shift := max(s.len()-1, 0)
	var o HorizontalScaling
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return HorizontalScaling{}, false
	}
	return o, true
}

// TextLeading is TextLeading in hayro-syntax
// src/content/ops_generated.rs, the operator TL.
type TextLeading struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o TextLeading) String() string {
	return displayOp("TextLeading", o.V0)
}

func (TextLeading) isTypedInstruction() {}

// textLeadingFromStack is OperatorTrait::from_stack of
// TextLeading in hayro-syntax src/content/ops_generated.rs.
func textLeadingFromStack(s *Stack) (TextLeading, bool) {
	shift := max(s.len()-1, 0)
	var o TextLeading
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return TextLeading{}, false
	}
	return o, true
}

// TextFont is TextFont in hayro-syntax
// src/content/ops_generated.rs, the operator Tf.
type TextFont struct {
	// V0 is the operand 0.
	V0 Name
	// V1 is the operand 1.
	V1 Number
}

// String returns the Display text of upstream.
func (o TextFont) String() string {
	return displayOp("TextFont", o.V0, o.V1)
}

func (TextFont) isTypedInstruction() {}

// textFontFromStack is OperatorTrait::from_stack of
// TextFont in hayro-syntax src/content/ops_generated.rs.
func textFontFromStack(s *Stack) (TextFont, bool) {
	shift := max(s.len()-2, 0)
	var o TextFont
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return TextFont{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return TextFont{}, false
	}
	return o, true
}

// TextRenderingMode is TextRenderingMode in hayro-syntax
// src/content/ops_generated.rs, the operator Tr.
type TextRenderingMode struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o TextRenderingMode) String() string {
	return displayOp("TextRenderingMode", o.V0)
}

func (TextRenderingMode) isTypedInstruction() {}

// textRenderingModeFromStack is OperatorTrait::from_stack of
// TextRenderingMode in hayro-syntax src/content/ops_generated.rs.
func textRenderingModeFromStack(s *Stack) (TextRenderingMode, bool) {
	shift := max(s.len()-1, 0)
	var o TextRenderingMode
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return TextRenderingMode{}, false
	}
	return o, true
}

// TextRise is TextRise in hayro-syntax
// src/content/ops_generated.rs, the operator Ts.
type TextRise struct {
	// V0 is the operand 0.
	V0 Number
}

// String returns the Display text of upstream.
func (o TextRise) String() string {
	return displayOp("TextRise", o.V0)
}

func (TextRise) isTypedInstruction() {}

// textRiseFromStack is OperatorTrait::from_stack of
// TextRise in hayro-syntax src/content/ops_generated.rs.
func textRiseFromStack(s *Stack) (TextRise, bool) {
	shift := max(s.len()-1, 0)
	var o TextRise
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return TextRise{}, false
	}
	return o, true
}

// BeginText is BeginText in hayro-syntax
// src/content/ops_generated.rs, the operator BT.
type BeginText struct{}

// String returns the Display text of upstream.
func (BeginText) String() string {
	return "BeginText"
}

func (BeginText) isTypedInstruction() {}

// beginTextFromStack is OperatorTrait::from_stack of
// BeginText in hayro-syntax src/content/ops_generated.rs.
func beginTextFromStack(*Stack) (BeginText, bool) {
	return BeginText{}, true
}

// EndText is EndText in hayro-syntax
// src/content/ops_generated.rs, the operator ET.
type EndText struct{}

// String returns the Display text of upstream.
func (EndText) String() string {
	return "EndText"
}

func (EndText) isTypedInstruction() {}

// endTextFromStack is OperatorTrait::from_stack of
// EndText in hayro-syntax src/content/ops_generated.rs.
func endTextFromStack(*Stack) (EndText, bool) {
	return EndText{}, true
}

// NextLine is NextLine in hayro-syntax
// src/content/ops_generated.rs, the operator Td.
type NextLine struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
}

// String returns the Display text of upstream.
func (o NextLine) String() string {
	return displayOp("NextLine", o.V0, o.V1)
}

func (NextLine) isTypedInstruction() {}

// nextLineFromStack is OperatorTrait::from_stack of
// NextLine in hayro-syntax src/content/ops_generated.rs.
func nextLineFromStack(s *Stack) (NextLine, bool) {
	shift := max(s.len()-2, 0)
	var o NextLine
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return NextLine{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return NextLine{}, false
	}
	return o, true
}

// NextLineAndSetLeading is NextLineAndSetLeading in hayro-syntax
// src/content/ops_generated.rs, the operator TD.
type NextLineAndSetLeading struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
}

// String returns the Display text of upstream.
func (o NextLineAndSetLeading) String() string {
	return displayOp("NextLineAndSetLeading", o.V0, o.V1)
}

func (NextLineAndSetLeading) isTypedInstruction() {}

// nextLineAndSetLeadingFromStack is OperatorTrait::from_stack of
// NextLineAndSetLeading in hayro-syntax src/content/ops_generated.rs.
func nextLineAndSetLeadingFromStack(s *Stack) (NextLineAndSetLeading, bool) {
	shift := max(s.len()-2, 0)
	var o NextLineAndSetLeading
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return NextLineAndSetLeading{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return NextLineAndSetLeading{}, false
	}
	return o, true
}

// SetTextMatrix is SetTextMatrix in hayro-syntax
// src/content/ops_generated.rs, the operator Tm.
type SetTextMatrix struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
	// V3 is the operand 3.
	V3 Number
	// V4 is the operand 4.
	V4 Number
	// V5 is the operand 5.
	V5 Number
}

// String returns the Display text of upstream.
func (o SetTextMatrix) String() string {
	return displayOp("SetTextMatrix", o.V0, o.V1, o.V2, o.V3, o.V4, o.V5)
}

func (SetTextMatrix) isTypedInstruction() {}

// setTextMatrixFromStack is OperatorTrait::from_stack of
// SetTextMatrix in hayro-syntax src/content/ops_generated.rs.
func setTextMatrixFromStack(s *Stack) (SetTextMatrix, bool) {
	shift := max(s.len()-6, 0)
	var o SetTextMatrix
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return SetTextMatrix{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return SetTextMatrix{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return SetTextMatrix{}, false
	}
	if o.V3, ok = stackGet[Number](s, 3+shift); !ok {
		return SetTextMatrix{}, false
	}
	if o.V4, ok = stackGet[Number](s, 4+shift); !ok {
		return SetTextMatrix{}, false
	}
	if o.V5, ok = stackGet[Number](s, 5+shift); !ok {
		return SetTextMatrix{}, false
	}
	return o, true
}

// NextLineUsingLeading is NextLineUsingLeading in hayro-syntax
// src/content/ops_generated.rs, the operator T*.
type NextLineUsingLeading struct{}

// String returns the Display text of upstream.
func (NextLineUsingLeading) String() string {
	return "NextLineUsingLeading"
}

func (NextLineUsingLeading) isTypedInstruction() {}

// nextLineUsingLeadingFromStack is OperatorTrait::from_stack of
// NextLineUsingLeading in hayro-syntax src/content/ops_generated.rs.
func nextLineUsingLeadingFromStack(*Stack) (NextLineUsingLeading, bool) {
	return NextLineUsingLeading{}, true
}

// ShowText is ShowText in hayro-syntax
// src/content/ops_generated.rs, the operator Tj.
type ShowText struct {
	// V0 is the operand 0.
	V0 String
}

// String returns the Display text of upstream.
func (o ShowText) String() string {
	return displayOp("ShowText", o.V0)
}

func (ShowText) isTypedInstruction() {}

// showTextFromStack is OperatorTrait::from_stack of
// ShowText in hayro-syntax src/content/ops_generated.rs.
func showTextFromStack(s *Stack) (ShowText, bool) {
	shift := max(s.len()-1, 0)
	var o ShowText
	var ok bool
	if o.V0, ok = stackGet[String](s, 0+shift); !ok {
		return ShowText{}, false
	}
	return o, true
}

// NextLineAndShowText is NextLineAndShowText in hayro-syntax
// src/content/ops_generated.rs, the operator '.
type NextLineAndShowText struct {
	// V0 is the operand 0.
	V0 String
}

// String returns the Display text of upstream.
func (o NextLineAndShowText) String() string {
	return displayOp("NextLineAndShowText", o.V0)
}

func (NextLineAndShowText) isTypedInstruction() {}

// nextLineAndShowTextFromStack is OperatorTrait::from_stack of
// NextLineAndShowText in hayro-syntax src/content/ops_generated.rs.
func nextLineAndShowTextFromStack(s *Stack) (NextLineAndShowText, bool) {
	shift := max(s.len()-1, 0)
	var o NextLineAndShowText
	var ok bool
	if o.V0, ok = stackGet[String](s, 0+shift); !ok {
		return NextLineAndShowText{}, false
	}
	return o, true
}

// ShowTextWithParameters is ShowTextWithParameters in hayro-syntax
// src/content/ops_generated.rs, the operator ".
type ShowTextWithParameters struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 String
}

// String returns the Display text of upstream.
func (o ShowTextWithParameters) String() string {
	return displayOp("ShowTextWithParameters", o.V0, o.V1, o.V2)
}

func (ShowTextWithParameters) isTypedInstruction() {}

// showTextWithParametersFromStack is OperatorTrait::from_stack of
// ShowTextWithParameters in hayro-syntax src/content/ops_generated.rs.
func showTextWithParametersFromStack(s *Stack) (ShowTextWithParameters, bool) {
	shift := max(s.len()-3, 0)
	var o ShowTextWithParameters
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return ShowTextWithParameters{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return ShowTextWithParameters{}, false
	}
	if o.V2, ok = stackGet[String](s, 2+shift); !ok {
		return ShowTextWithParameters{}, false
	}
	return o, true
}

// ShowTexts is ShowTexts in hayro-syntax
// src/content/ops_generated.rs, the operator TJ.
type ShowTexts struct {
	// V0 is the operand 0.
	V0 Array
}

// String returns the Display text of upstream.
func (o ShowTexts) String() string {
	return displayOp("ShowTexts", o.V0)
}

func (ShowTexts) isTypedInstruction() {}

// showTextsFromStack is OperatorTrait::from_stack of
// ShowTexts in hayro-syntax src/content/ops_generated.rs.
func showTextsFromStack(s *Stack) (ShowTexts, bool) {
	shift := max(s.len()-1, 0)
	var o ShowTexts
	var ok bool
	if o.V0, ok = stackGet[Array](s, 0+shift); !ok {
		return ShowTexts{}, false
	}
	return o, true
}

// ColorGlyph is ColorGlyph in hayro-syntax
// src/content/ops_generated.rs, the operator d0.
type ColorGlyph struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
}

// String returns the Display text of upstream.
func (o ColorGlyph) String() string {
	return displayOp("ColorGlyph", o.V0, o.V1)
}

func (ColorGlyph) isTypedInstruction() {}

// colorGlyphFromStack is OperatorTrait::from_stack of
// ColorGlyph in hayro-syntax src/content/ops_generated.rs.
func colorGlyphFromStack(s *Stack) (ColorGlyph, bool) {
	shift := max(s.len()-2, 0)
	var o ColorGlyph
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return ColorGlyph{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return ColorGlyph{}, false
	}
	return o, true
}

// ShapeGlyph is ShapeGlyph in hayro-syntax
// src/content/ops_generated.rs, the operator d1.
type ShapeGlyph struct {
	// V0 is the operand 0.
	V0 Number
	// V1 is the operand 1.
	V1 Number
	// V2 is the operand 2.
	V2 Number
	// V3 is the operand 3.
	V3 Number
	// V4 is the operand 4.
	V4 Number
	// V5 is the operand 5.
	V5 Number
}

// String returns the Display text of upstream.
func (o ShapeGlyph) String() string {
	return displayOp("ShapeGlyph", o.V0, o.V1, o.V2, o.V3, o.V4, o.V5)
}

func (ShapeGlyph) isTypedInstruction() {}

// shapeGlyphFromStack is OperatorTrait::from_stack of
// ShapeGlyph in hayro-syntax src/content/ops_generated.rs.
func shapeGlyphFromStack(s *Stack) (ShapeGlyph, bool) {
	shift := max(s.len()-6, 0)
	var o ShapeGlyph
	var ok bool
	if o.V0, ok = stackGet[Number](s, 0+shift); !ok {
		return ShapeGlyph{}, false
	}
	if o.V1, ok = stackGet[Number](s, 1+shift); !ok {
		return ShapeGlyph{}, false
	}
	if o.V2, ok = stackGet[Number](s, 2+shift); !ok {
		return ShapeGlyph{}, false
	}
	if o.V3, ok = stackGet[Number](s, 3+shift); !ok {
		return ShapeGlyph{}, false
	}
	if o.V4, ok = stackGet[Number](s, 4+shift); !ok {
		return ShapeGlyph{}, false
	}
	if o.V5, ok = stackGet[Number](s, 5+shift); !ok {
		return ShapeGlyph{}, false
	}
	return o, true
}

// MarkedContentPoint is MarkedContentPoint in hayro-syntax
// src/content/ops_generated.rs, the operator MP.
type MarkedContentPoint struct {
	// V0 is the operand 0.
	V0 Name
}

// String returns the Display text of upstream.
func (o MarkedContentPoint) String() string {
	return displayOp("MarkedContentPoint", o.V0)
}

func (MarkedContentPoint) isTypedInstruction() {}

// markedContentPointFromStack is OperatorTrait::from_stack of
// MarkedContentPoint in hayro-syntax src/content/ops_generated.rs.
func markedContentPointFromStack(s *Stack) (MarkedContentPoint, bool) {
	shift := max(s.len()-1, 0)
	var o MarkedContentPoint
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return MarkedContentPoint{}, false
	}
	return o, true
}

// MarkedContentPointWithProperties is MarkedContentPointWithProperties in hayro-syntax
// src/content/ops_generated.rs, the operator DP.
type MarkedContentPointWithProperties struct {
	// V0 is the operand 0.
	V0 Name
	// V1 is the operand 1.
	V1 Object
}

// String returns the Display text of upstream.
func (o MarkedContentPointWithProperties) String() string {
	return displayOp("MarkedContentPointWithProperties", o.V0, o.V1)
}

func (MarkedContentPointWithProperties) isTypedInstruction() {}

// markedContentPointWithPropertiesFromStack is OperatorTrait::from_stack of
// MarkedContentPointWithProperties in hayro-syntax src/content/ops_generated.rs.
func markedContentPointWithPropertiesFromStack(s *Stack) (MarkedContentPointWithProperties, bool) {
	shift := max(s.len()-2, 0)
	var o MarkedContentPointWithProperties
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return MarkedContentPointWithProperties{}, false
	}
	if o.V1, ok = stackGet[Object](s, 1+shift); !ok {
		return MarkedContentPointWithProperties{}, false
	}
	return o, true
}

// BeginMarkedContent is BeginMarkedContent in hayro-syntax
// src/content/ops_generated.rs, the operator BMC.
type BeginMarkedContent struct {
	// V0 is the operand 0.
	V0 Name
}

// String returns the Display text of upstream.
func (o BeginMarkedContent) String() string {
	return displayOp("BeginMarkedContent", o.V0)
}

func (BeginMarkedContent) isTypedInstruction() {}

// beginMarkedContentFromStack is OperatorTrait::from_stack of
// BeginMarkedContent in hayro-syntax src/content/ops_generated.rs.
func beginMarkedContentFromStack(s *Stack) (BeginMarkedContent, bool) {
	shift := max(s.len()-1, 0)
	var o BeginMarkedContent
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return BeginMarkedContent{}, false
	}
	return o, true
}

// BeginMarkedContentWithProperties is BeginMarkedContentWithProperties in hayro-syntax
// src/content/ops_generated.rs, the operator BDC.
type BeginMarkedContentWithProperties struct {
	// V0 is the operand 0.
	V0 Name
	// V1 is the operand 1.
	V1 Object
}

// String returns the Display text of upstream.
func (o BeginMarkedContentWithProperties) String() string {
	return displayOp("BeginMarkedContentWithProperties", o.V0, o.V1)
}

func (BeginMarkedContentWithProperties) isTypedInstruction() {}

// beginMarkedContentWithPropertiesFromStack is OperatorTrait::from_stack of
// BeginMarkedContentWithProperties in hayro-syntax src/content/ops_generated.rs.
func beginMarkedContentWithPropertiesFromStack(s *Stack) (BeginMarkedContentWithProperties, bool) {
	shift := max(s.len()-2, 0)
	var o BeginMarkedContentWithProperties
	var ok bool
	if o.V0, ok = stackGet[Name](s, 0+shift); !ok {
		return BeginMarkedContentWithProperties{}, false
	}
	if o.V1, ok = stackGet[Object](s, 1+shift); !ok {
		return BeginMarkedContentWithProperties{}, false
	}
	return o, true
}

// EndMarkedContent is EndMarkedContent in hayro-syntax
// src/content/ops_generated.rs, the operator EMC.
type EndMarkedContent struct{}

// String returns the Display text of upstream.
func (EndMarkedContent) String() string {
	return "EndMarkedContent"
}

func (EndMarkedContent) isTypedInstruction() {}

// endMarkedContentFromStack is OperatorTrait::from_stack of
// EndMarkedContent in hayro-syntax src/content/ops_generated.rs.
func endMarkedContentFromStack(*Stack) (EndMarkedContent, bool) {
	return EndMarkedContent{}, true
}

// dispatchTypedInstruction is TypedInstruction::dispatch in hayro-syntax
// src/content/ops_generated.rs. It returns false when the operands do not
// fit the operator.
func dispatchTypedInstruction(instruction Instruction) (TypedInstruction, bool) {
	s := instruction.Operands
	switch string(instruction.Operator) {
	case "BX":
		return dispatchOp(beginCompatibilityFromStack(s))
	case "EX":
		return dispatchOp(endCompatibilityFromStack(s))
	case "q":
		return dispatchOp(saveStateFromStack(s))
	case "Q":
		return dispatchOp(restoreStateFromStack(s))
	case "cm":
		return dispatchOp(transformOpsFromStack(s))
	case "w":
		return dispatchOp(lineWidthFromStack(s))
	case "J":
		return dispatchOp(lineCapFromStack(s))
	case "j":
		return dispatchOp(lineJoinFromStack(s))
	case "M":
		return dispatchOp(miterLimitFromStack(s))
	case "d":
		return dispatchOp(dashPatternFromStack(s))
	case "ri":
		return dispatchOp(renderingIntentFromStack(s))
	case "i":
		return dispatchOp(flatnessToleranceFromStack(s))
	case "gs":
		return dispatchOp(setGraphicsStateFromStack(s))
	case "m":
		return dispatchOp(moveToFromStack(s))
	case "l":
		return dispatchOp(lineToFromStack(s))
	case "c":
		return dispatchOp(cubicToFromStack(s))
	case "v":
		return dispatchOp(cubicStartToFromStack(s))
	case "y":
		return dispatchOp(cubicEndToFromStack(s))
	case "h":
		return dispatchOp(closePathFromStack(s))
	case "re":
		return dispatchOp(rectPathFromStack(s))
	case "S":
		return dispatchOp(strokePathFromStack(s))
	case "s":
		return dispatchOp(closeAndStrokePathFromStack(s))
	case "f":
		return dispatchOp(fillPathNonZeroFromStack(s))
	case "F":
		return dispatchOp(fillPathNonZeroCompatibilityFromStack(s))
	case "f*":
		return dispatchOp(fillPathEvenOddFromStack(s))
	case "B":
		return dispatchOp(fillAndStrokeNonZeroFromStack(s))
	case "B*":
		return dispatchOp(fillAndStrokeEvenOddFromStack(s))
	case "b":
		return dispatchOp(closeFillAndStrokeNonZeroFromStack(s))
	case "b*":
		return dispatchOp(closeFillAndStrokeEvenOddFromStack(s))
	case "n":
		return dispatchOp(endPathFromStack(s))
	case "W":
		return dispatchOp(clipNonZeroFromStack(s))
	case "W*":
		return dispatchOp(clipEvenOddFromStack(s))
	case "CS":
		return dispatchOp(colorSpaceStrokeFromStack(s))
	case "cs":
		return dispatchOp(colorSpaceNonStrokeFromStack(s))
	case "SC":
		return dispatchOp(strokeColorFromStack(s))
	case "SCN":
		return dispatchOp(strokeColorNamedFromStack(s))
	case "sc":
		return dispatchOp(nonStrokeColorFromStack(s))
	case "scn":
		return dispatchOp(nonStrokeColorNamedFromStack(s))
	case "G":
		return dispatchOp(strokeColorDeviceGrayFromStack(s))
	case "g":
		return dispatchOp(nonStrokeColorDeviceGrayFromStack(s))
	case "RG":
		return dispatchOp(strokeColorDeviceRgbFromStack(s))
	case "rg":
		return dispatchOp(nonStrokeColorDeviceRgbFromStack(s))
	case "K":
		return dispatchOp(strokeColorCmykFromStack(s))
	case "k":
		return dispatchOp(nonStrokeColorCmykFromStack(s))
	case "sh":
		return dispatchOp(shadingFromStack(s))
	case "Do":
		return dispatchOp(xObjectFromStack(s))
	case "BI":
		return dispatchOp(inlineImageFromStack(s))
	case "Tc":
		return dispatchOp(characterSpacingFromStack(s))
	case "Tw":
		return dispatchOp(wordSpacingFromStack(s))
	case "Tz":
		return dispatchOp(horizontalScalingFromStack(s))
	case "TL":
		return dispatchOp(textLeadingFromStack(s))
	case "Tf":
		return dispatchOp(textFontFromStack(s))
	case "Tr":
		return dispatchOp(textRenderingModeFromStack(s))
	case "Ts":
		return dispatchOp(textRiseFromStack(s))
	case "BT":
		return dispatchOp(beginTextFromStack(s))
	case "ET":
		return dispatchOp(endTextFromStack(s))
	case "Td":
		return dispatchOp(nextLineFromStack(s))
	case "TD":
		return dispatchOp(nextLineAndSetLeadingFromStack(s))
	case "Tm":
		return dispatchOp(setTextMatrixFromStack(s))
	case "T*":
		return dispatchOp(nextLineUsingLeadingFromStack(s))
	case "Tj":
		return dispatchOp(showTextFromStack(s))
	case "'":
		return dispatchOp(nextLineAndShowTextFromStack(s))
	case `"`:
		return dispatchOp(showTextWithParametersFromStack(s))
	case "TJ":
		return dispatchOp(showTextsFromStack(s))
	case "d0":
		return dispatchOp(colorGlyphFromStack(s))
	case "d1":
		return dispatchOp(shapeGlyphFromStack(s))
	case "MP":
		return dispatchOp(markedContentPointFromStack(s))
	case "DP":
		return dispatchOp(markedContentPointWithPropertiesFromStack(s))
	case "BMC":
		return dispatchOp(beginMarkedContentFromStack(s))
	case "BDC":
		return dispatchOp(beginMarkedContentWithPropertiesFromStack(s))
	case "EMC":
		return dispatchOp(endMarkedContentFromStack(s))
	}
	return Fallback{V0: instruction.Operator}, true
}
