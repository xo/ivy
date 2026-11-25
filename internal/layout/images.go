// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/images.py: fetch and decode images of various
// formats, and lay out the gradients.
//
// Port: upstream decodes the raster images with Pillow, and keeps them as
// the data of a PDF. The port keeps the encoded data, reads the size from
// the header, and decodes the pixels when the drawing asks for them, with
// the decoders of the standard library and of golang.org/x/image, within
// Limits.MaxPixels. An SVG image is drawn with the package svg of okage.
// The methods draw draw onto the stream of pdf_stream.go.

package layout

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	_ "golang.org/x/image/bmp"  // the BMP decoder, for image.Decode
	_ "golang.org/x/image/tiff" // the TIFF decoder, for image.Decode
	_ "golang.org/x/image/webp" // the WebP decoder, for image.Decode

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/svg"
	"github.com/xo/okage/tinycss2"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
)

// errImageLoading is ImageLoadingError in weasyprint/images.py: an error
// that occurred when an image was loaded. The data of the image is probably
// corrupted or in a format that is not valid.
var errImageLoading = errors.New("image loading error")

// IntrinsicSize is the result of get_intrinsic_size in
// weasyprint/images.py: the intrinsic width, height and ratio of an image,
// in pixels. A field whose flag is false is the None of upstream.
type IntrinsicSize struct {
	Width, Height, Ratio          float64
	HasWidth, HasHeight, HasRatio bool
}

// Image is an image of weasyprint/images.py: a *RasterImage, an *SVGImage,
// a *LinearGradient or a *RadialGradient. The port adds a *MetafileImage and
// an *ObjectImage, in objects.go.
type Image interface {
	// GetIntrinsicSize is get_intrinsic_size: the intrinsic size of the
	// image, for a resolution in dots per pixel and a font size.
	GetIntrinsicSize(imageResolution, fontSize float64) IntrinsicSize
}

// Orientation is the orientation of a raster image: an angle of 0, 90, 180
// or 270 degrees, which turns the image counterclockwise as the transpose
// ROTATE_<angle> of Pillow does, and then a flip from left to right.
type Orientation struct {
	Angle int
	Flip  bool
}

// RasterImage is RasterImage in weasyprint/images.py: an image of pixels.
type RasterImage struct {
	// ID is the digest of the URL of the image, which the drawing uses as
	// the key of a cache.
	ID string
	// Data is the encoded image.
	Data []byte
	// Format is the format of the encoding: "png", "jpeg", "gif", "webp",
	// "bmp" or "tiff".
	Format string
	// Width and Height are the size of the image in pixels, after the
	// orientation.
	Width, Height int
	// Ratio is the width divided by the height, or +Inf for no height.
	Ratio float64
	// Orientation is the orientation that the drawing applies to the
	// decoded pixels.
	Orientation Orientation

	limits  ivy.Limits
	once    sync.Once
	decoded image.Image
	err     error
}

// newRasterImage is RasterImage.__init__ in weasyprint/images.py. It reads
// the header of the image and applies the orientation, which is "none",
// "from-image", or the tuple (angle, flip).
func newRasterImage(data []byte, imageID string, orientation Value, limits ivy.Limits) (*RasterImage, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reading image header: %w", err)
	}
	size := image.Pt(cfg.Width, cfg.Height)
	// Port: upstream has no limit.
	if err := raster.CheckPixels(size, limits); err != nil {
		return nil, err
	}
	img := &RasterImage{ID: imageID, Data: data, Format: format, limits: limits}
	img.Orientation = rotateImage(data, format, orientation)
	if img.Orientation.Angle == 90 || img.Orientation.Angle == 270 {
		size.X, size.Y = size.Y, size.X
	}
	img.Width, img.Height = size.X, size.Y
	if img.Height != 0 {
		img.Ratio = float64(img.Width) / float64(img.Height)
	} else {
		img.Ratio = math.Inf(1)
	}
	return img, nil
}

// GetIntrinsicSize satisfies Image.
func (r *RasterImage) GetIntrinsicSize(resolution, _ float64) IntrinsicSize {
	return IntrinsicSize{
		Width: float64(r.Width) / resolution, Height: float64(r.Height) / resolution, Ratio: r.Ratio,
		HasWidth: true, HasHeight: true, HasRatio: true,
	}
}

// Decode decodes the pixels of the image, with its orientation, once. It
// returns an error that wraps ivy.ErrLimit for an image larger than
// Limits.MaxPixels, and ivy.ErrFormat for data that cannot be decoded.
func (r *RasterImage) Decode(ctx context.Context) (image.Image, error) {
	r.once.Do(func() {
		img, err := raster.Decode(ctx, r.Data, r.limits)
		if err != nil {
			r.err = err
			return
		}
		r.decoded = orient(img, r.Orientation)
	})
	return r.decoded, r.err
}

// rotateImage is rotate_pillow_image in weasyprint/images.py. It returns the
// orientation of an image: the orientation of its EXIF data for
// "from-image", none for "none", and the tuple (angle, flip) itself.
//
// Port: the orientation of the EXIF data is read from a JPEG image only.
// Pillow also reads it from PNG, WebP and TIFF images.
func rotateImage(data []byte, format string, orientation Value) Orientation {
	if isStr(orientation, "from-image") {
		if format == "jpeg" {
			return exifOrientation(data)
		}
		return Orientation{}
	}
	if t, ok := orientation.(Tuple); ok && len(t) == 2 {
		angle := int(num(t[0]))
		switch angle {
		case 90, 180, 270:
		default:
			angle = 0
		}
		return Orientation{Angle: angle, Flip: truthy(t[1])}
	}
	return Orientation{}
}

// exifOrientations are the orientations of the tag Orientation of EXIF, from
// 1 to 8, as ImageOps.exif_transpose of Pillow transposes them.
var exifOrientations = [9]Orientation{
	2: {0, true},    // FLIP_LEFT_RIGHT
	3: {180, false}, // ROTATE_180
	4: {180, true},  // FLIP_TOP_BOTTOM
	5: {270, true},  // TRANSPOSE
	6: {270, false}, // ROTATE_270
	7: {90, true},   // TRANSVERSE
	8: {90, false},  // ROTATE_90
}

// exifOrientation returns the orientation of the EXIF data of a JPEG image.
func exifOrientation(data []byte) Orientation {
	// Walk the segments of the JPEG data to the APP1 segment of EXIF.
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return Orientation{}
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return Orientation{}
		}
		marker := data[i+1]
		if marker == 0xDA || marker == 0xD9 {
			// The start of the scan: no more metadata.
			return Orientation{}
		}
		length := int(binary.BigEndian.Uint16(data[i+2:]))
		if length < 2 || i+2+length > len(data) {
			return Orientation{}
		}
		segment := data[i+4 : i+2+length]
		if marker == 0xE1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			if o := tiffOrientation(segment[6:]); o >= 1 && o <= 8 {
				return exifOrientations[o]
			}
			return Orientation{}
		}
		i += 2 + length
	}
	return Orientation{}
}

// tiffOrientation returns the value of the tag Orientation (0x0112) of the
// first directory of TIFF data, or 0.
func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	offset := int(order.Uint32(tiff[4:]))
	if offset < 8 || offset+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[offset:]))
	for i := range count {
		entry := offset + 2 + 12*i
		if entry+12 > len(tiff) {
			return 0
		}
		if order.Uint16(tiff[entry:]) == 0x0112 {
			return int(order.Uint16(tiff[entry+8:]))
		}
	}
	return 0
}

// orient turns and flips an image as an orientation says.
func orient(img image.Image, o Orientation) image.Image {
	if o == (Orientation{}) {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	ow, oh := w, h
	if o.Angle == 90 || o.Angle == 270 {
		ow, oh = h, w
	}
	out := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for y := range h {
		for x := range w {
			// The position after the rotation, counterclockwise as Pillow
			// does.
			var nx, ny int
			switch o.Angle {
			case 90:
				nx, ny = y, w-1-x
			case 180:
				nx, ny = w-1-x, h-1-y
			case 270:
				nx, ny = h-1-y, x
			default:
				nx, ny = x, y
			}
			if o.Flip {
				nx = ow - 1 - nx
			}
			out.Set(nx, ny, color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)))
		}
	}
	return out
}

