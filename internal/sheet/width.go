package sheet

import "github.com/xo/ivy/internal/fontdb"

// calibriWidths are the advances of the characters of Calibri from U+0020
// to U+017F, in units of 1/2048 of the size of the font: ASCII, Latin-1
// and Latin Extended-A. Carlito, which has the metrics of Calibri, gives
// them. The controls from U+007F to U+009F and the soft hyphen are 0.
var calibriWidths = [0x180 - 0x20]uint16{
	463, 667, 821, 1020, 1038, 1464, 1397, 452, 621, 621, 1020, 1020, 511, 627, 517, 791,
	1038, 1038, 1038, 1038, 1038, 1038, 1038, 1038, 1038, 1038, 548, 548, 1020, 1020, 1020, 949,
	1831, 1185, 1114, 1092, 1260, 1000, 941, 1292, 1276, 516, 653, 1064, 861, 1751, 1322, 1356,
	1058, 1378, 1112, 941, 998, 1314, 1162, 1822, 1063, 998, 959, 628, 791, 628, 1020, 1020,
	596, 981, 1076, 866, 1076, 1019, 625, 964, 1076, 470, 490, 931, 470, 1636, 1076, 1080,
	1076, 1076, 714, 801, 686, 1076, 925, 1464, 887, 927, 809, 644, 943, 644, 1020, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	463, 667, 1020, 1038, 1020, 1038, 1020, 1020, 804, 1709, 824, 1049, 1020, 0, 1038, 807,
	694, 1020, 688, 685, 598, 1126, 1200, 517, 629, 504, 865, 1049, 1303, 1375, 1383, 949,
	1185, 1185, 1185, 1185, 1185, 1185, 1563, 1092, 1000, 1000, 1000, 1000, 516, 516, 516, 516,
	1279, 1322, 1356, 1356, 1356, 1356, 1356, 1020, 1359, 1314, 1314, 1314, 1314, 998, 1058, 1080,
	981, 981, 981, 981, 981, 981, 1583, 866, 1019, 1019, 1019, 1019, 470, 470, 470, 470,
	1076, 1076, 1080, 1080, 1080, 1080, 1080, 1020, 1084, 1076, 1076, 1076, 1076, 927, 1076, 927,
	1185, 981, 1185, 981, 1185, 981, 1092, 866, 1092, 866, 1092, 866, 1092, 866, 1260, 1164,
	1279, 1130, 1000, 1019, 1000, 1019, 1000, 1019, 1000, 1019, 1000, 1019, 1292, 964, 1292, 964,
	1292, 964, 1292, 964, 1276, 1076, 1344, 1091, 516, 470, 516, 470, 516, 470, 516, 470,
	516, 470, 1170, 960, 653, 490, 1064, 931, 931, 861, 470, 861, 470, 866, 540, 1118,
	765, 880, 507, 1322, 1076, 1322, 1076, 1322, 1076, 1186, 1287, 1076, 1356, 1080, 1356, 1080,
	1356, 1080, 1775, 1740, 1112, 714, 1112, 714, 1112, 714, 941, 801, 941, 801, 941, 801,
	941, 801, 998, 686, 998, 708, 998, 700, 1314, 1076, 1314, 1076, 1314, 1076, 1314, 1076,
	1314, 1076, 1314, 1076, 1822, 1464, 998, 927, 998, 959, 809, 959, 809, 959, 809, 497,
}

// TextWidth returns the width in points of the text s in Calibri of the
// size size, the default font of Excel, without kerning. A character past
// the table of Calibri takes 0.5 of the size, a combining mark 0, and a
// wide character of East Asia the whole size. A format that does not lay
// out its text, such as csv for the widths of its columns, measures text
// with it.
func TextWidth(s string, size float64) float64 {
	units := 0
	for _, r := range s {
		switch {
		case r >= 0x20 && r < 0x180:
			units += int(calibriWidths[r-0x20])
		case r >= 0x300 && r <= 0x36f, r == 0x200b, r == 0xfeff:
			// combining marks and characters without a width
		case fontdb.IsCJK(r) && (r < 0xff61 || r > 0xffdc):
			// the half-width forms are half as wide
			units += 2048
		case r >= 0x180:
			units += 1024
		}
	}
	return float64(units) * size / 2048
}
