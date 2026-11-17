package interp

import (
	"bytes"
	"compress/flate"
	"compress/lzw"
	"fmt"
	"image/jpeg"
	"io"
	"strconv"

	tifflzw "golang.org/x/image/tiff/lzw"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/hayro/syntax"
)

// chunkSize is the number of bytes that a filter decodes at a time.
const chunkSize = 4096

// opFilter is filter (PLRM 3.13). The decoding filters read a file, a
// string or a procedure. The encoding filters discard what a program writes
// to them.
func opFilter(in *Interp) error {
	nameObj, err := in.Peek(0)
	if err != nil {
		return err
	}
	if nameObj.typ != TName {
		return errTypecheck
	}
	name := nameObj.NameVal()
	depth := 1
	var params *Dict
	if o, err := in.Peek(1); err == nil && o.typ == TDict {
		params = o.Dict()
		depth = 2
	}
	var eodCount int
	var eodString []byte
	switch name {
	case "SubFileDecode":
		if params != nil {
			if v, ok := params.GetName("EODCount"); ok && v.typ == TInt {
				eodCount = int(v.i)
			}
			if v, ok := params.GetName("EODString"); ok && v.typ == TString {
				eodString = append([]byte(nil), v.Bytes()...)
			}
		} else {
			s, err := in.Peek(1)
			if err != nil {
				return err
			}
			c, err := in.Peek(2)
			if err != nil {
				return err
			}
			if s.typ != TString || c.typ != TInt {
				return errTypecheck
			}
			if c.i < 0 {
				return errRangecheck
			}
			eodCount, eodString = int(c.i), append([]byte(nil), s.Bytes()...)
			depth = 3
		}
	case "RunLengthEncode":
		if params == nil {
			depth = 2
		}
	}
	srcObj, err := in.Peek(depth)
	if err != nil {
		return err
	}
	var f *File
	switch name {
	case "NullEncode", "ASCIIHexEncode", "ASCII85Encode", "RunLengthEncode", "LZWEncode",
		"FlateEncode", "DCTEncode", "CCITTFaxEncode":
		f = newFile(name, nil)
		f.write = true
	default:
		src, err := in.dataSource(srcObj)
		if err != nil {
			return err
		}
		if f, err = in.decodeFilter(name, params, src, eodCount, eodString); err != nil {
			return err
		}
	}
	in.os = in.os[:len(in.os)-depth-1]
	in.Push(Object{typ: TFile, ref: f})
	return nil
}

// dataSource returns the source of the data of a file, a string or a
// procedure.
func (in *Interp) dataSource(o Object) (source, error) {
	switch {
	case o.typ == TFile:
		return o.File(), nil
	case o.typ == TString:
		return &memSource{data: o.Bytes()}, nil
	case o.IsProc():
		return in.procSource(o), nil
	}
	return nil, errTypecheck
}

// procSource returns the source of the strings that a procedure returns,
// up to an empty string (PLRM 3.13.1).
func (in *Interp) procSource(proc Object) *bufSource {
	return &bufSource{fill: func() ([]byte, error) {
		if err := in.Call(proc); err != nil {
			return nil, err
		}
		s, err := in.PopType(TString)
		if err != nil {
			return nil, err
		}
		if s.n == 0 {
			return nil, io.EOF
		}
		return append([]byte(nil), s.Bytes()...), nil
	}}
}

// NewDataSource returns a reader of the data of a file, a string or a
// procedure, for the operators of images.
func (in *Interp) NewDataSource(o Object) (io.Reader, error) {
	src, err := in.dataSource(o)
	if err != nil {
		return nil, err
	}
	return &srcReader{src: src}, nil
}

// srcReader reads a source one byte at a time, so that a decoder reads no
// byte past the end of its data.
type srcReader struct {
	src source
	// tee keeps the bytes read, when it is not nil.
	tee *bytes.Buffer
}