// SVGImage is SVGImage in weasyprint/images.py: an image of SVG data, which
// the drawing renders with the package svg of okage.
//
// Port: upstream keeps the tree of ElementTree. The port keeps the document
// of roxmltree, which okage parses with svg.FromXMLTree when it draws the
// image. So the XML of an image is parsed once, and an inline svg element
// is not written as XML text (okage D58).
type SVGImage struct {
	// BaseURL is the URL that the links of the image resolve against.
	BaseURL string

	// doc is the XML document of the image. err is the error of an inline
	// svg element whose document cannot be built, which the drawing logs.
	doc *roxmltree.Document
	err error
	// dims are the attributes width, height and viewBox of the root
	// element.
	dims    svg.Dimensions
	fetcher *URLFetcher
	limits  ivy.Limits
}

// newSVGImage is SVGImage.__init__ in weasyprint/images.py. It parses the
// data. It fails when the data is not well-formed XML.
func newSVGImage(data []byte, baseURL string, fetcher *URLFetcher, limits ivy.Limits) (*SVGImage, error) {
	doc, err := roxmltree.ParseWithOptions(string(data), svgParsingOptions(limits))
	if err != nil {
		return nil, fmt.Errorf("parsing SVG: %w", err)
	}
	return &SVGImage{
		BaseURL: baseURL,
		doc:     doc,
		dims:    svg.NodeDimensions(doc.RootElement()),
		fetcher: fetcher,
		limits:  limits,
	}, nil
}

// newInlineSVGImage is SVGImage.__init__ for the tree of an <svg> element of
// the document, which handle_svg of weasyprint/html.py makes.
func newInlineSVGImage(element *Element, baseURL string, fetcher *URLFetcher, limits ivy.Limits) *SVGImage {
	s := &SVGImage{BaseURL: baseURL, fetcher: fetcher, limits: limits}
	doc, err := buildSVG(element, svgParsingOptions(limits))
	if err != nil {
		s.err = fmt.Errorf("building SVG image %s: %w", baseURL, err)
		return s
	}
	s.doc, s.dims = doc, svg.NodeDimensions(doc.RootElement())
	return s
}

// svgParsingOptions are the options of roxmltree for an SVG image: the
// default options, with at most Limits.MaxObjects nodes.
func svgParsingOptions(limits ivy.Limits) roxmltree.ParsingOptions {
	options := roxmltree.DefaultParsingOptions()
	options.NodesLimit = uint32(min(int64(limits.WithDefaults().MaxObjects), 1<<31))
	return options
}

// GetIntrinsicSize satisfies Image. It is SVGImage.get_intrinsic_size in
// weasyprint/images.py, with SVG.get_intrinsic_size of
// weasyprint/svg/__init__.py.
func (s *SVGImage) GetIntrinsicSize(_, fontSize float64) IntrinsicSize {
	var size IntrinsicSize
	size.Width, size.HasWidth = s.intrinsicLength("width", s.dims.Width, s.dims.HasWidth, fontSize)
	size.Height, size.HasHeight = s.intrinsicLength("height", s.dims.Height, s.dims.HasHeight, fontSize)

	switch {
	case !size.HasWidth || !size.HasHeight:
		// Port: upstream reads the numbers of the viewBox with get_viewbox,
		// and takes the ratio of any numbers that are not zero. The port
		// reads the viewBox of svg.NodeDimensions, as okage draws it. A
		// viewBox that okage does not accept, such as one with a negative
		// width, gives no ratio.
		if s.dims.HasViewBox {
			viewbox := s.dims.ViewBox
			size.Ratio, size.HasRatio = float64(viewbox.Width())/float64(viewbox.Height()), true
			if size.HasWidth && size.Width != 0 {
				size.Height, size.HasHeight = size.Width/size.Ratio, true
			} else if size.HasHeight && size.Height != 0 {
				size.Width, size.HasWidth = size.Height*size.Ratio, true
			}
		}
	case size.Width != 0 && size.Height != 0:
		size.Ratio, size.HasRatio = size.Width/size.Height, true
	default:
		size.Ratio, size.HasRatio = 1, true
	}
	return size
}

// intrinsicLength is the intrinsic width or height of
// SVG.get_intrinsic_size in weasyprint/svg/__init__.py, from the attribute
// name of the root element and its length of svg.NodeDimensions. It returns
// false for a missing attribute or a percentage, which upstream reads as
// 100%.
func (s *SVGImage) intrinsicLength(name string, length svg.Length, valid bool, fontSize float64) (float64, bool) {
	if valid {
		if length.Unit == svg.LengthUnitPercent {
			return 0, false
		}
		return svgLength(length, fontSize), true
	}
	// Port: svg.NodeDimensions does not read a unit that okage does not
	// know, such as rem, or text that is not a length, such as auto. Upstream
	// reads them with size of weasyprint/svg/utils.py, which gives 0 for
	// text that is not a length. The port does the same with svgSize.
	if s.doc == nil {
		return 0, false
	}
	value, ok := s.doc.RootElement().Attribute(roxmltree.ExpandedNameFromStr(name))
	if !ok || strings.Contains(value, "%") {
		return 0, false
	}
	return svgSize(value, fontSize, 0), true
}

// svgLength is size in weasyprint/svg/utils.py for a length that is not a
// percentage. It resolves em and ex with the font size of the layout, and
// assumes that 1em is 2ex, as upstream does.
func svgLength(length svg.Length, fontSize float64) float64 {
	unit := ""
	switch length.Unit {
	case svg.LengthUnitEm:
		return fontSize * length.Number
	case svg.LengthUnitEx:
		return fontSize * length.Number / 2
	case svg.LengthUnitPt:
		unit = "pt"
	case svg.LengthUnitPc:
		unit = "pc"
	case svg.LengthUnitIn:
		unit = "in"
	case svg.LengthUnitCm:
		unit = "cm"
	case svg.LengthUnitMm:
		unit = "mm"
	default:
		// A number, or a number of pixels.
		return length.Number
	}
	return length.Number * lengthsToPixels[unit]
}

// Tree parses the SVG image with okage, for the drawing. The options set the
// fonts and the size, and the fetcher of the document reads the images that
// the image links to.
func (s *SVGImage) Tree(opts ...svg.Option) (*svg.Tree, error) {
	if s.err != nil {
		return nil, s.err
	}
	tree, err := svg.FromXMLTree(s.doc, opts...)
	if err != nil {
		return nil, fmt.Errorf("parsing SVG image %s: %w", s.BaseURL, err)
	}
	return tree, nil
}

// buildSVG makes the XML document of an element of the document tree and
// its descendants with roxmltree.Builder, for the parser of SVG of okage. A
// name in the Clark notation, {namespace}local, keeps its namespace. A tag
// without a namespace is an element of HTML, such as in foreignObject, and
// takes the namespace of XHTML. An element deeper than DefaultMaxTreeDepth
// is left out, with its descendants.
func buildSVG(element *Element, options roxmltree.ParsingOptions) (*roxmltree.Document, error) {
	b := roxmltree.NewBuilder(options)
	if err := startSVGElement(b, element); err != nil {
		return nil, err
	}
	// The walk uses a stack, so a deep tree does not grow the Go stack.
	// next is the index of the next child of element.
	type frame struct {
		element *Element
		next    int
	}
	stack := []frame{{element: element}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.next == len(top.element.Children) {
			if err := b.EndElement(); err != nil {
				return nil, err
			}
			tail := top.element.Tail
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				// The tail of the root element is not part of the image.
				break
			}
			if err := b.Text(tail); err != nil {
				return nil, err
			}
			continue
		}
		child := top.element.Children[top.next]
		top.next++
		if len(stack) > DefaultMaxTreeDepth {
			if err := b.Text(child.Tail); err != nil {
				return nil, err
			}
			continue
		}
		if err := startSVGElement(b, child); err != nil {
			return nil, err
		}
		stack = append(stack, frame{element: child})
	}
	return b.Document()
}

