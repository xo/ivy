// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/x_object/soft_mask.rs.

package interpret

import (
	"fmt"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy/pdf/internal/syntax"
)

// MaskType is MaskType in hayro-interpret src/x_object/soft_mask.rs. It is
// the type of a soft mask.
type MaskType uint8

// The types of soft masks.
const (
	// MaskTypeLuminosity is MaskType::Luminosity.
	MaskTypeLuminosity MaskType = iota
	// MaskTypeAlpha is MaskType::Alpha.
	MaskTypeAlpha
)

// SoftMask is SoftMask in hayro-interpret src/x_object/soft_mask.rs, with
// Repr of the same file. It is a soft mask of a graphics state.
//
// Port: upstream shares the mask through an Rc. Go shares the pointer. Two
// masks are equal when their cache keys are equal.
type SoftMask struct {
	dictKey               Key128
	parentResourcesKey    Key128
	hasParentResourcesKey bool
	group                 *formXObject
	maskType              MaskType
	parentResources       *syntax.Resources
	rootTransform         kurbo.Affine
	bbox                  kurbo.Rect
	interpreterCache      *InterpreterCache
	transferFunction      *TransferFunction
	settings              InterpreterSettings
	background            Color
	xref                  *syntax.XRef
	nestingDepth          uint32
	run                   *runState
}

// String is the Debug implementation of SoftMask in hayro-interpret
// src/x_object/soft_mask.rs.
func (m *SoftMask) String() string {
	return fmt.Sprintf("SoftMask(%x)", m.dictKey[:])
}

// CacheKey is the CacheKey implementation of SoftMask in hayro-interpret
// src/x_object/soft_mask.rs. It hashes the dictionary, the resources of the
// parent when the group has none, the root transform and the box.
func (m *SoftMask) CacheKey() Key128 {
	h := newKeyHasher().key(m.dictKey).boolean(m.hasParentResourcesKey)
	if m.hasParentResourcesKey {
		h.key(m.parentResourcesKey)
	}
	return h.key(affineCacheKey(m.rootTransform)).key(rectCacheKey(m.bbox)).sum()
}

// Equal is the PartialEq implementation of SoftMask in hayro-interpret
// src/x_object/soft_mask.rs.
func (m *SoftMask) Equal(other *SoftMask) bool {
	return m.CacheKey() == other.CacheKey()
}

// newSoftMask is SoftMask::new in hayro-interpret
// src/x_object/soft_mask.rs.
func newSoftMask(dict syntax.Dict, ctx *Context, parentResources *syntax.Resources) (*SoftMask, bool) {
	groupStream, ok := dict.Get[syntax.Stream](syntax.KeyG)
	if !ok {
		return nil, false
	}
	group, ok := newFormXObject(groupStream)
	if !ok {
		return nil, false
	}
	var transferFunction *TransferFunction
	if o, ok := dict.Get[syntax.Object](syntax.KeyTR); ok {
		if f, ok := NewFunction(o); ok {
			transferFunction = newTransferFunction(f)
		}
	}
	s, ok := dict.Get[syntax.Name](syntax.KeyS)
	if !ok {
		return nil, false
	}
	var maskType MaskType
	var background Color
	switch s {
	case syntax.KeyLuminosity:
		groupDict, ok := group.dict.Get[syntax.Dict](syntax.KeyGroup)
		if !ok {
			return nil, false
		}
		csObj, ok := groupDict.Get[syntax.Object](syntax.KeyCS)
		if !ok {
			return nil, false
		}
		cs, ok := newColorSpace(csObj, ctx.interpreterCache.objectCache)
		if !ok {
			return nil, false
		}
		if c, ok := dict.Get[[]float32](syntax.KeyBC); ok {
			background = newColor(cs, c, 1)
		} else {
			background = newColor(DeviceGray(), ColorComponents{0}, 1)
		}
		maskType = MaskTypeLuminosity
	case syntax.KeyAlpha:
		// The background color is only for luminosity masks.
		background = newColor(DeviceGray(), ColorComponents{0}, 1)
		maskType = MaskTypeAlpha
	default:
		return nil, false
	}
	m := &SoftMask{
		dictKey:          dictCacheKey(dict),
		group:            group,
		maskType:         maskType,
		rootTransform:    ctx.ctm(),
		transferFunction: transferFunction,
		bbox:             ctx.currentBbox(),
		interpreterCache: ctx.interpreterCache,
		settings:         ctx.settings,
		xref:             ctx.xref,
		background:       background,
		parentResources:  parentResources,
		nestingDepth:     ctx.nestingDepth + 1,
		run:              ctx.run,
	}
	if group.resources == nil {
		m.parentResourcesKey, m.hasParentResourcesKey = resourcesCacheKey(parentResources), true
	}
	return m, true
}

// Interpret is SoftMask::interpret in hayro-interpret
// src/x_object/soft_mask.rs. It runs the group of the mask into the
// device.
func (m *SoftMask) Interpret(device Device) {
	defer recoverInto(m.run)
	s := newState(m.rootTransform)
	ctx := newContextWith(m.rootTransform, m.bbox, m.interpreterCache, m.xref, m.settings, s, m.nestingDepth, m.run)
	m.group.draw(m.parentResources, ctx, device)
}

// MaskType is SoftMask::mask_type in hayro-interpret
// src/x_object/soft_mask.rs.
func (m *SoftMask) MaskType() MaskType {
	return m.maskType
}

// BackgroundColor is SoftMask::background_color in hayro-interpret
// src/x_object/soft_mask.rs. It returns the color against which the mask
// is composited.
func (m *SoftMask) BackgroundColor() Color {
	return m.background
}

// TransferFunction is SoftMask::transfer_function in hayro-interpret
// src/x_object/soft_mask.rs. It returns the transfer function of the mask,
// or nil.
func (m *SoftMask) TransferFunction() *TransferFunction {
	return m.transferFunction
}
