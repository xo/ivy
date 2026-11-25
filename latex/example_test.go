package latex_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xo/ivy"
	"github.com/xo/ivy/latex"
)

// A program sets a LaTeX document that cites a bibliography. WithDir gives
// the folder of the document, where the engine finds its figures and its
// report.bbl, and WithName gives the job name, so that the engine reads
// report.bbl and not texput.bbl. The field TeX of ivy.Config gives the
// classes, the packages and the fonts of TeX, such as fonts.TeX of the
// module github.com/xo/ivy/fonts.
func ExampleWithName() {
	ctx := context.Background()
	dir := filepath.Join("testdata", "report")
	buf, err := os.ReadFile(filepath.Join(dir, "report.tex"))
	if err != nil {
		fmt.Println(err)
		return
	}
	cfg := ivy.Config{TeX: os.DirFS(filepath.Join("..", "fonts"))}
	d, err := latex.Open(ctx, bytes.NewReader(buf),
		latex.WithConfig(cfg),
		latex.WithDir(os.DirFS(dir)),
		latex.WithName("report"),
	)
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
	fmt.Println(n > 0)
	// Output: true
}

// A program reads the title and the authors that \title and \author give,
// without drawing the pages.
func ExampleDocument_Describe() {
	ctx := context.Background()
	dir := filepath.Join("testdata", "report")
	buf, err := os.ReadFile(filepath.Join(dir, "report.tex"))
	if err != nil {
		fmt.Println(err)
		return
	}
	cfg := ivy.Config{TeX: os.DirFS(filepath.Join("..", "fonts"))}
	d, err := latex.Open(ctx, bytes.NewReader(buf), latex.WithConfig(cfg), latex.WithDir(os.DirFS(dir)), latex.WithName("report"))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer d.Close()
	info, err := d.Describe(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%q %q\n", info.Title, info.Authors)
	// Output: "A Report on the Rendering of Documents" ["The Test Team"]
}
