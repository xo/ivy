// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/testing_utils.py: the capture of the log, FakeHTML,
// BASE_URL, parse, parse_all, render_pages, serialize, assert_tree and the
// checks of the boxes.

package layout

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/xo/okage/html"
	"github.com/xo/okage/svg"
	"github.com/xo/okage/tinycss2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
)

// testBaseURL is BASE_URL in tests/testing_utils.py: the URL of a dummy file
// in the folder of the resources, which is testdata.
const testBaseURL = "file:///%3Ctest%3E"

// logCapture is capture_logs in tests/testing_utils.py. It is a handler of
// slog that keeps each message at a level or above, as "LEVEL: message".
type logCapture struct {
	mu       sync.Mutex
	level    slog.Level
	messages []string
}

// Enabled satisfies slog.Handler.
func (c *logCapture) Enabled(context.Context, slog.Level) bool { return true }

// Handle satisfies slog.Handler.
func (c *logCapture) Handle(_ context.Context, r slog.Record) error {
	if r.Level < c.level {
		return nil
	}
	name := "INFO"
	switch {
	case r.Level >= slog.LevelError:
		name = "ERROR"
	case r.Level >= slog.LevelWarn:
		name = "WARNING"
	case r.Level < slog.LevelInfo:
		name = "DEBUG"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, name+": "+r.Message)
	return nil
}

// WithAttrs satisfies slog.Handler.
func (c *logCapture) WithAttrs([]slog.Attr) slog.Handler { return c }

// WithGroup satisfies slog.Handler.
func (c *logCapture) WithGroup(string) slog.Handler { return c }

// logs returns the messages.
func (c *logCapture) logs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.messages...)
}

// captureLogs returns a logger that captures the messages at level or
// above, as capture_logs does. The default level of upstream is INFO.
func captureLogs(level slog.Level) (*logger, *logCapture) {
	c := &logCapture{level: level}
	return newLogger(slog.New(c)), c
}

// assertNoLogs fails the test for each message, as assert_no_logs does.
func assertNoLogs(t *testing.T, c *logCapture) {
	t.Helper()
	for _, message := range c.logs() {
		t.Errorf("unexpected log: %s", message)
	}
}

// assertLog checks that the log holds one message that contains want.
func assertLog(t *testing.T, c *logCapture, want string) {
	t.Helper()
	logs := c.logs()
	if len(logs) != 1 {
		t.Errorf("expected 1 log with %q, got: %q", want, logs)
		return
	}
	if !strings.Contains(logs[0], want) {
		t.Errorf("expected a log with %q, got: %q", want, logs[0])
	}
}

