package msppt

import (
	"context"
	"encoding/binary"
	"fmt"
	"iter"

	"github.com/xo/magic/cfb"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/msole"
	"github.com/xo/ivy/internal/offcrypto"
)

var le = binary.LittleEndian

// The types of the records that the package reads (MS-PPT 2.13.24).
const (
	rtDocument            = 0x03E8
	rtDocumentAtom        = 0x03E9
	rtSlide               = 0x03EE
	rtSlidePersistAtom    = 0x03F3
	rtPPDrawing           = 0x040C
	rtPlaceholderAtom     = 0x0BC3
	rtOutlineTextRefAtom  = 0x0F9E
	rtTextHeaderAtom      = 0x0F9F
	rtTextCharsAtom       = 0x0FA0
	rtTextBytesAtom       = 0x0FA8
	rtSlideListWithText   = 0x0FF0
	rtUserEditAtom        = 0x0FF5
	rtCurrentUserAtom     = 0x0FF6
	rtPersistDirectory    = 0x1772
	rtCryptSession10      = 0x2F14
	rtOfficeArtDg         = 0xF002
	rtOfficeArtSpgr       = 0xF003
	rtOfficeArtSp         = 0xF004
	rtOfficeArtFSPGR      = 0xF009
	rtOfficeArtFSP        = 0xF00A
	rtOfficeArtClientText = 0xF00D
	rtOfficeArtChild      = 0xF00F
	rtOfficeArtClientAnch = 0xF010
	rtOfficeArtClientData = 0xF011
)

// The tokens of the CurrentUserAtom (MS-PPT 2.3.2).
const (
	tokenClear     = 0xE391C05F
	tokenEncrypted = 0xF3D1C4DF
)

// header is the header of a record (MS-PPT 2.3.1).
type header struct {
	ver, inst uint16
	typ       uint16
	size      uint32
}

// readHeader reads the header of the record at the start of b.
func readHeader(b []byte) (header, bool) {
	if len(b) < 8 {
		return header{}, false
	}
	vi := le.Uint16(b)
	return header{ver: vi & 0xF, inst: vi >> 4, typ: le.Uint16(b[2:]), size: le.Uint32(b[4:])}, true
}

// children iterates over the records in the body b of a container. It stops
// at a record that b cuts.
func children(b []byte) iter.Seq2[header, []byte] {
	return func(yield func(header, []byte) bool) {
		for len(b) >= 8 {
			h, _ := readHeader(b)
			if uint64(h.size) > uint64(len(b)-8) {
				return
			}
			if !yield(h, b[8:8+h.size]) {
				return
			}
			b = b[8+h.size:]
		}
	}
}

// child returns the body of the first child of the type typ in the body b
// of a container, or nil.
func child(b []byte, typ uint16) []byte {
	for h, body := range children(b) {
		if h.typ == typ {
			return body
		}
	}
	return nil
}

// stream is the stream "PowerPoint Document" with its persist directory,
// and the key when a password protects it.
type stream struct {
	data    []byte
	persist map[uint32]uint32
	docRef  uint32
	key     *offcrypto.Key
	lim     ivy.Limits
}

