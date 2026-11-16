// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/text/fonts.py: the fonts of a document, with
// the fonts of the @font-face rules of its style sheets.
//
// Port: upstream keeps the fonts in Fontconfig and Pango. The port keeps
// them in a FontDatabase of okage, which selects a face for a list of
// families, a weight, a style and a stretch, and resolves the generic
// families (okage D46). The fallback for a character that the fonts of the
// style do not draw is the fallback selector of okage (okage D39). go-text
// parses the faces, for the shaping and the metrics (D7). The other
// differences are marked with Port: in the code.

package layout

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"github.com/xo/okage/svg"
	"github.com/xo/okage/tinycss2"
	"github.com/xo/okage/webfont"

	"github.com/xo/ivy"
	ivyfont "github.com/xo/ivy/internal/font"
)

// FontConfiguration is FontConfiguration in weasyprint/text/fonts.py: the
// fonts of a document. It holds the fonts that the program gives, such as
// the fonts of the system, and the fonts of the @font-face rules of the
// style sheets of the document.
//
// A FontConfiguration is both the FontConfig and the TextMetrics of the
// cascade: give it to ComputedStylesOptions as both, so that the @font-face
// rules load their fonts into it, and the units of the fonts measure them.
//
// It is not safe for concurrent use, as the styles that use it are not.
type FontConfiguration struct {
	// db holds the faces. It is a copy of the database of the program, so
	// that the fonts of a document do not go into the database of another
	// one.
	db     *svg.FontDatabase
	limits ivy.Limits
	log    *logger
	// first is the number of the faces at the start of db that come before
	// the other faces of the program, and base is the number of the faces
	// of the program. The faces from base on are the faces of the @font-face
	// rules.
	first, base int

	// rules holds the @font-face rules that are loaded, by the key of their
	// descriptors. It is the folder of the fonts of upstream, whose files
	// are named by the digest of the descriptors.
	rules map[string]bool
	// fontData counts the bytes of the fonts that the rules loaded.
	fontData int64
	// faceRules holds the unicode-range and the features of the faces that
	// an @font-face rule loaded.
	faceRules map[svg.FontID]*faceRule
	// faces holds the faces of go-text, by font and variations.
	faces map[faceKey]*font.Face
	// fonts holds the fonts of go-text that the faces share, by font.
	fonts map[svg.FontID]*font.Font
	// chains holds the faces of the families of a style, by the key of the
	// font of the style.
	chains map[string][]svg.FontID
	// fallbacks holds the fallback faces of a character, by the first face
	// of a chain and the character. A missing face is -1.
	fallbacks map[fallbackKey]int
	// shaper shapes the text.
	shaper shaping.HarfbuzzShaper
	// segmenter splits the text into runs.
	segmenter shaping.Segmenter
	// shaped caches the shaped texts, and shapedBytes counts the bytes of
	// their texts.
	shaped      map[shapeKey]*shapedText
	shapedBytes int

	// strutLayouts is the cache strut_layouts of upstream.
	strutLayouts map[string][2]float64
}

// faceRule holds what an @font-face rule gives a face: its unicode-range
// and its features.
type faceRule struct {
	ranges   [][2]rune
	features []shaping.FontFeature
}

// faceKey is the key of a face of go-text: a font and its variations.
type faceKey struct {
	id         svg.FontID
	variations string
}

// fallbackKey is the key of a fallback face.
type fallbackKey struct {
	first svg.FontID
	r     rune
}

// FontConfigurationOptions holds the options of NewFontConfiguration.
type FontConfigurationOptions struct {
	// Limits bound the fonts that the @font-face rules load. A zero field
	// takes its default.
	Limits ivy.Limits
	// Logger receives the log of the fonts, or nil.
	Logger *slog.Logger
	// FirstFaces is the number of the faces at the start of db that come
	// before the other faces of db, such as the fonts of ivy.Config.Fonts.
	// For a family that font-family names, these faces also give the
	// substitute of ivy/internal/font.Substitute, such as Carlito for
	// Calibri, before the other faces of db give the family (D24).
	FirstFaces int

	// log is the logger of a document, for the configurations that the
	// package makes.
	log *logger
}

