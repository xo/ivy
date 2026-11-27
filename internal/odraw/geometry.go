package odraw

import (
	"strings"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/drawingml"
)

// geoSize is the size of the space of the geometry of a shape, and of its
// adjust values.
const geoSize = 21600

// The types of shapes of MS-ODRAW 2.4.24 that are not a preset of their own.
const (
	SptNotPrimitive  = 0
	SptRectangle     = 1
	SptRoundRect     = 2
	SptLine          = 20
	SptArc           = 19
	SptStraightConn  = 32
	SptPictureFrame  = 75
	SptHostControl   = 201
	SptTextBox       = 202
	SptTextEffectMin = 136
	SptTextEffectMax = 175
)

// The types of shapes of MS-ODRAW 2.4.24 whose adjust values the package
// converts, and the ellipse.
const (
	SptEllipse       = 3
	sptTriangle      = 5
	sptParallelogram = 7
	sptHexagon       = 9
)

// presets maps the types of shapes of MS-ODRAW 2.4.24 onto the preset
// shapes of ECMA-376 that draw them. A type that it does not hold draws as
// a rectangle.
var presets = map[int]string{
	1: "rect", 2: "roundRect", 3: "ellipse", 4: "diamond", 5: "triangle",
	6: "rtTriangle", 7: "parallelogram", 8: "trapezoid", 9: "hexagon",
	10: "octagon", 11: "plus", 12: "star5", 13: "rightArrow", 14: "rightArrow",
	15: "homePlate", 16: "cube", 19: "arc", 21: "plaque", 22: "can",
	23: "donut", 33: "bentConnector2", 34: "bentConnector3",
	35: "bentConnector4", 36: "bentConnector5", 37: "curvedConnector2",
	38: "curvedConnector3", 39: "curvedConnector4", 40: "curvedConnector5",
	41: "callout1", 42: "callout2", 43: "callout3", 44: "accentCallout1",
	45: "accentCallout2", 46: "accentCallout3", 47: "borderCallout1",
	48: "borderCallout2", 49: "borderCallout3", 50: "accentBorderCallout1",
	51: "accentBorderCallout2", 52: "accentBorderCallout3", 53: "ribbon",
	54: "ribbon2", 55: "chevron", 56: "pentagon", 57: "noSmoking",
	58: "star8", 59: "star16", 60: "star32", 61: "wedgeRectCallout",
	62: "wedgeRoundRectCallout", 63: "wedgeEllipseCallout", 64: "wave",
	65: "foldedCorner", 66: "leftArrow", 67: "downArrow", 68: "upArrow",
	69: "leftRightArrow", 70: "upDownArrow", 71: "irregularSeal1",
	72: "irregularSeal2", 73: "lightningBolt", 74: "heart", 76: "quadArrow",
	77: "leftArrowCallout", 78: "rightArrowCallout", 79: "upArrowCallout",
	80: "downArrowCallout", 81: "leftRightArrowCallout",
	82: "upDownArrowCallout", 83: "quadArrowCallout", 84: "bevel",
	85: "leftBracket", 86: "rightBracket", 87: "leftBrace", 88: "rightBrace",
	89: "leftUpArrow", 90: "bentUpArrow", 91: "bentArrow", 92: "star24",
	93: "stripedRightArrow", 94: "notchedRightArrow", 95: "blockArc",
	96: "smileyFace", 97: "verticalScroll", 98: "horizontalScroll",
	99: "circularArrow", 101: "uturnArrow", 102: "curvedRightArrow",
	103: "curvedLeftArrow", 104: "curvedUpArrow", 105: "curvedDownArrow",
	106: "cloudCallout", 107: "ellipseRibbon", 108: "ellipseRibbon2",
	109: "flowChartProcess", 110: "flowChartDecision",
	111: "flowChartInputOutput", 112: "flowChartPredefinedProcess",
	113: "flowChartInternalStorage", 114: "flowChartDocument",
	115: "flowChartMultidocument", 116: "flowChartTerminator",
	117: "flowChartPreparation", 118: "flowChartManualInput",
	119: "flowChartManualOperation", 120: "flowChartConnector",
	121: "flowChartPunchedCard", 122: "flowChartPunchedTape",
	123: "flowChartSummingJunction", 124: "flowChartOr",
	125: "flowChartCollate", 126: "flowChartSort", 127: "flowChartExtract",
	128: "flowChartMerge", 129: "flowChartOfflineStorage",
	130: "flowChartOnlineStorage", 131: "flowChartMagneticTape",
	132: "flowChartMagneticDisk", 133: "flowChartMagneticDrum",
	134: "flowChartDisplay", 135: "flowChartDelay",
	176: "flowChartAlternateProcess", 177: "flowChartOffpageConnector",
	178: "callout1", 179: "accentCallout1", 180: "borderCallout1",
	181: "accentBorderCallout1",
	182: "leftRightUpArrow", 183: "sun", 184: "moon", 185: "bracketPair",
	186: "bracePair", 187: "star4", 188: "doubleWave",
	189: "actionButtonBlank", 190: "actionButtonHome",
	191: "actionButtonHelp", 192: "actionButtonInformation",
	193: "actionButtonForwardNext", 194: "actionButtonBackPrevious",
	195: "actionButtonEnd", 196: "actionButtonBeginning",
	197: "actionButtonReturn", 198: "actionButtonDocument",
	199: "actionButtonSound", 200: "actionButtonMovie",
}

