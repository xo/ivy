package numfmt

import (
	"math"
	"testing"
)

func TestNumber(t *testing.T) {
	tests := []struct {
		code string
		v    float64
		want string
	}{
		{"General", 0, "0"},
		{"General", 1, "1"},
		{"@", 7, "7"},
		{"General", -1.5, "-1.5"},
		{"General", 1.0 / 3, "0.333333333"},
		{"General", 123456789012, "1.23457E+11"},
		{"General", 12345678901, "12345678901"},
		{"General", 0.000000000123, "1.23E-10"},
		{"General", 1e-5, "0.00001"},
		{"General", 0.1 + 0.2, "0.3"},
		{"0", 2.5, "3"},
		{"0", -2.5, "-3"},
		{"0", 0.4, "0"},
		{"0.00", 1.005, "1.01"},
		{"0.00", 3.14159, "3.14"},
		{"0.00", -3.14159, "-3.14"},
		{"#,##0", 1234567.891, "1,234,568"},
		{"#,##0.00", 1234.5, "1,234.50"},
		{"#,##0.00", 0.5, "0.50"},
		{"#.##", 0.5, ".5"},
		{"0.0#", 1.5, "1.5"},
		{"0.0#", 1.25, "1.25"},
		{"0.??", 1.5, "1.5 "},
		{"000", 7, "007"},
		{"000-00-0000", 123456789, "123-45-6789"},
		{"#,##0,", 1234567, "1,235"},
		{`0.0,,"M"`, 1234567, "1.2M"},
		{"0%", 0.256, "26%"},
		{"0.00%", 0.256, "25.60%"},
		{"0.00E+00", 12345, "1.23E+04"},
		{"0.00E+00", 0.00012345, "1.23E-04"},
		{"0.00E+00", 0, "0.00E+00"},
		{"##0.0E+0", 12345, "12.3E+3"},
		{"0.0E+0", 999999, "1.0E+6"},
		{"# ?/?", 1.5, "1 1/2"},
		{"# ?/?", 0.75, " 3/4"},
		{"# ??/??", 3.14159, "3 14/99"},
		{"# ?/?", 2, "2    "},
		{"?/8", 0.375, "3/8"},
		{"# ?/8", 1.25, "1 2/8"},
		{"?/?", 1.5, "3/2"},
		{`"$"#,##0.00_);[Red]\("$"#,##0.00\)`, 1234.5, "$1,234.50 "},
		{`"$"#,##0.00_);[Red]\("$"#,##0.00\)`, -1234.5, "($1,234.50)"},
		{"0;-0;\"zero\"", 0, "zero"},
		{"0;(0)", -5, "(5)"},
		{"0;;", -5, ""},
		{"[>=100]\"big\";[<100]\"small\"", 150, "big"},
		{"[>=100]\"big\";[<100]\"small\"", 50, "small"},
		{"[<0]\"neg \"0;0", -3, "neg 3"},
		{`0" kg"`, 12, "12 kg"},
		{`\$0`, 3, "$3"},
		{"[$€-407] #,##0.00", 1234.5, "€ 1,234.50"},
		{"[$-409]General", 42, "42"},
		{`_(* #,##0.00_);_(* \(#,##0.00\);_(* "-"??_);_(@_)`, 1234.5, " 1,234.50 "},
		{`_(* #,##0.00_);_(* \(#,##0.00\);_(* "-"??_);_(@_)`, 0, " -   "},
		{`_(* #,##0.00_);_(* \(#,##0.00\);_(* "-"??_);_(@_)`, -2, " (2.00)"},
		{"General\" units\"", 3, "3 units"},
		// dates and times, in the system of 1900
		{"m/d/yyyy", 1, "1/1/1900"},
		{"m/d/yyyy", 59, "2/28/1900"},
		{"m/d/yyyy", 60, "2/29/1900"},
		{"m/d/yyyy", 61, "3/1/1900"},
		{"yyyy-mm-dd", 45000, "2023-03-15"},
		{"d-mmm-yy", 45000, "15-Mar-23"},
		{"dddd, mmmm d, yyyy", 45000, "Wednesday, March 15, 2023"},
		{"ddd mmmmm", 45000, "Wed M"},
		{"yyyy-mm-dd", 0, "1900-01-00"},
		{"h:mm", 0.5, "12:00"},
		{"h:mm AM/PM", 0.75, "6:00 PM"},
		{"h:mm AM/PM", 0, "12:00 AM"},
		{"hh:mm:ss", 0.99998, "23:59:58"},
		{"h:mm:ss", 0.9999999, "0:00:00"},
		{"mm:ss.0", 1.5 / 86400, "00:01.5"},
		{"[h]:mm:ss", 1.5, "36:00:00"},
		{"[mm]:ss", 0.05, "72:00"},
		{"m/d/yyyy h:mm", 45000.25, "3/15/2023 6:00"},
		{"h:mm a/p", 0.25, "6:00 a"},
		{"yyyy", -1, "########"},
	}
	for _, tt := range tests {
		if got := Number(tt.code, tt.v, false).Text; got != tt.want {
			t.Errorf("Number(%q, %v): expected %q, got: %q", tt.code, tt.v, tt.want, got)
		}
	}
}

