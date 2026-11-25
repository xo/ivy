package msxls

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// chartRecords returns the records of a chart substream of a column chart
// of one series, with its values in the cache of the chart and the formats
// of Excel 97.
func chartRecords() []byte {
	var b biff
	b.record(recBOF, u16(biff8, bofChart, 0, 0), u32(0, 0))
	b.record(0x1002, make([]byte, 16))
	b.record(recBegin)
	b.record(recChartSeries, u16(1, 1, 3, 3, 1, 0))
	b.record(recBegin)
	b.record(recBRAI, []byte{0, 1}, u16(0, 0, 0))
	b.record(recSeriesText, u16(0), []byte{5, 0}, []byte("Sales"))
	b.record(recBRAI, []byte{1, 2}, u16(0, 0, 0))
	b.record(recDataFormat, u16(0xFFFF, 0, 0, 0))
	b.record(recBegin)
	// a red fill that is not automatic
	b.record(recAreaFormat, []byte{0xFF, 0, 0, 0}, make([]byte, 4), u16(1, 0, 10, 9))
	b.record(recEnd)
	b.record(recSerToCrt, u16(0))
	b.record(recEnd)
	b.record(recAxisParent, u16(0), make([]byte, 16))
	b.record(recBegin)
	b.record(recAxis, u16(0), make([]byte, 16))
	b.record(recAxis, u16(1), make([]byte, 16))
	b.record(recBegin)
	b.record(recAxisLine, u16(1))
	b.record(recLineFormat, []byte{0x80, 0x80, 0x80, 0}, u16(0, 0, 0, 23))
	b.record(recEnd)
	b.record(recChartFormat, make([]byte, 16), u16(0, 0))
	b.record(recBegin)
	b.record(recBar, u16(0, 150, 0))
	b.record(recLegend, make([]byte, 16), []byte{3, 1}, u16(0))
	b.record(recEnd)
	b.record(recEnd)
	b.record(recEnd)
	b.record(recSIIndex, u16(1))
	for i, v := range []float64{3, 1, 2} {
		b.record(recNumber, u16(i, 0, 0), float(v))
	}
	b.record(recEOF)
	return b.buf
}

func TestChartXML(t *testing.T) {
	w := &workbook{version: biff8, palette: defaultPalette, formats: map[int]string{}}
	var recs []record
	for rec := range records(chartRecords(), 0) {
		if rec.typ != recBOF && rec.typ != recEOF {
			recs = append(recs, rec)
		}
	}
	c := readChart(recs, w)
	if len(c.series) != 1 || len(c.groups) != 1 || c.groups[0].kind != "bar" {
		t.Fatalf("expected one series of a bar chart, got: %d series, %d groups", len(c.series), len(c.groups))
	}
	xml := c.xml(w, nil)
	for _, want := range []string{
		`<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>`,
		"<c:tx><c:v>Sales</c:v></c:tx>",
		`<c:spPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`,
		`<c:pt idx="0"><c:v>3</c:v></c:pt><c:pt idx="1"><c:v>1</c:v></c:pt><c:pt idx="2"><c:v>2</c:v></c:pt>`,
		`<c:gapWidth val="150"/>`,
		"<c:majorGridlines>",
		`<c:legendPos val="r"/>`,
		`<c:autoTitleDeleted val="1"/>`,
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("expected %q in the chart:\n%s", want, xml)
		}
	}
	if obj := w.chartObject(nil, recs, 300, 200, ivy.Config{}); obj == nil {
		t.Error("expected a chart object")
	}
}

// chartSheetWorkbook returns a workbook of a sheet of data and of a chart
// sheet, whose series takes its values from the cells Data!A1:A3, and the
// records of the chart.
func chartSheetWorkbook() ([]byte, []byte) {
	var b biff
	b.record(recBOF, u16(biff8, bofGlobals, 0, 0), u32(0, 0))
	b.record(recFont, u16(200, 0, 0x7FFF, 400, 0), []byte{0, 0, 0, 0}, []byte{5, 0}, []byte("Arial"))
	b.record(recXF, u16(0, 0), make([]byte, 16))
	pos1 := b.record(recBoundSheet, u32(0), []byte{0, 0}, []byte{4, 0}, []byte("Data"))
	pos2 := b.record(recBoundSheet, u32(0), []byte{0, 2}, []byte{5, 0}, []byte("Chart"))
	b.record(recSupBook, u16(2, 0x0401))
	b.record(recExternSheet, u16(1, 0, 0, 0))
	b.record(recEOF)
	binary.LittleEndian.PutUint32(b.buf[pos1+4:], uint32(len(b.buf)))
	b.record(recBOF, u16(biff8, bofWorksheet, 0, 0), u32(0, 0))
	for i := range 3 {
		b.record(recNumber, u16(i, 0, 0), float(float64(10*(i+1))))
	}
	b.record(recEOF)
	binary.LittleEndian.PutUint32(b.buf[pos2+4:], uint32(len(b.buf)))
	// the chart without its cache: the values are Data!A1:A3
	chart := chartRecords()
	area := append([]byte{ptgArea3d}, u16(0, 0, 2, 0, 0)...)
	chart = bytes.Replace(chart, append(u16(recBRAI, 8), append([]byte{1, 2}, u16(0, 0, 0)...)...),
		append(u16(recBRAI, 8+len(area)), append(append([]byte{1, 2}, u16(0, 0, len(area))...), area...)...), 1)
	if i := bytes.Index(chart, u16(recSIIndex, 2, 1)); i >= 0 {
		chart = append(chart[:i], u16(recEOF, 0)...)
	}
	b.buf = append(b.buf, chart...)
	return fixture.CFB(fixture.CFBEntry{Name: "Workbook", Data: b.buf}), chart
}

