// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/grid.py: the layout for grid containers
// and grid items.

package layout

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/xo/ivy"
)

// The bounds of the grids.
//
// Port: upstream has no bound. A hostile style, such as grid-row: 1000000
// or repeat(1000000000, 1px), makes a grid larger than the memory, or a
// placement that never ends.
const (
	// gridMaxLine bounds the numbers and the spans of the lines of the
	// grid items, as the clamping of the grid that the specification allows.
	gridMaxLine = 10000
	// gridMaxTracks bounds the tracks of a template, with its repetitions.
	gridMaxTracks = 10000
	// gridMaxImplicitTracks bounds the tracks of the implicit grid on each
	// axis. A larger grid stops the layout with an error that wraps
	// ivy.ErrLimit.
	gridMaxImplicitTracks = 4 * gridMaxTracks
	// gridMaxSearch bounds the steps of a search of a free place for an
	// item, past the extent of the grid.
	gridMaxSearch = 4 * gridMaxLine
)

// gridPlaceLine is a value of grid-row-start, grid-row-end, grid-column-start or
// grid-column-end: auto, or the tuple (span, number, ident) of upstream.
type gridPlaceLine struct {
	// auto is set for the value auto.
	auto bool
	// span is set when the line is a span, the 'span' of upstream.
	span bool
	// number is the number of the line, and hasNumber is false for None.
	number    int
	hasNumber bool
	// ident is the name of the line, or "" for None.
	ident string
}

// gridParseLine reads a computed value of a grid line.
func gridParseLine(v Value) gridPlaceLine {
	t, ok := v.(Tuple)
	if !ok || len(t) != 3 {
		// Port: any value that is not a tuple of three items is auto.
		return gridPlaceLine{auto: true}
	}
	var line gridPlaceLine
	line.span = t[0] != nil
	if t[1] != nil {
		// Port: the number is clamped to [-gridMaxLine, gridMaxLine].
		line.number = int(min(max(num(t[1]), -gridMaxLine), gridMaxLine))
		line.hasNumber = true
	}
	if t[2] != nil {
		line.ident = str(t[2])
	}
	return line
}

// gridNumberLine returns the line (None, number, None).
func gridNumberLine(number int) gridPlaceLine {
	return gridPlaceLine{number: number, hasNumber: true}
}

// orOne returns `number or 1` of upstream.
func (l gridPlaceLine) orOne() int {
	if !l.hasNumber || l.number == 0 {
		return 1
	}
	return l.number
}

// gridIsLength is _is_length in weasyprint/layout/grid.py.
func gridIsLength(sizing Value) bool {
	d, ok := sizing.(Dimension)
	return ok && !gridIsFrUnit(d.Unit)
}

// gridIsFr is _is_fr in weasyprint/layout/grid.py.
func gridIsFr(sizing Value) bool {
	d, ok := sizing.(Dimension)
	return ok && gridIsFrUnit(d.Unit)
}

// gridIsFrUnit reports whether a unit is fr, in any case.
func gridIsFrUnit(unit string) bool {
	return len(unit) == 2 && (unit[0] == 'f' || unit[0] == 'F') && (unit[1] == 'r' || unit[1] == 'R')
}

// gridIsFitContent reports whether a sizing function is fit-content(), the
// test function[0] == 'fit-content()' of upstream.
func gridIsFitContent(sizing Value) bool {
	t, ok := sizing.(Tuple)
	return ok && len(t) > 0 && isStr(t[0], "fit-content()")
}

// gridHasAny reports whether the tuple of keywords v holds one of names, as
// set(v) & {names} does in upstream.
func gridHasAny(v Value, names ...string) bool {
	t, ok := v.(Tuple)
	if !ok {
		return isStrIn(v, names...)
	}
	for _, item := range t {
		if isStrIn(item, names...) {
			return true
		}
	}
	return false
}

// gridIntersect is _intersect in weasyprint/layout/grid.py.
func gridIntersect(position1, size1, position2, size2 int) bool {
	return position1 < position2+size2 && position2 < position1+size1
}

// gridArea is the position of a grid item in the grid: (x, y, width,
// height), in tracks.
type gridArea struct {
	x, y, width, height int
}

// gridPositions is the dict children_positions of upstream: the areas of
// the grid items, in the order in which they are placed.
type gridPositions struct {
	order []*Box
	areas map[*Box]gridArea
}

// set places child in area.
func (p *gridPositions) set(child *Box, area gridArea) {
	if _, ok := p.areas[child]; !ok {
		p.order = append(p.order, child)
	}
	p.areas[child] = area
}

// gridIntersectWithChildren is _intersect_with_children in
// weasyprint/layout/grid.py.
func gridIntersectWithChildren(lc *layoutContext, area gridArea, positions *gridPositions) bool {
	lc.spend(1 + len(positions.order)/16)
	for _, child := range positions.order {
		full := positions.areas[child]
		xIntersect := gridIntersect(area.x, area.width, full.x, full.width)
		yIntersect := gridIntersect(area.y, area.height, full.y, full.height)
		if xIntersect && yIntersect {
			return true
		}
	}
	return false
}

// gridGetLine is _get_line in weasyprint/layout/grid.py. lines are the
// names of the lines of the grid. It returns the line, with its number
// changed as upstream changes it, and the coordinate of the line, with
// false for a coordinate of None.
func gridGetLine(line gridPlaceLine, lines [][]string, side string) (gridPlaceLine, int, bool) {
	coord := 0
	if line.ident != "" && !line.span && !line.hasNumber {
		found := false
		for c, names := range lines {
			if slices.Contains(names, line.ident+"-"+side) {
				coord, found = c, true
				break
			}
		}
		if !found {
			line.number, line.hasNumber = 1, true
		}
	}
	if line.hasNumber && !line.span {
		if line.ident == "" {
			coord = line.number - 1
		} else {
			step := -1
			if line.number > 0 {
				step = 1
			}
			n := len(lines)
			// Port: the coordinate of upstream is unbound for no line.
			c := 0
			broke := false
			for k := range n {
				index := k
				if step == -1 {
					index = n - 1 - k
				}
				c = k
				if slices.Contains(lines[index], line.ident) {
					line.number -= step
					broke = true
					break
				}
				if line.number == 0 {
					broke = true
					break
				}
			}
			if !broke {
				c += abs(line.number)
			}
			coord = c
			if step == -1 {
				coord = n - 1 - coord
			}
		}
	}
	return line, coord, !line.span
}

// gridGetPlacement is _get_placement in weasyprint/layout/grid.py. It
// returns the coordinate and the size of the placement of an item between
// the lines start and end, and false for None.
func gridGetPlacement(start, end gridPlaceLine, lines [][]string) (int, int, bool) {
	// Input coordinates are 1-indexed, returned coordinates are 0-indexed.
	if start.auto || start.span {
		if end.auto || end.span {
			return 0, 0, false
		}
	}
	var size int
	var spanIdent string
	coord, hasCoord := 0, false
	if !start.auto {
		var line gridPlaceLine
		line, coord, hasCoord = gridGetLine(start, lines, "start")
		if line.span {
			size = line.orOne()
			spanIdent = line.ident
		}
	} else {
		size = 1
	}
	if !end.auto {
		line, coordEnd, _ := gridGetLine(end, lines, "end")
		if line.span {
			spanNumber := line.orOne()
			size = spanNumber
			spanIdent = line.ident
			if spanIdent != "" {
				// Port: upstream fails on a start of None, which the test
				// at the start of the function excludes.
				lo, hi := gridSlice(coord+1, len(lines), len(lines))
				broke := false
				for k := lo; k < hi; k++ {
					size = k - lo + 1
					if slices.Contains(lines[k], spanIdent) {
						spanNumber--
					}
					if spanNumber == 0 {
						broke = true
						break
					}
				}
				if !broke {
					size += spanNumber
				}
			}
		} else if hasCoord {
			size = coordEnd - coord
		}
		if !hasCoord {
			if spanIdent == "" {
				coord = coordEnd - size
			} else {
				number := line.orOne()
				if coordEnd > 0 {
					first := min(coordEnd-1, len(lines)-1)
					found := false
					for k := 0; first-k >= 0; k++ {
						if slices.Contains(lines[first-k], spanIdent) {
							number--
						}
						if number == 0 {
							coord = coordEnd - 1 - k
							found = true
							break
						}
					}
					if !found {
						coord = -number
					}
				} else {
					coord = -number
				}
			}
			size = coordEnd - coord
		}
	} else {
		size = 1
	}
	if size < 0 {
		size = -size
		coord -= size
	}
	if size == 0 {
		size = 1
	}
	return coord, size, true
}

// gridGetSpan is _get_span in weasyprint/layout/grid.py.
func gridGetSpan(place gridPlaceLine) int {
	// TODO: Handle lines.
	span := 1
	if !place.auto && place.span {
		span = place.orOne()
	}
	return span
}

