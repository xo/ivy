// Package test holds the comparisons of ivy with its oracles (D13).
//
// The oracles are outside programs, such as ImageMagick, calibre and
// LibreOffice. This module is the only place that runs them, and CI does not
// run it. A maintainer runs it by hand, on a machine that has the oracles:
//
//	cd test && go test ./...
//
// A test that cannot find its oracle fails, and names the program.
package test