// startSVGElement starts an element of buildSVG, with its attributes and its
// text. The parser of HTML keeps the declarations of the namespaces as
// attributes. They are left out, because each name holds its namespace.
func startSVGElement(b *roxmltree.Builder, element *Element) error {
	if err := b.StartElement(expandedName(element.Tag, nsHTML)); err != nil {
		return err
	}
	for _, attr := range element.Attrib {
		if attr.Name == "xmlns" || strings.HasPrefix(attr.Name, "xmlns:") ||
			strings.HasPrefix(attr.Name, "{http://www.w3.org/2000/xmlns/}") {
			continue
		}
		if err := b.Attribute(expandedName(attr.Name, ""), attr.Value); err != nil {
			return err
		}
	}
	return b.Text(element.Text)
}

// expandedName returns the expanded name of a name in the Clark notation. A
// name without a namespace takes the namespace uri, or no namespace when uri
// is empty.
func expandedName(name, uri string) roxmltree.ExpandedName {
	if rest, ok := strings.CutPrefix(name, "{"); ok {
		uri, name, _ = strings.Cut(rest, "}")
	}
	if uri == "" {
		return roxmltree.ExpandedNameFromStr(name)
	}
	return roxmltree.ExpandedNameFromTuple(uri, name)
}

// imageLoader holds the arguments of get_image_from_uri in
// weasyprint/images.py that the building of the boxes binds: the cache, the
// fetcher and the limits.
type imageLoader struct {
	cache   map[string]Image
	fetcher *URLFetcher
	limits  ivy.Limits
	log     *logger
	// images counts the images that the loader decoded the header of.
	images int
}

// newImageLoader returns a loader of images with an empty cache.
func newImageLoader(fetcher *URLFetcher, limits ivy.Limits, log *logger) *imageLoader {
	if fetcher == nil {
		fetcher = &URLFetcher{}
	}
	return &imageLoader{cache: map[string]Image{}, fetcher: fetcher, limits: limits.WithDefaults(), log: log}
}

// get is get_image_from_uri in weasyprint/images.py. It returns the image
// of a URL, or nil when it cannot be loaded. forcedMimeType is "" for None.
// orientation is the computed value of image-orientation.
func (il *imageLoader) get(url, forcedMimeType string, orientation Value) Image {
	if img, ok := il.cache[url]; ok {
		return img
	}

	img, err := il.load(url, forcedMimeType, orientation)
	if err != nil {
		il.log.error("Failed to load image at %q: %v", url, err)
		il.log.debug("Error while loading image: %v", err)
		img = nil
	}

	il.cache[url] = img
	return img
}

// load fetches and reads an image, or returns an error that wraps
// ErrURLFetching or errImageLoading.
func (il *imageLoader) load(url, forcedMimeType string, orientation Value) (img Image, err error) {
	defer func() {
		// Port: a panic in a decoder is an image that cannot be loaded.
		if r := recover(); r != nil {
			img, err = nil, fmt.Errorf("%w: %v", errImageLoading, r)
		}
	}()
	il.images++
	if il.images > il.limits.MaxObjects {
		// Port: upstream has no limit.
		return nil, fmt.Errorf("%w: %w: too many images", errImageLoading, ivy.ErrLimit)
	}
	// Port: upstream has no objects.
	if name, ok := strings.CutPrefix(url, ObjectScheme+":"); ok {
		return il.object(name)
	}
	response, err := il.fetcher.Fetch(url)
	if err != nil {
		return nil, err
	}
	bytestring := response.Body
	mimeType := forcedMimeType
	if mimeType == "" {
		mimeType = response.ContentType
	}

	var svgErr error
	// Try to rely on given mimetype for SVG
	if mimeType == "image/svg+xml" {
		var svgImage *SVGImage
		svgImage, svgErr = newSVGImage(bytestring, url, il.fetcher, il.limits)
		if svgErr == nil {
			return svgImage, nil
		}
	}
	// Try the decoders of raster images, or for failing SVG
	_, _, rasterErr := image.DecodeConfig(bytes.NewReader(bytestring))
	if rasterErr != nil {
		// Port: upstream reads no metafile. A metafile of Windows comes
		// before the last chance of SVG.
		if img, ok := il.metafile(url, bytestring); ok {
			return img, nil
		}
		if mimeType == "image/svg+xml" {
			// Tried SVGImage then the raster decoders for a SVG, abort
			return nil, fmt.Errorf("%w: %w", errImageLoading, svgErr)
		}
		// Last chance, try SVG
		img, err := newSVGImage(bytestring, url, il.fetcher, il.limits)
		if err != nil {
			// Tried the raster decoders then SVGImage for a raster, abort
			return nil, fmt.Errorf("%w: %w", errImageLoading, rasterErr)
		}
		return img, nil
	}
	// Store image id to enable cache in Stream.add_image
	imageID := fmt.Sprintf("%x", md5.Sum([]byte(url)))
	raster, err := newRasterImage(bytestring, imageID, orientation, il.limits)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errImageLoading, err)
	}
	return raster, nil
}

// processColorStops is process_color_stops in weasyprint/images.py. It
// gives the positions and the hints of the color stops on the vector of a
// gradient. vectorLength is the distance between the starting point and
// the ending point of the vector. positions are nil or a Dimension in px or
// %, where 0 is the starting point and 1 the ending point. See
// https://drafts.csswg.org/css-images-3/#color-stop-syntax.
//
// It returns the positions of the stops, in px, and the exponents of the
// hints of PDF.
func processColorStops(vectorLength float64, positions, hints []Value, style *Style) ([]float64, []float64) {
	// Resolve percentages.
	n := 2*len(positions) - 1
	stops := make([]float64, n)
	known := make([]bool, n)
	for i := range n {
		var value Value
		if i%2 == 0 {
			value = positions[i/2]
		} else if i/2 < len(hints) {
			value = hints[i/2]
		}
		stops[i], known[i] = percentageFloat(value, style, vectorLength)
	}

	// First defaults to 0% and last defaults to 100%.
	if !known[0] {
		stops[0], known[0] = 0, true
	}
	if !known[n-1] {
		stops[n-1], known[n-1] = vectorLength, true
	}

	// Make sure stops are increasing.
	previousStop := stops[0]
	for i := range stops {
		if known[i] {
			if stops[i] < previousStop {
				stops[i] = previousStop
			} else {
				previousStop = stops[i]
			}
		}
	}

	// Assign missing positions.
	previousI := -1
	for i := range stops {
		if known[i] {
			// stops[-1] of Python for the first stop.
			base := stops[n-1]
			if previousI >= 0 {
				base = stops[previousI]
			}
			increment := (stops[i] - base) / float64(i-previousI)
			for j := 1; j < i-previousI; j++ {
				stops[previousI+j] = base + float64(j)*increment
				known[previousI+j] = true
			}
			previousI = i
		}
	}

	// Calculate exponential value for PDF hints.
	pdfHints := make([]float64, 0, len(positions)-1)
	for i := 1; i < n; i += 2 {
		hint, before, after := stops[i], stops[i-1], stops[i+1]
		switch {
		case hint == before:
			pdfHints = append(pdfHints, 0)
		case hint == after:
			pdfHints = append(pdfHints, 1<<16)
		case before == after:
			pdfHints = append(pdfHints, 1)
		default:
			ratio := (hint - before) / (after - before)
			if ratio <= 0 || ratio == 1 {
				// Port: upstream raises ValueError or ZeroDivisionError.
				pdfHints = append(pdfHints, 1)
			} else {
				pdfHints = append(pdfHints, math.Log(0.5)/math.Log(ratio))
			}
		}
	}

	evenStops := make([]float64, 0, len(positions))
	for i := 0; i < n; i += 2 {
		evenStops = append(evenStops, stops[i])
	}
	return evenStops, pdfHints
}

// normalizeStopPositions is normalize_stop_positions in
// weasyprint/images.py. It normalizes the positions of the stops between 0
// and 1, and returns the original first and last positions.
func normalizeStopPositions(positions []float64) (first, last float64, normalized []float64) {
	first, last = positions[0], positions[len(positions)-1]
	totalLength := last - first
	normalized = make([]float64, len(positions))
	if totalLength != 0 {
		for i, pos := range positions {
			normalized[i] = (pos - first) / totalLength
		}
	}
	return first, last, normalized
}

