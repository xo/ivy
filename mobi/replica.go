package mobi

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/pdf"
)

// replicaPDF opens the PDF files of a Print Replica book once.
type replicaPDF struct {
	mu   sync.Mutex
	docs []*pdf.Document
	// counts holds the number of pages of each PDF file.
	counts []int
	err    error
}

// open returns the PDF files of the book d, with the number of pages of
// each. It keeps the error, except the error of ctx, which a later call
// tries again.
func (r *replicaPDF) open(ctx context.Context, d *Document) (docs []*pdf.Document, counts []int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.docs != nil || r.err != nil {
		return r.docs, r.counts, r.err
	}
	defer func() {
		if err != nil && ctx.Err() == nil {
			r.err = err
		}
	}()
	files, err := d.replicaData()
	if err != nil {
		return nil, nil, err
	}
	for i, data := range files {
		doc, err := pdf.Open(ctx, bytes.NewReader(data), pdf.WithConfig(d.cfg))
		if err == nil {
			r.docs = append(r.docs, doc)
			var n int
			n, err = doc.PageCount(ctx)
			r.counts = append(r.counts, n)
		}
		if err != nil {
			err = errors.Join(fmt.Errorf("opening pdf file %d of a print replica book: %w", i+1, err), r.closeAll())
			return nil, nil, err
		}
	}
	return r.docs, r.counts, nil
}

// pageCount returns the number of pages of all the PDF files of the book d.
func (r *replicaPDF) pageCount(ctx context.Context, d *Document) (int, error) {
	_, counts, err := r.open(ctx, d)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	return total, nil
}

// page returns page n of the book d: a page of the PDF file that holds it.
// The pages of the PDF files follow one another in their order in the
// book.
func (r *replicaPDF) page(ctx context.Context, d *Document, n int) (ivy.Page, error) {
	docs, counts, err := r.open(ctx, d)
	if err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, fmt.Errorf("page %d: %w", n, ivy.ErrPageRange)
	}
	for i, doc := range docs {
		if n < counts[i] {
			return doc.Page(ctx, n)
		}
		n -= counts[i]
	}
	return nil, fmt.Errorf("page past the last page: %w", ivy.ErrPageRange)
}

// close closes the PDF files, if they are open.
func (r *replicaPDF) close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeAll()
}

// closeAll closes the PDF files. The caller holds r.mu.
func (r *replicaPDF) closeAll() error {
	errs := make([]error, 0, len(r.docs))
	for _, doc := range r.docs {
		errs = append(errs, doc.Close())
	}
	r.docs, r.counts = nil, nil
	return errors.Join(errs...)
}

// replicaData returns the PDF files of a Print Replica book, from its
// text.
func (d *Document) replicaData() (files [][]byte, err error) {
	defer ebook.Recover("reading print replica", &err)
	if d.book.encryption != 0 {
		return nil, fmt.Errorf("mobi text encrypted with type %d: %w", d.book.encryption, ivy.ErrDRM)
	}
	text, err := d.text(d.book)
	if err != nil {
		return nil, err
	}
	return replicaFiles(text, d.lim)
}

// replicaFiles returns the PDF files of the text of a Print Replica book.
//
// The text starts with "%MOP" and a table of sections. The 32-bit number at
// offset 4 is the number of tables. Then each table gives the number of its
// sections, in 32 bits. Then come the sections of all the tables, in order,
// each with its offset in the text and its length, in 32 bits each. The
// first section of a table is a PDF file, and the other sections hold
// resources of the reader. The book is the PDF files of the tables, in
// order. A table whose first section is not a PDF file is left out.
func replicaFiles(text []byte, lim ivy.Limits) ([][]byte, error) {
	bad := func(what string) error {
		return fmt.Errorf("print replica %s: %w", what, ivy.ErrFormat)
	}
	if len(text) < 8 || string(text[:4]) != replicaMagic {
		return nil, bad("without its header")
	}
	tables := uint64(binary.BigEndian.Uint32(text[4:]))
	if tables == 0 || tables > uint64(lim.MaxObjects) || 8+4*tables > uint64(len(text)) {
		return nil, bad(fmt.Sprintf("of %d tables", tables))
	}
	index := 8 + 4*tables
	var files [][]byte
	var errs []error
	for i := range tables {
		sections := uint64(binary.BigEndian.Uint32(text[8+4*i:]))
		if sections > uint64(lim.MaxObjects) {
			return nil, bad(fmt.Sprintf("table of %d sections", sections))
		}
		for j := range sections {
			if index+8 > uint64(len(text)) {
				return nil, errors.Join(append(errs, bad("table past the end of the text"))...)
			}
			offset := uint64(binary.BigEndian.Uint32(text[index:]))
			length := uint64(binary.BigEndian.Uint32(text[index+4:]))
			index += 8
			if j != 0 {
				continue
			}
			if offset+length > uint64(len(text)) {
				errs = append(errs, bad(fmt.Sprintf("section of %d bytes at %d past the end of the text", length, offset)))
				continue
			}
			data := text[offset : offset+length]
			if bytes.HasPrefix(data, []byte("%PDF")) {
				files = append(files, data)
			}
		}
	}
	if len(files) > 0 {
		return files, nil
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return nil, bad("without a pdf file")
}
