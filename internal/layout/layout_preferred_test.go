// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/layout/test_preferred.py: the tests of the
// shrink-to-fit algorithm.

package layout

import (
	"fmt"
	"strings"
	"testing"
)

func TestShrinkToFitFloatingPointError1(t *testing.T) {
	for marginLeft := 1; marginLeft < 10; marginLeft++ {
		for fontSize := 1; fontSize < 10; fontSize++ {
			t.Run(fmt.Sprintf("%d-%d", marginLeft, fontSize), func(t *testing.T) {
				// See bugs #325 and #288, see commit fac5ee9.
				page := tablePage(t, fmt.Sprintf(`
      <style>
        @page { size: 100000px 100px }
        p { float: left; margin-left: 0.%din; font-size: 0.%dem;
            font-family: weasyprint }
      </style>
      <p>this parrot is dead</p>
    `, marginLeft, fontSize))
				_, body := pageBody(t, page)
				p := only(t, body)
				unpack(t, p, 1)
			})
		}
	}
}

func TestShrinkToFitFloatingPointError2(t *testing.T) {
	for _, fontSize := range []int{1, 5, 10, 50, 100, 1000, 10000} {
		t.Run(fmt.Sprint(fontSize), func(t *testing.T) {
			letters := 1
			for {
				page := tablePage(t, fmt.Sprintf(`
          <style>
            @page { size: %d0pt %d0px }
            p { font-size: %dpt; font-family: weasyprint }
          </style>
          <p>mmm <b>%s a</b></p>
        `, fontSize, fontSize, fontSize, strings.Repeat("i", letters)))
				_, body := pageBody(t, page)
				p := only(t, body)
				if n := len(p.Children); n != 1 && n != 2 {
					t.Fatalf("p holds %d lines, want 1 or 2", n)
				}
				line := unpack(t, p.Children[0], 2)
				text := only(t, line[1]).Text
				if text == "" {
					t.Fatal("the text of the first line is empty")
				}
				if strings.HasSuffix(text, "i") {
					break
				}
				letters++
				if letters > 1000 {
					// Port: upstream loops until the text ends with i.
					t.Fatal("the text never ends with i")
				}
			}
		})
	}
}

// preferredDiv lays out a document of one page whose body holds a div,
// and returns the div.
func preferredDiv(t *testing.T, source string) *Box {
	t.Helper()
	_, body := pageBody(t, tablePage(t, source))
	return only(t, body)
}

func TestPreferredInlineZeroWidthInlineBlock(t *testing.T) {
	div := preferredDiv(t, `
      <style>
        div { font: 2px weasyprint; float: left }
      </style>
      <div><span style="display: inline-block"></span> a</div>
    `)
	tableEq(t, "div.width", div.Width, 4)
	tableEq(t, "div.height", div.Height, 2)
}

func TestPreferredInlineNestedTrailingSpaces(t *testing.T) {
	div := preferredDiv(t, `
      <style>
        div { font: 2px weasyprint; float: left }
      </style>
      <div><span>a </span> </div>
    `)
	tableEq(t, "div.width", div.Width, 2)
	tableEq(t, "div.height", div.Height, 2)
}

func TestPreferredInlineTrailingSpaceInNested(t *testing.T) {
	div := preferredDiv(t, `
      <style>
        div { font: 2px weasyprint; float: left }
      </style>
      <div><span style="display: inline-block"></span><span><span>a</span> </span></div>
    `)
	tableEq(t, "div.width", div.Width, 2)
	tableEq(t, "div.height", div.Height, 2)
}
