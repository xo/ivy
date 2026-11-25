package pdf_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xo/ivy"
	"github.com/xo/ivy/pdf"
)

// A program opens a PDF file that a password protects. Without the
// password, Open returns ivy.ErrPassword. A program can also give
// PasswordFunc, to ask a person for the password.
func ExampleWithConfig() {
	ctx := context.Background()
	buf, err := os.ReadFile(filepath.Join("testdata", "password_2.pdf"))
	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = pdf.Open(ctx, bytes.NewReader(buf))
	fmt.Println(errors.Is(err, ivy.ErrPassword))
	d, err := pdf.Open(ctx, bytes.NewReader(buf), pdf.WithConfig(ivy.Config{Password: "samplefiles"}))
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
	fmt.Println(n)
	// Output:
	// true
	// 3
}
