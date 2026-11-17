// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/interpret/stack.rs.

package interpret

import (
	"math"
	"slices"

	"github.com/xo/okage/kurbo"
)

// groupItem is an item of Group in hayro-interpret
// src/interpret/stack.rs: a value and the depth that set it.
type groupItem[T any] struct {
	value T
	depth uint16
}

// group is Group in hayro-interpret src/interpret/stack.rs. It holds the
// values of one part of the graphics state, sorted by depth, with at most
// one item for each depth. A save copies a value only when a later
// operator changes it.
//
// Port: upstream clones with the trait Clone. Go takes the clone function
// of the type.
type group[T any] struct {
	items []groupItem[T]
	clone func(T) T
}

// newGroup is Group::new in hayro-interpret src/interpret/stack.rs.
func newGroup[T any](value T, clone func(T) T) group[T] {
	return group[T]{items: []groupItem[T]{{value: value}}, clone: clone}
}

// get is Group::get in hayro-interpret src/interpret/stack.rs.
func (g *group[T]) get(idx uint16) *T {
	return &g.items[idx].value
}

// getMut is Group::get_mut in hayro-interpret src/interpret/stack.rs. It
// copies the value of an earlier depth before the caller changes it. The
// pointer is valid until the next call of getMut.
func (g *group[T]) getMut(idx *uint16, depth uint16) *T {
	if g.items[*idx].depth != depth {
		value := g.clone(g.items[*idx].value)
		g.items = append(g.items, groupItem[T]{value: value, depth: depth})
		*idx = uint16(len(g.items) - 1)
	}
	return &g.items[*idx].value
}

// truncate is Group::truncate in hayro-interpret src/interpret/stack.rs.
func (g *group[T]) truncate(idx uint16) {
	n := int(idx) + 1
	if n < len(g.items) {
		clear(g.items[n:])
		g.items = g.items[:n]
	}
}

// frame is Frame in hayro-interpret src/interpret/stack.rs. It holds the
// index of each part of the state of one level of the stack.
type frame struct {
	ctm         uint16
	stroke      uint16
	fill        uint16
	strokeProps uint16
	text        uint16
	compositing uint16
	clips       uint32
}

// stateStack is StateStack in hayro-interpret src/interpret/stack.rs. It is
// the stack of graphics states of q and Q.
type stateStack struct {
	frames []frame
	// overflow counts the saves past the largest depth. Upstream notes
	// that this becomes an error once the interpretation can fail.
	overflow    int
	ctm         group[kurbo.Affine]
	stroke      group[paintState]
	fill        group[paintState]
	strokeProps group[StrokeProps]
	text        group[textState]
	compositing group[compositingState]
	clips       []clipType
}

// identity returns its argument. It is the clone of a type without
// pointers that change.
func identity[T any](v T) T { return v }

// cloneStrokeProps is the derived Clone of StrokeProps.
func cloneStrokeProps(p StrokeProps) StrokeProps {
	p.DashArray = slices.Clone(p.DashArray)
	return p
}

// newStateStack is StateStack::new in hayro-interpret
// src/interpret/stack.rs.
func newStateStack(s state) *stateStack {
	return &stateStack{
		frames:      []frame{{}},
		ctm:         newGroup(s.ctm, identity[kurbo.Affine]),
		stroke:      newGroup(s.stroke, identity[paintState]),
		fill:        newGroup(s.fill, identity[paintState]),
		strokeProps: newGroup(s.strokeProps, cloneStrokeProps),
		text:        newGroup(s.textState, textState.clone),
		compositing: newGroup(s.compositing, identity[compositingState]),
	}
}

// top is StateStack::top in hayro-interpret src/interpret/stack.rs.
func (s *stateStack) top() *frame {
	return &s.frames[len(s.frames)-1]
}

// depth is StateStack::depth in hayro-interpret src/interpret/stack.rs.
func (s *stateStack) depth() uint16 {
	return uint16(len(s.frames) - 1)
}

// The accessors of the macro accessors! in hayro-interpret
// src/interpret/stack.rs. A pointer of a mut accessor is valid until the
// next call of the same mut accessor.

