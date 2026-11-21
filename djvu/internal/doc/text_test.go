// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/text_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ivy: the copy leaves out outlineSexp, TestParseOutlineErrors and the
// parts of TestTextAnnoOutlineCorpus for the outline and the annotations,
// because it leaves out the outline. It leaves out TestBooks, which runs
// djvused of DjVuLibre, because only the test module of ivy runs an
// outside program. TestTextBooks in textlayer_test.go takes its place.

// dsed holds the set-ant and set-txt blocks of `djvused -e output-all`
// by component id.
type dsed map[string]map[string]string

func readDsed(b []byte) dsed {
	r := dsed{}
	id, cmd := "", ""
	var buf strings.Builder
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(nil, 1<<24)
	for sc.Scan() {
		l := sc.Text()
		switch {
		case cmd != "" && l == ".":
			if r[id] == nil {
				r[id] = map[string]string{}
			}
			r[id][cmd] = buf.String()
			cmd = ""
			buf.Reset()
		case cmd != "":
			buf.WriteString(l + "\n")
		case strings.HasPrefix(l, `select "`):
			// The ID is a quoted string with octal escapes for non-ASCII.
			q := l[7 : strings.LastIndexByte(l, '"')+1]
			id = ParseSexps([]byte("(s " + q + ")"))[0].List[1].Atom
		case l == "set-ant" || l == "set-txt":
			cmd = l
		}
	}
	return r
}

func list(head string, items ...*Sexp) *Sexp {
	return &Sexp{IsList: true, List: append([]*Sexp{{Atom: head}}, items...)}
}

func str(s string) *Sexp { return &Sexp{Atom: s, Quoted: true} }

// zoneSexp prints a zone as djvused print-txt does: text on leaves only,
// without separators.
func zoneSexp(z *Zone) *Sexp {
	s := list(z.Type.String())
	for _, v := range []int{z.X0, z.Y0, z.X1, z.Y1} {
		s.List = append(s.List, &Sexp{Atom: strconv.Itoa(v)})
	}
	if len(z.Children) == 0 {
		// djvused drops trailing separators of the zone's own level only.
		sep := map[ZoneType]string{ZPage: "\f", ZColumn: "\v", ZRegion: "\x1d",
			ZParagraph: "\x1f", ZLine: "\n", ZWord: " "}[z.Type]
		s.List = append(s.List, str(strings.TrimRight(z.Text, sep)))
	}
	for _, k := range z.Children {
		s.List = append(s.List, zoneSexp(k))
	}
	return s
}

// TestTextCorpus compares the text of each component of the files that
// have a .dsed golden with the set-txt blocks of djvused output-all.
//
// ivy: it is the part for the text of TestTextAnnoOutlineCorpus.
func TestTextCorpus(t *testing.T) {
	goldens, _ := filepath.Glob(filepath.Join(corpus, "*.dsed"))
	if len(goldens) == 0 {
		t.Fatal("no goldens")
	}
	for _, g := range goldens {
		base := strings.TrimSuffix(g, ".dsed")
		b, err := os.ReadFile(g) // ivy: upstream calls mustRead
		if err != nil {
			t.Fatal(err)
		}
		if nt := checkText(t, base+".djvu", b); nt == 0 {
			t.Errorf("%s: no text", g)
		}
	}
}

// checkText compares the text of a single or bundled document with djvused
// output-all output. It returns the number of text layers seen.
//
// ivy: it is the part for the text of checkDoc.
func checkText(t *testing.T, path string, dsed []byte) (nt int) {
	t.Helper()
	want := readDsed(dsed)
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// ivy: a single page document has no directory. djvused names its
	// page after the file.
	comps := []Component{{ID: filepath.Base(path)}}
	if d.Dir != nil {
		comps = d.Dir.Components
	}
	for _, c := range comps {
		f := d.Root
		if d.Dir != nil {
			if f, err = d.component(c); err != nil {
				t.Fatal(err)
			}
		}
		z, err := textOf(d.lim, f) // ivy: within the limits of the document
		if err != nil {
			t.Fatalf("%s %s: %v", path, c.ID, err)
		}
		got, wt := "", ""
		if z != nil {
			got = zoneSexp(z).String()
			nt++
		}
		if e := ParseSexps([]byte(want[c.ID]["set-txt"])); len(e) > 0 {
			wt = e[0].String()
		}
		if got != wt {
			t.Errorf("%s %s text:\n got %s\nwant %s", path, c.ID, got, wt)
		}
	}
	return nt
}

func TestParseSexps(t *testing.T) {
	in := `(maparea "#3" "a \"b\"\n\101\\" (rect 1 2 3 4) (none)) ) (zoom page) (broken "x"`
	e := ParseSexps([]byte(in))
	if len(e) != 2 || e[0].Head() != "maparea" || e[1].String() != "(zoom page)" {
		t.Fatalf("got %v", e)
	}
	if s := e[0].List[2].Atom; s != "a \"b\"\nA\\" {
		t.Errorf("string %q", s)
	}
	if s := e[0].String(); s != `(maparea "#3" "a \"b\"\nA\\" (rect 1 2 3 4) (none))` {
		t.Errorf("format %s", s)
	}
}

func TestParseTextErrors(t *testing.T) {
	ok := []byte{0, 0, 2, 'h', 'i', 1,
		1, 0x80, 0, 0x80, 0, 0x80, 10, 0x80, 10, 0x80, 0, 0, 0, 2, 0, 0, 1,
		6, 0x80, 1, 0x80, 2, 0x80, 3, 0x80, 4, 0x80, 0, 0, 0, 2, 0, 0, 0}
	z, err := ParseText(ok)
	if err != nil {
		t.Fatal(err)
	}
	w := z.Children[0]
	if w.Type != ZWord || w.Text != "hi" || w.X0 != 1 || w.Y0 != 10-2-4 || w.X1 != 4 || w.Y1 != 8 {
		t.Errorf("word %+v", *w)
	}
	for _, n := range []int{2, 5, 10, len(ok) - 1} {
		if _, err := ParseText(ok[:n]); err == nil {
			t.Errorf("truncated at %d accepted", n)
		}
	}
	bad := bytes.Clone(ok)
	bad[len(bad)-4] = 3 // word text longer than the page text
	if _, err := ParseText(bad); err == nil {
		t.Error("text overflow accepted")
	}
}