// NewFontConfiguration is FontConfiguration.__init__ in
// weasyprint/text/fonts.py. db holds the fonts of the program, such as the
// fonts of the system that FontDatabase.LoadOSFonts loads, or nil for no
// font. The configuration copies db, so that the fonts of the document do
// not change it.
//
// Port: upstream reads the fonts of the system from Fontconfig. ivy does
// not read the system, so the program gives the fonts (D1).
func NewFontConfiguration(db *svg.FontDatabase, opts FontConfigurationOptions) *FontConfiguration {
	if db == nil {
		db = svg.NewFontDatabase()
	}
	log := opts.log
	if log == nil {
		log = newLogger(opts.Logger)
	}
	return &FontConfiguration{
		db:           db.Clone(),
		limits:       opts.Limits.WithDefaults(),
		log:          log,
		first:        min(max(opts.FirstFaces, 0), db.Len()),
		base:         db.Len(),
		rules:        map[string]bool{},
		faceRules:    map[svg.FontID]*faceRule{},
		faces:        map[faceKey]*font.Face{},
		fonts:        map[svg.FontID]*font.Font{},
		chains:       map[string][]svg.FontID{},
		fallbacks:    map[fallbackKey]int{},
		shaped:       map[shapeKey]*shapedText{},
		strutLayouts: map[string][2]float64{},
	}
}

// FontDatabase returns the fonts of the configuration, with the fonts of
// the @font-face rules. The drawing reads the data of the faces of the
// glyph runs from it.
func (fc *FontConfiguration) FontDatabase() *svg.FontDatabase {
	return fc.db
}

// noFontsKey is the key of the configuration without fonts in the cache of
// a StyleContext.
type noFontsKey struct{}

// fontConfigOf returns the FontConfiguration of a style: its FontConfig or
// its TextMetrics. A style without one gets a configuration without fonts,
// which measures the text with the default metrics, kept in the cache of
// its context.
//
// Port: upstream reads style.font_config.
func fontConfigOf(style *Style) *FontConfiguration {
	ctx := style.Context()
	if ctx == nil {
		return NewFontConfiguration(nil, FontConfigurationOptions{})
	}
	if fc, ok := ctx.FontConfig.(*FontConfiguration); ok {
		return fc
	}
	if fc, ok := ctx.Metrics.(*FontConfiguration); ok {
		return fc
	}
	if ctx.Cache == nil {
		ctx.Cache = map[any]any{}
	}
	if fc, ok := ctx.Cache[noFontsKey{}].(*FontConfiguration); ok {
		return fc
	}
	fc := NewFontConfiguration(nil, FontConfigurationOptions{log: ctx.log})
	ctx.Cache[noFontsKey{}] = fc
	return fc
}

