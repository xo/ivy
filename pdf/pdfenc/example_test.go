package pdfenc_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xo/ivy/docx"
	"github.com/xo/ivy/pdf"
	"github.com/xo/ivy/pdf/pdfenc"
)

// A program writes the pages of a document of any format as a PDF file.
// The text of the pages stays text, and the title and the authors of the
// document become the information of the file.
func ExampleEncode() {
	ctx := context.Background()
	buf, err := os.ReadFile(filepath.Join("..", "..", "docx", "testdata", "sample.docx"))
	if err != nil {
		fmt.Println(err)
		return
	}
	d, err := docx.Open(ctx, bytes.NewReader(buf))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer d.Close()
	var out bytes.Buffer
	if err := pdfenc.Encode(ctx, &out, d); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(bytes.HasPrefix(out.Bytes(), []byte("%PDF-")))
	// Output: true
}

// A program writes only some pages, without the outline, and reads the
// written file back with the package pdf.
func ExampleWithPages() {
	ctx := context.Background()
	buf, err := os.ReadFile(filepath.Join("..", "..", "docx", "testdata", "sample.docx"))
	if err != nil {
		fmt.Println(err)
		return
	}
	d, err := docx.Open(ctx, bytes.NewReader(buf))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer d.Close()
	var out bytes.Buffer
	if err := pdfenc.Encode(ctx, &out, d, pdfenc.WithPages(0), pdfenc.WithOutline(false)); err != nil {
		fmt.Println(err)
		return
	}
	written, err := pdf.Open(ctx, bytes.NewReader(out.Bytes()))
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
	fmt.Println(n)
	// Output: 1
}
