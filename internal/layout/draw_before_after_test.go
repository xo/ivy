// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_before_after.py: the drawing of the ::before and ::after pseudo-elements.

package layout

import "testing"

// TestDrawBeforeAfter1 is test_before_after_1 in tests/draw/test_before_after.py.
func TestDrawBeforeAfter1(t *testing.T) {
	assertSameRenderings(t, "before_after_1", 10,
		`
            <style>
                @page { size: 300px 30px }
                body { margin: 0 }
                a[href]:before { content: '[' attr(href) '] ' }
            </style>
            <p><a href="some url">some content</a></p>
        `,
		`
            <style>
                @page { size: 300px 30PX }
                body { margin: 0 }
            </style>
            <p><a href="another url"><span>[some url] </span>some content</p>
        `)
}

// TestDrawBeforeAfter2 is test_before_after_2 in tests/draw/test_before_after.py.
func TestDrawBeforeAfter2(t *testing.T) {
	assertSameRenderings(t, "before_after_2", 10,
		`
            <style>
                @page { size: 500px 30px }
                body { margin: 0; quotes: '«' '»' '“' '”' }
                q:before { content: open-quote ' '}
                q:after { content: ' ' close-quote }
            </style>
            <p><q>Lorem ipsum <q>dolor</q> sit amet</q></p>
        `,
		`
            <style>
                @page { size: 500px 30Px }
                body { margin: 0 }
                q:before, q:after { content: none }
            </style>
            <p><span><span>« </span>Lorem ipsum
                <span><span>“ </span>dolor<span> ”</span></span>
                sit amet<span> »</span></span></p>
        `)
}

// TestDrawBeforeAfter3 is test_before_after_3 in tests/draw/test_before_after.py.
func TestDrawBeforeAfter3(t *testing.T) {
	assertSameRenderings(t, "before_after_3", 10,
		`
            <style>
                @page { size: 100px 30px }
                body { margin: 0; }
                p:before { content: 'a' url(pattern.png) 'b'}
            </style>
            <p>c</p>
        `,
		`
            <style>
                @page { size: 100PX 30px }
                body { margin: 0 }
            </style>
            <p><span>a<img src="pattern.png" alt="Missing image">b</span>c</p>
        `)
}