// AddFontFace is FontConfiguration.add_font_face in
// weasyprint/text/fonts.py. It adds the font of an @font-face rule to the
// configuration. It satisfies FontConfig.
//
// Port: upstream writes the font into a temporary folder and a
// configuration of Fontconfig. The port loads the font into the database,
// with the family, the style, the weight and the stretch of the rule. A
// font is read with the fetcher of the document, at most Limits.MaxMemory
// bytes in all.
func (fc *FontConfiguration) AddFontFace(ruleDescriptors map[string]Value, urlFetcher *URLFetcher) {
	defer func() {
		// Port: a panic in the reader of a font is a font that cannot be
		// loaded.
		if r := recover(); r != nil {
			fc.log.debug("Failed to load font: %v", r)
		}
	}()

	// Define the key of the font, depending on the rule descriptors.
	configKey := descriptorsKey(ruleDescriptors)
	configDigest := fmt.Sprintf("%x", md5.Sum([]byte(configKey)))
	if fc.rules[configDigest] {
		// Font already exists, we have nothing more to do.
		return
	}

	family := str(ruleDescriptors["font_family"])
	src, _ := ruleDescriptors["src"].(Tuple)
	// Try values in "src" descriptor until one works.
	for _, item := range src {
		pair, ok := item.(Tuple)
		if !ok || len(pair) != 2 {
			continue
		}
		fontType, u := str(pair[0]), pair[1]
		// Abort if font URL is broken.
		if u == nil || fontType == "internal" {
			continue
		}

		var ids []svg.FontID
		if fontType == "local" {
			// Try to find a font installed on the system that matches
			// descriptors.
			id, ok := fc.localFont(str(u))
			if !ok {
				fc.log.debug("Failed to load local font %q", str(u))
				continue
			}
			// Port: upstream registers the file of the local font again,
			// with the family of the rule. The port copies its face.
			info, _ := fc.db.Face(id)
			ids = []svg.FontID{fc.db.PushFaceInfo(*info)}
		} else {
			url := str(u)
			// Get font content.
			response, err := urlFetcher.Fetch(url)
			if err != nil {
				fc.log.debug("Failed to load font at %q (%v)", url, err)
				continue
			}
			data := response.Body
			if fc.fontData+int64(len(data)) > fc.limits.MaxMemory {
				// Port: upstream has no limit.
				fc.log.debug("Failed to load font at %q (%v)", url, ivy.ErrLimit)
				continue
			}
			// Decode woff, woff2 and eot fonts.
			switch webfont.DetectFormat(data) {
			case webfont.FormatWOFF, webfont.FormatWOFF2, webfont.FormatEOT:
				data, err = webfont.Convert(data)
				if err != nil {
					fc.log.debug("Failed to handle woff font at %q (%v)", url, err)
					continue
				}
			}
			ids = fc.db.LoadFontData(data)
			if len(ids) == 0 {
				fc.log.debug("Failed to load font at %q", url)
				continue
			}
			fc.fontData += int64(len(data))
		}

		rule := &faceRule{features: fontFeaturesOfRule(ruleDescriptors)}
		if ranges, ok := ruleDescriptors["unicode_range"].(Tuple); ok {
			for _, r := range ranges {
				if t, ok := r.(Token); ok {
					if u, ok := t.Node.(*tinycss2.UnicodeRangeToken); ok {
						rule.ranges = append(rule.ranges, [2]rune{rune(u.Start), rune(u.End)})
					}
				}
			}
		}
		for _, id := range ids {
			info, _ := fc.db.Face(id)
			// Prepend the family of the rule, as upstream does: replacing
			// the font family breaks Pango, see #2510.
			info.Families = append([]svg.FontFamilyName{{Name: family, Language: svg.FontLanguageEnglishUnitedStates}}, info.Families...)
			if style, ok := ruleDescriptors["font_style"]; ok {
				info.Style = fontStyle(str(style))
			}
			if weight, ok := ruleDescriptors["font_weight"]; ok {
				info.Weight = uint16(fontWeight(weight))
			}
			if stretch, ok := ruleDescriptors["font_stretch"]; ok {
				info.Stretch = fontStretch(str(stretch))
			}
			fc.faceRules[id] = rule
		}
		fc.rules[configDigest] = true
		// The new faces change the faces of the families.
		clear(fc.chains)
		clear(fc.fallbacks)
		clear(fc.shaped)
		fc.shapedBytes = 0
		clear(fc.strutLayouts)
		return
	}
	fc.log.warning("Font-face %q cannot be loaded", family)
}

// descriptorsKey is str(rule_descriptors) of upstream: a key that is the
// same for the same descriptors.
func descriptorsKey(descriptors map[string]Value) string {
	keys := make([]string, 0, len(descriptors))
	for key := range descriptors {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "%s=%s;", key, ValueString(descriptors[key]))
	}
	return b.String()
}