// gridGetSecondPlacement is _get_second_placement in
// weasyprint/layout/grid.py. It returns the placement on the second axis
// of an item placed at (firstCoord, firstSize) on the first axis.
//
// Port: upstream gives the whole list of the tracks, with their sizes, to
// the search of the named lines. The port gives the names of the lines.
func gridGetSecondPlacement(lc *layoutContext, firstCoord, firstSize int, secondStart, secondEnd gridPlaceLine, secondLines [][]string, positions *gridPositions, firstFlow string, dense bool) (int, int) {
	occupiedTracks := map[int]bool{}
	for _, child := range positions.order {
		a := positions.areas[child]
		// Test whether cells overlap.
		if firstFlow == "row" {
			if gridIntersect(a.y, a.height, firstCoord, firstSize) {
				lc.spend(1 + a.width/64)
				for x := a.x; x < a.x+a.width; x++ {
					occupiedTracks[x] = true
				}
			}
		} else {
			if gridIntersect(a.x, a.width, firstCoord, firstSize) {
				lc.spend(1 + a.height/64)
				for y := a.y; y < a.y+a.height; y++ {
					occupiedTracks[y] = true
				}
			}
		}
	}
	maxOccupied := 0
	if len(occupiedTracks) > 0 {
		maxOccupied = math.MinInt
		for track := range occupiedTracks {
			maxOccupied = max(maxOccupied, track)
		}
	}
	if dense {
		var coord, size int
		for track := 0; ; track++ {
			lc.spend(1)
			if occupiedTracks[track] {
				continue
			}
			if secondStart.auto {
				coord, size, _ = gridGetPlacement(gridNumberLine(track+1), secondEnd, secondLines)
			} else {
				// If the placement contains two spans, remove the one
				// contributed by the end grid-placement property.
				// https://drafts.csswg.org/css-grid/#grid-placement-errors
				//
				// Port: upstream asserts that the start is a span.
				span := gridGetSpan(secondStart)
				coord, size, _ = gridGetPlacement(secondStart, gridNumberLine(track+1+span), secondLines)
			}
			free := true
			for t := coord; t < coord+size; t++ {
				if occupiedTracks[t] {
					free = false
					break
				}
			}
			// Port: upstream searches without end. The port stops past the
			// occupied tracks, and takes the last placement.
			if free || track > maxOccupied+gridMaxSearch {
				return coord, size
			}
		}
	}
	track := maxOccupied + 1
	if secondStart.auto {
		coord, size, _ := gridGetPlacement(gridNumberLine(track+1), secondEnd, secondLines)
		return coord, size
	}
	// If the placement contains two spans, remove the one contributed by
	// the end grid-placement property.
	// https://drafts.csswg.org/css-grid/#grid-placement-errors
	//
	// Port: upstream asserts that the start is a span.
	for endTrack := track + 1; ; endTrack++ {
		lc.spend(1)
		coord, size, _ := gridGetPlacement(secondStart, gridNumberLine(endTrack+1), secondLines)
		if coord >= track {
			return coord, size
		}
		// Port: upstream searches without end, for a span of a name that
		// no line has. The port stops and places the item at the track.
		if endTrack > track+gridMaxSearch {
			return track, size
		}
	}
}

// gridGetSizingFunctions is _get_sizing_functions in
// weasyprint/layout/grid.py: the min and the max sizing functions of a
// track size.
func gridGetSizingFunctions(size Value) [2]Value {
	minSizing, maxSizing := size, size
	if t, ok := size.(Tuple); ok && len(t) == 3 && isStr(t[0], "minmax()") {
		minSizing, maxSizing = t[1], t[2]
	}
	if gridIsFitContent(minSizing) || gridIsFr(minSizing) {
		minSizing = String("auto")
	}
	return [2]Value{minSizing, maxSizing}
}

// gridRepeatNumber returns the number of repetitions of repeat().
//
// Port: a number that is not an integer of at least 1, which upstream
// fails to use, repeats nothing. The number is at most gridMaxTracks.
func gridRepeatNumber(number Value) int {
	if n, ok := number.(Int); ok {
		return int(min(max(n, 0), gridMaxTracks))
	}
	return 0
}

// gridGetAutoRepeatNumber is _get_auto_repeat_number in
// weasyprint/layout/grid.py: the number of repetitions for an auto-fill
// repeat(). tracks are the track sizes of the template.
//
// See https://drafts.csswg.org/css-grid/#auto-repeat.
func gridGetAutoRepeatNumber(tracks []Value, boxSize, gap float64, style *Style) int {
	if boxSize == Auto {
		// TODO: Use max-width/-height and min-width/-height.
		return 1
	}

	size := func(track Value) (float64, bool) {
		// Each track counts as its max sizing function if it is definite,
		// or as its min sizing function if it is definite. If both are
		// definite, the max sizing function is floored by the min sizing
		// function.
		functions := gridGetSizingFunctions(track)
		result, found := 0.0, false
		for _, sizing := range functions {
			if !gridIsLength(sizing) {
				continue
			}
			// Port: a length that percentage cannot resolve is not
			// definite. Upstream fails to compare None.
			if v, ok := percentageFloat(sizing, style, boxSize); ok {
				if !found || v > result {
					result = v
				}
				found = true
			}
		}
		return result, found
	}

	repeatSize := 0.0
	for _, track := range tracks {
		if t, ok := track.(Tuple); ok && len(t) == 3 && isStr(t[0], "repeat()") {
			number := t[1]
			repeatTracks, _ := t[2].(Tuple)
			sum, count := 0.0, 0
			for i := 1; i < len(repeatTracks); i += 2 {
				s, ok := size(repeatTracks[i])
				if !ok {
					// Neither sizing function is definite.
					return 1
				}
				sum += s
				count++
			}
			if isStrIn(number, "auto-fill", "auto-fit") {
				repeatSize = sum + float64(count)*gap
			} else {
				boxSize -= float64(gridRepeatNumber(number)) * (sum + float64(count)*gap)
			}
		} else {
			trackSize, ok := size(track)
			if !ok {
				// Neither sizing function is definite.
				return 1
			}
			boxSize -= trackSize + gap
		}
	}
	if repeatSize == 0 {
		return 1
	}
	repetitions := (boxSize + gap) / repeatSize
	// Port: upstream fails on a number of repetitions that is not a number
	// or that is infinite. The number is at most gridMaxTracks.
	if !(repetitions >= 1) {
		return 1
	}
	return max(1, int(min(repetitions, gridMaxTracks)))
}

// gridTracks is the list of the tracks of upstream, which alternates the
// names of the lines and the sizes of the tracks: [names, size, names, ...,
// size, names]. lines holds the names, and sizes the sizes, so that
// lines[i] is rows[2*i] and sizes[i] is rows[2*i+1] of upstream.
type gridTracks struct {
	lines [][]string
	sizes []Value
}

// appendTrack is tracks.append(size) and tracks.append([]).
func (t *gridTracks) appendTrack(size Value) {
	t.sizes = append(t.sizes, size)
	t.lines = append(t.lines, nil)
}

// prependTrack is tracks.insert(0, size) and tracks.insert(0, []).
func (t *gridTracks) prependTrack(size Value) {
	t.sizes = slices.Insert(t.sizes, 0, size)
	t.lines = slices.Insert(t.lines, 0, []string(nil))
}

// addNames is the part of the line names of _get_template_tracks: it adds
// the names to the last line, or starts a new line.
func (t *gridTracks) addNames(names Value) {
	tuple, _ := names.(Tuple)
	list := make([]string, 0, len(tuple))
	for _, name := range tuple {
		list = append(list, str(name))
	}
	if len(t.lines) > len(t.sizes) {
		t.lines[len(t.lines)-1] = append(t.lines[len(t.lines)-1], list...)
	} else {
		t.lines = append(t.lines, list)
	}
}

// addSize is the part of the track sizes of _get_template_tracks.
//
// Port: a size that follows a size, which upstream keeps, gets an empty
// line between them, so that lines and sizes stay in step.
func (t *gridTracks) addSize(size Value) {
	if len(t.lines) == len(t.sizes) {
		t.lines = append(t.lines, nil)
	}
	t.sizes = append(t.sizes, size)
}

// gridHasName reports whether one of the lines has the name.
func gridHasName(lines [][]string, name string) bool {
	for _, names := range lines {
		if slices.Contains(names, name) {
			return true
		}
	}
	return false
}

// gridGetTemplateTracks is _get_template_tracks in
// weasyprint/layout/grid.py: the tracks of grid-template-rows or of
// grid-template-columns, with the repetitions of repeat().
func gridGetTemplateTracks(lc *layoutContext, tracks Value, boxSize, gap float64, style *Style) *gridTracks {
	list, _ := tracks.(Tuple)
	if isStr(tracks, "none") {
		list = Tuple{Tuple{}}
	}
	result := &gridTracks{}
	for _, track := range list {
		if isStr(track, "subgrid") {
			// TODO: Support subgrids.
			lc.log.warning("Subgrids are unsupported")
			return &gridTracks{lines: [][]string{nil}}
		}
	}
	var templateSizes []Value
	for i := 1; i < len(list); i += 2 {
		templateSizes = append(templateSizes, list[i])
	}
	for i, track := range list {
		if i%2 == 0 {
			// Line names.
			result.addNames(track)
			continue
		}
		// Track size.
		t, ok := track.(Tuple)
		if !ok || len(t) != 3 || !isStr(t[0], "repeat()") {
			// Port: the tracks of a template are at most gridMaxTracks.
			if len(result.sizes) < gridMaxTracks {
				result.addSize(track)
			}
			continue
		}
		repeatNumber := 0
		switch {
		case isStr(t[1], "auto-fill"):
			repeatNumber = gridGetAutoRepeatNumber(templateSizes, boxSize, gap, style)
		case isStr(t[1], "auto-fit"):
			// TODO: Respect auto-fit.
			lc.log.warning(`"auto-fit" is unsupported in repeat()`)
			repeatNumber = 1
		default:
			repeatNumber = gridRepeatNumber(t[1])
		}
		repeatTrackList, _ := t[2].(Tuple)
		// Port: the tracks of a template are at most gridMaxTracks.
		for range repeatNumber {
			if len(result.sizes) >= gridMaxTracks {
				break
			}
			lc.spend(1)
			for j, repeatTrack := range repeatTrackList {
				if j%2 == 1 {
					// Track size in repeat.
					result.addSize(repeatTrack)
				} else {
					// Line names in repeat.
					result.addNames(repeatTrack)
				}
			}
		}
	}
	if len(result.lines) == len(result.sizes) {
		// Port: the list of upstream ends with names. The port adds empty
		// names when it does not.
		result.lines = append(result.lines, nil)
	}
	return result
}

// gridDistributeExtraSpaceToTracks is _distribute_extra_space_to_tracks in
// weasyprint/layout/grid.py.
func gridDistributeExtraSpaceToTracks(space float64, tracksSizes [][2]float64, tracksNumbers []int, sizeIndex int, itemIncurredIncreases []float64, beyondLimits bool) float64 {
	distributedSpace := space / float64(max(len(tracksNumbers), 1))
	for _, trackNumber := range tracksNumbers {
		itemIncurredIncrease := distributedSpace
		affectedSize := tracksSizes[trackNumber][sizeIndex]
		limit := tracksSizes[trackNumber][1]
		if beyondLimits {
			limit = math.Inf(1)
		}
		if affectedSize+itemIncurredIncrease >= limit {
			extra := itemIncurredIncrease + affectedSize - limit
			itemIncurredIncrease -= extra
		}
		space -= itemIncurredIncrease
		itemIncurredIncreases[trackNumber] = itemIncurredIncrease
	}
	return space
}

