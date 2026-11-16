// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports the tests of hayro-jpeg2000 src/j2c/tile.rs.

import "testing"

// TestJPEG2000StandardExampleB4 is test_jpeg2000_standard_example_b4.
func TestJPEG2000StandardExampleB4(t *testing.T) {
	componentSizeInfo0 := componentSizeInfo{precision: 8, horizontalResolution: 1, verticalResolution: 1}
	dummyCodingStyle := codingStyleComponent{
		parameters: codingStyleParameters{transformation: irreversible97},
	}
	dummyQuantization := quantizationInfo{quantizationStyle: noQuantization}

	componentInfo0 := componentInfo{
		sizeInfo:         componentSizeInfo0,
		codingStyle:      dummyCodingStyle,
		quantizationInfo: dummyQuantization,
	}
	componentSizeInfo1 := componentSizeInfo{precision: 8, horizontalResolution: 2, verticalResolution: 2}
	componentInfo1 := componentInfo{
		sizeInfo:         componentSizeInfo1,
		codingStyle:      dummyCodingStyle,
		quantizationInfo: dummyQuantization,
	}

	sd := sizeData{
		referenceGridWidth:      1432,
		referenceGridHeight:     954,
		imageAreaXOffset:        152,
		imageAreaYOffset:        234,
		tileWidth:               396,
		tileHeight:              297,
		componentSizes:          []componentSizeInfo{componentSizeInfo0, componentSizeInfo1},
		xShrinkFactor:           1,
		yShrinkFactor:           1,
		xResolutionShrinkFactor: 1,
		yResolutionShrinkFactor: 1,
	}

	eq := func(what string, got, want uint32) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %d, want %d", what, got, want)
		}
	}

	eq("image_width", sd.imageWidth(), 1280)
	eq("image_height", sd.imageHeight(), 720)
	eq("num_x_tiles", sd.numXTiles(), 4)
	eq("num_y_tiles", sd.numYTiles(), 4)
	n, err := sd.numTiles()
	if err != nil {
		t.Fatal(err)
	}
	eq("num_tiles", n, 16)

	h := &header{
		sizeData: sd,
		globalCodingStyle: codingStyleDefault{
			progressionOrder:    layerResolutionComponentPosition,
			componentParameters: dummyCodingStyle,
		},
	}

	rect := func(idx uint32, info *componentInfo) intRect {
		tl := newTile(idx, h)
		return newComponentTile(&tl, info).rect
	}
	checkRect := func(what string, r intRect, x0, y0, x1, y1, w, hh uint32) {
		t.Helper()
		eq(what+".x0", r.x0, x0)
		eq(what+".y0", r.y0, y0)
		eq(what+".x1", r.x1, x1)
		eq(what+".y1", r.y1, y1)
		eq(what+".width", r.width(), w)
		eq(what+".height", r.height(), hh)
	}

	checkRect("coords_0_0", rect(0, &componentInfo0), 152, 234, 396, 297, 244, 63)
	checkRect("coords_1_0", rect(1, &componentInfo0), 396, 234, 792, 297, 396, 63)
	checkRect("coords_0_1", rect(4, &componentInfo0), 152, 297, 396, 594, 244, 297)
	checkRect("coords_1_1", rect(5, &componentInfo0), 396, 297, 792, 594, 396, 297)
	checkRect("coords_3_3", rect(15, &componentInfo0), 1188, 891, 1432, 954, 244, 63)

	checkRect("tile_0_0_comp1", rect(0, &componentInfo1), 76, 117, 198, 149, 122, 32)
	checkRect("tile_1_0_comp1", rect(1, &componentInfo1), 198, 117, 396, 149, 198, 32)
	checkRect("tile_0_1_comp1", rect(4, &componentInfo1), 76, 149, 198, 297, 122, 148)
	tile11 := rect(5, &componentInfo1)
	checkRect("tile_1_1_comp1", tile11, 198, 149, 396, 297, 198, 148)
	tile21 := rect(6, &componentInfo1)
	checkRect("tile_2_1_comp1", tile21, 396, 149, 594, 297, 198, 148)

	eq("tile_1_1_comp1.width == tile_2_1_comp1.width", tile11.width(), tile21.width())
	eq("tile_1_1_comp1.height == tile_2_1_comp1.height", tile11.height(), tile21.height())
}