// readStream reads the stream "PowerPoint Document" of f, through the
// stream "Current User" and the chain of UserEditAtoms, to the persist
// directory (MS-PPT 2.1.2).
func readStream(ctx context.Context, f *cfb.File, cfg ivy.Config) (*stream, error) {
	lim := cfg.Limits
	cu, err := msole.ReadStream(f, "Current User", lim)
	if err != nil {
		return nil, err
	}
	h, ok := readHeader(cu)
	if !ok || h.typ != rtCurrentUserAtom || len(cu) < 20 {
		return nil, fmt.Errorf("powerpoint file without a current user atom: %w", ivy.ErrFormat)
	}
	token := le.Uint32(cu[12:])
	edit := le.Uint32(cu[16:])
	data, err := msole.ReadStream(f, "PowerPoint Document", lim)
	if err != nil {
		return nil, err
	}
	s := &stream{data: data, persist: make(map[uint32]uint32), lim: lim}
	var crypt uint32
	first := true
	seen := make(map[uint32]bool)
	for edit != 0 || first {
		if seen[edit] || len(seen) > lim.MaxObjects {
			break
		}
		seen[edit] = true
		ue, ok := s.record(edit, rtUserEditAtom)
		if !ok || len(ue) < 28 {
			if first {
				return nil, fmt.Errorf("powerpoint file without a user edit atom at %d: %w", edit, ivy.ErrFormat)
			}
			break
		}
		if first {
			s.docRef = le.Uint32(ue[16:])
			if len(ue) >= 32 {
				crypt = le.Uint32(ue[28:])
			}
		}
		first = false
		if err := s.readDirectory(le.Uint32(ue[12:])); err != nil {
			return nil, err
		}
		edit = le.Uint32(ue[8:])
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("reading powerpoint file: %w", err)
		}
	}
	if token == tokenEncrypted || crypt != 0 {
		if err := s.unlock(ctx, crypt, cfg); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// record returns the body of the record of the type typ at off, or false.
func (s *stream) record(off uint32, typ uint16) ([]byte, bool) {
	if uint64(off) >= uint64(len(s.data)) {
		return nil, false
	}
	h, ok := readHeader(s.data[off:])
	if !ok || h.typ != typ || uint64(off)+8+uint64(h.size) > uint64(len(s.data)) {
		return nil, false
	}
	return s.data[off+8 : off+8+h.size], true
}

// readDirectory adds the entries of the PersistDirectoryAtom at off that a
// later edit did not give (MS-PPT 2.3.4).
func (s *stream) readDirectory(off uint32) error {
	b, ok := s.record(off, rtPersistDirectory)
	if !ok {
		return fmt.Errorf("powerpoint file without a persist directory at %d: %w", off, ivy.ErrFormat)
	}
	for len(b) >= 4 {
		v := le.Uint32(b)
		id, n := v&0xFFFFF, int(v>>20)
		b = b[4:]
		if 4*n > len(b) {
			return fmt.Errorf("persist directory entry of %d offsets: %w", n, ivy.ErrFormat)
		}
		for i := range n {
			if _, ok := s.persist[id+uint32(i)]; !ok {
				s.persist[id+uint32(i)] = le.Uint32(b[4*i:])
			}
		}
		if len(s.persist) > s.lim.MaxObjects {
			return fmt.Errorf("persist directory of more than %d objects: %w", s.lim.MaxObjects, ivy.ErrLimit)
		}
		b = b[4*n:]
	}
	return nil
}

// unlock reads the CryptSession10Container of the persist object crypt, and
// derives the key of the password (MS-PPT 2.3.7).
func (s *stream) unlock(ctx context.Context, crypt uint32, cfg ivy.Config) error {
	off, ok := s.persist[crypt]
	if !ok {
		return fmt.Errorf("encrypted powerpoint file without its encryption info: %w", ivy.ErrFormat)
	}
	b, ok := s.record(off, rtCryptSession10)
	if !ok {
		return fmt.Errorf("encrypted powerpoint file without a crypt session at %d: %w", off, ivy.ErrFormat)
	}
	info, err := offcrypto.ParseEncryptionInfo(b)
	if err != nil {
		return err
	}
	if info.Scheme != offcrypto.RC4CryptoAPI {
		return fmt.Errorf("powerpoint file with %v encryption: %w", info.Scheme, ivy.ErrUnsupported)
	}
	key, err := msole.Key(ctx, info, cfg)
	if err != nil {
		return fmt.Errorf("opening powerpoint file: %w", err)
	}
	s.key = key
	// the crypt session itself stays in the clear
	delete(s.persist, crypt)
	return nil
}

// object returns the header and the body of the persist object id,
// decrypted with the block number id if a password protects the file.
func (s *stream) object(id uint32) (header, []byte, bool) {
	off, ok := s.persist[id]
	if !ok || uint64(off)+8 > uint64(len(s.data)) {
		return header{}, nil, false
	}
	raw := s.data[off:]
	if s.key == nil {
		h, _ := readHeader(raw)
		if uint64(h.size) > uint64(len(raw)-8) {
			return header{}, nil, false
		}
		return h, raw[8 : 8+h.size], true
	}
	c, err := s.key.Stream(id)
	if err != nil {
		return header{}, nil, false
	}
	var hb [8]byte
	c.XORKeyStream(hb[:], raw[:8])
	h, _ := readHeader(hb[:])
	if uint64(h.size) > uint64(len(raw)-8) || int64(h.size) > s.lim.MaxMemory {
		return header{}, nil, false
	}
	plain := make([]byte, 8+h.size)
	if c, err = s.key.Stream(id); err != nil {
		return header{}, nil, false
	}
	c.XORKeyStream(plain, raw[:8+h.size])
	return h, plain[8:], true
}