// srgba returns the sRGB coordinates and the alpha of a color.
func srgba(c *tinycss2.Color) (r, g, b, a float64) {
	if c == nil {
		return 0, 0, 0, 0
	}
	s, ok := c.To("srgb")
	if !ok || len(s.Coordinates) < 3 {
		return 0, 0, 0, c.Alpha
	}
	return s.Coordinates[0], s.Coordinates[1], s.Coordinates[2], c.Alpha
}

// newSRGB returns a color of sRGB.
func newSRGB(r, g, b, a float64) *tinycss2.Color {
	return &tinycss2.Color{Space: "srgb", Coordinates: []float64{r, g, b}, Alpha: min(max(a, 0), 1)}
}

// gradientAverageColor is gradient_average_color in weasyprint/images.py.
// See https://drafts.csswg.org/css-images-3/#gradient-average-color.
//
// Port: upstream reads the coordinates of the colors in their own spaces.
// The port converts them to sRGB.
func gradientAverageColor(colors []*tinycss2.Color, positions []float64) *tinycss2.Color {
	// TODO: handle color spaces.
	nbStops := len(positions)
	if nbStops < 2 || nbStops != len(colors) {
		// Port: upstream asserts.
		return newSRGB(0, 0, 0, 0)
	}
	totalLength := positions[nbStops-1] - positions[0]
	if totalLength == 0 {
		positions = make([]float64, nbStops)
		for i := range positions {
			positions[i] = float64(i)
		}
		totalLength = float64(nbStops - 1)
	}
	premulR := make([]float64, nbStops)
	premulG := make([]float64, nbStops)
	premulB := make([]float64, nbStops)
	alpha := make([]float64, nbStops)
	for i, c := range colors {
		r, g, b, a := srgba(c)
		premulR[i], premulG[i], premulB[i], alpha[i] = r*a, g*a, b*a, a
	}
	var resultR, resultG, resultB, resultA float64
	totalWeight := 2 * totalLength
	for i := 1; i < nbStops; i++ {
		weight := (positions[i] - positions[i-1]) / totalWeight
		for _, j := range [2]int{i - 1, i} {
			resultR += premulR[j] * weight
			resultG += premulG[j] * weight
			resultB += premulB[j] * weight
			resultA += alpha[j] * weight
		}
	}
	// Un-premultiply.
	if resultA == 0 {
		return newSRGB(0, 0, 0, 0)
	}
	return newSRGB(resultR/resultA, resultG/resultA, resultB/resultA, resultA)
}

// GradientLayout is the result of the method layout of a gradient in
// weasyprint/images.py.
type GradientLayout struct {
	// ScaleY is the vertical scale of the gradient: a float for the
	// ellipses of the radial gradients, and 1 otherwise.
	ScaleY float64
	// Type is the type of the gradient: "solid", "linear" or "radial".
	Type string
	// Points are the coordinates of the useful points: nil for "solid",
	// (x0, y0, x1, y1), the starting and the ending points, for "linear",
	// and (cx0, cy0, radius0, cx1, cy1, radius1), the starting and the
	// ending circles, for "radial".
	Points []float64
	// Positions are the positions of the color stops, between 0 and 1: 0
	// at the starting point and 1 at the ending point.
	Positions []float64
	// Colors are the colors of the stops.
	Colors []*tinycss2.Color
	// Hints are the exponents of the hints between the stops.
	Hints []float64
}

// colorsOf returns the colors of the stops of a gradient.
func (g *Gradient) colorsOf() []*tinycss2.Color {
	colors := make([]*tinycss2.Color, len(g.Colors))
	for i, c := range g.Colors {
		if c, ok := c.(Color); ok {
			colors[i] = c.Color
		}
	}
	return colors
}

// GetIntrinsicSize satisfies Image: a gradient has no intrinsic size.
func (*LinearGradient) GetIntrinsicSize(_, _ float64) IntrinsicSize { return IntrinsicSize{} }

// GetIntrinsicSize satisfies Image: a gradient has no intrinsic size.
func (*RadialGradient) GetIntrinsicSize(_, _ float64) IntrinsicSize { return IntrinsicSize{} }

// solidLayout is the layout of a gradient of one color.
func solidLayout(c *tinycss2.Color) GradientLayout {
	return GradientLayout{ScaleY: 1, Type: "solid", Colors: []*tinycss2.Color{c}}
}

// Layout is LinearGradient.layout in weasyprint/images.py. It returns the
// layout of the gradient in a box of width and height, whose top left
// corner is at (0, 0), for a box of style.
func (lg *LinearGradient) Layout(width, height float64, style *Style) GradientLayout {
	colors := lg.colorsOf()
	// Only one color, render the gradient as a solid color
	if len(colors) <= 1 {
		var c *tinycss2.Color
		if len(colors) == 1 {
			c = colors[0]
		}
		return solidLayout(c)
	}

	// Define the (dx, dy) unit vector giving the direction of the gradient.
	// Positive dx: right, positive dy: down.
	var dx, dy float64
	if lg.DirectionType == "corner" {
		y, x, _ := strings.Cut(str(lg.Direction), "_")
		factorX, factorY := 1.0, 1.0
		if x == "left" {
			factorX = -1
		}
		if y == "top" {
			factorY = -1
		}
		diagonal := math.Hypot(width, height)
		if diagonal == 0 {
			// Port: upstream raises ZeroDivisionError.
			diagonal = 1
		}
		// Note the direction swap: dx based on height, dy based on width
		// The gradient line is perpendicular to a diagonal.
		dx = factorX * height / diagonal
		dy = factorY * width / diagonal
	} else {
		angle := num(lg.Direction) // 0 upwards, then clockwise
		dx = math.Sin(angle)
		dy = -math.Cos(angle)
	}

	// Round dx and dy to avoid floating points errors caused by
	// trigonometry and angle units conversions
	dx, dy = roundTo(dx, 9), roundTo(dy, 9)

	// Normalize colors positions
	vectorLength := math.Abs(width*dx) + math.Abs(height*dy)
	positions, hints := processColorStops(vectorLength, lg.StopPositions, lg.ColorHints, style)
	if !lg.Repeating {
		// Add explicit colors at boundaries if needed, because PDF doesn’t
		// extend color stops that are not displayed
		if positions[0] == positions[1] {
			positions = slices.Insert(positions, 0, positions[0]-1)
			colors = slices.Insert(colors, 0, colors[0])
			hints = slices.Insert(hints, 0, 1)
		}
		if positions[len(positions)-2] == positions[len(positions)-1] {
			positions = append(positions, positions[len(positions)-1]+1)
			colors = append(colors, colors[len(colors)-1])
			hints = append(hints, 1)
		}
	}
	first, last, positions := normalizeStopPositions(positions)

	if lg.Repeating {
		// Render as a solid color if the first and last positions are equal
		// See https://drafts.csswg.org/css-images-3/#repeating-gradients
		if first == last {
			return solidLayout(gradientAverageColor(colors, positions))
		}

		// Define defined gradient length and steps between positions
		stopLength := last - first
		positionSteps := make([]float64, len(positions)-1)
		for i := range positionSteps {
			positionSteps[i] = positions[i+1] - positions[i]
		}

		// Create cycles used to add colors
		nextSteps := append([]float64{0}, positionSteps...)
		previousSteps := append([]float64{0}, reversed(positionSteps)...)
		nextColors, nextHints := slices.Clone(colors), slices.Clone(hints)
		previousColors, previousHints := reversed(colors), reversed(hints)

		// Add colors after last step
		// Port: upstream has no bound. A gradient whose stops are very
		// close repeats at most maxGradientStops times.
		for i := 0; last < vectorLength && len(positions) < maxGradientStops; i++ {
			step := nextSteps[i%len(nextSteps)]
			colors = append(colors, nextColors[i%len(nextColors)])
			if len(nextHints) > 0 {
				hints = append(hints, nextHints[i%len(nextHints)])
			}
			positions = append(positions, positions[len(positions)-1]+step)
			last += step * stopLength
		}

		// Add colors before first step
		for i := 0; first > 0 && len(positions) < maxGradientStops; i++ {
			step := previousSteps[i%len(previousSteps)]
			colors = slices.Insert(colors, 0, previousColors[i%len(previousColors)])
			if len(previousHints) > 0 {
				hints = slices.Insert(hints, 0, previousHints[i%len(previousHints)])
			}
			positions = slices.Insert(positions, 0, positions[0]-step)
			first -= step * stopLength
		}
	}

	// Define the coordinates of the starting and ending points
	startX := (width - dx*vectorLength) / 2
	startY := (height - dy*vectorLength) / 2
	points := []float64{
		startX + dx*first, startY + dy*first,
		startX + dx*last, startY + dy*last,
	}

	return GradientLayout{ScaleY: 1, Type: "linear", Points: points, Positions: positions, Colors: colors, Hints: hints}
}