// localFont returns the face whose full name or PostScript name is name, as
// Fontconfig matches the fullname and the postscriptname of a local font.
func (fc *FontConfiguration) localFont(name string) (svg.FontID, bool) {
	lower := strings.ToLower(name)
	for face := range fc.db.Faces() {
		if strings.ToLower(face.PostScriptName) == lower {
			return face.ID, true
		}
		if full, ok := fc.db.FaceName(face.ID, 4); ok && strings.ToLower(full) == lower {
			return face.ID, true
		}
	}
	return 0, false
}

// fontFeaturesOfRule returns the features that the descriptors
// font-variant and font-feature-settings of an @font-face rule set: steps 2
// and 3 of https://www.w3.org/TR/css-fonts-3/#feature-precedence.
func fontFeaturesOfRule(descriptors map[string]Value) []shaping.FontFeature {
	args := fontFeatureArgs{
		kerning: String("normal"), ligatures: String("normal"), position: String("normal"),
		caps: String("normal"), numeric: String("normal"), alternates: String("normal"),
		eastAsian: String("normal"), settings: String("normal"),
	}
	if variants, ok := descriptors["font_variant"].(Tuple); ok {
		for _, rules := range variants {
			// Each item is the list of the declarations of the expander of
			// font-variant: ((name, value),).
			r, ok := rules.(Tuple)
			if !ok || len(r) == 0 {
				continue
			}
			pair, ok := r[0].(Tuple)
			if !ok || len(pair) != 2 {
				continue
			}
			switch strings.ReplaceAll(str(pair[0]), "-", "_") {
			case "font_variant_ligatures":
				args.ligatures = pair[1]
			case "font_variant_position":
				args.position = pair[1]
			case "font_variant_caps":
				args.caps = pair[1]
			case "font_variant_numeric":
				args.numeric = pair[1]
			case "font_variant_alternates":
				args.alternates = pair[1]
			case "font_variant_east_asian":
				args.eastAsian = pair[1]
			}
		}
	}
	if settings, ok := descriptors["font_feature_settings"]; ok {
		args.settings = settings
	}
	return toShapingFeatures(fontFeatures(args))
}

// fontFeatureArgs are the arguments of font_features in
// weasyprint/text/fonts.py.
type fontFeatureArgs struct {
	kerning, ligatures, position, caps, numeric, alternates, eastAsian, settings Value
}

// fontFeature is a feature of font_features: an OpenType tag and its
// value.
type fontFeature struct {
	tag   string
	value int
}

// fontFeatures is font_features in weasyprint/text/fonts.py. It returns the
// font features of the properties of a style, in the order of upstream. See
// https://www.w3.org/TR/css-fonts-3/#feature-precedence.
func fontFeatures(args fontFeatureArgs) []fontFeature {
	var features []fontFeature
	set := func(key string, value int) {
		for i := range features {
			if features[i].tag == key {
				features[i].value = value
				return
			}
		}
		features = append(features, fontFeature{key, value})
	}

	// Step 1: getting the default, we rely on Pango for this.
	// Step 2: @font-face font-variant, done in fonts.add_font_face.
	// Step 3: @font-face font-feature-settings, done in fonts.add_font_face.

	// Step 4: font-variant and OpenType features.

	if !isStr(args.kerning, "auto") {
		if isStr(args.kerning, "normal") {
			set("kern", 1)
		} else {
			set("kern", 0)
		}
	}

	switch {
	case isStr(args.ligatures, "none"):
		for _, key := range ligatureKeysOrder {
			for _, tag := range ligatureKeys[key] {
				set(tag, 0)
			}
		}
	case !isStr(args.ligatures, "normal"):
		for _, ligatureType := range valueStrings(args.ligatures) {
			value := 1
			if strings.HasPrefix(ligatureType, "no-") {
				value = 0
				ligatureType = ligatureType[3:]
			}
			for _, key := range ligatureKeys[ligatureType] {
				set(key, value)
			}
		}
	}

	switch {
	case isStr(args.position, "sub"):
		// TODO: the specification asks for additional checks
		// https://www.w3.org/TR/css-fonts-3/#font-variant-position-prop
		set("subs", 1)
	case isStr(args.position, "super"):
		set("sups", 1)
	}

	if !isStr(args.caps, "normal") {
		// TODO: the specification asks for additional checks
		// https://www.w3.org/TR/css-fonts-3/#font-variant-caps-prop
		for _, key := range capsKeys[str(args.caps)] {
			set(key, 1)
		}
	}

	if !isStr(args.numeric, "normal") {
		for _, key := range valueStrings(args.numeric) {
			if tag, ok := numericKeys[key]; ok {
				set(tag, 1)
			}
		}
	}

	if !isStr(args.alternates, "normal") {
		// TODO: support other values
		// See https://drafts.csswg.org/css-fonts/#font-variant-alternates-prop
		if isStr(args.alternates, "historical-forms") {
			set("hist", 1)
		}
	}

	if !isStr(args.eastAsian, "normal") {
		for _, key := range valueStrings(args.eastAsian) {
			if tag, ok := eastAsianKeys[key]; ok {
				set(tag, 1)
			}
		}
	}

	// Step 5: incompatible non-OpenType features, already handled by Pango.

	// Step 6: font-feature-settings.

	if t, ok := args.settings.(Tuple); ok {
		for _, item := range t {
			if pair, ok := item.(Tuple); ok && len(pair) == 2 {
				set(str(pair[0]), int(num(pair[1])))
			}
		}
	}

	return features
}

