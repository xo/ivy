package zipdoc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

func open(t *testing.T, lim ivy.Limits, entries ...fixture.Entry) *Archive {
	t.Helper()
	a, err := Open(bytes.NewReader(fixture.Zip(entries...)), lim)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestFindAndPreviews(t *testing.T) {
	a := open(t, ivy.Limits{},
		fixture.Entry{Name: "Thumbnails/thumbnail.png", Data: fixture.PNG(16, 8, color.White)},
		fixture.Entry{Name: "docProps/thumbnail.emf", Data: []byte("emf")},
		fixture.Entry{Name: "preview.jpg", Data: fixture.JPEG(32, 32, color.Black)},
	)
	if a.Find("thumbnails/THUMBNAIL.png") == nil {
		t.Error("expected a match without regard to case")
	}
	if a.Find("missing") != nil {
		t.Error("expected no match")
	}
	var sizes []image.Point
	for p, err := range a.Previews(t.Context(),
		Candidate{ivy.Thumbnail, "docProps/thumbnail.emf"},
		Candidate{ivy.Thumbnail, "Thumbnails/thumbnail.png"},
		Candidate{ivy.Cover, "missing.png"},
		Candidate{ivy.Cover, "preview.jpg"},
	) {
		if err != nil {
			t.Fatal(err)
		}
		img, err := p.Decode(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Size() != p.Size {
			t.Errorf("expected %v, got: %v", p.Size, img.Bounds().Size())
		}
		sizes = append(sizes, p.Size)
	}
	if len(sizes) != 2 || sizes[0] != image.Pt(16, 8) || sizes[1] != image.Pt(32, 32) {
		t.Errorf("expected two previews, got: %v", sizes)
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(bytes.NewReader([]byte("not a zip")), ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	buf := fixture.Zip(fixture.Entry{Name: "a"}, fixture.Entry{Name: "b"})
	if _, err := Open(bytes.NewReader(buf), ivy.Limits{MaxObjects: 1}); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
}

func TestReadLimits(t *testing.T) {
	a := open(t, ivy.Limits{MaxMemory: 4}, fixture.Entry{Name: "big", Data: []byte("12345")})
	if _, err := a.Read(a.Find("big")); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
}

func TestDecodable(t *testing.T) {
	for name, want := range map[string]bool{
		"a.PNG": true, "b.jpeg": true, "c.jpg": true, "d.gif": true,
		"e.emf": false, "f.wmf": false, "g": false,
	} {
		if got := Decodable(name); got != want {
			t.Errorf("%s: expected %v, got: %v", name, want, got)
		}
	}
}

func TestParseXML(t *testing.T) {
	a := open(t, ivy.Limits{MaxObjects: 8},
		fixture.Entry{Name: "a.xml", Data: []byte("\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"windows-1252\"?>\n<a><b> one <i>two</i></b><c/></a>")},
		fixture.Entry{Name: "big.xml", Data: []byte("<a><b/><b/><b/><b/><b/><b/><b/><b/><b/></a>")},
		fixture.Entry{Name: "bad.xml", Data: []byte("<a><b></a>")},
	)
	doc, err := a.ParseXML(a.Find("a.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := ChildText(doc.RootElement(), "b"); got != "one two" {
		t.Errorf("expected %q, got: %q", "one two", got)
	}
	if got := ChildText(doc.RootElement(), "missing"); got != "" {
		t.Errorf("expected nothing, got: %q", got)
	}
	if _, err := a.ParseXML(a.Find("big.xml")); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got: %v", err)
	}
	if _, err := a.ParseXML(a.Find("bad.xml")); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
}