// maxGradientStops bounds the stops of a repeating gradient.
const maxGradientStops = 1 << 16

// reversed returns a reversed copy of a slice.
func reversed[T any](s []T) []T {
	r := slices.Clone(s)
	slices.Reverse(r)
	return r
}

// roundTo is round(x, digits) of Python.
func roundTo(x float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.RoundToEven(x*p) / p
}

// Layout is RadialGradient.layout in weasyprint/images.py. It returns the
// layout of the gradient in a box of width and height, whose top left
// corner is at (0, 0), for a box of style.
func (rg *RadialGradient) Layout(width, height float64, style *Style) GradientLayout {
	colors := rg.colorsOf()
	// Only one color, render the gradient as a solid color
	if len(colors) <= 1 {
		var c *tinycss2.Color
		if len(colors) == 1 {
			c = colors[0]
		}
		return solidLayout(c)
	}

	// Define the center of the gradient
	var originX, originY string
	var centerX, centerY float64
	if center, ok := rg.Center.(Tuple); ok && len(center) == 4 {
		originX, originY = str(center[0]), str(center[2])
		centerX, _ = percentageFloat(center[1], style, width)
		centerY, _ = percentageFloat(center[3], style, height)
	}
	if originX == "right" {
		centerX = width - centerX
	}
	if originY == "bottom" {
		centerY = height - centerY
	}

	// Resolve sizes and vertical scale
	sizeX, sizeY := rg.handleDegenerate(rg.resolveSize(width, height, centerX, centerY, style))
	scaleY := sizeY / sizeX

	// Normalize colors positions
	positions, hints := processColorStops(sizeX, rg.StopPositions, rg.ColorHints, style)
	if !rg.Repeating {
		// Add explicit colors at boundaries if needed, because PDF doesn’t
		// extend color stops that are not displayed
		if positions[0] > 0 && positions[0] == positions[1] {
			positions = slices.Insert(positions, 0, 0)
			colors = slices.Insert(colors, 0, colors[0])
			hints = slices.Insert(hints, 0, 1)
		}
		if positions[len(positions)-2] == positions[len(positions)-1] {
			positions = append(positions, positions[len(positions)-1]+1)
			colors = append(colors, colors[len(colors)-1])
			hints = append(hints, 1)
		}
	}
	if positions[0] < 0 {
		// PDF doesn’t like negative radiuses, shift into the positive realm
		if rg.Repeating {
			// Add vector lengths to first position until positive
			vectorLength := positions[len(positions)-1] - positions[0]
			offset := vectorLength * (1 + math.Floor(-positions[0]/vectorLength))
			for i := range positions {
				positions[i] += offset
			}
		} else {
			// Only keep colors with position >= 0, interpolate if needed
			if positions[len(positions)-1] <= 0 {
				// All stops are negative, fill with the last color
				return solidLayout(colors[len(colors)-1])
			}
			for i, position := range positions {
				if position == 0 {
					// Keep colors and positions from this rank
					colors, positions = colors[i:], positions[i:]
					break
				}
				if position > 0 {
					// Interpolate with previous rank to get color at 0
					color := colors[i]
					previousColor := colors[i-1]
					previousPosition := positions[i-1]
					intermediateColor := gradientAverageColor(
						[]*tinycss2.Color{previousColor, previousColor, color, color},
						[]float64{previousPosition, 0, 0, position})
					colors = append([]*tinycss2.Color{intermediateColor}, colors[i:]...)
					positions = append([]float64{0}, positions[i:]...)
					break
				}
			}
		}
	}
	first, last, positions := normalizeStopPositions(positions)

	// Render as a solid color if the first and last positions are the same
	// See https://drafts.csswg.org/css-images-3/#repeating-gradients
	if first == last && rg.Repeating {
		return solidLayout(gradientAverageColor(colors, positions))
	}

	// Define the coordinates of the gradient circles
	points := []float64{centerX, centerY / scaleY, first, centerX, centerY / scaleY, last}

	if rg.Repeating {
		points, positions, colors, hints = rg.repeat(width, height, scaleY, points, positions, colors, hints)
	}

	return GradientLayout{ScaleY: scaleY, Type: "radial", Points: points, Positions: positions, Colors: colors, Hints: hints}
}

// repeat is RadialGradient._repeat in weasyprint/images.py.
func (rg *RadialGradient) repeat(width, height, scaleY float64, points, positions []float64, colors []*tinycss2.Color, hints []float64) ([]float64, []float64, []*tinycss2.Color, []float64) {
	// Keep original lists and values, they’re useful
	originalColors := slices.Clone(colors)
	originalHints := slices.Clone(hints)
	originalPositions := slices.Clone(positions)
	gradientLength := points[5] - points[2]

	// Get the maximum distance between the center and the corners, to find
	// how many times we have to repeat the colors outside
	maxDistance := max(
		math.Hypot(width-points[0], height/scaleY-points[1]),
		math.Hypot(width-points[0], -points[1]*scaleY),
		math.Hypot(-points[0], height/scaleY-points[1]),
		math.Hypot(-points[0], -points[1]*scaleY))
	repeatAfter := math.Ceil((maxDistance - points[5]) / gradientLength)
	// Port: upstream has no bound.
	repeatAfter = min(repeatAfter, float64(maxGradientStops/max(len(positions), 1)))
	if repeatAfter > 0 {
		// Repeat colors and extrapolate positions
		repeat := 1 + int(repeatAfter)
		newColors := make([]*tinycss2.Color, 0, len(colors)*repeat)
		newHints := make([]float64, 0, (len(hints)+1)*repeat)
		newPositions := make([]float64, 0, len(positions)*repeat)
		for i := range repeat {
			newColors = append(newColors, colors...)
			newHints = append(newHints, hints...)
			newHints = append(newHints, 1)
			for _, position := range positions {
				newPositions = append(newPositions, float64(i)+position)
			}
		}
		colors, hints, positions = newColors, newHints[:len(newHints)-1], newPositions
		points = []float64{points[0], points[1], points[2], points[3], points[4], points[5] + gradientLength*repeatAfter}
	}

	if points[2] == 0 {
		// Inner circle has 0 radius, no need to repeat inside, return
		return points, positions, colors, hints
	}

	// Find how many times we have to repeat the colors inside
	repeatBefore := points[2] / gradientLength

	// Set the inner circle size to 0
	points = []float64{points[0], points[1], 0, points[3], points[4], points[5]}

	// Find how many times the whole gradient can be repeated
	fullRepeat := int(min(repeatBefore, float64(maxGradientStops/max(len(originalPositions), 1))))
	if fullRepeat > 0 {
		// Repeat colors and extrapolate positions
		for range fullRepeat {
			colors = append(colors, originalColors...)
			hints = append(hints, 1)
			hints = append(hints, originalHints...)
		}
		var newPositions []float64
		for i := range fullRepeat {
			for _, position := range originalPositions {
				newPositions = append(newPositions, float64(i-fullRepeat)+position)
			}
		}
		positions = append(newPositions, positions...)
	}

	// Find the ratio of gradient that must be added to reach the center
	partialRepeat := repeatBefore - float64(fullRepeat)
	if partialRepeat <= 0 || partialRepeat >= 1 {
		// No partial repeat, return
		return points, positions, colors, hints
	}

	// Iterate through positions in reverse order, from the outer circle to
	// the original inner circle, to find positions from the inner circle
	// (including full repeats) to the center
	n := len(originalPositions)
	ratio := 1 - partialRepeat
	for i := 1; i <= n; i++ {
		position := originalPositions[n-i]
		if position == ratio {
			// The center is a color of the gradient, truncate original
			// colors and positions and prepend them
			colors = append(slices.Clone(originalColors[len(originalColors)-min(i, len(originalColors)):]), colors...)
			newHints := slices.Clone(originalHints[len(originalHints)-min(i, len(originalHints)):])
			hints = append(append(newHints, 1), hints...)
			var newPositions []float64
			for _, p := range originalPositions[n-i:] {
				newPositions = append(newPositions, p-float64(fullRepeat)-1)
			}
			positions = append(newPositions, positions...)
			return points, positions, colors, hints
		}
		if position < ratio && i > 1 {
			// The center is between two colors of the gradient, define the
			// center color as the average of these two gradient colors
			color := originalColors[n-i]
			nextColor := originalColors[n-i+1]
			nextPosition := originalPositions[n-i+1]
			averageColors := []*tinycss2.Color{color, color, nextColor, nextColor}
			averagePositions := []float64{position, ratio, ratio, nextPosition}
			zeroColor := gradientAverageColor(averageColors, averagePositions)
			newColors := append([]*tinycss2.Color{zeroColor}, originalColors[n-i+1:]...)
			colors = append(newColors, colors...)
			newHints := append([]float64{1}, originalHints[len(originalHints)-min(i-1, len(originalHints)):]...)
			hints = append(append(newHints, 1), hints...)
			newPositions := []float64{ratio - 1 - float64(fullRepeat)}
			for _, p := range originalPositions[n-i+1:] {
				newPositions = append(newPositions, p-1-float64(fullRepeat))
			}
			positions = append(newPositions, positions...)
			return points, positions, colors, hints
		}
	}
	return points, positions, colors, hints
}