// gridSpanItem is an item of the tracks children of the spanning items: a
// grid item and the parent that its layout uses.
type gridSpanItem struct {
	child, parent *Box
}

// gridDistributeExtraSpace is _distribute_extra_space in
// weasyprint/layout/grid.py. affectedSizes is "min" or "max",
// affectedTracksTypes is "intrinsic", "content-based" or "max-content",
// sizeContribution is "minimum", "min-content" or "max-content", and
// direction is 'x' or 'y'.
func gridDistributeExtraSpace(lc *layoutContext, affectedSizes, affectedTracksTypes, sizeContribution string, tracksChildren [][]gridSpanItem, sizingFunctions [][2]Value, tracksSizes [][2]float64, span int, direction byte) {
	// 1. Maintain separately for each affected track a planned increase.
	plannedIncreases := make([]float64, len(tracksSizes))

	// 2. Distribute space.
	var affectedTracks []bool
	affectedSizeIndex := 0
	if affectedSizes == "max" {
		affectedSizeIndex = 1
	}
	currentSpan := 0
	for k := range min(len(tracksChildren), len(sizingFunctions)) {
		if len(tracksChildren[k]) > 0 {
			currentSpan = span
		}
		if currentSpan == 0 {
			affectedTracks = append(affectedTracks, false)
			continue
		}
		currentSpan--
		function := sizingFunctions[k][affectedSizeIndex]
		affected := false
		switch affectedTracksTypes {
		case "intrinsic":
			affected = isStrIn(function, "min-content", "max-content", "auto") || gridIsFitContent(function)
		case "content-based":
			affected = isStrIn(function, "min-content", "max-content")
		case "max-content":
			affected = isStrIn(function, "max-content", "auto")
		}
		affectedTracks = append(affectedTracks, affected)
	}
	for i, children := range tracksChildren {
		for _, item := range children {
			lc.spend(1)
			// 2.1 Find the space distribution.
			// TODO: Differentiate minimum and min-content values.
			// TODO: Find a better way to get height.
			var space float64
			if direction == 'x' {
				if sizeContribution == "minimum" || sizeContribution == "min-content" {
					space = minContentWidth(lc, item.child, true)
				} else {
					space = maxContentWidth(lc, item.child, true)
				}
			} else {
				child := item.child.DeepCopy()
				child.PositionX = 0
				child.PositionY = 0
				lc.createBlockFormattingContext(item.parent)
				result := blockLevelLayout(lc, child, math.Inf(-1), nil, item.parent,
					true, &[]*Box{}, nil, nil, nil, nil, false, noMaxLines)
				lc.finishBlockFormattingContext(nil)
				// Port: upstream fails on an item that gives no box.
				if result.box != nil {
					space = result.box.MarginHeight()
				}
			}
			lo, hi := gridSlice(i, i+span, len(tracksSizes))
			for _, sizes := range tracksSizes[lo:hi] {
				space -= sizes[affectedSizeIndex]
			}
			space = max(0, space)
			// 2.2 Distribute space up to limits.
			itemIncurredIncreases := make([]float64, len(sizingFunctions))
			var affectedTracksNumbers, unaffectedTracksNumbers []int
			lo, hi = gridSlice(i, i+span, len(affectedTracks))
			for j := lo; j < hi; j++ {
				if affectedTracks[j] {
					affectedTracksNumbers = append(affectedTracksNumbers, j)
				} else {
					unaffectedTracksNumbers = append(unaffectedTracksNumbers, j)
				}
			}
			space = gridDistributeExtraSpaceToTracks(
				space, tracksSizes, affectedTracksNumbers, affectedSizeIndex,
				itemIncurredIncreases, false)
			// 2.3 Distribute space to non-affected tracks.
			if space != 0 && len(affectedTracksNumbers) > 0 {
				space = gridDistributeExtraSpaceToTracks(
					space, tracksSizes, unaffectedTracksNumbers, affectedSizeIndex,
					itemIncurredIncreases, false)
			}
			// 2.4 Distribute space beyond limits.
			if space != 0 {
				// TODO: Filter tracks with right track sizing functions.
				gridDistributeExtraSpaceToTracks(
					space, tracksSizes, affectedTracksNumbers,
					affectedSizeIndex, itemIncurredIncreases, true)
			}
			// 2.5. Set the track’s planned increase.
			for k, extra := range itemIncurredIncreases {
				if k < len(plannedIncreases) && extra > plannedIncreases[k] {
					plannedIncreases[k] = extra
				}
			}
		}
	}

	// 3. Update the tracks’ affected size.
	for k, affected := range affectedTracks {
		if !affected || k >= len(tracksSizes) {
			continue
		}
		if affectedSizes == "max" && math.IsInf(tracksSizes[k][1], 1) {
			tracksSizes[k][1] = tracksSizes[k][0] + plannedIncreases[k]
		} else {
			tracksSizes[k][affectedSizeIndex] += plannedIncreases[k]
		}
	}
}

// gridAnonymousParent is boxes.BlockContainerBox.anonymous_from(source,
// ()) of upstream: an anonymous block container that inherits from source.
//
// Port: upstream makes an instance of the abstract class
// BlockContainerBox. The port makes a block box. A source that is a
// sizeBlock, which has no style, gives the box of fallback.
func gridAnonymousParent(source, fallback *Box) *Box {
	if source == nil || !source.isBox() || source.Style == nil {
		source = fallback
	}
	return anonymousFrom(KindBlock, source, nil)
}