// Geometry returns the paths of the shape sh with the properties c in its
// box of w by h points, and whether its fill draws. A line and a connector
// are not filled.
func (s *Style) Geometry(sh *Shape, c Chain, w, h float64) ([]drawingml.Path, bool) {
	box, _ := okage.RectFromXYWH(0, 0, float32(max(w, 1e-3)), float32(max(h, 1e-3)))
	rectPath := []drawingml.Path{{Path: okage.PathFromRect(box), Stroke: true}}
	if c.Complex(PropVertices) != nil {
		if paths, ok := custom(c, w, h); ok {
			return paths, true
		}
	}
	switch sh.Type {
	case SptLine, SptStraightConn:
		pb := okage.NewPathBuilder()
		pb.MoveTo(0, 0)
		pb.LineTo(float32(w), float32(h))
		p, ok := pb.Finish()
		if !ok {
			return nil, false
		}
		return []drawingml.Path{{Path: p, Fill: drawingml.FillNone, Stroke: true}}, false
	case SptNotPrimitive, SptPictureFrame, SptTextBox, SptHostControl:
		return rectPath, true
	}
	name, ok := presets[sh.Type]
	if !ok {
		return rectPath, true
	}
	g, ok := drawingml.Preset(name)
	if !ok {
		return rectPath, true
	}
	f := frameOf(sh.Type, c, w, h)
	paths, _ := drawingml.Evaluated(g, adjustOf(sh.Type, c, g.AdjustNames(), w, h), f.w, f.h, s.log())
	if len(paths) == 0 {
		return rectPath, true
	}
	if f != (frame{w: w, h: h}) {
		t := f.transform()
		for i := range paths {
			if p, ok := paths[i].Path.Transform(t); ok {
				paths[i].Path = p
			}
		}
	}
	open := name == "arc" || strings.HasPrefix(name, "bentConnector") || strings.HasPrefix(name, "curvedConnector")
	return paths, !open
}

// TextRect returns the box of the text of a shape of the type of sh with
// the properties c and the size w by h points: the text rectangle of its
// preset geometry, such as the rectangle inside an ellipse, or the whole
// box. A shape of custom geometry and a text box have the whole box.
func (s *Style) TextRect(sh *Shape, c Chain, w, h float64) okage.Rect {
	box, _ := okage.RectFromXYWH(0, 0, float32(max(w, 1e-3)), float32(max(h, 1e-3)))
	if c.Complex(PropVertices) != nil {
		return box
	}
	name, ok := presets[sh.Type]
	if !ok {
		return box
	}
	g, ok := drawingml.Preset(name)
	if !ok {
		return box
	}
	f := frameOf(sh.Type, c, w, h)
	_, text := drawingml.Evaluated(g, adjustOf(sh.Type, c, g.AdjustNames(), w, h), f.w, f.h, s.log())
	if f == (frame{w: w, h: h}) {
		return text
	}
	x0, y0 := f.point(text.Left(), text.Top())
	x1, y1 := f.point(text.Right(), text.Bottom())
	r, ok := okage.RectFromLTRB(min(x0, x1), min(y0, y1), max(x0, x1), max(y0, y1))
	if !ok {
		return box
	}
	return r
}