// TestChartSheet checks that a chart sheet draws its chart, and that a
// series takes its values from the cells of another sheet.
func TestChartSheet(t *testing.T) {
	buf, chart := chartSheetWorkbook()
	book := bookOf(t, buf)
	if len(book.Sheets) != 2 {
		t.Fatalf("expected 2 sheets, got: %d", len(book.Sheets))
	}
	img := book.Sheets[1].Images
	if len(img) != 1 || img[0].Object == "" || book.Objects[img[0].Object] == nil {
		t.Fatalf("expected the chart of the chart sheet, got: %+v", img)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	var recs []record
	for rec := range records(chart, 0) {
		if rec.typ != recBOF && rec.typ != recEOF {
			recs = append(recs, rec)
		}
	}
	look := &chartLookup{ctx: t.Context(), lim: d.cfg.Limits, w: d.wb}
	xml := readChart(recs, d.wb).xml(d.wb, look)
	if want := `<c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt><c:pt idx="2"><c:v>30</c:v></c:pt>`; !strings.Contains(xml, want) {
		t.Errorf("expected %q in the chart:\n%s", want, xml)
	}
}

// TestChartTrendTable checks that a series with SerParent and SerAuxTrend
// is the trendline of its parent, and not a series of its own, and that
// the record Dat gives the data table.
func TestChartTrendTable(t *testing.T) {
	chart := chartRecords()
	var b biff
	// a polynomial of the order 2 with its equation, and a forecast of 1
	b.record(recChartSeries, u16(1, 1, 0, 0, 1, 0))
	b.record(recBegin)
	b.record(recDataFormat, u16(0xFFFF, 0, 1, 0))
	b.record(recBegin)
	b.record(recLineFormat, []byte{0, 0x80, 0, 0}, u16(1, 0, 0, 0))
	b.record(recEnd)
	b.record(recSerParent, u16(1))
	b.record(recSerAuxTrend, []byte{0, 2}, []byte{0, 0, 0, 0, 0, 0, 0xFF, 0xFF}, []byte{1, 0}, float(1), float(0))
	b.record(recEnd)
	b.record(recDat, u16(0x0F))
	b.record(recBegin)
	b.record(recEnd)
	// the records go before the axis group
	at := bytes.Index(chart, u16(recAxisParent, 18))
	chart = append(append(append([]byte{}, chart[:at]...), b.buf...), chart[at:]...)
	w := &workbook{version: biff8, palette: defaultPalette, formats: map[int]string{}}
	var recs []record
	for rec := range records(chart, 0) {
		if rec.typ != recBOF && rec.typ != recEOF {
			recs = append(recs, rec)
		}
	}
	c := readChart(recs, w)
	if len(c.series) != 2 || c.series[1].parent != 1 || c.series[1].trend == nil || c.table == nil {
		t.Fatalf("expected a series with its trendline and a data table, got: %d series", len(c.series))
	}
	xml := c.xml(w, nil)
	for _, want := range []string{
		`<c:trendline><c:spPr><a:ln w="12700"><a:solidFill><a:srgbClr val="008000"/></a:solidFill>` +
			`<a:prstDash val="dash"/></a:ln></c:spPr><c:trendlineType val="poly"/><c:order val="2"/>` +
			`<c:forward val="1"/><c:dispRSqr val="0"/><c:dispEq val="1"/></c:trendline>`,
		`<c:dTable><c:showHorzBorder val="1"/><c:showVertBorder val="1"/><c:showOutline val="1"/><c:showKeys val="1"/></c:dTable>`,
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("expected %q in the chart:\n%s", want, xml)
		}
	}
	if n := strings.Count(xml, "<c:ser>"); n != 1 {
		t.Errorf("expected one series, got %d", n)
	}
}
