// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_validation.py.

package layout

import (
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"
	"testing"
)

// getValue is get_value in tests/css/test_validation.py. It returns the
// value of the only declaration of css, or nil. With expectedError, the log
// must hold one message that contains it. Else the log must be empty.
func getValue(t *testing.T, css, expectedError string, level slog.Level) Value {
	t.Helper()
	log, capture := captureLogs(level)
	declarations := preprocessCSS(log, css)
	if expectedError != "" {
		assertLog(t, capture, expectedError)
	} else {
		assertNoLogs(t, capture)
	}
	if len(declarations) == 0 {
		return nil
	}
	if len(declarations) != 1 {
		t.Fatalf("%s: expected 1 declaration, got: %d", css, len(declarations))
	}
	return declarations[0].Value
}

// assertInvalid is assert_invalid in tests/css/test_validation.py.
func assertInvalid(t *testing.T, css, message string, level slog.Level) {
	t.Helper()
	if v := getValue(t, css, message, level); v != nil {
		t.Errorf("%s: expected an invalid value, got: %s", css, ValueString(v))
	}
}

// valueCase is a case of a parametrized test of values: a rule and the
// value of its declaration.
type valueCase struct {
	rule string
	want Value
}

// checkValues checks the value of the declaration that format makes of
// each rule.
func checkValues(t *testing.T, format string, cases []valueCase) {
	t.Helper()
	for _, c := range cases {
		css := fmt.Sprintf(format, c.rule)
		if got := getValue(t, css, "", slog.LevelInfo); !Equal(got, c.want) {
			t.Errorf("%s: expected %s, got: %s", css, ValueString(c.want), ValueString(got))
		}
	}
}

// checkInvalid checks that the declaration that format makes of each rule
// is invalid, with a log that holds "invalid".
func checkInvalid(t *testing.T, format string, rules []string) {
	t.Helper()
	for _, rule := range rules {
		assertInvalid(t, fmt.Sprintf(format, rule), "invalid", slog.LevelInfo)
	}
}

// pi is math.Pi as a variable, so that pi/6 rounds as the float division
// of Python does, and not as a constant expression.
var pi = math.Pi

// The short constructors of the expected values.
func dim(v float64, unit string) Dimension { return Dimension{v, unit} }
func tup(items ...Value) Tuple             { return Tuple(items) }
func strs(items ...string) Tuple           { return tupleOf(items...) }

func TestNotPrint(t *testing.T) {
	assertInvalid(t, "volume: 42", "does not apply for the print media", slog.LevelDebug)
}

func TestUnstablePrefix(t *testing.T) {
	got := getValue(t, "-weasy-max-lines: 3", "prefixes on unstable attributes are deprecated", slog.LevelInfo)
	if !Equal(got, Int(3)) {
		t.Errorf("expected 3, got: %s", ValueString(got))
	}
}

func TestNormalPrefix(t *testing.T) {
	assertInvalid(t, "-weasy-display: block", "prefix on this attribute is not supported", slog.LevelInfo)
}

func TestUnknownPrefix(t *testing.T) {
	assertInvalid(t, "-unknown-display: block", "prefixed selectors are ignored", slog.LevelDebug)
}

func TestEmptyPropertyValue(t *testing.T) {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		assertInvalid(t, name+":", "Ignored", slog.LevelInfo)
	}
}

func TestClip(t *testing.T) {
	checkValues(t, "clip: rect(%s)", []valueCase{
		{"1px, 3EM, auto, auto", tup(dim(1, "px"), dim(3, "em"), String("auto"), String("auto"))},
		{"1px,3em,auto,1px", tup(dim(1, "px"), dim(3, "em"), String("auto"), dim(1, "px"))},
	})
}

func TestClipInvalid(t *testing.T) {
	checkInvalid(t, "%s", []string{
		"clip: square(1px, 3em, auto, auto)",
		"clip: rect(1px 3em auto auto)",
		"clip: rect(1px, 3em, auto)",
		"clip: rect(1px, 3em / auto)",
	})
}

func TestCounters(t *testing.T) {
	checkValues(t, "%s", []valueCase{
		{"counter-reset: foo bar 2 baz", tup(tup(String("foo"), Int(0)), tup(String("bar"), Int(2)), tup(String("baz"), Int(0)))},
		{"counter-increment: foo bar 2 baz", tup(tup(String("foo"), Int(1)), tup(String("bar"), Int(2)), tup(String("baz"), Int(1)))},
		{"counter-reset: foo", tup(tup(String("foo"), Int(0)))},
		{"counter-set: FoO", tup(tup(String("FoO"), Int(0)))},
		{"counter-increment: foo bAr 2 Bar", tup(tup(String("foo"), Int(1)), tup(String("bAr"), Int(2)), tup(String("Bar"), Int(1)))},
		{"counter-reset: none", tup()},
	})
}

func TestCountersWarning(t *testing.T) {
	assertInvalid(t, "counter-reset: foo initial", "Invalid counter name: initial.", slog.LevelInfo)
	assertInvalid(t, "counter-reset: foo none", "Invalid counter name: none.", slog.LevelInfo)
}

func TestCountersInvalid(t *testing.T) {
	checkInvalid(t, "%s", []string{"counter-reset: foo 3pX", "counter-reset: 3"})
}

func TestSpacing(t *testing.T) {
	checkValues(t, "%s", []valueCase{
		{"letter-spacing: normal", String("normal")},
		{"letter-spacing: 3px", dim(3, "px")},
		{"word-spacing: normal", String("normal")},
		{"word-spacing: 3px", dim(3, "px")},
	})
}

func TestSpacingInvalid(t *testing.T) {
	checkInvalid(t, "%s", []string{"letter-spacing: 3", "word-spacing: 3"})
}

func TestTextDecorationLine(t *testing.T) {
	checkValues(t, "text-decoration-line: %s", []valueCase{
		{"none", String("none")},
		{"overline", newStringSet("overline")},
		{"overline blink line-through", newStringSet("blink", "line-through", "overline")},
	})
}

func TestTextDecorationStyle(t *testing.T) {
	checkValues(t, "text-decoration-style: %s", []valueCase{
		{"solid", String("solid")},
		{"double", String("double")},
		{"dotted", String("dotted")},
		{"dashed", String("dashed")},
	})
}

