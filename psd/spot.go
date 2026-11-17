package psd

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"math"

	"github.com/xo/ivy/psd/internal/oovpsd"
)

// The image resources that describe the extra channels of the merged image.
const (
	// resDisplayInfoOld holds the color, the opacity and the kind of each
	// extra channel, in 14 bytes, before Photoshop CS3.
	resDisplayInfoOld = 0x03ef
	// resAlphaIDs holds an identifier of 4 bytes for each extra channel.
	resAlphaIDs = 0x041d
	// resDisplayInfo holds the version, then the color, the opacity and the
	// kind of each extra channel, in 13 bytes.
	resDisplayInfo = 0x0435
	// resAltSpotColors holds the version, the count, then the identifier of
	// a channel and its color, for the spot channels.
	resAltSpotColors = 0x042b
)

// kindSpot is the kind of a spot channel in the display information. The
// kinds 0 and 1 are alpha channels, which hold a selection.
const kindSpot = 2

// The color spaces of the color structure of Photoshop that ivy converts.
const (
	spaceRGB  = 0
	spaceHSB  = 1
	spaceCMYK = 2
	spaceLab  = 7
)

// spot is a spot channel: the ink of the channel, its color and its solidity.
type spot struct {
	// data is the channel, in the depth of the file. A value of 0 is full ink.
	data []byte
	// r, g and b are the color of the ink in sRGB, from 0 to 1.
	r, g, b float64
	// solidity is how much the ink covers the inks below it, from 0 to 1.
	solidity float64
}

// drawSpots draws the spot channels of the merged image of p onto img, in
// their order. A spot channel is an ink of its own color. Photoshop shows it
// over the other inks: with a solidity of 0 the ink multiplies the color
// below it, and with a solidity of 1 the ink covers it. A file whose extra
// channels do not match its display information has no spot channels here.
func drawSpots(ctx context.Context, img *image.NRGBA, p *oovpsd.PSD) error {
	spots := findSpots(p)
	b := img.Bounds()
	if len(spots) == 0 || b.Size() != p.Config.Rect.Size() {
		return nil
	}
	w := b.Dx()
	for _, s := range spots {
		for y := range b.Dy() {
			if y%256 == 0 {
				if err := ctx.Err(); err != nil {
					return fmt.Errorf("drawing psd spot channels: %w", err)
				}
			}
			row := img.Pix[y*img.Stride : y*img.Stride+4*w]
			for x := range w {
				ink := inkAt(s.data, p.Config.Depth, y*w+x)
				if ink <= 0 {
					continue
				}
				s.drawPixel(row[4*x:4*x+4], ink)
			}
		}
	}
	return nil
}

// drawPixel draws an amount of the ink of s, from 0 to 1, onto the pixel px of
// an *image.NRGBA. Over the opaque part of the pixel, the ink mixes a multiply
// with the color below and the color of the ink, by its solidity. Over the
// transparent part, the ink shows its own color.
func (s *spot) drawPixel(px []byte, ink float64) {
	a := float64(px[3]) / 255
	ink = min(ink, 1)
	out := a + ink*(1-a)
	if out <= 0 {
		return
	}
	for i, c := range [3]float64{s.r, s.g, s.b} {
		under := float64(px[i]) / 255
		mixed := under*c*(1-s.solidity) + c*s.solidity
		// the premultiplied color, then back to a straight one
		v := (1-ink)*a*under + ink*(a*mixed+(1-a)*c)
		px[i] = byte(math.Round(min(max(v/out, 0), 1) * 255))
	}
	px[3] = byte(math.Round(out * 255))
}

// inkAt returns the amount of ink of the channel data at the pixel i, from 0
// to 1. A channel of 1 bit has no spot ink.
func inkAt(data []byte, depth, i int) float64 {
	switch depth {
	case 8:
		if i < len(data) {
			return float64(255-data[i]) / 255
		}
	case 16:
		if 2*i+2 <= len(data) {
			return float64(0xffff-binary.BigEndian.Uint16(data[2*i:])) / 0xffff
		}
	case 32:
		if 4*i+4 <= len(data) {
			v := float64(math.Float32frombits(binary.BigEndian.Uint32(data[4*i:])))
			if math.IsNaN(v) {
				return 0
			}
			return 1 - min(max(v, 0), 1)
		}
	}
	return 0
}

// findSpots returns the spot channels of the merged image of p. The extra
// channels follow the channels of the color mode. If the file has a channel
// of transparency, it is the first extra channel, and the display information
// does not list it. The identifiers of the alpha channels name each extra
// channel, and the alternate colors of the spots use them.
func findSpots(p *oovpsd.PSD) []spot {
	cfg := p.Config
	base := cfg.ColorMode.Channels()
	if base < 1 || cfg.Channels <= base || cfg.Depth == 1 {
		return nil
	}
	extra := cfg.Channels - base
	ids := alphaIDs(cfg.Res)
	start := 0
	if hasTransparency(ids) {
		start = 1
	}
	infos := displayInfos(cfg.Res)
	if len(infos) != extra-start {
		return nil
	}
	alt := altSpotColors(cfg.Res)
	var spots []spot
	for i, info := range infos {
		if info.kind != kindSpot {
			continue
		}
		c, ok := info.color, info.ok
		if len(ids) == extra {
			if a, found := alt[ids[start+i]]; found {
				c, ok = a, true
			}
		}
		ch, found := p.Channel[base+start+i]
		if !ok || !found {
			continue
		}
		spots = append(spots, spot{
			data:     ch.Data,
			r:        c[0],
			g:        c[1],
			b:        c[2],
			solidity: min(float64(info.opacity), 100) / 100,
		})
	}
	return spots
}

