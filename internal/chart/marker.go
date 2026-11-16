package chart

import (
	"image/color"
	"math"

	"github.com/xo/okage"
)

// autoMarkers are the markers that the series of a line chart take in
// turn, as LibreOffice gives them to a chart of Office.
var autoMarkers = []string{"square", "diamond", "triangle", "x", "star", "circle", "plus", "dash", "dot"}

// markerOf returns the marker of the points of a series, or empty for
// none.
func (cv *canvas) markerOf(p *plot, s *series) string {
	if s.noMarker || s.marker == "none" {
		return ""
	}
	if s.marker != "" && s.marker != "auto" {
		return s.marker
	}
	if !p.markers || p.hiLow || p.upDown {
		return ""
	}
	if cv.c.look == lookModern {
		return "circle"
	}
	return autoMarkers[max(0, s.index)%len(autoMarkers)]
}

// markerSize returns the size of the markers of a series, in points.
func (cv *canvas) markerSize(_ *plot, s *series) float64 {
	if s.markerSize > 0 {
		return s.markerSize
	}
	if cv.c.look == lookLibre {
		return 7
	}
	return 5
}

// drawsLine says whether a series of a line, a scatter or a radar chart
// draws its line.
func (cv *canvas) drawsLine(p *plot, s *series) bool {
	if s.noLine {
		return false
	}
	switch p.kind {
	case kindScatter:
		return p.lines && !p.bubbles
	case kindLine:
		return !p.hiLow || s.hasLine
	}
	return true
}

// marker draws a marker of a series at (x, y), of a size in points. line
// is the color of a marker that the series does not color.
func (cv *canvas) marker(symbol string, x, y, size float64, s *series, line color.NRGBA) {
	if symbol == "" || !(size > 0) || math.IsNaN(x) || math.IsNaN(y) {
		return
	}
	fill, edge := line, line
	if s != nil {
		fill, edge = s.color, s.color
		if s.hasMarkerFill {
			fill = s.markerFill
		}
		if s.hasMarkerLine {
			edge = s.markerLine
		}
	}
	r := size / 2
	pb := okage.NewPathBuilder()
	fx, fy, fr := float32(x), float32(y), float32(r)
	filled := true
	switch symbol {
	case "circle", "dot":
		if symbol == "dot" {
			fr *= 0.6
		}
		pb.PushCircle(fx, fy, fr)
	case "diamond":
		pb.MoveTo(fx, fy-fr*1.2)
		pb.LineTo(fx+fr*1.2, fy)
		pb.LineTo(fx, fy+fr*1.2)
		pb.LineTo(fx-fr*1.2, fy)
		pb.Close()
	case "triangle":
		pb.MoveTo(fx, fy-fr*1.15)
		pb.LineTo(fx+fr*1.15, fy+fr)
		pb.LineTo(fx-fr*1.15, fy+fr)
		pb.Close()
	case "x":
		pb.MoveTo(fx-fr, fy-fr)
		pb.LineTo(fx+fr, fy+fr)
		pb.MoveTo(fx+fr, fy-fr)
		pb.LineTo(fx-fr, fy+fr)
		filled = false
	case "plus":
		pb.MoveTo(fx-fr, fy)
		pb.LineTo(fx+fr, fy)
		pb.MoveTo(fx, fy-fr)
		pb.LineTo(fx, fy+fr)
		filled = false
	case "star":
		pb.MoveTo(fx-fr, fy-fr)
		pb.LineTo(fx+fr, fy+fr)
		pb.MoveTo(fx+fr, fy-fr)
		pb.LineTo(fx-fr, fy+fr)
		pb.MoveTo(fx, fy-fr)
		pb.LineTo(fx, fy+fr)
		filled = false
	case "dash":
		pb.MoveTo(fx-fr, fy)
		pb.LineTo(fx+fr, fy)
		filled = false
	default:
		if rc, ok := okage.RectFromLTRB(fx-fr, fy-fr, fx+fr, fy+fr); ok {
			pb.PushRect(rc)
		}
	}
	path, ok := pb.Finish()
	if !ok {
		return
	}
	if filled {
		cv.fill(path, fill)
		cv.stroke(path, edge, 0.75, "", false)
		return
	}
	cv.stroke(path, edge, max(1, size/5), "", false)
}