func TestSize(t *testing.T) {
	checkValues(t, "size: %s", []valueCase{
		{"200px", tup(dim(200, "px"), dim(200, "px"))},
		{"200px 300pt", tup(dim(200, "px"), dim(300, "pt"))},
		{"auto", tup(dim(210, "mm"), dim(297, "mm"))},
		{"portrait", tup(dim(210, "mm"), dim(297, "mm"))},
		{"landscape", tup(dim(297, "mm"), dim(210, "mm"))},
		{"A3 portrait", tup(dim(297, "mm"), dim(420, "mm"))},
		{"A3 landscape", tup(dim(420, "mm"), dim(297, "mm"))},
		{"portrait A3", tup(dim(297, "mm"), dim(420, "mm"))},
		{"landscape A3", tup(dim(420, "mm"), dim(297, "mm"))},
	})
}

func TestSizeInvalid(t *testing.T) {
	checkInvalid(t, "size: %s", []string{"A3 landscape A3", "A12", "foo", "foo bar", "20%"})
}

func TestTransform(t *testing.T) {
	checkValues(t, "transform: %s", []valueCase{
		{"none", tup()},
		{"translate(6px) rotate(90deg)", tup(
			tup(String("translate"), tup(dim(6, "px"), dim(0, "px"))),
			tup(String("rotate"), Float(pi/2)))},
		{"rotate(0)", tup(tup(String("rotate"), Int(0)))},
		{"translate(-4px, 0)", tup(tup(String("translate"), tup(dim(-4, "px"), dim(0, ""))))},
		{"translate(6px, 20%)", tup(tup(String("translate"), tup(dim(6, "px"), dim(20, "%"))))},
		{"scale(2)", tup(tup(String("scale"), tup(Int(2), Int(2))))},
	})
}

func TestTransformInvalid(t *testing.T) {
	checkInvalid(t, "transform: %s", []string{
		"lipsumize(6px)", "foo", "scale(2) foo", "6px", "translate(6px 20%)",
	})
}

func TestBackgroundImageInvalid(t *testing.T) {
	checkInvalid(t, "background-image: %s", []string{"inexistent-gradient(blue, green)"})
}

func TestBackgroundPosition(t *testing.T) {
	position := func(originX string, x Dimension, originY string, y Dimension) Value {
		return tup(tup(String(originX), x, String(originY), y))
	}
	cases := make([]valueCase, 0, 64)
	cases = append(cases, []valueCase{
		// One token, vertical.
		{"top", position("left", dim(50, "%"), "top", dim(0, "%"))},
		{"bottom", position("left", dim(50, "%"), "top", dim(100, "%"))},

		// Three tokens.
		{"center top 10%", position("left", dim(50, "%"), "top", dim(10, "%"))},
		{"top 10% center", position("left", dim(50, "%"), "top", dim(10, "%"))},
		{"center bottom 10%", position("left", dim(50, "%"), "bottom", dim(10, "%"))},
		{"bottom 10% center", position("left", dim(50, "%"), "bottom", dim(10, "%"))},

		{"right top 10%", position("right", dim(0, "%"), "top", dim(10, "%"))},
		{"top 10% right", position("right", dim(0, "%"), "top", dim(10, "%"))},
		{"right bottom 10%", position("right", dim(0, "%"), "bottom", dim(10, "%"))},
		{"bottom 10% right", position("right", dim(0, "%"), "bottom", dim(10, "%"))},

		{"center left 10%", position("left", dim(10, "%"), "top", dim(50, "%"))},
		{"left 10% center", position("left", dim(10, "%"), "top", dim(50, "%"))},
		{"center right 10%", position("right", dim(10, "%"), "top", dim(50, "%"))},
		{"right 10% center", position("right", dim(10, "%"), "top", dim(50, "%"))},

		{"bottom left 10%", position("left", dim(10, "%"), "bottom", dim(0, "%"))},
		{"left 10% bottom", position("left", dim(10, "%"), "bottom", dim(0, "%"))},
		{"bottom right 10%", position("right", dim(10, "%"), "bottom", dim(0, "%"))},
		{"right 10% bottom", position("right", dim(10, "%"), "bottom", dim(0, "%"))},

		// Four tokens.
		{"left 10% bottom 3PX", position("left", dim(10, "%"), "bottom", dim(3, "px"))},
		{"bottom 3px left 10%", position("left", dim(10, "%"), "bottom", dim(3, "px"))},
		{"right 10% top 3px", position("right", dim(10, "%"), "top", dim(3, "px"))},
		{"top 3px right 10%", position("right", dim(10, "%"), "top", dim(3, "px"))},
	}...)
	xs := []struct {
		css string
		val Dimension
	}{
		{"left", dim(0, "%")}, {"center", dim(50, "%")}, {"right", dim(100, "%")},
		{"4.5%", dim(4.5, "%")}, {"12px", dim(12, "px")},
	}
	ys := []struct {
		css string
		val Dimension
	}{
		{"top", dim(0, "%")}, {"center", dim(50, "%")}, {"bottom", dim(100, "%")},
		{"7%", dim(7, "%")}, {"1.5px", dim(1.5, "px")},
	}
	for _, x := range xs {
		cases = append(cases, valueCase{x.css, position("left", x.val, "top", dim(50, "%"))})
	}
	for _, x := range xs {
		for _, y := range ys {
			cases = append(cases, valueCase{x.css + " " + y.css, position("left", x.val, "top", y.val)})
		}
	}
	checkValues(t, "background-position: %s", cases)
}

func TestBackgroundPositionInvalid(t *testing.T) {
	checkInvalid(t, "background-position: %s", []string{
		"10px lipsum", "left center 3px", "3px left", "bottom 4%", "bottom top",
	})
}

func TestFontFamilyInvalid(t *testing.T) {
	checkInvalid(t, "font-family: %s", []string{
		`"My" Font, serif`, `"My" "Font", serif`, `"My", 12pt, serif`,
	})
}

func TestLineHeight(t *testing.T) {
	checkValues(t, "line-height: %s", []valueCase{
		{"1px", dim(1, "px")},
		{"1.1%", dim(1.1, "%")},
		{"1Em", dim(1, "em")},
		{"1", dim(1, "")},
		{"1.3", dim(1.3, "")},
		{"-0", dim(0, "")},
		{"0px", dim(0, "px")},
	})
}

func TestLineHeightInvalid(t *testing.T) {
	checkInvalid(t, "line-height: %s", []string{"1deg", "-1px", "-1", "-0.5%", "1px 1px"})
}

