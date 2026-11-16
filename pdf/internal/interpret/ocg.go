// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/ocg.rs.

package interpret

import "github.com/xo/ivy/pdf/internal/syntax"

// ocgState is OcgState in hayro-interpret src/ocg.rs. It tracks which
// optional content is visible.
type ocgState struct {
	inactiveOcgs    map[syntax.ObjectIdentifier]bool
	visibilityStack []bool
}

// ocgStateFromCatalog is OcgState::from_catalog in hayro-interpret
// src/ocg.rs. It reads the default configuration of the optional content of
// the document.
func ocgStateFromCatalog(catalog syntax.Dict) *ocgState {
	ocProperties, ok := catalog.Get[syntax.Dict](syntax.KeyOCProperties)
	if !ok {
		return &ocgState{}
	}
	config, ok := ocProperties.Get[syntax.Dict](syntax.KeyD)
	if !ok {
		return &ocgState{}
	}
	inactive := make(map[syntax.ObjectIdentifier]bool)
	baseState := baseStateOn
	if name, ok := config.Get[syntax.Name](syntax.KeyBaseState); ok {
		if b, ok := baseStateFromName(name); ok {
			baseState = b
		}
	}
	refsOf := func(arr syntax.Array) []syntax.ObjectIdentifier {
		var ids []syntax.ObjectIdentifier
		it := arr.RawIter()
		for {
			item, ok := it.Next()
			if !ok {
				return ids
			}
			if r, ok := item.AsObjRef(); ok {
				ids = append(ids, r.ID())
			}
		}
	}
	if baseState == baseStateOff {
		if ocgs, ok := ocProperties.Get[syntax.Array](syntax.KeyOCGs); ok {
			for _, id := range refsOf(ocgs) {
				inactive[id] = true
			}
		}
	}
	readOcgArray := func(key syntax.Name, insertActive bool) {
		arr, ok := config.Get[syntax.Array](key)
		if !ok {
			return
		}
		for _, id := range refsOf(arr) {
			if insertActive {
				delete(inactive, id)
			} else {
				inactive[id] = true
			}
		}
	}
	readOcgArray(syntax.KeyON, true)
	readOcgArray(syntax.KeyOFF, false)
	return &ocgState{inactiveOcgs: inactive}
}

// beginSingleOc is OcgState::begin_single_oc in hayro-interpret
// src/ocg.rs.
func (s *ocgState) beginSingleOc(id syntax.ObjectIdentifier) {
	isActive := !s.inactiveOcgs[id]
	s.visibilityStack = append(s.visibilityStack, s.isVisible() && isActive)
}

// beginOcmd is OcgState::begin_ocmd in hayro-interpret src/ocg.rs. It
// enters a membership dictionary of optional content.
func (s *ocgState) beginOcmd(ocmd syntax.Dict) {
	policy := ocmdPolicyAnyOn
	if name, ok := ocmd.Get[syntax.Name](syntax.KeyP); ok {
		if p, ok := ocmdPolicyFromName(name); ok {
			policy = p
		}
	}
	var ids []syntax.ObjectIdentifier
	if arr, ok := ocmd.Get[syntax.Array](syntax.KeyOCGs); ok {
		it := arr.RawIter()
		for {
			item, ok := it.Next()
			if !ok {
				break
			}
			if r, ok := item.AsObjRef(); ok {
				ids = append(ids, r.ID())
			}
		}
	} else if r, ok := ocmd.GetRef(syntax.KeyOCGs); ok {
		ids = append(ids, r.ID())
	}
	isActive := true
	if len(ids) > 0 {
		inactive := func(id syntax.ObjectIdentifier) bool { return s.inactiveOcgs[id] }
		switch policy {
		case ocmdPolicyAllOn:
			isActive = all(ids, func(id syntax.ObjectIdentifier) bool { return !inactive(id) })
		case ocmdPolicyAnyOn:
			isActive = anyOf(ids, func(id syntax.ObjectIdentifier) bool { return !inactive(id) })
		case ocmdPolicyAnyOff:
			isActive = anyOf(ids, inactive)
		case ocmdPolicyAllOff:
			isActive = all(ids, inactive)
		}
	}
	s.visibilityStack = append(s.visibilityStack, s.isVisible() && isActive)
}

// all is Iterator::all of Rust.
func all[T any](s []T, f func(T) bool) bool {
	for _, v := range s {
		if !f(v) {
			return false
		}
	}
	return true
}

// anyOf is Iterator::any of Rust.
func anyOf[T any](s []T, f func(T) bool) bool {
	for _, v := range s {
		if f(v) {
			return true
		}
	}
	return false
}

// beginOcg is OcgState::begin_ocg in hayro-interpret src/ocg.rs.
func (s *ocgState) beginOcg(props syntax.Dict, refID syntax.ObjectIdentifier) {
	if t, ok := props.Get[syntax.Name](syntax.KeyType); ok && t == syntax.KeyOCMD {
		s.beginOcmd(props)
		return
	}
	s.beginSingleOc(refID)
}

// beginMarkedContent is OcgState::begin_marked_content in hayro-interpret
// src/ocg.rs.
func (s *ocgState) beginMarkedContent() {
	s.visibilityStack = append(s.visibilityStack, s.isVisible())
}

// endMarkedContent is OcgState::end_marked_content in hayro-interpret
// src/ocg.rs.
func (s *ocgState) endMarkedContent() {
	if len(s.visibilityStack) > 0 {
		s.visibilityStack = s.visibilityStack[:len(s.visibilityStack)-1]
	}
}

// isVisible is OcgState::is_visible in hayro-interpret src/ocg.rs.
func (s *ocgState) isVisible() bool {
	if len(s.visibilityStack) == 0 {
		return true
	}
	return s.visibilityStack[len(s.visibilityStack)-1]
}

// baseState is BaseState in hayro-interpret src/ocg.rs.
type baseState uint8

// The cases of BaseState.
const (
	baseStateOn baseState = iota
	baseStateOff
	baseStateUnchanged
)

// baseStateFromName is BaseState::from_name in hayro-interpret src/ocg.rs.
func baseStateFromName(name syntax.Name) (baseState, bool) {
	switch name {
	case "ON":
		return baseStateOn, true
	case "OFF":
		return baseStateOff, true
	case "Unchanged":
		return baseStateUnchanged, true
	}
	return 0, false
}

// ocmdPolicy is OcmdPolicy in hayro-interpret src/ocg.rs.
type ocmdPolicy uint8

// The cases of OcmdPolicy.
const (
	ocmdPolicyAllOn ocmdPolicy = iota
	ocmdPolicyAnyOn
	ocmdPolicyAnyOff
	ocmdPolicyAllOff
)

// ocmdPolicyFromName is OcmdPolicy::from_name in hayro-interpret
// src/ocg.rs.
func ocmdPolicyFromName(name syntax.Name) (ocmdPolicy, bool) {
	switch name {
	case "AllOn":
		return ocmdPolicyAllOn, true
	case "AnyOn":
		return ocmdPolicyAnyOn, true
	case "AnyOff":
		return ocmdPolicyAnyOff, true
	case "AllOff":
		return ocmdPolicyAllOff, true
	}
	return 0, false
}
