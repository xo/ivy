// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/progression.rs.

import (
	"cmp"
	"slices"
)

// progressionData is ProgressionData in hayro-jpeg2000
// src/j2c/progression.rs. It names one packet.
type progressionData struct {
	layerNum   uint8
	resolution uint8
	component  uint8
	precinct   uint64
}

// progressionIter is the Box<dyn Iterator<Item = ProgressionData>> of
// upstream. It returns the next packet, and false at the end.
type progressionIter func() (progressionData, bool)

// iteratorInput is IteratorInput in hayro-jpeg2000 src/j2c/progression.rs.
// Each pair holds the start and the end, without the end.
type iteratorInput struct {
	layers      [2]uint8
	tile        *tile
	resolutions [2]uint8
	components  [2]uint8
}

// newIteratorInput is IteratorInput::new.
func newIteratorInput(t *tile) iteratorInput {
	return newIteratorInputWithCustomBounds(t, [2]uint8{0, 0xff}, [2]uint8{0, 0xff}, [2]uint8{0, 0xff})
}

// newIteratorInputWithCustomBounds is IteratorInput::new_with_custom_bounds.
func newIteratorInputWithCustomBounds(t *tile, resolutions, layers, components [2]uint8) iteratorInput {
	var maxResolution uint8
	for i := range t.componentInfos {
		maxResolution = max(maxResolution, t.componentInfos[i].codingStyle.parameters.numResolutionLevels)
	}
	maxLayer := t.numLayers
	maxComponent := uint8(len(t.componentInfos))

	resolutions[1] = min(resolutions[1], maxResolution)
	layers[1] = min(layers[1], maxLayer)
	components[1] = min(components[1], maxComponent)

	return iteratorInput{layers: layers, tile: t, resolutions: resolutions, components: components}
}

func (in *iteratorInput) minLayer() uint8           { return in.layers[0] }
func (in *iteratorInput) maxLayer() uint8           { return in.layers[1] }
func (in *iteratorInput) minResolution() uint8      { return in.resolutions[0] }
func (in *iteratorInput) totalMaxResolution() uint8 { return in.resolutions[1] }
func (in *iteratorInput) minComp() uint8            { return in.components[0] }
func (in *iteratorInput) maxComp() uint8            { return in.components[1] }

// maxResolution is IteratorInput::max_resolution. The field resolutions
// holds the maximum over all components.
func (in *iteratorInput) maxResolution(componentIdx uint8) uint8 {
	return min(in.totalMaxResolution(), in.tile.componentInfos[componentIdx].numResolutionLevels())
}

// isEmpty is IteratorInput::is_empty.
func (in *iteratorInput) isEmpty() bool {
	return in.layers[0] >= in.layers[1] || in.resolutions[0] >= in.resolutions[1] || in.components[0] >= in.components[1]
}

// componentTiles is IteratorInput::component_tiles.
func (in *iteratorInput) componentTiles() []componentTile {
	out := make([]componentTile, len(in.tile.componentInfos))
	for i := range in.tile.componentInfos {
		out[i] = newComponentTile(in.tile, &in.tile.componentInfos[i])
	}
	return out
}

// numPrecincts is IteratorInput::num_precincts.
func (in *iteratorInput) numPrecincts(cts []componentTile, componentIdx, resolution uint8) uint64 {
	if componentIdx >= in.maxComp() || resolution >= in.maxResolution(componentIdx) {
		return 0
	}
	rt, err := newResolutionTile(cts[componentIdx], resolution)
	if err != nil {
		return 0
	}
	return rt.numPrecincts()
}

