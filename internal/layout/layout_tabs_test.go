// This file is not a port. It tests the tab stops of -ivy-tab-stops.

package layout

import (
	"log/slog"
	"testing"
)

// tabsStyle is the style of the tests of the tab stops: each glyph of
// weasyprint.otf is a square of 1em, so a letter is 10px wide.
const tabsStyle = `
      <style>
        @font-face { src: url(weasyprint.otf); font-family: weasyprint }
        @page { size: 1000px 200px; margin: 0 }
        body { margin: 0; font: 10px/20px weasyprint }
        p { margin: 0; white-space: pre-wrap }
      </style>`

// tabsLeaves returns the boxes of the text of a line, from left to right.
func tabsLeaves(line *Box) []*Box {
	var leaves []*Box
	var collect func(*Box)
	collect = func(b *Box) {
		for _, child := range b.Children {
			if child.Kind == KindText {
				leaves = append(leaves, child)
			} else {
				collect(child)
			}
		}
	}
	collect(line)
	return leaves
}

// checkTabs checks the texts of the leaves of a line and their positions.
func checkTabs(t *testing.T, line *Box, texts []string, xs []float64) {
	t.Helper()
	leaves := tabsLeaves(line)
	if len(leaves) != len(texts) {
		got := make([]string, 0, len(leaves))
		for _, leaf := range leaves {
			got = append(got, leaf.Text)
		}
		t.Fatalf("got the texts %q, want %q", got, texts)
	}
	for i, leaf := range leaves {
		inlineText(t, leaf, texts[i])
		assertApprox(t, "position_x of "+texts[i], leaf.PositionX, xs[i])
	}
}

func TestTabStopsAlign(t *testing.T) {
	body := inlineBody(t, tabsStyle+`
      <p style="-ivy-tab-stops: 300px center, 100px, 200px right">a`+"\t"+`b`+"\t"+`cc`+"\t"+`dddd</p>`)
	line := only(t, only(t, body))
	checkTabs(t, line,
		[]string{"a", "\t", "b", "\t", "cc", "\t", "dddd"},
		[]float64{0, 10, 100, 110, 180, 200, 280})
	assertApprox(t, "line.width", line.Width, 320)
}

func TestTabStopsAcrossBoxes(t *testing.T) {
	// The text after a tab that aligns right is in other boxes, with a
	// padding.
	body := inlineBody(t, tabsStyle+`
      <p style="-ivy-tab-stops: 200px right '.'"><span>ab`+"\t"+`</span><b>c<span style="padding-left: 5px">d</span></b></p>`)
	line := only(t, only(t, body))
	checkTabs(t, line,
		[]string{"ab", "\t", "c", "d"},
		[]float64{0, 20, 175, 190})
	leaves := tabsLeaves(line)
	assertApprox(t, "tab.width", leaves[1].Width, 155)
	span := line.Children[0]
	assertApprox(t, "span.width", span.Width, 175)
}

func TestTabStopsDecimal(t *testing.T) {
	body := inlineBody(t, tabsStyle+`
      <p style="-ivy-tab-stops: 100px decimal">`+"\t"+`12.5`+"\n\t"+`7</p>`)
	lines := unpack(t, only(t, body), 2)
	checkTabs(t, lines[0], []string{"\t", "12.5"}, []float64{0, 80})
	// Without a full stop, the text aligns right.
	checkTabs(t, lines[1], []string{"\t", "7"}, []float64{0, 90})
}

func TestTabStopsDefault(t *testing.T) {
	// After the last custom stop, the tabs go to the multiples of tab-size
	// from the start of the margin box, past the indent of the paragraph.
	body := inlineBody(t, tabsStyle+`
      <p style="margin-left: 30px; tab-size: 40px; -ivy-tab-stops: 15px">a`+"\t"+`b`+"\t"+`c</p>`)
	line := only(t, only(t, body))
	checkTabs(t, line, []string{"a", "\t", "b", "\t", "c"}, []float64{30, 40, 80, 90, 120})

	body = inlineBody(t, tabsStyle+`
      <p style="margin-left: 30px; tab-size: 40px; -ivy-tab-stops: auto">a`+"\t"+`b</p>`)
	line = only(t, only(t, body))
	checkTabs(t, line, []string{"a", "\t", "b"}, []float64{30, 40, 80})
}

func TestTabStopsNone(t *testing.T) {
	// Without tab stops, a tab stays in the text, as in upstream.
	body := inlineBody(t, tabsStyle+`
      <p style="tab-size: 40px">a`+"\t"+`b</p>`)
	line := only(t, only(t, body))
	checkTabs(t, line, []string{"a\tb"}, []float64{0})
	assertApprox(t, "line.width", line.Width, 50)
}

func TestTabStopsValidation(t *testing.T) {
	for _, tt := range []struct {
		css  string
		want string
	}{
		{"-ivy-tab-stops: none", `"none"`},
		{"-ivy-tab-stops: auto", `"auto"`},
		{"-ivy-tab-stops: 1pt right '.', 2px", `((Dimension(1.0, "pt"), "right", "."), (Dimension(2.0, "px"), "left", ""))`},
		{"-ivy-tab-stops: 1px center decimal", ""},
		{"-ivy-tab-stops: right", ""},
		{"-ivy-tab-stops: 1px 'a' 'b'", ""},
		{"-weasy-tab-stops: 1px", ""},
		{"tab-stops: 1px", ""},
	} {
		log, _ := captureLogs(slog.LevelDebug)
		decls := preprocessCSS(log, tt.css)
		got := ""
		if len(decls) == 1 {
			if decls[0].Name != "tab_stops" {
				t.Errorf("%s: got the name %q", tt.css, decls[0].Name)
			}
			got = ValueString(decls[0].Value)
		} else if len(decls) > 1 {
			t.Errorf("%s: got %d declarations", tt.css, len(decls))
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.css, got, tt.want)
		}
	}
}

func TestTabStopsLeader(t *testing.T) {
	// The leader fills the tab with as many copies as fit, which end at
	// the end of the tab.
	assertPixels(t, "tab_stops_leader", `
        RR_RRRRRR_
        RR_RRRRRR_
    `, `
      <style>
        @font-face { src: url(weasyprint.otf); font-family: weasyprint }
        @page { size: 10px 2px }
        body { margin: 0; color: red; font: 2px/2px weasyprint }
        p { margin: 0; white-space: pre-wrap; -ivy-tab-stops: 9px right 'a' }
      </style>
      <p>a`+"\t"+`b</p>`)
}
