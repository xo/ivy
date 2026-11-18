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
// style do not draw is the fallback selector of okage (okage D39). The
// database parses each font once with go-text, and gives the faces for the
// shaping and the metrics, and the character maps (okage D57). The other
// differences are marked with Port: in the code.

package layout

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"image/color"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/xo/okage/svg"
	"github.com/xo/okage/tinycss2"
	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/segmenter"
	"github.com/xo/okage/typesetting/shaping"
	"github.com/xo/okage/webfont"

	"github.com/xo/ivy"
	ivyfont "github.com/xo/ivy/internal/font"
	"github.com/xo/ivy/internal/fontdb"
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
// Several books can share one configuration, such as the text bodies of a
// slide (D54). These books lock mu for each access to the configuration.
//
// Port: upstream has no mu and no prog. It lays out one document at a time,
// and makes no configuration from another one.
type FontConfiguration struct {
	// mu is the lock of the books that use the configuration. A book holds
	// it while it lays out or draws its pages.
	mu sync.Mutex
	// prog is the database of the program, which db copies. A book makes
	// the configurations of its goroutines from it (D48).
	prog *svg.FontDatabase
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
	// faces holds the faces of go-text, by font and variations. A face is
	// not safe for concurrent use, so the configuration keeps its own faces
	// of the fonts that the database parses.
	faces map[faceKey]*font.Face
	// colorFonts tells for each font whether it has the table COLR or the
	// table SVG, which hold color glyphs.
	colorFonts map[svg.FontID]bool
	// colorGlyphs holds the color glyphs that the drawing drew, and nil for
	// a glyph that has none, by colorGlyphKey. It holds at most
	// maxColorGlyphs glyphs.
	colorGlyphs map[colorGlyphKey]*svg.Tree
	// chains holds the faces of the families of a style, by the key of the
	// font of the style.
	chains map[string][]svg.FontID
	// fallbacks holds the fallback faces of a character, by the first face
	// of a chain and the character. A missing face is -1.
	fallbacks map[fallbackKey]int
	// eastAsian holds the East Asian faces of the program, which
	// eastAsianFaces reads on first use.
	eastAsian *fontdb.EastAsian
	// eastAsianChains holds the East Asian faces of the program for a kind,
	// a language and a query, the best first.
	eastAsianChains map[eastAsianKey][]svg.FontID
	// shaper shapes the text.
	shaper shaping.HarfbuzzShaper
	// segmenter splits the text into runs.
	segmenter shaping.Segmenter
	// shaped caches the shaped texts, and shapedBytes counts the bytes of
	// their texts.
	shaped      map[shapeKey]*shapedText
	shapedBytes int
	// shapeRunes, shapeClusters, shapeFound and shapeIndex are the buffers
	// of shapeText, which it reuses for each text.
	shapeRunes    []rune
	shapeClusters []textCluster
	shapeFound    []bool
	shapeIndex    []int32
	// breakRunes and breaker are the buffer and the segmenter of
	// lineBreaks, which it reuses for each text.
	breakRunes []rune
	breaker    segmenter.Segmenter

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

// colorGlyphKey is the key of a color glyph: a font, a glyph, the
// variations of the font, and the color of the text. The palette is always
// the default palette, because the port has no font-palette.
type colorGlyphKey struct {
	id         svg.FontID
	glyph      uint32
	variations string
	color      color.NRGBA
}

// maxColorGlyphs bounds the color glyphs that a FontConfiguration keeps.
const maxColorGlyphs = 256

// fallbackKey is the key of a fallback face: the first face of a chain, the
// character, and the kind and the language of the East Asian text of the
// style.
type fallbackKey struct {
	first svg.FontID
	r     rune
	kind  fontdb.Kind
	lang  fontdb.Lang
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
		prog:         db,
		db:           db.Clone(),
		limits:       opts.Limits.WithDefaults(),
		log:          log,
		first:        min(max(opts.FirstFaces, 0), db.Len()),
		base:         db.Len(),
		rules:        map[string]bool{},
		faceRules:    map[svg.FontID]*faceRule{},
		faces:        map[faceKey]*font.Face{},
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
	key := fontStyleCacheKey(style, false)
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
		if !ok && family.Kind == svg.FontFamilyNamed {
			id, ok = fc.eastAsianSubstitute(family.Name, styleLang(style), query, style)
		}
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

// eastAsianFaces returns the East Asian faces of the program, such as the
// fonts of the system for Japanese, Chinese and Korean text.
func (fc *FontConfiguration) eastAsianFaces() *fontdb.EastAsian {
	if fc.eastAsian == nil {
		fc.eastAsian = fontdb.NewEastAsian(fc.db, 0, fc.base)
	}
	return fc.eastAsian
}

// eastAsianKey is the key of the East Asian faces for a kind, a language
// and a query.
type eastAsianKey struct {
	kind    fontdb.Kind
	lang    fontdb.Lang
	weight  uint16
	stretch svg.FontStretch
	style   svg.FontStyle
}

// eastAsianChain returns the best face for q of each East Asian family of
// the program, the best family for kind and lang first
// (fontdb.EastAsian.Families).
func (fc *FontConfiguration) eastAsianChain(kind fontdb.Kind, lang fontdb.Lang, q svg.FontQuery) []svg.FontID {
	key := eastAsianKey{kind, lang, q.Weight, q.Stretch, q.Style}
	if ids, ok := fc.eastAsianChains[key]; ok {
		return ids
	}
	var ids []svg.FontID
	for _, family := range fc.eastAsianFaces().Families(kind, lang) {
		if id, ok := fc.query(svg.FontFamily{Kind: svg.FontFamilyNamed, Name: family}, q); ok && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if fc.eastAsianChains == nil {
		fc.eastAsianChains = map[eastAsianKey][]svg.FontID{}
	}
	fc.eastAsianChains[key] = ids
	return ids
}

// eastAsianSubstitute returns the substitute of an East Asian font that the
// program does not hold, such as MS Mincho: the best face for q of an East
// Asian family of the program of the same kind (fontdb.ClassifyName). It
// prefers the language of the text, lang, or else the language of the
// font. A font of fontdb.LookupEastAsian takes its own face first, by
// another of its names. The face must draw the fontdb.Probe of the
// language. It returns false for a family that is not an East Asian font,
// and when the program holds no East Asian face, as without the fonts of
// the system (D23).
//
// Port: Fontconfig gives such a family the fonts of its language and of
// its generic family. ivy reads no configuration of Fontconfig (D1).
func (fc *FontConfiguration) eastAsianSubstitute(name string, lang fontdb.Lang, q svg.FontQuery, style *Style) (svg.FontID, bool) {
	kind, fontLang, ok := fontdb.ClassifyName(name)
	if !ok || fc.eastAsianFaces().Len() == 0 {
		return 0, false
	}
	if f, ok := fontdb.LookupEastAsian(name); ok {
		// Another name of the same font, such as MS PGothic for
		// MS Pゴシック, which differs from ＭＳ Ｐゴシック in the width of its
		// letters.
		for _, n := range append([]string{f.Family}, f.Names...) {
			if id, ok := fc.query(svg.FontFamily{Kind: svg.FontFamilyNamed, Name: n}, q); ok {
				return id, true
			}
		}
	}
	if lang == fontdb.NoLang {
		lang = fontLang
	}
	probe := fontdb.Probe(lang)
	for _, id := range fc.eastAsianChain(kind, lang, q) {
		if fc.covers(id, probe) && fc.face(id, style) != nil {
			return id, true
		}
	}
	return 0, false
}

// styleLang returns the East Asian language of the lang of a style, or
// fontdb.NoLang.
func styleLang(style *Style) fontdb.Lang {
	if lang, ok := style.Get("lang").(String); ok {
		return fontdb.ParseLang(string(lang))
	}
	return fontdb.NoLang
}

// eastAsianStyle returns the kind and the language of the East Asian text
// of a style, for the fallback of an East Asian character. The language is
// the lang of the style, or else the language of the first East Asian font
// of font-family. The kind is the kind of that font, or else the kind of
// the first generic family of font-family, or else serif, as the default
// family of the chain.
func eastAsianStyle(style *Style) (fontdb.Kind, fontdb.Lang) {
	kind, lang := fontdb.NoKind, styleLang(style)
	generic := fontdb.NoKind
	for _, name := range valueStrings(style.Get("font_family")) {
		if g, ok := genericFamilies[strings.ToLower(name)]; ok {
			if generic == fontdb.NoKind {
				generic = fontdb.Serif
				if g.Kind != svg.FontFamilySerif && g.Kind != svg.FontFamilyCursive && g.Kind != svg.FontFamilyFantasy {
					generic = fontdb.Sans
				}
			}
			continue
		}
		k, l, ok := fontdb.ClassifyName(name)
		if !ok {
			continue
		}
		if lang == fontdb.NoLang {
			lang = l
		}
		if kind == fontdb.NoKind {
			kind = k
		}
		break
	}
	if kind == fontdb.NoKind {
		kind = generic
	}
	if kind == fontdb.NoKind {
		kind = fontdb.Serif
	}
	return kind, lang
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
// style, or nil when the font cannot be parsed. The face shares the font
// that the database parses once (okage D57).
//
// A style that keeps a fontmap of fc gives the face of that fontmap, whose
// variations are computed once.
func (fc *FontConfiguration) face(id svg.FontID, style *Style) *font.Face {
	if style != nil {
		if sf := style.fonts; sf != nil && sf.fc == fc {
			return sf.m.face(id)
		}
	}
	variations := fontVariations(style)
	return fc.faceWith(id, variations, variationsKey(variations))
}

// faceWith returns the face of a font with variations, whose key is key.
func (fc *FontConfiguration) faceWith(id svg.FontID, variations []svg.FontVariation, key string) *font.Face {
	k := faceKey{id, key}
	if f, ok := fc.faces[k]; ok {
		return f
	}
	f := newFace(fc.db, id, variations)
	fc.faces[k] = f
	return f
}

// newFace returns a new face of the font id of db with variations, or nil.
func newFace(db *svg.FontDatabase, id svg.FontID, variations []svg.FontVariation) (f *font.Face) {
	defer func() {
		// Port: a panic in the reader of a font is a font that cannot be
		// parsed.
		if r := recover(); r != nil {
			f = nil
		}
	}()
	f, ok := db.NewFace(id, variations)
	if !ok {
		return nil
	}
	return f
}

// colorGlyph returns the color glyph of the table COLR or of the table SVG
// of the glyph of the font id, with variations and the color of the text,
// or nil when the glyph has none. The tree is in em units, with the origin
// of the glyph at (0, 0) on the baseline (okage D57).
//
// Port: upstream draws the color glyphs with Cairo.
func (fc *FontConfiguration) colorGlyph(id svg.FontID, glyph uint32, variations []svg.FontVariation, fg color.NRGBA) *svg.Tree {
	if !fc.hasColorGlyphs(id) {
		return nil
	}
	key := colorGlyphKey{id, glyph, variationsKey(variations), fg}
	if tree, ok := fc.colorGlyphs[key]; ok {
		return tree
	}
	tree := colorGlyph(fc.db, id, glyph, svg.ColorGlyphStyle{Variations: variations, Foreground: fg})
	if fc.colorGlyphs == nil || len(fc.colorGlyphs) >= maxColorGlyphs {
		fc.colorGlyphs = map[colorGlyphKey]*svg.Tree{}
	}
	fc.colorGlyphs[key] = tree
	return tree
}

// colorGlyph returns the color glyph of db, or nil.
func colorGlyph(db *svg.FontDatabase, id svg.FontID, glyph uint32, style svg.ColorGlyphStyle) (tree *svg.Tree) {
	defer func() {
		// Port: a panic in the reader of a font is a glyph without color.
		if recover() != nil {
			tree = nil
		}
	}()
	tree, ok := db.ColorGlyph(id, svg.GlyphID(glyph), style)
	if !ok {
		return nil
	}
	return tree
}

// hasColorGlyphs reports whether the font id has the table COLR or the
// table SVG. It reads the table directory of the font once, so that the
// glyphs of the other fonts do not ask for a color glyph.
func (fc *FontConfiguration) hasColorGlyphs(id svg.FontID) bool {
	if ok, found := fc.colorFonts[id]; found {
		return ok
	}
	ok := hasColorTables(fc.db, id)
	if fc.colorFonts == nil {
		fc.colorFonts = map[svg.FontID]bool{}
	}
	fc.colorFonts[id] = ok
	return ok
}

// hasColorTables reports whether the font id of db has the table COLR or
// the table SVG.
func hasColorTables(db *svg.FontDatabase, id svg.FontID) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	data, index, found := db.FaceData(id)
	if !found {
		return false
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || int(index) >= len(loaders) {
		return false
	}
	ld := loaders[index]
	return ld.HasTable(ot.MustNewTag("COLR")) || ld.HasTable(ot.MustNewTag("SVG "))
}

// syntheticBold reports whether the text of a style draws the face id in a
// synthetic bold: the style asks for a bold weight, the face is not bold,
// and the face has no variations that make it bold. A family without a
// bold face, such as SimSun, then still draws its bold text bold.
//
// Port: upstream draws with Pango, which emboldens such a face with Cairo.
func (fc *FontConfiguration) syntheticBold(id svg.FontID, style *Style) bool {
	if fontWeight(style.Get("font_weight")) < 600 || fc.db == nil {
		return false
	}
	info, ok := fc.db.Face(id)
	if !ok || info.Weight >= 600 {
		return false
	}
	f := fc.face(id, style)
	return f != nil && len(f.Coords()) == 0
}

// fontVariations returns the variations of a style: the axes of
// font-variation-settings, as get_font_description in
// weasyprint/text/fonts.py sets them, and the axes of the weight, the
// stretch and the style that svg.AutoFontVariations adds.
//
// Port: Pango sets the axis wght of the weight only. The port also sets the
// axes wdth, ital and slnt, as okage does for the text of SVG (okage D57).
func fontVariations(style *Style) []svg.FontVariation {
	if style == nil {
		return nil
	}
	var variations []svg.FontVariation
	if settings, ok := style.Get("font_variation_settings").(Tuple); ok {
		for _, item := range settings {
			pair, ok := item.(Tuple)
			if !ok || len(pair) != 2 || len(str(pair[0])) != 4 {
				continue
			}
			variations = append(variations, svg.NewFontVariation([4]byte([]byte(str(pair[0]))), float32(num(pair[1]))))
		}
	}
	return svg.AutoFontVariations(variations,
		uint16(fontWeight(style.Get("font_weight"))),
		fontStretch(str(style.Get("font_stretch"))),
		fontStyle(str(style.Get("font_style"))))
}

// variationsKey returns a key for variations.
func variationsKey(variations []svg.FontVariation) string {
	var b strings.Builder
	for _, v := range variations {
		b.Write(v.Tag[:])
		b.WriteString(strconv.FormatFloat(float64(v.Value), 'g', -1, 32))
		b.WriteByte(' ')
	}
	return b.String()
}

// covers reports whether the font id draws the character r, within the
// unicode-range of its @font-face rule. It reads the character map that the
// database keeps for each font file, and it does not parse the face (okage
// D57).
func (fc *FontConfiguration) covers(id svg.FontID, r rune) (ok bool) {
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
	defer func() {
		// Port: a panic in the reader of a font is a font that draws
		// nothing.
		if recover() != nil {
			ok = false
		}
	}()
	_, ok = fc.db.NominalGlyph(id, r)
	return ok
}

// fontmap is the Fontmap of go-text of a style: it gives the face of each
// character, as Pango selects it from the font description.
type fontmap struct {
	fc     *FontConfiguration
	style  *Style
	chain  []svg.FontID
	byFace map[*font.Face]svg.FontID
	// kind and lang are the kind and the language of the East Asian text
	// of the style (eastAsianStyle).
	kind fontdb.Kind
	lang fontdb.Lang
	// variations are the variations of the faces of the style, and
	// variationsKey is their key. newFontmap computes them once.
	variations    []svg.FontVariation
	variationsKey string
	// faces holds the face of each font that the fontmap asked for, and nil
	// for a font that go-text cannot parse.
	faces []fontmapFace
}

// fontmapFace is a font and its face, in a fontmap.
type fontmapFace struct {
	id   svg.FontID
	face *font.Face
}

// newFontmap returns the fontmap of a style.
func (fc *FontConfiguration) newFontmap(style *Style) *fontmap {
	m := &fontmap{fc: fc, style: style, chain: fc.chain(style), byFace: map[*font.Face]svg.FontID{}}
	m.kind, m.lang = eastAsianStyle(style)
	m.variations = fontVariations(style)
	m.variationsKey = variationsKey(m.variations)
	return m
}

// face returns the face of a font with the variations of the style of the
// fontmap, or nil when the font cannot be parsed. It is face of
// FontConfiguration, without the computation of the variations for each
// call.
func (m *fontmap) face(id svg.FontID) *font.Face {
	for _, f := range m.faces {
		if f.id == id {
			return f.face
		}
	}
	f := m.fc.faceWith(id, m.variations, m.variationsKey)
	m.faces = append(m.faces, fontmapFace{id, f})
	return f
}

// ResolveFace satisfies shaping.Fontmap. It returns the first face of the
// families that draws r, then for an East Asian character the East Asian
// face of the program that draws it (eastAsianFallback), then the fallback
// face of okage, then the first face.
func (m *fontmap) ResolveFace(r rune) *font.Face {
	for _, id := range m.chain {
		if !m.fc.covers(id, r) {
			continue
		}
		if f := m.face(id); f != nil {
			m.byFace[f] = id
			return f
		}
	}
	if len(m.chain) == 0 {
		return nil
	}
	key := fallbackKey{first: m.chain[0], r: r}
	if fontdb.IsCJK(r) {
		key.kind, key.lang = m.kind, m.lang
	}
	id, ok := m.fc.fallbacks[key]
	if !ok {
		id = -1
		if fallback, found := m.eastAsianFallback(r); found {
			id = int(fallback)
		} else if fallback, found := svg.DefaultFallbackSelector(nil)(r, m.chain, m.fc.db); found {
			id = int(fallback)
		}
		m.fc.fallbacks[key] = id
	}
	if id >= 0 {
		if f := m.face(svg.FontID(id)); f != nil {
			m.byFace[f] = svg.FontID(id)
			return f
		}
	}
	for _, id := range m.chain {
		if f := m.face(id); f != nil {
			m.byFace[f] = id
			return f
		}
	}
	return nil
}

// eastAsianFallback returns the East Asian face of the program that draws
// the East Asian character r, the best for the kind and the language of the
// style (fontdb.EastAsian.Families), with the weight, the style and the
// stretch of the style. A style without a language takes the order of
// fontdb.EastAsian.Families for no language for each character, so that
// the kana and the Han characters of a text take the same face when it
// draws them. It returns false for another character, and when no East
// Asian face draws r.
//
// The East Asian faces come before the other fallback faces, so that a
// Japanese text takes a Japanese font of the system, and not Droid Sans
// Fallback of the module github.com/xo/ivy/fonts, which draws the Han
// characters in their Chinese forms. Without the fonts of the system,
// the program holds no East Asian face, and the fallback does not change
// (D23).
//
// Port: Fontconfig sorts the fonts of the system by the language of the
// text for a fallback. ivy prefers the language and the kind of the text.
func (m *fontmap) eastAsianFallback(r rune) (svg.FontID, bool) {
	if !fontdb.IsCJK(r) || m.fc.eastAsianFaces().Len() == 0 {
		return 0, false
	}
	q := svg.FontQuery{
		Weight:  uint16(fontWeight(m.style.Get("font_weight"))),
		Stretch: fontStretch(str(m.style.Get("font_stretch"))),
		Style:   fontStyle(str(m.style.Get("font_style"))),
	}
	for _, id := range m.fc.eastAsianChain(m.kind, m.lang, q) {
		if m.fc.covers(id, r) && m.face(id) != nil {
			return id, true
		}
	}
	return 0, false
}

// primary returns the first face of the families that go-text can use, and
// its font, or nil.
func (m *fontmap) primary() (*font.Face, svg.FontID) {
	for _, id := range m.chain {
		if f := m.face(id); f != nil {
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