func TestListStyleTypeInvalid(t *testing.T) {
	checkInvalid(t, "list-style-type: %s", []string{
		"symbols()", "symbols(cyclic)", "symbols(symbolic)", "symbols(fixed)",
		`symbols(alphabetic "a")`, `symbols(numeric "1")`, `symbols(test "a" "b")`,
		`symbols(fixed symbolic "a" "b")`,
	})
}

func TestImageOrientation(t *testing.T) {
	checkValues(t, "image-orientation: %s", []valueCase{
		{"none", String("none")},
		{"from-image", String("from-image")},
		{"90deg", tup(Float(pi/2), Bool(false))},
		{"30deg", tup(Float(pi/6), Bool(false))},
		{"180deg flip", tup(Float(pi), Bool(true))},
		{"0deg flip", tup(Int(0), Bool(true))},
		{"flip 90deg", tup(Float(pi/2), Bool(true))},
		{"flip", tup(Int(0), Bool(true))},
	})
}

func TestImageOrientationInvalid(t *testing.T) {
	checkInvalid(t, "image-orientation: %s", []string{
		"none none", "unknown", "none flip", "from-image flip", "10", "10 flip",
		"flip 10", "flip flip", "90deg flop", "90deg 180deg",
	})
}

// borderSliceCases are the cases of test_border_image_slice and
// test_mask_border_slice.
var borderSliceCases = []valueCase{
	{"1", tup(dim(1, ""))},
	{"1 2    3 4", tup(dim(1, ""), dim(2, ""), dim(3, ""), dim(4, ""))},
	{"50% 1000.1 0", tup(dim(50, "%"), dim(1000.1, ""), dim(0, ""))},
	{"1% 2% 3% 4%", tup(dim(1, "%"), dim(2, "%"), dim(3, "%"), dim(4, "%"))},
	{"fill 10% 20", tup(String("fill"), dim(10, "%"), dim(20, ""))},
	{"0 1 0.5 fill", tup(dim(0, ""), dim(1, ""), dim(0.5, ""), String("fill"))},
}

// borderSliceInvalid are the rules of test_border_image_slice_invalid and
// test_mask_border_slice_invalid.
var borderSliceInvalid = []string{
	"none", "1, 2", "-10", "-10%", "1 2 3 -10%", "-0.3", "1 fill 2", "fill 1 2 3 fill",
	"scale(calc(0))",
}

func TestBorderImageSlice(t *testing.T) {
	checkValues(t, "border-image-slice: %s", append(slices.Clone(borderSliceCases),
		valueCase{"calc(10%)", tup(dim(10, "%"))},
		valueCase{"min(10%, 20%)", tup(dim(10, "%"))},
		valueCase{"clamp(0%, 50%, 100%) fill", tup(dim(50, "%"), String("fill"))},
	))
}

func TestBorderImageSliceInvalid(t *testing.T) {
	// A math function that cannot resolve to a number or percentage must be
	// rejected, not crash with an AttributeError on a FunctionBlock.
	checkInvalid(t, "border-image-slice: %s", append(slices.Clone(borderSliceInvalid), "min(0px, 0)"))
}

// TestBorderImageSliceMathNoCrash is test_border_image_slice_math_no_crash:
// a math function of *-border-slice that cannot be resolved is ignored with
// a warning, and the page still renders.
func TestBorderImageSliceMathNoCrash(t *testing.T) {
	for _, c := range [][2]string{
		{"border-image-slice", "scale(calc(0))"},
		{"border-image-slice", "min(0px, 0)"},
		{"mask-border-slice", "scale(calc(0))"},
		{"mask-border-slice", "rgb(calc(0%) 0 0)"},
	} {
		t.Run(c[0]+": "+c[1], func(t *testing.T) {
			log, capture := captureLogs(slog.LevelInfo)
			pages := renderPagesLog(t, `<div style="`+c[0]+`: `+c[1]+`">a</div>`, testBaseURL, log)
			logs := capture.logs()
			if len(logs) != 1 || !strings.Contains(logs[0], "invalid value") {
				t.Errorf("got logs %q, want one invalid value", logs)
			}
			if len(pages) != 1 {
				t.Fatalf("got %d pages", len(pages))
			}
			_, body := pageBody(t, pages[0])
			if len(body.Children) == 0 {
				t.Error("the body has no child")
			}
		})
	}
}

// borderWidthCases are the cases of test_border_image_width and
// test_mask_border_width.
var borderWidthCases = []valueCase{
	{"1", tup(dim(1, ""))},
	{"1 2    3 4", tup(dim(1, ""), dim(2, ""), dim(3, ""), dim(4, ""))},
	{"50% 1000.1 0", tup(dim(50, "%"), dim(1000.1, ""), dim(0, ""))},
	{"1% 2px 3em 4", tup(dim(1, "%"), dim(2, "px"), dim(3, "em"), dim(4, ""))},
	{"auto", strs("auto")},
	{"1 auto", tup(dim(1, ""), String("auto"))},
	{"auto auto", strs("auto", "auto")},
	{"auto auto auto 2", tup(String("auto"), String("auto"), String("auto"), dim(2, ""))},
}

// borderWidthInvalid are the rules of test_border_image_width_invalid and
// test_mask_border_width_invalid.
var borderWidthInvalid = []string{
	"none", "1, 2", "1 -2", "-10", "-10%", "1px 2px 3px -10%", "-3px",
	"auto auto auto auto auto", "1 2 3 4 5",
}

// borderOutsetCases are the cases of test_border_image_outset and
// test_mask_border_outset.
var borderOutsetCases = []valueCase{
	{"1", tup(dim(1, ""))},
	{"1 2    3 4", tup(dim(1, ""), dim(2, ""), dim(3, ""), dim(4, ""))},
	{"50px 1000.1 0", tup(dim(50, "px"), dim(1000.1, ""), dim(0, ""))},
	{"1in 2px 3em 4", tup(dim(1, "in"), dim(2, "px"), dim(3, "em"), dim(4, ""))},
}

// borderOutsetInvalid are the rules of test_border_image_outset_invalid and
// test_mask_border_outset_invalid.
var borderOutsetInvalid = []string{
	"none", "auto", "1, 2", "-10", "1 -2", "10%", "1px 2px 3px -10px", "-3px", "1 2 3 4 5",
}

// borderRepeatCases are the cases of test_border_image_repeat and
// test_mask_border_repeat.
var borderRepeatCases = []valueCase{
	{"stretch", strs("stretch")},
	{"repeat repeat", strs("repeat", "repeat")},
	{"round     space", strs("round", "space")},
}