// newProgressionIterator is progression_iterator in hayro-jpeg2000
// src/j2c/progression.rs.
func newProgressionIterator(t *tile, h *header, lim *limits) (progressionIter, error) {
	changes := h.progressionChanges
	if len(t.progressionChanges) != 0 {
		changes = t.progressionChanges
	}

	if len(changes) == 0 {
		return progressionOrderIterator(newIteratorInput(t), t.progressionOrder, lim)
	}

	clamp := func(v uint16) uint8 {
		if v > 0xff {
			return 0xff
		}
		return uint8(v)
	}

	iterators := make([]progressionIter, 0, len(changes))
	for _, c := range changes {
		in := newIteratorInputWithCustomBounds(
			t,
			[2]uint8{c.resolutionStart, c.resolutionEnd},
			[2]uint8{0, clamp(c.layerEnd)},
			[2]uint8{clamp(c.componentStart), clamp(c.componentEnd)},
		)
		it, err := progressionOrderIterator(in, c.progressionOrder, lim)
		if err != nil {
			return nil, err
		}
		iterators = append(iterators, it)
	}

	// Skip packets already emitted by an earlier progression change.
	included := make(map[progressionData]struct{})
	cur := 0
	return func() (progressionData, bool) {
		for cur < len(iterators) {
			d, ok := iterators[cur]()
			if !ok {
				cur++
				continue
			}
			if _, seen := included[d]; seen {
				continue
			}
			included[d] = struct{}{}
			return d, true
		}
		return progressionData{}, false
	}, nil
}

// progressionOrderIterator is progression_order_iterator in hayro-jpeg2000
// src/j2c/progression.rs.
func progressionOrderIterator(in iteratorInput, order progressionOrder, lim *limits) (progressionIter, error) {
	var sort func(p, s *precinctStore) int
	switch order {
	case layerResolutionComponentPosition:
		return layerResolutionComponentPositionProgression(in), nil
	case resolutionLayerComponentPosition:
		return resolutionLayerComponentPositionProgression(in), nil
	case resolutionPositionComponentLayer:
		sort = func(p, s *precinctStore) int {
			return cmp.Or(
				cmp.Compare(p.resolution, s.resolution),
				cmp.Compare(p.precinctY, s.precinctY),
				cmp.Compare(p.precinctX, s.precinctX),
				cmp.Compare(p.componentIdx, s.componentIdx),
				cmp.Compare(p.precinctIdx, s.precinctIdx),
			)
		}
	case positionComponentResolutionLayer:
		sort = func(p, s *precinctStore) int {
			return cmp.Or(
				cmp.Compare(p.precinctY, s.precinctY),
				cmp.Compare(p.precinctX, s.precinctX),
				cmp.Compare(p.componentIdx, s.componentIdx),
				cmp.Compare(p.resolution, s.resolution),
				cmp.Compare(p.precinctIdx, s.precinctIdx),
			)
		}
	case componentPositionResolutionLayer:
		sort = func(p, s *precinctStore) int {
			return cmp.Or(
				cmp.Compare(p.componentIdx, s.componentIdx),
				cmp.Compare(p.precinctY, s.precinctY),
				cmp.Compare(p.precinctX, s.precinctX),
				cmp.Compare(p.resolution, s.resolution),
				cmp.Compare(p.precinctIdx, s.precinctIdx),
			)
		}
	default:
		return nil, errInvalidProgressionIterator
	}
	it, err := positionProgressionCommon(in, sort, lim)
	if err != nil {
		return nil, err
	}
	if it == nil {
		return nil, errInvalidProgressionIterator
	}
	return it, nil
}

