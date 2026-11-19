package fontdb

import (
	"testing"

	"github.com/xo/okage/svg"
)

func TestSource(t *testing.T) {
	db := Default()
	id, ok := db.Query(svg.FontQuery{Families: []svg.FontFamily{{Name: "Go"}}, Weight: 400})
	if !ok {
		t.Fatal("expected the face Go")
	}
	src, ok := Source(db, id, []svg.FontVariation{svg.NewFontVariation([4]byte{'w', 'g', 'h', 't'}, 700)})
	if !ok || len(src.Data) == 0 || len(src.Variations) != 1 || src.Variations[0].Value != 700 {
		t.Fatalf("expected the bytes and the variations of the face, got %d bytes, %v, %t", len(src.Data), src.Variations, ok)
	}
	face := NewFace(db, id, nil)
	if face == nil {
		t.Fatal("expected a face")
	}
	got, ok := face.Source()
	if !ok || got.Key() != src.Key() {
		t.Error("expected the face to share the bytes of the database")
	}
	if _, ok := Source(db, svg.FontID(db.Len()+1), nil); ok {
		t.Error("expected no source for a face that the database does not hold")
	}
}
