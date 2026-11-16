// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_expanders.py, the tests of the expanders of
// the shorthand properties.

package layout

import (
	"log/slog"
	"maps"
	"slices"
	"strings"
	"testing"
)

// etup, edim and estr build the expected values of the tests.
func etup(items ...Value) Tuple             { return Tuple(items) }
func edim(v float64, unit string) Dimension { return Dimension{v, unit} }
func estr(s string) String                  { return String(s) }

// expandToDict is expand_to_dict in tests/css/test_expanders.py. It expands
// the declarations of css, and returns the values that are not 'initial',
// with the colors parsed. It checks that the log holds one message that
// contains expectedError, or no message when expectedError is "".
func expandToDict(t *testing.T, css, expectedError string) map[string]Value {
	t.Helper()
	log, capture := captureLogs(slog.LevelInfo)
	declarations := preprocessCSS(log, css)
	if expectedError != "" {
		assertLog(t, capture, expectedError)
	} else {
		assertNoLogs(t, capture)
	}
	result := map[string]Value{}
	for _, d := range declarations {
		if isStr(d.Value, "initial") {
			continue
		}
		value := d.Value
		if strings.HasSuffix(d.Name, "_color") && !isStr(value, "inherit") {
			value = parseColor(value)
		}
		result[d.Name] = value
	}
	return result
}

// assertExpandInvalid is assert_invalid in tests/css/test_expanders.py.
func assertExpandInvalid(t *testing.T, css, message string) {
	t.Helper()
	if message == "" {
		message = "invalid"
	}
	if got := expandToDict(t, css, message); len(got) != 0 {
		t.Errorf("%s: expected no value, got: %v", css, dictString(got))
	}
}