// ReadByte satisfies the io.ByteReader interface.
func (r *srcReader) ReadByte() (byte, error) {
	b, err := r.src.ReadByte()
	if err == nil && r.tee != nil {
		r.tee.WriteByte(b)
	}
	return b, err //nolint:wrapcheck // the error of the source goes on as it is
}

// Read satisfies the io.Reader interface.
func (r *srcReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		b, err := r.ReadByte()
		if err != nil {
			if n > 0 {
				return n, nil
			}
			return 0, err
		}
		p[n] = b
		n++
	}
	return n, nil
}

// decodeFilter returns the file of a decoding filter.
func (in *Interp) decodeFilter(name string, params *Dict, src source, eodCount int, eodString []byte) (*File, error) {
	r := &srcReader{src: src}
	var fill func() ([]byte, error)
	switch name {
	case "ASCIIHexDecode":
		fill = hexFill(r)
	case "ASCII85Decode":
		fill = a85Fill(r)
	case "RunLengthDecode":
		fill = runLengthFill(r)
	case "SubFileDecode":
		fill = subFileFill(r, eodCount, eodString)
	case "FlateDecode", "LZWDecode":
		fill = in.onceFill(func() ([]byte, error) { return in.decodeLZWFlate(name, params, r) })
	case "DCTDecode":
		fill = in.onceFill(func() ([]byte, error) {
			data, err := jpegBytes(r, in.lim.MaxMemory)
			if err != nil {
				return nil, err
			}
			return in.decodeSyntax(name, params, data)
		})
	case "CCITTFaxDecode":
		fill = in.onceFill(func() ([]byte, error) {
			data, err := readAll(r, in.lim.MaxMemory)
			if err != nil {
				return nil, err
			}
			return in.decodeSyntax(name, params, data)
		})
	default:
		return nil, ErrUndefinedResource
	}
	return newFile(name, &bufSource{fill: fill}), nil
}

// onceFill returns a fill function that decodes all the data at its first
// call, and returns it in chunks.
func (in *Interp) onceFill(decode func() ([]byte, error)) func() ([]byte, error) {
	var data []byte
	done := false
	return func() ([]byte, error) {
		if !done {
			done = true
			var err error
			if data, err = decode(); err != nil {
				if fatal(err) {
					return nil, err
				}
				in.log.Debug("postscript: filter", "err", err)
				data = nil
			}
		}
		if len(data) == 0 {
			return nil, io.EOF
		}
		n := min(len(data), 64<<10)
		chunk := data[:n]
		data = data[n:]
		return chunk, nil
	}
}

// readAll reads all of r, up to max bytes.
func readAll(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("postscript filter data past %d bytes: %w", maxBytes, ivy.ErrLimit)
	}
	// an error of the data ends it, and a limit or the context ends the
	// program
	if fatal(err) {
		return nil, fmt.Errorf("reading postscript filter data: %w", err)
	}
	return data, nil
}

// hexFill decodes ASCIIHexDecode, up to the >.
func hexFill(r *srcReader) func() ([]byte, error) {
	done := false
	return func() ([]byte, error) {
		if done {
			return nil, io.EOF
		}
		out := make([]byte, 0, chunkSize)
		half, hi := false, 0
		for len(out) < chunkSize || half {
			b, err := r.ReadByte()
			if err != nil || b == '>' {
				done = true
				if half {
					out = append(out, byte(hi<<4))
				}
				break
			}
			v := hexValue(b)
			if v < 0 {
				if byteClass[b] == clsSpace {
					continue
				}
				done = true
				return out, ErrIO
			}
			if half {
				out = append(out, byte(hi<<4|v))
			} else {
				hi = v
			}
			half = !half
		}
		return out, nil
	}
}

