package odf

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// odtMeta is a meta.xml with each field that Describe reads.
const odtMeta = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-meta xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/"><office:meta>
<meta:generator>LibreOffice/26.8</meta:generator><dc:title> The title </dc:title><dc:subject>The subject</dc:subject>
<dc:language>en-US</dc:language><meta:initial-creator>First Author</meta:initial-creator><dc:creator>Last Editor</dc:creator>
<meta:creation-date>2007-11-15T14:00:07.25</meta:creation-date><dc:date>2008-11-10T12:09:27Z</dc:date>
</office:meta></office:document-meta>`

func TestDescribe(t *testing.T) {
	describe := func(buf []byte, opts ...Option) ivy.Info {
		t.Helper()
		d, err := Open(t.Context(), bytes.NewReader(buf), opts...)
		if err != nil {
			t.Fatal(err)
		}
		info, err := d.Describe(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	info := describe(odtFile(`<text:p>x</text:p>`, fixture.Entry{Name: "meta.xml", Data: []byte(odtMeta)}))
	exp := ivy.Info{
		Title:    "The title",
		Authors:  []string{"First Author"},
		Subject:  "The subject",
		Language: "en-US",
		Created:  time.Date(2007, 11, 15, 14, 0, 7, 250000000, time.UTC),
		Modified: time.Date(2008, 11, 10, 12, 9, 27, 0, time.UTC),
		Producer: "LibreOffice/26.8",
	}
	if info.Title != exp.Title || !slices.Equal(info.Authors, exp.Authors) || info.Subject != exp.Subject ||
		info.Language != exp.Language || !info.Created.Equal(exp.Created) || !info.Modified.Equal(exp.Modified) || info.Producer != exp.Producer {
		t.Errorf("expected %+v, got: %+v", exp, info)
	}
	// without meta:initial-creator, the author is dc:creator
	meta := bytes.Replace([]byte(odtMeta), []byte("<meta:initial-creator>First Author</meta:initial-creator>"), nil, 1)
	if info := describe(odtFile(`<text:p>x</text:p>`, fixture.Entry{Name: "meta.xml", Data: meta})); !slices.Equal(info.Authors, []string{"Last Editor"}) {
		t.Errorf("expected the author Last Editor, got: %v", info.Authors)
	}
	// a file without meta.xml, or with a damaged one, says nothing
	if info := describe(odtFile(`<text:p>x</text:p>`)); info.Title != "" || info.Authors != nil || !info.Created.IsZero() {
		t.Errorf("expected nothing, got: %+v", info)
	}
	if info := describe(odtFile(`<text:p>x</text:p>`, fixture.Entry{Name: "meta.xml", Data: []byte("<office:document-meta")})); info.Title != "" {
		t.Errorf("expected nothing, got: %+v", info)
	}
	// the metadata of an encrypted file is decrypted
	buf, err := os.ReadFile(filepath.Join("testdata", "encrypted_aes.odt"))
	if err != nil {
		t.Fatal(err)
	}
	if info := describe(buf, WithConfig(ivy.Config{Password: "secret"})); info.Producer == "" {
		t.Errorf("expected the program of the encrypted file, got: %+v", info)
	}
}
