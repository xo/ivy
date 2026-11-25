# layout

The package `layout` is a port of the layout of
[WeasyPrint](https://github.com/Kozea/WeasyPrint) 70.0 (D15, D21). It lays
out documents of HTML and CSS onto pages. The parsers come from okage:
`html` and `roxmltree` read the documents, `tinycss2` reads the CSS, and
`cssselect2` matches the selectors.

The port starts from the commit `69b19deb03ddf53907899ec450da03117c6a0a8c`
of WeasyPrint, under the BSD-3-Clause license. `LICENSE` in this folder is
the license file of WeasyPrint, byte for byte. `css/` holds the style sheets
of the user agent of `weasyprint/css/`, byte for byte.

The port is done in stages, one after the other. Stages A, B, C and D are
in this package. Stage A is the document tree, the parts of `html.py` and
`urls.py` that read a document, and the cascade of `css/`. Stage B is the
box tree of `formatting_structure/`, the fonts and the line breaks of
`text/`, and the images of `images.py`. Stage C is the layout of
`layout/`, which puts the boxes on pages. Stage D is the drawing of `draw/`
and `stacking.py`, which draws the pages onto the display list of D10.
`Book` puts the documents of a whole `Source` on pages, for the formats of
ebooks.

## The files

Each Python file `weasyprint/a/b.py` becomes the Go file `a_b.go`, and
`a/__init__.py` becomes `a.go`. The tests of `tests/css/test_x.py` are in
the Go test file of the module that they test, or in `css_x_test.go`. A
file that a later stage ports is partly ported when an earlier stage needs
one of its functions.

| Upstream file | Lines | Go file | Lines | State |
| --- | ---: | --- | ---: | --- |
| `css/__init__.py` | 1901 | `css.go` | 3200 | Ported. `ColorProfile` keeps the bytes of the profile, see below |
| `css/computed_values.py` | 742 | `css_computed_values.go` | 1062 | Ported |
| `css/counters.py` | 296 | `css_counters.go` | 516 | Ported |
| `css/functions.py` | 215 | `css_functions.go` | 357 | Ported |
| `css/media_queries.py` | 41 | `css_media_queries.go` | 47 | Ported |
| `css/properties.py` | 414 | `css_properties.go` | 425 | Ported |
| `css/targets.py` | 182 | `css_targets.go` | 277 | Ported. `lookup_target` takes the source box |
| `css/tokens.py` | 694 | `css_tokens.go` | 1165 | Ported |
| `css/units.py` | 120 | `css_units.go` | 268 | Ported. `strut` and `character_ratio` of `text/line_break.py` are a hook, see below |
| `css/validation/__init__.py` | 246 | `css_validation.go` | 351 | Ported |
| `css/validation/descriptors.py` | 384 | `css_validation_descriptors.go` | 529 | Ported |
| `css/validation/expanders.py` | 1140 | `css_validation_expanders.go` | 1582 | Ported |
| `css/validation/properties.py` | 2165 | `css_validation_properties.go` | 2665 | Ported |
| `css/html5_ua.css`, `css/html5_ua_form.css`, `css/html5_ph.css` | 337 | `css/`, embedded by `html.go` | | Copied |
| `html.py` | 424 | `html.go` | 478 | Ported |
| `urls.py` | 430 | `urls.go` | 362 | Partly ported: the URLs and the fetcher of a `Source`, see below |
| `__init__.py` | 379 | `weasyprint.go`, and `CSS` in `css.go` | 154 | Partly ported: `HTML`, `CSS` and `_find_base_url`. `Book` and `DrawPage` stand for the rendering. `Attachment` and the options of PDF are not ported |
| `images.py` | 897 | `images.go` | 1651 | Ported. The methods `draw` draw onto the stream. See "The images" |
| `formatting_structure/boxes.py` | 970 | `formatting_structure_boxes.go` | 1400 | Ported. See "The boxes" |
| `formatting_structure/build.py` | 1333 | `formatting_structure_build.go` | 1739 | Ported |
| `formatting_structure/text.py` | 131 | `formatting_structure_text.go` | 628 | Ported, with the special casing of the methods `upper` and `lower` of Python |
| `text/constants.py` | 542 | `text_constants.go` | 498 | Ported, without the constants of Pango and Fontconfig |
| `text/fonts.py` | 398 | `text_fonts.go` | 929 | Ported on the fonts of okage and go-text. See "The fonts" |
| `text/line_break.py` | 665 | `text_line_break.go` | 1415 | Ported on the fonts of okage and go-text. See "The text" |
| `text/ffi.py` | 547 | none | | Not ported: it binds Pango, Fontconfig and HarfBuzz |
| `layout/__init__.py` | 396 | `layout.go` | 1143 | Ported. `layout_document` is the type `Document`. The file also holds the types that the layout shares. See "The layout" |
| `layout/absolute.py` | 345 | `layout_absolute.go` | 458 | Ported. See "The placeholders" |
| `layout/background.py` | 255 | `layout_background.go` | 382 | Ported |
| `layout/block.py` | 1130 | `layout_block.go` | 1479 | Ported |
| `layout/column.py` | 407 | `layout_column.go` | 573 | Ported |
| `layout/flex.py` | 965 | `layout_flex.go` | 1336 | Ported |
| `layout/float.py` | 261 | `layout_float.go` | 324 | Ported |
| `layout/grid.py` | 1490 | `layout_grid.go` | 2321 | Ported |
| `layout/inline.py` | 1246 | `layout_inline.go` | 1652 | Ported |
| `layout/leader.py` | 74 | `layout_leader.go` | 122 | Ported |
| `layout/min_max.py` | 35 | `layout_min_max.go` | 46 | Ported. The decorators are generic functions that take a closure |
| `layout/page.py` | 994 | `layout_page.go` | 1392 | Ported. `make_pages` is the type `pageIter` |
| `layout/percent.py` | 154 | `layout_percent.go` | 296 | Ported |
| `layout/preferred.py` | 869 | `layout_preferred.go` | 1249 | Ported |
| `layout/replaced.py` | 304 | `layout_replaced.go` | 404 | Ported |
| `layout/table.py` | 1109 | `layout_table.go` | 1581 | Ported |
| `draw/__init__.py` | 625 | `draw.go` | 829 | Ported. See "The drawing" |
| `draw/border.py` | 781 | `draw_border.go` | 1034 | Ported |
| `draw/color.py` | 45 | `draw_color.go` | 122 | Ported, with `rgb_to_hsv` and `hsv_to_rgb` of `colorsys` |
| `draw/text.py` | 301 | `draw_text.go` | 243 | Ported, without the emojis. See "The drawing" |
| `stacking.py` | 132 | `stacking.go` | 208 | Ported |
| `matrix.py` | 51 | `matrix.go` | 77 | Ported |
| `pdf/stream.py` | 339 | `pdf_stream.go` | 981 | Partly ported: the class `Stream`, with the operators of pydyf that the drawing calls, onto the display list. See "The drawing" |
| `anchors.py` | 469 | `anchors.go` | 371 | Partly ported: `gather_anchors` without the forms, `rectangle_aabb`, and the levels of `make_page_bookmark_tree`, which the PDF device of pdfenc ports. See "The drawing" |
| `document.py` | 361 | `document.go` | 112 | Partly ported: `Page.paint`, as `DrawPage` |
| `svg/utils.py` | 216 | `svg_utils.go` | 109 | Partly ported: `normalize` and `size`, for the intrinsic size of an SVG image. okage draws the SVG images |
| `logger.py` | 21 | `logger.go` | 55 | Ported |
| none | | `value.go` | 329 | The Go types of the values of the styles |
| none | | `tree.go` | 342 | The document tree and its adapter for `cssselect2` |
| none | | `source.go` | 130 | `Source`, the input of a format, and the parts of a long document |
| none | | `book.go` | 931 | `Book`, the pages of a whole `Source`. See "The book" |
| none | | `part.go` | 254 | The pages of a part of a long document. See "The document" |
| none | | `objects.go` | 232 | The metafiles of Windows, the objects of a `Source`, and the boxes of display lists. See "The metafiles and the objects" |
| none | | `css_ivy.go`, `layout_tabs.go`, `layout_page_fit.go`, `text_autospace.go`, `layout_word.go` | 907 | The properties that ivy adds for the word processors. See "The properties of ivy" |
| `tests/testing_utils.py` | | `testing_utils_test.go` | 641 | Ported, without `tree_position` |
| `tests/test_boxes.py` | 1280 | `formatting_structure_build_test.go` | 649 | Ported. The tests that need the layout are in `layout_page_test.go` |
| `tests/test_text.py` | 1671 | `text_line_break_test.go`, `layout_text_test.go` | 1533 | The tests that split lines and the tests of `text-transform`, then the tests that need the layout |
| `tests/layout/test_block.py` | 1122 | `layout_block_test.go` | 957 | Ported |
| `tests/layout/test_column.py` | 1207 | `layout_column_test.go` | 1035 | Ported |
| `tests/layout/test_flex.py` | 2167 | `layout_flex_test.go` | 1843 | Ported |
| `tests/layout/test_float.py` | 1000 | `layout_float_test.go` | 769 | Ported |
| `tests/layout/test_footnotes.py`, `test_notes.py` | 1186 | `layout_footnotes_test.go` | 897 | Ported |
| `tests/layout/test_grid.py` | 2128 | `layout_grid_test.go` | 3051 | Ported, with tests of the guards |
| `tests/layout/test_image.py` | 596 | `layout_replaced_test.go` | 650 | Ported |
| `tests/layout/test_inline.py` | 1118 | `layout_inline_test.go` | 1021 | Ported |
| `tests/layout/test_inline_block.py`, `test_list.py`, `test_logical.py` | 376 | `layout_block_misc_test.go` | 381 | Ported |
| `tests/layout/test_page.py` | 1834 | `layout_page_test.go` | 2360 | Ported, with the tests of `tests/test_boxes.py` that need the pages, and the tests of `Document` |
| `tests/layout/test_position.py` | 571 | `layout_absolute_test.go` | 497 | Ported |
| `tests/layout/test_preferred.py` | 95 | `layout_preferred_test.go` | 112 | Ported |
| `tests/layout/test_table.py` | 3700 | `layout_table_test.go`, `layout_table2_test.go` | 3272 | Ported |
| none | | `text_fonts_test.go` | 149 | The tests of `@font-face`, its descriptors and the fallback |
| none | | `images_test.go` | 314 | The tests of the images, the replaced boxes and the gradients |
| none | | `port_test.go` | 409 | The tests of the port that upstream does not have, and the fuzz test |
| `tests/draw/__init__.py` | 137 | `draw_test.go` | 476 | Ported: `assert_pixels` and the other helpers, with `document_write_png` of `tests/conftest.py` |
| `tests/draw/test_x.py`, 23 files | 11854 | `draw_x_test.go` | 14111 | Ported. See "The tests" |
| `tests/test_stacking.py`, `test_acid2.py`, `test_unicode.py`, `test_api.py` | | `stacking_test.go`, `acid2_test.go`, `unicode_test.go`, `api_test.go` | 444 | The tests that draw pixels. The other tests of `test_api.py` test the API of Python, the command line and the PDF file |
| none | | `book_test.go` | 242 | The tests of `Book` |
| none | | `layout_tabs_test.go`, `layout_page_fit_test.go`, `text_autospace_test.go`, `draw_synthetic_bold_test.go`, `draw_bitmap_glyph_test.go` | 479 | The tests of the properties of ivy, of the synthetic bold and of the bitmap glyphs |
| none | | `book_fonts_test.go` | 158 | The tests of the font files of a book |

The other files of `weasyprint/` write PDF, which ivy does not write, or
are the port of SVG of `svg/`, which D15 leaves out. The test resources
`acid2-reference.html`, `acid2-test.html`, `blue.jpg`, `border.svg`,
`border2.svg`, `cmyk_without_icc.jpg`,
`icon.png`, `mask.svg`, `not-optimized.jpg`, `not-optimized-exif.jpg`,
`pattern.gif`, `pattern.palette.png`, `pattern.png`, `pattern.svg`,
`pattern-transparent.svg`, `really-a-png.svg`, `really-a-svg.png`,
`weasyprint.otf` and `weasyprint.woff` of `testdata/` are copies of the
files of `tests/resources/` of upstream.

## The document tree

WeasyPrint reads an `ElementTree` that `tinyhtml5` builds with
`namespace_html_elements=False`. The port builds an `Element` tree that
keeps that shape, so that the later stages follow the Python code line by
line:

- `Tag` is the name of an element of HTML, such as `p`, and the name of any
  other element in the Clark notation, such as
  `{http://www.w3.org/2000/svg}svg`.
- `Attrib` holds the attributes in the order of the source. A name in a
  namespace is in the Clark notation, such as
  `{http://www.w3.org/XML/1998/namespace}lang`. `Get` and `GetDefault` are
  `element.get`.
- `Text` and `Tail` are the attributes `text` and `tail` of `ElementTree`.
  `Children` are the child elements, and `Iter` and `IterTag` are
  `element.iter`.

`TreeFromHTML` builds the tree of an HTML document of okage. `TreeFromXML`
builds the tree of an XHTML document of `roxmltree`, and gives each element
of XHTML the tag of the same element of HTML. So the rest of the layout reads
both kinds of documents the same way. `LoadHTML` reads a document of a
`Source` with the parser that its extension selects. The tree holds no
comment and no processing instruction. Their tails join the text around
them.

`selectorElement` adapts an `Element` to `cssselect2.Element`, and
`wrapperElement` returns the `Element` of a wrapper of `cssselect2`, the
`etree_element` of upstream. Both kinds of documents are wrapped with
`FromHTMLRoot`, as upstream does.

## The values of the styles

WeasyPrint keeps the values of the properties as Python objects. Each type
of Go of `value.go` stands for one of them, and `Value` is the interface
that only these types implement:

| Python | Go |
| --- | --- |
| `str`, a keyword or a string | `String` |
| `int`, `float`, `bool` | `Int`, `Float`, `Bool` |
| `Dimension(value, unit)` | `Dimension`. A unit of `None` is `""` |
| `tuple` or `list` | `Tuple` |
| `set` of `str` | `StringSet`, sorted |
| a token of tinycss2 | `Token` |
| a list of tokens, the value of a custom property | `Tokens` |
| `Color` of tinycss2 | `Color` |
| `PendingProperty`, `PendingExpander` | `*Pending` |
| `LinearGradient`, `RadialGradient` | `*LinearGradient`, `*RadialGradient` |
| `None` | `nil` |

`Equal` compares two values as the operator `==` of Python does, and
`ValueString` prints a value as Python does. The helpers `isStr`, `isStrIn`,
`str`, `num` and `tupleHas` stand for the comparisons that the Python code
makes, such as `value == 'auto'`.

The tagged tuples of upstream stay tuples. For example, an item of
`content` is `Tuple{String("counter()"), Tuple{String("chapter"),
String("decimal")}}`, as `('counter()', ('chapter', 'decimal'))` is in
Python.

## The styles

`Style` is the style dictionary of upstream: `Style`, `InitialStyle`,
`AnonymousStyle` and `ComputedStyle`. It maps the name of each property, in
snake case such as `margin_top`, to its computed value. The name of a custom
property starts with `__`. A value is computed when it is read, as the
method `__missing__` of upstream does:

| Python | Go |
| --- | --- |
| `style['margin_top']` | `style.Get("margin_top")` |
| `style['margin_top'] = value` | `style.Set("margin_top", value)` |
| `'margin_top' in style` | `style.Has("margin_top")` |
| `del style['margin_top']` | `style.Delete("margin_top")` |
| `style.specified['float']` | `style.Specified("float")` |
| `style.anonymous_style` | `style.AnonymousStyle()` |
| `style.copy()` | `style.Copy()` |

A later stage reads a value with `Get` and a type switch or a helper:

```go
if d, ok := style.Get("margin_top").(Dimension); ok { ... }
fontSize := num(style.Get("font_size"))
if isStr(style.Get("float"), "none") { ... }
```

`GetAllComputedStyles` is `get_all_computed_styles`. It returns a
`StyleFor`, whose `Get(owner, pseudoType)` is `style_for(element,
pseudo_type)`. An owner is an `*Element`, or a `PageType` for the styles of
the pages. `AddPageDeclarations`, `SetComputedStyles` and `CascadedKeys` are
the methods that `layout/page.py` calls. A `StyleFor` and its styles are not
safe for concurrent use, because reading a value can compute it.

## The boxes

Upstream has a class for each type of box, and the layout tells them apart
with `isinstance`. The port has one struct, `Box`, whose `Kind` is the
concrete class of upstream, such as `KindBlock` for `BlockBox` and
`KindInlineReplaced` for `InlineReplacedBox`. A box of the port holds the
attributes of all the classes. A field that only some classes use says so.

| Python | Go |
| --- | --- |
| `isinstance(box, boxes.TextBox)` | `box.Kind == KindText` |
| `isinstance(box, boxes.BlockBox)` | `box.Is(KindBlock)`, or `box.IsBlockBox()` |
| `isinstance(box, boxes.TableBox)` | `box.Is(KindTable)`, or `box.IsTable()` |
| `isinstance(box, boxes.ParentBox)` | `box.IsParent()` |
| `isinstance(box, boxes.BlockLevelBox)` | `box.IsBlockLevel()` |
| `isinstance(box, boxes.BlockContainerBox)` | `box.IsBlockContainer()` |
| `isinstance(box, boxes.InlineLevelBox)` | `box.IsInlineLevel()` |
| `isinstance(box, boxes.AtomicInlineLevelBox)` | `box.IsAtomicInlineLevel()` |
| `isinstance(box, boxes.ReplacedBox)` | `box.IsReplaced()` |
| `isinstance(box, boxes.FlexContainerBox)` | `box.IsFlexContainer()` |
| `type(box).__name__` | `box.Kind.String() + "Box"` |
| `box.copy()`, `box.deepcopy()` | `box.Copy()`, `box.DeepCopy()` |
| `box.text`, `box.replacement` | `box.Text`, `box.Replacement` |
| `box.margin_top`, `box.width` | `box.MarginTop`, `box.Width` |
| `box.pango_layout` | `box.TextLayout` |

The layout of stage C reads a box with its kind and its methods, and sets
the used values in the fields of the box model: `PositionX`, `Width`,
`MarginTop`, `BorderTopWidth` and the others. "The layout" describes how
the port keeps the value `auto` of these fields.

`BuildFormattingStructure` is `build_formatting_structure`. It takes the
document, its `StyleFor` and the `TargetCollector` and the `CounterStyle`
of the cascade, and returns the box of the root element and the
footnotes. The functions of `build.py` are the methods of a `builder`,
which holds the arguments that upstream passes from function to function.
`contentToBoxes` and `computeContentList` take a `PageContext` for the
content of the margin boxes. The layout of the pages gives it.

## The text

Upstream lays out text with Pango. The port has no Pango. `TextLayout` is
the class `Layout` of `line_break.py` with the `PangoLayout` that it holds.
It selects the faces with the `FontDatabase` of okage, shapes the text with
the shaper of go-text, finds the opportunities to break the lines with the
segmenter of go-text (UAX #14), and breaks the first line as Pango does
with the wrap mode `PANGO_WRAP_WORD`, or `PANGO_WRAP_CHAR` for
`overflow-wrap`. `SplitFirstLine` and the functions around it follow the
Python code line by line, with the calls to Pango replaced by the methods
of `TextLayout`.

| Python | Go |
| --- | --- |
| `split_first_line(text, style, context, max_width, ...)` | `SplitFirstLine(text, style, maxWidth, ...)`, with `nil` for a `max_width` of `None` |
| its result `(layout, length, resume_index, width, height, baseline)` | `FirstLineMetrics`, with `HasResume` false for a `resume_index` of `None` |
| `create_layout(...)` | `CreateLayout(...)` |
| `layout.text`, `layout.max_width` | `layout.Text`, `layout.MaxWidth` |
| `layout.get_first_line()` | `layout.FirstLine()`, which returns a `TextLine`, the index of the second line and false for `None` |
| `layout.first_line_direction` | `layout.FirstLineDirection` |
| `layout.deactivate()`, `layout.reactivate(style)` | `layout.Deactivate()`, `layout.Reactivate(style)` |
| `layout.ascent`, `layout.underline_position` and the other metrics | the same fields, when `HasDecorationMetrics` is true |
| `line_size(line, style)` | `lineSize(line, style)` |
| `strut(style)`, `character_ratio(style, unit)` | `strut(style)` and `characterRatio(style, unit)` of `css_units.go` |
| `can_break_text`, `get_next_word_boundaries`, `get_last_word_end` | `CanBreakText`, `GetNextWordBoundaries`, `GetLastWordEnd`, with indexes in runes |

`TextLine.GlyphRuns` gives the glyphs of a line for the drawing of stage D:
for each run, the face in the `FontDatabase` of the configuration, the
size, and for each glyph its ID, its position from the start of the line
on the baseline, its advance and the byte of its cluster. The width of a
line is the sum of the advances of its clusters, with the letter spacing
and the word spacing. A line does not shape its text again: a long text
is shaped once, and its lines are measured from its clusters.

## The fonts

`FontConfiguration` is `FontConfiguration` of `fonts.py`. The program
gives it a `FontDatabase` of okage, such as the fonts of the system that
`FontDatabase.LoadOSFonts` loads, or `nil` for no font. The configuration
copies the database, so that the fonts of a document do not go into the
database of another one. Give the configuration to
`ComputedStylesOptions.FontConfig`. It is the `FontConfig` hook of the
`@font-face` rules and the `TextMetrics` hook of the units of the fonts.

1. A face is selected for the families of `font-family`, the weight, the
   style and the stretch with the query of okage, family by family. The
   generic families, such as `serif` and `monospace`, are the generic
   families of the database. The default family `serif` comes last, as in
   the font selector of okage. A family is looked up in the faces of the
   `@font-face` rules first, then in the first faces of the program
   (`FontConfigurationOptions.FirstFaces`), then in the other faces of the
   program. If the first faces have no face of the family, they give the
   substitute of the family before the other faces do, such as Carlito for
   Calibri. `Substitute` of `internal/font` gives the substitute (D24). The
   names of the families match without regard to case, as in Fontconfig.
2. A character that no face of the families draws takes the fallback face
   of okage, `svg.DefaultFallbackSelector`, from the whole database.
3. An `@font-face` rule loads its font with the fetcher of the document.
   `webfont` of okage converts a WOFF, WOFF2 or EOT font. The faces of the
   font get the family of the rule first, and the style, the weight and
   the stretch of the descriptors. `local()` finds a face by its full name
   or its PostScript name. `unicode-range` limits the characters of the
   face, and `font-variant` and `font-feature-settings` add features.
4. go-text parses each face for the shaping and the metrics. The weight
   and `font-variation-settings` set the variations of a variable font.

A style without a `FontConfiguration` measures its text with the default
metrics of stage A: each character is half the font size, and a line is
1.2 times the font size, with the baseline at 0.9 times the font size.

## The images

`Image` is the interface of the images of `images.py`. Its method
`GetIntrinsicSize` returns an `IntrinsicSize`, with a flag for each `None`
of upstream.

1. `RasterImage` holds the encoded data of a PNG, JPEG, GIF, WebP, BMP or
   TIFF image, its size from its header, and its orientation. `Decode`
   decodes its pixels once, within `Limits.MaxPixels`, with the
   orientation applied.
2. `SVGImage` holds SVG data and the attributes `width`, `height` and
   `viewBox` of its root element. `Tree` parses the data with the package
   `svg` of okage. The inline `<svg>` element of a document is written back
   as XML. The drawing renders the image with okage into pixels at the
   scale of the device, because okage renders SVG into a pixmap. The
   images that the SVG data links to are read with the fetcher of the
   document, never from the file system.
3. `LinearGradient` and `RadialGradient` have the method `Layout` of
   upstream, which returns a `GradientLayout`.

`get_image_from_uri` is the method `get` of an `imageLoader`, which holds
the cache of the images of a document.

## The metafiles and the objects

`objects.go` is not a port. It adds two kinds of images to `Image`, for
the Office formats of phase 5 (D24). Upstream has neither of them.

1. `MetafileImage` is an image in EMF, EMF+ or WMF. The loader reads a
   metafile with `internal/metafile` when the decoders of raster images
   do not read the data, and before the last try as SVG. The size of the
   metafile in points is its intrinsic size. The drawing plays the records
   of the metafile onto the display list, as vectors, with the fonts of the
   document.
2. `ObjectImage` is an object that a format draws itself, such as a chart.
   A format gives its objects by name in `Source.Objects`. An `<img>`
   element whose `src` is `ObjectURL(name)`, such as `ivy-object:chart1`,
   or an `<object>` element whose `data` is that URL, shows the object.
   The scheme `ivy-object` cannot name a file of the FS, whose URLs have
   the scheme `file`, and the fetcher reads no object. The loader gives a
   name that `Source.Objects` does not hold as an image that cannot be
   loaded.

The interface `Object` of `source.go` has two methods. `Size` returns the
intrinsic size of the object in points, or zero for none. `Draw` draws
the object onto a `display.Device`, into a rectangle of the user space of
the device, whose y axis points down. The drawing records a call to
`Draw`, and the stream makes the call when it replays its commands. An
error of `Draw` is logged, and the page is drawn in part. The package
`internal/chart` gives its charts as objects.

`ListObject` is an `Object` whose content is a `display.List` (D51). It
has a size in points, and a function that draws the list in the space of
the box. The box records the list once, on its first draw, and replays it
scaled to the rectangle of the box on each draw. A recording that the
context stops is not kept. The page records the replay as it records the
rest of the page (D47). `msdoc` gives each shape of Office Drawing that
`internal/odraw` draws as such a box: in an `<img>` element, or as the
`background-image` of the block of a text box. The images of the styles
load an object through `url()` as the images of the elements do.

## The layout

`layout.go` holds the types that the files of `layout/` share:

| Python | Go |
| --- | --- |
| `'auto'` in a used value, such as `box.width = 'auto'` | `Auto` in the float64 field, such as `box.Width = Auto` |
| `containing_block`, a box or a tuple `(width, height)` | a `*Box`. A tuple is `sizeBlock(width, height)`, whose kind is `kindSize`, and `cb.isBox()` is `isinstance(cb, boxes.Box)` |
| a skip stack or a `resume_at`, `{index: child_stack}` or `None` | `skipStack`, a `map[int]skipStack`, with `nil` for `None` |
| `(index, child), = skip_stack.items()` | `skip.item()` |
| `next_page`, `{'break': 'any', 'page': None}` | `nextPage{brk: "any"}` |
| the tuple of `block_level_layout` | `blockLayoutResult` |
| `max_lines` of `None` | `noMaxLines` |
| `absolute_boxes`, `fixed_boxes`, lists that the layout fills | `*[]*Box` |
| `LayoutContext` | `layoutContext` |
| a decorator of `min_max.py` | `handleMinMaxWidth(box, f)` and `handleMinMaxHeight(box, f)`, where the closure `f` is the function without its decorator |

`Auto` is negative infinity, which no used value of upstream takes. Compare
a field with `Auto` with `==`. The layout resolves each `auto` that it
reads, so the boxes that it returns hold no `Auto`, except in the offsets
`Top`, `Right`, `Bottom` and `Left`. The used values of `min-width`,
`max-width`, `min-height` and `max-height` are the fields `MinWidth`,
`MaxWidth`, `MinHeight` and `MaxHeight`. A value of `none` is infinity.

A style of `::first-line` or `::first-letter` is a dict of the cascaded
properties upstream. The port keeps it in a copy of the style that holds
only those values.

Upstream shares one list of the adjoining margins between a block and its
first and its last child, and changes it in place. The functions of
`layout_block.go` share it with a pointer to a slice. The functions that
other files call give a copy.

### The placeholders

Upstream leaves an `AbsolutePlaceholder` in the children of a box where an
absolutely positioned box was taken out of the flow. The placeholder
pretends to be the box, and the layout replaces its box by the laid out
box.

The placeholder of the port is a copy of the box, with a state:
`newAbsolutePlaceholder(box)` makes it, and `IsPlaceholder` is
`isinstance(box, AbsolutePlaceholder)`. `setLaidOutBox` copies the laid
out box into the placeholder, so each parent that holds the placeholder
sees the laid out box. A placeholder whose box is not laid out yet only
moves its own position in `Translate`, as upstream does.
`placeholderBox` is the attribute `_box`. So a placeholder is a box among
the children: `Descendants` yields it and not its descendants, as
upstream does by default. `descendants(true, yield)` also yields its
descendants, as `descendants(placeholders=True)`.

### The document

`Document` is `layout_document`. `NewDocument` takes the document, the box
of its root element and its footnotes from `BuildFormattingStructure`, its
styles from `GetAllComputedStyles`, the `FontConfiguration`, the
`TargetCollector` and the `CounterStyle`. It lays out no page yet:

1. `Page(ctx, i)` returns the page box of index `i`, from 0, with its
   margin boxes. It lays out the pages up to `i` only, as the generator
   `make_pages` of upstream does. So a program can show the first page of
   a long book without the layout of the whole book.
2. `Count(ctx)` lays out the whole document and returns the number of its
   pages. `All(ctx)` lays out the whole document and returns all its pages.
3. `Page` returns an error that wraps `ErrNoPage` for a page that the
   document does not have.

The whole layout runs the loop of upstream: when a page needs
`counter(pages)` or a target counter of a later page, the document is laid
out again, at most 8 times.

`Page` lays out the whole document before it returns a page when the
styles use `counter(pages)`, a target counter (`target-counter()`,
`target-counters()` or `target-text()`) or `position: fixed` (D43). These
features need the later pages: the number of pages, the page of a later
target, and the fixed boxes of the later pages, which each page repeats.
`Page` also lays out the whole document when the first pass changes the
content of the page or of a page before it, such as for `counter(page)` in
the content of an element. Another document keeps the lazy layout, so its
first page comes fast. After `Count` or `All`, `Page` returns the pages of
the whole layout. The limits of `ivy.Limits` and the context apply to the
whole layout as they apply to `Count`.

The cascade finds these features when it computes the styles
(`findWholeLayoutUses`). It reads the properties `content`, `string-set`,
`bookmark-label` and `position` of each cascaded style, and the
declarations of the `@page` rules. A declaration of an element that the
cascade overrides does not count. A value with `var()` counts the features
of the tokens of all the custom properties of the document. The cascade
also tells whether the styles of the elements use `counter(pages)`, or only
the `@page` rules and their margin rules. When only the margin boxes use
it, the number of pages does not change the layout of the content (D55).

`DocumentOptions.Part` makes the document a part of a long document, such
as a part of a long sheet (D55). This is not a port. A row of a part is an
element `tr`, and the rows are counted in tree order. A page starts at a
row when the row is the first row of the bodies of the tables on the page,
and the page before does not hold the same row. The rows of a header or a
footer of a table do not count. The document keeps the pages from the page
that starts at the cut row to the page before the page that holds the
probe row. When the probe row does not start its page, `Probe` reports the
first row of the last page that starts at a row, up to the page of the
probe row. The document then keeps the pages before that page.
`counter(page)` of the first page that the document keeps is
`PagesBefore + 1`, and `counter(pages)` counts only the pages that it
keeps. `Page(ctx, i)` gives a page only when the end of the part cannot
move before that page, so it lays out the page after it too.

Upstream changes the pages of its list in place when it adds the margin
boxes. The port adds them to copies of the page box and of the box of the
root element, so the pages of a pass stay as `make_pages` made them.

The context of a call stops the layout: the layout checks it after each 256
units of work. An error other than `ErrNoPage` stops the document, and the
document gives the same error again.

## The drawing

Upstream draws a page onto a stream of PDF with the operators of pydyf, and
a reader of PDF, such as Ghostscript in its tests, draws the PDF file. The
port draws onto the display list of D10 instead.

1. `DrawPage` is `Page.paint` of `document.py`. It sets the transformation
   matrices of the boxes, as `gather_anchors` does, draws the page box onto
   a `stream`, and replays the commands of the stream onto a
   `display.Device`. `toDevice` maps the CSS pixels of the page onto the
   device. The drawing sets `TransformationMatrix` on the boxes, so a page
   box is not safe for concurrent use.
2. `stream` in `pdf_stream.go` is the class `Stream` of upstream. It keeps
   the part of the graphics state of PDF that the drawing changes: the
   transform, the colors and the alphas of the filling and of the stroking,
   the width, the cap and the dash of the line, and the soft masks. Each
   operator records a command of `display.Device`.
3. `add_group` makes a stream of its own, which `draw_x_object` draws as a
   group of the device, with the alpha of the filling. `set_alpha_state`
   starts a group with a soft mask, which ends at the restore of the state
   or at the end of its stream.
4. A tiling pattern of `add_pattern` is drawn once into a pixmap at the
   scale of the device, and fills as a pattern shader of okage.
5. A shading of `add_shading` fills the clip with a gradient of okage.
   okage interpolates linearly between the stops of a gradient, so the
   stops sample the functions of PDF whose exponent is not 1, such as the
   hints of the colors. The alphas of a gradient are a soft mask of
   luminosity, as upstream writes them.
6. `draw_text.go` draws `TextLine.GlyphRuns` of the first line of the
   layout of a text box, with the outlines of its faces. The faces are the
   faces of go-text of the `FontConfiguration`, with the variations of the
   style, and `internal/font` gives their outlines. A glyph without an
   outline draws the bitmap of the font when it has one, such as an emoji
   of the tables CBDT and sbix, or a glyph of `weasyprint.otb`. A bitmap
   of one bit for each pixel is a mask that the color of the text fills.
   When the style asks for a weight of 600 or more, and the face is not
   bold and has no variations, the glyphs draw in a synthetic bold: the
   outlines are filled, and stroked with a line of 1/24 of the font size,
   as FreeType emboldens a face for Pango. A family without a bold face,
   such as SimSun, then draws its bold text bold.
7. The colors are converted to sRGB. DeviceCMYK is converted without a
   profile.
8. An SVG image is rendered by okage into pixels at the scale of the device,
   over the part of the image that the page shows.
9. `DrawOptions.Aliased` and `DrawOptions.Sharp` draw as Ghostscript does in
   the tests of upstream: without antialiasing, and without the
   interpolation of the images. Without antialiasing, a stroke thinner than
   a pixel is one pixel wide, and an SVG image is rendered at 4 times the
   scale and reduced, so that a pixel that a shape touches is painted.
   `OkageDevice.SetAntiAlias` turns the antialiasing of the device off.

`DrawPage` gives the links, the anchors and the bookmarks of the page to a
device that is a `display.Linker` or a `display.Bookmarker`, when its
options hold a `PageLinks` (D58). `gather_anchors` finds them. A book names
the anchor of the id X of the document at the path P as "P#X", and the
start of the document as "P", because it holds many documents. When the
`Source` has a table of contents, its entries give the bookmarks, at the
anchors of their targets, and the headings give none. The forms and the
tags of a tagged PDF are not drawn.

## The book

`Book` is not a port. It puts the documents of a whole `Source` on pages,
one after the other. `NewBook` reads no document. `Page(ctx, n)` returns
page `n` of the book, from 0, as an `ivy.Page`, and `PageCount(ctx)` lays
out the whole book.

1. Each document of `Source.Documents` is one document of upstream:
   `LoadHTML`, `GetAllComputedStyles` with `Source.StyleSheets` as the user
   style sheets, `BuildFormattingStructure` and `NewDocument`. The
   documents share one `FontConfiguration`.
2. The layout is lazy. `Page(ctx, n)` lays out only the documents and the
   pages up to page `n`. A document that needs the whole layout is laid
   out whole, as `Document.Page` does.
3. The pages are numbered across the book. `counter(page)` goes on across
   the documents: `DocumentOptions.PagesBefore` gives each document the
   number of pages before it. `counter(pages)` is the number of pages of
   the whole book (D43). When the book loads a document that uses
   `counter(pages)`, it first counts the pages of all the documents after
   it, from the last one. So a page of that document comes after the
   layout of the whole book. A book whose documents do not use
   `counter(pages)` stays lazy. A document that uses `counter(pages)` only
   in its margin boxes counts its own pages before the pages after it,
   because the number of pages does not change its layout (D55).
4. The book keeps the layout of at most 4 documents, the most recently
   used. A document that it drops is laid out again when a page of it is
   asked for. The pages that a program holds stay valid.
5. A document that cannot be read or laid out is logged and left out, so a
   book with one bad chapter still opens. If no document gives a page, the
   book returns the first error, which wraps `ivy.ErrFormat`. The book has
   at most `Limits.MaxObjects` / 16 pages.
6. The size of a page is the margin box of its page box, in points: a CSS
   pixel is 0.75 point. `Render` draws with `DrawPage` onto an
   `OkageDevice`. One mutex serializes the layout and the drawing, so the
   book and its pages are safe for concurrent use. A page drawn in part
   returns an `*ivy.PartialError`.
7. `Draw(ctx, n, dev, t, scale)` draws page `n` onto any `display.Device`.
   `t` maps the points of the page onto the user space of the device, and
   `scale` is the number of pixels of the device for a unit of its user
   space. The drawing renders the SVG images and the patterns at that
   resolution. The text, the paths and the images go to the device as
   commands of the display list, so a `display.List` holds the glyphs, and
   the clip and the groups of the device apply to the page. A format that
   draws a page of a book inside a page of its own uses it, such as the
   text of a shape of a slide in `internal/drawingml`.
8. `PageCount`, and a document that waits for the count of the documents
   after it, first lay out the documents whose count is not known on up to
   8 goroutines, each with a font configuration of its own (D48). A
   goroutine keeps the count and drops the layout. A document that uses
   `counter(pages)` in its content keeps no count from a goroutine. A
   document that uses it only in its margin boxes keeps its count (D55).
9. A `Source` can give a `Splitter`, which cuts a long document, such as a
   long sheet, into parts (D55). `NewBook` makes a document of the book for
   each part, with `DocumentOptions.Part`. The goroutines of item 8 lay out
   the parts at the same time. The count of a part is good when the probe
   row of the part before it starts a page. So `PageCount` and `Page` walk
   the parts in order. When the probe row of a part does not start a page,
   the book asks `Splitter.Recut` for a new next part. The new part starts
   at the row that `Document.Probe` reports, and ends where the old next
   part ends, so the parts after it stay good when its probe row starts a
   page. When `Page(ctx, n)` asks for a page past the pages that a part is
   expected to keep, the book first counts the parts up to that page on the
   goroutines.

The fonts of a book are the fonts of `BookOptions.Fonts`. By default, they
are the database that `Load` of `internal/fontdb` gives for
`BookOptions.FontFiles` and `BookOptions.SystemFonts`. The metafiles and
the charts use the same database, so their text looks like the text of the
book. Without font files, the database holds the Go fonts of
`golang.org/x/image/font/gofont`, as in the tests: Go is the serif, the
sans-serif, the cursive and the fantasy family, and Go Mono the monospace
family. okage bundles no font, and ivy reads the fonts of the system only
when a program asks. `BookOptions.SystemFonts` adds the fonts of the
system, which `LoadOSFonts` of okage reads once. A program sets it for
every format with `ivy.Config.SystemFonts` (D23).

`BookOptions.FontFiles` holds font files that come before the Go fonts. A
program gives them for every format with `ivy.Config.Fonts`, such as the
fonts of the module `github.com/xo/ivy/fonts` (D24). The book looks for a
family in this order:

1. The faces of the `@font-face` rules of the document.
2. The faces of `FontFiles`.
3. The faces of `FontFiles` for the substitute of a font of Office that
   the document names, such as Carlito for Calibri or Liberation Sans for
   Arial. `Substitute` of `internal/font` holds the table.
4. The Go fonts.
5. The fonts of the system, when `SystemFonts` is set.

`internal/fontdb` reads TrueType, OpenType and their collections, and
WOFF and WOFF2, from each folder of `FontFiles`. It reads at most
`Limits.MaxObjects` files, of at most `Limits.MaxMemory` bytes each, in
folders at most `Limits.MaxDepth` deep, and it logs each file that it
leaves out. It reads the names and the styles of the faces first, and the
whole file of a face when the text uses the face. The books, the metafiles
and the charts share the faces of the same `FontFiles`, when its value is
comparable, as the `embed.FS` of `github.com/xo/ivy/fonts` is. They share
at most 16 such sets. The tests of `internal/fontdb` check the limits and
the shared sets.

`book_fonts_test.go` checks that a book with the files of `fonts/` draws
Calibri with Carlito and Arial with Liberation Sans, and the order of the
lookup. It reads the files from the folder, because the main module cannot
import the module of the fonts.

## The properties of ivy

`css_ivy.go` is not a port. It adds fourteen properties for the formats of
the word processors, such as `docx` and `odf`. Upstream does not have them. A
style sheet names each one with the prefix `-ivy-`, such as
`-ivy-tab-stops`, and the validation ignores the name without the prefix.

1. `-ivy-tab-stops` places the tabs of a text at tab stops, as Word and
   LibreOffice do. Its value is `none`, the initial value, `auto`, or a
   list of stops separated by commas. A stop is a length, an alignment
   `left`, `right`, `center` or `decimal`, `left` by default, and a
   string, the leader that fills the tab, such as `'.'`. An example is
   `-ivy-tab-stops: 36pt, 432pt right '.'`. The property is inherited. The
   rules are these:
   1. With `none`, a tab goes to the next multiple of `tab-size` from the
      start of its text box, as in upstream.
   2. Otherwise, each tab of a text is a text box of its own. A tab goes
      to the first stop after the place where it starts. The positions
      are measured from the start edge of the margin box of the block
      container of the line, the left margin of a paragraph whose
      `margin-left` is its indent. After the last stop, and for `auto`,
      the tab goes to the next multiple of `tab-size` from that edge, a
      default stop that aligns left.
   3. The text after a tab, up to the next tab or to the end of the line,
      starts at a stop that aligns left, ends at a stop that aligns right,
      has its center at a stop that centers, and has its first full stop
      at a decimal stop. A decimal stop without a full stop in the text
      aligns right. When the text before the tab goes past the place, the
      tab has no width.
   4. The leader is drawn as many times as it fits in the tab, and ends at
      the end of the tab. An underline of the text of the tab underlines
      the tab.
   5. The breaking of the line gives the tabs their width at stops that
      align left, and no width at the other stops. After the line, the
      layout places each tab again, from left to right. A line with a tab
      to a stop that does not align left can go past the width of the
      line. The tabs of a right-to-left text keep `tab-size`.
2. `-ivy-margin-box-fit` is `none`, the initial value, or `grow`. With
   `grow` on a page, when the content of the margin boxes at the top or at
   the bottom of the page is taller than its margin, the margin grows to
   hold it, and the page is made again. The content is measured from the
   margin edge of its first box to the margin edge of its last box, with
   the margins of the last children of the blocks, and the padding and the
   border of the margin box. A margin grows to at most 40% of the height
   of the page. It is the place of the text of Word below a tall header.
3. `-ivy-static-top` is `auto`, the initial value, or `block`. With
   `block`, the static position of an absolutely positioned box of a line
   is at the top of the part of the block container of the line on its
   page, inside its border, and not below the line. It is the place of a
   shape that a paragraph of Word anchors.
4. `-ivy-text-autospace` is `none`, the initial value, or one or both of
   `ideograph-alpha` and `ideograph-numeric`, as `text-autospace` of CSS
   Text 4. It adds 1/8 em after a character of Han, Hiragana, Katakana or
   Bopomofo that a letter, or a digit, of a text that is not full width
   follows, and before such a character after one. The space is added in
   one text box only. The property is inherited. It is the space of
   `w:autoSpaceDE` and `w:autoSpaceDN` of Word.
5. `-ivy-page-overflow` is `none`, the initial value, or `descent`. With
   `descent`, the last line of a page can pass the bottom of the page by
   up to its descent, and stays on the page, when the line has no text
   (D52). A line has no text when its text, and the text of its inline
   boxes, is only spaces, as the line of an empty paragraph or of a
   paragraph of spaces. A line with text goes to the next page, as in
   upstream. Word keeps an empty line on the page in this way, and moves
   a line with text. The descent is the distance from the baseline of the
   line to the lowest bottom of its fonts, and at most to the bottom of
   the line. The fonts are the font of the block and the fonts of the
   text of the line. A block that ends with such a line passes the bottom
   of the page by the same amount, and its parent accepts it. The rule
   holds only for the root box and for its in-flow block descendants.
   The lines of a table cell, a float, a flex item, a grid item, an inline
   block or a column keep the rule of upstream, because their parents
   check the bottom of the page without the descent. The property is
   inherited. `docx`, `msdoc`, `odf` and `rtf` turn it on.
6. `-ivy-line-numbers` is `none`, the initial value, or four values in
   this order: the distance, the step, the first number and the restart,
   such as `18pt 1 1 page` (D56). The distance is a length that is not
   negative. The step and the first number are integers from 1 to 32767.
   The restart is `page`, `section` or `continuous`. The property is
   inherited. The box that declares the value, whose parent has another
   value, starts a section. After a page is laid out, the layout numbers
   the line boxes of the in-flow blocks of the root box. The lines of a
   table, a float, a flex or grid item, an absolutely positioned box, a
   running element, a margin box and a footnote have no number. A line
   with no height has no number, and a line of a block with `none` does
   not count. The first line of the document has the first number. Each
   next line has the next number, but the first line of a page with
   `page`, and the first line of a section with `section`, has the first
   number again. A number that is a multiple of the step is drawn in the
   style of the box of its section, with its end at the distance before
   the start of the content box of that box, and with its baseline at the
   baseline of its line. The numbers are text boxes in an anonymous block
   that is the last child of the root box of the page. The state of the
   numbers at the end of a page goes to the next page with the state of
   the pages. `docx`, `msdoc`, `odf` and `rtf` read the numbering of the
   lines of their sections.
7. `-ivy-line-multiple` is `none`, the initial value, or a number that is
   not negative: the multiple of a single line that the line height of a
   word processor is (D56). With a number m of 1 or more, the text of a
   line stays where it is in a single line of the height `line-height / m`,
   with the line gap of that single line above the text, and the rest of
   the line goes below the text. The top of the text is then
   `line-height / m - h` below the top of the line, where h is the height
   of the text, and at least 0. CSS puts half of the leading above the
   text. With m = 1, the whole line gap goes above the text. A number below
   1, and a line that is not higher than its text, keep the rule of CSS.
   The property is inherited. `docx`, `msdoc`, `odf` and `rtf` set it on a
   paragraph with a single or a multiple line spacing, as Word and
   LibreOffice put the line gap above the text and the extra space below
   it.
8. `-ivy-page-top-space` is `keep`, the initial value, or `discard`. With
   `discard`, a block of the flow of the root box that starts a page
   other than the first, after a break that is not forced, has no space
   before it (D56). The space is its top margin, and its top padding when
   it has no top border and no background. A break is not forced when the
   page before ends because its content does not fit. The `margin-break`
   of CSS truncates the margins only for the children of the root box and
   of the boxes whose margins collapse with it, and upstream counts each
   break of a named page as forced. In columns, the first block of a
   column has no space before it either, as in Word: each column after a
   column that a forced break does not end, and the first column at the
   top of a page after a break that is not forced. A column break keeps
   the space, as a page break does. The property is inherited. `docx`,
   `msdoc`, `odf` and `rtf` turn it on.
9. `-ivy-float-position` is `auto`, the initial value, or `page` or
   `margin` and a length, `top`, `center` or `bottom`, in any order, such
   as `margin 1in`. It places a float of the flow of the page at a vertical
   position from the top of the page or of the page area, as a word
   processor places a picture or a table at its position (D60). The length
   can be negative. The property is not inherited. See
   `layout_page_float.go`.
10. `-ivy-float-overlap` is `allow`, the initial value, or `avoid`. With
    `avoid`, a float of `-ivy-float-position` moves down below the page
    floats before it that it covers, as LibreOffice moves a table with
    `w:tblOverlap` never (D60). The property is not inherited.
11. `-ivy-wrap-flow` is `auto`, the initial value, or `both`, `start`, `end`
    or `maximum` and an optional length that is not negative, such as
    `both 9pt`. With `both`, the text goes on both sides of the float, as a
    word processor wraps the text around a picture in the middle of a
    column. The length is the distance between the float and the text, to
    the left and to the right. `start`, `end` and `maximum` put the text on
    one side (D63). The property is not inherited, and a table gives it to
    its wrapper. See `layout_wrap_flow.go`.
12. `-ivy-stroke-adjust` is `none`, the initial value, or `pixels`. With
    `pixels`, a collapsed border of the table of the box is drawn on whole
    pixels, at least one pixel wide, as a viewer draws the hairlines of a
    grid. `csv` gives it to the gridlines of a sheet. The property is not
    inherited.
13. `-ivy-line-baseline` is `normal`, the initial value, or `win`. With
    `win`, the baseline of a line is at the height of the line times
    usWinAscent over the sum of usWinAscent and usWinDescent of the font,
    as PowerPoint puts it (D71). The text bodies of DrawingML,
    `msppt` and `msxls` give it.
14. `-ivy-text-outline` is `none`, the initial value, or a length that is
    not negative. With a length, the layout strokes the outlines of the
    glyphs of a text with that width in the color of the text, and does
    not fill them, as Word draws the outline effect of a run. `docx` gives
    it for `w:outline`. The property is inherited.

## The hooks for the later stages

1. `TextMetrics` is the hook of `strut` and `character_ratio` of
   `text/line_break.py`, which the units `ex`, `ch`, `cap`, `ic`, `lh` and
   their root forms need, and the percentages of `vertical-align`.
   `FontConfiguration` fills it. A `FontConfig` that is also a
   `TextMetrics` fills it too. Without one, a normal line is 1.2 times the
   font size, and each character is half the font size, the fallback of
   `character_ratio`.
2. `FontConfig` is the hook of `FontConfiguration.add_font_face` of
   `text/fonts.py`. `FontConfiguration` fills it. `FontFaces` only keeps the
   rules.
3. `StyleContext.InitialPageSizes` holds the sizes of the first page, which
   the units of the viewport read. `makePage` sets them for the first page.
4. `StyleContext.Cache` is the cache of the styles that the code of the
   text uses: `character_ratio` keeps its ratios in it.
5. `PageContext` gives the named strings and the running elements of a
   page to the content of its margin boxes. `layoutContext` fills it.
6. `TargetCollector.CacheTargetPageCounters` takes the length of the page
   maker of `layoutContext`.

The drawing of stage D reads these parts of the laid out pages:

1. The page box: its children are the box of the root element, the
   footnote area and the note area when they have children, and the margin
   boxes. `Background`, `CanvasBackground` and `BleedArea` give its
   backgrounds.
2. The used values of each box: the position, the size, the margins, the
   paddings, the borders and the radii of the corners, which
   `layoutBackgrounds` resolves.
3. `Background` and its `BackgroundLayer` items, `BorderImage` and
   `MaskBorderImage`.
4. The text boxes: `TextLayout`, `Baseline` and `JustificationSpacing`,
   and `TextLine.GlyphRuns` of the line of the layout. The line boxes:
   `TextOverflow` and `BlockEllipsis`.
5. The replaced boxes: `replacedBoxLayout`, the size and the position of
   the image in the content box, and `RasterImage.Decode`, `SVGImage.Tree`
   and the `GradientLayout` of the gradients.
6. The tables: `ColumnPositions`, `ColumnWidths`, `CollapsedBorderGrid`,
   `SkipCellBorderTop`, `SkipCellBorderBottom`, `SkippedRows`, `Empty` and
   `ColumnCells` of a column.
7. The placeholders of the absolute boxes, which `stacking.py` reads with
   `IsPlaceholder`. The placeholder of the port is the laid out box itself.
8. The columns of a multi-column box: the boxes whose `IsColumn` is set.
9. `Link`, `BookmarkLabel`, `StringSet` and the anchors of the styles, for
   the links and the bookmarks.

## The URLs

ivy never reads the network (D1). The URL of a path of the FS of a `Source`
is a file URL rooted at `/`, such as `file:///OEBPS/text/chapter1.xhtml`,
from `pathToURL`. So the links of a document resolve as `urljoin` resolves
them in upstream. `URLFetcher` reads a file URL from the FS, with its `%`
escapes decoded, and decodes a data URL. Any other URL, such as an `http`
URL, fails with `ErrURLFetching`, and the code that fetches it logs an error
and goes on, as upstream does for a URL that it cannot read. A file without
a known extension has the media type `text/plain`, as the `FileHandler` of
`urllib` gives it. So a `<link>` to it is not a style sheet.

## The guards of D18

Each guard is marked with `Port:` in the code.

1. The tree keeps at most `Limits.MaxObjects` elements and
   `DefaultMaxTreeDepth` levels.
2. The fetcher reads at most `Limits.MaxMemory` bytes of a resource.
3. `@import` nests at most `Limits.MaxDepth` deep and is followed at most
   1024 times, and the style sheets of a document read at most
   `Limits.MaxMemory` bytes. Upstream follows a style sheet that imports
   itself without end.
4. The rules of a document count at most `Limits.MaxObjects`. The rules past
   the limit are ignored with a warning. The nested style rules nest at most
   `Limits.MaxDepth` deep.
5. The cascade counts each declaration that it applies and each style that
   it computes. Past 64 times `Limits.MaxObjects`, `GetAllComputedStyles`
   fails with an error that wraps `ivy.ErrLimit`.
6. `var()` substitutes at most 65,536 tokens in one value. A chain of custom
   properties that each use the previous one twice makes a value of
   exponential size in upstream. A longer value is not valid.
7. A math function that needs its own property, such as `font-size:
   calc(1em + 1%)`, stops at the second call. Upstream recurses until
   `RecursionError`, and gives the same value.
8. The text of a counter holds at most 4096 bytes, and the fallbacks of the
   counter styles nest at most 32 deep.
9. `GetAllComputedStyles` turns a panic into an error that wraps
   `ivy.ErrFormat`. The fuzz test `FuzzCascade` still fails on the panic.
10. Where upstream raises `IndexError`, `KeyError`, `AttributeError`,
    `UnboundLocalError` or `NameError`, or fails an assertion, for a
    document or a style sheet that is not valid, the port treats the value
    as not valid, as the comment `Port:` says at each place.
11. A document makes at most 4 times `Limits.MaxObjects` boxes. Past that,
    `BuildFormattingStructure` fails with an error that wraps
    `ivy.ErrLimit`.
12. `BuildFormattingStructure` turns a panic into an error that wraps
    `ivy.ErrFormat`. A panic in the reader of a font, in the shaper or in
    a decoder of images is a font, a text or an image that cannot be used.
13. `colspan` is at most 1000 and `rowspan` at most 65534, the limits of
    HTML. The `span` of a column is at most 1000. The row spans of a table
    mark at most 2<sup>24</sup> cells, and a grid of collapsed borders holds
    at most 2<sup>22</sup> borders.
14. The fonts of the `@font-face` rules of a document read at most
    `Limits.MaxMemory` bytes in all. A style reads at most 64 families of
    `font-family`.
15. A layout shapes at most 2<sup>18</sup> characters of a text, and a line
    tries at most 256 soft hyphens. A configuration keeps at most 4096
    shaped texts, of 4 MiB in all.
16. An image has at most `Limits.MaxPixels` pixels, and a document loads at
    most `Limits.MaxObjects` images. A repeating gradient has at most
    65,536 stops.
17. `strut` measures a style without its letter spacing and word spacing.
    Upstream recurses until `RecursionError` for `letter-spacing: 1lh`.
18. `Document` lays out at most `Limits.MaxObjects` / 16 pages, 65,536 by
    default. Past that, it fails with an error that wraps `ivy.ErrLimit`.
    The loop of `layout_document` lays out the document at most 8 times, as
    upstream does.
19. The layout of a page counts its units of work: the layout of a box, of
    a line, of a row or of a track. Past 64 times `Limits.MaxObjects`
    units, the document fails with an error that wraps `ivy.ErrLimit`.
    Nested flex containers and nested tables lay out the same boxes a
    number of times that is exponential in their depth upstream.
20. The layouts nest at most 2 times `DefaultMaxTreeDepth` plus 64 levels.
    Upstream recurses until `RecursionError`.
21. `Document` turns a panic into an error that wraps `ivy.ErrFormat`. The
    fuzz test still fails on it.
22. A line that resumes where it starts ends the lines of its box. A line
    tries at most 1024 times to fit between the floats, and
    `_break_waiting_children` tries at most 65,536 widths. A leader repeats
    its text at most 65,536 times.
23. When the in-flow layout of a page makes no progress, the port renders
    the page again with forced fragmentation, as upstream does. If that
    gives no box either, or if the page is empty, the document ends with
    this page.
24. A row of columns has at most 1000 columns, and the search for the
    height of the columns stops after 1000 loops.
25. The loop of the flexible lengths of a flex line stops after one pass
    more than the number of its items. Then it freezes all the items.
26. The line numbers and the spans of a grid are clamped to [-10000,
    10000]. A template keeps at most 10,000 tracks, with the repetitions of
    `repeat()`. The implicit grid has at most 40,000 tracks on each axis.
    Past that, the layout fails with an error that wraps `ivy.ErrLimit`.
    The searches for a free place in the grid stop after 40,000 steps plus
    4 for each item. Upstream can loop without end in these searches.
27. The grid of the preferred widths of a table holds at most
    2<sup>22</sup> cells, as the grid of its collapsed borders does. Past
    that, the layout fails with an error that wraps `ivy.ErrLimit`.
28. Where upstream asserts, or raises `TypeError`, `IndexError` or
    `KeyError`, for a box tree or a style that is not valid, the layout goes
    on with a value that is safe, as the comment `Port:` says at each place.
    For example, `min_content_width` gives 0 for a box that it does not
    handle, and a skip stack past the children of a box gives no child.
29. The drawing of a page records at most 64 times `Limits.MaxObjects`
    commands. Past that, `DrawPage` stops and returns an `*ivy.PartialError`
    that wraps `ivy.ErrLimit`. The drawing checks the context after each 256
    commands, when it records them and when it replays them.
30. The groups and the soft masks nest at most 32 deep, because each one
    draws to a layer of the size of the page. A deeper group draws onto its
    parent, and a deeper soft mask is left out.
31. A cell of a tiling pattern and the pixels of an SVG image have at most
    2<sup>24</sup> pixels. A gradient samples at most 16 times 4096 stops.
32. A side of a border, an outline or a line of a text decoration with more
    than 65,536 dashes or dots is drawn solid. A region of a border image
    draws at most 1024 copies of the image. The ellipsis of a block removes
    at most 1024 words.
33. `DrawPage` turns a panic into an `*ivy.PartialError` that wraps
    `ivy.ErrFormat`. A panic of okage in an SVG image leaves the image out.
    An image that cannot be decoded is left out, and the page is drawn in
    part.
34. Where upstream asserts for a box or a style that is not valid, the
    drawing goes on with a value that is safe, as the comment `Port:` says
    at each place. For example, a line of a border that is neither
    vertical nor horizontal is not drawn.

## The other differences from upstream

1. Upstream logs to a global logger. The port passes a `*slog.Logger`
   through `HTMLOptions`, so that two documents can be laid out at the same
   time. `PROGRESS_LOGGER` is not ported.
2. The style sheets of the user agent are parsed once, and the documents
   share them. `ComputedStylesOptions.UserAgentStylesheets` replaces them,
   as `FakeHTML` of the tests of upstream does, and the counter styles of the
   user agent stay.
3. `ComputedStylesOptions.UserStylesheets` takes the style sheets of a
   format as CSS text, with the origin `user`.
4. A value of a position or another tuple that holds a math function, such
   as `background-position: calc(10px + 5%) top`, keeps its math function
   for the computer functions and the layout. Upstream raises `TypeError`
   and then `AttributeError` there.
5. `ColorProfile` keeps the bytes of the profile. Upstream reads it with
   PIL, for its name. The name waits for the stage that writes the colors.
6. `ParseColor5` of okage takes a list of color schemes. The value `normal`
   of `color-scheme` gives the light scheme, as in upstream.
7. As upstream does, the computed value of a gradient changes the gradient
   in place, and the styles that share the declaration share it.
8. The text of the port has no Pango. Pango puts the letter spacing between
   the characters, and `line_size` adds it after the last one. The port
   puts the letter spacing after each character, and the word spacing
   after each space, which is the result that upstream gets. Upstream
   writes a zero-width space after a single space to get its word spacing.
   The port does not need it.
9. The runs of a line from right to left are in the reverse order. Pango
   orders the runs with the levels of the bidirectional algorithm.
10. The language does not change the opportunities to break the lines.
    Pango tailors the rules for some languages.
11. The hyphenation of `hyphens: auto` is off, because Pyphen and its
    dictionaries are not ported (D15). A soft hyphen still hyphenates.
12. The orientation of the EXIF data is read from a JPEG image only. Pillow
    also reads it from PNG, WebP and TIFF images.
13. An inline `<svg>` element always makes a replaced box. Upstream parses
    it first and makes no box when it fails. okage parses it when it is
    drawn.
14. `gradient_average_color` reads the colors in sRGB. Upstream reads their
    coordinates in their own spaces.
15. A `::marker` with `display: none` makes no box. Upstream fails to make
    its box first.
16. `skip_stack.item()` takes the smallest index of a skip stack of more
    than one item. Upstream fails to unpack it.
17. `get_clearance` returns the largest int64 as a float after a broken
    float, for the `sys.maxsize` of upstream.
18. `absolute_replaced` gives a tuple of four values as a containing block,
    and `block_level_width` then reads `cb_x` as the width. The port gives
    the width and the height.
19. `split_text_box` moves a skip past the end of a text, or inside a
    character, to the start of a character. When `split_first_line` gives
    a resume index of 0, it skips one character. Upstream asserts on this.
    `first_letter_to_box` slices the text in UTF-8 bytes.
20. A float of `inline-start` or `inline-end` in a line adds its width to
    its side. Upstream fails with `KeyError`.
21. `replaced_box_height` keeps `Auto` where upstream sets `None` for an
    image without an intrinsic height. `replaced_box_width` gives 300 where
    upstream multiplies a height of `auto` by the ratio.
22. In `auto_table_layout`, a missing lower or upper guess takes the first
    or the last guess. Where the sum of the added widths is 0, the lower
    guess is used. Upstream fails or divides by zero.
23. In a flex line, when the flex grow factors of the unfrozen items add up
    to 0, each item keeps its flex base size. Upstream divides by zero. A
    row gap or a column gap that is a math function gives 0.
24. For a grid item whose start line is a span, in the packing that is not
    dense, the search reads the cursor. Upstream reads a variable that an
    earlier item set, or that is not set and fails, so its search never
    ends.
25. One type, `orientedBox`, stands for the classes `VerticalBox` and
    `HorizontalBox` of `page.py`. `set_page_type_computed_styles` is
    `setPageTypeStyles`, because the tests of stage A have a helper with
    the first name.
26. A floated note area is stored as a list of one excluded shape. Upstream
    stores the box itself, and the layout of the root box then fails.
27. The names of the running elements are iterated in sorted order.
28. A columns box whose children all fail to render resumes at `{0:
    None}`. Upstream returns the tuple `(0, None)`.
29. `layout_backgrounds` rounds the repeats of `round` half to even, as
    `round` of Python does, and does not round a size of 0, where upstream
    divides by zero.
30. A line that fits but for its final space gives that space no width, and
    a wrapped line leaves out the width of its final space, as the
    `zero_line_final_space` of Pango does. The width of a line is rounded
    down to the units of Pango. A wrapped line of `white-space: pre-wrap`
    keeps the width of 0 for its final space when `first_line_metrics`
    lays out its text again. Upstream gives the space its width there, so
    that the line overflows and breaks before the box of the line.
31. A stacking context of an inline block stands in the children of its
    parent as its box, and the map `inline` of the contexts of a page gives
    the context of the box. Upstream puts the context itself in the
    children.
32. `copy_with_children` of upstream clears the removed decorations of the
    original box. The drawing of the port does not change the boxes of the
    layout, which a later pass of the layout of a document can read.
33. The glyphs of a line are drawn at the positions of the shaper, without
    the widths, the kerning and the mapping to Unicode of the text of PDF.
    A control character, such as a tab, is not drawn. Pango draws it as an
    empty glyph.
34. `text-overflow: ellipsis` searches for the longest start of the text
    that fits with the ellipsis, for the `PANGO_ELLIPSIZE_END` of Pango.
35. html5lib of upstream decodes each encoding of the Encoding Standard.
    The parser of okage decodes UTF-8, UTF-16 and windows-1252, and
    `LoadHTML` gives it `charset.Reader` of `internal/charset` for the other
    encodings, such as the code pages of a CHM file. `LoadHTML` passes the
    charset of a response, as the `transport_encoding` of upstream.
36. Each part of a table row that a page break splits takes the height of
    its content. Upstream keeps the height of the row in each part, so that
    a row with a height never splits and goes whole to the next page.
37. The positioned descendants of a margin box are in the stacking context
    of the page. A box with a negative `z-index` in a header then draws
    behind the content of the page, as the watermark of Word does.
    Upstream draws it in the context of the margin box, over the content.
38. A line of `white-space: pre-wrap` is justified, without the spaces at
    its end. Upstream justifies only the text whose spaces collapse.
39. When a cell aligns its content to the middle or to the bottom, a box of
    the cell that is absolutely positioned with a `top` or a `bottom` stays
    in place, because its containing block does not move. Upstream moves it
    with the content of the cell.
40. A cell that spans rows and ends in a row with a height gives that row
    only the part of the cell that is past the rows before it, as for a
    row of the height `auto`. Upstream compares the whole height of the
    cell with the height of the row, which makes the row too high.
41. A `math` element of MathML is a replaced box, as an inline `svg`
    element is. `internal/formula` lays it out with `texmath` of okage. An
    element with `display="block"` is a centered line of its own. Upstream
    draws the text of its children.
42. A line feed between two characters of Chinese or Japanese goes, with
    the spaces and the tabs around it, as CSS Text 3 says. With
    `white-space: normal`, a line feed that one or more ideographic spaces
    follow stays, so that the line breaks there (D68). Upstream turns each
    line feed into a space.
43. A replaced box with a fixed width and a `max-width` in percent has a
    min-content width of 0, as CSS Sizing 3 and the browsers say, so that
    a table of images fits its page. Upstream does this only for a width
    of `auto`.

## What the port leaves out

1. The fetching of `Attachment`, and `select_source`. The metadata keeps the
   URLs of the attachments.
2. `svg/`, the port of SVG of upstream (D15). okage renders the SVG
   images.
3. The writing of PDF of `pdf/` and `document.py`, the forms and the tags
   of a tagged PDF. pdfenc writes the links, the anchors and the
   bookmarks (D58).
4. `draw_emojis` and the color glyphs of the tables SVG and COLR. okage
   draws such glyphs in the text of an SVG image, and has no API that
   draws a glyph of a face. The bitmap glyphs are drawn, see "The
   drawing".
5. The profiles of the colors: DeviceCMYK, Lab and the colors of
   `@color-profile` are converted to sRGB without a profile.
6. `text/ffi.py`, the constants of Pango and Fontconfig, and the cache of
   the font features of Pango.
7. The network, `iri_to_uri` for the encoding of the file system, and the
   decompression of gzip of the fetcher.
8. `computed_from_cascaded` of `css/__init__.py`, which no code calls.
9. The dictionaries of the hyphenation of `LayoutContext`, and
   `PROGRESS_LOGGER`.
10. The `__repr__` of `AbsolutePlaceholder`. `Box.String` prints the box.

## The tests

The tests port the tests of `tests/css/`, `tests/test_boxes.py`,
`tests/test_text.py`, `tests/layout/` and `tests/draw/` as table tests. Each test
function of upstream is a Go test, and its parametrized cases are
subtests. A test of `tests/css/` or `tests/test_boxes.py` that checks a
value of the cascade or of the box tree reads the computed style or the
boxes of `parse_all`, and says so in its comment. The tests compare the box
trees with the literals of Python of upstream, which `parsePyTrees` reads.
The tests of the layout read the positions and the sizes of the boxes of
`renderPages`, the `render_pages` of upstream, which calls `Document.All`.
The tests of the drawing compare the pixels of each page with the maps of
characters of upstream, with `assertPixels`. `documentToPixels` draws each
page at 96 dots per inch without antialiasing and without the interpolation
of the images, as the command of Ghostscript of `tests/conftest.py` does.

The tests do not read the fonts of the machine. Their `FontDatabase` holds
the Go fonts of `golang.org/x/image/font/gofont`: Go is the serif and the
sans-serif family, and Go Mono the monospace family. Many tests use
`weasyprint.otf` of upstream, whose glyphs are 1em wide. The expected
values of upstream stay the same, except in these places, which a comment
`Adapted:` marks:

1. `assertPixels` accepts a difference of 1 in each channel. okage rounds
   the composition of the alphas to the nearest value, and Ghostscript
   truncates it, so a pixel of two layers of 50% red is 64 in okage and 63
   in the expected map. 12 tests of the tables with `assertSameRenderings`
   and `test_opacity_black` accept the same difference.
2. `test_acid2` accepts at most 32 pixels that differ from the reference.
   24 differ, on the diagonal edges of the nose: Ghostscript paints each
   pixel that a path touches, and okage the pixels whose center the path
   covers.

On 2026-11-17, the package runs 1346 test functions with 1373 subtests.
1284 functions and 1355 subtests pass, and 62 functions and 18 subtests are
skipped. The tests of stage C are 672 functions with 684 subtests: 648
functions and 668 subtests pass, and 24 functions and 16 subtests are
skipped. All the 580 test functions of `tests/layout/` are ported. The
tests of stage D are 413 functions with 111 subtests, from the 401 test
functions of the 23 modules of `tests/draw/` and the tests that draw of
`tests/test_stacking.py`, `test_acid2.py`, `test_unicode.py` and
`test_api.py`: 378 functions and the 111 subtests pass, and 35 functions are
skipped. The skipped tests of stage C are:

1. The tests that upstream marks as expected failures: `test_counters_9`,
   `test_math_functions_color`, `test_math_functions_gradient_color`,
   `test_box_decoration_break_slice_bottom_padding`,
   `test_nested_blocks_padding_border`, `test_float_fail`,
   `test_flex_absolute_positioning`, `test_grid_absolute_positioning`,
   `test_lists_whitespace_item`, `test_after_marker_footnote`,
   `test_grid_in_columns_with_break`, `test_empty_linebox_removed_space`,
   `test_breaking_linebox_regression_9`, `test_margin_break_clearance`,
   `test_layout_table_auto_49` and `test_max_lines`.
2. Two cases of `test_math_functions_percentage_and_font_unit`: `font-size`,
   where upstream recurses until `RecursionError`, and `transform-origin`,
   which upstream reads only on an element with a transform.
3. The tests of `hyphens: auto`, because the port has no hyphenation
   dictionaries (D15): `test_hyphenate_character_1` to `_5`,
   `test_hyphenate_limit_zone_1` to `_4`, `test_hyphen_nbsp`, the 15 cases
   of `test_hyphenate_limit_chars`, one case of `test_line_count` and two
   cases of `test_overflow_wrap`.
4. `test_font_stretch`: the fonts of the tests have no condensed face.

The skipped tests of stage D are:

1. 21 tests that upstream marks as expected failures.
2. The 6 tests of `font-variant-caps`: their expected pixels draw the small
   capitals of DejaVu Sans, and the Go fonts of the tests have none.
3. `test_otb_font`: `weasyprint.otb` has bitmap glyphs only.
4. The 6 tests of `test_cmyk_color_profiles.py`: their expected pixels come
   from the profile of CMYK of Ghostscript, ivy reads no profile when it
   draws, and the decoder of JPEG of Go does not decode the subsampling of
   the CMYK images. `testdata/` leaves out `cmyk.icc` and
   `cmyk_with_icc.jpg` of upstream, which only these tests read. They are
   1.7 MB, and the license of the profile is not known.
5. `test_image_exif_image_orientation_keep_format`: it checks the data of
   the PDF file.

The tests of `tests/draw/svg/` test the port of SVG of upstream, which is
not ported (D15). The other tests of `tests/test_api.py` test the API of
Python, the command line and the PDF file.

`book_test.go` checks that `Page(n)` of a book of 50 documents reads only
the documents up to page `n`, that two pages draw at the same time, that a
canceled context stops the book, that a bad document is left out, and
that `counter(pages)` counts the pages of the whole book.

`layout_page_test.go` checks the whole layout of D43: `counter(pages)` and
a target counter of a later page are right on the first page, the fixed
box of the last page shows on the first page, and a document without
these features lays out only the first pages for `Page(0)`.
`TestWholeLayoutUses` checks the features that the cascade finds.

`FuzzCascade` parses a document and a style sheet, computes each property of
each element, pseudo-element and page, and renders the counters. Then it
builds the boxes, splits each text box into lines of 3em, and lays out the
first three pages with `Document.Page`. Each page must come, or the error
must wrap `ErrNoPage` or `ivy.ErrLimit`. Then it draws each page with
`DrawPage` onto a pixmap of 64 by 64 pixels. The drawing must succeed, or
return an `*ivy.PartialError` that wraps `ivy.ErrLimit` or `ivy.ErrFormat`. Run it for a minute with
`go test -run '^$' -fuzz FuzzCascade -fuzztime 1m ./internal/layout`.

## Open problems

1. The matching of a selector such as `:has()` costs time in the square of
   the number of elements. The cascade bounds the work that it applies, not
   the work of the matcher of okage.
2. `split_first_line` shapes the rest of the text of a box for each line,
   as upstream lays it out again. A long text without the short text of
   upstream, such as a text of `white-space: pre-wrap` with a large width,
   costs time in the square of its length.
3. A `Book` takes the number of pages of a document to be the same
   whatever the number of its first page. A document whose page numbers
   change the breaks of its pages, such as with a long `counter(page)` in
   its content, can get a wrong count.
4. A gradient without `extend`, such as a repeating gradient, paints its
   first and its last colors past its domain. okage has no spread mode that
   paints nothing.