// a85Fill decodes ASCII85Decode, up to the ~>.
func a85Fill(r *srcReader) func() ([]byte, error) {
	done := false
	var d a85Decoder
	return func() ([]byte, error) {
		if done {
			return nil, io.EOF
		}
		out := make([]byte, 0, chunkSize+4)
		for len(out) < chunkSize {
			b, err := r.ReadByte()
			if err != nil || b == '~' {
				if err == nil {
					// the > after the ~
					if c, err := r.ReadByte(); err == nil && c != '>' {
						_ = r.src.UnreadByte()
					}
				}
				done = true
				out, _ = d.finish(out)
				break
			}
			var ok bool
			if out, ok = d.add(out, b); !ok {
				done = true
				return out, ErrIO
			}
		}
		return out, nil
	}
}

// runLengthFill decodes RunLengthDecode, up to the code 128.
func runLengthFill(r *srcReader) func() ([]byte, error) {
	done := false
	return func() ([]byte, error) {
		if done {
			return nil, io.EOF
		}
		out := make([]byte, 0, chunkSize+128)
		for len(out) < chunkSize {
			n, ok := nextByte(r)
			if !ok || n == 128 {
				done = true
				break
			}
			if n < 128 {
				for range int(n) + 1 {
					b, ok := nextByte(r)
					if !ok {
						done = true
						return out, nil
					}
					out = append(out, b)
				}
				continue
			}
			b, err := r.ReadByte()
			if err != nil {
				done = true
				break
			}
			for range 257 - int(n) {
				out = append(out, b)
			}
		}
		return out, nil
	}
}

// subFileFill decodes SubFileDecode. With an end string, it passes eodCount
// occurrences of the string and stops before the next one. Without one, it
// passes eodCount bytes, or all the data when eodCount is 0.
func subFileFill(r *srcReader, eodCount int, eod []byte) func() ([]byte, error) {
	done := false
	remaining := eodCount
	var pending []byte
	return func() ([]byte, error) {
		if done {
			return nil, io.EOF
		}
		out := make([]byte, 0, chunkSize)
		if len(eod) == 0 {
			for len(out) < chunkSize {
				if eodCount > 0 && remaining == 0 {
					done = true
					break
				}
				b, err := r.ReadByte()
				if err != nil {
					done = true
					break
				}
				out = append(out, b)
				remaining--
			}
			return out, nil
		}
		for len(out) < chunkSize {
			b, err := r.ReadByte()
			if err != nil {
				done = true
				out = append(out, pending...)
				break
			}
			pending = append(pending, b)
			// keep the longest end of pending that starts eod
			for len(pending) > 0 && !bytes.HasPrefix(eod, pending) {
				out = append(out, pending[0])
				pending = pending[1:]
			}
			if len(pending) == len(eod) {
				if remaining == 0 {
					done = true
					break
				}
				remaining--
				out = append(out, pending...)
				pending = pending[:0]
			}
		}
		return out, nil
	}
}

// predictor returns the predictor of the parameters of a filter.
func predictor(params *Dict) int {
	if params == nil {
		return 1
	}
	if v, ok := params.GetName("Predictor"); ok && v.typ == TInt {
		return int(v.i)
	}
	return 1
}

// decodeLZWFlate decodes FlateDecode or LZWDecode. It reads its data
// with the decoders of Go, which stop at the end of the data. With a
// predictor, it decodes the bytes again with the port of hayro-syntax.
func (in *Interp) decodeLZWFlate(name string, params *Dict, r *srcReader) ([]byte, error) {
	if predictor(params) > 1 {
		r.tee = &bytes.Buffer{}
	}
	var rc io.Reader
	switch name {
	case "FlateDecode":
		h0, ok0 := nextByte(r)
		h1, ok1 := nextByte(r)
		if !ok0 || !ok1 {
			return nil, nil
		}
		if h0&0x0f == 8 && (uint16(h0)<<8|uint16(h1))%31 == 0 {
			rc = flate.NewReader(r)
		} else {
			rc = flate.NewReader(io.MultiReader(bytes.NewReader([]byte{h0, h1}), r))
		}
	default:
		early := true
		if params != nil {
			if v, ok := params.GetName("EarlyChange"); ok && v.typ == TInt {
				early = v.i != 0
			}
		}
		if early {
			rc = tifflzw.NewReader(r, tifflzw.MSB, 8)
		} else {
			rc = lzw.NewReader(r, lzw.MSB, 8)
		}
	}
	data, err := readAll(rc, in.lim.MaxMemory)
	if err != nil {
		return nil, err
	}
	if name == "FlateDecode" {
		// the checksum of zlib, which flate does not read
		for range 4 {
			if _, err := r.ReadByte(); err != nil {
				break
			}
		}
	}
	if r.tee == nil {
		return data, nil
	}
	return in.decodeSyntax(name, params, r.tee.Bytes())
}

