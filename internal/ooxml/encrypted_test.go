package ooxml

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
)

// password is the password of the test files of msoffcrypto-tool.
const password = "Password1234_"

func TestOpenEncrypted(t *testing.T) {
	for _, name := range []string{"example_password.docx", "ecma376standard_password.docx"} {
		t.Run(name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			src := bytes.NewReader(buf)
			d, err := Open(t.Context(), src, ivy.Config{Password: password})
			if err != nil {
				t.Fatal(err)
			}
			if d.archive.Find("word/document.xml") == nil {
				t.Error("expected word/document.xml in the package")
			}
			if _, err := Open(t.Context(), src, ivy.Config{Password: "wrong"}); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ErrPassword, got: %v", err)
			}
			attempts := 0
			cfg := ivy.Config{PasswordFunc: func(_ context.Context, attempt int) (string, error) {
				attempts = attempt
				if attempt < 2 {
					return "wrong", nil
				}
				return password, nil
			}}
			if _, err := Open(t.Context(), src, cfg); err != nil || attempts != 2 {
				t.Errorf("expected the password on attempt 2, got: %d, %v", attempts, err)
			}
		})
	}
}

func TestOpenDamagedCompoundFile(t *testing.T) {
	buf := append([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, make([]byte, 504)...)
	if _, err := Open(t.Context(), bytes.NewReader(buf), ivy.Config{}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
}

func TestRelationships(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "example_password.docx"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(t.Context(), bytes.NewReader(buf), ivy.Config{Password: password})
	if err != nil {
		t.Fatal(err)
	}
	main, err := d.MainPart(ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if main != "word/document.xml" {
		t.Errorf("expected word/document.xml, got: %q", main)
	}
	rels, err := d.Relationships(main, ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rels {
		if r.TypeIs("styles") {
			found = r.Target == "word/styles.xml"
		}
	}
	if !found {
		t.Errorf("expected the styles of word/styles.xml, got: %+v", rels)
	}
}

func TestResolveTarget(t *testing.T) {
	for _, test := range []struct{ source, target, exp string }{
		{"word/document.xml", "media/image1.png", "word/media/image1.png"},
		{"word/document.xml", "../customXml/item1.xml", "customXml/item1.xml"},
		{"ppt/slides/slide1.xml", "/ppt/media/a.emf", "ppt/media/a.emf"},
		{"", "word/document.xml", "word/document.xml"},
		{"xl/workbook.xml", "worksheets/sheet1.xml#A1", "xl/worksheets/sheet1.xml"},
		{"a.xml", "../../../x", "x"},
	} {
		if got := ResolveTarget(test.source, test.target); got != test.exp {
			t.Errorf("ResolveTarget(%q, %q): expected %q, got: %q", test.source, test.target, test.exp, got)
		}
	}
}
