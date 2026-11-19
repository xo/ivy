package xlsx

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/xo/ivy"
)

// TestRowHeights compares the heights of the rows of the files
// testdata/row-heights-*.xlsx with the heights that Excel 2024 printed to
// PDF, in dots of 600 dots per inch. Each file has rows of 6 to 100 points
// with customHeight, three rows without ht, two rows of 30 points without
// customHeight, and a row of 12 points without customHeight that holds two
// lines of text that wraps. Excel fits the last six rows to their cells.
// P and S are in the table measuredRows of internal/sheet, so the test does
// not depend on the fonts of the program.
func TestRowHeights(t *testing.T) {
	tests := []struct {
		font string
		dots []int
	}{
		{"arial-10", []int{48, 72, 97, 109, 121, 145, 169, 194, 242, 290, 387, 581, 807, 103, 103, 103, 103, 103, 206}},
		{"arial-12", []int{49, 73, 98, 110, 123, 147, 172, 196, 246, 295, 393, 590, 819, 123, 123, 123, 123, 123, 246}},
		{"calibri-8", []int{48, 72, 96, 108, 120, 144, 168, 192, 240, 288, 384, 576, 799, 90, 90, 90, 90, 90, 180}},
		{"calibri-11", []int{48, 72, 96, 109, 121, 145, 169, 193, 242, 290, 387, 580, 806, 121, 121, 121, 121, 121, 242}},
		{"calibri-16", []int{48, 73, 97, 110, 122, 146, 171, 195, 244, 293, 390, 586, 813, 171, 171, 171, 171, 171, 342}},
		{"century-gothic-9", []int{41, 62, 83, 93, 104, 125, 146, 166, 208, 250, 333, 500, 694, 99, 99, 99, 99, 99, 198}},
		{"century-gothic-11", []int{43, 65, 86, 97, 108, 129, 151, 173, 216, 259, 346, 519, 720, 119, 119, 119, 119, 119, 238}},
		{"courier-new-10", []int{42, 64, 85, 96, 106, 128, 149, 170, 213, 256, 341, 512, 710, 96, 96, 96, 96, 96, 192}},
		{"courier-new-12", []int{43, 65, 87, 98, 109, 131, 153, 175, 219, 262, 350, 525, 729, 115, 115, 115, 115, 115, 230}},
		{"ms-pgothic-11", []int{45, 68, 90, 102, 113, 136, 158, 181, 226, 272, 362, 544, 755, 102, 102, 102, 102, 102, 204}},
		{"times-new-roman-12", []int{48, 72, 96, 108, 120, 144, 168, 192, 240, 288, 384, 576, 799, 126, 126, 126, 126, 126, 252}},
		{"verdana-10", []int{50, 76, 101, 114, 127, 152, 178, 203, 254, 305, 406, 610, 846, 108, 108, 108, 108, 108, 216}},
	}
	def := regexp.MustCompile(`tr\{break-inside:avoid;height:([0-9.]+)pt\}`)
	row := regexp.MustCompile(`<tr(?: class="pb")?(?: style="height:([0-9.]+)pt")?>`)
	for _, test := range tests {
		t.Run(test.font, func(t *testing.T) {
			buf, err := os.ReadFile("testdata/row-heights-" + test.font + ".xlsx")
			if err != nil {
				t.Fatal(err)
			}
			_, docs := sourceOf(t, open(t, buf, ivy.Config{}))
			m := def.FindStringSubmatch(docs[0])
			if m == nil {
				t.Fatalf("no default height in the sheet:\n%s", docs[0])
			}
			rows := row.FindAllStringSubmatch(docs[0], -1)
			if len(rows) != len(test.dots) {
				t.Fatalf("expected %d rows, got: %d", len(test.dots), len(rows))
			}
			for i, r := range rows {
				v := m[1]
				if r[1] != "" {
					v = r[1]
				}
				pt, err := strconv.ParseFloat(v, 64)
				if err != nil {
					t.Fatal(err)
				}
				// the PDF files of Excel put 17 of the 156 rows of set
				// heights 1 dot away from trunc(h × P / S), in both
				// directions, and no other rule of rounding fits better
				got, want := int(math.Round(pt*600/72)), test.dots[i]
				if i < 13 && abs(got-want) > 1 || i >= 13 && got != want {
					t.Errorf("row %d: expected %d dots, got: %d", i+1, want, got)
				}
			}
		})
	}
}

// abs returns the absolute value of v.
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
