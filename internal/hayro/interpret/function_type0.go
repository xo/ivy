// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/function/type0.rs.

package interpret

import (
	"math"

	"github.com/xo/ivy/internal/hayro/syntax"
)

// maxSampledValues is the largest number of samples of a function of type
// 0.
//
// Port: upstream bounds the samples by the length of the data, but a
// stream of 512 MiB holds 4 billion samples of 1 bit, which take 16 GiB as
// f32. The cap keeps the memory bounded (D18).
const maxSampledValues = 1 << 26

// functionType0 is Type0 in hayro-interpret src/function/type0.rs. It is a
// sampled function.
type functionType0 struct {
	inputs      []sampledInput
	decode      []interval
	sampleRange interval
	samples     []float32
}

// sampledInput is SampledInput in hayro-interpret src/function/type0.rs.
type sampledInput struct {
	domain interval
	encode interval
	size   uint32
	stride int
}

// samplePosition is SampledInput::sample_position in hayro-interpret
// src/function/type0.rs. It returns the index of the sample before x, and
// the fraction to the next sample, if x is between two samples.
func (s *sampledInput) samplePosition(x float32) (int, float32, bool) {
	last := s.size - 1
	position := rustMax(s.domain.mapTo(x, s.encode), 0)
	floor := float32(math.Floor(float64(position)))
	index := min(asU32(floor), last)
	fraction := position - floor
	return int(index), fraction, index < last && fraction > 0
}

// newFunctionType0 is Type0::new in hayro-interpret src/function/type0.rs.
func newFunctionType0(stream syntax.Stream, domain, rng []interval) (*functionType0, bool) {
	dict := stream.Dict()
	bitsPerSample, ok := dict.Get[uint8](syntax.KeyBitsPerSample)
	if !ok {
		return nil, false
	}
	switch bitsPerSample {
	case 1, 2, 4, 8, 12, 16, 24, 32:
	default:
		return nil, false
	}
	sizeArr, ok := dict.Get[syntax.Array](syntax.KeySize)
	if !ok {
		return nil, false
	}
	sizes := sizeArr.Iter[uint32]().Collect()
	if len(sizes) != len(domain) {
		return nil, false
	}
	for _, s := range sizes {
		if s == 0 {
			return nil, false
		}
	}
	encode, ok := readIntervals(dict, syntax.KeyEncode)
	if !ok {
		encode = make([]interval, len(sizes))
		for i, s := range sizes {
			encode[i] = interval{min: 0, max: float32(s - 1)}
		}
	}
	decode, ok := readIntervals(dict, syntax.KeyDecode)
	if !ok {
		decode = append([]interval(nil), rng...)
	}
	if len(encode) != len(sizes) || len(decode) != len(rng) {
		return nil, false
	}
	stride := len(rng)
	inputs := make([]sampledInput, 0, len(sizes))
	for i, size := range sizes {
		inputs = append(inputs, sampledInput{domain: domain[i], encode: encode[i], size: size, stride: stride})
		if stride != 0 && int(size) > math.MaxInt/stride {
			return nil, false
		}
		stride *= int(size)
	}
	decoded, err := stream.DecodedData()
	if err != nil {
		return nil, false
	}
	if len(decoded)*8/int(bitsPerSample) < stride {
		return nil, false
	}
	// Port: see maxSampledValues.
	if stride > maxSampledValues {
		return nil, false
	}
	reader := syntax.NewBitReader(decoded)
	samples := make([]float32, stride)
	for i := range samples {
		v, ok := reader.Read(bitsPerSample)
		if !ok {
			return nil, false
		}
		samples[i] = float32(v)
	}
	return &functionType0{
		inputs:      inputs,
		decode:      decode,
		sampleRange: interval{min: 0, max: float32(uint64(1)<<bitsPerSample - 1)},
		samples:     samples,
	}, true
}

// eval is Type0::eval in hayro-interpret src/function/type0.rs. It
// interpolates the samples around the input in each dimension.
func (t *functionType0) eval(input, output []float32) {
	offset := 0
	var fractions [maxComponents]struct {
		stride   int
		fraction float32
	}
	numFractions := 0
	for i := 0; i < len(input) && i < len(t.inputs); i++ {
		in := &t.inputs[i]
		index, fraction, ok := in.samplePosition(input[i])
		offset += index * in.stride
		if ok {
			fractions[numFractions].stride = in.stride
			fractions[numFractions].fraction = fraction
			numFractions++
		}
	}
	fs := fractions[:numFractions]
	for i := range output {
		output[i] = 0
	}
	for corner := range 1 << len(fs) {
		weight := float32(1)
		cornerOffset := offset
		for i, f := range fs {
			if corner>>(len(fs)-1-i)&1 == 1 {
				weight *= f.fraction
				cornerOffset += f.stride
			} else {
				weight *= 1 - f.fraction
			}
		}
		end := min(cornerOffset+len(output), len(t.samples))
		for i, s := range t.samples[cornerOffset:end] {
			output[i] += weight * s
		}
	}
	for i := 0; i < len(output) && i < len(t.decode); i++ {
		output[i] = t.sampleRange.mapTo(output[i], t.decode[i])
	}
}