// borderRepeatInvalid are the rules of test_border_image_repeat_invalid and
// test_mask_border_repeat_invalid.
var borderRepeatInvalid = []string{
	"none", "test", "round round round", "stretch space round", "repeat test",
}

func TestBorderImageWidth(t *testing.T) {
	checkValues(t, "border-image-width: %s", borderWidthCases)
}

func TestBorderImageWidthInvalid(t *testing.T) {
	checkInvalid(t, "border-image-width: %s", borderWidthInvalid)
}

func TestBorderImageOutset(t *testing.T) {
	checkValues(t, "border-image-outset: %s", borderOutsetCases)
}

func TestBorderImageOutsetInvalid(t *testing.T) {
	checkInvalid(t, "border-image-outset: %s", borderOutsetInvalid)
}

func TestBorderImageRepeat(t *testing.T) {
	checkValues(t, "border-image-repeat: %s", borderRepeatCases)
}

func TestBorderImageRepeatInvalid(t *testing.T) {
	checkInvalid(t, "border-image-repeat: %s", borderRepeatInvalid)
}

func TestMaskBorderSlice(t *testing.T) {
	checkValues(t, "mask-border-slice: %s", borderSliceCases)
}

func TestMaskBorderSliceInvalid(t *testing.T) {
	checkInvalid(t, "mask-border-slice: %s", append(slices.Clone(borderSliceInvalid), "rgb(calc(0%) 0 0)"))
}

func TestMaskBorderWidth(t *testing.T) {
	checkValues(t, "mask-border-width: %s", borderWidthCases)
}

func TestMaskBorderWidthInvalid(t *testing.T) {
	checkInvalid(t, "mask-border-width: %s", borderWidthInvalid)
}

func TestMaskBorderOutset(t *testing.T) {
	checkValues(t, "mask-border-outset: %s", borderOutsetCases)
}

func TestMaskBorderOutsetInvalid(t *testing.T) {
	checkInvalid(t, "mask-border-outset: %s", borderOutsetInvalid)
}

func TestMaskBorderRepeat(t *testing.T) {
	checkValues(t, "mask-border-repeat: %s", borderRepeatCases)
}

func TestMaskBorderRepeatInvalid(t *testing.T) {
	checkInvalid(t, "mask-border-repeat: %s", borderRepeatInvalid)
}

func TestMaskBorderMode(t *testing.T) {
	checkValues(t, "mask-border-mode: %s", []valueCase{
		{"alpha", String("alpha")},
		{"luminance", String("luminance")},
		{"alpha     ", String("alpha")},
	})
}

func TestMaskBorderModeInvalid(t *testing.T) {
	checkInvalid(t, "mask-border-mode: %s", []string{"none", "test", "alpha alpha", "alpha luminance"})
}

func TestStringSet(t *testing.T) {
	set := func(name string, items ...Value) Tuple { return tup(String(name), tup(items...)) }
	contentText := strs("content()", "text")
	stringItem := strs("string", "string")
	attr := func(name, type_, fallback string) Value { return tup(String("attr()"), strs(name, type_, fallback)) }
	counter := func(args ...string) Value { return tup(String("counter()"), strs(args...)) }
	counters := func(args ...string) Value { return tup(String("counters()"), strs(args...)) }
	checkValues(t, "string-set: %s", []valueCase{
		{"test content(text)", tup(set("test", contentText))},
		{"test content(before)", tup(set("test", strs("content()", "before")))},
		{`test "string"`, tup(set("test", stringItem))},
		{`test1 "string", test2 "string"`, tup(set("test1", stringItem), set("test2", stringItem))},
		{"test attr(class)", tup(set("test", attr("class", "string", "")))},
		{"test attr(class url)", tup(set("test", attr("class", "url", "")))},
		{`test attr(class, "test")`, tup(set("test", attr("class", "string", "test")))},
		{`test attr(class string, "test")`, tup(set("test", attr("class", "string", "test")))},
		{"test counter(count)", tup(set("test", counter("count", "decimal")))},
		{"test counter(count, upper-roman)", tup(set("test", counter("count", "upper-roman")))},
		{`test counters(count, ".")`, tup(set("test", counters("count", ".", "decimal")))},
		{`test counters(count, ".", upper-roman)`, tup(set("test", counters("count", ".", "upper-roman")))},
		{`test content(text) "string" attr(title) attr(title) counter(count)`, tup(set("test",
			contentText, stringItem, attr("title", "string", ""), attr("title", "string", ""),
			counter("count", "decimal")))},
	})
}

func TestStringSetInvalid(t *testing.T) {
	checkInvalid(t, "string-set: %s", []string{"test", "test test1", "test content(test)", "test unknown()"})
}

func TestOverflowWrap(t *testing.T) {
	checkValues(t, "overflow-wrap: %s", []valueCase{
		{"normal", String("normal")},
		{"break-word", String("break-word")},
		{"inherit", String("inherit")},
	})
}

func TestOverflowWrapInvalid(t *testing.T) {
	checkInvalid(t, "overflow-wrap: %s", []string{"none", "normal, break-word"})
}

// rgba is a color of the tests, as its sRGB channels and its alpha.
type rgba [4]float64

var (
	blue = rgba{0, 0, 1, 1}
	lime = rgba{0, 1, 0, 1}
	red  = rgba{1, 0, 0, 1}
)

// checkColors checks the colors of a gradient against their channels.
func checkColors(t *testing.T, css string, colors []Value, want []rgba) {
	t.Helper()
	if len(colors) != len(want) {
		t.Errorf("%s: expected %d colors, got: %d", css, len(want), len(colors))
		return
	}
	for i, c := range colors {
		color, ok := c.(Color)
		if !ok || color.Color == nil {
			t.Errorf("%s: expected a color, got: %s", css, ValueString(c))
			continue
		}
		srgb, ok := color.To("srgb")
		if !ok {
			t.Errorf("%s: cannot convert %s to srgb", css, ValueString(c))
			continue
		}
		got := rgba{srgb.Coordinates[0], srgb.Coordinates[1], srgb.Coordinates[2], srgb.Alpha}
		if got != want[i] {
			t.Errorf("%s: expected color %v, got: %v", css, want[i], got)
		}
	}
}

// checkStops checks the stop positions of a gradient.
func checkStops(t *testing.T, css string, stops []Value, want Tuple) {
	t.Helper()
	if !Equal(Tuple(stops), want) {
		t.Errorf("%s: expected stops %s, got: %s", css, ValueString(want), ValueString(Tuple(stops)))
	}
}

