// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/lib.rs.

// Package interpret is a port of the crate hayro-interpret, which
// interprets the content of the pages of PDF files.
//
// The package runs the operators of a content stream and sends each mark,
// such as a path, a run of glyphs or an image, to an abstract Device. A
// program implements a Device to draw the marks, for example into a bitmap
// or into SVG. InterpretPage interprets one page with a Context.
//
// The package draws nothing itself. It resolves the color spaces, the
// functions, the shadings, the patterns, the fonts, the images and the soft
// masks, and it gives them to the device in a form that is ready to draw.
//
// The marks of a page are untrusted input. The package bounds the nesting
// and the work of a page, and a caller can stop an interpretation with
// InterpreterSettings.Cancel.
package interpret