// gridResolveTracksSizes is _resolve_tracks_sizes in
// weasyprint/layout/grid.py. It returns the base size and the growth limit
// of each track. direction is 'x' or 'y'.
func gridResolveTracksSizes(lc *layoutContext, sizingFunctions [][2]Value, boxSize float64, positions *gridPositions, implicitStart int, direction byte, gap float64, containingBlock *Box, orthogonalSizes []float64) [][2]float64 {
	tracksSizes := make([][2]float64, 0, len(sizingFunctions))
	// TODO: Check that auto box size is 0 for percentages.
	percentBoxSize := boxSize
	if boxSize == Auto {
		percentBoxSize = 0
	}
	// 1.1 Initialize track sizes.
	for _, functions := range sizingFunctions {
		baseSize := 0.0
		if gridIsLength(functions[0]) {
			baseSize, _ = percentageFloat(functions[0], containingBlock.Style, percentBoxSize)
		}
		growthLimit := math.Inf(1)
		if gridIsLength(functions[1]) {
			// Port: a length that percentage cannot resolve has no limit.
			// Upstream fails to compare None.
			if v, ok := percentageFloat(functions[1], containingBlock.Style, percentBoxSize); ok {
				growthLimit = v
			}
		}
		growthLimit = max(baseSize, growthLimit)
		tracksSizes = append(tracksSizes, [2]float64{baseSize, growthLimit})
	}

	coordAndSize := func(a gridArea) (int, int) {
		if direction == 'x' {
			return a.x, a.width
		}
		return a.y, a.height
	}

	// 1.2 Resolve intrinsic track sizes.
	// 1.2.1 Shim baseline-aligned items.
	// TODO: Shim items.
	// 1.2.2 Size tracks to fit non-spanning items.
	tracksChildren := make([][]*Box, len(tracksSizes))
	for _, child := range positions.order {
		coord, size := coordAndSize(positions.areas[child])
		if size != 1 {
			continue
		}
		// Port: upstream raises IndexError for an item out of the tracks.
		if k, ok := gridPyIndex(coord-implicitStart, len(tracksChildren)); ok {
			tracksChildren[k] = append(tracksChildren[k], child)
		}
	}
	for k, children := range tracksChildren {
		if len(children) == 0 || k >= len(sizingFunctions) {
			continue
		}
		minFunction, maxFunction := sizingFunctions[k][0], sizingFunctions[k][1]
		sizes := &tracksSizes[k]
		var minSize, maxSize float64
		if direction == 'y' {
			// TODO: Find a better way to get height.
			height := 0.0
			for _, child := range children {
				lc.spend(1)
				a := positions.areas[child]
				width := gridSum(orthogonalSizes, a.x, a.x+a.width)
				child = child.DeepCopy()
				child.PositionX = 0
				child.PositionY = 0
				parent := gridAnonymousParent(containingBlock, containingBlock)
				resolvePercentages(parent, containingBlock)
				parent.PositionX = child.PositionX
				parent.PositionY = child.PositionY
				parent.Width = width
				parent.Height = height
				bottomSpace := math.Inf(-1)
				result := blockLevelLayout(lc, child, bottomSpace, nil, parent,
					true, &[]*Box{}, nil, nil, nil, nil, false, noMaxLines)
				// Port: upstream fails on an item that gives no box.
				if result.box != nil {
					height = max(height, result.box.MarginHeight())
				}
			}
			// TODO: Get real min/max contributions.
			minSize, maxSize = height, height
		} else {
			for _, child := range children {
				lc.spend(1)
				minSize = max(minSize, minContentWidth(lc, child, true))
				maxSize = max(maxSize, maxContentWidth(lc, child, true))
			}
		}
		switch {
		case isStr(minFunction, "min-content"):
			sizes[0] = minSize
		case isStr(minFunction, "max-content"):
			sizes[0] = maxSize
		case isStr(minFunction, "auto"):
			// TODO: Handle min-/max-content constrained parents.
			// TODO: Use real "minimum contributions".
			sizes[0] = min(minSize, sizes[1])
		}
		if isStr(maxFunction, "min-content") {
			sizes[1] = minSize
		} else if isStrIn(maxFunction, "auto", "max-content") || gridIsFitContent(maxFunction) {
			sizes[1] = maxSize
		}
		sizes[1] = max(sizes[0], sizes[1])
	}
	// 1.2.3 Increase sizes to accommodate items spanning content-sized
	// tracks.
	spanSet := map[int]bool{}
	for _, child := range positions.order {
		if _, size := coordAndSize(positions.areas[child]); size >= 2 {
			spanSet[size] = true
		}
	}
	spans := slices.Sorted(maps.Keys(spanSet))
	for _, span := range spans {
		spanChildren := make([][]gridSpanItem, len(sizingFunctions))
		for i, child := range positions.order {
			lc.spend(1)
			a := positions.areas[child]
			coord, size := coordAndSize(a)
			if size != span {
				continue
			}
			hasFr := false
			lo, hi := gridSlice(i, i+span+1, len(sizingFunctions))
			for _, functions := range sizingFunctions[lo:hi] {
				if gridIsFr(functions[1]) {
					hasFr = true
					break
				}
			}
			if hasFr {
				continue
			}
			parent := gridAnonymousParent(containingBlock, containingBlock)
			resolvePercentages(parent, containingBlock)
			if direction == 'y' {
				parent.Width = gridSum(orthogonalSizes, a.x, a.x+a.width)
			}
			// Port: upstream raises IndexError for an item out of the
			// tracks.
			if k, ok := gridPyIndex(coord-implicitStart, len(spanChildren)); ok {
				spanChildren[k] = append(spanChildren[k], gridSpanItem{child, parent})
			}
		}
		// 1.2.3.1 For intrinsic minimums.
		// TODO: Respect min-/max-content constraint.
		gridDistributeExtraSpace(lc, "min", "intrinsic", "minimum", spanChildren,
			sizingFunctions, tracksSizes, span, direction)
		// 1.2.3.2 For content-based minimums.
		gridDistributeExtraSpace(lc, "min", "content-based", "min-content", spanChildren,
			sizingFunctions, tracksSizes, span, direction)
		// 1.2.3.3 For max-content minimums.
		// TODO: Respect max-content constraint.
		gridDistributeExtraSpace(lc, "min", "max-content", "max-content", spanChildren,
			sizingFunctions, tracksSizes, span, direction)
		// 1.2.3.4 Increase growth limit.
		// TODO: Increase growth limit.
		// 1.2.3.5 For intrinsic maximums.
		gridDistributeExtraSpace(lc, "max", "intrinsic", "min-content", spanChildren,
			sizingFunctions, tracksSizes, span, direction)
		// 1.2.3.6 For max-content maximums.
		gridDistributeExtraSpace(lc, "max", "max-content", "max-content", spanChildren,
			sizingFunctions, tracksSizes, span, direction)
	}
	// 1.2.4 Increase sizes to accommodate items spanning flexible tracks.
	// TODO: Support spans for flexible tracks.
	// 1.2.5 Fix infinite growth limits.
	for k := range tracksSizes {
		if math.IsInf(tracksSizes[k][1], 1) {
			tracksSizes[k][1] = tracksSizes[k][0]
		}
	}
	// 1.3 Maximize tracks.
	var freeSpace float64
	if boxSize == Auto {
		// TODO: detect if the grid container is sized under min-content
		// constraint.
		freeSpace = math.Inf(1)
	} else {
		freeSpace = boxSize - gridSumBase(tracksSizes, 0, len(tracksSizes)) -
			float64(len(tracksSizes)-1)*gap
	}
	// Port: upstream divides by zero for no track.
	if freeSpace > 0 && len(tracksSizes) > 0 {
		distributedFreeSpace := freeSpace / float64(len(tracksSizes))
		for k := range tracksSizes {
			baseSize, growthLimit := tracksSizes[k][0], tracksSizes[k][1]
			if baseSize+distributedFreeSpace > growthLimit {
				tracksSizes[k][0] = growthLimit
				freeSpace -= growthLimit - baseSize
			} else {
				tracksSizes[k][0] += distributedFreeSpace
				freeSpace -= distributedFreeSpace
			}
		}
	}
	// TODO: Respect max-width/-height.
	// 1.4 Expand flexible tracks.
	inflexibleTracks := map[int]bool{}
	var flexFraction float64
	switch {
	case freeSpace <= 0:
		// TODO: Respect min-content constraint.
		flexFraction = 0
	case freeSpace < math.Inf(1):
		var hypotheticalFrSize float64
		// Port: each pass makes one more track inflexible, or stops. The
		// passes are at most one more than the tracks.
		for pass := 0; pass <= len(tracksSizes); pass++ {
			lc.spend(1)
			leftoverSpace := freeSpace
			flexFactorSum := 0.0
			for k, functions := range sizingFunctions {
				if k >= len(tracksSizes) {
					break
				}
				if d, ok := functions[1].(Dimension); ok && gridIsFr(d) {
					leftoverSpace += tracksSizes[k][0]
					if !inflexibleTracks[k] {
						flexFactorSum += d.Value
					}
				}
			}
			flexFactorSum = max(1, flexFactorSum)
			hypotheticalFrSize = leftoverSpace / flexFactorSum
			stop := true
			for k, functions := range sizingFunctions {
				if k >= len(tracksSizes) {
					break
				}
				if d, ok := functions[1].(Dimension); ok && !inflexibleTracks[k] && gridIsFr(d) {
					if hypotheticalFrSize*d.Value < tracksSizes[k][0] {
						inflexibleTracks[k] = true
						freeSpace -= tracksSizes[k][0]
						stop = freeSpace > 0
					}
				}
			}
			if stop {
				break
			}
		}
		flexFraction = hypotheticalFrSize
	default:
		flexFraction = 0
		for k, functions := range sizingFunctions {
			if k >= len(tracksSizes) {
				break
			}
			if d, ok := functions[1].(Dimension); ok && gridIsFr(d) {
				if d.Value > 1 {
					flexFraction = max(flexFraction, d.Value*tracksSizes[k][0])
				} else {
					flexFraction = max(flexFraction, tracksSizes[k][0])
				}
			}
		}
		// TODO: Respect grid items max-content contribution.
		// TODO: Respect min-* constraint.
	}
	for k, functions := range sizingFunctions {
		if k >= len(tracksSizes) {
			break
		}
		if d, ok := functions[1].(Dimension); ok && gridIsFr(d) && !inflexibleTracks[k] {
			if flexFraction*d.Value > tracksSizes[k][0] {
				freeSpace -= flexFraction * d.Value
				tracksSizes[k][0] = flexFraction * d.Value
			}
		}
	}
	// 1.5 Expand stretched auto tracks.
	justifyContent := containingBlock.Style.Get("justify_content")
	alignContent := containingBlock.Style.Get("align_content")
	xStretch := direction == 'x' && gridHasAny(justifyContent, "normal", "stretch")
	yStretch := direction == 'y' && gridHasAny(alignContent, "normal", "stretch")
	if (xStretch || yStretch) && 0 < freeSpace && freeSpace < math.Inf(1) {
		var autoTracks []int
		for k, functions := range sizingFunctions {
			if k < len(tracksSizes) && isStr(functions[1], "auto") {
				autoTracks = append(autoTracks, k)
			}
		}
		if len(autoTracks) > 0 {
			distributedFreeSpace := freeSpace / float64(len(autoTracks))
			for _, k := range autoTracks {
				tracksSizes[k][0] += distributedFreeSpace
			}
		}
	}

	return tracksSizes
}

// gridSlice returns the bounds of the slice [start:end] of Python in a
// list of length n.
func gridSlice(start, end, n int) (int, int) {
	norm := func(i int) int {
		if i < 0 {
			i += n
		}
		return min(max(i, 0), n)
	}
	start, end = norm(start), norm(end)
	return start, max(start, end)
}

// gridPyIndex returns the index of Python i in a list of length n, and
// false when it is out of the list.
func gridPyIndex(i, n int) (int, bool) {
	if i < 0 {
		i += n
	}
	return i, i >= 0 && i < n
}

// gridSum returns sum(list[start:end]) of Python.
func gridSum(list []float64, start, end int) float64 {
	lo, hi := gridSlice(start, end, len(list))
	sum := 0.0
	for _, v := range list[lo:hi] {
		sum += v
	}
	return sum
}

// gridSumBase returns the sum of the base sizes of sizes[start:end] of
// Python, sum(size for size, _ in sizes[start:end]).
func gridSumBase(sizes [][2]float64, start, end int) float64 {
	lo, hi := gridSlice(start, end, len(sizes))
	sum := 0.0
	for _, size := range sizes[lo:hi] {
		sum += size[0]
	}
	return sum
}

// gridLenSlice returns len(list[start:end]) of Python for a list of length
// n.
func gridLenSlice(start, end, n int) int {
	lo, hi := gridSlice(start, end, n)
	return hi - lo
}

// gridAt returns list[i] of Python, or fallback when i is out of the list.
func gridAt(list []float64, i int, fallback float64) float64 {
	if k, ok := gridPyIndex(i, len(list)); ok {
		return list[k]
	}
	// Port: upstream raises IndexError.
	return fallback
}

// gridCycle is itertools.cycle of a tuple of track sizes.
type gridCycle struct {
	values Tuple
	i      int
}

// newGridCycle returns a cycle of the tuple v, reversed when reverse is
// set.
//
// Port: upstream never ends next() on an empty tuple. An empty tuple
// cycles auto.
func newGridCycle(v Value, reverse bool) *gridCycle {
	t, _ := v.(Tuple)
	if len(t) == 0 {
		t = Tuple{String("auto")}
	}
	if reverse {
		t = slices.Clone(t)
		slices.Reverse(t)
	}
	return &gridCycle{values: t}
}

// next returns the next value of the cycle.
func (c *gridCycle) next() Value {
	v := c.values[c.i%len(c.values)]
	c.i++
	return v
}

// gridAdvancement is a value of the advancements of a grid container, with
// its rank in the order of insertion of the dict of upstream.
type gridAdvancement struct {
	value float64
	seq   int
}

// gridAdvancementKey is a key of the advancements of a grid container, the
// tuple (x, y) of upstream.
type gridAdvancementKey [2]int

// gridAdvancements returns the advancements of a grid container, the dict
// box.advancements of upstream. The map maps a gridAdvancementKey to a
// gridAdvancement. The copies of the box share it, as they share the dict
// of upstream.
func gridAdvancements(box *Box) map[any]any {
	if box.extra().Advancements == nil {
		box.ownExtra().Advancements = map[any]any{}
	}
	return box.extra().Advancements
}

// gridAdvancementGet returns advancements.get((x, y)), and false for None.
func gridAdvancementGet(advancements map[any]any, x, y int) (float64, bool) {
	a, ok := advancements[gridAdvancementKey{x, y}].(gridAdvancement)
	return a.value, ok
}