// valueStrings returns the strings of a Tuple or of a StringSet.
func valueStrings(v Value) []string {
	switch v := v.(type) {
	case Tuple:
		strs := make([]string, 0, len(v))
		for _, item := range v {
			strs = append(strs, str(item))
		}
		return strs
	case StringSet:
		return v
	case String:
		return []string{string(v)}
	}
	return nil
}

// toShapingFeatures converts the features to the features of go-text. A
// tag that is not 4 characters long is ignored.
func toShapingFeatures(features []fontFeature) []shaping.FontFeature {
	out := make([]shaping.FontFeature, 0, len(features))
	for _, f := range features {
		if len(f.tag) != 4 {
			continue
		}
		out = append(out, shaping.FontFeature{Tag: ot.MustNewTag(f.tag), Value: uint32(max(f.value, 0))})
	}
	return out
}

// styleFeatures returns the font features of a style, as Layout.setup of
// weasyprint/text/line_break.py gives them.
func styleFeatures(style *Style) []shaping.FontFeature {
	return toShapingFeatures(fontFeatures(fontFeatureArgs{
		kerning:    style.Get("font_kerning"),
		ligatures:  style.Get("font_variant_ligatures"),
		position:   style.Get("font_variant_position"),
		caps:       style.Get("font_variant_caps"),
		numeric:    style.Get("font_variant_numeric"),
		alternates: style.Get("font_variant_alternates"),
		eastAsian:  style.Get("font_variant_east_asian"),
		settings:   style.Get("font_feature_settings"),
	}))
}

// fontStyle returns the style of okage of a value of font-style.
func fontStyle(style string) svg.FontStyle {
	switch style {
	case "italic":
		return svg.FontStyleItalic
	case "oblique":
		return svg.FontStyleOblique
	}
	return svg.FontStyleNormal
}

// fontWeight returns the weight of a value of font-weight, from 1 to 1000.
func fontWeight(weight Value) int {
	switch {
	case isStr(weight, "normal"):
		return 400
	case isStr(weight, "bold"):
		return 700
	}
	return min(max(int(num(weight)), 1), 1000)
}

// fontStretches are the keywords of font-stretch, in the order of the
// stretches of okage.
var fontStretches = []string{
	"ultra-condensed", "extra-condensed", "condensed", "semi-condensed",
	"normal", "semi-expanded", "expanded", "extra-expanded", "ultra-expanded",
}

