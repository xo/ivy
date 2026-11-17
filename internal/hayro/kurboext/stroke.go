// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// This file ports the items of kurbo kurbo/src/stroke.rs that the port of
// hayro needs and that the subset of kurbo in okage does not hold.

package kurboext

// Cap is Cap in kurbo kurbo/src/stroke.rs. It is the shape at the ends of
// an open stroke.
type Cap uint8

// The caps of kurbo.
const (
	// CapButt is Cap::Butt: the stroke ends at the end of the path.
	CapButt Cap = iota
	// CapSquare is Cap::Square: the stroke ends half the width of the line
	// past the end of the path.
	CapSquare
	// CapRound is Cap::Round: the stroke ends in a half circle.
	CapRound
)

// String returns the Debug text of upstream.
func (c Cap) String() string {
	switch c {
	case CapSquare:
		return "Square"
	case CapRound:
		return "Round"
	}
	return "Butt"
}

// Join is Join in kurbo kurbo/src/stroke.rs. It is the shape where two
// segments of a stroke meet.
type Join uint8

// The joins of kurbo.
const (
	// JoinBevel is Join::Bevel: a straight line joins the two corners.
	JoinBevel Join = iota
	// JoinMiter is Join::Miter: the outer edges meet at a point.
	JoinMiter
	// JoinRound is Join::Round: an arc joins the two corners.
	JoinRound
)

// String returns the Debug text of upstream.
func (j Join) String() string {
	switch j {
	case JoinMiter:
		return "Miter"
	case JoinRound:
		return "Round"
	}
	return "Bevel"
}
