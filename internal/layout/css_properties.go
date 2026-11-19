// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/properties.py.

package layout

import (
	"math"
	"strings"

	"github.com/xo/okage/tinycss2"
)

// InitialValues is INITIAL_VALUES in weasyprint/css/properties.py: the
// initial value of each known property, with the name of the property in
// snake case. A value of a tuple is shared, and no code changes it.
//
// The value of "size" is set by css_computed_values.go, as upstream does.
var InitialValues = map[string]Value{

	// CSS 2.1: https://www.w3.org/TR/CSS21/propidx.html
	"bottom":         String("auto"),
	"caption_side":   String("top"),
	"clear":          String("none"),
	"clip":           Tuple{},
	"color":          mustParseColor("black"),
	"direction":      String("ltr"),
	"display":        Tuple{String("inline"), String("flow")},
	"empty_cells":    String("show"),
	"float":          String("none"),
	"left":           String("auto"),
	"line_height":    String("normal"),
	"margin_top":     Dimension{0, "px"},
	"margin_right":   Dimension{0, "px"},
	"margin_bottom":  Dimension{0, "px"},
	"margin_left":    Dimension{0, "px"},
	"padding_top":    Dimension{0, "px"},
	"padding_right":  Dimension{0, "px"},
	"padding_bottom": Dimension{0, "px"},
	"padding_left":   Dimension{0, "px"},
	"position":       String("static"),
	"right":          String("auto"),
	"table_layout":   String("auto"),
	"top":            String("auto"),
	"unicode_bidi":   String("normal"),
	"vertical_align": String("baseline"),
	"visibility":     String("visible"),
	"z_index":        String("auto"),

	// Backgrounds and Borders 3 (CR): https://www.w3.org/TR/css-backgrounds-3/
	"background_attachment":      Tuple{String("scroll")},
	"background_clip":            Tuple{String("border-box")},
	"background_color":           mustParseColor("transparent"),
	"background_image":           Tuple{Tuple{String("none"), nil}},
	"background_origin":          Tuple{String("padding-box")},
	"background_position":        Tuple{Tuple{String("left"), Dimension{0, "%"}, String("top"), Dimension{0, "%"}}},
	"background_repeat":          Tuple{Tuple{String("repeat"), String("repeat")}},
	"background_size":            Tuple{Tuple{String("auto"), String("auto")}},
	"border_bottom_color":        String("currentcolor"),
	"border_bottom_left_radius":  Tuple{Dimension{0, "px"}, Dimension{0, "px"}},
	"border_bottom_right_radius": Tuple{Dimension{0, "px"}, Dimension{0, "px"}},
	"border_bottom_style":        String("none"),
	"border_bottom_width":        Int(3),
	"border_collapse":            String("separate"),
	"border_left_color":          String("currentcolor"),
	"border_left_style":          String("none"),
	"border_left_width":          Int(3),
	"border_right_color":         String("currentcolor"),
	"border_right_style":         String("none"),
	"border_right_width":         Int(3),
	"border_spacing":             Tuple{Int(0), Int(0)},
	"border_top_color":           String("currentcolor"),
	"border_top_left_radius":     Tuple{Dimension{0, "px"}, Dimension{0, "px"}},
	"border_top_right_radius":    Tuple{Dimension{0, "px"}, Dimension{0, "px"}},
	"border_top_style":           String("none"),
	"border_top_width":           Int(3),
	"border_image_source":        Tuple{String("none"), nil},
	"border_image_slice":         Tuple{Dimension{100, "%"}, Dimension{100, "%"}, Dimension{100, "%"}, Dimension{100, "%"}, nil},
	"border_image_width":         Tuple{Int(1), Int(1), Int(1), Int(1)},
	"border_image_outset":        Tuple{Dimension{0, ""}, Dimension{0, ""}, Dimension{0, ""}, Dimension{0, ""}},
	"border_image_repeat":        Tuple{String("stretch"), String("stretch")},
	"box_shadow":                 Tuple{},
	"mask_border_source":         Tuple{String("none"), nil},
	"mask_border_slice":          Tuple{Dimension{100, "%"}, Dimension{100, "%"}, Dimension{100, "%"}, Dimension{100, "%"}, nil},
	"mask_border_width":          Tuple{String("auto"), String("auto"), String("auto"), String("auto")},
	"mask_border_outset":         Tuple{Dimension{0, ""}, Dimension{0, ""}, Dimension{0, ""}, Dimension{0, ""}},
	"mask_border_repeat":         Tuple{String("stretch"), String("stretch")},
	"mask_border_mode":           String("alpha"),

	// Color Adjustment 1 (CRD): https://www.w3.org/TR/css-color-adjust-1
	"color_scheme": String("normal"),

	// Color 3 (REC): https://www.w3.org/TR/css-color-3/
	"opacity": Int(1),

	// Multi-column Layout (WD): https://www.w3.org/TR/css-multicol-1/
	"column_width":      String("auto"),
	"column_count":      String("auto"),
	"column_rule_color": String("currentcolor"),
	"column_rule_style": String("none"),
	"column_rule_width": String("medium"),
	"column_fill":       String("balance"),
	"column_span":       String("none"),

	// Fonts 3 (REC): https://www.w3.org/TR/css-fonts-3/
	"font_family":             Tuple{String("serif")},
	"font_feature_settings":   String("normal"),
	"font_kerning":            String("auto"),
	"font_language_override":  String("normal"),
	"font_size":               Int(16),
	"font_stretch":            String("normal"),
	"font_style":              String("normal"),
	"font_variant":            String("normal"),
	"font_variant_alternates": String("normal"),
	"font_variant_caps":       String("normal"),
	"font_variant_east_asian": String("normal"),
	"font_variant_ligatures":  String("normal"),
	"font_variant_numeric":    String("normal"),
	"font_variant_position":   String("normal"),
	"font_weight":             Int(400),

	// Fonts 4 (WD): https://www.w3.org/TR/css-fonts-4/
	"font_variation_settings": String("normal"),

	// Fragmentation 3/4 (CR/WD): https://www.w3.org/TR/css-break-4/
	"box_decoration_break": String("slice"),
	"break_after":          String("auto"),
	"break_before":         String("auto"),
	"break_inside":         String("auto"),
	"margin_break":         String("auto"),
	"orphans":              Int(2),
	"widows":               Int(2),

	// Generated Content 3 (WD): https://www.w3.org/TR/css-content-3/
	"bookmark_label":   Tuple{Tuple{String("content"), String("text")}},
	"bookmark_level":   String("none"),
	"bookmark_state":   String("open"),
	"content":          String("normal"),
	"footnote_display": String("block"),
	"footnote_policy":  String("auto"),
	"quotes":           String("auto"),
	"string_set":       String("none"),

	// Images 3/4 (CR/WD): https://www.w3.org/TR/css-images-4/
	"image_resolution":  Int(1),
	"image_rendering":   String("auto"),
	"image_orientation": String("from-image"),
	"object_fit":        String("fill"),
	"object_position":   Tuple{Tuple{String("left"), Dimension{50, "%"}, String("top"), Dimension{50, "%"}}},

	// Paged Media 3 (WD): https://www.w3.org/TR/css-page-3/
	"size":         nil,
	"page":         String("auto"),
	"bleed_left":   String("auto"),
	"bleed_right":  String("auto"),
	"bleed_top":    String("auto"),
	"bleed_bottom": String("auto"),
	"marks":        Tuple{},

	// Text 3/4 (WD/WD): https://www.w3.org/TR/css-text-4/
	"hyphenate_character":   String("‐"),
	"hyphenate_limit_chars": Tuple{Int(5), Int(2), Int(2)},
	"hyphenate_limit_zone":  Dimension{0, "px"},
	"hyphens":               String("manual"),
	"letter_spacing":        String("normal"),
	"tab_size":              Int(8),
	"tab_stops":             String("none"),
	"text_autospace":        String("none"),
	"text_align_all":        String("start"),
	"text_align_last":       String("auto"),
	"text_indent":           Dimension{0, "px"},
	"text_transform":        String("none"),
	"white_space":           String("normal"),
	"word_break":            String("normal"),
	"word_spacing":          Int(0),

	// Transforms 1 (CR): https://www.w3.org/TR/css-transforms-1/
	"transform_origin": Tuple{Dimension{50, "%"}, Dimension{50, "%"}},
	"transform":        Tuple{},

	// User Interface 3/4 (REC/WD): https://www.w3.org/TR/css-ui-4/
	"appearance":     String("none"),
	"outline_color":  String("currentcolor"),
	"outline_style":  String("none"),
	"outline_width":  Int(3),
	"outline_offset": Int(0),

	// Sizing 3 (WD): https://www.w3.org/TR/css-sizing-3/
	"box_sizing": String("content-box"),
	"height":     String("auto"),
	"max_height": Dimension{math.Inf(1), "px"},
	"max_width":  Dimension{math.Inf(1), "px"},
	"min_height": String("auto"),
	"min_width":  String("auto"),
	"width":      String("auto"),

	// Logical Properties and Values 1 (WD): https://www.w3.org/TR/css-logical-1/
	"block_size":                String("auto"),
	"inline_size":               String("auto"),
	"max_block_size":            Dimension{math.Inf(1), "px"},
	"max_inline_size":           Dimension{math.Inf(1), "px"},
	"min_block_size":            String("auto"),
	"min_inline_size":           String("auto"),
	"margin_block_start":        Int(0),
	"margin_inline_start":       Int(0),
	"margin_block_end":          Int(0),
	"margin_inline_end":         Int(0),
	"padding_block_start":       Int(0),
	"padding_inline_start":      Int(0),
	"padding_block_end":         Int(0),
	"padding_inline_end":        Int(0),
	"inset_block_start":         String("auto"),
	"inset_inline_start":        String("auto"),
	"inset_block_end":           String("auto"),
	"inset_inline_end":          String("auto"),
	"border_block_start_width":  Int(3),
	"border_inline_start_width": Int(3),
	"border_block_end_width":    Int(3),
	"border_inline_end_width":   Int(3),
	"border_block_start_style":  String("none"),
	"border_inline_start_style": String("none"),
	"border_block_end_style":    String("none"),
	"border_inline_end_style":   String("none"),
	"border_block_start_color":  String("currentcolor"),
	"border_inline_start_color": String("currentcolor"),
	"border_block_end_color":    String("currentcolor"),
	"border_inline_end_color":   String("currentcolor"),
	"border_start_start_radius": Int(0),
	"border_start_end_radius":   Int(0),
	"border_end_start_radius":   Int(0),
	"border_end_end_radius":     Int(0),

	// Flexible Box Layout Module 1 (CR): https://www.w3.org/TR/css-flexbox-1/
	"flex_basis":     String("auto"),
	"flex_direction": String("row"),
	"flex_grow":      Int(0),
	"flex_shrink":    Int(1),
	"flex_wrap":      String("nowrap"),

	// Grid Layout Module Level 2 (CR): https://www.w3.org/TR/css-grid-2/
	"grid_auto_columns":     Tuple{String("auto")},
	"grid_auto_flow":        Tuple{String("row")},
	"grid_auto_rows":        Tuple{String("auto")},
	"grid_template_areas":   String("none"),
	"grid_template_columns": String("none"),
	"grid_template_rows":    String("none"),
	"grid_row_start":        String("auto"),
	"grid_column_start":     String("auto"),
	"grid_row_end":          String("auto"),
	"grid_column_end":       String("auto"),

	// CSS Box Alignment Module Level 3 (WD): https://www.w3.org/TR/css-align-3/
	"align_content":   Tuple{String("normal")},
	"align_items":     Tuple{String("normal")},
	"align_self":      Tuple{String("auto")},
	"justify_content": Tuple{String("normal")},
	"justify_items":   Tuple{String("normal")},
	"justify_self":    Tuple{String("auto")},
	"order":           Int(0),
	"column_gap":      String("normal"),
	"row_gap":         String("normal"),

	// Text Decoration Module 3/4 (CR/WD): https://www.w3.org/TR/css-text-decor-4/
	"text_decoration_line":      String("none"),
	"text_decoration_color":     String("currentcolor"),
	"text_decoration_style":     String("solid"),
	"text_decoration_thickness": String("auto"),
	"text_underline_offset":     String("auto"),

	// Overflow Module 3/4 (WD): https://www.w3.org/TR/css-overflow-4/
	"block_ellipsis": String("none"),
	"continue":       String("auto"),
	"max_lines":      String("none"),
	"overflow":       String("visible"),
	"overflow_wrap":  String("normal"),
	"text_overflow":  String("clip"),

	// Lists Module 3 (WD): https://drafts.csswg.org/css-lists-3/
	// Means 'none', but allow `display: list-item` to increment the
	// list-item counter. If we ever have a way for authors to query
	// computed values (JavaScript?), this value should serialize to 'none'.
	"counter_increment":   String("auto"),
	"counter_reset":       Tuple{},
	"counter_set":         Tuple{},
	"list_style_image":    Tuple{String("none"), nil},
	"list_style_position": String("outside"),
	"list_style_type":     String("disc"),

	// Proprietary
	"anchor": nil,
	"lang":   nil,

	// The properties that ivy adds, with the prefix -ivy-.
	"margin_box_fit": String("none"),
	"static_top":     String("auto"),
	"page_overflow":  String("none"),
	"line_numbers":   String("none"),
	"line_multiple":  String("none"),
	"page_top_space": String("keep"),
	"float_position": String("auto"),
}