func TestLinearGradient(t *testing.T) {
	type gradientCase struct {
		rule      string
		direction Tuple
		colors    []rgba
		stops     Tuple
	}
	angle := func(a float64) Tuple { return tup(String("angle"), Float(a)) }
	corner := func(c string) Tuple { return strs("corner", c) }
	threeColors := []rgba{blue, lime, red}
	threeStops := tup(dim(1, "%"), nil, dim(2, "em"))
	cases := []gradientCase{
		{"blue", nil, nil, nil},
		{"red", nil, []rgba{red}, nil},
		{"blue 1%, lime,red 2em ", nil, threeColors, threeStops},
		{"18deg, blue", angle(pi / 10), nil, nil},
		{"4rad, blue", angle(4), nil, nil},
		{".25turn, blue", angle(pi / 2), nil, nil},
		{"100grad, blue", angle(pi / 2), nil, nil},
		{"12rad, blue 1%, lime,red 2em ", angle(12), threeColors, threeStops},
		{"to top, blue", angle(0), nil, nil},
		{"to right, blue", angle(pi / 2), nil, nil},
		{"to bottom, blue", angle(pi), nil, nil},
		{"to left, blue", angle(pi * 3 / 2), nil, nil},
		{"to right, blue 1%, lime,red 2em ", angle(pi / 2), threeColors, threeStops},
		{"to top left, blue", corner("top_left"), nil, nil},
		{"to left top, blue", corner("top_left"), nil, nil},
		{"to top right, blue", corner("top_right"), nil, nil},
		{"to right top, blue", corner("top_right"), nil, nil},
		{"to bottom left, blue", corner("bottom_left"), nil, nil},
		{"to left bottom, blue", corner("bottom_left"), nil, nil},
		{"to bottom right, blue", corner("bottom_right"), nil, nil},
		{"to right bottom, blue", corner("bottom_right"), nil, nil},
	}
	for _, c := range cases {
		direction, colors, stops := c.direction, c.colors, c.stops
		if direction == nil {
			direction = angle(pi)
		}
		if colors == nil {
			colors = []rgba{blue}
		}
		if stops == nil {
			stops = tup(nil)
		}
		for _, repeating := range []bool{false, true} {
			prefix := ""
			if repeating {
				prefix = "repeating-"
			}
			css := fmt.Sprintf("background-image: %slinear-gradient(%s)", prefix, c.rule)
			value, ok := getValue(t, css, "", slog.LevelInfo).(Tuple)
			if !ok || len(value) != 1 {
				t.Errorf("%s: expected one image, got: %s", css, ValueString(value))
				continue
			}
			image, ok := value[0].(Tuple)
			if !ok || len(image) != 2 || !isStr(image[0], "linear-gradient") {
				t.Errorf("%s: expected a linear gradient, got: %s", css, ValueString(value[0]))
				continue
			}
			gradient, ok := image[1].(*LinearGradient)
			if !ok {
				t.Errorf("%s: expected a *LinearGradient", css)
				continue
			}
			if gradient.Repeating != repeating {
				t.Errorf("%s: expected repeating %t", css, repeating)
			}
			if gradient.DirectionType != str(direction[0]) {
				t.Errorf("%s: expected direction type %s, got: %s", css, ValueString(direction[0]), gradient.DirectionType)
			}
			if f, ok := direction[1].(Float); ok {
				if math.Abs(num(gradient.Direction)-float64(f)) > 1e-9 {
					t.Errorf("%s: expected direction %v, got: %s", css, f, ValueString(gradient.Direction))
				}
			}
			checkColors(t, css, gradient.Colors, colors)
			checkStops(t, css, gradient.StopPositions, stops)
		}
	}
}

func TestLinearGradientInvalid(t *testing.T) {
	rules := []string{
		" ", "1% blue", "blue 10deg", "blue 4", "soylent-green 4px", "red 4px 2px",
		"18deg", "10arc-minutes, blue", "10px, blue", "to 90deg, blue",
		"to the top, blue", "to up, blue", "into top, blue", "top, blue",
		"to bottom up, blue", "bottom left, blue",
	}
	checkInvalid(t, "background-image: linear-gradient(%s)", rules)
	checkInvalid(t, "background-image: repeating-linear-gradient(%s)", rules)
}

