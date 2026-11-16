package jpeg2000

import (
	"fmt"

	"github.com/xo/ivy/internal/moxcms"
)

// This file ports no upstream file. It joins the hook of D16 to the port of
// moxcms, which wave 1 of the port of hayro wrote at the same time as this
// package.

// MoxcmsTransform is the ICCTransform that upstream makes with moxcms. It
// reads the profile, makes a transform of 8 bits to sRGB with the default
// options, and runs it, as hayro-jpeg2000 src/integration.rs does.
func MoxcmsTransform(profile, src []byte, srcLayout Layout, dst []byte, dstLayout Layout) error {
	from, err := moxcms.NewFromSlice(profile)
	if err != nil {
		return fmt.Errorf("reading icc profile: %w", err)
	}
	t, err := from.CreateTransform8Bit(moxcmsLayout(srcLayout), moxcms.NewSrgb(), moxcmsLayout(dstLayout), moxcms.DefaultTransformOptions())
	if err != nil {
		return fmt.Errorf("making icc transform: %w", err)
	}
	if err := t.Transform(src, dst); err != nil {
		return fmt.Errorf("running icc transform: %w", err)
	}
	return nil
}

// moxcmsLayout returns the layout of moxcms for l.
func moxcmsLayout(l Layout) moxcms.Layout {
	switch l {
	case LayoutGray:
		return moxcms.LayoutGray
	case LayoutGrayAlpha:
		return moxcms.LayoutGrayAlpha
	case LayoutRGB:
		return moxcms.LayoutRgb
	}
	return moxcms.LayoutRgba
}
