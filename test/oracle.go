package test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os/exec"
)

// run runs the oracle name with args, and decodes the PNG image that it writes
// to its standard output.
func run(ctx context.Context, name string, args ...string) (image.Image, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("oracle %s is not installed: %w", name, err)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %s: %w: %s", name, err, stderr.String())
	}
	img, err := png.Decode(&stdout)
	if err != nil {
		return nil, fmt.Errorf("decoding the png of %s: %w", name, err)
	}
	return img, nil
}

// meanDiff returns the mean absolute difference of the channels of a and b,
// each over a white background, from 0 to 255. It returns -1 if the sizes
// differ.
func meanDiff(a, b image.Image) float64 {
	if a.Bounds().Size() != b.Bounds().Size() {
		return -1
	}
	var sum, n float64
	ab, bb := a.Bounds(), b.Bounds()
	for y := range ab.Dy() {
		for x := range ab.Dx() {
			ca := overWhite(a.At(ab.Min.X+x, ab.Min.Y+y))
			cb := overWhite(b.At(bb.Min.X+x, bb.Min.Y+y))
			for i := range ca {
				d := ca[i] - cb[i]
				if d < 0 {
					d = -d
				}
				sum += d
				n++
			}
		}
	}
	return sum / n
}

// overWhite returns c over white, in 8 bits for each channel.
func overWhite(c color.Color) [3]float64 {
	r, g, b, a := c.RGBA()
	white := float64(0xffff - a)
	return [3]float64{(float64(r) + white) / 257, (float64(g) + white) / 257, (float64(b) + white) / 257}
}
