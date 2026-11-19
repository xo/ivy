package odf

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPageFloatFiles checks the position on the page of the frames of the
// files of D60, which LibreOffice wrote from the .docx files of the tests
// of docx, with style:vertical-rel page-content and wrap (D60).
func TestPageFloatFiles(t *testing.T) {
	for _, tt := range []struct {
		name, want string
	}{
		{"f-late", "-ivy-float-position: margin 72pt"},
		{"f-bottom", "-ivy-float-position: margin bottom"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", "float-"+tt.name+".odt"))
			if err != nil {
				t.Fatal(err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			src, err := convertText(t.Context(), d)
			if err != nil {
				t.Fatal(err)
			}
			b, err := fs.ReadFile(src.FS, src.Documents[0])
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(string(b), "-ivy-float-position"); got != 1 || !strings.Contains(string(b), tt.want) {
				t.Errorf("got %d positions, want %q", got, tt.want)
			}
		})
	}
}