// fontStretch returns the stretch of okage of a keyword of font-stretch.
func fontStretch(stretch string) svg.FontStretch {
	if i := slices.Index(fontStretches, stretch); i >= 0 {
		return svg.FontStretchUltraCondensed + svg.FontStretch(i)
	}
	return svg.FontStretchNormal
}

// genericFamilies maps the generic families of CSS, and the aliases that
// Fontconfig knows, to the generic families of okage.
var genericFamilies = map[string]svg.FontFamily{
	"serif":         {Kind: svg.FontFamilySerif},
	"sans-serif":    {Kind: svg.FontFamilySansSerif},
	"sans":          {Kind: svg.FontFamilySansSerif},
	"system-ui":     {Kind: svg.FontFamilySansSerif},
	"ui-sans-serif": {Kind: svg.FontFamilySansSerif},
	"ui-serif":      {Kind: svg.FontFamilySerif},
	"monospace":     {Kind: svg.FontFamilyMonospace},
	"mono":          {Kind: svg.FontFamilyMonospace},
	"ui-monospace":  {Kind: svg.FontFamilyMonospace},
	"cursive":       {Kind: svg.FontFamilyCursive},
	"fantasy":       {Kind: svg.FontFamilyFantasy},
}

// fontKey is _font_style_cache_key in weasyprint/text/line_break.py,
// without the size: the key of the font of a style.
func fontKey(style *Style) string {
	var b strings.Builder
	for _, name := range []string{
		"font_family", "font_style", "font_stretch", "font_weight",
		"font_variant_ligatures", "font_variant_position",
		"font_variant_caps", "font_variant_numeric",
		"font_variant_alternates", "font_variant_east_asian",
		"font_feature_settings", "font_variation_settings",
		"font_language_override", "lang",
	} {
		b.WriteString(ValueString(style.Get(name)))
		b.WriteByte(0)
	}
	return b.String()
}

// maxFamilies bounds the families of font-family that select the faces.
const maxFamilies = 64

// chain returns the faces of the families of a style, in the order of the
// families, with the default font of the database at the end. It is the
// font description of get_font_description in weasyprint/text/fonts.py.
//
// Port: Fontconfig sorts all the fonts of the system for a pattern. The
// port takes the best face of each family of font-family, with the query of
// okage, and then the default serif family, as the font selector of okage
// does. The query looks for each family in the faces in the order of
// FontConfiguration.query.
func (fc *FontConfiguration) chain(style *Style) []svg.FontID {
	key := fontKey(style)
	if ids, ok := fc.chains[key]; ok {
		return ids
	}
	weight := uint16(fontWeight(style.Get("font_weight")))
	fstyle := fontStyle(str(style.Get("font_style")))
	stretch := fontStretch(str(style.Get("font_stretch")))
	var families []svg.FontFamily
	names := valueStrings(style.Get("font_family"))
	// Port: upstream gives all the families to Fontconfig. The port reads
	// at most maxFamilies of them.
	names = names[:min(len(names), maxFamilies)]
	for _, name := range names {
		if generic, ok := genericFamilies[strings.ToLower(name)]; ok {
			families = append(families, generic)
		} else {
			families = append(families, svg.FontFamily{Kind: svg.FontFamilyNamed, Name: name})
		}
	}
	// Use the default font as fallback.
	families = append(families, svg.FontFamily{Kind: svg.FontFamilySerif})
	var ids []svg.FontID
	query := svg.FontQuery{Weight: weight, Stretch: stretch, Style: fstyle}
	for _, family := range families {
		id, ok := fc.query(family, query)
		if ok && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		// Port: Fontconfig always gives a font. Take the first face.
		for face := range fc.db.Faces() {
			ids = append(ids, face.ID)
			break
		}
	}
	fc.chains[key] = ids
	return ids
}