// gridAdvancementSet is advancements[x, y] = value.
func gridAdvancementSet(advancements map[any]any, x, y int, value float64) {
	key := gridAdvancementKey{x, y}
	if a, ok := advancements[key].(gridAdvancement); ok {
		a.value = value
		advancements[key] = a
		return
	}
	advancements[key] = gridAdvancement{value: value, seq: len(advancements)}
}

// gridAdvancementEntry is an item of the advancements of a grid container.
type gridAdvancementEntry struct {
	key   gridAdvancementKey
	value float64
}

// gridAdvancementItems returns advancements.items(), in the order of
// insertion.
func gridAdvancementItems(advancements map[any]any) []gridAdvancementEntry {
	type seqEntry struct {
		entry gridAdvancementEntry
		seq   int
	}
	var list []seqEntry
	for k, v := range advancements {
		key, ok1 := k.(gridAdvancementKey)
		a, ok2 := v.(gridAdvancement)
		if ok1 && ok2 {
			list = append(list, seqEntry{gridAdvancementEntry{key, a.value}, a.seq})
		}
	}
	slices.SortFunc(list, func(a, b seqEntry) int { return cmp.Compare(a.seq, b.seq) })
	entries := make([]gridAdvancementEntry, len(list))
	for i, e := range list {
		entries[i] = e.entry
	}
	return entries
}

// gridPageBreaks are the breaks of a row of the grid, the dict of
// page_breaks_by_row of upstream.
type gridPageBreaks struct {
	before, after, inside string
}