// testFS returns the files of testdata, the resources of upstream, with
// extra files.
func testFS(t *testing.T, extra map[string]string) fstest.MapFS {
	t.Helper()
	fsys := fstest.MapFS{}
	entries := []string{
		"doc1.html", "mini_ua.css", "user.css", "sheet2.css", "tests_ua.css",
		"logo_small.png", "sub_directory/sheet1.css",
	}
	for _, name := range entries {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		fsys[name] = &fstest.MapFile{Data: data}
	}
	for name, data := range extra {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

// testUAStylesheet is the text of tests_ua.css, TEST_UA_STYLESHEET in
// tests/testing_utils.py.
func testUAStylesheet(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/tests_ua.css")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// fakeHTML is FakeHTML(string=source, base_url=baseURL) in
// tests/testing_utils.py: a document of HTML text, with the files of
// testdata, which logs to log.
func fakeHTML(t *testing.T, source, baseURL string, log *logger) *HTML {
	t.Helper()
	// The string of FakeHTML(string=source) is already decoded.
	doc, err := html.ParseDocument(strings.NewReader(source), html.WithCharset("utf-8"))
	if err != nil {
		t.Fatal(err)
	}
	root := TreeFromHTML(doc, 0, DefaultMaxTreeDepth)
	fetcher := &URLFetcher{FS: testFS(t, nil)}
	h := NewHTML(root, baseURL, fetcher, HTMLOptions{})
	h.log = log
	return h
}

// fakeHTMLFile is FakeHTML(resource_path(name)): a document of testdata.
func fakeHTMLFile(t *testing.T, name string, log *logger) *HTML {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.ParseDocument(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	root := TreeFromHTML(doc, 0, DefaultMaxTreeDepth)
	fetcher := &URLFetcher{FS: testFS(t, nil)}
	h := NewHTML(root, pathToURL(name), fetcher, HTMLOptions{})
	h.log = log
	return h
}

// computedStyles is get_all_computed_styles of a FakeHTML: the styles of a
// document with the style sheet of the tests as the style sheet of the user
// agent.
func computedStyles(t *testing.T, h *HTML, opts ComputedStylesOptions) *StyleFor {
	t.Helper()
	if opts.UserAgentStylesheets == nil {
		opts.UserAgentStylesheets = []CSSSource{{Text: testUAStylesheet(t), BaseURL: pathToURL("tests_ua.css")}}
	}
	sf, err := GetAllComputedStyles(h, opts)
	if err != nil {
		t.Fatal(err)
	}
	return sf
}

// preprocessCSS is preprocess_declarations(base_url,
// tinycss2.parse_blocks_contents(css)) with the base URL of the tests of
// validation and expanders.
func preprocessCSS(log *logger, css string) []Declaration {
	return preprocessDeclarations(log, "https://weasyprint.org/foo/", tinycss2.ParseBlocksContents(css))
}

// findElement returns the first element with a tag, in the order of the
// document.
func findElement(root *Element, tag string) *Element {
	for el := range root.IterTag(tag) {
		return el
	}
	return nil
}

// defaultLimits are the limits of the tests.
var defaultLimits = ivy.Limits{}.WithDefaults()

// testFontDatabase is the font database of the tests: it holds only the Go
// fonts of golang.org/x/image, so that the tests do not depend on the fonts
// of the machine. Go is the serif, the sans-serif and the cursive family,
// and Go Mono the monospace family.
var testFontDatabase = sync.OnceValue(func() *svg.FontDatabase {
	db := svg.NewFontDatabase()
	for _, data := range [][]byte{
		goregular.TTF, gobold.TTF, goitalic.TTF, gobolditalic.TTF,
		gomono.TTF, gomonobold.TTF, gomonoitalic.TTF, gomonobolditalic.TTF,
	} {
		db.LoadFontData(data)
	}
	db.SetSerifFamily("Go")
	db.SetSansSerifFamily("Go")
	db.SetCursiveFamily("Go")
	db.SetFantasyFamily("Go")
	db.SetMonospaceFamily("Go Mono")
	return db
})

// testFontConfig is TEST_UA_FONT_CONFIG in tests/testing_utils.py: a font
// configuration with the fonts of the tests.
func testFontConfig(log *logger) *FontConfiguration {
	return NewFontConfiguration(testFontDatabase(), FontConfigurationOptions{log: log})
}

// testResources are the files of testdata that the tests of the boxes
// read, with the files of the CSS tests.
var testResources = []string{
	"pattern.png", "pattern.svg", "weasyprint.otf", "weasyprint.woff",
	"pattern.gif", "blue.jpg", "really-a-png.svg", "really-a-svg.png",
	"icon.png",
}

// parsed holds what _parse_base in tests/testing_utils.py returns.
type parsed struct {
	html     *HTML
	styleFor *StyleFor
	builder  *builder
	fc       *FontConfiguration
}

// parseBase is _parse_base in tests/testing_utils.py: a FakeHTML document,
// its styles, and the builder of its boxes.
func parseBase(t *testing.T, source, baseURL string, log *logger) parsed {
	t.Helper()
	h := fakeHTML(t, source, baseURL, log)
	fsys := h.URLFetcher.FS.(fstest.MapFS)
	for _, name := range testResources {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		fsys[name] = &fstest.MapFile{Data: data}
	}
	fc := testFontConfig(log)
	targetCollector := NewTargetCollector(log)
	counterStyle := CounterStyle{}
	styleFor := computedStyles(t, h, ComputedStylesOptions{
		FontConfig:      fc,
		CounterStyle:    counterStyle,
		TargetCollector: targetCollector,
	})
	b := newBuilder(h, styleFor, BuildOptions{TargetCollector: targetCollector, CounterStyle: counterStyle})
	return parsed{html: h, styleFor: styleFor, builder: b, fc: fc}
}

// parse is parse in tests/testing_utils.py: it parses some HTML, applies
// the style sheets and turns them into boxes, without the corrections of
// build_formatting_structure.
func parse(t *testing.T, source string) *Box {
	t.Helper()
	p := parseBase(t, source, testBaseURL, nil)
	boxes := p.builder.elementToBoxRoot(p.html.EtreeElement)
	if len(boxes) != 1 {
		t.Fatalf("element_to_box gives %d boxes", len(boxes))
	}
	return boxes[0]
}

// parseAll is parse_all in tests/testing_utils.py: it parses some HTML and
// runs all the corrections of the boxes.
func parseAll(t *testing.T, source string) *Box {
	t.Helper()
	return parseAllLog(t, source, testBaseURL, nil)
}

// parseAllLog is parseAll with a base URL and a logger.
func parseAllLog(t *testing.T, source, baseURL string, log *logger) *Box {
	t.Helper()
	p := parseBase(t, source, baseURL, log)
	box, err := p.builder.buildFormattingStructure(p.html.EtreeElement)
	if err != nil {
		t.Fatal(err)
	}
	sanityChecks(t, box)
	return box
}

// renderPages is render_pages in tests/testing_utils.py: it lays out a
// document and returns its page boxes.
func renderPages(t *testing.T, source string) []*Box {
	t.Helper()
	return renderPagesLog(t, source, testBaseURL, nil)
}

// renderPagesLog is renderPages with a base URL and a logger.
func renderPagesLog(t *testing.T, source, baseURL string, log *logger) []*Box {
	t.Helper()
	pages, err := renderPagesErr(t, source, baseURL, log)
	if err != nil {
		t.Fatal(err)
	}
	return pages
}

// renderPagesErr is renderPages that returns the error of the layout.
func renderPagesErr(t *testing.T, source, baseURL string, log *logger) ([]*Box, error) {
	t.Helper()
	p := parseBase(t, source, baseURL, log)
	root, err := p.builder.buildFormattingStructure(p.html.EtreeElement)
	if err != nil {
		return nil, err
	}
	doc := NewDocument(p.html, root, p.styleFor, DocumentOptions{
		FontConfig:      p.fc,
		TargetCollector: p.builder.targetCollector,
		CounterStyle:    p.builder.counterStyle,
		Footnotes:       p.builder.footnotes,
	})
	return doc.All(context.Background())
}

// unpack returns the children of box, and fails the test when box does not
// have n children. It is `a, b = box.children` of the tests of upstream.
func unpack(t *testing.T, box *Box, n int) []*Box {
	t.Helper()
	if len(box.Children) != n {
		t.Fatalf("%s has %d children, want %d: %v", box, len(box.Children), n, box.Children)
	}
	return box.Children
}

// only returns the only child of box, and fails the test when box does not
// have one child. It is `child, = box.children` of the tests of upstream.
func only(t *testing.T, box *Box) *Box {
	t.Helper()
	return unpack(t, box, 1)[0]
}

// pageBody returns the boxes of the html and of the body elements of a
// page: `html, = page.children` and `body, = html.children`.
func pageBody(t *testing.T, page *Box) (html, body *Box) {
	t.Helper()
	html = only(t, page)
	return html, only(t, html)
}

// assertApprox fails the test when got is not want, with a tolerance of
// 1e-6 times the larger value, or 1e-6. It is the comparison of
// pytest.approx.
func assertApprox(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6*max(1, math.Abs(want)) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

// pyTree is a serialized box of serialize in tests/testing_utils.py: the
// tag, the name of the class and the text, '<replaced>' or the children.
type pyTree struct {
	tag, typ string
	text     string
	leaf     bool
	children []pyTree
}

// String prints the tree as Python does.
func (p pyTree) String() string {
	if p.leaf {
		return fmt.Sprintf("(%s, %s, %s)", pyRepr(p.tag), pyRepr(p.typ), pyRepr(p.text))
	}
	return fmt.Sprintf("(%s, %s, %s)", pyRepr(p.tag), pyRepr(p.typ), pyTrees(p.children))
}

// pyTrees prints a list of trees as Python does.
func pyTrees(trees []pyTree) string {
	parts := make([]string, len(trees))
	for i, tree := range trees {
		parts[i] = tree.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// pyRepr prints a string as Python does.
func pyRepr(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, "'", `\'`, "\n", `\n`, "\t", `\t`).Replace(s) + "'"
}

// serialize is serialize in tests/testing_utils.py. It turns a list of boxes
// into a structure that is easier to compare in the tests.
func serialize(boxes []*Box) []pyTree {
	trees := make([]pyTree, 0, len(boxes))
	for _, box := range boxes {
		tree := pyTree{tag: box.ElementTag, typ: box.Kind.String()}
		// All concrete boxes are either text, replaced, column or parent.
		switch {
		case box.Kind == KindText:
			tree.text, tree.leaf = box.Text, true
		case box.IsReplaced():
			tree.text, tree.leaf = "<replaced>", true
		default:
			tree.children = serialize(append(slices.Clone(box.table().ColumnGroups), box.Children...))
		}
		trees = append(trees, tree)
	}
	return trees
}

// parsePyTrees parses a list of serialized boxes written as a literal of
// Python, such as [('p', 'Text', 'a')].
func parsePyTrees(t *testing.T, literal string) []pyTree {
	t.Helper()
	p := &pyLiteral{s: literal}
	value := p.value()
	p.space()
	if p.err != "" || p.i != len(p.s) {
		t.Fatalf("parsing the literal at %d: %s", p.i, p.err)
	}
	return toPyTrees(t, value)
}

// toPyTrees converts a parsed list to trees.
func toPyTrees(t *testing.T, value any) []pyTree {
	t.Helper()
	list, ok := value.([]any)
	if !ok {
		t.Fatalf("not a list: %v", value)
	}
	trees := make([]pyTree, len(list))
	for i, item := range list {
		tuple, ok := item.([]any)
		if !ok || len(tuple) != 3 {
			t.Fatalf("not a tuple of 3: %v", item)
		}
		trees[i] = pyTree{tag: tuple[0].(string), typ: tuple[1].(string)}
		if text, ok := tuple[2].(string); ok {
			trees[i].text, trees[i].leaf = text, true
		} else {
			trees[i].children = toPyTrees(t, tuple[2])
		}
	}
	return trees
}

// pyLiteral parses the lists, the tuples and the strings of Python.
type pyLiteral struct {
	s   string
	i   int
	err string
}

// space skips the spaces and the comments.
func (p *pyLiteral) space() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\n', '\t', '\r':
			p.i++
		case '#':
			for p.i < len(p.s) && p.s[p.i] != '\n' {
				p.i++
			}
		default:
			return
		}
	}
}

// value parses a list, a tuple or a string.
func (p *pyLiteral) value() any {
	p.space()
	if p.i >= len(p.s) {
		p.err = "unexpected end"
		return nil
	}
	switch c := p.s[p.i]; c {
	case '[', '(':
		end := byte(']')
		if c == '(' {
			end = ')'
		}
		p.i++
		items := []any{}
		for {
			p.space()
			if p.i < len(p.s) && p.s[p.i] == end {
				p.i++
				return items
			}
			items = append(items, p.value())
			if p.err != "" {
				return nil
			}
			p.space()
			if p.i < len(p.s) && p.s[p.i] == ',' {
				p.i++
			}
		}
	case '\'', '"':
		p.i++
		var b strings.Builder
		for p.i < len(p.s) && p.s[p.i] != c {
			if p.s[p.i] == '\\' && p.i+1 < len(p.s) {
				p.i++
				switch e := p.s[p.i]; e {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case 'u':
					n, _ := strconv.ParseUint(p.s[p.i+1:p.i+5], 16, 32)
					b.WriteRune(rune(n))
					p.i += 4
				case 'x':
					n, _ := strconv.ParseUint(p.s[p.i+1:p.i+3], 16, 32)
					b.WriteRune(rune(n))
					p.i += 2
				default:
					b.WriteByte(e)
				}
				p.i++
				continue
			}
			b.WriteByte(p.s[p.i])
			p.i++
		}
		p.i++
		return b.String()
	}
	p.err = fmt.Sprintf("unexpected %q", p.s[p.i])
	return nil
}

// assertTree is assert_tree in tests/testing_utils.py. It checks the box
// tree of a document: box is the box of <html>, and expected is the list of
// the serialized children of <body>, as a literal of Python.
func assertTree(t *testing.T, box *Box, expected string) {
	t.Helper()
	if box.ElementTag != "html" || box.Kind != KindBlock || len(box.Children) != 1 {
		t.Fatalf("bad root: %v with %d children", box, len(box.Children))
	}
	box = box.Children[0]
	if box.Kind != KindBlock || box.ElementTag != "body" {
		t.Fatalf("bad body: %v", box)
	}
	got := serialize(box.Children)
	want := parsePyTrees(t, expected)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("box tree:\n got: %s\nwant: %s", pyTrees(got), pyTrees(want))
	}
}

// properChildren is PROPER_CHILDREN in tests/testing_utils.py: the
// children that a box can have. The children can be of any kind of one of
// the lists.
func properChildren(box *Box) [][]func(*Box) bool {
	isBlockLevel := func(b *Box) bool { return b.IsBlockLevel() }
	isLine := func(b *Box) bool { return b.Kind == KindLine }
	isInlineLevel := func(b *Box) bool { return b.IsInlineLevel() }
	isKind := func(kinds ...Kind) func(*Box) bool {
		return func(b *Box) bool { return slices.Contains(kinds, b.Kind) }
	}
	switch {
	case box.IsBlockContainer():
		return [][]func(*Box) bool{{isBlockLevel}, {isLine}}
	case box.Kind == KindLine, box.Kind == KindInline:
		return [][]func(*Box) bool{{isInlineLevel}}
	case box.IsTable():
		return [][]func(*Box) bool{{isKind(KindTableCaption, KindTableColumnGroup, KindTableColumn, KindTableRowGroup, KindTableRow)}}
	case box.Kind == KindTableColumnGroup:
		return [][]func(*Box) bool{{isKind(KindTableColumn)}}
	case box.Kind == KindTableRowGroup:
		return [][]func(*Box) bool{{isKind(KindTableRow)}}
	case box.Kind == KindTableRow:
		return [][]func(*Box) bool{{isKind(KindTableCell)}}
	}
	return nil
}

// sanityChecks is _sanity_checks in tests/testing_utils.py. It checks that
// a block container holds either only block-level boxes or only line boxes,
// and that the line boxes and the inline boxes hold only inline-level
// boxes.
func sanityChecks(t *testing.T, box *Box) {
	t.Helper()
	if !box.IsParent() {
		return
	}
	acceptable := properChildren(box)
	if acceptable != nil {
		ok := false
		for _, tests := range acceptable {
			all := true
			for _, child := range box.Children {
				if !(slices.ContainsFunc(tests, func(test func(*Box) bool) bool { return test(child) }) || !child.IsInNormalFlow()) {
					all = false
					break
				}
			}
			if all {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("bad children of %v: %v", box, box.Children)
		}
	}
	for _, child := range box.Children {
		sanityChecks(t, child)
	}
}
