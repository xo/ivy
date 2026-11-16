// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/j2c/mct.rs.

// applyInverse is apply_inverse in hayro-jpeg2000 src/j2c/mct.rs. It runs
// the inverse multiple component transformation on the first three
// components.
func applyInverse(components []ComponentData, infos []componentInfo, h *header, c cancel) error {
	if len(components) < 3 || len(infos) < 3 {
		if h.strict {
			return errMct
		}
		return nil
	}

	s0, s1, s2 := components[0].container.data, components[1].container.data, components[2].container.data

	transform := infos[0].waveletTransform()
	if transform != infos[1].waveletTransform() || infos[1].waveletTransform() != infos[2].waveletTransform() {
		return errMct
	}
	if len(s0) != len(s1) || len(s1) != len(s2) {
		return errMct
	}

	for start := 0; start < len(s0); start += chunk {
		if err := c.check(); err != nil {
			return err
		}
		end := min(start+chunk, len(s0))
		applyInner(transform, s0[start:end], s1[start:end], s2[start:end])
	}
	return nil
}

// applyInner is apply_inner in hayro-jpeg2000 src/j2c/mct.rs, with
// apply_inner_impl in its scalar version. Like upstream, it leaves out the
// samples after the last full block of simdWidth samples.
func applyInner(transform waveletTransform, s0, s1, s2 []float32) {
	n := len(s0) / simdWidth * simdWidth
	s0, s1, s2 = s0[:n], s1[:n], s2[:n]

	switch transform {
	case irreversible97:
		for i := range s0 {
			y0, y1, y2 := s0[i], s1[i], s2[i]
			s0[i] = mulAdd(y2, 1.402, y0)
			s1[i] = mulAdd(y2, -0.71414, mulAdd(y1, -0.34413, y0))
			s2[i] = mulAdd(y1, 1.772, y0)
		}
	case reversible53:
		for i := range s0 {
			y0, y1, y2 := s0[i], s1[i], s2[i]
			i1 := y0 - floorF32((y2+y1)*0.25)
			s0[i] = y2 + i1
			s1[i] = i1
			s2[i] = y1 + i1
		}
	}
}