// layerResolutionComponentPositionProgression is
// layer_resolution_component_position_progression in hayro-jpeg2000
// src/j2c/progression.rs.
func layerResolutionComponentPositionProgression(in iteratorInput) progressionIter {
	cts := in.componentTiles()

	layer := in.minLayer()
	resolution := in.minResolution()
	componentIdx := in.minComp()
	numPrecincts := in.numPrecincts(cts, componentIdx, resolution)
	var precinct uint64

	return func() (progressionData, bool) {
		if in.isEmpty() || layer >= in.maxLayer() {
			return progressionData{}, false
		}

		for precinct == numPrecincts {
			precinct = 0
			componentIdx++
			if componentIdx == in.maxComp() {
				componentIdx = in.minComp()
				resolution++
				if resolution == in.totalMaxResolution() {
					resolution = in.minResolution()
					layer++
					if layer == in.maxLayer() {
						return progressionData{}, false
					}
				}
			}
			numPrecincts = in.numPrecincts(cts, componentIdx, resolution)
		}

		d := progressionData{layerNum: layer, resolution: resolution, component: componentIdx, precinct: precinct}
		precinct++
		return d, true
	}
}

// resolutionLayerComponentPositionProgression is
// resolution_layer_component_position_progression in hayro-jpeg2000
// src/j2c/progression.rs.
func resolutionLayerComponentPositionProgression(in iteratorInput) progressionIter {
	cts := in.componentTiles()

	layer := in.minLayer()
	resolution := in.minResolution()
	componentIdx := in.minComp()
	numPrecincts := in.numPrecincts(cts, componentIdx, resolution)
	var precinct uint64

	return func() (progressionData, bool) {
		if in.isEmpty() || resolution >= in.totalMaxResolution() {
			return progressionData{}, false
		}

		for precinct == numPrecincts {
			precinct = 0
			componentIdx++
			if componentIdx == in.maxComp() {
				componentIdx = in.minComp()
				layer++
				if layer == in.maxLayer() {
					layer = in.minLayer()
					resolution++
					if resolution == in.totalMaxResolution() {
						return progressionData{}, false
					}
				}
			}
			numPrecincts = in.numPrecincts(cts, componentIdx, resolution)
		}

		d := progressionData{layerNum: layer, resolution: resolution, component: componentIdx, precinct: precinct}
		precinct++
		return d, true
	}
}

// precinctStore is PrecinctStore in hayro-jpeg2000 src/j2c/progression.rs.
type precinctStore struct {
	resolution   uint8
	precinctY    uint32
	precinctX    uint32
	componentIdx uint8
	precinctIdx  uint64
}

// positionProgressionCommon is position_progression_common in
// hayro-jpeg2000 src/j2c/progression.rs. It returns nil when a resolution
// has no valid precincts.
//
// Port: the limits bound the number of precincts that it sorts.
func positionProgressionCommon(in iteratorInput, sort func(p, s *precinctStore) int, lim *limits) (progressionIter, error) {
	cts := in.componentTiles()
	var elements []precinctStore

	for componentIdx := in.minComp(); componentIdx < in.maxComp(); componentIdx++ {
		if int(componentIdx) >= len(cts) {
			return nil, nil
		}
		ct := cts[componentIdx]
		for resolution := in.minResolution(); resolution < in.maxResolution(componentIdx); resolution++ {
			rt, err := newResolutionTile(ct, resolution)
			if err != nil {
				return nil, err
			}
			if err := lim.objects(uint64(len(elements)) + rt.numPrecincts()); err != nil {
				return nil, err
			}
			seq, ok := rt.precincts()
			if !ok {
				return nil, nil
			}
			for d := range seq {
				elements = append(elements, precinctStore{
					precinctY:    d.rY,
					precinctX:    d.rX,
					componentIdx: componentIdx,
					resolution:   resolution,
					precinctIdx:  d.idx,
				})
			}
		}
	}

	slices.SortStableFunc(elements, func(a, b precinctStore) int {
		return sort(&a, &b)
	})

	i := 0
	layer := in.minLayer()
	return func() (progressionData, bool) {
		for i < len(elements) {
			if layer >= in.maxLayer() {
				i++
				layer = in.minLayer()
				continue
			}
			e := &elements[i]
			d := progressionData{layerNum: layer, resolution: e.resolution, component: e.componentIdx, precinct: e.precinctIdx}
			layer++
			return d, true
		}
		return progressionData{}, false
	}, nil
}
