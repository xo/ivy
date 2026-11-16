// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/djvu/text_test.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.

package doc // ivy: renamed from djvu, the name of the package of the format

import "testing"

// ivy: the copy keeps only TestParseSexps of this file. The other tests
// read the text layer and the outline, which the copy leaves out, or run
// djvused of DjVuLibre, and only the test module of ivy runs an outside
// program.

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