// query returns the best face of a family for the weight, the stretch and
// the style of q. It looks for the family in these faces, in this order:
//
//  1. The faces of the @font-face rules of the document.
//  2. The first faces of the program, FontConfigurationOptions.FirstFaces,
//     such as the fonts of ivy.Config.Fonts.
//  3. The first faces of the program again, for the substitute of the family
//     that ivy/internal/font.Substitute gives, such as Carlito for Calibri.
//  4. The other faces of the program, such as the Go fonts and the fonts of
//     the system.
//
// A family of the faces of a step hides the same family in the faces of the
// later steps, as the fonts of @font-face rules do in CSS.
//
// Port: Fontconfig matches the names of the families without regard to
// case, and so does the query. The query of okage matches the case.
func (fc *FontConfiguration) query(family svg.FontFamily, q svg.FontQuery) (svg.FontID, bool) {
	name := fc.db.FamilyName(family)
	if id, ok := fc.queryRange(fc.base, fc.db.Len(), name, q); ok {
		return id, true
	}
	if id, ok := fc.queryRange(0, fc.first, name, q); ok {
		return id, true
	}
	if sub, ok := ivyfont.Substitute(name); ok && family.Kind == svg.FontFamilyNamed {
		if id, ok := fc.queryRange(0, fc.first, sub, q); ok {
			return id, true
		}
	}
	return fc.queryRange(fc.first, fc.base, name, q)
}

// queryRange returns the best face of the family name among the faces from
// lo to hi of the database, with the query of okage.
func (fc *FontConfiguration) queryRange(lo, hi int, name string, q svg.FontQuery) (svg.FontID, bool) {
	var sub *svg.FontDatabase
	var ids []svg.FontID
	for id := lo; id < hi; id++ {
		face, ok := fc.db.Face(svg.FontID(id))
		if !ok || !slices.ContainsFunc(face.Families, func(f svg.FontFamilyName) bool {
			return strings.EqualFold(f.Name, name)
		}) {
			continue
		}
		if sub == nil {
			sub = svg.NewFontDatabase()
		}
		// The copy shares the bytes of the face. It has one family, so that
		// the query of okage matches it whatever the case.
		info := *face
		info.Families = []svg.FontFamilyName{{Name: name, Language: svg.FontLanguageEnglishUnitedStates}}
		sub.PushFaceInfo(info)
		ids = append(ids, face.ID)
	}
	if sub == nil {
		return 0, false
	}
	q.Families = []svg.FontFamily{{Kind: svg.FontFamilyNamed, Name: name}}
	i, ok := sub.Query(q)
	if !ok || int(i) >= len(ids) {
		return 0, false
	}
	return ids[i], true
}

// face returns the face of go-text of a font, with the variations of a
// style, or nil when the font cannot be parsed.
func (fc *FontConfiguration) face(id svg.FontID, style *Style) *font.Face {
	variations := fontVariations(style)
	key := faceKey{id, variationsKey(variations)}
	if f, ok := fc.faces[key]; ok {
		return f
	}
	ft, ok := fc.fonts[id]
	if !ok {
		ft = fc.parseFont(id)
		fc.fonts[id] = ft
	}
	var f *font.Face
	if ft != nil {
		f = font.NewFace(ft)
		if len(variations) > 0 {
			f.SetVariations(variations)
		}
	}
	fc.faces[key] = f
	return f
}