// dictString formats a dictionary of values for a message.
func dictString(d map[string]Value) string {
	keys := slices.Sorted(maps.Keys(d))
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + ": " + ValueString(d[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// assertDict checks that two dictionaries of values are equal.
func assertDict(t *testing.T, css string, got, want map[string]Value) {
	t.Helper()
	equal := len(got) == len(want)
	for k, v := range want {
		if g, ok := got[k]; !ok || !Equal(g, v) {
			equal = false
		}
	}
	if !equal {
		t.Errorf("%s:\n got: %s\nwant: %s", css, dictString(got), dictString(want))
	}
}

// expandCase is a case of a table of valid rules.
type expandCase struct {
	rule string
	want map[string]Value
}

// runExpandCases checks each case, with prefix before the rule.
func runExpandCases(t *testing.T, prefix string, cases []expandCase) {
	t.Helper()
	for _, c := range cases {
		css := prefix + c.rule
		assertDict(t, css, expandToDict(t, css, ""), c.want)
	}
}

// runInvalidCases checks that each rule is invalid, with prefix before the
// rule.
func runInvalidCases(t *testing.T, prefix string, rules []string) {
	t.Helper()
	for _, rule := range rules {
		assertExpandInvalid(t, prefix+rule, "")
	}
}

func TestEmptyExpanderValue(t *testing.T) {
	for name := range expanders {
		assertExpandInvalid(t, name+":", "Ignored")
	}
}

func TestExpanderTextDecoration(t *testing.T) {
	inherit := map[string]Value{}
	for _, key := range []string{"color", "line", "style", "thickness"} {
		inherit["text_decoration_"+key] = estr("inherit")
	}
	runExpandCases(t, "text-decoration: ", []expandCase{
		{"none", map[string]Value{"text_decoration_line": estr("none")}},
		{"overline", map[string]Value{"text_decoration_line": newStringSet("overline")}},
		{"overline blink line-through", map[string]Value{
			"text_decoration_line": newStringSet("blink", "line-through", "overline"),
		}},
		{"red", map[string]Value{"text_decoration_color": mustParseColor("red")}},
		{"blue 1px", map[string]Value{
			"text_decoration_color":     mustParseColor("blue"),
			"text_decoration_thickness": edim(1, "px"),
		}},
		{"100% none", map[string]Value{
			"text_decoration_line":      estr("none"),
			"text_decoration_thickness": edim(100, "%"),
		}},
		{"inherit", inherit},
	})
}

func TestExpanderTextDecorationInvalid(t *testing.T) {
	runInvalidCases(t, "text-decoration: ", []string{
		"solid solid", "red red", "underline none", "1px 100%", "none none",
	})
}

func TestExpanderFourSides(t *testing.T) {
	runExpandCases(t, "", []expandCase{
		{"margin: inherit", map[string]Value{
			"margin_top":    estr("inherit"),
			"margin_right":  estr("inherit"),
			"margin_bottom": estr("inherit"),
			"margin_left":   estr("inherit"),
		}},
		{"margin: 1em", map[string]Value{
			"margin_top":    edim(1, "em"),
			"margin_right":  edim(1, "em"),
			"margin_bottom": edim(1, "em"),
			"margin_left":   edim(1, "em"),
		}},
		{"margin: -1em auto 20%", map[string]Value{
			"margin_top":    edim(-1, "em"),
			"margin_right":  estr("auto"),
			"margin_bottom": edim(20, "%"),
			"margin_left":   estr("auto"),
		}},
		{"padding: 1em 0", map[string]Value{
			"padding_top":    edim(1, "em"),
			"padding_right":  edim(0, ""),
			"padding_bottom": edim(1, "em"),
			"padding_left":   edim(0, ""),
		}},
		{"padding: 1em 0 2%", map[string]Value{
			"padding_top":    edim(1, "em"),
			"padding_right":  edim(0, ""),
			"padding_bottom": edim(2, "%"),
			"padding_left":   edim(0, ""),
		}},
		{"padding: 1em 0 2em 5px", map[string]Value{
			"padding_top":    edim(1, "em"),
			"padding_right":  edim(0, ""),
			"padding_bottom": edim(2, "em"),
			"padding_left":   edim(5, "px"),
		}},
	})
}

func TestExpanderFourSidesWarning(t *testing.T) {
	if got := expandToDict(t, "padding: 1 2 3 4 5", "Expected 1 to 4 tokens, got 5"); len(got) != 0 {
		t.Errorf("expected no value, got: %s", dictString(got))
	}
}

func TestExpanderFourSidesInvalid(t *testing.T) {
	runInvalidCases(t, "", []string{
		"margin: rgb(0, 0, 0)",
		"padding: auto",
		"padding: -12px",
		"border-width: -3em",
		"border-width: 12%",
	})
}

func TestExpanderBorders(t *testing.T) {
	lime := map[string]Value{}
	for _, side := range []string{"top", "left", "bottom", "right"} {
		lime["border_"+side+"_width"] = edim(6, "px")
		lime["border_"+side+"_style"] = estr("dashed")
		lime["border_"+side+"_color"] = mustParseColor("lime")
	}
	runExpandCases(t, "", []expandCase{
		{"border-top: 3px dotted red", map[string]Value{
			"border_top_width": edim(3, "px"),
			"border_top_style": estr("dotted"),
			"border_top_color": mustParseColor("red"),
		}},
		{"border-top: 3px dotted", map[string]Value{
			"border_top_width": edim(3, "px"),
			"border_top_style": estr("dotted"),
		}},
		{"border-top: 3px red", map[string]Value{
			"border_top_width": edim(3, "px"),
			"border_top_color": mustParseColor("red"),
		}},
		{"border-top: solid", map[string]Value{"border_top_style": estr("solid")}},
		{"border: 6px dashed lime", lime},
	})
}

func TestExpanderBordersInvalid(t *testing.T) {
	assertExpandInvalid(t, "border: 6px dashed left", "")
}

func TestExpanderListStyle(t *testing.T) {
	runExpandCases(t, "list-style: ", []expandCase{
		{"inherit", map[string]Value{
			"list_style_position": estr("inherit"),
			"list_style_image":    estr("inherit"),
			"list_style_type":     estr("inherit"),
		}},
		{"url(../bar/lipsum.png)", map[string]Value{
			"list_style_image": etup(estr("url"), estr("https://weasyprint.org/bar/lipsum.png")),
		}},
		{"square", map[string]Value{"list_style_type": estr("square")}},
		{"circle inside", map[string]Value{
			"list_style_position": estr("inside"),
			"list_style_type":     estr("circle"),
		}},
		{"none circle inside", map[string]Value{
			"list_style_position": estr("inside"),
			"list_style_image":    etup(estr("none"), nil),
			"list_style_type":     estr("circle"),
		}},
		{"none inside none", map[string]Value{
			"list_style_position": estr("inside"),
			"list_style_image":    etup(estr("none"), nil),
			"list_style_type":     estr("none"),
		}},
		{"inside special none", map[string]Value{
			"list_style_position": estr("inside"),
			"list_style_image":    etup(estr("none"), nil),
			"list_style_type":     estr("special"),
		}},
	})
}

func TestExpanderListStyleWarning(t *testing.T) {
	assertExpandInvalid(t, "list-style: circle disc", "got multiple type values in a list-style shorthand")
}

func TestExpanderListStyleInvalid(t *testing.T) {
	runInvalidCases(t, "list-style: ", []string{"none inside none none", "1px"})
}

func TestExpanderBackground(t *testing.T) {
	url := func(u string) Value { return etup(estr("url"), estr(u)) }
	position := func(ox string, x Dimension, oy string, y Dimension) Value {
		return etup(estr(ox), x, estr(oy), y)
	}
	cases := []expandCase{
		{"red", map[string]Value{"background_color": mustParseColor("red")}},
		{"url(lipsum.png)", map[string]Value{
			"background_image": etup(url("https://weasyprint.org/foo/lipsum.png")),
		}},
		{"no-repeat", map[string]Value{
			"background_repeat": etup(etup(estr("no-repeat"), estr("no-repeat"))),
		}},
		{"fixed", map[string]Value{"background_attachment": etup(estr("fixed"))}},
		{"repeat no-repeat fixed", map[string]Value{
			"background_repeat":     etup(etup(estr("repeat"), estr("no-repeat"))),
			"background_attachment": etup(estr("fixed")),
		}},
		{"top", map[string]Value{
			"background_position": etup(position("left", edim(50, "%"), "top", edim(0, "%"))),
		}},
		{"top right", map[string]Value{
			"background_position": etup(position("left", edim(100, "%"), "top", edim(0, "%"))),
		}},
		{"top right 20px", map[string]Value{
			"background_position": etup(position("right", edim(20, "px"), "top", edim(0, "%"))),
		}},
		{"top 1% right 20px", map[string]Value{
			"background_position": etup(position("right", edim(20, "px"), "top", edim(1, "%"))),
		}},
		{"top no-repeat", map[string]Value{
			"background_repeat":   etup(etup(estr("no-repeat"), estr("no-repeat"))),
			"background_position": etup(position("left", edim(50, "%"), "top", edim(0, "%"))),
		}},
		{"top right no-repeat", map[string]Value{
			"background_repeat":   etup(etup(estr("no-repeat"), estr("no-repeat"))),
			"background_position": etup(position("left", edim(100, "%"), "top", edim(0, "%"))),
		}},
		{"top right 20px no-repeat", map[string]Value{
			"background_repeat":   etup(etup(estr("no-repeat"), estr("no-repeat"))),
			"background_position": etup(position("right", edim(20, "px"), "top", edim(0, "%"))),
		}},
		{"top 1% right 20px no-repeat", map[string]Value{
			"background_repeat":   etup(etup(estr("no-repeat"), estr("no-repeat"))),
			"background_position": etup(position("right", edim(20, "px"), "top", edim(1, "%"))),
		}},
		{"url(bar) #f00 repeat-y center left fixed", map[string]Value{
			"background_color":      mustParseColor("red"),
			"background_image":      etup(url("https://weasyprint.org/foo/bar")),
			"background_repeat":     etup(etup(estr("no-repeat"), estr("repeat"))),
			"background_attachment": etup(estr("fixed")),
			"background_position":   etup(position("left", edim(0, "%"), "top", edim(50, "%"))),
		}},
		{"#00f 10% 200px", map[string]Value{
			"background_color":    mustParseColor("blue"),
			"background_position": etup(position("left", edim(10, "%"), "top", edim(200, "px"))),
		}},
		{"right 78px fixed", map[string]Value{
			"background_attachment": etup(estr("fixed")),
			"background_position":   etup(position("left", edim(100, "%"), "top", edim(78, "px"))),
		}},
		{"center / cover red", map[string]Value{
			"background_size":     etup(estr("cover")),
			"background_position": etup(position("left", edim(50, "%"), "top", edim(50, "%"))),
			"background_color":    mustParseColor("red"),
		}},
		{"center / auto red", map[string]Value{
			"background_size":     etup(etup(estr("auto"), estr("auto"))),
			"background_position": etup(position("left", edim(50, "%"), "top", edim(50, "%"))),
			"background_color":    mustParseColor("red"),
		}},
		{"center / 42px", map[string]Value{
			"background_size":     etup(etup(edim(42, "px"), estr("auto"))),
			"background_position": etup(position("left", edim(50, "%"), "top", edim(50, "%"))),
		}},
		{"center / 7% 4em", map[string]Value{
			"background_size":     etup(etup(edim(7, "%"), edim(4, "em"))),
			"background_position": etup(position("left", edim(50, "%"), "top", edim(50, "%"))),
		}},
		{"red content-box", map[string]Value{
			"background_color":  mustParseColor("red"),
			"background_origin": etup(estr("content-box")),
			"background_clip":   etup(estr("content-box")),
		}},
		{"red border-box content-box", map[string]Value{
			"background_color":  mustParseColor("red"),
			"background_origin": etup(estr("border-box")),
			"background_clip":   etup(estr("content-box")),
		}},
		{"border-box red", map[string]Value{
			"background_color":  mustParseColor("red"),
			"background_origin": etup(estr("border-box")),
		}},
		{"url(bar) center, no-repeat", map[string]Value{
			"background_color": mustParseColor("transparent"),
			"background_image": etup(url("https://weasyprint.org/foo/bar"), etup(estr("none"), nil)),
			"background_position": etup(
				position("left", edim(50, "%"), "top", edim(50, "%")),
				position("left", edim(0, "%"), "top", edim(0, "%"))),
			"background_repeat": etup(
				etup(estr("repeat"), estr("repeat")), etup(estr("no-repeat"), estr("no-repeat"))),
		}},
	}
	for _, c := range cases {
		css := "background: " + c.rule
		expanded := expandToDict(t, css, "")
		want := maps.Clone(c.want)
		wantColor, ok := want["background_color"]
		if !ok {
			wantColor = InitialValues["background_color"]
		}
		delete(want, "background_color")
		if got := expanded["background_color"]; !Equal(got, wantColor) {
			t.Errorf("%s: background_color: got %s, want %s", css, ValueString(got), ValueString(wantColor))
		}
		delete(expanded, "background_color")
		images, _ := expanded["background_image"].(Tuple)
		nbLayers := len(images)
		for name, value := range want {
			if got := expanded[name]; !Equal(got, value) {
				t.Errorf("%s: %s: got %s, want %s", css, name, ValueString(got), ValueString(value))
			}
			delete(expanded, name)
		}
		for name, value := range expanded {
			initial, _ := InitialValues[name].(Tuple)
			var repeated Tuple
			for range nbLayers {
				repeated = append(repeated, initial...)
			}
			if !Equal(value, repeated) {
				t.Errorf("%s: %s: got %s, want %s", css, name, ValueString(value), ValueString(repeated))
			}
		}
	}

	inherit := map[string]Value{}
	for _, name := range backgroundExpandedNames {
		inherit[strings.ReplaceAll(name, "-", "_")] = estr("inherit")
	}
	assertDict(t, "background: inherit", expandToDict(t, "background: inherit", ""), inherit)
}

func TestExpanderBackgroundInvalid(t *testing.T) {
	runInvalidCases(t, "background: ", []string{
		"red, url(foo)", "10px lipsum", "content-box red content-box",
	})
}

func TestExpanderBorderRadius(t *testing.T) {
	corners := func(tl, tr, br, bl Value) map[string]Value {
		return map[string]Value{
			"border_top_left_radius":     tl,
			"border_top_right_radius":    tr,
			"border_bottom_right_radius": br,
			"border_bottom_left_radius":  bl,
		}
	}
	pair := func(a, b Dimension) Value { return etup(a, b) }
	px1, em2, px3, pc4, pc3, rem4 := edim(1, "px"), edim(2, "em"), edim(3, "px"), edim(4, "%"), edim(3, "%"), edim(4, "rem")
	runExpandCases(t, "border-radius: ", []expandCase{
		{"1px", corners(pair(px1, px1), pair(px1, px1), pair(px1, px1), pair(px1, px1))},
		{"1px 2em", corners(pair(px1, px1), pair(em2, em2), pair(px1, px1), pair(em2, em2))},
		{"1px / 2em", corners(pair(px1, em2), pair(px1, em2), pair(px1, em2), pair(px1, em2))},
		{"1px 3px / 2em 4%", corners(pair(px1, em2), pair(px3, pc4), pair(px1, em2), pair(px3, pc4))},
		{"1px 2em 3%", corners(pair(px1, px1), pair(em2, em2), pair(pc3, pc3), pair(em2, em2))},
		{"1px 2em 3% 4rem", corners(pair(px1, px1), pair(em2, em2), pair(pc3, pc3), pair(rem4, rem4))},
		{"inherit", corners(estr("inherit"), estr("inherit"), estr("inherit"), estr("inherit"))},
	})
}

func TestExpanderBorderRadiusInvalid(t *testing.T) {
	for _, c := range []struct{ rule, message string }{
		{"1px 1px 1px 1px 1px", "1 to 4 token"},
		{"1px 1px 1px 1px 1px / 1px", "1 to 4 token"},
		{"1px / 1px / 1px", `only one "/"`},
		{"12deg", "invalid"},
		{"1px 1px 1px 12deg", "invalid"},
		{"super", "invalid"},
		{"1px, 1px", "invalid"},
		{"1px /", `value after "/"`},
	} {
		assertExpandInvalid(t, "border-radius: "+c.rule, c.message)
	}
}

// borderImageCases returns the cases of test_border_image, with the prefix
// of the names of the properties.
func borderImageCases(prefix string) []expandCase {
	source := etup(estr("url"), estr("https://weasyprint.org/foo/border.png"))
	one := func(v float64, unit string) Value { return etup(edim(v, unit)) }
	return []expandCase{
		{"url(border.png) 27", map[string]Value{
			prefix + "_source": source,
			prefix + "_slice":  one(27, ""),
		}},
		{"url(border.png) 10 / 4 / 2 round stretch", map[string]Value{
			prefix + "_source": source,
			prefix + "_slice":  one(10, ""),
			prefix + "_width":  one(4, ""),
			prefix + "_outset": one(2, ""),
			prefix + "_repeat": etup(estr("round"), estr("stretch")),
		}},
		{"10 // 2", map[string]Value{
			prefix + "_slice":  one(10, ""),
			prefix + "_outset": one(2, ""),
		}},
		{"5.5%", map[string]Value{prefix + "_slice": one(5.5, "%")}},
		{`stretch 2 url("border.png")`, map[string]Value{
			prefix + "_source": source,
			prefix + "_slice":  one(2, ""),
			prefix + "_repeat": etup(estr("stretch")),
		}},
		{"1/2 round", map[string]Value{
			prefix + "_slice":  one(1, ""),
			prefix + "_width":  one(2, ""),
			prefix + "_repeat": etup(estr("round")),
		}},
		{"none", map[string]Value{prefix + "_source": etup(estr("none"), nil)}},
	}
}

// borderImageInvalidCases are the cases of test_border_image_invalid.
var borderImageInvalidCases = []struct{ rule, reason string }{
	{"url(border.png) url(border.png)", "multiple source"},
	{"10 10 10 10 10", "multiple slice"},
	{"1 / 2 / 3 / 4", "invalid"},
	{"/1", "invalid"},
	{"round round round", "invalid"},
	{"-1", "invalid"},
	{"1 repeat 2", "multiple slice"},
	{"1% // 1%", "invalid"},
	{"1 / repeat", "invalid"},
	{"", "no value"},
}

func TestExpanderBorderImage(t *testing.T) {
	runExpandCases(t, "border-image: ", borderImageCases("border_image"))
}

func TestExpanderBorderImageInvalid(t *testing.T) {
	for _, c := range borderImageInvalidCases {
		assertExpandInvalid(t, "border-image: "+c.rule, c.reason)
	}
}

func TestExpanderMaskBorder(t *testing.T) {
	cases := borderImageCases("mask_border")
	source := etup(estr("url"), estr("https://weasyprint.org/foo/border.png"))
	cases = append(cases,
		expandCase{"url(border.png) 27 alpha", map[string]Value{
			"mask_border_source": source,
			"mask_border_slice":  etup(edim(27, "")),
			"mask_border_mode":   estr("alpha"),
		}},
		expandCase{"url(border.png) 27 luminance", map[string]Value{
			"mask_border_source": source,
			"mask_border_slice":  etup(edim(27, "")),
			"mask_border_mode":   estr("luminance"),
		}},
	)
	runExpandCases(t, "mask-border: ", cases)
}

func TestExpanderMaskBorderInvalid(t *testing.T) {
	cases := append(slices.Clone(borderImageInvalidCases),
		struct{ rule, reason string }{"alpha alpha", "multiple mode"},
		struct{ rule, reason string }{"alpha luminance", "multiple mode"},
	)
	for _, c := range cases {
		assertExpandInvalid(t, "mask-border: "+c.rule, c.reason)
	}
}

func TestExpanderFont(t *testing.T) {
	runExpandCases(t, "font: ", []expandCase{
		{"12px My Fancy Font, serif", map[string]Value{
			"font_size":   edim(12, "px"),
			"font_family": etup(estr("My Fancy Font"), estr("serif")),
		}},
		{`small/1.2 "Some Font", serif`, map[string]Value{
			"font_size":   estr("small"),
			"line_height": edim(1.2, ""),
			"font_family": etup(estr("Some Font"), estr("serif")),
		}},
		{"small-caps italic 700 large serif", map[string]Value{
			"font_style":        estr("italic"),
			"font_variant_caps": estr("small-caps"),
			"font_weight":       Int(700),
			"font_size":         estr("large"),
			"font_family":       etup(estr("serif")),
		}},
		{"small-caps condensed normal 700 large serif", map[string]Value{
			"font_stretch":      estr("condensed"),
			"font_variant_caps": estr("small-caps"),
			"font_weight":       Int(700),
			"font_size":         estr("large"),
			"font_family":       etup(estr("serif")),
		}},
	})
}

func TestExpanderFontInvalid(t *testing.T) {
	for _, c := range []struct{ rule, message string }{
		{"menu", "System fonts are not supported"},
		{"12deg My Fancy Font, serif", "invalid"},
		{"12px", "invalid"},
		{"12px/foo serif", "invalid"},
		{`12px "Invalid" family`, "invalid"},
		{"normal normal normal normal normal large serif", "invalid"},
		{"normal small-caps italic 700 condensed large serif", "invalid"},
		{"small-caps italic 700 normal condensed large serif", "invalid"},
		{"small-caps italic 700 condensed normal large serif", "invalid"},
		{"normal normal normal normal", "invalid"},
		{"normal normal normal italic", "invalid"},
		{"italic normal", "invalid"},
		{"normal", "invalid"},
		{"caption", "System fonts"},
	} {
		assertExpandInvalid(t, "font: "+c.rule, c.message)
	}
}

func TestExpanderFontVariant(t *testing.T) {
	all := func(ligatures string) map[string]Value {
		return map[string]Value{
			"font_variant_alternates": estr("normal"),
			"font_variant_caps":       estr("normal"),
			"font_variant_east_asian": estr("normal"),
			"font_variant_ligatures":  estr(ligatures),
			"font_variant_numeric":    estr("normal"),
			"font_variant_position":   estr("normal"),
		}
	}
	runExpandCases(t, "font-variant: ", []expandCase{
		{"normal", all("normal")},
		{"none", all("none")},
		{"historical-forms petite-caps", map[string]Value{
			"font_variant_alternates": estr("historical-forms"),
			"font_variant_caps":       estr("petite-caps"),
		}},
		{"lining-nums contextual small-caps common-ligatures", map[string]Value{
			"font_variant_ligatures": etup(estr("contextual"), estr("common-ligatures")),
			"font_variant_numeric":   etup(estr("lining-nums")),
			"font_variant_caps":      estr("small-caps"),
		}},
		{"jis78 ruby proportional-width", map[string]Value{
			"font_variant_east_asian": etup(estr("jis78"), estr("ruby"), estr("proportional-width")),
		}},
		// CSS2-style font-variant.
		{"small-caps", map[string]Value{"font_variant_caps": estr("small-caps")}},
	})
}

func TestExpanderFontVariantInvalid(t *testing.T) {
	runInvalidCases(t, "font-variant: ", []string{
		"normal normal",
		"2",
		`""`,
		"extra",
		"jis78 jis04",
		"full-width lining-nums ordinal normal",
		"diagonal-fractions stacked-fractions",
		"common-ligatures contextual no-common-ligatures",
		"sub super",
		"slashed-zero slashed-zero",
	})
}

func TestExpanderWordWrap(t *testing.T) {
	runExpandCases(t, "word-wrap: ", []expandCase{
		{"normal", map[string]Value{"overflow_wrap": estr("normal")}},
		{"break-word", map[string]Value{"overflow_wrap": estr("break-word")}},
		{"inherit", map[string]Value{"overflow_wrap": estr("inherit")}},
	})
	runInvalidCases(t, "word-wrap: ", []string{"none", "normal, break-word"})
}

func TestExpanderFlex(t *testing.T) {
	flex := func(grow, shrink float64, basis Value) map[string]Value {
		return map[string]Value{"flex_grow": Float(grow), "flex_shrink": Float(shrink), "flex_basis": basis}
	}
	runExpandCases(t, "flex: ", []expandCase{
		{"auto", flex(1, 1, estr("auto"))},
		{"none", flex(0, 0, estr("auto"))},
		{"10", flex(10, 1, ZeroPixels)},
		{"2 2", flex(2, 2, ZeroPixels)},
		{"2 2 1px", flex(2, 2, edim(1, "px"))},
		{"2 2 auto", flex(2, 2, estr("auto"))},
		{"2 auto", flex(2, 1, estr("auto"))},
		{"0 auto", flex(0, 1, estr("auto"))},
		{"inherit", map[string]Value{
			"flex_grow":   estr("inherit"),
			"flex_shrink": estr("inherit"),
			"flex_basis":  estr("inherit"),
		}},
	})
}

func TestExpanderFlexInvalid(t *testing.T) {
	runInvalidCases(t, "flex: ", []string{"auto 0 0 0", "1px 2px", "auto auto", "auto 1 auto"})
}

func TestExpanderFlexFlow(t *testing.T) {
	runExpandCases(t, "flex-flow: ", []expandCase{
		{"column", map[string]Value{"flex_direction": estr("column")}},
		{"wrap", map[string]Value{"flex_wrap": estr("wrap")}},
		{"wrap column", map[string]Value{"flex_direction": estr("column"), "flex_wrap": estr("wrap")}},
		{"row wrap", map[string]Value{"flex_direction": estr("row"), "flex_wrap": estr("wrap")}},
		{"inherit", map[string]Value{"flex_direction": estr("inherit"), "flex_wrap": estr("inherit")}},
	})
}

func TestExpanderFlexFlowInvalid(t *testing.T) {
	runInvalidCases(t, "flex-flow: ", []string{
		"1px", "wrap 1px", "row row", "wrap nowrap", "column wrap nowrap row",
	})
}

// gridLine builds the value of a grid line: (span, number, ident), with nil
// for None.
func gridLine(span string, number int, ident string) Value {
	t := Tuple{nil, nil, nil}
	if span != "" {
		t[0] = estr(span)
	}
	if number != 0 {
		t[1] = Int(number)
	}
	if ident != "" {
		t[2] = estr(ident)
	}
	return t
}

// withPrefix returns the dictionary with a prefix before each key.
func withPrefix(prefix string, d map[string]Value) map[string]Value {
	result := make(map[string]Value, len(d))
	for k, v := range d {
		result[prefix+k] = v
	}
	return result
}

func TestExpanderGridColumnRow(t *testing.T) {
	auto := estr("auto")
	cases := []expandCase{
		{"auto", map[string]Value{"start": auto, "end": auto}},
		{"auto / auto", map[string]Value{"start": auto, "end": auto}},
		{"4", map[string]Value{"start": gridLine("", 4, ""), "end": auto}},
		{"c", map[string]Value{"start": gridLine("", 0, "c"), "end": gridLine("", 0, "c")}},
		{"4 / -4", map[string]Value{"start": gridLine("", 4, ""), "end": gridLine("", -4, "")}},
		{"c / d", map[string]Value{"start": gridLine("", 0, "c"), "end": gridLine("", 0, "d")}},
		{"ab / cd 4", map[string]Value{"start": gridLine("", 0, "ab"), "end": gridLine("", 4, "cd")}},
		{"ab 2 span", map[string]Value{"start": gridLine("span", 2, "ab"), "end": auto}},
	}
	for _, c := range cases {
		for _, name := range []string{"grid-column", "grid-row"} {
			css := name + ": " + c.rule
			assertDict(t, css, expandToDict(t, css, ""), withPrefix(strings.ReplaceAll(name, "-", "_")+"_", c.want))
		}
	}
}

func TestExpanderGridColumnRowInvalid(t *testing.T) {
	for _, rule := range []string{"auto auto", "4 / 2 / c", "span", "4 / span", "c /", "/4", "col / 2.1"} {
		assertExpandInvalid(t, "grid-column: "+rule, "")
		assertExpandInvalid(t, "grid-row: "+rule, "")
	}
}

func TestExpanderGridArea(t *testing.T) {
	auto := estr("auto")
	allAuto := map[string]Value{"row_start": auto, "row_end": auto, "column_start": auto, "column_end": auto}
	cases := []expandCase{
		{"auto", allAuto},
		{"auto / auto", allAuto},
		{"auto / auto / auto", allAuto},
		{"auto / auto / auto / auto", allAuto},
		{"1/c/2 d/span 2 ab", map[string]Value{
			"row_start": gridLine("", 1, ""), "column_start": gridLine("", 0, "c"),
			"row_end": gridLine("", 2, "d"), "column_end": gridLine("span", 2, "ab"),
		}},
		{"1  /  c", map[string]Value{
			"row_start": gridLine("", 1, ""), "column_start": gridLine("", 0, "c"),
			"row_end": auto, "column_end": gridLine("", 0, "c"),
		}},
		{"a / c 2", map[string]Value{
			"row_start": gridLine("", 0, "a"), "column_start": gridLine("", 2, "c"),
			"row_end": gridLine("", 0, "a"), "column_end": auto,
		}},
		{"a", map[string]Value{
			"row_start": gridLine("", 0, "a"), "row_end": gridLine("", 0, "a"),
			"column_start": gridLine("", 0, "a"), "column_end": gridLine("", 0, "a"),
		}},
		{"span 2", map[string]Value{
			"row_start": gridLine("span", 2, ""), "row_end": auto,
			"column_start": auto, "column_end": auto,
		}},
	}
	for _, c := range cases {
		css := "grid-area: " + c.rule
		assertDict(t, css, expandToDict(t, css, ""), withPrefix("grid_", c.want))
	}
}

func TestExpanderGridAreaInvalid(t *testing.T) {
	runInvalidCases(t, "grid-area: ", []string{
		"auto auto", "auto / auto auto", "4 / 2 / c / d / e", "span", "4 / span",
		"c /", "/4", "c//4", "/", "1 / 2 / 4 / 0.5",
	})
}

// gridTemplateCases are the values of test_grid_template that test_grid
// shares.
var gridTemplateCases = []struct {
	rule          string
	rows, columns Value
}{
	{rule: "none", rows: estr("none"), columns: estr("none")},
	{
		rule:    "subgrid / [outer-edge] 20px [main-start]",
		rows:    etup(estr("subgrid"), etup()),
		columns: etup(etup(estr("outer-edge")), edim(20, "px"), etup(estr("main-start"))),
	},
	{
		rule: "repeat(2, [e] 40px) repeat(5, auto) / subgrid [a] repeat(auto-fill, [b])",
		rows: etup(
			etup(), etup(estr("repeat()"), Int(2), etup(etup(estr("e")), edim(40, "px"), etup())), etup(),
			etup(estr("repeat()"), Int(5), etup(etup(), estr("auto"), etup())), etup()),
		columns: etup(estr("subgrid"), etup(
			etup(estr("a")), etup(estr("repeat()"), estr("auto-fill"), etup(etup(estr("b")))))),
	},
}

func TestExpanderGridTemplate(t *testing.T) {
	for _, c := range gridTemplateCases {
		css := "grid-template: " + c.rule
		assertDict(t, css, expandToDict(t, css, ""), map[string]Value{
			"grid_template_rows":    c.rows,
			"grid_template_columns": c.columns,
			"grid_template_areas":   estr("none"),
		})
	}
}

// gridTemplateInvalid are the rules of test_grid_template_invalid.
var gridTemplateInvalid = []string{
	"none none",
	"auto",
	"subgrid / subgrid / subgrid",
	"[a] 1px [b] / none /",
	"[a] 1px [b] // none",
	"[a] 1px [b] none",
}

func TestExpanderGridTemplateInvalid(t *testing.T) {
	runInvalidCases(t, "grid-template: ", gridTemplateInvalid)
}

func TestExpanderGrid(t *testing.T) {
	auto := etup(estr("auto"))
	for _, c := range gridTemplateCases {
		css := "grid: " + c.rule
		assertDict(t, css, expandToDict(t, css, ""), map[string]Value{
			"grid_template_rows":    c.rows,
			"grid_template_columns": c.columns,
			"grid_template_areas":   estr("none"),
			"grid_auto_rows":        auto,
			"grid_auto_columns":     auto,
			"grid_auto_flow":        etup(estr("row")),
		})
	}
	runExpandCases(t, "grid: ", []expandCase{
		{"auto-flow 1fr / 100px", map[string]Value{
			"grid_template_rows":    estr("none"),
			"grid_template_columns": etup(etup(), edim(100, "px"), etup()),
			"grid_template_areas":   estr("none"),
			"grid_auto_rows":        etup(edim(1, "fr")),
			"grid_auto_columns":     auto,
			"grid_auto_flow":        etup(estr("row")),
		}},
		{"none / dense auto-flow 1fr", map[string]Value{
			"grid_template_rows":    estr("none"),
			"grid_template_columns": estr("none"),
			"grid_template_areas":   estr("none"),
			"grid_auto_rows":        auto,
			"grid_auto_columns":     etup(edim(1, "fr")),
			"grid_auto_flow":        etup(estr("column"), estr("dense")),
		}},
	})
}

func TestExpanderGridInvalid(t *testing.T) {
	runInvalidCases(t, "grid: ", append(slices.Clone(gridTemplateInvalid),
		"none / auto-flow 1fr dense",
		"none / dense 1fr auto-flow",
		"100px auto-flow / none",
		"dense 100px / auto-flow 1fr",
	))
}

func TestExpanderPageBreak(t *testing.T) {
	runExpandCases(t, "", []expandCase{
		{"page-break-after: left", map[string]Value{"break_after": estr("left")}},
		{"page-break-before: always", map[string]Value{"break_before": estr("page")}},
		{"page-break-after: inherit", map[string]Value{"break_after": estr("inherit")}},
		{"page-break-before: inherit", map[string]Value{"break_before": estr("inherit")}},
	})
	runInvalidCases(t, "", []string{"page-break-after: top", "page-break-before: 1px"})
}

func TestExpanderPageBreakInside(t *testing.T) {
	runExpandCases(t, "page-break-inside: ", []expandCase{
		{"avoid", map[string]Value{"break_inside": estr("avoid")}},
		{"inherit", map[string]Value{"break_inside": estr("inherit")}},
	})
	runInvalidCases(t, "page-break-inside: ", []string{"top"})
}

func TestExpanderColumns(t *testing.T) {
	runExpandCases(t, "columns: ", []expandCase{
		{"1em", map[string]Value{"column_width": edim(1, "em"), "column_count": estr("auto")}},
		{"auto", map[string]Value{"column_width": estr("auto"), "column_count": estr("auto")}},
		{"auto auto", map[string]Value{"column_width": estr("auto"), "column_count": estr("auto")}},
	})
	assertExpandInvalid(t, "columns: 1px 2px", "invalid")
	assertExpandInvalid(t, "columns: auto auto auto", "multiple")
}

func TestExpanderLineClamp(t *testing.T) {
	runExpandCases(t, "line-clamp: ", []expandCase{
		{"none", map[string]Value{
			"max_lines": estr("none"), "continue": estr("auto"), "block_ellipsis": estr("none"),
		}},
		{"2", map[string]Value{
			"max_lines": Int(2), "continue": estr("discard"), "block_ellipsis": estr("auto"),
		}},
		{`3 "…"`, map[string]Value{
			"max_lines": Int(3), "continue": estr("discard"),
			"block_ellipsis": etup(estr("string"), estr("…")),
		}},
		{"inherit", map[string]Value{
			"max_lines": estr("inherit"), "continue": estr("inherit"), "block_ellipsis": estr("inherit"),
		}},
	})
	for _, rule := range []string{"none none none", "1px", `0 "…"`, "1px 2px"} {
		assertExpandInvalid(t, "line-clamp: "+rule, "invalid")
	}
}

func TestExpanderTextAlign(t *testing.T) {
	runExpandCases(t, "text-align: ", []expandCase{
		{"start", map[string]Value{"text_align_all": estr("start"), "text_align_last": estr("start")}},
		{"right", map[string]Value{"text_align_all": estr("right"), "text_align_last": estr("right")}},
		{"justify", map[string]Value{"text_align_all": estr("justify"), "text_align_last": estr("start")}},
		{"justify-all", map[string]Value{"text_align_all": estr("justify"), "text_align_last": estr("justify")}},
		{"inherit", map[string]Value{"text_align_all": estr("inherit"), "text_align_last": estr("inherit")}},
	})
	for _, rule := range []string{"none", "start end", "1", "left left", "top", `"right"`, "1px"} {
		assertExpandInvalid(t, "text-align: "+rule, "invalid")
	}
}
