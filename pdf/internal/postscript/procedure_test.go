// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-postscript src/procedure.rs.

package postscript

import (
	"errors"
	"strings"
	"testing"
)

// readProcedure is parse_procedure in the tests of hayro-postscript
// src/procedure.rs.
func readProcedure(input string) ([]byte, error) {
	return parseProcedure(newReader([]byte(input)))
}

// wantProcedure compares the procedure of input with want.
func wantProcedure(t *testing.T, input, want string) {
	t.Helper()
	got, err := readProcedure(input)
	if err != nil || string(got) != want {
		t.Fatalf("readProcedure(%q) = %q, %v, want %q", input, got, err, want)
	}
}

func TestProcedureEmpty(t *testing.T) {
	wantProcedure(t, "{}", "")
}

func TestProcedureSimple(t *testing.T) {
	wantProcedure(t, "{add 2 div}", "add 2 div")
}

func TestProcedureNested(t *testing.T) {
	wantProcedure(t, "{dup 0 lt {neg} if}", "dup 0 lt {neg} if")
}

func TestProcedureUnbalancedBrackets(t *testing.T) {
	wantProcedure(t, "{[ 1 2}", "[ 1 2")
	wantProcedure(t, "{] 1}", "] 1")
}

func TestProcedureWithStrings(t *testing.T) {
	wantProcedure(t, "{(}) <7D> <~Ac~>}", "(}) <7D> <~Ac~>")
}

func TestProcedureWithComment(t *testing.T) {
	wantProcedure(t, "{1 % comment with }\n2}", "1 % comment with }\n2")
}

func TestProcedureUnterminated(t *testing.T) {
	if _, err := readProcedure("{1 {2}"); !errors.Is(err, ErrSyntax) {
		t.Fatalf("err = %v, want ErrSyntax", err)
	}
}

func TestProcedureDeeplyNestedUnterminated(t *testing.T) {
	if _, err := readProcedure(strings.Repeat("{", 1_000_000)); !errors.Is(err, ErrSyntax) {
		t.Fatalf("err = %v, want ErrSyntax", err)
	}
}
