// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/convert.rs.

package interpret

import (
	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/internal/hayro/kurboext"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// convertTransform is convert_transform in hayro-interpret src/convert.rs.
func convertTransform(t syntax.TransformOps) kurbo.Affine {
	return kurbo.NewAffine([6]float64{
		t.V0.AsF64(), t.V1.AsF64(), t.V2.AsF64(), t.V3.AsF64(), t.V4.AsF64(), t.V5.AsF64(),
	})
}

// convertLineCap is convert_line_cap in hayro-interpret src/convert.rs.
func convertLineCap(lc syntax.LineCap) kurboext.Cap {
	switch lc.V0.AsI64() {
	case 1:
		return kurboext.CapRound
	case 2:
		return kurboext.CapSquare
	}
	return kurboext.CapButt
}

// convertLineJoin is convert_line_join in hayro-interpret src/convert.rs.
func convertLineJoin(lj syntax.LineJoin) kurboext.Join {
	switch lj.V0.AsI64() {
	case 1:
		return kurboext.JoinRound
	case 2:
		return kurboext.JoinBevel
	}
	return kurboext.JoinMiter
}
