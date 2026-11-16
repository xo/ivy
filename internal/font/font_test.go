package font

import (
	"errors"
	"sync"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
)

func TestLoad(t *testing.T) {
	f, err := Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	gid, ok := f.Glyph('H')
	if !ok || gid == 0 {
		t.Fatalf("expected a glyph for H, got: %d %v", gid, ok)
	}
	if adv := f.Advance(gid); adv < 0.5 || adv > 1 {
		t.Errorf("expected an advance of about 0.7 em, got: %v", adv)
	}
	p := f.Outline(gid)
	if p == nil {
		t.Fatal("expected an outline")
	}
	b := p.Bounds()
	// H stands on the baseline, so its top is above it, at a negative y
	if b.Top() > -0.5 || b.Bottom() > 0.01 || b.Left() < 0 {
		t.Errorf("expected H above the baseline, got: %v", b)
	}
	if f.Outline(gid) != p {
		t.Error("expected the kept outline")
	}
	space, _ := f.Glyph(' ')
	if f.Outline(space) != nil {
		t.Error("expected no outline for a space")
	}
}

func TestConcurrent(t *testing.T) {
	f, err := Load(goregular.TTF, 0)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for r := 'a'; r <= 'z'; r++ {
		wg.Go(func() {
			gid, _ := f.Glyph(r)
			_ = f.Outline(gid)
			_ = f.Advance(gid)
		})
	}
	wg.Wait()
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load([]byte("not a font"), 0); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	if _, err := Load(goregular.TTF, 1); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
}