// knownProperties is KNOWN_PROPERTIES in weasyprint/css/properties.py: the
// names of the known properties, in kebab case.
var knownProperties = func() map[string]bool {
	known := make(map[string]bool, len(InitialValues))
	for name := range InitialValues {
		known[strings.ReplaceAll(name, "_", "-")] = true
	}
	return known
}()

// Inherited is INHERITED in weasyprint/css/properties.py: the inherited
// properties.
//
// Shorthand properties are not listed, as they are handled before the
// inheritance. Inherited values that do not apply to print are not listed.
// text_decoration is not really inherited, see
// https://www.w3.org/TR/CSS2/text.html#propdef-text-decoration
var Inherited = map[string]bool{
	"block_ellipsis":          true,
	"border_collapse":         true,
	"border_spacing":          true,
	"caption_side":            true,
	"color":                   true,
	"color_scheme":            true,
	"direction":               true,
	"empty_cells":             true,
	"font_family":             true,
	"font_feature_settings":   true,
	"font_kerning":            true,
	"font_language_override":  true,
	"font_size":               true,
	"font_stretch":            true,
	"font_style":              true,
	"font_variant":            true,
	"font_variant_alternates": true,
	"font_variant_caps":       true,
	"font_variant_east_asian": true,
	"font_variant_ligatures":  true,
	"font_variant_numeric":    true,
	"font_variant_position":   true,
	"font_variation_settings": true,
	"font_weight":             true,
	"hyphenate_character":     true,
	"hyphenate_limit_chars":   true,
	"hyphenate_limit_zone":    true,
	"hyphens":                 true,
	"image_rendering":         true,
	"image_resolution":        true,
	"lang":                    true,
	"letter_spacing":          true,
	"line_height":             true,
	"line_multiple":           true,
	"line_numbers":            true,
	"list_style_image":        true,
	"list_style_position":     true,
	"list_style_type":         true,
	"orphans":                 true,
	"overflow_wrap":           true,
	"page_overflow":           true,
	"page_top_space":          true,
	"quotes":                  true,
	"tab_size":                true,
	"tab_stops":               true,
	"text_autospace":          true,
	"text_align_all":          true,
	"text_align_last":         true,
	"text_indent":             true,
	"text_transform":          true,
	"text_underline_offset":   true,
	"visibility":              true,
	"white_space":             true,
	"widows":                  true,
	"word_break":              true,
	"word_spacing":            true,
}