// parseFont parses the font id with go-text, or returns nil.
func (fc *FontConfiguration) parseFont(id svg.FontID) (ft *font.Font) {
	defer func() {
		// Port: a panic in the reader of a font is a font that cannot be
		// parsed.
		if r := recover(); r != nil {
			ft = nil
		}
	}()
	data, index, ok := fc.db.FaceData(id)
	if !ok {
		return nil
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || int(index) >= len(loaders) {
		return nil
	}
	ft, err = font.NewFont(loaders[index])
	if err != nil {
		return nil
	}
	return ft
}

// fontVariations returns the variations of a style: the weight, and the
// axes of font-variation-settings, as get_font_description in
// weasyprint/text/fonts.py sets them.
func fontVariations(style *Style) []font.Variation {
	if style == nil {
		return nil
	}
	variations := []font.Variation{{Tag: ot.MustNewTag("wght"), Value: float32(fontWeight(style.Get("font_weight")))}}
	if settings, ok := style.Get("font_variation_settings").(Tuple); ok {
		for _, item := range settings {
			pair, ok := item.(Tuple)
			if !ok || len(pair) != 2 || len(str(pair[0])) != 4 {
				continue
			}
			variations = append(variations, font.Variation{Tag: ot.MustNewTag(str(pair[0])), Value: float32(num(pair[1]))})
		}
	}
	return variations
}

// variationsKey returns a key for variations.
func variationsKey(variations []font.Variation) string {
	var b strings.Builder
	for _, v := range variations {
		b.WriteString(v.Tag.String())
		b.WriteString(strconv.FormatFloat(float64(v.Value), 'g', -1, 32))
		b.WriteByte(' ')
	}
	return b.String()
}

// covers reports whether the font id draws the character r, within the
// unicode-range of its @font-face rule.
func (fc *FontConfiguration) covers(id svg.FontID, f *font.Face, r rune) bool {
	if f == nil {
		return false
	}
	if rule, ok := fc.faceRules[id]; ok && len(rule.ranges) > 0 {
		in := false
		for _, rg := range rule.ranges {
			if r >= rg[0] && r <= rg[1] {
				in = true
				break
			}
		}
		if !in {
			return false
		}
	}
	_, ok := f.NominalGlyph(r)
	return ok
}

// fontmap is the Fontmap of go-text of a style: it gives the face of each
// character, as Pango selects it from the font description.
type fontmap struct {
	fc     *FontConfiguration
	style  *Style
	chain  []svg.FontID
	byFace map[*font.Face]svg.FontID
}

// newFontmap returns the fontmap of a style.
func (fc *FontConfiguration) newFontmap(style *Style) *fontmap {
	return &fontmap{fc: fc, style: style, chain: fc.chain(style), byFace: map[*font.Face]svg.FontID{}}
}

// ResolveFace satisfies shaping.Fontmap. It returns the first face of the
// families that draws r, then the fallback face of okage, then the first
// face.
func (m *fontmap) ResolveFace(r rune) *font.Face {
	for _, id := range m.chain {
		if f := m.fc.face(id, m.style); m.fc.covers(id, f, r) {
			m.byFace[f] = id
			return f
		}
	}
	if len(m.chain) == 0 {
		return nil
	}
	key := fallbackKey{m.chain[0], r}
	id, ok := m.fc.fallbacks[key]
	if !ok {
		id = -1
		if fallback, found := svg.DefaultFallbackSelector(nil)(r, m.chain, m.fc.db); found {
			id = int(fallback)
		}
		m.fc.fallbacks[key] = id
	}
	if id >= 0 {
		if f := m.fc.face(svg.FontID(id), m.style); f != nil {
			m.byFace[f] = svg.FontID(id)
			return f
		}
	}
	for _, id := range m.chain {
		if f := m.fc.face(id, m.style); f != nil {
			m.byFace[f] = id
			return f
		}
	}
	return nil
}

// primary returns the first face of the families that go-text can use, and
// its font, or nil.
func (m *fontmap) primary() (*font.Face, svg.FontID) {
	for _, id := range m.chain {
		if f := m.fc.face(id, m.style); f != nil {
			return f, id
		}
	}
	return nil, 0
}

// languageOf returns the language of a style, as Layout.setup of
// weasyprint/text/line_break.py sets it on the context of Pango.
func languageOf(style *Style) language.Language {
	if override := style.Get("font_language_override"); !isStr(override, "normal") && override != nil {
		lang := str(override)
		if iso, ok := lstToISO[strings.ToLower(lang)]; ok {
			lang = iso
		}
		return language.NewLanguage(lang)
	}
	if lang, ok := style.Get("lang").(String); ok && lang != "" {
		return language.NewLanguage(string(lang))
	}
	return language.DefaultLanguage()
}