func TestDate1904(t *testing.T) {
	if got := Number("yyyy-mm-dd", 0, true).Text; got != "1904-01-01" {
		t.Errorf("expected 1904-01-01, got: %q", got)
	}
	if got := Number("yyyy-mm-dd", 43538, true).Text; got != "2023-03-15" {
		t.Errorf("expected 2023-03-15, got: %q", got)
	}
}

func TestColor(t *testing.T) {
	tests := []struct {
		code string
		v    float64
		want int
	}{
		{"0;[Red]-0", -1, 3},
		{"0;[Red]-0", 1, 0},
		{"[Blue]0", 1, 5},
		{"[Color 10]0", 1, 10},
		{"[color45]0", 1, 45},
		{"[Color 99]0", 1, 0},
	}
	for _, tt := range tests {
		if got := Number(tt.code, tt.v, false).Color; got != tt.want {
			t.Errorf("Number(%q, %v): expected color %d, got: %d", tt.code, tt.v, tt.want, got)
		}
	}
}

func TestText(t *testing.T) {
	tests := []struct {
		code, s, want string
	}{
		{"General", "abc", "abc"},
		{"@", "abc", "abc"},
		{`"<"@">"`, "abc", "<abc>"},
		{"0;-0;0;\"text: \"@", "abc", "text: abc"},
		{"0.00", "abc", "abc"},
	}
	for _, tt := range tests {
		if got := Text(tt.code, tt.s).Text; got != tt.want {
			t.Errorf("Text(%q, %q): expected %q, got: %q", tt.code, tt.s, tt.want, got)
		}
	}
}

func TestBuiltin(t *testing.T) {
	for id, want := range map[int]string{0: "General", 14: "m/d/yyyy", 49: "@"} {
		if got, ok := Builtin(id); !ok || got != want {
			t.Errorf("Builtin(%d): expected %q, got: %q %v", id, want, got, ok)
		}
	}
	if _, ok := Builtin(164); ok {
		t.Error("expected no built-in format 164")
	}
	for id := range 50 {
		code, ok := Builtin(id)
		if !ok {
			continue
		}
		// each built-in format formats a number without a panic
		_ = Number(code, 1234.5678, false)
		_ = Number(code, -1234.5678, false)
	}
	if !Parse("yyyy-mm-dd").IsDate() || Parse("0.00").IsDate() || Parse("[h]:mm").IsDate() == false {
		t.Error("expected IsDate for the dates only")
	}
}

func TestShape(t *testing.T) {
	for _, tt := range []struct {
		code string
		want Shape
	}{
		{"General", Shape{General: true}},
		{"@", Shape{General: true}},
		{"", Shape{General: true}},
		{"0", Shape{}},
		{"#,##0.00", Shape{Thousands: true, Decimals: 2}},
		{"0.0%", Shape{Percent: true, Decimals: 1}},
		{`"$"#,##0;("$"#,##0)`, Shape{Currency: true, Thousands: true}},
		{"0.00E+00", Shape{Scientific: true, Decimals: 2}},
		{"# ?/?", Shape{Fraction: true}},
		{"m/d/yyyy", Shape{Date: true}},
		{"[h]:mm", Shape{Date: true}},
	} {
		if got := Parse(tt.code).Shape(); got != tt.want {
			t.Errorf("Shape(%q): expected %+v, got: %+v", tt.code, tt.want, got)
		}
	}
}

func TestNotFinite(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := Number("0.00", v, false).Text; got != "#NUM!" {
			t.Errorf("expected #NUM! for %v, got: %q", v, got)
		}
	}
}

func TestAsText(t *testing.T) {
	for _, tt := range []struct {
		code string
		want bool
	}{
		{"@", true},
		{`"Code "@`, true},
		{"General", false},
		{"0.00", false},
		{"0;@", false},
	} {
		if got := Number(tt.code, 12209, false).AsText; got != tt.want {
			t.Errorf("Number(%q).AsText: expected %v, got: %v", tt.code, tt.want, got)
		}
	}
}

func FuzzNumber(f *testing.F) {
	for _, code := range builtin {
		f.Add(code, 1234.5)
	}
	f.Add(`[>=100][Red]"x"0.0E+00;[<0]# ??/??;[h]:mm:ss.000;@`, -0.25)
	f.Add("0000000000000000000000000000000000000000.000000000000000000000000000000000000", 1e300)
	f.Fuzz(func(_ *testing.T, code string, v float64) {
		if len(code) > 512 {
			return
		}
		_ = Number(code, v, false)
		_ = Number(code, v, true)
		_ = Text(code, "text")
	})
}