func (s *stateStack) ctmRef() *kurbo.Affine { return s.ctm.get(s.top().ctm) }
func (s *stateStack) ctmMut() *kurbo.Affine {
	return s.ctm.getMut(&s.top().ctm, s.depth())
}

func (s *stateStack) strokeRef() *paintState { return s.stroke.get(s.top().stroke) }
func (s *stateStack) strokeMut() *paintState {
	return s.stroke.getMut(&s.top().stroke, s.depth())
}

func (s *stateStack) fillRef() *paintState { return s.fill.get(s.top().fill) }
func (s *stateStack) fillMut() *paintState {
	return s.fill.getMut(&s.top().fill, s.depth())
}

func (s *stateStack) strokePropsRef() *StrokeProps { return s.strokeProps.get(s.top().strokeProps) }
func (s *stateStack) strokePropsMut() *StrokeProps {
	return s.strokeProps.getMut(&s.top().strokeProps, s.depth())
}

func (s *stateStack) textRef() *textState { return s.text.get(s.top().text) }
func (s *stateStack) textMut() *textState {
	return s.text.getMut(&s.top().text, s.depth())
}

func (s *stateStack) compositingRef() *compositingState {
	return s.compositing.get(s.top().compositing)
}

func (s *stateStack) compositingMut() *compositingState {
	return s.compositing.getMut(&s.top().compositing, s.depth())
}

// len is StateStack::len in hayro-interpret src/interpret/stack.rs.
func (s *stateStack) len() int {
	return len(s.frames) + s.overflow
}

// save is StateStack::save in hayro-interpret src/interpret/stack.rs.
func (s *stateStack) save() {
	if len(s.frames) > math.MaxUint16 {
		s.overflow++
		return
	}
	f := *s.top()
	f.clips = uint32(len(s.clips))
	s.frames = append(s.frames, f)
}

// restoreTargetClips is StateStack::restore_target_clips in hayro-interpret
// src/interpret/stack.rs. It returns the number of clips that a restore
// keeps, and false when there is nothing to restore.
func (s *stateStack) restoreTargetClips() (int, bool) {
	if s.overflow > 0 {
		return len(s.clips), true
	}
	if len(s.frames) > 1 {
		return int(s.top().clips), true
	}
	return 0, false
}

// restore is StateStack::restore in hayro-interpret src/interpret/stack.rs.
func (s *stateStack) restore() {
	if s.overflow > 0 {
		s.overflow--
		return
	}
	if len(s.frames) <= 1 {
		return
	}
	s.frames = s.frames[:len(s.frames)-1]
	top := *s.top()
	s.ctm.truncate(top.ctm)
	s.stroke.truncate(top.stroke)
	s.fill.truncate(top.fill)
	s.strokeProps.truncate(top.strokeProps)
	s.text.truncate(top.text)
	s.compositing.truncate(top.compositing)
}

// pushClip is StateStack::push_clip in hayro-interpret
// src/interpret/stack.rs.
func (s *stateStack) pushClip(c clipType) {
	s.clips = append(s.clips, c)
}

// popClip is StateStack::pop_clip in hayro-interpret
// src/interpret/stack.rs.
func (s *stateStack) popClip() (clipType, bool) {
	if len(s.clips) == 0 {
		return 0, false
	}
	c := s.clips[len(s.clips)-1]
	s.clips = s.clips[:len(s.clips)-1]
	return c, true
}

// numClips is StateStack::num_clips in hayro-interpret
// src/interpret/stack.rs.
func (s *stateStack) numClips() int {
	return len(s.clips)
}

// snapshot is StateStack::snapshot in hayro-interpret
// src/interpret/stack.rs.
//
// Port: the snapshot has the default text state. Upstream copies the text
// state, with the paths of the clip of the text, but its one caller, the
// glyph of Type 3, resets the text state before it runs. A page with many
// glyphs of Type 3 in a text that clips copied the paths once for each
// glyph (D18).
func (s *stateStack) snapshot() state {
	return state{
		ctm:         *s.ctmRef(),
		stroke:      *s.strokeRef(),
		fill:        *s.fillRef(),
		strokeProps: cloneStrokeProps(*s.strokePropsRef()),
		textState:   defaultTextState(),
		compositing: *s.compositingRef(),
	}
}