func TestRadialGradient(t *testing.T) {
	type gradientCase struct {
		rule   string
		shape  string
		size   Tuple
		center Value
		colors []rgba
		stops  Tuple
	}
	threeColors := []rgba{blue, lime, red}
	threeStops := tup(dim(1, "%"), nil, dim(2, "em"))
	keyword := func(k string) Tuple { return strs("keyword", k) }
	explicit := func(x, y Dimension) Tuple { return tup(String("explicit"), tup(x, y)) }
	cases := []gradientCase{
		{"blue", "", nil, nil, nil, nil},
		{"red", "", nil, nil, []rgba{red}, nil},
		{"blue 1%, lime,red 2em ", "", nil, nil, threeColors, threeStops},
		{"circle, blue", "circle", nil, nil, nil, nil},
		{"ellipse, blue", "", nil, nil, nil, nil},
		{"ellipse closest-corner, blue", "ellipse", keyword("closest-corner"), nil, nil, nil},
		{"circle closest-side, blue", "circle", keyword("closest-side"), nil, nil, nil},
		{"farthest-corner circle, blue", "circle", keyword("farthest-corner"), nil, nil, nil},
		{"farthest-side, blue", "", keyword("farthest-side"), nil, nil, nil},
		{"5ch, blue", "circle", explicit(dim(5, "ch"), dim(5, "ch")), nil, nil, nil},
		{"5ch circle, blue", "circle", explicit(dim(5, "ch"), dim(5, "ch")), nil, nil, nil},
		{"circle 5ch, blue", "circle", explicit(dim(5, "ch"), dim(5, "ch")), nil, nil, nil},
		{"10px 50px, blue", "", explicit(dim(10, "px"), dim(50, "px")), nil, nil, nil},
		{"10px 50px ellipse, blue", "", explicit(dim(10, "px"), dim(50, "px")), nil, nil, nil},
		{"ellipse 10px 50px, blue", "", explicit(dim(10, "px"), dim(50, "px")), nil, nil, nil},
		{"at top 10% right, blue", "", nil,
			tup(String("right"), dim(0, "%"), String("top"), dim(10, "%")), nil, nil},
		{"circle at bottom, blue", "circle", nil,
			tup(String("left"), dim(50, "%"), String("top"), dim(100, "%")), nil, nil},
		{"circle at 10px, blue", "circle", nil,
			tup(String("left"), dim(10, "px"), String("top"), dim(50, "%")), nil, nil},
		{"closest-side circle at right 5em, blue", "circle", keyword("closest-side"),
			tup(String("left"), dim(100, "%"), String("top"), dim(5, "em")), nil, nil},
	}
	for _, c := range cases {
		shape, size, center, colors, stops := c.shape, c.size, c.center, c.colors, c.stops
		if shape == "" {
			shape = "ellipse"
		}
		if size == nil {
			size = keyword("farthest-corner")
		}
		if center == nil {
			center = tup(String("left"), dim(50, "%"), String("top"), dim(50, "%"))
		}
		if colors == nil {
			colors = []rgba{blue}
		}
		if stops == nil {
			stops = tup(nil)
		}
		for _, repeating := range []bool{false, true} {
			prefix := ""
			if repeating {
				prefix = "repeating-"
			}
			css := fmt.Sprintf("background-image: %sradial-gradient(%s)", prefix, c.rule)
			value, ok := getValue(t, css, "", slog.LevelInfo).(Tuple)
			if !ok || len(value) != 1 {
				t.Errorf("%s: expected one image, got: %s", css, ValueString(value))
				continue
			}
			image, ok := value[0].(Tuple)
			if !ok || len(image) != 2 || !isStr(image[0], "radial-gradient") {
				t.Errorf("%s: expected a radial gradient, got: %s", css, ValueString(value[0]))
				continue
			}
			gradient, ok := image[1].(*RadialGradient)
			if !ok {
				t.Errorf("%s: expected a *RadialGradient", css)
				continue
			}
			if gradient.Repeating != repeating {
				t.Errorf("%s: expected repeating %t", css, repeating)
			}
			if gradient.Shape != shape {
				t.Errorf("%s: expected shape %s, got: %s", css, shape, gradient.Shape)
			}
			if gradient.SizeType != str(size[0]) || !Equal(gradient.Size, size[1]) {
				t.Errorf("%s: expected size %s, got: %s %s", css, ValueString(size), gradient.SizeType, ValueString(gradient.Size))
			}
			if !Equal(gradient.Center, center) {
				t.Errorf("%s: expected center %s, got: %s", css, ValueString(center), ValueString(gradient.Center))
			}
			checkColors(t, css, gradient.Colors, colors)
			checkStops(t, css, gradient.StopPositions, stops)
		}
	}
}

func TestRadialGradientInvalid(t *testing.T) {
	rules := []string{
		" ", "1% blue", "blue 10deg", "blue 4", "soylent-green 4px", "red 4px 2px",
		"circle", "square, blue", "closest-triangle, blue", "center, blue",
		"ellipse 5ch", "5ch ellipse", "circle 10px 50px, blue", "10px 50px circle, blue",
		"10%, blue", "10% circle, blue", "circle 10%, blue", "at appex, blue",
	}
	checkInvalid(t, "background-image: radial-gradient(%s)", rules)
	checkInvalid(t, "background-image: repeating-radial-gradient(%s)", rules)
}

func TestGridAutoColumnsRows(t *testing.T) {
	cases := []valueCase{
		{"40px", tup(dim(40, "px"))},
		{"2fr", tup(dim(2, "fr"))},
		{"18%", tup(dim(18, "%"))},
		{"auto", strs("auto")},
		{"min-content", strs("min-content")},
		{"max-content", strs("max-content")},
		{"fit-content(20%)", tup(tup(String("fit-content()"), dim(20, "%")))},
		{"minmax(20px, 25px)", tup(tup(String("minmax()"), dim(20, "px"), dim(25, "px")))},
		{"minmax(min-content, max-content)", tup(strs("minmax()", "min-content", "max-content"))},
		{"min-content max-content", strs("min-content", "max-content")},
	}
	checkValues(t, "grid-auto-columns: %s", cases)
	checkValues(t, "grid-auto-rows: %s", cases)
}

func TestGridAutoColumnsRowsInvalid(t *testing.T) {
	rules := []string{
		"40", "coucou", "fit-content", "fit-content(min-content)", "minmax(40px)",
		"minmax(2fr, 1fr)", "1fr 1fr coucou", "fit-content()", "fit-content(2%, 18%)",
	}
	checkInvalid(t, "grid-auto-columns: %s", rules)
	checkInvalid(t, "grid-auto-rows: %s", rules)
}

func TestGridAutoFlow(t *testing.T) {
	checkValues(t, "grid-auto-flow: %s", []valueCase{
		{"row", strs("row")},
		{"column", strs("column")},
		{"row dense", strs("row", "dense")},
		{"column dense", strs("column", "dense")},
		{"dense row", strs("dense", "row")},
		{"dense column", strs("dense", "column")},
		{"dense", strs("dense", "row")},
	})
}

func TestGridAutoFlowInvalid(t *testing.T) {
	checkInvalid(t, "grid-auto-flow: %s", []string{
		"row row", "column column", "dense dense", "coucou", "row column", "column row",
		"row coucou", "column coucou", "coucou row", "coucou column", "row column dense",
	})
}

func TestGridTemplateColumnsRows(t *testing.T) {
	cases := []valueCase{
		{"none", String("none")},
		{"subgrid", tup(String("subgrid"), tup())},
		{"subgrid [a] repeat(auto-fill, [b]) [c]", tup(String("subgrid"), tup(
			strs("a"), tup(String("repeat()"), String("auto-fill"), tup(strs("b"))), strs("c")))},
		{"subgrid [a] [a] [a] [a] repeat(auto-fill, [b]) [c] [c]", tup(String("subgrid"), tup(
			strs("a"), strs("a"), strs("a"), strs("a"),
			tup(String("repeat()"), String("auto-fill"), tup(strs("b"))), strs("c"), strs("c")))},
		{"subgrid [] [a]", tup(String("subgrid"), tup(tup(), strs("a")))},
		{"subgrid [a] [b] [c] [d] [e] [f]", tup(String("subgrid"), tup(
			strs("a"), strs("b"), strs("c"), strs("d"), strs("e"), strs("f")))},
		{"[outer-edge] 20px [main-start] 1fr [center] 1fr max-content [main-end]", tup(
			strs("outer-edge"), dim(20, "px"), strs("main-start"), dim(1, "fr"), strs("center"),
			dim(1, "fr"), tup(), String("max-content"), strs("main-end"))},
		{"repeat(auto-fill, minmax(25ch, 1fr))", tup(
			tup(), tup(String("repeat()"), String("auto-fill"), tup(
				tup(), tup(String("minmax()"), dim(25, "ch"), dim(1, "fr")), tup())), tup())},
		{"[a] auto [b] minmax(min-content, 1fr) [b c d] repeat(2, [e] 40px) repeat(5, auto)", tup(
			strs("a"), String("auto"), strs("b"), tup(String("minmax()"), String("min-content"), dim(1, "fr")),
			strs("b", "c", "d"), tup(String("repeat()"), Int(2), tup(strs("e"), dim(40, "px"), tup())),
			tup(), tup(String("repeat()"), Int(5), tup(tup(), String("auto"), tup())), tup())},
	}
	checkValues(t, "grid-template-columns: %s", cases)
	checkValues(t, "grid-template-rows: %s", cases)
}

