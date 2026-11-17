package djvu

import (
	"bytes"
	"encoding/hex"
	"errors"
	"image/color"
	"testing"

	"github.com/xo/ivy"
)

// redJ2K is a raw JPEG 2000 codestream of 24 by 16 red pixels, which
// ImageMagick 7 with OpenJPEG 2.5.4 wrote:
// magick -size 24x16 xc:'#ff0000' -define jp2:rate=1 r.j2k
const redJ2K = "ff4fff51002f0000000000180000001000000000000000000000001800000010000000000000000000030f01010f01010f0101" +
	"ff52000c00000001010404040001ff5c00104080888890888890888890888890ff640025000143726561746564206279204f70" +
	"656e4a5045472076657273696f6e20322e352e34ff90000a0000000000280001ff93cffc300c093f4f80dff8901001b380808" +
	"0808080808080808080ffd9"

func TestJPEG2000Background(t *testing.T) {
	t.Parallel()
	j2k, err := hex.DecodeString(redJ2K)
	if err != nil {
		t.Fatal(err)
	}
	// a page of 24 by 16 pixels at 300 dots per inch
	info := []byte{0, 24, 0, 16, 24, 0, 0x2c, 0x01, 22, 1}
	d, err := Open(t.Context(), bytes.NewReader(form("DJVU", chunk("INFO", info), chunk("BG2k", j2k))))
	if err != nil {
		t.Fatal(err)
	}
	img := render(t, d, 0, 300)
	if img.Bounds().Dx() != 24 || img.Bounds().Dy() != 16 {
		t.Fatalf("expected 24x16, got: %v", img.Bounds())
	}
	for _, p := range [][2]int{{2, 2}, {12, 8}, {21, 13}} {
		if c := img.At(p[0], p[1]); !near(c, [3]int{255, 0, 0}, 16) {
			t.Errorf("at %v: expected red, got: %v", p, c)
		}
	}
	// a damaged layer leaves the page white, in part
	d, err = Open(t.Context(), bytes.NewReader(form("DJVU", chunk("INFO", info), chunk("BG2k", j2k[:20]))))
	if err != nil {
		t.Fatal(err)
	}
	page, _ := d.Page(t.Context(), 0)
	if _, err := ivy.RenderImage(t.Context(), page, 300, color.White); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
}