// TableWrapperBoxProperties is TABLE_WRAPPER_BOX_PROPERTIES in
// weasyprint/css/properties.py, the properties of a table that go to its
// wrapper box. See https://www.w3.org/TR/CSS21/tables.html#model. Only the
// properties that are not inherited are listed.
//
// Port: upstream has no -ivy-float-position. It goes to the wrapper, which
// floats (D60).
var TableWrapperBoxProperties = map[string]bool{
	"bottom":              true,
	"break_after":         true,
	"break_before":        true,
	"clear":               true,
	"counter_increment":   true,
	"counter_reset":       true,
	"counter_set":         true,
	"float":               true,
	"float_position":      true,
	"left":                true,
	"margin_block_end":    true,
	"margin_block_start":  true,
	"margin_bottom":       true,
	"margin_inline_end":   true,
	"margin_inline_start": true,
	"margin_left":         true,
	"margin_right":        true,
	"margin_top":          true,
	"opacity":             true,
	"overflow":            true,
	"position":            true,
	"right":               true,
	"top":                 true,
	"transform":           true,
	"transform_origin":    true,
	"vertical_align":      true,
	"z_index":             true,
}

// initialNotComputed is INITIAL_NOT_COMPUTED in
// weasyprint/css/properties.py: the properties whose initial value is not
// always the same when computed.
var initialNotComputed = map[string]bool{
	"bleed_bottom":              true,
	"bleed_left":                true,
	"bleed_right":               true,
	"bleed_top":                 true,
	"border_block_end_width":    true,
	"border_block_start_width":  true,
	"border_bottom_width":       true,
	"border_inline_end_width":   true,
	"border_inline_start_width": true,
	"border_left_width":         true,
	"border_right_width":        true,
	"border_top_width":          true,
	"column_gap":                true,
	"column_rule_width":         true,
	"display":                   true,
	"outline_width":             true,
}

// mustParseColor is parse_color of tinycss2 color5.py for a color keyword
// that is known to be valid.
func mustParseColor(s string) Value {
	color, _ := tinycss2.ParseColor5(tinycss2.ParseOneComponentValue(s))
	return Color{color}
}