func TestGridTemplateColumnsRowsInvalid(t *testing.T) {
	rules := []string{
		"coucou", "subgrid subgrid", "subgrid coucou", "subgrid [coucou] repeat(0, [wow])",
		"subgrid [coucou] repeat(auto-fit [wow])", "fit-content(18%) repeat(auto-fill, 15em)",
		"[coucou] [wow]",
	}
	checkInvalid(t, "grid-template-columns: %s", rules)
	checkInvalid(t, "grid-template-rows: %s", rules)
}

func TestGridTemplateAreas(t *testing.T) {
	checkValues(t, "grid-template-areas: %s", []valueCase{
		{"none", String("none")},
		{`"head head" "nav main" "foot ...."`, tup(
			strs("head", "head"), strs("nav", "main"), tup(String("foot"), nil))},
		{`"title board" "stats board"`, tup(strs("title", "board"), strs("stats", "board"))},
		{`". a" "b a" ".a"`, tup(tup(nil, String("a")), strs("b", "a"), tup(nil, String("a")))},
		{`"a b b" "c b b" "d e f"`, tup(strs("a", "b", "b"), strs("c", "b", "b"), strs("d", "e", "f"))},
	})
}

func TestGridTemplateAreasInvalid(t *testing.T) {
	checkInvalid(t, "grid-template-areas: %s", []string{
		`"head head coucou" "nav main" "foot ...."`,
		`". a" "b c" ". a"`,
		`". a" "b a" "a a"`,
		`"a a a a" "a b b a" "a a a a"`,
		`" "`,
	})
}

func TestGridLine(t *testing.T) {
	cases := []valueCase{
		{"auto", String("auto")},
		{"4", tup(nil, Int(4), nil)},
		{"C", tup(nil, nil, String("C"))},
		{"4 c", tup(nil, Int(4), String("c"))},
		{"col -4", tup(nil, Int(-4), String("col"))},
		{"span c 4", tup(String("span"), Int(4), String("c"))},
		{"span 4 c", tup(String("span"), Int(4), String("c"))},
		{"4 span C", tup(String("span"), Int(4), String("C"))},
		{"super 4 span", tup(String("span"), Int(4), String("super"))},
	}
	for _, name := range []string{"grid-row-start", "grid-row-end", "grid-column-start", "grid-column-end"} {
		checkValues(t, name+": %s", cases)
	}
}

func TestGridLineInvalid(t *testing.T) {
	rules := []string{
		"span", "0", "1.1", "span 0", "span -1", "span 2.1", "span auto", "auto auto",
		"-4 cOL span", "span 1.1 col",
	}
	for _, name := range []string{"grid-row-start", "grid-row-end", "grid-column-start", "grid-column-end"} {
		checkInvalid(t, name+": %s", rules)
	}
}

// keywordCases makes the cases of a test whose values are tuples of the
// keywords of the rule, with "baseline" alone as ("first", "baseline").
func keywordCases(rules ...string) []valueCase {
	cases := make([]valueCase, len(rules))
	for i, rule := range rules {
		keywords := splitSpaces(rule)
		if rule == "baseline" {
			keywords = []string{"first", "baseline"}
		}
		cases[i] = valueCase{rule, strs(keywords...)}
	}
	return cases
}

