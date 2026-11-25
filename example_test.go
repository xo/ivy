package ivy_test

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/docx"
	"github.com/xo/ivy/epub"
	"github.com/xo/ivy/pdf"
	"github.com/xo/ivy/pdf/pdfenc"
	"github.com/xo/ivy/xlsx"
)

// A program opens a Word document, draws its first page as a PNG image at
// 96 dots for each inch, and writes the whole document as a PDF file. A
// program writes to files in the same way. This example writes to memory,
// and decodes the PNG image and the PDF file again to show what they hold.
func Example() {
	ctx := context.Background()
	f, err := os.Open(filepath.Join("docx", "testdata", "sample.docx"))
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
	// An io.SectionReader of the file is an ivy.Source: it reads at any
	// offset, and it knows its size.
	doc, err := docx.Open(ctx, io.NewSectionReader(f, 0, fi.Size()))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer doc.Close()

	// Draw the first page as a PNG image.
	page, err := doc.Page(ctx, 0)
	if err != nil {
		fmt.Println(err)
		return
	}
	img, err := ivy.RenderImage(ctx, page, 96, color.White)
	if err != nil {
		fmt.Println(err)
		return
	}
	var pngFile bytes.Buffer
	if err := png.Encode(&pngFile, img); err != nil {
		fmt.Println(err)
		return
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(pngFile.Bytes()))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("PNG: %d by %d pixels\n", cfg.Width, cfg.Height)

	// Write every page as a PDF file, with its text, links and outline.
	var pdfFile bytes.Buffer
	if err := pdfenc.Encode(ctx, &pdfFile, doc); err != nil {
		fmt.Println(err)
		return
	}
	written, err := pdf.Open(ctx, bytes.NewReader(pdfFile.Bytes()))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer written.Close()
	n, err := written.PageCount(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}
	w, h := page.Size()
	fmt.Printf("PDF: %d page of %.0f by %.0f points\n", n, w, h)
	// Output:
	// PNG: 816 by 1056 pixels
	// PDF: 1 page of 612 by 792 points
}

// A program builds its own registry of formats from the Format values of
// the packages that it imports. ivy holds no registry, so a program links
// only these formats. A program can also find the type of a file with
// github.com/xo/magic and look up its mime type in Mimes.
func Example_registry() {
	formats := []ivy.Format{docx.Format, epub.Format, pdf.Format, xlsx.Format}
	byExt := make(map[string]ivy.Format)
	for _, f := range formats {
		for _, ext := range f.Exts {
			byExt[ext] = f
		}
	}
	for _, name := range []string{"report.docx", "book.epub", "paper.pdf", "sheet.xlsx", "photo.heic"} {
		ext := strings.TrimPrefix(filepath.Ext(name), ".")
		f, ok := byExt[ext]
		if !ok {
			fmt.Println(name, "no format")
			continue
		}
		fmt.Println(name, f.Name)
	}
	// Output:
	// report.docx docx
	// book.epub epub
	// paper.pdf pdf
	// sheet.xlsx xlsx
	// photo.heic no format
}

// A program opens a document through a Format of its registry, and asks
// the document for the capabilities that its format has: Pager for the
// pages, Describer for the metadata and Previewer for the previews.
func Example_capabilities() {
	ctx := context.Background()
	buf, err := os.ReadFile(filepath.Join("docx", "testdata", "sample.docx"))
	if err != nil {
		fmt.Println(err)
		return
	}
	d, err := docx.Format.Open(ctx, bytes.NewReader(buf), ivy.Config{})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer d.Close()
	if pager, ok := d.(ivy.Pager); ok {
		n, err := pager.PageCount(ctx)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("pages:", n)
	}
	_, describes := d.(ivy.Describer)
	_, previews := d.(ivy.Previewer)
	fmt.Println("metadata:", describes, "previews:", previews)
	// Output:
	// pages: 1
	// metadata: true previews: true
}