// resolveSize is RadialGradient._resolve_size in weasyprint/images.py: the
// size of the circle of the gradient.
func (rg *RadialGradient) resolveSize(width, height, centerX, centerY float64, style *Style) (float64, float64) {
	if rg.SizeType == "explicit" {
		size, _ := rg.Size.(Tuple)
		if len(size) != 2 {
			return 0, 0
		}
		sizeX, _ := percentageFloat(size[0], style, width)
		sizeY, _ := percentageFloat(size[1], style, height)
		return sizeX, sizeY
	}
	left := math.Abs(centerX)
	right := math.Abs(width - centerX)
	top := math.Abs(centerY)
	bottom := math.Abs(height - centerY)
	keyword := str(rg.Size)
	pick := func(values ...float64) float64 {
		if strings.HasPrefix(keyword, "closest") {
			return slices.Min(values)
		}
		return slices.Max(values)
	}
	if strings.HasSuffix(keyword, "side") {
		if rg.Shape == "circle" {
			sizeXY := pick(left, right, top, bottom)
			return sizeXY, sizeXY
		}
		// else: ellipse
		return pick(left, right), pick(top, bottom)
	}
	// else: corner
	if rg.Shape == "circle" {
		sizeXY := pick(math.Hypot(left, top), math.Hypot(left, bottom),
			math.Hypot(right, top), math.Hypot(right, bottom))
		return sizeXY, sizeXY
	}
	// else: ellipse
	corners := [][2]float64{{left, top}, {left, bottom}, {right, top}, {right, bottom}}
	best := corners[0]
	for _, corner := range corners[1:] {
		d, bestD := math.Hypot(corner[0], corner[1]), math.Hypot(best[0], best[1])
		if strings.HasPrefix(keyword, "closest") && d < bestD || !strings.HasPrefix(keyword, "closest") && d > bestD {
			best = corner
		}
	}
	return best[0] * math.Sqrt2, best[1] * math.Sqrt2
}

// handleDegenerate is RadialGradient._handle_degenerate in
// weasyprint/images.py. See
// https://drafts.csswg.org/css-images-3/#degenerate-radials.
func (rg *RadialGradient) handleDegenerate(sizeX, sizeY float64) (float64, float64) {
	switch {
	case sizeX == 0 && sizeY == 0:
		sizeX, sizeY = 1e-7, 1e-7
	case sizeX == 0:
		sizeX, sizeY = 1e-7, 1e7
	case sizeY == 0:
		sizeX, sizeY = 1e7, 1e-7
	}
	return sizeX, sizeY
}

// Each type of image satisfies Image.
var (
	_ Image = (*RasterImage)(nil)
	_ Image = (*SVGImage)(nil)
	_ Image = (*LinearGradient)(nil)
	_ Image = (*RadialGradient)(nil)
)

// Gradient is Gradient in weasyprint/images.py: the data that the linear and
// the radial gradients share.
type Gradient struct {
	// Colors are the colors of the stops, as Color values.
	Colors []Value
	// StopPositions are the positions of the stops, as Dimension values, or
	// nil for a stop without a position.
	StopPositions []Value
	// ColorHints are the hints between the stops, as Dimension values, or
	// nil for no hint.
	ColorHints []Value
	// Repeating is true for repeating-linear-gradient() and
	// repeating-radial-gradient().
	Repeating bool
}

// newGradient is Gradient.__init__ in weasyprint/images.py. Each color stop
// is a Color and a position, or nil for no position.
func newGradient(colorStops [][2]Value, repeating bool, colorHints []Value) Gradient {
	g := Gradient{
		Colors:        make([]Value, len(colorStops)),
		StopPositions: make([]Value, len(colorStops)),
		ColorHints:    colorHints,
		Repeating:     repeating,
	}
	for i, stop := range colorStops {
		g.Colors[i], g.StopPositions[i] = stop[0], stop[1]
	}
	return g
}

// LinearGradient is LinearGradient in weasyprint/images.py.
type LinearGradient struct {
	Gradient
	// DirectionType is "corner" or "angle".
	DirectionType string
	// Direction is a corner, such as "top_left", for "corner", or an angle in
	// radians, 0 upwards and then clockwise, for "angle".
	Direction Value
}

// newLinearGradient is LinearGradient.__init__ in weasyprint/images.py.
// direction is the tuple ('corner', keyword) or ('angle', radians).
func newLinearGradient(colorStops [][2]Value, direction Tuple, repeating bool, colorHints []Value) *LinearGradient {
	return &LinearGradient{
		Gradient:      newGradient(colorStops, repeating, colorHints),
		DirectionType: str(direction[0]),
		Direction:     direction[1],
	}
}

// RadialGradient is RadialGradient in weasyprint/images.py.
type RadialGradient struct {
	Gradient
	// Center is the center of the ending shape: the tuple (origin_x, pos_x,
	// origin_y, pos_y).
	Center Value
	// Shape is the type of the ending shape: "circle" or "ellipse".
	Shape string
	// SizeType is "keyword" or "explicit".
	SizeType string
	// Size is "closest-corner", "farthest-corner", "closest-side" or
	// "farthest-side" for "keyword", and the tuple (radius_x, radius_y) for
	// "explicit".
	Size Value
}

// newRadialGradient is RadialGradient.__init__ in weasyprint/images.py.
// size is the tuple (size_type, size).
func newRadialGradient(colorStops [][2]Value, shape string, size Tuple, center Value, repeating bool, colorHints []Value) *RadialGradient {
	return &RadialGradient{
		Gradient: newGradient(colorStops, repeating, colorHints),
		Center:   center,
		Shape:    shape,
		SizeType: str(size[0]),
		Size:     size[1],
	}
}

func (*LinearGradient) isValue() {}
func (*RadialGradient) isValue() {}

// paintImage calls the method draw of an image of upstream: it draws img in
// the rectangle (0, 0, concreteWidth, concreteHeight) of the current user
// space. style can be nil.
func paintImage(s *stream, img Image, concreteWidth, concreteHeight float64, style *Style) {
	switch img := img.(type) {
	case *RasterImage:
		img.draw(s, concreteWidth, concreteHeight, style)
	case *SVGImage:
		img.draw(s, concreteWidth, concreteHeight, style)
	case *LinearGradient:
		img.draw(s, concreteWidth, concreteHeight, style)
	case *RadialGradient:
		img.draw(s, concreteWidth, concreteHeight, style)
	case *MetafileImage:
		img.draw(s, concreteWidth, concreteHeight)
	case *ObjectImage:
		img.draw(s, concreteWidth, concreteHeight)
	}
}

