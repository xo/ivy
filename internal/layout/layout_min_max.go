// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/min_max.py: the decorators that handle
// min- and max- widths and heights.
//
// Port: a decorator of upstream wraps a function. The port calls a function
// through handleMinMaxWidth or handleMinMaxHeight, with a closure that
// holds its arguments. The undecorated function, the attribute
// without_min_max of upstream, is the closure itself.

package layout

// handleMinMaxWidth is the decorator handle_min_max_width: f sets the used
// width of box. handleMinMaxWidth calls f, and calls it again with the
// width that min-width and max-width give.
func handleMinMaxWidth[R any](box *Box, f func() R) R {
	result := f()
	if box.Width > box.MaxWidth {
		box.Width = box.MaxWidth
		result = f()
	}
	if box.Width < box.MinWidth {
		box.Width = box.MinWidth
		result = f()
	}
	return result
}

// handleMinMaxHeight is the decorator handle_min_max_height: f sets the
// used height of box. handleMinMaxHeight calls f, and calls it again with
// the height that min-height and max-height give.
func handleMinMaxHeight[R any](box *Box, f func() R) R {
	result := f()
	if box.Height > box.MaxHeight {
		box.Height = box.MaxHeight
		result = f()
	}
	if box.Height < box.MinHeight {
		box.Height = box.MinHeight
		result = f()
	}
	return result
}