// splitSpaces splits a rule on its spaces.
func splitSpaces(s string) []string {
	var parts []string
	start := -1
	for i, c := range s + " " {
		if c == ' ' {
			if start >= 0 {
				parts = append(parts, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	return parts
}

func TestAlignContent(t *testing.T) {
	checkValues(t, "align-content: %s", keywordCases(
		"normal", "baseline", "first baseline", "last baseline", "baseline last",
		"space-between", "space-around", "space-evenly", "stretch", "center", "start",
		"end", "flex-start", "flex-end", "safe center", "unsafe start", "safe end",
		"safe flex-start", "unsafe flex-start"))
}

func TestAlignContentInvalid(t *testing.T) {
	checkInvalid(t, "align-content: %s", []string{
		"auto", "none", "auto auto", "first last", "baseline baseline", "start safe",
		"start end", "safe unsafe", "left", "right",
	})
}

func TestAlignItems(t *testing.T) {
	checkValues(t, "align-items: %s", keywordCases(
		"normal", "stretch", "baseline", "first baseline", "last baseline",
		"baseline last", "center", "self-start", "self-end", "start", "end",
		"flex-start", "flex-end", "safe center", "unsafe start", "safe end",
		"unsafe self-start", "safe self-end", "safe flex-start", "unsafe flex-start"))
}

func TestAlignItemsInvalid(t *testing.T) {
	checkInvalid(t, "align-items: %s", []string{
		"auto", "none", "auto auto", "first last", "baseline baseline", "start safe",
		"start end", "safe unsafe", "left", "right", "space-between",
	})
}

func TestAlignSelf(t *testing.T) {
	checkValues(t, "align-self: %s", keywordCases(
		"auto", "normal", "stretch", "baseline", "first baseline", "last baseline",
		"baseline last", "center", "self-start", "self-end", "start", "end",
		"flex-start", "flex-end", "safe center", "unsafe start", "safe end",
		"unsafe self-start", "safe self-end", "safe flex-start", "unsafe flex-start"))
}

func TestAlignSelfInvalid(t *testing.T) {
	checkInvalid(t, "align-self: %s", []string{
		"none", "auto auto", "first last", "baseline baseline", "start safe",
		"start end", "safe unsafe", "left", "right", "space-between",
	})
}

func TestJustifyContent(t *testing.T) {
	checkValues(t, "justify-content: %s", keywordCases(
		"normal", "space-between", "space-around", "space-evenly", "stretch",
		"center", "left", "right", "start", "end", "flex-start", "flex-end",
		"safe center", "unsafe start", "safe end", "unsafe left", "safe right",
		"safe flex-start", "unsafe flex-start"))
}

func TestJustifyContentInvalid(t *testing.T) {
	checkInvalid(t, "justify-content: %s", []string{
		"auto", "none", "baseline", "auto auto", "first last", "baseline baseline",
		"start safe", "start end", "safe unsafe",
	})
}

func TestJustifyItems(t *testing.T) {
	checkValues(t, "justify-items: %s", keywordCases(
		"normal", "stretch", "baseline", "first baseline", "last baseline",
		"baseline last", "center", "self-start", "self-end", "start", "end", "left",
		"right", "flex-start", "flex-end", "safe center", "unsafe start", "safe end",
		"unsafe self-start", "safe self-end", "safe flex-start", "unsafe flex-start",
		"legacy", "legacy left", "left legacy", "legacy center"))
}

func TestJustifyItemsInvalid(t *testing.T) {
	checkInvalid(t, "justify-items: %s", []string{
		"auto", "none", "auto auto", "first last", "baseline baseline", "start safe",
		"start end", "safe unsafe", "space-between",
	})
}

func TestJustifySelf(t *testing.T) {
	checkValues(t, "justify-self: %s", keywordCases(
		"auto", "normal", "stretch", "baseline", "first baseline", "last baseline",
		"baseline last", "center", "self-start", "self-end", "start", "end", "left",
		"right", "flex-start", "flex-end", "safe center", "unsafe start", "safe end",
		"unsafe left", "safe right", "unsafe self-start", "safe self-end",
		"safe flex-start", "unsafe flex-start"))
}

func TestJustifySelfInvalid(t *testing.T) {
	checkInvalid(t, "justify-self: %s", []string{
		"none", "auto auto", "first last", "baseline baseline", "start safe",
		"start end", "safe unsafe", "space-between",
	})
}

func TestColorsInvalid(t *testing.T) {
	for _, rule := range []string{
		"color", "background", "background-color", "border-color", "border-left-color", "outline-color",
	} {
		if _, ok := expanders[rule]; !ok && !knownProperties[rule] {
			t.Logf("%s: skipped, the expander is not ported yet", rule)
			continue
		}
		for _, value := range []string{"rgb()", "device-cmyk(0%, 0%, 0%, 30%)", "#abcde"} {
			assertInvalid(t, rule+": "+value, "invalid", slog.LevelInfo)
		}
	}
}

func TestColorScheme(t *testing.T) {
	checkValues(t, "color-scheme: %s", []valueCase{
		{"normal", String("normal")},
		{"light", strs("light")},
		{"dark", strs("dark")},
		{"light dark", strs("light", "dark")},
		{"dark light", strs("dark", "light")},
		{"light only", strs("light", "only")},
		{"only light", strs("light", "only")},
		{"dark dark", strs("dark", "dark")},
		{"light something", strs("light", "something")},
		{"only dark light", strs("dark", "light", "only")},
		{"only something light", strs("something", "light", "only")},
	})
}

func TestColorSchemeInvalid(t *testing.T) {
	checkInvalid(t, "color-scheme: %s", []string{
		"normal only", "normal something", "only", "only only", "light only dark",
	})
}

func TestBoxShadow(t *testing.T) {
	shadow := func(x, y, blur, spread Dimension, inset bool) Tuple {
		return tup(x, y, blur, spread, nil, Bool(inset))
	}
	px := func(v float64) Dimension { return dim(v, "px") }
	cases := []struct {
		rule string
		want Tuple
	}{
		{"none", tup()},
		{"1px 10px", tup(shadow(px(1), px(10), px(0), px(0), false))},
		{"1px 10px 0 3cm", tup(shadow(px(1), px(10), dim(0, ""), dim(3, "cm"), false))},
		{"1px 10px inset", tup(shadow(px(1), px(10), px(0), px(0), true))},
		{"1px 10px 2px 5cm inset", tup(shadow(px(1), px(10), px(2), dim(5, "cm"), true))},
		{"black 1px 10px 2px 5cm", tup(shadow(px(1), px(10), px(2), dim(5, "cm"), false))},
		{"red 1px 10px inset, blue 1px 10px inset", tup(
			shadow(px(1), px(10), px(0), px(0), true),
			shadow(px(1), px(10), px(0), px(0), true))},
		{"red 1px 10px 2px 5cm", tup(shadow(px(1), px(10), px(2), dim(5, "cm"), false))},
		{"inset red 1px 10px 2px 5cm", tup(shadow(px(1), px(10), px(2), dim(5, "cm"), true))},
		{"1px 10px 2px 5cm red inset", tup(shadow(px(1), px(10), px(2), dim(5, "cm"), true))},
		{"1px 10px, 2px 4px #ff0000", tup(
			shadow(px(1), px(10), px(0), px(0), false),
			shadow(px(2), px(4), px(0), px(0), false))},
	}
	for _, c := range cases {
		css := "box-shadow: " + c.rule
		result, ok := getValue(t, css, "", slog.LevelInfo).(Tuple)
		if !ok || len(result) != len(c.want) {
			t.Errorf("%s: expected %s, got: %s", css, ValueString(c.want), ValueString(result))
			continue
		}
		for i, got := range result {
			gotShadow, ok := got.(Tuple)
			want := c.want[i].(Tuple)
			if !ok || len(gotShadow) != 6 {
				t.Errorf("%s: expected a shadow, got: %s", css, ValueString(got))
				continue
			}
			if !Equal(gotShadow[:4], want[:4]) || !Equal(gotShadow[5], want[5]) {
				t.Errorf("%s: expected %s, got: %s", css, ValueString(want), ValueString(gotShadow))
			}
			if parseColor(gotShadow[4]) == nil {
				t.Errorf("%s: expected a color, got: %s", css, ValueString(gotShadow[4]))
			}
		}
	}
}

func TestBoxShadowInvalid(t *testing.T) {
	checkInvalid(t, "box-shadow: %s", []string{
		"1px 10px,", "1px 10px,,1px", "1px red inset", "1px 1px red blue inset",
		"blue 1px 1px red", "inset 1px 1px 1px blue inset", "1px 10px red 3px",
		"1px 10px inset 3px", "1px 1px 1px inset inset", "1px 0 0 3%", "1px 0, none",
		"none, none",
	})
}