// hasTransparency reports whether the first extra channel is the
// transparency, as the decoder of the merged image takes it: a file without
// identifiers, or with an identifier of 0.
func hasTransparency(ids []uint32) bool {
	if ids == nil {
		return true
	}
	for _, id := range ids {
		if id == 0 {
			return true
		}
	}
	return false
}

// alphaIDs returns the identifiers of the extra channels, or nil if the file
// has none.
func alphaIDs(res map[int]oovpsd.ImageResource) []uint32 {
	r, ok := res[resAlphaIDs]
	if !ok {
		return nil
	}
	ids := make([]uint32, 0, len(r.Data)/4)
	for i := 0; i+4 <= len(r.Data); i += 4 {
		ids = append(ids, binary.BigEndian.Uint32(r.Data[i:]))
	}
	return ids
}

// displayInfo is the display information of an extra channel.
type displayInfo struct {
	color   [3]float64
	ok      bool
	opacity uint16
	kind    byte
}

// displayInfos returns the display information of the extra channels, from
// the resource of Photoshop CS3 and later, or else from the older one.
func displayInfos(res map[int]oovpsd.ImageResource) []displayInfo {
	data, size := []byte(nil), 0
	if r, ok := res[resDisplayInfo]; ok && len(r.Data) >= 4 && binary.BigEndian.Uint32(r.Data) == 1 {
		data, size = r.Data[4:], 13
	} else if r, ok := res[resDisplayInfoOld]; ok {
		data, size = r.Data, 14
	}
	var infos []displayInfo
	for i := 0; i+13 <= len(data); i += size {
		c, ok := readColor(data[i : i+10])
		infos = append(infos, displayInfo{
			color:   c,
			ok:      ok,
			opacity: binary.BigEndian.Uint16(data[i+10:]),
			kind:    data[i+12],
		})
	}
	return infos
}

// altSpotColors returns the alternate colors of the spot channels, by the
// identifier of their channel. Photoshop writes them for the colors of a
// book, such as Pantone, whose color structure it cannot convert.
func altSpotColors(res map[int]oovpsd.ImageResource) map[uint32][3]float64 {
	r, ok := res[resAltSpotColors]
	if !ok || len(r.Data) < 4 {
		return nil
	}
	n := int(binary.BigEndian.Uint16(r.Data[2:]))
	colors := make(map[uint32][3]float64)
	for i, off := 0, 4; i < n && off+14 <= len(r.Data); i, off = i+1, off+14 {
		if c, ok := readColor(r.Data[off+4 : off+14]); ok {
			colors[binary.BigEndian.Uint32(r.Data[off:])] = c
		}
	}
	return colors
}

// readColor converts the color structure of Photoshop in b, of 10 bytes, to
// sRGB, from 0 to 1. It reports false for a color space that it does not
// convert.
func readColor(b []byte) ([3]float64, bool) {
	space := binary.BigEndian.Uint16(b)
	var v [4]float64
	for i := range v {
		v[i] = float64(binary.BigEndian.Uint16(b[2+2*i:])) / 0xffff
	}
	switch space {
	case spaceRGB:
		return [3]float64{v[0], v[1], v[2]}, true
	case spaceHSB:
		return hsbToRGB(v[0]*360, v[1], v[2]), true
	case spaceCMYK:
		// a value of 0 is full ink, as in the channels of a CMYK file
		return [3]float64{v[0] * v[3], v[1] * v[3], v[2] * v[3]}, true
	case spaceLab:
		l := float64(binary.BigEndian.Uint16(b[2:])) / 100
		a := float64(int16(binary.BigEndian.Uint16(b[4:]))) / 100
		bb := float64(int16(binary.BigEndian.Uint16(b[6:]))) / 100
		return labToSRGB(l, a, bb), true
	}
	return [3]float64{}, false
}

// hsbToRGB converts a hue in degrees, a saturation and a brightness, from 0 to
// 1, to RGB.
func hsbToRGB(h, s, v float64) [3]float64 {
	c := v * s
	hp := math.Mod(h, 360) / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g = c, x
	case hp < 2:
		r, g = x, c
	case hp < 3:
		g, b = c, x
	case hp < 4:
		g, b = x, c
	case hp < 5:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := v - c
	return [3]float64{r + m, g + m, b + m}
}

// labToSRGB converts a color of CIE Lab to sRGB, from 0 to 1. Photoshop
// gives Lab under the white of daylight at 5000 K. The Bradford transform
// adapts it to the white of sRGB, daylight at 6500 K, and a color outside
// sRGB is clipped.
func labToSRGB(l, a, b float64) [3]float64 {
	const delta = 6.0 / 29
	finv := func(t float64) float64 {
		if t > delta {
			return t * t * t
		}
		return 3 * delta * delta * (t - 4.0/29)
	}
	fy := (l + 16) / 116
	x := 0.96422 * finv(fy+a/500)
	y := finv(fy)
	z := 0.82521 * finv(fy-b/200)
	lin := [3]float64{
		3.1338561*x - 1.6168667*y - 0.4906146*z,
		-0.9787684*x + 1.9161415*y + 0.0334540*z,
		0.0719453*x - 0.2289914*y + 1.4052427*z,
	}
	var rgb [3]float64
	for i, c := range lin {
		c = min(max(c, 0), 1)
		if c <= 0.0031308 {
			rgb[i] = 12.92 * c
		} else {
			rgb[i] = 1.055*math.Pow(c, 1/2.4) - 0.055
		}
	}
	return rgb
}
