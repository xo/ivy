package ooxml

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/magic/container"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// TestDetect finds the type of the encrypted files of testdata as a program
// does, with magic and its package container (magic D65). Each is an
// encrypted Office Open XML file, and only the key tells that it holds a
// Word document.
func TestDetect(t *testing.T) {
	for _, name := range []string{"example_password.docx", "ecma376standard_password.docx"} {
		t.Run(name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			mime, f := fixture.DetectMIME(t, buf)
			if f != container.EncryptedOOXML || mime != "application/encrypted" {
				t.Errorf("expected %v and application/encrypted, got: %v and %q", container.EncryptedOOXML, f, mime)
			}
			if _, err := Open(t.Context(), bytes.NewReader(buf), ivy.Config{Password: password}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
