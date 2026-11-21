// ivy: this file is not in dejapdf. It holds the tests that ivy adds for
// the text layer: the books of the corpus of the review, the chunk TXTa,
// the limits, and the fuzz tests (D5, D26).

package doc

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/bzz"
	"github.com/xo/ivy/djvu/internal/iff"
)

// TestTextBooks compares the text layers of the books of the corpus of the
// review with text.sha256. Each line of text.sha256 holds the SHA-256 of the
// text of one page, as djvused print-txt prints it, with the file and the
// id of the page. IVY_DJVU_BOOKS names the folder of the books, such as
// test/review/cache/files/archive-org/djvu. Without it, the test skips.
func TestTextBooks(t *testing.T) {
	dir := os.Getenv("IVY_DJVU_BOOKS")
	if dir == "" {
		t.Skip("IVY_DJVU_BOOKS is not set")
	}
	want := map[string]map[string]string{}
	f, err := os.Open(filepath.Join(corpus, "text.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		w := strings.Fields(sc.Text())
		if len(w) != 3 {
			t.Fatalf("expected 3 fields, got: %q", sc.Text())
		}
		if want[w[1]] == nil {
			want[w[1]] = map[string]string{}
		}
		want[w[1]][w[2]] = w[0]
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	found := 0
	for name, pages := range want {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Logf("skip %s: %v", name, err)
			continue
		}
		found++
		d, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, c := range d.Dir.Components {
			cf, err := d.component(c)
			if err != nil {
				t.Fatal(err)
			}
			z, err := textOf(d.lim, cf)
			if err != nil {
				t.Errorf("%s %s: %v", name, c.ID, err)
				continue
			}
			if z == nil {
				if pages[c.ID] != "" {
					t.Errorf("%s %s: expected text, got none", name, c.ID)
				}
				continue
			}
			n++
			if got := fmt.Sprintf("%x", sha256.Sum256([]byte(zoneSexp(z).String()))); got != pages[c.ID] {
				t.Errorf("%s %s: expected %s, got: %s", name, c.ID, pages[c.ID], got)
			}
		}
		if n != len(pages) {
			t.Errorf("%s: expected %d pages with text, got: %d", name, len(pages), n)
		}
	}
	if found == 0 {
		t.Fatalf("expected a book of text.sha256 in %s", dir)
	}
}

// withTXTa returns text.djvu with its TXTz chunk decoded into a TXTa chunk.
func withTXTa(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(corpus, "text.djvu"))
	if err != nil {
		t.Fatal(err)
	}
	root, err := iff.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(root.Type)
	for _, c := range root.Children {
		if c.Composite() {
			t.Fatalf("expected only data chunks, got: %s", c.Name())
		}
		id, data := c.ID, c.Data
		if id == "TXTz" {
			if data, err = bzz.Decode(data); err != nil {
				t.Fatal(err)
			}
			id = "TXTa"
		}
		body = append(body, id...)
		body = binary.BigEndian.AppendUint32(body, uint32(len(data)))
		body = append(body, data...)
		if len(data)%2 == 1 {
			body = append(body, 0)
		}
	}
	out := append([]byte("AT&TFORM"), binary.BigEndian.AppendUint32(nil, uint32(len(body)))...)
	return append(out, body...)
}

