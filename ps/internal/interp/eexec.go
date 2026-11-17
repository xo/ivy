package interp

import "io"

// The constants of the encryption of eexec ("Adobe Type 1 Font Format",
// 7.2).
const (
	eexecKey = 55665
	eexecC1  = 52845
	eexecC2  = 22719
)

// eexecSource decrypts the private part of a Type 1 font as eexec reads
// it. The ciphertext is binary, or hexadecimal when its first four bytes
// are hexadecimal digits.
type eexecSource struct {
	src     source
	r       uint16
	started bool
	hex     bool
	// head holds the first bytes, which tell the two forms apart.
	head []byte
	last byte
	back bool
}

// raw reads one byte of the source, after the bytes of the head.
func (e *eexecSource) raw() (byte, error) {
	if len(e.head) > 0 {
		b := e.head[0]
		e.head = e.head[1:]
		return b, nil
	}
	// the end of the source is the end of the decrypted text
	return e.src.ReadByte() //nolint:wrapcheck
}

// cipherByte reads one byte of ciphertext.
func (e *eexecSource) cipherByte() (byte, error) {
	if !e.hex {
		return e.raw()
	}
	var v [2]int
	for i := 0; i < 2; {
		b, err := e.raw()
		if err != nil {
			return 0, err
		}
		h := hexValue(b)
		if h < 0 {
			if isSpace(b) {
				continue
			}
			return 0, io.EOF
		}
		v[i] = h
		i++
	}
	return byte(v[0]<<4 | v[1]), nil
}

// isSpace reports whether b is a white-space character of PostScript.
func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\f', 0:
		return true
	}
	return false
}

// start skips the white space before the ciphertext, tells the two forms
// apart, and drops the four random bytes of the start.
func (e *eexecSource) start() error {
	e.started = true
	e.r = eexecKey
	var b byte
	var err error
	for {
		if b, err = e.src.ReadByte(); err != nil {
			return err //nolint:wrapcheck // the end of the source
		}
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			break
		}
	}
	e.head = append(e.head, b)
	for len(e.head) < 4 {
		if b, err = e.src.ReadByte(); err != nil {
			break
		}
		e.head = append(e.head, b)
	}
	e.hex = len(e.head) == 4
	for _, c := range e.head {
		e.hex = e.hex && hexValue(c) >= 0
	}
	for range 4 {
		if _, err := e.decrypt(); err != nil {
			return err
		}
	}
	return nil
}

// decrypt reads and decrypts one byte.
func (e *eexecSource) decrypt() (byte, error) {
	c, err := e.cipherByte()
	if err != nil {
		return 0, err
	}
	p := c ^ byte(e.r>>8)
	e.r = (uint16(c)+e.r)*eexecC1 + eexecC2
	return p, nil
}

// ReadByte satisfies the io.ByteReader interface.
func (e *eexecSource) ReadByte() (byte, error) {
	if e.back {
		e.back = false
		return e.last, nil
	}
	if !e.started {
		if err := e.start(); err != nil {
			return 0, err
		}
	}
	b, err := e.decrypt()
	if err != nil {
		return 0, err
	}
	e.last = b
	return b, nil
}

// UnreadByte satisfies the source interface.
func (e *eexecSource) UnreadByte() error {
	e.back = true
	return nil
}

// opsEexec defines eexec: it runs the decrypted text of a file or a string
// with systemdict on the dictionary stack, as a font program needs.
func (in *Interp) opsEexec() {
	in.Define("eexec", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		src, err := in.dataSource(o)
		if err != nil {
			return err
		}
		_, _ = in.Pop()
		f := newFile("eexec", &eexecSource{src: src})
		if err := in.Begin(in.SystemDict); err != nil {
			return err
		}
		if err := in.pushFrame(frame{kind: fCont, fn: func() error {
			if len(in.ds) > 3 && in.ds[len(in.ds)-1] == in.SystemDict {
				in.ds = in.ds[:len(in.ds)-1]
			}
			return nil
		}}); err != nil {
			return err
		}
		return in.pushFrame(frame{kind: fFile, obj: Object{typ: TFile, ref: f, exec: true}})
	})
}