// draw is RasterImage.draw in weasyprint/images.py.
//
// Port: upstream writes the image in the PDF file, and lowers its
// resolution for the option dpi. The port decodes the image, and draws its
// pixels. An image that cannot be decoded is left out, and the page is drawn
// in part.
func (r *RasterImage) draw(s *stream, concreteWidth, concreteHeight float64, style *Style) {
	if r.Width <= 0 || r.Height <= 0 {
		return
	}

	interpolate := style == nil || isStr(style.Get("image_rendering"), "auto")
	img, err := r.Decode(s.dc.ctx)
	if err != nil {
		s.dc.log.warning("Failed to draw image %s: %v", r.ID, err)
		s.dc.fail(fmt.Errorf("drawing image: %w", err))
		return
	}

	s.transform(concreteWidth, 0, 0, -concreteHeight, 0, concreteHeight)
	s.drawImage(img, interpolate)
}

// draw is SVGImage.draw in weasyprint/images.py, with SVG.draw of
// weasyprint/svg/__init__.py.
//
// Port: upstream draws the SVG image as vectors with its own port of SVG,
// which ivy does not port (D15). okage renders the image into pixels at the
// scale of the device, and the stream draws the pixels. The root element
// gets the concrete size, as set_svg_size does, and the content that
// overflows the concrete size is drawn, as upstream draws it. The images
// that the SVG data links to are read with the fetcher of the document.
func (s *SVGImage) draw(st *stream, concreteWidth, concreteHeight float64, _ *Style) {
	if !(concreteWidth > 0) || !(concreteHeight > 0) {
		return
	}
	doc, err := s.sizedDocument(concreteWidth, concreteHeight)
	if err != nil {
		st.dc.log.error("Failed to render SVG image %s", s.BaseURL)
		st.dc.log.debug("Error while rendering SVG image: %v", err)
		return
	}
	opts := []svg.Option{
		svg.WithImageHrefResolver(svg.ImageHrefResolver{ResolveString: s.resolveHref}),
		svg.WithMaxDepth(uint32(min(max(s.limits.MaxDepth, 1), 1024))),
	}
	if st.dc.fonts != nil {
		opts = append(opts, svg.WithFontDatabase(st.dc.fonts))
	}
	toDevice := st.absolute()
	toUser, ok := toDevice.Invert()
	if !ok {
		return
	}
	img, toPixels, err := s.render(doc, toDevice, st.area, !st.dc.antiAlias, opts)
	if err != nil {
		st.dc.log.error("Failed to render SVG image %s", s.BaseURL)
		st.dc.log.debug("Error while rendering SVG image: %v", err)
		return
	}
	if img == nil {
		return
	}
	fromPixels, ok := toPixels.Invert()
	if !ok {
		return
	}
	// The pixels map to the device, then to the current user space.
	st.drawImageMatrix(img, fromPixels.Mul(toUser), true)
}

// sizedDocument returns the document of the SVG image with the concrete
// size on the root element, as set_svg_size and preserve_ratio of upstream
// size it: a viewBox fills the concrete size as its preserveAspectRatio
// says, an image without a viewBox and with an intrinsic size is stretched
// to the concrete size, and another image is drawn at its own scale.
//
// Port: upstream sets the attributes on its tree. The port builds a new
// document with roxmltree.Builder: the root element with the new
// attributes, then a copy of each child. A viewBox that okage does not
// accept counts as missing, as okage draws it.
func (s *SVGImage) sizedDocument(concreteWidth, concreteHeight float64) (*roxmltree.Document, error) {
	if s.err != nil {
		return nil, s.err
	}
	attributes := [][2]string{
		{"width", strconv.FormatFloat(concreteWidth, 'g', -1, 64)},
		{"height", strconv.FormatFloat(concreteHeight, 'g', -1, 64)},
	}
	if !s.dims.HasViewBox {
		// The intrinsic size of upstream, with 12pt for the font size.
		size := s.GetIntrinsicSize(1, 16)
		if size.HasWidth && size.HasHeight && size.Width > 0 && size.Height > 0 {
			viewBox := "0 0 " + strconv.FormatFloat(size.Width, 'g', -1, 64) + " " + strconv.FormatFloat(size.Height, 'g', -1, 64)
			attributes = append(attributes, [2]string{"viewBox", viewBox}, [2]string{"preserveAspectRatio", "none"})
		}
	}
	doc, err := buildSized(s.doc.RootElement(), attributes, svgParsingOptions(s.limits))
	if err != nil {
		return nil, fmt.Errorf("sizing SVG image %s: %w", s.BaseURL, err)
	}
	return doc, nil
}

// buildSized copies the root element of a document into a new document,
// with attributes in place of the attributes of the same names that have
// no namespace.
func buildSized(root roxmltree.Node, attributes [][2]string, options roxmltree.ParsingOptions) (*roxmltree.Document, error) {
	b := roxmltree.NewBuilder(options)
	if err := b.StartElement(root.TagName()); err != nil {
		return nil, err
	}
	for ns := range root.Namespaces() {
		prefix, _ := ns.Name()
		if err := b.Namespace(prefix, ns.URI()); err != nil {
			return nil, err
		}
	}
	for _, attr := range attributes {
		if err := b.Attribute(roxmltree.ExpandedNameFromStr(attr[0]), attr[1]); err != nil {
			return nil, err
		}
	}
	for attr := range root.Attributes() {
		name := roxmltree.ExpandedNameFromStr(attr.Name())
		if uri, ok := attr.Namespace(); ok {
			name = roxmltree.ExpandedNameFromTuple(uri, attr.Name())
		} else if slices.ContainsFunc(attributes, func(a [2]string) bool { return a[0] == attr.Name() }) {
			continue
		}
		if err := b.Attribute(name, attr.Value()); err != nil {
			return nil, err
		}
	}
	for child := range root.Children() {
		if err := b.Copy(child); err != nil {
			return nil, err
		}
	}
	if err := b.EndElement(); err != nil {
		return nil, err
	}
	return b.Document()
}

// render renders the sized document of the SVG image, whose user space maps to the device by
// toDevice, into an image of the pixels of the device inside area, which
// is (x, y, width, height) in pixels. It returns the image and the
// transform from the device to its pixels, or a nil image when the SVG
// image draws nothing in area.
//
// Without antialiasing, the image is rendered at 4 times the resolution of
// the device with crisp edges, and reduced with aliasDownsample. The image
// has at most maxCellPixels pixels, as boundCell says.
func (s *SVGImage) render(doc *roxmltree.Document, toDevice Matrix, area [4]float64, aliased bool, opts []svg.Option) (img *image.RGBA, toPixels Matrix, err error) {
	// Port: a panic in the parser or the renderer of okage is an image
	// that cannot be drawn.
	defer func() {
		if r := recover(); r != nil {
			img, err = nil, fmt.Errorf("rendering SVG image: panic: %v: %w", r, ivy.ErrFormat)
		}
	}()
	if aliased {
		opts = append(opts, svg.WithShapeRendering(svg.ShapeRenderingCrispEdges))
	}
	tree, err := svg.FromXMLTree(doc, opts...)
	if err != nil {
		return nil, Matrix{}, fmt.Errorf("parsing SVG image %s: %w", s.BaseURL, err)
	}
	bbox, ok := tree.Root().AbsLayerBoundingBox()
	if !ok {
		return nil, Matrix{}, nil
	}
	// The box of the content on the device, inside area.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, corner := range [4][2]float32{
		{bbox.Left(), bbox.Top()}, {bbox.Right(), bbox.Top()},
		{bbox.Left(), bbox.Bottom()}, {bbox.Right(), bbox.Bottom()},
	} {
		x, y := toDevice.TransformPoint(float64(corner[0]), float64(corner[1]))
		minX, minY = min(minX, x), min(minY, y)
		maxX, maxY = max(maxX, x), max(maxY, y)
	}
	minX, minY = math.Floor(max(minX, area[0])), math.Floor(max(minY, area[1]))
	maxX, maxY = math.Ceil(min(maxX, area[0]+area[2])), math.Ceil(min(maxY, area[1]+area[3]))
	if !(maxX > minX) || !(maxY > minY) {
		return nil, Matrix{}, nil
	}
	sub := 1
	if aliased {
		sub = 4
	}
	w, h, scale := boundCell((maxX-minX)*float64(sub), (maxY-minY)*float64(sub))
	w, h = max(1, w/sub), max(1, h/sub)
	toPixels = translation(-minX, -minY).Mul(Matrix{A: scale, D: scale})
	toSub := toPixels.Mul(Matrix{A: float64(sub), D: float64(sub)})
	subPixmap, ok := okage.NewPixmap(uint32(w*sub), uint32(h*sub))
	if !ok {
		return nil, Matrix{}, fmt.Errorf("SVG image of %d by %d pixels: %w", w*sub, h*sub, ivy.ErrLimit)
	}
	pm := subPixmap.AsMut()
	svg.Render(tree, toDevice.Mul(toSub).transform(), &pm)
	img = &image.RGBA{Pix: subPixmap.Take(), Stride: 4 * w * sub, Rect: image.Rect(0, 0, w*sub, h*sub)}
	if sub > 1 {
		img = aliasDownsample(img, sub)
	}
	return img, toPixels, nil
}

