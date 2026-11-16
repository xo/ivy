package camera

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/xo/ivy"
)

// rafMagic starts a RAF file of Fujifilm.
const rafMagic = "FUJIFILMCCD-RAW "

// readRAF reads the header of a RAF file. The header holds the name of the
// camera at 28, and the offset and the length of the JPEG at 84 and 88, big
// endian.
func (d *Document) readRAF() ([]region, error) {
	head := make([]byte, 92)
	if n, _ := d.src.ReadAt(head, 0); n < len(head) {
		return nil, fmt.Errorf("short raf header: %w", ivy.ErrFormat)
	}
	name, _, _ := bytes.Cut(head[28:60], []byte{0})
	d.info.Producer = strings.TrimSpace(string(name))
	return []region{{
		off: int64(binary.BigEndian.Uint32(head[84:])),
		n:   int64(binary.BigEndian.Uint32(head[88:])),
	}}, nil
}
