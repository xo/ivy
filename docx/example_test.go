package docx_test

import (
	"context"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"

	"github.com/xo/ivy"
	"github.com/xo/ivy/docx"
)

// A program opens a Word document from a file, counts its pages, and draws
// the first page at 96 dots for each inch on a white background. An
// io.SectionReader of the file gives the size that ivy.Source needs.
func ExampleOpen() {
	ctx := context.Background()
	f, err := os.Open(filepath.Join("testdata", "sample.docx"))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		fmt.Println(err)
		return
	}
	d, err := docx.Open(ctx, io.NewSectionReader(f, 0, fi.Size()))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer d.Close()
	n, err := d.PageCount(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}
	page, err := d.Page(ctx, 0)
	if err != nil {
		fmt.Println(err)
		return
	}
	w, h := page.Size()
	img, err := ivy.RenderImage(ctx, page, 96, color.White)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%d pages, the first is %.0f by %.0f points, drawn as %v\n", n, w, h, img.Bounds().Size())
	// Output: 1 pages, the first is 612 by 792 points, drawn as (816,1056)
}
