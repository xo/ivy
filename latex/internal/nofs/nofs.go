// Package nofs gives the engine of LaTeX the part of the API of the package
// os that it calls, without the file system and without the environment
// (D62). The engine is a copy, and its files import this package in place
// of os, so that a new commit of the engine still applies.
//
// A document is input from a stranger (D5). Through os, a document could
// read a file of the machine with \input, and the variables of the
// environment would change the pages. Here each read fails as a missing
// file does, each variable is empty, and Stderr discards what it receives.
// The engine reads the files that a program gives through the resolver of
// its options instead.
package nofs

import (
	"io"
	"io/fs"
)

// Stderr discards the trace of the engine.
var Stderr = io.Discard

// ErrPermission is the error of a read that is not allowed.
var ErrPermission = fs.ErrPermission

// PathError is the error of a read of a path.
type PathError = fs.PathError

// Getenv returns the empty string for each variable.
func Getenv(string) string {
	return ""
}

// Getwd returns an error, because a document has no folder on the disk.
func Getwd() (string, error) {
	return "", &fs.PathError{Op: "getwd", Path: ".", Err: fs.ErrNotExist}
}

// ReadFile returns an error that wraps fs.ErrNotExist for each name.
func ReadFile(name string) ([]byte, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}
