package charset

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		label string
		in    []byte
		exp   string
	}{
		{"windows-1251", []byte{0xcf, 0xf0, 0xe8, 0xe2, 0xe5, 0xf2}, "Привет"},
		{"cp1251", []byte{0xcf, 0xf0, 0xe8}, "При"},
		{"KOI8-R", []byte{0xf0, 0xd2, 0xc9, 0xd7, 0xc5, 0xd4}, "Привет"},
		{"windows-1250", []byte{0x8a, 0x9a}, "Šš"},
		{"shift_jis", []byte{0x82, 0xa0}, "あ"},
		{"gbk", []byte{0xc4, 0xe3}, "你"},
		{"big5", []byte{0xa4, 0x40}, "一"},
		{"euc-kr", []byte{0xb0, 0xa1}, "가"},
		{"iso-8859-1", []byte{0x80, 0xe9}, "€é"},
	}
	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			s, ok := Decode(test.in, test.label)
			if !ok {
				t.Fatalf("expected the label %q to be known", test.label)
			}
			if s != test.exp {
				t.Errorf("expected %q, got: %q", test.exp, s)
			}
		})
	}
}

func TestDecodeUnknown(t *testing.T) {
	if s, ok := Decode([]byte("abc"), "x-no-such"); ok || s != "abc" {
		t.Errorf("expected the text as it is and false, got: %q, %t", s, ok)
	}
	if Known("x-no-such") {
		t.Error("expected an unknown label")
	}
}

func TestReader(t *testing.T) {
	r, err := Reader("koi8-r", strings.NewReader("\xf0\xd2\xc9"))
	if err != nil {
		t.Fatal(err)
	}
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "При" {
		t.Errorf("expected %q, got: %q", "При", buf)
	}
	if _, err := Reader("x-no-such", strings.NewReader("")); !errors.Is(err, ErrUnknown) {
		t.Errorf("expected ErrUnknown, got: %v", err)
	}
}
