package interp

import (
	"bytes"
	"errors"
	"io"
)

// maxPrinted bounds the bytes that a program prints to the log.
const maxPrinted = 64 << 10

// print sends the text that a program prints to the log, up to a bound.
func (in *Interp) print(text string) {
	if in.printed >= maxPrinted {
		return
	}
	in.printed += len(text)
	in.log.Debug("postscript: print", "text", text)
}

// currentFile returns the file that the innermost file frame reads.
func (in *Interp) currentFile() Object {
	for i := len(in.es) - 1; i >= 0; i-- {
		if in.es[i].kind == fFile {
			return in.es[i].obj.Literal()
		}
	}
	f := newFile("closed", nil)
	f.closed = true
	return Object{typ: TFile, ref: f}
}

// popFile pops a file.
func (in *Interp) popFile() (*File, error) {
	o, err := in.PopType(TFile)
	if err != nil {
		return nil, err
	}
	return o.File(), nil
}

// opsFile are the operators of files (PLRM 8.1). A program cannot read or
// change a file of the computer: file opens only the standard files.
func (in *Interp) opsFile() {
	in.Define("file", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		name, mode := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if name.typ != TString || mode.typ != TString {
			return errTypecheck
		}
		var f *File
		switch name.Text() {
		case "%stdin", "%lineedit", "%statementedit":
			f = newFile(name.Text(), &memSource{})
		case "%stdout", "%stderr":
			f = newFile(name.Text(), nil)
			f.write = true
		default:
			return ErrUndefinedFilename
		}
		in.os = in.os[:len(in.os)-2]
		in.Push(Object{typ: TFile, ref: f})
		return nil
	})
	in.Define("filter", opFilter)
	in.Define("closefile", func(in *Interp) error {
		f, err := in.popFile()
		if err != nil {
			return err
		}
		f.Close()
		return nil
	})
	in.Define("read", func(in *Interp) error {
		f, err := in.popFile()
		if err != nil {
			return err
		}
		b, err := f.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				in.Push(Bool(false))
				return nil
			}
			return ioError(err)
		}
		in.Push(Int(int(b)))
		in.Push(Bool(true))
		return nil
	})
	in.Define("readstring", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		fo, s := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if fo.typ != TFile || s.typ != TString {
			return errTypecheck
		}
		if s.n == 0 {
			return errRangecheck
		}
		n, err := io.ReadFull(fo.File(), s.Bytes())
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return ioError(err)
		}
		in.os = in.os[:len(in.os)-2]
		full := n == int(s.n)
		s.n = int32(n)
		in.Push(s)
		in.Push(Bool(full))
		return nil
	})
	in.Define("readhexstring", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		fo, s := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if fo.typ != TFile || s.typ != TString {
			return errTypecheck
		}
		f, dst := fo.File(), s.Bytes()
		n, half, hi := 0, false, 0
		for n < len(dst) {
			b, err := f.ReadByte()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return ioError(err)
				}
				break
			}
			v := hexValue(b)
			if v < 0 {
				continue
			}
			if half {
				dst[n] = byte(hi<<4 | v)
				n++
			} else {
				hi = v
			}
			half = !half
		}
		in.os = in.os[:len(in.os)-2]
		full := n == len(dst)
		s.n = int32(n)
		in.Push(s)
		in.Push(Bool(full))
		return nil
	})
	in.Define("readline", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		fo, s := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if fo.typ != TFile || s.typ != TString {
			return errTypecheck
		}
		f, dst := fo.File(), s.Bytes()
		n, found := 0, false
		for {
			b, err := f.ReadByte()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return ioError(err)
				}
				break
			}
			if b == '\n' {
				found = true
				break
			}
			if b == '\r' {
				if c, err := f.ReadByte(); err == nil && c != '\n' {
					_ = f.UnreadByte()
				}
				found = true
				break
			}
			if n >= len(dst) {
				return errRangecheck
			}
			dst[n] = b
			n++
		}
		in.os = in.os[:len(in.os)-2]
		s.n = int32(n)
		in.Push(s)
		in.Push(Bool(found))
		return nil
	})
	in.Define("token", func(in *Interp) error {
		o, err := in.Peek(0)
		if err != nil {
			return err
		}
		switch o.typ {
		case TFile:
			t, ok, err := in.token(o.File(), 0)
			if err != nil {
				return err
			}
			in.os = in.os[:len(in.os)-1]
			if ok {
				in.Push(t)
			}
			in.Push(Bool(ok))
		case TString:
			m := &memSource{data: o.Bytes()}
			t, ok, err := in.token(newFile("string", m), 0)
			if err != nil {
				return err
			}
			in.os = in.os[:len(in.os)-1]
			if ok {
				// a white space after the token is consumed (PLRM token)
				post := o
				post.off += int32(m.pos)
				post.n -= int32(m.pos)
				in.Push(post)
				in.Push(t)
			}
			in.Push(Bool(ok))
		default:
			return errTypecheck
		}
		return nil
	})
	in.Define("write", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		if in.os[len(in.os)-2].typ != TFile || in.os[len(in.os)-1].typ != TInt {
			return errTypecheck
		}
		in.os[len(in.os)-2].File().written++
		in.os = in.os[:len(in.os)-2]
		return nil
	})
	for _, name := range []string{"writestring", "writehexstring"} {
		in.Define(name, func(in *Interp) error {
			if len(in.os) < 2 {
				return errUnderflow
			}
			fo, s := in.os[len(in.os)-2], in.os[len(in.os)-1]
			if fo.typ != TFile || s.typ != TString {
				return errTypecheck
			}
			f := fo.File()
			f.written += int64(s.n)
			if f.name == "%stdout" || f.name == "%stderr" {
				in.print(s.Text())
			}
			in.os = in.os[:len(in.os)-2]
			return nil
		})
	}
	in.Define("bytesavailable", func(in *Interp) error {
		f, err := in.popFile()
		if err != nil {
			return err
		}
		if m, ok := f.src.(*memSource); ok && !f.closed {
			in.Push(Int(len(m.data) - m.pos))
			return nil
		}
		in.Push(Int(-1))
		return nil
	})
	// flushfile reads an input file up to its end and discards the data
	// (PLRM 8.2). Adobe Illustrator skips its XMP metadata so, through a
	// SubFileDecode filter of currentfile.
	in.Define("flushfile", func(in *Interp) error {
		f, err := in.popFile()
		if err != nil || f.write {
			return err
		}
		for n := 0; ; n++ {
			if _, err := f.ReadByte(); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return ioError(err)
			}
			if n&0xffff == 0xffff && in.ctx != nil {
				if err := in.ctx.Err(); err != nil {
					return err
				}
			}
		}
	})
	in.Define("resetfile", func(in *Interp) error {
		_, err := in.popFile()
		return err
	})
	in.Define("flush", func(*Interp) error { return nil })
	in.Define("status", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		switch o.typ {
		case TFile:
			in.Push(Bool(!o.File().closed))
		case TString:
			in.Push(Bool(false))
		default:
			return errTypecheck
		}
		return nil
	})
	in.Define("run", func(in *Interp) error {
		if _, err := in.PopType(TString); err != nil {
			return err
		}
		return ErrUndefinedFilename
	})
	in.Define("currentfile", func(in *Interp) error {
		in.Push(in.currentFile())
		return nil
	})
	for _, name := range []string{"deletefile", "renamefile"} {
		in.Define(name, func(*Interp) error {
			return ErrInvalidFileAccess
		})
	}
	in.Define("filenameforall", func(in *Interp) error {
		if len(in.os) < 3 {
			return errUnderflow
		}
		in.os = in.os[:len(in.os)-3]
		return nil
	})
	in.Define("fileposition", func(in *Interp) error {
		f, err := in.popFile()
		if err != nil {
			return err
		}
		m, ok := f.src.(*memSource)
		if !ok {
			return ErrIO
		}
		in.Push(Int(m.pos))
		return nil
	})
	in.Define("setfileposition", func(in *Interp) error {
		if len(in.os) < 2 {
			return errUnderflow
		}
		fo, po := in.os[len(in.os)-2], in.os[len(in.os)-1]
		if fo.typ != TFile || po.typ != TInt {
			return errTypecheck
		}
		m, ok := fo.File().src.(*memSource)
		if !ok || po.i < 0 || int(po.i) > len(m.data) {
			return ErrIO
		}
		m.pos = int(po.i)
		in.os = in.os[:len(in.os)-2]
		return nil
	})
	in.Define("print", func(in *Interp) error {
		o, err := in.PopType(TString)
		if err != nil {
			return err
		}
		in.print(o.Text())
		return nil
	})
	in.Define("=", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		in.print(in.cvs(o))
		return nil
	})
	in.Define("==", func(in *Interp) error {
		o, err := in.Pop()
		if err != nil {
			return err
		}
		in.print(in.repr(o, 0))
		return nil
	})
	for _, name := range []string{"stack", "pstack"} {
		in.Define(name, func(in *Interp) error {
			var b bytes.Buffer
			for i := len(in.os) - 1; i >= 0 && b.Len() < 4096; i-- {
				b.WriteString(in.repr(in.os[i], 0))
				b.WriteByte('\n')
			}
			in.print(b.String())
			return nil
		})
	}
	in.Define("printobject", func(in *Interp) error {
		_, err := in.PopNums(0)
		if err != nil {
			return err
		}
		if len(in.os) < 2 {
			return errUnderflow
		}
		in.os = in.os[:len(in.os)-2]
		return nil
	})
	in.Define("writeobject", func(in *Interp) error {
		if len(in.os) < 3 {
			return errUnderflow
		}
		in.os = in.os[:len(in.os)-3]
		return nil
	})
	in.Define("setobjectformat", func(in *Interp) error {
		n, err := in.PopInt()
		in.objectFormat = n
		return err
	})
	in.Define("currentobjectformat", func(in *Interp) error {
		in.Push(Int(in.objectFormat))
		return nil
	})
	in.Define("echo", func(in *Interp) error {
		_, err := in.PopBool()
		return err
	})
	for _, name := range []string{"prompt", "executive"} {
		in.Define(name, func(*Interp) error { return nil })
	}
}