// The types of the segments of a path of MS-ODRAW 2.4.30.
const (
	segLineTo  = 0
	segCurveTo = 1
	segMoveTo  = 2
	segClose   = 3
	segEnd     = 4
	segEscape  = 5
)

// The escapes of a path that the package reads (MS-ODRAW 2.4.29).
const (
	escNoFill = 0x0A
	escNoLine = 0x0B
)

// custom returns the paths of the custom geometry of the properties c: its
// vertices, and the segments that join them (MS-ODRAW 2.3.6).
func custom(c Chain, w, h float64) ([]drawingml.Path, bool) {
	verts := Array(c.Complex(PropVertices), 4)
	if len(verts) == 0 {
		return nil, false
	}
	left := float64(int32(c.Value(PropGeoLeft, 0)))
	top := float64(int32(c.Value(PropGeoTop, 0)))
	right := float64(int32(c.Value(PropGeoRight, geoSize)))
	bottom := float64(int32(c.Value(PropGeoBottom, geoSize)))
	sx, sy := w/nonzero(right-left), h/nonzero(bottom-top)
	pt := func(i int) (float32, float32, bool) {
		if i >= len(verts) {
			return 0, 0, false
		}
		v := verts[i]
		var x, y float64
		if len(v) >= 8 {
			x, y = float64(int32(le.Uint32(v))), float64(int32(le.Uint32(v[4:])))
		} else {
			x, y = float64(int16(le.Uint16(v))), float64(int16(le.Uint16(v[2:])))
		}
		return float32((x - left) * sx), float32((y - top) * sy), true
	}
	segs := Array(c.Complex(PropSegmentInfo), 2)
	var paths []drawingml.Path
	pb := okage.NewPathBuilder()
	fill, stroke := true, true
	started := false
	flush := func() {
		if !started {
			return
		}
		if p, ok := pb.Finish(); ok {
			fm := drawingml.FillNorm
			if !fill {
				fm = drawingml.FillNone
			}
			paths = append(paths, drawingml.Path{Path: p, Fill: fm, Stroke: stroke})
		}
		pb = okage.NewPathBuilder()
		fill, stroke, started = true, true, false
	}
	vi := 0
	if len(segs) == 0 {
		// without segments, the vertices make one line
		for i := range verts {
			x, y, _ := pt(i)
			if i == 0 {
				pb.MoveTo(x, y)
			} else {
				pb.LineTo(x, y)
			}
			started = true
		}
		flush()
		return paths, len(paths) > 0
	}
	for _, s := range segs {
		if len(s) < 2 {
			// a segment of a malformed file
			continue
		}
		v := le.Uint16(s)
		typ, n := int(v>>13), int(v&0x1FFF)
		switch typ {
		case segMoveTo:
			if x, y, ok := pt(vi); ok {
				pb.MoveTo(x, y)
				started = true
			}
			vi++
		case segLineTo:
			for k := 0; k < max(n, 1) && vi < len(verts); k++ {
				if x, y, ok := pt(vi); ok {
					if !started {
						pb.MoveTo(x, y)
						started = true
					} else {
						pb.LineTo(x, y)
					}
				}
				vi++
			}
		case segCurveTo:
			for k := 0; k < max(n, 1) && vi < len(verts); k++ {
				x1, y1, ok1 := pt(vi)
				x2, y2, ok2 := pt(vi + 1)
				x3, y3, ok3 := pt(vi + 2)
				if ok1 && ok2 && ok3 && started {
					pb.CubicTo(x1, y1, x2, y2, x3, y3)
				}
				vi += 3
			}
		case segClose:
			if started {
				pb.Close()
			}
		case segEnd:
			flush()
		case segEscape:
			switch (v >> 8) & 0x1F {
			case escNoFill:
				fill = false
			case escNoLine:
				stroke = false
			}
			vi += int(v & 0xFF)
		default:
			vi += int(v & 0xFF)
		}
		if vi > len(verts) {
			break
		}
	}
	flush()
	return paths, len(paths) > 0
}