// TestTextTXTa reads the same text from a TXTa chunk as from a TXTz chunk.
func TestTextTXTa(t *testing.T) {
	want, err := Open(filepath.Join(corpus, "text.djvu"))
	if err != nil {
		t.Fatal(err)
	}
	wz, err := want.Pages[0].Text()
	if err != nil || wz == nil {
		t.Fatalf("expected the text of text.djvu, got: %v, %v", wz, err)
	}
	b := withTXTa(t)
	if !bytes.Contains(b, []byte("TXTa")) || bytes.Contains(b, []byte("TXTz")) {
		t.Fatal("expected a TXTa chunk in place of the TXTz chunk")
	}
	d, err := Parse(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	z, err := d.Pages[0].Text()
	if err != nil {
		t.Fatal(err)
	}
	if got, w := zoneSexp(z).String(), zoneSexp(wz).String(); got != w {
		t.Errorf("expected:\n%s\ngot:\n%s", w, got)
	}
}

// TestTextZones checks the zones of text.djvu: each kind of zone, its
// rectangle, and the text with its separators.
func TestTextZones(t *testing.T) {
	d, err := Open(filepath.Join(corpus, "text.djvu"))
	if err != nil {
		t.Fatal(err)
	}
	z, err := d.Pages[0].Text()
	if err != nil {
		t.Fatal(err)
	}
	count := map[ZoneType]int{}
	var walk func(z *Zone)
	walk = func(z *Zone) {
		count[z.Type]++
		for _, k := range z.Children {
			if k.Type <= z.Type {
				t.Errorf("%s inside %s", k.Type, z.Type)
			}
			walk(k)
		}
	}
	walk(z)
	want := map[ZoneType]int{ZPage: 1, ZColumn: 2, ZRegion: 3, ZParagraph: 4, ZLine: 5, ZWord: 10, ZChar: 4}
	for typ, n := range want {
		if count[typ] != n {
			t.Errorf("expected %d zones of the type %s, got: %d", n, typ, count[typ])
		}
	}
	if z.X0 != 0 || z.Y0 != 0 || z.X1 != 2550 || z.Y1 != 3300 {
		t.Errorf("expected the page 0 0 2550 3300, got: %d %d %d %d", z.X0, z.Y0, z.X1, z.Y1)
	}
	line := z.Children[0].Children[0].Children[0].Children[1]
	if line.Type != ZLine || line.Text != "Größ Größe \n" {
		t.Errorf("expected the line %q, got: %s %q", "Größ Größe \n", line.Type, line.Text)
	}
	ch := line.Children[0].Children[2]
	if ch.Type != ZChar || ch.Text != "ö" || ch.X0 != 500 || ch.Y0 != 2900 || ch.X1 != 560 || ch.Y1 != 2960 {
		t.Errorf("expected the char ö at 500 2900 560 2960, got: %+v", *ch)
	}
	// The zones share the text of the page, and do not copy it.
	if unsafe.StringData(ch.Text) != unsafe.StringData(z.Text[strings.Index(z.Text, "ö"):]) {
		t.Error("expected the text of a zone inside the text of the page")
	}
}

// TestTextLimits checks that the limits of the document bound the text.
func TestTextLimits(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(corpus, "text.djvu"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		lim  ivy.Limits
	}{
		{"objects", ivy.Limits{MaxObjects: 10}},
		{"depth", ivy.Limits{MaxDepth: 3}},
		{"memory", ivy.Limits{MaxMemory: 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := ParseWithin(b, nil, tc.lim)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Pages[0].Text(); !errors.Is(err, ivy.ErrLimit) {
				t.Errorf("expected ErrLimit, got: %v", err)
			}
		})
	}
	// The zones of text.djvu nest 6 deep, below the page, and number 29.
	d, err := ParseWithin(b, nil, ivy.Limits{MaxObjects: 29, MaxDepth: 6})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pages[0].Text(); err != nil {
		t.Errorf("expected the text within the limits, got: %v", err)
	}
	// Zones that each take the whole text do not copy it. The first child
	// starts where the page starts. Each later child starts 1<<14 bytes
	// before the end of the one before it, which is where it starts.
	const n = 1000
	text := bytes.Repeat([]byte("x"), 1<<14)
	data := []byte{0, 0x40, 0}
	data = append(data, text...)
	data = append(data, 1)
	zone := func(typ ZoneType, start uint16, kids int) []byte {
		return []byte{byte(typ), 0x80, 0, 0x80, 0, 0x80, 10, 0x80, 10, byte(start >> 8), byte(start),
			0, 0x40, 0, byte(kids >> 16), byte(kids >> 8), byte(kids)}
	}
	data = append(data, zone(ZPage, 0x8000, n)...)
	data = append(data, zone(ZRegion, 0x8000, 0)...)
	for range n - 1 {
		data = append(data, zone(ZRegion, 0x8000-1<<14, 0)...)
	}
	z, err := ParseText(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range z.Children {
		if unsafe.StringData(k.Text) != unsafe.StringData(z.Text) {
			t.Fatal("expected the zones to share the text")
		}
	}
}

// FuzzParseText parses the text layers of the corpus, decoded, with
// changes.
func FuzzParseText(f *testing.F) {
	seeds(f, true, "TXTz")
	seeds(f, false, "TXTa")
	f.Add([]byte{0, 0, 2, 'h', 'i', 1,
		1, 0x80, 0, 0x80, 0, 0x80, 10, 0x80, 10, 0x80, 0, 0, 0, 2, 0, 0, 1,
		6, 0x80, 1, 0x80, 2, 0x80, 3, 0x80, 4, 0x80, 0, 0, 0, 2, 0, 0, 0})
	lim := ivy.Limits{MaxObjects: 1 << 12, MaxDepth: 16}
	f.Fuzz(func(t *testing.T, b []byte) {
		z, err := ParseTextWithin(b, lim)
		if err != nil || z == nil {
			return
		}
		n := 0
		var walk func(z *Zone)
		walk = func(z *Zone) {
			n++
			_ = z.Type.String()
			for _, k := range z.Children {
				walk(k)
			}
		}
		walk(z)
		if n > lim.MaxObjects {
			t.Errorf("expected at most %d zones, got: %d", lim.MaxObjects, n)
		}
	})
}

// FuzzPageText reads the text of each page of a whole document.
func FuzzPageText(f *testing.F) {
	for _, n := range []string{"text.djvu", "bundled.djvu", "pdf2djvu.djvu"} {
		b, err := os.ReadFile(filepath.Join(corpus, n))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	lim := ivy.Limits{MaxMemory: 1 << 20, MaxObjects: 1 << 12, MaxDepth: 16}
	f.Fuzz(func(t *testing.T, b []byte) {
		d, err := ParseWithin(b, nil, lim)
		if err != nil {
			return
		}
		for _, p := range d.Pages {
			if p.Form != nil {
				_, _ = p.Text()
			}
		}
	})
}
