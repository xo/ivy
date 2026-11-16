// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/draw/test_table.py.

package layout

import "testing"

// assertPixelsTolerance is assertPixels with a tolerance on each channel,
// for the colors that okage and Ghostscript round in different ways.
func assertPixelsTolerance(t *testing.T, tolerance int, name, expectedPixels, html string) {
	t.Helper()
	expectedWidth, expectedHeight, expected := parsePixels(t, expectedPixels)
	width, height, pixels := htmlToPixels(t, html)
	if expectedWidth != width || expectedHeight != height {
		t.Fatalf("Images do not have the same sizes:\n- expected: %d × %d\n- result: %d × %d",
			expectedWidth, expectedHeight, width, height)
	}
	assertPixelsEqual(t, name, width, height, pixels, expected, tolerance)
}

// TestDrawTables1 is test_tables_1 in tests/draw/test_table.py.
func TestDrawTables1(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_1", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__ssssss_ssssss_ssssss__B_
        _B__s____s_s____s_s____s__B_
        _B__s____s_s____s_s____s__B_
        _B__s____s_s____s_s____s__B_
        _B__s____s_s____s_s____s__B_
        _B__ssssss_s____s_ssssss__B_
        _B_________s____s_________B_
        _B__sssssssSssssS_ssssss__B_
        _B__s______s____S_s____s__B_
        _B__s______s____S_s____s__B_
        _B__s______s____S_s____s__B_
        _B__s______s____S_s____s__B_
        _B__sssssssSSSSSS_ssssss__B_
        _B________________________B_
        _B__ssssss_ssssss_________B_
        _B__s____s_s____s_________B_
        _B__s____s_s____s_________B_
        _B__s____s_s____s_________B_
        _B__s____s_s____s_________B_
        _B__ssssss_ssssss_________B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed }
      td { border-color: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables1RTL is test_tables_1_rtl in tests/draw/test_table.py.
func TestDrawTables1RTL(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_1_rtl", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__ssssss_ssssss_ssssss__B_
        _B__s____s_s____s_s____s__B_
        _B__s____s_s____s_s____s__B_
        _B__s____s_s____s_s____s__B_
        _B__s____s_s____s_s____s__B_
        _B__ssssss_s____s_ssssss__B_
        _B_________s____s_________B_
        _B__ssssss_SssssSsssssss__B_
        _B__s____s_S____s______s__B_
        _B__s____s_S____s______s__B_
        _B__s____s_S____s______s__B_
        _B__s____s_S____s______s__B_
        _B__ssssss_SSSSSSsssssss__B_
        _B________________________B_
        _B_________ssssss_ssssss__B_
        _B_________s____s_s____s__B_
        _B_________s____s_s____s__B_
        _B_________s____s_s____s__B_
        _B_________s____s_s____s__B_
        _B_________ssssss_ssssss__B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed;
                direction: rtl; }
      td { border-color: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables2 is test_tables_2 in tests/draw/test_table.py.
func TestDrawTables2(t *testing.T) {
	assertPixels(t, "tables_2", `
        ____________________________
        _BBBBBBBBBBBBBBBBBB_________
        _BBBBBBBBBBBBBBBBBB_________
        _BB____s____s____BB_________
        _BB____s____s____BB_________
        _BB____s____s____BB_________
        _BB____s____s____BB_________
        _BBsssss____sssssBB_________
        _BB_________s____BB_________
        _BB_________s____BB_________
        _BB_________s____BB_________
        _BB_________s____BB_________
        _BBssssssssssssssBB_________
        _BB____s____s____BB_________
        _BB____s____s____BB_________
        _BB____s____s____BB_________
        _BB____s____s____BB_________
        _BBBBBBBBBBBBBBBBBB_________
        _BBBBBBBBBBBBBBBBBB_________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border: 2px solid #00f; table-layout: fixed;
                border-collapse: collapse }
      td { border-color: #ff7f7f }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables2RTL is test_tables_2_rtl in tests/draw/test_table.py.
func TestDrawTables2RTL(t *testing.T) {
	assertPixels(t, "tables_2_rtl", `
        ____________________________
        _________BBBBBBBBBBBBBBBBBB_
        _________BBBBBBBBBBBBBBBBBB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BBsssss____sssssBB_
        _________BB____s_________BB_
        _________BB____s_________BB_
        _________BB____s_________BB_
        _________BB____s_________BB_
        _________BBssssssssssssssBB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BBBBBBBBBBBBBBBBBB_
        _________BBBBBBBBBBBBBBBBBB_
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      body { direction: rtl; }
      table { border: 2px solid #00f; table-layout: fixed; border-collapse: collapse }
      td { border-color: #ff7f7f }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables3 is test_tables_3 in tests/draw/test_table.py.
func TestDrawTables3(t *testing.T) {
	assertPixels(t, "tables_3", `
        ____________________________
        _tttttttttttttttttttttttttt_
        _t________________________t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BB____s____s____BB_____t_
        _t_BB____s____s____BB_____t_
        _t_BB____s____s____BB_____t_
        _t_BB____s____s____BB_____t_
        _t_BBsssss____sssssBB_____t_
        _t_BB_________s____BB_____t_
        _t_BB_________s____BB_____t_
        _t_BB_________s____BB_____t_
        _t_BB_________s____BB_____t_
        _t_BBssssssssssssssBB_____t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _tttttttttttttttttttttttttt_
        ____________________________
        ____________________________
        _tttttttttttttttttttttttttt_
        _t_BBssssssssssssssBB_____t_
        _t_BB____s____s____BB_____t_
        _t_BB____s____s____BB_____t_
        _t_BB____s____s____BB_____t_
        _t_BB____s____s____BB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t_BBBBBBBBBBBBBBBBBB_____t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _tttttttttttttttttttttttttt_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border: solid #00f; border-width: 8px 2px;
              table-layout: fixed; border-collapse: collapse }
      td { border-color: #ff7f7f }
      @page { size: 28px 26px; margin: 1px;
              border: 1px solid rgba(0, 255, 0, 0.5); }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables3RTL is test_tables_3_rtl in tests/draw/test_table.py.
func TestDrawTables3RTL(t *testing.T) {
	assertPixels(t, "tables_3_rtl", `
        ____________________________
        _tttttttttttttttttttttttttt_
        _t________________________t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BBsssss____sssssBB_t_
        _t_____BB____s_________BB_t_
        _t_____BB____s_________BB_t_
        _t_____BB____s_________BB_t_
        _t_____BB____s_________BB_t_
        _t_____BBssssssssssssssBB_t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _tttttttttttttttttttttttttt_
        ____________________________
        ____________________________
        _tttttttttttttttttttttttttt_
        _t_____BBssssssssssssssBB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _tttttttttttttttttttttttttt_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      body { direction: rtl; }
      table { border: solid #00f; border-width: 8px 2px;
              table-layout: fixed; border-collapse: collapse }
      td { border-color: #ff7f7f }
      @page { size: 28px 26px; margin: 1px;
              border: 1px solid rgba(0, 255, 0, 0.5); }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables4 is test_tables_4 in tests/draw/test_table.py.
func TestDrawTables4(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_4", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B_________ssssss_________B_
        _B__sssssssSSSSSS_ssssss__B_
        _B__sssssssSSSSSS_ssssss__B_
        _B__sssssssSSSSSS_ssssss__B_
        _B__sssssssSSSSSS_ssssss__B_
        _B__sssssssSSSSSS_ssssss__B_
        _B__sssssssSSSSSS_ssssss__B_
        _B________________________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed }
      td { background: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables4RTL is test_tables_4_rtl in tests/draw/test_table.py.
func TestDrawTables4RTL(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_4_rtl", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B__ssssss_ssssss_ssssss__B_
        _B_________ssssss_________B_
        _B__ssssss_SSSSSSsssssss__B_
        _B__ssssss_SSSSSSsssssss__B_
        _B__ssssss_SSSSSSsssssss__B_
        _B__ssssss_SSSSSSsssssss__B_
        _B__ssssss_SSSSSSsssssss__B_
        _B__ssssss_SSSSSSsssssss__B_
        _B________________________B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed; direction: rtl }
      td { background: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables5 is test_tables_5 in tests/draw/test_table.py.
func TestDrawTables5(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_5", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B_________uuuuuu_________B_
        _B__uuuuuuupppppp_uuuuuu__B_
        _B__uuuuuuupppppp_uuuuuu__B_
        _B__uuuuuuupppppp_uuuuuu__B_
        _B__uuuuuuupppppp_uuuuuu__B_
        _B__uuuuuuupppppp_uuuuuu__B_
        _B__uuuuuuupppppp_uuuuuu__B_
        _B________________________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B__ssssss_ssssss_________B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed }
      #tbody { background: rgba(0, 0, 255, 1) }
      tr { background: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables5RTL is test_tables_5_rtl in tests/draw/test_table.py.
func TestDrawTables5RTL(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_5_rtl", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B_________uuuuuu_________B_
        _B__uuuuuu_ppppppuuuuuuu__B_
        _B__uuuuuu_ppppppuuuuuuu__B_
        _B__uuuuuu_ppppppuuuuuuu__B_
        _B__uuuuuu_ppppppuuuuuuu__B_
        _B__uuuuuu_ppppppuuuuuuu__B_
        _B__uuuuuu_ppppppuuuuuuu__B_
        _B________________________B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B_________ssssss_ssssss__B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed; direction: rtl }
      #tbody { background: rgba(0, 0, 255, 1) }
      tr { background: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables6 is test_tables_6 in tests/draw/test_table.py.
func TestDrawTables6(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_6", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__uuuuuu_uuuuuu_ssssss__B_
        _B__uuuuuu_uuuuuu_ssssss__B_
        _B__uuuuuu_uuuuuu_ssssss__B_
        _B__uuuuuu_uuuuuu_ssssss__B_
        _B__uuuuuu_uuuuuu_ssssss__B_
        _B__uuuuuu_uuuuuu_ssssss__B_
        _B_________uuuuuu_________B_
        _B__uuuuuuupppppp_ssssss__B_
        _B__uuuuuuupppppp_ssssss__B_
        _B__uuuuuuupppppp_ssssss__B_
        _B__uuuuuuupppppp_ssssss__B_
        _B__uuuuuuupppppp_ssssss__B_
        _B__uuuuuuupppppp_ssssss__B_
        _B________________________B_
        _B__uuuuuu_uuuuuu_________B_
        _B__uuuuuu_uuuuuu_________B_
        _B__uuuuuu_uuuuuu_________B_
        _B__uuuuuu_uuuuuu_________B_
        _B__uuuuuu_uuuuuu_________B_
        _B__uuuuuu_uuuuuu_________B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed;}
      .colgroup { background: rgba(0, 0, 255, 1) }
      col { background: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables6RTL is test_tables_6_rtl in tests/draw/test_table.py.
func TestDrawTables6RTL(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_6_rtl", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__ssssss_uuuuuu_uuuuuu__B_
        _B__ssssss_uuuuuu_uuuuuu__B_
        _B__ssssss_uuuuuu_uuuuuu__B_
        _B__ssssss_uuuuuu_uuuuuu__B_
        _B__ssssss_uuuuuu_uuuuuu__B_
        _B__ssssss_uuuuuu_uuuuuu__B_
        _B_________uuuuuu_________B_
        _B__ssssss_ppppppuuuuuuu__B_
        _B__ssssss_ppppppuuuuuuu__B_
        _B__ssssss_ppppppuuuuuuu__B_
        _B__ssssss_ppppppuuuuuuu__B_
        _B__ssssss_ppppppuuuuuuu__B_
        _B__ssssss_ppppppuuuuuuu__B_
        _B________________________B_
        _B_________uuuuuu_uuuuuu__B_
        _B_________uuuuuu_uuuuuu__B_
        _B_________uuuuuu_uuuuuu__B_
        _B_________uuuuuu_uuuuuu__B_
        _B_________uuuuuu_uuuuuu__B_
        _B_________uuuuuu_uuuuuu__B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed; direction: rtl }
      .colgroup { background: rgba(0, 0, 255, 1) }
      col { background: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables7 is test_tables_7 in tests/draw/test_table.py.
func TestDrawTables7(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_7", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uBBBBu_uBBBBu_uBBBBu__B_
        _B__uBBBBu_uBBBBu_uBBBBu__B_
        _B__uBBBBu_uBBBBu_uBBBBu__B_
        _B__uBBBBu_uBBBBu_uBBBBu__B_
        _B__uuuuuu_uBBBBu_uuuuuu__B_
        _B_________uBBBBu_________B_
        _B__ssssssspuuuup_ssssss__B_
        _B__s______uBBBBp_s____s__B_
        _B__s______uBBBBp_s____s__B_
        _B__s______uBBBBp_s____s__B_
        _B__s______uBBBBp_s____s__B_
        _B__ssssssspppppp_ssssss__B_
        _B________________________B_
        _B__ssssss_ssssss_________B_
        _B__s____s_s____s_________B_
        _B__s____s_s____s_________B_
        _B__s____s_s____s_________B_
        _B__s____s_s____s_________B_
        _B__ssssss_ssssss_________B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed }
      #tbody tr:first-child { background: blue }
      td { border-color: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables7RTL is test_tables_7_rtl in tests/draw/test_table.py.
func TestDrawTables7RTL(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_7_rtl", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__uuuuuu_uuuuuu_uuuuuu__B_
        _B__uBBBBu_uBBBBu_uBBBBu__B_
        _B__uBBBBu_uBBBBu_uBBBBu__B_
        _B__uBBBBu_uBBBBu_uBBBBu__B_
        _B__uBBBBu_uBBBBu_uBBBBu__B_
        _B__uuuuuu_uBBBBu_uuuuuu__B_
        _B_________uBBBBu_________B_
        _B__ssssss_puuuupsssssss__B_
        _B__s____s_pBBBBu______s__B_
        _B__s____s_pBBBBu______s__B_
        _B__s____s_pBBBBu______s__B_
        _B__s____s_pBBBBu______s__B_
        _B__ssssss_ppppppsssssss__B_
        _B________________________B_
        _B_________ssssss_ssssss__B_
        _B_________s____s_s____s__B_
        _B_________s____s_s____s__B_
        _B_________s____s_s____s__B_
        _B_________s____s_s____s__B_
        _B_________ssssss_ssssss__B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed; direction: rtl }
      #tbody tr:first-child { background: blue }
      td { border-color: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables8 is test_tables_8 in tests/draw/test_table.py.
func TestDrawTables8(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_8", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__uuuuuu_ssssss_ssssss__B_
        _B__uBBBBu_s____s_s____s__B_
        _B__uBBBBu_s____s_s____s__B_
        _B__uBBBBu_s____s_s____s__B_
        _B__uBBBBu_s____s_s____s__B_
        _B__uuuuuu_s____s_ssssss__B_
        _B_________s____s_________B_
        _B__uuuuuuupuuuup_ssssss__B_
        _B__uBBBBBBuBBBBp_s____s__B_
        _B__uBBBBBBuBBBBp_s____s__B_
        _B__uBBBBBBuBBBBp_s____s__B_
        _B__uBBBBBBuBBBBp_s____s__B_
        _B__uuuuuuupppppp_ssssss__B_
        _B________________________B_
        _B__uuuuuu_ssssss_________B_
        _B__uBBBBu_s____s_________B_
        _B__uBBBBu_s____s_________B_
        _B__uBBBBu_s____s_________B_
        _B__uBBBBu_s____s_________B_
        _B__uuuuuu_ssssss_________B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed }
      .colgroup col:first-child { background: blue }
      td { border-color: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables8RTL is test_tables_8_rtl in tests/draw/test_table.py.
func TestDrawTables8RTL(t *testing.T) {
	// Adapted: okage rounds the color of two layers of 50% red, 63.75,
	// up to 64, where Ghostscript gives 63.
	assertPixelsTolerance(t, 1, "tables_8_rtl", `
        ____________________________
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        _B________________________B_
        _B________________________B_
        _B__ssssss_ssssss_uuuuuu__B_
        _B__s____s_s____s_uBBBBu__B_
        _B__s____s_s____s_uBBBBu__B_
        _B__s____s_s____s_uBBBBu__B_
        _B__s____s_s____s_uBBBBu__B_
        _B__ssssss_s____s_uuuuuu__B_
        _B_________s____s_________B_
        _B__ssssss_puuuupuuuuuuu__B_
        _B__s____s_pBBBBuBBBBBBu__B_
        _B__s____s_pBBBBuBBBBBBu__B_
        _B__s____s_pBBBBuBBBBBBu__B_
        _B__s____s_pBBBBuBBBBBBu__B_
        _B__ssssss_ppppppuuuuuuu__B_
        _B________________________B_
        _B_________ssssss_uuuuuu__B_
        _B_________s____s_uBBBBu__B_
        _B_________s____s_uBBBBu__B_
        _B_________s____s_uBBBBu__B_
        _B_________s____s_uBBBBu__B_
        _B_________ssssss_uuuuuu__B_
        _B________________________B_
        _B________________________B_
        _BBBBBBBBBBBBBBBBBBBBBBBBBB_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      table { border-color: #00f; table-layout: fixed; direction: rtl }
      .colgroup col:first-child { background: blue }
      td { border-color: rgba(255, 0, 0, 0.5) }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables9 is test_tables_9 in tests/draw/test_table.py.
func TestDrawTables9(t *testing.T) {
	assertPixels(t, "tables_9", `
        ______________________
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBB____R____R____BBB_
        _BBB____R____R____BBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        __R_____R____R_____R__
        __R_____R____R_____R__
        __RRRRRRRRRRRRRRRRRR__
        __R_____R____R_____R__
        __R_____R____R_____R__
        __RRRRRRRRRRRRRRRRRR__
        ______________________
        ______________________
        ______________________
        ______________________
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBB____R____R____BBB_
        _BBB____R____R____BBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        __R_____R____R_____R__
        __RRRRRRRRRRRRRRRRRR__
        ______________________
        ______________________
        ______________________
        ______________________
        ______________________
        ______________________
        ______________________
    `, `
      <style>
        @page { size: 22px 18px; margin: 1px }
        td { border: 1px red solid; width: 4px; height: 2px; }
      </style>
      <table style="table-layout: fixed; border-collapse: collapse">
        <thead style="border: blue solid; border-width: 3px;
            "><td></td><td></td><td></td></thead>
        <tr><td></td><td></td><td></td></tr>
        <tr><td></td><td></td><td></td></tr>
        <tr><td></td><td></td><td></td></tr>`)
}

// TestDrawTables10 is test_tables_10 in tests/draw/test_table.py.
func TestDrawTables10(t *testing.T) {
	assertPixels(t, "tables_10", `
        ______________________
        __RRRRRRRRRRRRRRRRRR__
        __R_____R____R_____R__
        __R_____R____R_____R__
        __RRRRRRRRRRRRRRRRRR__
        __R_____R____R_____R__
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBB____R____R____BBB_
        _BBB____R____R____BBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        ______________________
        ______________________
        ______________________
        ______________________
        __RRRRRRRRRRRRRRRRRR__
        __R_____R____R_____R__
        __R_____R____R_____R__
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBB____R____R____BBB_
        _BBB____R____R____BBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        ______________________
        ______________________
        ______________________
        ______________________
        ______________________
    `, `
      <style>
        @page { size: 22px 17px; margin: 1px }
        td { border: 1px red solid; width: 4px; height: 2px; }
      </style>
      <table style="table-layout: fixed; margin-left: 1px; border-collapse: collapse">
        <tr><td></td><td></td><td></td></tr>
        <tr><td></td><td></td><td></td></tr>
        <tr><td></td><td></td><td></td></tr>
        <tfoot style="border: blue solid; border-width: 3px;
            "><td></td><td></td><td></td></tfoot>`)
}

// TestDrawTables11 is test_tables_11 in tests/draw/test_table.py.
func TestDrawTables11(t *testing.T) {
	// Regression test for #82.
	assertPixels(t, "tables_11", `
      ____________________
      ________RRRRRRRRRRR_
      ________R____R____R_
      ________R____R____R_
      ________R____R____R_
      ________RRRRRRRRRRR_
      ____________________
      ____________________
      ____________________
      ____________________
    `, `
      <style>
        @page { size: 20px 10px; margin: 1px }
        body { text-align: right; font-size: 0 }
        table { display: inline-table; width: 11px }
        td { border: 1px red solid; width: 4px; height: 3px }
      </style>
      <table style="table-layout: fixed; border-collapse: collapse">
        <tr><td></td><td></td></tr>`)
}

// TestDrawTables12 is test_tables_12 in tests/draw/test_table.py.
func TestDrawTables12(t *testing.T) {
	assertPixels(t, "tables_12", `
        ____________________________
        _________BBBBBBBBBBBBBBBBBB_
        _________BBBBBBBBBBBBBBBBBB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BBsssss____sssssBB_
        _________BB____s_________BB_
        _________BB____s_________BB_
        _________BB____s_________BB_
        _________BB____s_________BB_
        _________BBssssssssssssssBB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BB____s____s____BB_
        _________BBBBBBBBBBBBBBBBBB_
        _________BBBBBBBBBBBBBBBBBB_
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      body { direction: rtl }
      table { border: 2px solid #00f; table-layout: fixed; border-collapse: collapse }
      td { border-color: #ff7f7f }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables13 is test_tables_13 in tests/draw/test_table.py.
func TestDrawTables13(t *testing.T) {
	assertPixels(t, "tables_13", `
        ____________________________
        _tttttttttttttttttttttttttt_
        _t________________________t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BBsssss____sssssBB_t_
        _t_____BB____s_________BB_t_
        _t_____BB____s_________BB_t_
        _t_____BB____s_________BB_t_
        _t_____BB____s_________BB_t_
        _t_____BBssssssssssssssBB_t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _tttttttttttttttttttttttttt_
        ____________________________
        ____________________________
        _tttttttttttttttttttttttttt_
        _t_____BBssssssssssssssBB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BB____s____s____BB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t_____BBBBBBBBBBBBBBBBBB_t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _t________________________t_
        _tttttttttttttttttttttttttt_
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      body { direction: rtl }
      table { border: solid #00f; border-width: 8px 2px;
              table-layout: fixed; border-collapse: collapse }
      td { border-color: #ff7f7f }
      @page { size: 28px 26px; margin: 1px;
              border: 1px solid rgba(0, 255, 0, 0.5); }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables14 is test_tables_14 in tests/draw/test_table.py.
func TestDrawTables14(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "tables_14", `
        ____________________________
        _RRR_RRR_RRR________________
        _RRR_RRR_RRR________________
        _RRR_RRR_RRR________________
        _RRR_RRR_RRR________________
        _RRR_RRR_RRR________________
        _RRR_RRR_RRR________________
        _RRR_RRR_RRR________________
        _RRR_RRR_RRR________________
        _RRR_RRR_RRR________________
        _RRR_RRR_RRR________________
        _____RRR____________________
        _RRRRRRR_RRR________________
        _RRRRRRR_RRR________________
        _RRRRRRR_RRR________________
        _RRRRRRR_RRR________________
        _RRRRRRR_RRR________________
        _RRRRRRR_RRR________________
        _RRRRRRR_RRR________________
        _RRRRRRR_RRR________________
        _RRRRRRR_RRR________________
        _RRRRRRR_RRR________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        _RRR_RRR____________________
        _RRR_RRR____________________
        _RRR_RRR____________________
        _RRR_RRR____________________
        _RRR_RRR____________________
        _RRR_RRR____________________
        _RRR_RRR____________________
        _RRR_RRR____________________
        _RRR_RRR____________________
        _RRR_RRR____________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
        ____________________________
    `, `
  <style>
    @page { size: 28px }
    table { margin: 1px; padding: 1px; border-spacing: 1px;
            border: 1px solid transparent }
    td { width: 2px; height: 2px; padding: 1px; border: 1px solid transparent }
    
      @page { size: 28px 26px }
      table { margin: 0; padding: 0; border: 0 }
      col { background: red }
      td { padding: 0; width: 1px; height: 8px }
    
  </style>
  <table>
    <colgroup class=colgroup>
      <col></col>
      <col></col>
    </colgroup>
    <col></col>
    <tbody id=tbody>
      <tr>
        <td></td>
        <td rowspan=2></td>
        <td></td>
      </tr>
      <tr>
        <td colspan=2></td>
        <td></td>
      </tr>
    </tbody>
    <tr>
      <td></td>
      <td></td>
    </tr>
  </table>
`)
}

// TestDrawTables15 is test_tables_15 in tests/draw/test_table.py.
func TestDrawTables15(t *testing.T) {
	// Regression test for #1250.
	assertPixels(t, "tables_15", `
        ______________________
        __RRRRRRRRRRRRRRRRRR__
        __R_____R____R_____R__
        __R_____R____R_____R__
        __R_____R____R_____R__
        __RRRRRRRRRRRRRRRRRR__
        __R_____R____R_____R__
        __R_____R____R_____R__
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBB____R____R____BBB_
        _BBB____R____R____BBB_
        _BBB____R____R____BBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        ______________________
        ______________________
        __RRRRRRRRRRRRRRRRRR__
        __R________________R__
        __R________________R__
        __R________________R__
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBB____R____R____BBB_
        _BBB____R____R____BBB_
        _BBB____R____R____BBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        _BBBBBBBBBBBBBBBBBBBB_
        ______________________
        ______________________
        ______________________
        ______________________
    `, `
      <style>
        @page { size: 22px 18px; margin: 1px }
        td { border: 1px red solid; width: 4px; height: 3px; }
      </style>
      <table style="table-layout: fixed; margin-left: 1px; border-collapse: collapse">
        <tr><td></td><td></td><td></td></tr>
        <tr><td></td><td></td><td></td></tr>
        <tr><td colspan="3"></td></tr>
        <tfoot style="border: blue solid; border-width: 3px;
            "><td></td><td></td><td></td></tfoot>`)
}

// TestDrawTables16 is test_tables_16 in tests/draw/test_table.py.
func TestDrawTables16(t *testing.T) {
	assertPixels(t, "tables_16", `
      ____________________
      _RRRRRRRRRRR________
      _R____R____R________
      _R____R____R________
      _R____R_RRRRRRRRRRR_
      _RRRRRRRRRRR_R____R_
      ________R____R____R_
      ________R____R____R_
      ________RRRRRRRRRRR_
      ____________________
    `, `
      <style>
        @page { size: 20px 10px; margin: 1px }
        body { text-align: right; font-size: 0 }
        table { position: absolute; width: 11px;
                table-layout: fixed; border-collapse: collapse }
        td { border: 1px red solid; width: 4px; height: 3px }
      </style>
      <table style="top: 0; left: 0">
        <tr><td></td><td></td></tr>
      <table style="bottom: 0; right: 0">
        <tr><td></td><td></td></tr>`)
}

// TestDrawTables17 is test_tables_17 in tests/draw/test_table.py.
func TestDrawTables17(t *testing.T) {
	assertPixels(t, "tables_17", `
      ________________
      _RRRRRRRRRRRRRR_
      _RRRRRRRRRRRRRR_
      _RR____RR____RR_
      _RR_BB_RR_BB_RR_
      _RR_BB_RR_BB_RR_
      _RR_BB_RR____RR_
      _RR_BB_RR____RR_
      _RR____RR____RR_
      ________________
      ________________
      _RR_BB_RR____RR_
      _RR_BB_RR____RR_
      _RR_BB_RR____RR_
      _RR_BB_RR____RR_
      _RR____RR____RR_
      _RRRRRRRRRRRRRR_
      _RRRRRRRRRRRRRR_
      ________________
      ________________
    `, `
      <style>
        @page { size: 16px 10px; margin: 1px }
        table { border-collapse: collapse; font-size: 2px; line-height: 1;
                color: blue; font-family: weasyprint }
        td { border: 2px red solid; padding: 1px; line-height: 1 }
      </style>
      <table><tr><td>a a a a</td><td>a</td></tr>`)
}

// TestDrawTables18 is test_tables_18 in tests/draw/test_table.py.
func TestDrawTables18(t *testing.T) {
	assertPixels(t, "tables_18", `
      ____________
      _RRRRRRRRRR_
      _R________R_
      _R_RRRRRR_R_
      _R_R____R_R_
      _R_R_BB_R_R_
      _R_R_BB_R_R_
      _R_R_BB_R_R_
      _R_R_BB_R_R_
      _R_R____R_R_
      ____________
      ____________
      _R_R_BB_R_R_
      _R_R_BB_R_R_
      _R_R_BB_R_R_
      _R_R_BB_R_R_
      _R_R____R_R_
      _R_RRRRRR_R_
      _R________R_
      _RRRRRRRRRR_
      ____________
      ____________
    `, `
      <style>
        @page { size: 12px 11px; margin: 1px }
        table { border: 1px red solid; border-spacing: 1px; font-size: 2px;
                line-height: 1; color: blue; font-family: weasyprint }
        td { border: 1px red solid; padding: 1px; line-height: 1; }
      </style>
      <table><tr><td>a a a a</td></tr>`)
}

// TestDrawTables19 is test_tables_19 in tests/draw/test_table.py.
func TestDrawTables19(t *testing.T) {
	// Regression test for #1523.
	assertPixels(t, "tables_19", `
      RR
      RR
      RR
      RR
      RR
      RR
      RR
      RR
    `, `
      <style>
        @page { size: 2px 4px }
        table { border-collapse: collapse; color: red }
        body { font-size: 2px; font-family: weasyprint; line-height: 1 }
      </style>
      <table><tr><td>a a a a</td></tr></table>`)
}

// TestDrawTables20 is test_tables_20 in tests/draw/test_table.py.
func TestDrawTables20(t *testing.T) {
	assertPixels(t, "tables_20", `
      ____________________
      _RRRRRRRRRRRR_______
      _RBBBBBBBBBBR_______
      _RRRRRRRRRRRR_______
      ____________________
    `, `
      <style>
        @page { size: 20px 5px; margin: 1px }
        table { width: 10px; border: 1px red solid }
        td { height: 1px; background: blue }
        col, tr, tbody, tfoot { background: lime }
      </style>
      <table>
      <col></col><col></col>
      <tbody><tr></tr><tr><td></td></tr></tbody>
      <tfoot></tfoot>`)
}

// TestDrawTables21 is test_tables_21 in tests/draw/test_table.py.
func TestDrawTables21(t *testing.T) {
	assertPixels(t, "tables_21", `
      _________________________
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _rBBBBBBBBBBrBBBBBBBBBBr_
      _rBKKKKKKBBBrBKKKKKKBBBr_
      _rBKKKKKKBBBrBKKKKKKBBBr_
      _rBBBBBBBBBBrBBBBBBBBBBr_
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _________________________
      _________________________
      _________________________
      _________________________
      _________________________
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _rBBBBBBBBBBrBBBBBBBBBBr_
      _rBKKKKKKBBBrBBBBBBBBBBr_
      _rBKKKKKKBBBrBBBBBBBBBBr_
      _rBBBBBBBBBBrBBBBBBBBBBr_
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _________________________
      _________________________
      _________________________
      _________________________
    `, `
      <style>
        @page { size: 25px 11px; margin: 1px }
        table { border-collapse: collapse; font: 2px weasyprint; width: 100% }
        td { background: blue; padding: 1px; border: 1px solid red }
      </style>
      <table>
        <tr><td>abc</td><td>abc</td></tr>
        <tr><td>abc</td><td></td></tr>`)
}

// TestDrawTables22 is test_tables_22 in tests/draw/test_table.py.
func TestDrawTables22(t *testing.T) {
	assertPixels(t, "tables_22", `
      _________________________
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _rKKKKKKKKKKrKKKKKKKKKKr_
      _rKKKKKKKKKKrKKKKKKKKKKr_
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _rKKKKKKBBBBrBBBBBBBBBBr_
      _rKKKKKKBBBBrBBBBBBBBBBr_
      _rBBBBBBBBBBrBBBBBBBBBBr_
      _________________________
      _________________________
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _rKKKKKKKKKKrKKKKKKKKKKr_
      _rKKKKKKKKKKrKKKKKKKKKKr_
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _rKKKKKKBBBBrBBBBBBBBBBr_
      _rKKKKKKBBBBrBBBBBBBBBBr_
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _________________________
    `, `
      <style>
        @page { size: 25px 9px; margin: 1px }
        table { border-collapse: collapse; font: 2px/1 weasyprint }
        td { background: blue; border: 1px solid red }
      </style>
      <table>
        <thead><tr><td>abcde</td><td>abcde</td></tr></thead>
        <tbody><tr><td>abc abc</td><td></td></tr></tbody>`)
}

// TestDrawTables23 is test_tables_23 in tests/draw/test_table.py.
func TestDrawTables23(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "tables_23", `
      _________________________
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _rKKKKKKKKKKrKKKKKKKKKKr_
      _rKKKKKKKKKKrKKKKKKKKKKr_
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _rKKKKKKBBBBrBBBBBBBBBBr_
      _rKKKKKKBBBBrBBBBBBBBBBr_
      _rBBBBBBBBBBrBBBBBBBBBBr_
      _________________________
      _________________________
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _rKKKKKKKKKKrKKKKKKKKKKr_
      _rKKKKKKKKKKrKKKKKKKKKKr_
      _rKKKKKKBBBBrBBBBBBBBBBr_
      _rKKKKKKBBBBrBBBBBBBBBBr_
      _rrrrrrrrrrrrrrrrrrrrrrr_
      _________________________
      _________________________
    `, `
      <style>
        @page { size: 25px 9px; margin: 1px }
        table { border-collapse: collapse; font: 2px/1 weasyprint }
        td { background: blue; border: 1px solid red }
        thead td { border-bottom: none }
      </style>
      <table>
        <thead><tr><td>abcde</td><td>abcde</td></tr></thead>
        <tbody><tr><td>abc abc</td><td></td></tr></tbody>`)
}

// TestDrawTables24 is test_tables_24 in tests/draw/test_table.py.
func TestDrawTables24(t *testing.T) {
	assertPixels(t, "tables_24", `
        __________________
        _RKKKKgYYYYYYGGG__
        _RKKKKgKKYYKKGGG__
        _BBBBBBKKYYKKGGG__
        _BBBBBBYYYYYYGGG__
        _BBBBBBCCCCCCGGG__
        _BBBBBB___________
        _BBBBBB___________
        __________________
        __________________
    `, `
      <style>
        @page { size: 18px 10px }
        table {
          border-collapse: collapse;
          font: 2px/1 weasyprint;
          margin: 1px;
        }
        tr {
          border-left: 1px solid red;
          border-right: 3px solid lime;
        }
        td.left {
          background-color: magenta;
          border-bottom: 5px solid blue;
          border-right: 1px solid green;
        }
        td.right {
          background-color: yellow;
          border-bottom: 1px solid cyan;
          border-left: 1px dotted orange;
        }
        td {
          vertical-align: middle;
        }
      </style>
      <table>
        <tr>
          <td class="left">XX</td>
          <td class="right">X X</td>
        </tr>
      </table>
    `)
}

// TestDrawTables24RTL is test_tables_24_rtl in tests/draw/test_table.py.
func TestDrawTables24RTL(t *testing.T) {
	assertPixels(t, "tables_24_rtl", `
        __________________
        _RKKKKgYYYYYYGGG__
        _RKKKKgKKYYKKGGG__
        _BBBBBBKKYYKKGGG__
        _BBBBBBYYYYYYGGG__
        _BBBBBBCCCCCCGGG__
        _BBBBBB___________
        _BBBBBB___________
        __________________
        __________________
    `, `
      <style>
        @page { size: 18px 10px }
        table {
          border-collapse: collapse;
          direction: rtl;
          font: 2px/1 weasyprint;
          margin: 1px;
        }
        tr {
          border-left: 1px solid red;
          border-right: 3px solid lime;
        }
        td.left {
          background-color: magenta;
          border-bottom: 5px solid blue;
          border-right: 1px solid green;
        }
        td.right {
          background-color: yellow;
          border-bottom: 1px solid cyan;
          border-left: 1px dotted orange;
        }
        td {
          vertical-align: middle;
        }
      </style>
      <table>
        <tr>
          <td class="right">X X</td>
          <td class="left">XX</td>
        </tr>
      </table>
    `)
}

// TestDrawRunningElementsTableBorderCollapse is test_running_elements_table_border_collapse in tests/draw/test_table.py.
func TestDrawRunningElementsTableBorderCollapse(t *testing.T) {
	assertPixels(t, "running_elements_table_border_collapse", `
      KK_____________
      KK_____________
      _______________
      _______________
      _______________
      KKKKKKK________
      KRRKRRK________
      KRRKRRK________
      KKKKKKK________
      KRRKRRK________
      KRRKRRK________
      KKKKKKK________
      _______________
      _______________
      _______________
    
      KK_____________
      KK_____________
      _______________
      _______________
      _______________
      KKKKKKK________
      KRRKRRK________
      KRRKRRK________
      KKKKKKK________
      KRRKRRK________
      KRRKRRK________
      KKKKKKK________
      _______________
      _______________
      _______________
    `, `
      <style>
        @page {
          margin: 0 0 10px 0;
          size: 15px;
          @bottom-left { content: element(table) }
        }
        body { font: 2px/1 weasyprint }
        table {
          border: 1px solid black;
          border-collapse: collapse;
          color: red;
          position: running(table);
        }
        td { border: 1px solid black }
        div { page-break-after: always }
      </style>
      <table>
        <tr> <td>A</td> <td>B</td> </tr>
        <tr> <td>C</td> <td>D</td> </tr>
      </table>
      <div>1</div>
      <div>2</div>
    `)
}

// TestDrawRunningElementsTableBorderCollapseEmpty is test_running_elements_table_border_collapse_empty in tests/draw/test_table.py.
func TestDrawRunningElementsTableBorderCollapseEmpty(t *testing.T) {
	assertPixels(t, "running_elements_table_border_collapse_empty", `
      KK________
      KK________
      __________
      __________
      __________
      __________
      __________
      __________
      __________
      __________
    
      KK________
      KK________
      __________
      __________
      __________
      __________
      __________
      __________
      __________
      __________
    `, `
      <style>
        @page {
          margin: 0 0 5px 0;
          size: 10px;
          @bottom-left { content: element(table) }
        }
        body { font: 2px/1 weasyprint }
        table {
          border: 1px solid black;
          border-collapse: collapse;
          color: red;
          position: running(table);
        }
        td { border: 1px solid black }
        div { page-break-after: always }
      </style>
      <table></table>
      <div>1</div>
      <div>2</div>
    `)
}

// TestDrawRunningElementsTableBorderCollapseBorderStyle is test_running_elements_table_border_collapse_border_style in tests/draw/test_table.py.
func TestDrawRunningElementsTableBorderCollapseBorderStyle(t *testing.T) {
	t.Skip("upstream marks it as an expected failure")
	assertPixels(t, "running_elements_table_border_collapse_border_style", `
      KK_____________
      KK_____________
      _______________
      _______________
      _______________
      KKKZ___________
      KRR_RR_________
      KRR_RR_________
      KKKK__Z________
      KRRKRRK________
      KRRKRRK________
      KKKKKKK________
      _______________
      _______________
      _______________
    
      KK_____________
      KK_____________
      _______________
      _______________
      _______________
      KKKZ___________
      KRR_RR_________
      KRR_RR_________
      KKKK__Z________
      KRRKRRK________
      KRRKRRK________
      KKKKKKK________
      _______________
      _______________
      _______________
    `, `
      <style>
        @page {
          margin: 0 0 10px 0;
          size: 15px;
          @bottom-left { content: element(table) }
        }
        body { font: 2px/1 weasyprint }
        table {
          border: 1px solid black;
          border-collapse: collapse;
          color: red;
          position: running(table);
        }
        td { border: 1px solid black }
        div { page-break-after: always }
      </style>
      <table>
        <tr> <td>A</td> <td style="border-style: hidden">B</td> </tr>
        <tr> <td>C</td> <td style="border-style: none">D</td> </tr>
      </table>
      <div>1</div>
      <div>2</div>
    `)
}

// TestDrawRunningElementsTableBorderCollapseSpan is test_running_elements_table_border_collapse_span in tests/draw/test_table.py.
func TestDrawRunningElementsTableBorderCollapseSpan(t *testing.T) {
	assertPixels(t, "running_elements_table_border_collapse_span", `
      KK_____________
      KK_____________
      _______________
      _______________
      _______________
      KKKKKKKKKK_____
      KRRKRRKRRK_____
      KRRKRRKRRK_____
      K__KKKKKKK_____
      K__KRR___K_____
      K__KRR___K_____
      KKKKKKKKKK_____
      _______________
      _______________
      _______________
    
      KK_____________
      KK_____________
      _______________
      _______________
      _______________
      KKKKKKKKKK_____
      KRRKRRKRRK_____
      KRRKRRKRRK_____
      K__KKKKKKK_____
      K__KRR___K_____
      K__KRR___K_____
      KKKKKKKKKK_____
      _______________
      _______________
      _______________
    `, `
      <style>
        @page {
          margin: 0 0 10px 0;
          size: 15px;
          @bottom-left { content: element(table) }
        }
        body { font: 2px/1 weasyprint }
        table {
          border: 1px solid black;
          border-collapse: collapse;
          color: red;
          position: running(table);
        }
        td { border: 1px solid black }
        div { page-break-after: always }
      </style>
      <table>
        <tr> <td rowspan=2>A</td> <td>B</td> <td>C</td> </tr>
        <tr> <td colspan=2>D</td> </tr>
      </table>
      <div>1</div>
      <div>2</div>
    `)
}

// TestDrawRunningElementsTableBorderCollapseMargin is test_running_elements_table_border_collapse_margin in tests/draw/test_table.py.
func TestDrawRunningElementsTableBorderCollapseMargin(t *testing.T) {
	assertPixels(t, "running_elements_table_border_collapse_margin", `
      KK_____________
      KK_____________
      _______________
      _______________
      _______________
      _______________
      ____KKKKKKK____
      ____KRRKRRK____
      ____KRRKRRK____
      ____KKKKKKK____
      ____KRRKRRK____
      ____KRRKRRK____
      ____KKKKKKK____
      _______________
      _______________
    
      KK_____________
      KK_____________
      _______________
      _______________
      _______________
      _______________
      ____KKKKKKK____
      ____KRRKRRK____
      ____KRRKRRK____
      ____KKKKKKK____
      ____KRRKRRK____
      ____KRRKRRK____
      ____KKKKKKK____
      _______________
      _______________
    `, `
      <style>
        @page {
          margin: 0 0 10px 0;
          size: 15px;
          @bottom-center { content: element(table); width: 100% }
        }
        body { font: 2px/1 weasyprint }
        table {
          border: 1px solid black;
          border-collapse: collapse;
          color: red;
          margin: 1px auto;
          position: running(table);
        }
        td { border: 1px solid black }
        div { page-break-after: always }
      </style>
      <table>
        <tr> <td>A</td> <td>B</td> </tr>
        <tr> <td>C</td> <td>D</td> </tr>
      </table>
      <div>1</div>
      <div>2</div>
    `)
}

// TestDrawTablesSplitRow is test_tables_split_row in tests/draw/test_table.py.
func TestDrawTablesSplitRow(t *testing.T) {
	assertPixels(t, "tables_split_row", `
      KKKKKKKK
      KRRKKRRK
      KRRKKRRK
      K__KK__K

      K__KKRRK
      K__KKRRK
      KKKKKKKK
      ________
    `, `
      <style>
        @page { margin: 0; size: 8px 4px }
        td { border: 1px solid black; color: red; vertical-align: top;
             font-family: weasyprint; font-size: 2px; line-height: 1 }
      </style>
      <table>
        <tr>
          <td>a</td>
          <td>a<br>a</td>
        </tr>
      </table>`)
}

// TestDrawTablesColumnBackground is test_tables_column_background in tests/draw/test_table.py.
func TestDrawTablesColumnBackground(t *testing.T) {
	// Regression test for #2296.
	assertPixels(t, "tables_column_background", `
      KKKKKKKK
      KRRBBRRK
      KRRBBRRK
      KKKKKKKK

      KKKKKKKK
      KRRBBRRK
      KRRBBRRK
      KKKKKKKK
    `, `
      <style>
        @page { margin: 0; size: 8px 4px }
        col { background: blue }
        td { border: 1px solid black; color: red; vertical-align: top;
             font-family: weasyprint; font-size: 2px; line-height: 1 }
      </style>
      <table>
        <colgroup>
          <col>
        </colgroup>
        <tr>
          <td>a a</td>
        </tr>
        <tr>
          <td>a a</td>
        </tr>
      </table>`)
}

// TestDrawTablesBorderTfoot is test_tables_border_tfoot in tests/draw/test_table.py.
func TestDrawTablesBorderTfoot(t *testing.T) {
	// Regression test for #2873.
	assertPixels(t, "tables_border_tfoot", `
      KK__________
      RR__________
      RR__________
      KK__________
      RR__________
      RR__________
      KK__________
      RR__________
      RR__________
      BB__________
      BB__________
      ____________

      RR__________
      RR__________
      BB__________
      BB__________
      ____________
      ____________
      ____________
      ____________
      ____________
      ____________
      ____________
      ____________
    `, `
      <style>
        @page { size: 12px }
        body { font: 2px/1 weasyprint }
        table { border-collapse: collapse; color: red }
        td { border-top: 1px solid black }
        tfoot { color: blue }
      </style>
      <table>
        <tr> <td>A</td> </tr>
        <tr> <td>B</td> </tr>
        <tr> <td>C</td> </tr>
        <tr> <td style="border: none">D</td> </tr>
        <tfoot> <tr> <td style="border: none">E</td> </tr>
      </table>
    `)
}
