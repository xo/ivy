package review

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// faultColumns are the columns of the file of the faults of the references,
// in order.
var faultColumns = []string{"name", "page", "reference", "note"}

// Fault is a known fault of a reference on a page of a document: the
// reference draws the page wrong, so the review does not compare ivy with
// it. The report marks the cell, shows the note, and leaves the cell out of
// the worst numbers and of "references disagree".
type Fault struct {
	// Name is the name of the document in the manifest, or that name with
	// the paper, such as "doc.txt [A4 landscape]".
	Name string `json:"name"`
	// Page is the page, from 1, or 0 for each page.
	Page int `json:"page"`
	// Ref is the id of the reference, such as "soffice" or "msoffice", or
	// "*" for each reference.
	Ref  string `json:"reference"`
	Note string `json:"note"`
	// Line is the line of the fault in its file.
	Line int `json:"-"`
}

// LoadFaults reads the file of the faults of the references at path.
func LoadFaults(path string) ([]Fault, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening the faults of the references: %w", err)
	}
	defer f.Close()
	faults, err := ReadFaults(f)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return faults, nil
}

// ReadFaults reads a file of the faults of the references: a header line,
// then one line for each fault, with the columns name, page, reference and
// note, separated by tabs. An empty line and a line that starts with # are
// skipped. The page is a number from 1, or * for each page. The reference
// is the id of a reference, or * for each reference.
func ReadFaults(r io.Reader) ([]Fault, error) {
	sc := bufio.NewScanner(r)
	var faults []Fault
	header := false
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(text) == "" || strings.HasPrefix(text, "#") {
			continue
		}
		cols := strings.Split(text, "\t")
		if !header {
			if strings.Join(cols, "\t") != strings.Join(faultColumns, "\t") {
				return nil, fmt.Errorf("line %d: expected the header %q, got: %q", line, strings.Join(faultColumns, "\t"), text)
			}
			header = true
			continue
		}
		if len(cols) != len(faultColumns) {
			return nil, fmt.Errorf("line %d: expected %d columns, got: %d", line, len(faultColumns), len(cols))
		}
		f := Fault{
			Name: strings.TrimSpace(cols[0]),
			Ref:  strings.TrimSpace(cols[2]),
			Note: strings.TrimSpace(cols[3]),
			Line: line,
		}
		if f.Name == "" || f.Ref == "" || f.Note == "" {
			return nil, fmt.Errorf("line %d: expected a name, a reference and a note", line)
		}
		if p := strings.TrimSpace(cols[1]); p != "*" {
			n, err := strconv.Atoi(p)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("line %d: expected a page from 1 or *, got: %q", line, p)
			}
			f.Page = n
		}
		faults = append(faults, f)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading the faults: %w", err)
	}
	if !header {
		return nil, errors.New("the file of the faults has no header line")
	}
	return faults, nil
}

// faultNote returns the notes of the faults of the reference ref on page
// n, from 1, of a document with the names names, joined, or an empty
// string.
func faultNote(faults []Fault, names []string, n int, ref string) string {
	var notes []string
	for _, f := range faults {
		if (f.Page == 0 || f.Page == n) && (f.Ref == "*" || f.Ref == ref) && contains(names, f.Name) {
			notes = append(notes, f.Note)
		}
	}
	return strings.Join(notes, " ")
}
