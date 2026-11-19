package msdoc

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
)

// TestLineNumbers checks that a section gives the numbers of its lines to
// the layout: the step, the distance, the first number and the restart,
// and that a paragraph can leave its lines out
// (libreoffice/doc/line-numbers.doc).
func TestLineNumbers(t *testing.T) {
	op := func(op uint16, operand ...byte) []byte {
		return append(u16s(op), operand...)
	}
	tests := []struct {
		name   string
		grpprl []byte
		want   string
	}{
		{"none", nil, ""},
		// The distance 0 is 0.25 inch, and the numbers restart on each
		// page by default.
		{"default", op(sprmSNLnnMod, 1, 0), "18pt 1 1 page"},
		{"all", bytes.Join([][]byte{
			op(sprmSNLnnMod, 5, 0), op(sprmSDxaLnn, 0xA0, 0x05), op(sprmSLnnMin, 9, 0), op(sprmSLnc, lncRestart),
		}, nil), "72pt 5 10 section"},
		{"continue", append(op(sprmSNLnnMod, 1, 0), op(sprmSLnc, lncContinue)...), "18pt 1 1 continuous"},
		// Word takes at most 100 for the step.
		{"step", op(sprmSNLnnMod, 101, 0), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// a PlcfSed of one section, whose Sepx is at 100 in the stream
			// WordDocument
			plc := slices.Concat(u32s(0, 10), u16s(0), u32s(100), u16s(0), u32s(0))
			p := testParser(plc, nil)
			p.s.fib.fcLcb[fcPlcfSed] = [2]uint32{0, uint32(len(plc))}
			p.s.word = slices.Concat(make([]byte, 100), u16s(uint16(len(tt.grpprl))), tt.grpprl)
			if got := p.sectionPage(0, ivy.Paper{}).lines.css(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
	var pp pap
	applyPap(&pp, sprm{op: sprmPFNoLineNumb, operand: []byte{1}})
	p := &parser{styles: parseStyles(nil, 0)}
	var b strings.Builder
	p.writeParagraph(&b, paragraph{pap: pp, runs: []run{{text: "x", chp: defaultChp()}}})
	if got := b.String(); !strings.Contains(got, "-ivy-line-numbers: none") {
		t.Errorf("expected a paragraph without numbers, got: %s", got)
	}
}

// TestLineMultiple checks that the lines of a multiple spacing put their
// extra space below their text, and that the pages leave out the space
// before a paragraph at their top (libreoffice/doc/line-numbers.doc).
func TestLineMultiple(t *testing.T) {
	var pp pap
	applyPap(&pp, sprm{op: sprmPDyaLine, operand: u16s(360, 1)})
	p := &parser{styles: parseStyles(nil, 0)}
	var b strings.Builder
	p.writeParagraph(&b, paragraph{pap: pp, runs: []run{{text: "x", chp: defaultChp()}}})
	if got := b.String(); !strings.Contains(got, "-ivy-line-multiple: 1.500") {
		t.Errorf("expected the multiple 1.5, got: %s", got)
	}
	if css := testParser(nil, nil).css(letter); !strings.Contains(css, "-ivy-page-top-space: discard") {
		t.Errorf("expected the space before a paragraph discarded at the top of a page, got: %s", css)
	}
}