// decodeSyntax decodes data with a filter of the port of hayro-syntax, from
// a stream of PDF that it makes.
func (in *Interp) decodeSyntax(name string, params *Dict, data []byte) ([]byte, error) {
	var head bytes.Buffer
	// the reader of a stream of hayro-syntax reads the dictionary and the
	// stream, without the header of an indirect object
	head.WriteString("<< /Filter /" + name)
	if params != nil {
		head.WriteString(" /DecodeParms ")
		head.Write(ToPDF(DictObj(params)))
	}
	head.WriteString(" /Length " + strconv.Itoa(len(data)) + " >>\nstream\n")
	head.Write(data)
	head.WriteString("\nendstream\n")
	stream, ok := syntax.FromBytes[syntax.Stream](head.Bytes())
	if !ok {
		return nil, fmt.Errorf("postscript %s data: %w", name, ivy.ErrFormat)
	}
	// the filters of images of hayro-syntax take the size of the image from
	// its dictionary, which a filter of PostScript does not have: a JPEG
	// gives its own size. CCITTFaxDecode gives bits, as an indexed image.
	ip := &syntax.ImageDecodeParams{}
	switch name {
	case "DCTDecode":
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("postscript %s data: %w: %w", name, ivy.ErrFormat, err)
		}
		ip.Width, ip.Height = uint32(cfg.Width), uint32(cfg.Height)
	case "CCITTFaxDecode":
		ip.IsIndexed = true
	}
	res, err := stream.DecodedImage(ip)
	if err != nil {
		return nil, fmt.Errorf("postscript %s data: %w", name, err)
	}
	return res.Data, nil
}

// jpegBytes reads the bytes of one JPEG image, up to its EOI marker, and no
// byte after it.
func jpegBytes(r *srcReader, maxBytes int64) ([]byte, error) {
	var out []byte
	// limit is the error of the limit of memory, which ends the read
	var limit error
	read := func() (byte, bool) {
		b, ok := nextByte(r)
		if !ok {
			return 0, false
		}
		if int64(len(out)) >= maxBytes {
			limit = fmt.Errorf("postscript jpeg past %d bytes: %w", maxBytes, ivy.ErrLimit)
			return 0, false
		}
		out = append(out, b)
		return b, true
	}
	// find SOI
	for {
		b, ok := read()
		if !ok {
			return out, limit
		}
		if b == 0xff {
			c, ok := read()
			if !ok {
				return out, limit
			}
			if c == 0xd8 {
				break
			}
		}
	}
	for {
		b, ok := read()
		if !ok {
			return out, limit
		}
		if b != 0xff {
			continue
		}
		m, ok := read()
		if !ok {
			return out, limit
		}
		for m == 0xff {
			if m, ok = read(); !ok {
				return out, limit
			}
		}
		switch {
		case m == 0xd9:
			return out, nil
		case m == 0 || m >= 0xd0 && m <= 0xd7 || m == 0x01:
			// stuffed byte, restart marker or TEM: in the entropy data
			continue
		}
		hi, ok1 := read()
		lo, ok2 := read()
		if !ok1 || !ok2 {
			return out, limit
		}
		n := int(hi)<<8 | int(lo)
		for range n - 2 {
			if _, ok := read(); !ok {
				return out, limit
			}
		}
		// after SOS, the entropy data runs to the next marker, which the
		// loop finds
	}
}