// aliasDownsample reduces an image of premultiplied colors that is k times
// larger than the device, drawn without antialiasing, to the pixels of the
// device. A pixel of the device that a sub-pixel covers takes the largest
// alpha of its sub-pixels and the average color of the covered sub-pixels,
// as a pixel that a path touches is painted without antialiasing.
func aliasDownsample(src *image.RGBA, k int) *image.RGBA {
	w, h := (src.Rect.Dx()+k-1)/k, (src.Rect.Dy()+k-1)/k
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			var r, g, b, a, maxA uint32
			for sy := y * k; sy < min((y+1)*k, src.Rect.Dy()); sy++ {
				for sx := x * k; sx < min((x+1)*k, src.Rect.Dx()); sx++ {
					p := src.Pix[sy*src.Stride+4*sx:]
					r, g, b, a = r+uint32(p[0]), g+uint32(p[1]), b+uint32(p[2]), a+uint32(p[3])
					maxA = max(maxA, uint32(p[3]))
				}
			}
			if a == 0 {
				continue
			}
			d := dst.Pix[y*dst.Stride+4*x:]
			d[0] = uint8(min(255, (r*maxA+a/2)/a))
			d[1] = uint8(min(255, (g*maxA+a/2)/a))
			d[2] = uint8(min(255, (b*maxA+a/2)/a))
			d[3] = uint8(maxA)
		}
	}
	return dst
}

// resolveHref reads an image that the SVG data links to with the fetcher of
// the document. It reads no file of the system.
func (s *SVGImage) resolveHref(href string) (svg.ImageKind, bool) {
	if s.fetcher == nil {
		return nil, false
	}
	response, err := s.fetcher.Fetch(urljoin(s.BaseURL, href))
	if err != nil {
		return nil, false
	}
	data := response.Body
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	switch format {
	case "png":
		return svg.ImageKindPNG(data), true
	case "jpeg":
		return svg.ImageKindJPEG(data), true
	case "gif":
		return svg.ImageKindGIF(data), true
	case "webp":
		return svg.ImageKindWEBP(data), true
	}
	return nil, false
}

// draw is Gradient.draw in weasyprint/images.py, for a linear gradient.
func (lg *LinearGradient) draw(s *stream, concreteWidth, concreteHeight float64, style *Style) {
	drawGradient(s, lg.Layout(concreteWidth, concreteHeight, style), lg.Repeating, concreteWidth, concreteHeight)
}

// draw is Gradient.draw in weasyprint/images.py, for a radial gradient.
func (rg *RadialGradient) draw(s *stream, concreteWidth, concreteHeight float64, style *Style) {
	drawGradient(s, rg.Layout(concreteWidth, concreteHeight, style), rg.Repeating, concreteWidth, concreteHeight)
}

// drawGradient is the body of Gradient.draw in weasyprint/images.py, with
// the layout of the gradient.
func drawGradient(s *stream, layout GradientLayout, repeating bool, concreteWidth, concreteHeight float64) {
	scaleY, colors, positions := layout.ScaleY, layout.Colors, layout.Positions

	if layout.Type == "solid" {
		s.rectangle(0, 0, concreteWidth, concreteHeight)
		if len(colors) > 0 {
			s.setColor(colors[0], false)
		}
		s.fill(false)
		return
	}
	// Port: upstream fails on a layout without two stops.
	if len(colors) < 2 || len(positions) != len(colors) || scaleY == 0 {
		return
	}

	hint := func(i int) float64 {
		if i < len(layout.Hints) {
			return layout.Hints[i]
		}
		return 1
	}
	alphas := make([]float64, len(colors))
	rgbs := make([][]float64, len(colors))
	for i, c := range colors {
		r, g, b, a := srgbOf(c)
		alphas[i], rgbs[i] = a, []float64{r, g, b}
	}
	type couple struct {
		c0, c1 []float64
		hint   float64
	}
	alphaCouples := make([]couple, len(alphas)-1)
	colorCouples := make([]couple, len(colors)-1)
	for i := range len(colors) - 1 {
		alphaCouples[i] = couple{[]float64{alphas[i]}, []float64{alphas[i+1]}, hint(i)}
		// TODO: handle other color spaces.
		colorCouples[i] = couple{rgbs[i], rgbs[i+1], hint(i)}
	}

	// Premultiply the colors.
	for i, alpha := range alphas {
		if alpha == 0 {
			if i > 0 {
				colorCouples[i-1].c1 = colorCouples[i-1].c0
			}
			if i < len(colors)-1 {
				colorCouples[i].c0 = colorCouples[i].c1
			}
		}
	}
	for i, ac := range alphaCouples {
		a0, a1 := ac.c0[0], ac.c1[0]
		if a0 != 0 && a1 != 0 && (a0 != 1 || a1 != 1) {
			colorCouples[i].hint = a0 / a1
		}
	}

	shadingType := 2
	if layout.Type != "linear" {
		shadingType = 3
	}
	domain := [2]float64{positions[0], positions[len(positions)-1]}
	extend := !repeating
	encode := make([]float64, 0, 2*(len(colors)-1))
	for range len(colors) - 1 {
		encode = append(encode, 0, 1)
	}
	bounds := positions[1 : len(positions)-1]
	subFunctions := make([]pdfFunction, len(colorCouples))
	for i, cc := range colorCouples {
		subFunctions[i] = createInterpolationFunction([2]float64{0, 1}, cc.c0, cc.c1, cc.hint)
	}
	function := createStitchingFunction(domain, encode, bounds, subFunctions)
	sh := s.addShading(shadingType, domain, layout.Points, extend, function)
	// Port: upstream paints the shading over the whole clip, so a gradient
	// with a background-size and no-repeat covers the whole painting area.
	// The port clips the shading to the size of the image.
	s.rectangle(0, 0, concreteWidth, concreteHeight)
	s.clip(false)
	s.end()
	s.transform(1, 0, 0, scaleY, 0, 0)

	if slices.ContainsFunc(alphas, func(alpha float64) bool { return alpha != 1 }) {
		alphaStream := s.setAlphaState(0, 0, concreteWidth, concreteHeight/scaleY, "luminosity")

		alphaFunctions := make([]pdfFunction, len(alphaCouples))
		for i, ac := range alphaCouples {
			alphaFunctions[i] = createInterpolationFunction([2]float64{0, 1}, ac.c0, ac.c1, ac.hint)
		}
		function := createStitchingFunction(domain, encode, bounds, alphaFunctions)
		alphaShading := alphaStream.addShading(shadingType, domain, layout.Points, extend, function)
		alphaStream.transform(1, 0, 0, scaleY, 0, 0)
		alphaStream.paintShading(alphaShading)
	}

	s.paintShading(sh)
}
