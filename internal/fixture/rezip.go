package fixture

import (
	"archive/zip"
	"bytes"
	"io"
)

// Rezip returns the zip archive buf with the entries: an entry with the
// name of an entry of buf takes its place, and the others go at the end.
// A test uses it to change a part of a package that a builder makes, such
// as the theme of a PPTX.
func Rezip(buf []byte, entries ...Entry) []byte {
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		panic(err)
	}
	extra := make(map[string]Entry, len(entries))
	for _, e := range entries {
		extra[e.Name] = e
	}
	out := make([]Entry, 0, len(zr.File)+len(entries))
	for _, f := range zr.File {
		if e, ok := extra[f.Name]; ok {
			out = append(out, e)
			delete(extra, f.Name)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			panic(err)
		}
		data, err := io.ReadAll(rc)
		if err != nil {
			panic(err)
		}
		if err := rc.Close(); err != nil {
			panic(err)
		}
		out = append(out, Entry{Name: f.Name, Data: data, Store: f.Method == zip.Store})
	}
	for _, e := range entries {
		if _, ok := extra[e.Name]; ok {
			out = append(out, e)
		}
	}
	return Zip(out...)
}
