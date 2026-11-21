// This file of ivy replaces pdfdriver.go of go-tex/engine (D62). Upstream
// writes PDF with go-pdfkit. ivy draws the pages on its display list in
// ivy_driver.go, and pdf/pdfenc writes them as PDF (D58).

package engine

import (
	"errors"
	"io"
)

// errNoPDFDriver is the error of RenderPDF.
var errNoPDFDriver = errors.New("texengine: the PDF driver of go-pdfkit is not in the copy of ivy")

// RenderPDF returns an error. The functions of api.go that write PDF call
// it, and ivy does not call them.
func (e *Engine) RenderPDF(io.Writer, float64) error {
	return errNoPDFDriver
}
