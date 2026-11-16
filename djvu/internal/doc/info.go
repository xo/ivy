// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/info.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"encoding/binary"
	"fmt"
)

// Info is the content of an INFO chunk.
type Info struct {
	Width, Height int
	Version       int // minor | major<<8
	DPI           int
	Gamma         int // tenths
	Rotate        int // clockwise degrees, as PDF /Rotate
}

// ParseInfo decodes an INFO chunk. Missing or out-of-range fields
// get defaults: 300 dpi, gamma 2.2 clamped to 0.3..5.0.
func ParseInfo(b []byte) (Info, error) {
	if len(b) < 4 {
		return Info{}, fmt.Errorf("djvu: INFO chunk too short (%d bytes)", len(b))
	}
	in := Info{
		Width:  int(binary.BigEndian.Uint16(b[0:])),
		Height: int(binary.BigEndian.Uint16(b[2:])),
		DPI:    300,
		Gamma:  22,
	}
	if len(b) >= 6 {
		in.Version = int(b[4]) | int(b[5])<<8
	} else if len(b) == 5 {
		in.Version = int(b[4])
	}
	if len(b) >= 8 {
		if d := int(binary.LittleEndian.Uint16(b[6:])); d >= 25 && d <= 6000 {
			in.DPI = d
		}
	}
	if len(b) >= 9 {
		in.Gamma = min(max(int(b[8]), 3), 50)
	}
	if len(b) >= 10 {
		switch b[9] & 7 {
		case 6: // 90° counter-clockwise
			in.Rotate = 270
		case 2:
			in.Rotate = 180
		case 5: // 90° clockwise
			in.Rotate = 90
		}
	}
	return in, nil
}
