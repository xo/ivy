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

// replicaPDF opens the PDF file of a Print Replica book once.
type replicaPDF struct {
	mu  sync.Mutex
	doc *pdf.Document
	err error
}

// open returns the PDF file of the book d. It keeps the error, except the
// error of ctx, which a later call tries again.
func (r *replicaPDF) open(ctx context.Context, d *Document) (doc *pdf.Document, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.doc != nil || r.err != nil {
		return r.doc, r.err
	}
	defer func() {
		if err != nil && ctx.Err() == nil {
			r.err = err
		}
	}()
	data, err := d.replicaData()
	if err != nil {
		return nil, err
	}
	r.doc, err = pdf.Open(ctx, bytes.NewReader(data), pdf.WithConfig(d.cfg))
	if err != nil {
		return nil, fmt.Errorf("opening the pdf of a print replica book: %w", err)
	}
	return r.doc, nil
}

// close closes the PDF file, if it is open.
func (r *replicaPDF) close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.doc == nil {
		return nil
	}
	err := r.doc.Close()
	r.doc = nil
	return err
}

// replicaData returns the PDF file of a Print Replica book, from its text.
func (d *Document) replicaData() (data []byte, err error) {
	defer ebook.Recover("reading print replica", &err)
	if d.book.encryption != 0 {
		return nil, fmt.Errorf("mobi text encrypted with type %d: %w", d.book.encryption, ivy.ErrDRM)
	}
	text, err := d.text(d.book)
	if err != nil {
		return nil, err
	}
	return replicaFile(text, d.lim)
}

// replicaFile returns the PDF file of the text of a Print Replica book.
//
// The text starts with "%MOP" and a table of sections. The 32-bit number at
// offset 4 is the number of tables. Then each table gives the number of its
// sections, in 32 bits. Then come the sections of all the tables, in order,
// each with its offset in the text and its length, in 32 bits each. The
// first section of a table is a PDF file. The book is the first PDF file;
// the other sections hold resources of the reader.
func replicaFile(text []byte, lim ivy.Limits) ([]byte, error) {
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
				return data, nil
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return nil, bad("without a pdf file")
}
