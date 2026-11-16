package chart

import (
	"context"
	"errors"
	"testing"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

func FuzzParse(f *testing.F) {
	for _, test := range testCharts() {
		f.Add(test.xml)
	}
	f.Add([]byte(odfChart))
	f.Fuzz(func(t *testing.T, data []byte) {
		opts := Options{Config: ivy.Config{Limits: ivy.Limits{MaxObjects: 1 << 10}}}
		c, err := Parse(data, opts)
		if err != nil {
			if c, err = ParseODF(data, opts); err != nil {
				return
			}
		}
		var l display.List
		r, _ := okage.RectFromXYWH(0, 0, 160, 120)
		err = c.Draw(context.Background(), &l, r)
		if pe, ok := errors.AsType[*ivy.PartialError](err); ok && !errors.Is(pe, ivy.ErrLimit) {
			t.Fatalf("expected no panic, got: %v", err)
		}
	})
}
