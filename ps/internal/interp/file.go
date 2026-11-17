package interp

import (
	"errors"
	"io"
)

// source is the bytes of a file that a program reads.
type source interface {
	io.ByteReader
	// UnreadByte puts back the last byte that ReadByte returned.
	UnreadByte() error
}

// File is a file object (PLRM 3.8). A program reads its own text, a string,
// or a filter. It cannot open a file of the computer: ivy reads only the
// document.
type File struct {
	src    source
	name   string
	write  bool
	closed bool
	// closer closes the source of a filter that closes its source.
	closer func()
	// written counts the bytes that the program wrote to a file that
	// discards them.
	written int64
}

// nextByte reads one byte of a source, and false at its end or at an
// error, which ends the token or the data that the caller reads.
func nextByte(s io.ByteReader) (byte, bool) {
	b, err := s.ReadByte()
	return b, err == nil
}

// newFile returns a file that reads src.
func newFile(name string, src source) *File {
	return &File{src: src, name: name}
}

// ReadByte reads one byte. It returns io.EOF at the end of the file, or
// when the file is closed.
func (f *File) ReadByte() (byte, error) {
	if f.closed || f.src == nil {
		return 0, io.EOF
	}
	b, err := f.src.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return 0, io.EOF
		}
		// an error of a filter, such as a limit, goes to the operator as it
		// is
		return 0, err //nolint:wrapcheck
	}
	return b, nil
}

// UnreadByte puts back the last byte that ReadByte returned.
func (f *File) UnreadByte() error {
	if f.closed || f.src == nil {
		return io.EOF
	}
	return f.src.UnreadByte()
}

// Read reads into p. It makes the file an io.Reader for the decoders of the
// filters.
func (f *File) Read(p []byte) (int, error) {
	if m, ok := f.src.(*memSource); ok && !f.closed {
		return m.Read(p)
	}
	n := 0
	for n < len(p) {
		b, err := f.ReadByte()
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

// Close closes the file.
func (f *File) Close() {
	if f.closed {
		return
	}
	f.closed = true
	if f.closer != nil {
		f.closer()
	}
}

// memSource reads bytes from memory.
type memSource struct {
	data []byte
	pos  int
}

// ReadByte satisfies the io.ByteReader interface.
func (m *memSource) ReadByte() (byte, error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	b := m.data[m.pos]
	m.pos++
	return b, nil
}

// UnreadByte satisfies the source interface.
func (m *memSource) UnreadByte() error {
	if m.pos > 0 {
		m.pos--
	}
	return nil
}

// Read satisfies the io.Reader interface.
func (m *memSource) Read(p []byte) (int, error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n := copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

// bufSource reads the bytes that a function decodes, one chunk at a time.
type bufSource struct {
	buf  []byte
	pos  int
	err  error
	fill func() ([]byte, error)
	// empty counts the calls of fill in a row that returned no byte.
	empty int
}

// ReadByte satisfies the io.ByteReader interface.
func (s *bufSource) ReadByte() (byte, error) {
	for s.pos >= len(s.buf) {
		if s.err != nil {
			return 0, s.err
		}
		// keep the last byte so that UnreadByte works across chunks
		var last []byte
		if len(s.buf) > 0 {
			last = s.buf[len(s.buf)-1:]
		}
		chunk, err := s.fill()
		s.err = err
		if len(chunk) == 0 {
			s.empty++
			if s.empty > 64 && s.err == nil {
				s.err = io.EOF
			}
			continue
		}
		s.empty = 0
		s.buf = append(append(s.buf[:0], last...), chunk...)
		s.pos = len(last)
	}
	b := s.buf[s.pos]
	s.pos++
	return b, nil
}

// UnreadByte satisfies the source interface.
func (s *bufSource) UnreadByte() error {
	if s.pos > 0 {
		s.pos--
	}
	return nil
}