// gridLayout is grid_layout in weasyprint/layout/grid.py: the layout of a
// grid container.
func gridLayout(lc *layoutContext, box *Box, bottomSpace float64, skip skipStack, containingBlock *Box, pageIsEmpty bool, absoluteBoxes, fixedBoxes *[]*Box) blockLayoutResult {
	lc.enter()
	defer lc.leave()
	lc.createGridFormattingContext(box)
	defer lc.finishGridFormattingContext()

	noBox := blockLayoutResult{nextPage: anyNextPage(), adjoiningMargins: []float64{}, maxLines: noMaxLines}

	if len(skip) > 0 && !isStr(box.Style.Get("box_decoration_break"), "clone") {
		box.RemoveDecoration(true, false)
	}

	if isStr(box.Style.Get("position"), "relative") || absoluteBoxes == nil {
		// New containing block, use a new absolute list
		absoluteBoxes = &[]*Box{}
	}

	// Define explicit grid
	style := box.Style
	flow := style.Get("grid_auto_flow")
	autoRows := newGridCycle(style.Get("grid_auto_rows"), false)
	autoColumns := newGridCycle(style.Get("grid_auto_columns"), false)
	autoRowsBack := newGridCycle(style.Get("grid_auto_rows"), true)
	autoColumnsBack := newGridCycle(style.Get("grid_auto_columns"), true)
	columnGap := 0.0
	if gap := style.Get("column_gap"); !isStr(gap, "normal") {
		referTo := box.Width
		if box.Width == Auto {
			referTo = containingBlock.Width
		}
		columnGap, _ = percentageFloat(gap, style, referTo)
	}
	rowGap := 0.0
	if gap := style.Get("row_gap"); !isStr(gap, "normal") {
		referTo := box.Height
		if box.Height == Auto {
			referTo = 0
		}
		rowGap, _ = percentageFloat(gap, style, referTo)
	}

	// A name of an area is "" for None.
	var gridAreas [][]string
	if areas, ok := style.Get("grid_template_areas").(Tuple); ok {
		for _, row := range areas {
			cells, _ := row.(Tuple)
			names := make([]string, len(cells))
			for i, cell := range cells {
				if cell != nil {
					names[i] = str(cell)
				}
			}
			gridAreas = append(gridAreas, names)
		}
	}
	if len(gridAreas) == 0 {
		gridAreas = [][]string{{""}}
	}

	// 2. Find the size of the grid container.

	if box.Kind == KindGrid {
		blockLevelWidth(box, containingBlock)
	} else {
		// Port: upstream asserts that the box is an InlineGridBox.
		inlineBlockWidth(box, lc, containingBlock)
	}
	if box.Width == Auto {
		// TODO: Calculate max-width.
		box.Width = containingBlock.Width
	}

	rows := gridGetTemplateTracks(lc, style.Get("grid_template_rows"), box.Height, rowGap, style)
	columns := gridGetTemplateTracks(lc, style.Get("grid_template_columns"), box.Width, columnGap, style)

	// Adjust rows number
	gridAreasColumns := 0
	if len(gridAreas) > 0 {
		gridAreasColumns = len(gridAreas[0])
	}
	if rowsDiff := len(rows.sizes) - len(gridAreas); rowsDiff > 0 {
		for range rowsDiff {
			gridAreas = append(gridAreas, make([]string, gridAreasColumns))
		}
	} else if rowsDiff < 0 {
		for range -rowsDiff {
			rows.appendTrack(autoRows.next())
		}
	}

	// Adjust columns number
	if columnsDiff := len(columns.sizes) - gridAreasColumns; columnsDiff > 0 {
		for i := range gridAreas {
			for range columnsDiff {
				gridAreas[i] = append(gridAreas[i], "")
			}
		}
	} else if columnsDiff < 0 {
		for range -columnsDiff {
			columns.appendTrack(autoColumns.next())
		}
	}

	// Add implicit line names
	for y, row := range gridAreas {
		for x, areaName := range row {
			if areaName == "" {
				continue
			}
			lc.spend(1)
			startName := areaName + "-start"
			if !gridHasName(rows.lines, startName) && y < len(rows.lines) {
				rows.lines[y] = append(rows.lines[y], startName)
			}
			if !gridHasName(columns.lines, startName) && x < len(columns.lines) {
				columns.lines[x] = append(columns.lines[x], startName)
			}
		}
	}
	for y := range gridAreas {
		row := gridAreas[len(gridAreas)-1-y]
		for x := range row {
			areaName := row[len(row)-1-x]
			if areaName == "" {
				continue
			}
			lc.spend(1)
			endName := areaName + "-end"
			if k := len(rows.lines) - 1 - y; !gridHasName(rows.lines, endName) && k >= 0 {
				rows.lines[k] = append(rows.lines[k], endName)
			}
			if k := len(columns.lines) - 1 - x; !gridHasName(columns.lines, endName) && k >= 0 {
				columns.lines[k] = append(columns.lines[k], endName)
			}
		}
	}

	// 1. Run the grid placement algorithm.

	firstFlow, secondFlow := "row", "column" // auto flow axis, other axis
	if tupleHas(flow, "column") {
		firstFlow, secondFlow = "column", "row"
	}
	firstTracks, secondTracks := rows, columns
	if firstFlow != "row" {
		firstTracks, secondTracks = columns, rows
	}
	dense := tupleHas(flow, "dense")
	lineOf := func(child *Box, flow, side string) gridPlaceLine {
		return gridParseLine(child.Style.Get("grid_" + flow + "_" + side))
	}
	// place returns the area of the placement on the first and the second
	// axes.
	place := func(firstI, firstSize, secondI, secondSize int) gridArea {
		if firstFlow == "row" {
			return gridArea{secondI, firstI, secondSize, firstSize}
		}
		return gridArea{firstI, secondI, firstSize, secondSize}
	}

	// 1.1 Position anything that’s not auto-positioned.
	children := slices.Clone(box.Children)
	slices.SortStableFunc(children, func(a, b *Box) int {
		return cmp.Compare(num(a.Style.Get("order")), num(b.Style.Get("order")))
	})
	positions := &gridPositions{areas: map[*Box]gridArea{}}
	for _, child := range children {
		lc.spend(1)
		x, width, okColumn := gridGetPlacement(
			lineOf(child, "column", "start"), lineOf(child, "column", "end"), columns.lines)
		y, height, okRow := gridGetPlacement(
			lineOf(child, "row", "start"), lineOf(child, "row", "end"), rows.lines)
		if okColumn && okRow {
			positions.set(child, gridArea{x, y, width, height})
		}
	}

	// 1.2 Process the items locked to a given row (resp. column).
	for _, child := range children {
		if _, ok := positions.areas[child]; ok {
			continue
		}
		lc.spend(1)
		firstCoord, firstSize, ok := gridGetPlacement(
			lineOf(child, firstFlow, "start"), lineOf(child, firstFlow, "end"), firstTracks.lines)
		if !ok {
			continue
		}
		secondCoord, secondSize := gridGetSecondPlacement(
			lc, firstCoord, firstSize, lineOf(child, secondFlow, "start"),
			lineOf(child, secondFlow, "end"), secondTracks.lines, positions, firstFlow, dense)
		positions.set(child, place(firstCoord, firstSize, secondCoord, secondSize))
	}

	// 1.3 Determine the columns (resp. rows) in the implicit grid.
	// 1.3.1 Start with the columns (resp. rows) from the explicit grid.
	implicitSecond1 := 0
	implicitSecond2 := len(gridAreas)
	if secondFlow == "column" {
		implicitSecond2 = gridAreasColumnsOf(gridAreas)
	}
	// 1.3.2 Add columns (resp. rows) to the beginning and end of the
	// implicit grid.
	var remainingGridItems []*Box
	for _, child := range children {
		var i, size int
		if a, ok := positions.areas[child]; ok {
			if secondFlow == "column" {
				i, size = a.x, a.width
			} else {
				i, size = a.y, a.height
			}
		} else {
			coord, s, ok := gridGetPlacement(
				lineOf(child, secondFlow, "start"), lineOf(child, secondFlow, "end"), secondTracks.lines)
			remainingGridItems = append(remainingGridItems, child)
			if !ok {
				continue
			}
			i, size = coord, s
		}
		implicitSecond1 = min(i, implicitSecond1)
		implicitSecond2 = max(i+size, implicitSecond2)
	}
	// 1.3.3 Add columns (resp. rows) to accommodate max track span.
	for _, child := range remainingGridItems {
		secondStart := lineOf(child, secondFlow, "start")
		secondEnd := lineOf(child, secondFlow, "end")
		span := 1
		if !secondStart.auto && secondStart.span {
			span = secondStart.orOne()
		} else if !secondEnd.auto && secondEnd.span {
			span = secondEnd.orOne()
		}
		implicitSecond2 = max(implicitSecond1+span, implicitSecond2)
	}

	// 1.4 Position the remaining grid items.
	implicitFirst1 := 0
	implicitFirst2 := gridAreasColumnsOf(gridAreas)
	if firstFlow == "row" {
		implicitFirst2 = len(gridAreas)
	}
	for _, child := range positions.order {
		a := positions.areas[child]
		i, size := a.x, a.width
		if firstFlow == "row" {
			i, size = a.y, a.height
		}
		implicitFirst1 = min(i, implicitFirst1)
		implicitFirst2 = max(i+size, implicitFirst2)
	}
	cursorFirst, cursorSecond := implicitFirst1, implicitSecond1
	var firstI, firstSize, secondI, secondSize int
	// maxSearch bounds the searches of a free place.
	//
	// Port: upstream searches without end, for example for an item whose
	// span never fits.
	maxSearch := gridMaxSearch + 4*len(children)
	firstPlacement := func(child *Box, base int) {
		firstStart := lineOf(child, firstFlow, "start")
		firstEnd := lineOf(child, firstFlow, "end")
		if firstStart.auto {
			firstI, firstSize, _ = gridGetPlacement(gridNumberLine(base+1), firstEnd, firstTracks.lines)
		} else {
			// Port: upstream asserts that the start is a span.
			span := gridGetSpan(firstStart)
			firstI, firstSize, _ = gridGetPlacement(firstStart, gridNumberLine(base+1+span), firstTracks.lines)
		}
	}
	secondPlacementAt := func(child *Box, base int) {
		secondStart := lineOf(child, secondFlow, "start")
		secondEnd := lineOf(child, secondFlow, "end")
		if secondStart.auto {
			secondI, secondSize, _ = gridGetPlacement(gridNumberLine(base+1), secondEnd, secondTracks.lines)
		} else {
			span := gridGetSpan(secondStart)
			secondI, secondSize, _ = gridGetPlacement(secondStart, gridNumberLine(base+1+span), secondTracks.lines)
		}
	}
	// searchFree is the loop `while True` of the items without a definite
	// position on either axis: it finds the first free place from the
	// cursor. dense is set for the dense packing.
	searchFree := func(child *Box) {
		for iteration := 0; ; iteration++ {
			lc.spend(1)
			// 2. Increment the column (resp. row) position of the cursor.
			firstI = cursorFirst
			for second := cursorSecond; second < implicitSecond2; second++ {
				firstPlacement(child, firstI)
				secondPlacementAt(child, second)
				area := place(firstI, firstSize, secondI, secondSize)
				intersect := gridIntersectWithChildren(lc, area, positions)
				overflow := secondI+secondSize > implicitSecond2
				if intersect || overflow {
					// Child intersects with a positioned child or
					// overflows.
					continue
				}
				// Free place found.
				// 3. Set the item’s row-/column-start lines.
				positions.set(child, area)
				if dense {
					if firstDiff := cursorFirst + firstSize - 1 - implicitFirst2; firstDiff > 0 {
						implicitFirst2 += firstDiff
					}
				}
				return
			}
			// No room found.
			// 2. Return to the previous step.
			cursorFirst++
			if firstDiff := cursorFirst + 1 - implicitFirst2; firstDiff > 0 {
				implicitFirst2 += firstDiff
			}
			cursorSecond = implicitSecond1
			if iteration > maxSearch {
				// Port: upstream searches without end. The port places the
				// item at the cursor, where it can overflow.
				firstPlacement(child, cursorFirst)
				secondPlacementAt(child, implicitSecond1)
				positions.set(child, place(firstI, firstSize, secondI, secondSize))
				return
			}
		}
	}
	for _, child := range remainingGridItems {
		secondPlacementCoord, secondPlacementSize, hasSecond := gridGetPlacement(
			lineOf(child, secondFlow, "start"), lineOf(child, secondFlow, "end"), secondTracks.lines)
		switch {
		case dense && hasSecond:
			// 1. Set the row (resp. column) position of the cursor.
			cursorFirst = implicitFirst1
			secondI, secondSize = secondPlacementCoord, secondPlacementSize
			cursorSecond = secondI
			// 2. Increment the cursor’s row (resp. column) position.
			for n := 0; ; n++ {
				lc.spend(1)
				firstPlacement(child, cursorFirst+n)
				if firstI < cursorFirst && n <= maxSearch {
					continue
				}
				// Port: upstream tests the same place for each row that the
				// item spans. The port tests it once.
				area := place(firstI, firstSize, secondI, secondSize)
				// Child doesn’t intersect with any positioned child on any
				// row.
				if !gridIntersectWithChildren(lc, area, positions) || n > maxSearch {
					break
				}
				// Child intersects with a positioned child on current row.
			}
			if firstDiff := firstI + firstSize - implicitFirst2; firstDiff > 0 {
				implicitFirst2 += firstDiff
			}
			// 3. Set the item’s row-start line.
			positions.set(child, place(firstI, firstSize, secondI, secondSize))
		case dense:
			// 1. Set the cursor’s row and column positions.
			cursorFirst, cursorSecond = implicitFirst1, implicitSecond1
			searchFree(child)
		case hasSecond:
			// 1. Set the column (resp. row) position of the cursor.
			secondI, secondSize = secondPlacementCoord, secondPlacementSize
			if secondI < cursorSecond {
				cursorFirst++
			}
			cursorSecond = secondI
			// 2. Increment the cursor’s row (resp. column) position.
			start := cursorFirst
			var area gridArea
			for n := 0; ; n++ {
				lc.spend(1)
				cursorFirst = start + n
				// Port: for a start that is a span, upstream reads first_i,
				// which an earlier item set, or which is not set and fails.
				// The port reads the cursor, as for a start of auto and as
				// the dense packing does. The value of upstream never moves,
				// and its search never ends.
				firstPlacement(child, cursorFirst)
				if firstI < cursorFirst && n <= maxSearch {
					continue
				}
				// Port: upstream tests the same place for each row that the
				// item spans. The port tests it once.
				area = place(firstI, firstSize, secondI, secondSize)
				// Child doesn’t intersect with any positioned child on any
				// row.
				if !gridIntersectWithChildren(lc, area, positions) || n > maxSearch {
					break
				}
				// Child intersects with a positioned child on current row.
			}
			if firstDiff := firstI + firstSize - implicitFirst2; firstDiff > 0 {
				implicitFirst2 += firstDiff
			}
			// 3. Set the item’s row-start line.
			positions.set(child, area)
		default:
			searchFree(child)
		}
	}

	implicitX1, implicitX2 := implicitSecond1, implicitSecond2
	implicitY1, implicitY2 := implicitFirst1, implicitFirst2
	if firstFlow != "row" {
		implicitX1, implicitX2 = implicitFirst1, implicitFirst2
		implicitY1, implicitY2 = implicitSecond1, implicitSecond2
	}
	// Port: upstream has no bound on the size of the implicit grid.
	if implicitX2-implicitX1 > gridMaxImplicitTracks || implicitY2-implicitY1 > gridMaxImplicitTracks {
		lc.abort(fmt.Errorf("%w: a grid has more than %d tracks", ivy.ErrLimit, gridMaxImplicitTracks))
	}
	for range -implicitX1 {
		columns.prependTrack(autoColumnsBack.next())
	}
	for range implicitX2 - gridAreasColumnsOf(gridAreas) {
		columns.appendTrack(autoColumns.next())
	}
	for range -implicitY1 {
		rows.prependTrack(autoRowsBack.next())
	}
	for range implicitY2 - len(gridAreas) {
		rows.appendTrack(autoRows.next())
	}

	pageBreaksByRow := map[int]*gridPageBreaks{}
	pageBreaks := func(y int) *gridPageBreaks {
		breaks, ok := pageBreaksByRow[y]
		if !ok {
			breaks = &gridPageBreaks{before: "auto", after: "auto", inside: "auto"}
			pageBreaksByRow[y] = breaks
		}
		return breaks
	}
	for _, child := range positions.order {
		rowPageBreak := pageBreaks(positions.areas[child].y)
		if isStrIn(child.Style.Get("break_inside"), "avoid", "avoid-page") {
			rowPageBreak.inside = "avoid"
		}
		for _, side := range [...]string{"before", "after"} {
			value := child.Style.Get("break_" + side)
			current := &rowPageBreak.before
			if side == "after" {
				current = &rowPageBreak.after
			}
			switch {
			case isStrIn(value, "avoid", "avoid-page"):
				if *current != "page" {
					*current = "avoid"
				}
			case isStrIn(value, "page", "always"):
				*current = "page"
			case isStrIn(value, "left", "right", "recto", "verso"):
				*current = str(value)
			}
		}
	}

	// 3. Run the grid sizing algorithm.

	// 3.0 List min/max sizing functions.
	rowSizingFunctions := make([][2]Value, len(rows.sizes))
	for i, row := range rows.sizes {
		rowSizingFunctions[i] = gridGetSizingFunctions(row)
	}
	columnSizingFunctions := make([][2]Value, len(columns.sizes))
	for i, column := range columns.sizes {
		columnSizingFunctions[i] = gridGetSizingFunctions(column)
	}

	// 3.1 Resolve the sizes of the grid columns.
	columnsSizes := gridResolveTracksSizes(
		lc, columnSizingFunctions, box.Width, positions, implicitSecond1,
		'x', columnGap, box, nil)

	// 3.2 Resolve the sizes of the grid rows.
	verticalSizes := make([]float64, len(columnsSizes))
	for i, size := range columnsSizes {
		verticalSizes[i] = size[0]
	}
	rowsSizes := gridResolveTracksSizes(
		lc, rowSizingFunctions, box.Height, positions, implicitY1, 'y',
		rowGap, box, verticalSizes)

	// 3.3 Re-resolve the sizes of the grid columns with min-/max-content.
	// TODO: Re-resolve.

	// 3.4 Re-re-resolve the sizes of the grid columns with
	// min-/max-content.
	// TODO: Re-re-resolve.

	// 3.5 Align the tracks within the grid container.
	// TODO: Support safe/unsafe.
	justifyContent := style.Get("justify_content")
	x := box.ContentBoxX()
	freeWidth := max(0, box.Width-gridSumBase(columnsSizes, 0, len(columnsSizes)))
	columnsPositions := make([]float64, 0, len(columnsSizes))
	columnsNumber := float64(len(columnsSizes))
	switch {
	case gridHasAny(justifyContent, "center"):
		x += freeWidth / 2
		for _, size := range columnsSizes {
			columnsPositions = append(columnsPositions, x)
			x += size[0] + columnGap
		}
	case gridHasAny(justifyContent, "right", "end", "flex-end"):
		x += freeWidth
		for _, size := range columnsSizes {
			columnsPositions = append(columnsPositions, x)
			x += size[0] + columnGap
		}
	case gridHasAny(justifyContent, "space-around"):
		x += freeWidth / 2 / columnsNumber
		for _, size := range columnsSizes {
			columnsPositions = append(columnsPositions, x)
			x += size[0] + freeWidth/columnsNumber + columnGap
		}
	case gridHasAny(justifyContent, "space-between"):
		for _, size := range columnsSizes {
			columnsPositions = append(columnsPositions, x)
			if columnsNumber >= 2 {
				x += size[0] + freeWidth/(columnsNumber-1) + columnGap
			}
		}
	case gridHasAny(justifyContent, "space-evenly"):
		x += freeWidth / (columnsNumber + 1)
		for _, size := range columnsSizes {
			columnsPositions = append(columnsPositions, x)
			x += size[0] + freeWidth/(columnsNumber+1) + columnGap
		}
	default:
		for _, size := range columnsSizes {
			columnsPositions = append(columnsPositions, x)
			x += size[0] + columnGap
		}
	}

	alignContent := style.Get("align_content")
	y := box.ContentBoxY()
	freeHeight := 0.0
	if box.Height != Auto {
		freeHeight = box.Height - gridSumBase(rowsSizes, 0, len(rowsSizes)) -
			float64(len(rowsSizes)-1)*rowGap
		freeHeight = max(0, freeHeight)
	}
	rowsPositions := make([]float64, 0, len(rowsSizes))
	rowsNumber := float64(len(rowsSizes))
	switch {
	case gridHasAny(alignContent, "center"):
		y += freeHeight / 2
		for _, size := range rowsSizes {
			rowsPositions = append(rowsPositions, y)
			y += size[0] + rowGap
		}
	case gridHasAny(alignContent, "right", "end", "flex-end"):
		y += freeHeight
		for _, size := range rowsSizes {
			rowsPositions = append(rowsPositions, y)
			y += size[0] + rowGap
		}
	case gridHasAny(alignContent, "space-around"):
		y += freeHeight / 2 / rowsNumber
		for _, size := range rowsSizes {
			rowsPositions = append(rowsPositions, y)
			y += size[0] + freeHeight/rowsNumber + rowGap
		}
	case gridHasAny(alignContent, "space-between"):
		for _, size := range rowsSizes {
			rowsPositions = append(rowsPositions, y)
			if rowsNumber >= 2 {
				y += size[0] + freeHeight/(rowsNumber-1) + rowGap
			}
		}
	case gridHasAny(alignContent, "space-evenly"):
		y += freeHeight / (rowsNumber + 1)
		for _, size := range rowsSizes {
			rowsPositions = append(rowsPositions, y)
			y += size[0] + freeHeight/(rowsNumber+1) + rowGap
		}
	default:
		if gridHasAny(alignContent, "baseline") {
			// TODO: Support baseline value.
			lc.log.warning("Baseline alignment is not supported for grid layout")
		}
		for _, size := range rowsSizes {
			rowsPositions = append(rowsPositions, y)
			y += size[0] + rowGap
		}
	}

	// 4. Lay out the grid items into their respective containing blocks.

	// Find resume_at row.
	type pageChild struct {
		index int
		child *Box
	}
	var thisPageChildren []pageChild
	resumeRow, hasResumeRow := 0, false
	extraSkipHeight := 0.0
	advancements := gridAdvancements(box)
	var firstSkipRow, lastSkipRow int
	var skipHeight float64
	if len(skip) > 0 {
		first := true
		for row := range skip {
			if first || row < firstSkipRow {
				firstSkipRow = row
			}
			if first || row > lastSkipRow {
				lastSkipRow = row
			}
			first = false
		}
		skipHeight = gridSumBase(rowsSizes, 0, lastSkipRow) +
			float64(gridLenSlice(0, lastSkipRow, len(rowsSizes))-1)*rowGap
		for index, child := range positions.order {
			a := positions.areas[child]
			advancement, ok := gridAdvancementGet(advancements, a.x, a.y)
			if !ok {
				continue
			}
			lc.spend(1)
			span := gridGetSpan(gridParseLine(child.Style.Get("grid_row_start")))
			spanHeight := gridSumBase(rowsSizes, a.y, a.y+span) + float64(span-1)*rowGap
			width := gridSum(verticalSizes, a.x, a.x+a.width)
			child = child.DeepCopy()
			child.PositionX = 0
			child.PositionY = 0
			var childSkipStack skipStack
			if rowSkip, ok := skip[a.y]; ok {
				childSkipStack = rowSkip[index]
			}
			parent := gridAnonymousParent(containingBlock, box)
			resolvePercentages(parent, containingBlock)
			parent.PositionX = 0
			parent.PositionY = 0
			parent.Width = width
			parent.Height = 0
			lc.createBlockFormattingContext(parent)
			result := blockLevelLayout(lc, child, math.Inf(-1), childSkipStack, parent,
				true, &[]*Box{}, nil, nil, nil, nil, false, noMaxLines)
			lc.finishBlockFormattingContext(nil)
			// Port: upstream fails on an item that gives no box.
			if result.box == nil {
				continue
			}
			skipStackAdvancement := spanHeight - result.box.MarginHeight()
			if skipStackAdvancement < advancement {
				extraSkipHeight = max(extraSkipHeight, advancement-skipStackAdvancement)
			}
		}
		found := false
		for _, item := range gridAdvancementItems(advancements) {
			if item.key[1] != lastSkipRow {
				continue
			}
			skipHeight += item.value
			found = true
			break
		}
		if !found {
			extraSkipHeight = 0
		}
		skipHeight -= extraSkipHeight
	}
	var resumeAt skipStack
	totalHeight := gridSumBase(rowsSizes, lastSkipRow, len(rowsSizes)) +
		float64(gridLenSlice(lastSkipRow, len(rowsSizes), len(rowsSizes))-1)*rowGap

	addPageChildren := func(maxRow float64) {
		for j, child := range children {
			a, ok := positions.areas[child]
			if !ok {
				// Port: upstream raises KeyError for an item without a
				// place, which the placement never leaves.
				continue
			}
			y := a.y
			if !(firstSkipRow <= y && float64(y) < maxRow) {
				// Item in previous or next rows.
				continue
			}
			switch {
			case skip == nil:
				// No skip stack, draw on this page.
				thisPageChildren = append(thisPageChildren, pageChild{j, child})
			case y > lastSkipRow:
				// Row after the skip stack, draw on this page.
				thisPageChildren = append(thisPageChildren, pageChild{j, child})
			default:
				if childSkipStack, ok := skip[y]; ok {
					if _, in := childSkipStack[j]; childSkipStack == nil || in {
						// Child in skip stack, draw on this page.
						thisPageChildren = append(thisPageChildren, pageChild{j, child})
					}
				}
			}
		}
	}

	lo, hi := gridSlice(firstSkipRow+1, len(rowsPositions), len(rowsPositions))
	rowLinesPositions := append(slices.Clone(rowsPositions[lo:hi]), box.ContentBoxY()+totalHeight)
	originalPageIsEmpty := pageIsEmpty
	broke := false
	for k, rowY := range rowLinesPositions {
		i := firstSkipRow + k
		// TODO: handle break-before and break-after for rows.
		if !lc.overflowsPage(bottomSpace, rowY-skipHeight) {
			// No need to force rendering, at least a whole row fits in page.
			pageIsEmpty = false
			continue
		}
		resumeRow, hasResumeRow = i, true
		if isStr(style.Get("break_inside"), "avoid") && !originalPageIsEmpty {
			// Avoid breaks inside grid container, break before.
			return noBox
		}
		if pageBreaks(i).inside == "avoid" && !pageIsEmpty {
			// Break before current row.
			if resumeRow == 0 {
				// First row, break before grid container.
				return noBox
			}
			// Mark all children before and in current row as drawn on the
			// page.
			addPageChildren(float64(resumeRow))
			resumeAt = skipStack{resumeRow: nil}
		} else {
			// Break inside current row.
			// Mark all children before current row as drawn on the page.
			addPageChildren(float64(resumeRow + 1))
		}
		broke = true
		break
	}
	if !broke {
		// Mark all children as drawn on the page.
		addPageChildren(math.Inf(1))
	}
	if box.Height == Auto {
		end := len(rowsSizes)
		if hasResumeRow {
			end = resumeRow
		}
		box.Height = gridSumBase(rowsSizes, 0, end) +
			float64(gridLenSlice(0, end, len(rowsSizes))-1)*rowGap - skipHeight
	}

	// Lay out grid items.
	justifyItems := style.Get("justify_items")
	alignItems := style.Get("align_items")
	var newChildren []*Box
	type rowChild struct {
		x     int
		child *Box
	}
	newChildrenByRows := map[int][]rowChild{}
	baseline, hasBaseline := 0.0, false
	nextPage := anyNextPage()
	// childContentHeight is read after the loop, with the value of the
	// last item, as upstream does.
	var childContentHeight float64
	// ensureRow returns resume_at[y], and makes it first.
	//
	// Port: upstream fails to set an item in the row of a resume_at of
	// None. The port makes the row.
	ensureRow := func(y int) skipStack {
		if resumeAt == nil {
			resumeAt = skipStack{}
		}
		if resumeAt[y] == nil {
			resumeAt[y] = skipStack{}
		}
		return resumeAt[y]
	}
	for _, pc := range thisPageChildren {
		lc.spend(1)
		i, child := pc.index, pc.child
		a := positions.areas[child]
		x, y, width, height := a.x, a.y, a.width, a.height
		index := i
		var childSkipStack skipStack
		if len(skip) > 0 && len(skip[y]) > 0 {
			if s, ok := skip[y][index]; ok {
				childSkipStack = s
			} else {
				// Port: upstream asserts that the child is a parent box.
				childSkipStack = skipStack{len(child.Children): nil}
			}
		}
		child = child.DeepCopy()
		child.PositionX = gridAt(columnsPositions, x, box.ContentBoxX())
		child.PositionY = gridAt(rowsPositions, y, box.ContentBoxY()) - skipHeight
		resolvePercentages(child, box)
		widthSize := gridSumBase(columnsSizes, x, x+width) + float64(width-1)*columnGap
		heightSize := gridSumBase(rowsSizes, y, y+height) + float64(height-1)*rowGap
		if len(skip) > 0 {
			if advancement, ok := gridAdvancementGet(advancements, x, y); ok {
				child.PositionY += advancement - extraSkipHeight
				heightSize -= advancement - extraSkipHeight
			}
		}

		// TODO: Apply auto margin.
		if child.MarginTop == Auto {
			child.MarginTop = 0
		}
		if child.MarginRight == Auto {
			child.MarginRight = 0
		}
		if child.MarginBottom == Auto {
			child.MarginBottom = 0
		}
		if child.MarginLeft == Auto {
			child.MarginLeft = 0
		}

		childBorderWidth := widthSize - (child.MarginLeft + child.MarginRight)
		childContentWidth := childBorderWidth - (child.BorderLeftWidth + child.PaddingLeft +
			child.BorderRightWidth + child.PaddingRight)
		childBorderHeight := heightSize - child.MarginBottom
		childContentHeight = childBorderHeight - (child.BorderBottomWidth + child.PaddingBottom)
		if len(childSkipStack) == 0 || isStr(child.Style.Get("box_decoration_break"), "clone") {
			childBorderHeight -= child.MarginTop
			childContentHeight -= child.MarginTop + child.BorderTopWidth + child.PaddingTop
		}

		justifySelf := child.Style.Get("justify_self")
		if gridHasAny(justifySelf, "auto") {
			justifySelf = justifyItems
		}
		if gridHasAny(justifySelf, "normal", "stretch") {
			if isStr(child.Style.Get("width"), "auto") {
				child.Style = child.Style.Copy()
				childWidth := childBorderWidth
				if isStr(child.Style.Get("box_sizing"), "content-box") {
					childWidth = childContentWidth
				}
				child.Style.Set("width", Dimension{Value: childWidth, Unit: "px"})
			}
		}
		alignSelf := child.Style.Get("align_self")
		if gridHasAny(alignSelf, "auto") {
			alignSelf = alignItems
		}
		if gridHasAny(alignSelf, "normal", "stretch") {
			if isStr(child.Style.Get("height"), "auto") {
				child.Style = child.Style.Copy()
				childHeight := childBorderHeight
				if isStr(child.Style.Get("box_sizing"), "content-box") {
					childHeight = childContentHeight
				}
				child.Style.Set("height", Dimension{Value: childHeight, Unit: "px"})
			}
		}

		// TODO: Find a better solution for the layout.
		parent := gridAnonymousParent(box, box)
		resolvePercentages(parent, containingBlock)
		parent.PositionX = child.PositionX
		parent.PositionY = child.PositionY
		parent.Width = widthSize
		parent.Height = heightSize
		lc.createBlockFormattingContext(parent)
		result := blockLevelLayout(lc, child, bottomSpace, childSkipStack, parent,
			pageIsEmpty, absoluteBoxes, fixedBoxes, nil, nil, nil, false, noMaxLines)
		lc.finishBlockFormattingContext(parent)
		newChild, childResumeAt := result.box, result.resumeAt
		if newChild == nil {
			ensureRow(y)[i] = nil
			continue
		}
		brokenChild := false
		span := gridGetSpan(gridParseLine(child.Style.Get("grid_row_start")))
		if len(childResumeAt) > 0 {
			brokenChild = true
		} else if hasResumeRow && y+span >= resumeRow+1 {
			brokenChild = true
		}
		if brokenChild {
			// Child is broken, add row to resume_at.
			if resumeAt == nil {
				resumeAt = skipStack{}
			}
			if _, ok := resumeAt[y]; !ok {
				resumeAt[y] = skipStack{}
			}
		}
		if len(childResumeAt) > 0 {
			// There is some content left for next page, save the cell’s
			// resume_at.
			ensureRow(y)[i] = childResumeAt
		} else if brokenChild {
			// Everything fits but the cell overflows. Only display the
			// bottom of an empty cell on next page, set the cell’s
			// resume_at after the cell’s last child.
			//
			// Port: upstream asserts that the new child is a parent box.
			previousSkipChild := 0
			if len(childSkipStack) > 0 {
				previousSkipChild = math.MinInt
				for key := range childSkipStack {
					previousSkipChild = max(previousSkipChild, key)
				}
			}
			if !pageIsEmpty {
				ensureRow(y)[i] = skipStack{previousSkipChild + len(newChild.Children): nil}
			}
		}
		pageIsEmpty = false

		if gridHasAny(justifySelf, "normal", "stretch") {
			newChild.Width = max(childContentWidth, newChild.Width)
		} else {
			if isStr(newChild.Style.Get("width"), "auto") {
				newChild.Width = maxContentWidth(lc, newChild, false)
			}
			diff := childContentWidth - newChild.Width
			if gridHasAny(justifySelf, "center") {
				newChild.Translate(diff/2, 0, false)
			} else if gridHasAny(justifySelf, "right", "end", "flex-end", "self-end") {
				newChild.Translate(diff, 0, false)
			}
		}

		// TODO: Apply auto margins.
		if gridHasAny(alignSelf, "normal", "stretch") {
			newChild.Height = max(childContentHeight, newChild.Height)
		} else {
			diff := childContentHeight - newChild.Height
			if gridHasAny(alignSelf, "center") {
				newChild.Translate(0, diff/2, false)
			} else if gridHasAny(alignSelf, "end", "flex-end", "self-end") {
				newChild.Translate(0, diff, false)
			}
		}

		newChildren = append(newChildren, newChild)
		newChildrenByRows[y] = append(newChildrenByRows[y], rowChild{x, newChild})
		if !hasBaseline && y == implicitY1 {
			baseline, hasBaseline = findInFlowBaseline(newChild, false)
		}
	}

	// Abort whole grid rendering if no child fits.
	if len(thisPageChildren) > 0 && len(newChildren) == 0 {
		return noBox
	}

	oldAdvancements := maps.Clone(advancements)
	clear(advancements)
	box = box.CopyWithChildren(newChildren)
	if box.Kind == KindInlineGrid {
		// TODO: Synthetize a real baseline value.
		lc.log.warning("Inline grids are not supported")
		box.Baseline = baseline
	}

	if isStr(box.Style.Get("position"), "relative") {
		// New containing block, resolve the layout of the absolute
		// descendants
		for _, absoluteBox := range *absoluteBoxes {
			absoluteLayout(lc, absoluteBox, box, fixedBoxes, bottomSpace, nil)
		}
	}

	for _, child := range box.Children {
		relativePositioning(child, sizeBlock(box.Width, box.Height))
	}

	// Resume early when there’s no resume_at.
	if len(resumeAt) == 0 {
		return blockLayoutResult{box: box, nextPage: nextPage, adjoiningMargins: []float64{}, maxLines: noMaxLines}
	}

	// Set broken rows’ bottom at the bottom of the page.
	nextPageFirstRow, nextPageLastRow := 0, 0
	first := true
	for row := range resumeAt {
		if first || row < nextPageFirstRow {
			nextPageFirstRow = row
		}
		if first || row > nextPageLastRow {
			nextPageLastRow = row
		}
		first = false
	}
	// Port: upstream fails on no laid out row. The port sets no bottom.
	lastPageRow := nextPageFirstRow - 1
	first = true
	for row := range newChildrenByRows {
		if first || row > lastPageRow {
			lastPageRow = row
		}
		first = false
	}
	extraAdvancement := 0.0
	for y := lastPageRow; y >= nextPageFirstRow; y-- {
		for _, rc := range newChildrenByRows[y] {
			child := rc.child
			span := gridGetSpan(gridParseLine(child.Style.Get("grid_row_start")))
			if y+span < lastPageRow+1 {
				// Child finishing before the last row, do nothing.
				continue
			}
			brokenChild := y+span >= nextPageLastRow+1
			if brokenChild && !isStr(child.Style.Get("box_decoration_break"), "clone") {
				child.RemoveDecoration(false, true)
			}
			child.Height = max(0,
				lc.pageBottom-bottomSpace-child.PositionY-
					child.MarginTop-child.BorderTopWidth-child.PaddingTop-
					child.MarginBottom-child.BorderBottomWidth-child.PaddingBottom)
			if brokenChild {
				// Child not fully drawn, keep advancement.
				value := child.MarginHeight()
				if old, ok := gridAdvancementGet(oldAdvancements, rc.x, y); ok {
					value += old - extraSkipHeight
				}
				gridAdvancementSet(advancements, rc.x, y, value)
			} else {
				// Child fully drawn, save the extra height added to reach
				// the bottom of the page to substract it from the
				// advancements.
				extraAdvancement = max(extraAdvancement, child.Height-childContentHeight)
			}
		}
	}

	// Substract the extra height added to reach the bottom of the page from
	// all the advancements.
	if extraAdvancement != 0 {
		for _, item := range gridAdvancementItems(advancements) {
			gridAdvancementSet(advancements, item.key[0], item.key[1], item.value-extraAdvancement)
		}
	}

	// Set box height and remove bottom decoration.
	box.Height = lc.pageBottom - bottomSpace - box.PositionY -
		box.MarginTop - box.BorderTopWidth - box.PaddingTop
	if !isStr(box.Style.Get("box_decoration_break"), "clone") {
		box.RemoveDecoration(false, true)
		box.Height -= box.MarginBottom + box.BorderBottomWidth + box.PaddingBottom
	}

	return blockLayoutResult{box: box, resumeAt: resumeAt, nextPage: nextPage, adjoiningMargins: []float64{}, maxLines: noMaxLines}
}

// gridAreasColumnsOf returns len(grid_areas[0]) if grid_areas else 0.
func gridAreasColumnsOf(gridAreas [][]string) int {
	if len(gridAreas) == 0 {
		return 0
	}
	return len(gridAreas[0])
}
