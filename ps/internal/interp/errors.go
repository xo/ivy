package interp

import (
	"context"
	"errors"
	"fmt"

	"github.com/xo/ivy"
)

// Error is an error of PostScript, such as typecheck (PLRM 3.11). An
// operator returns it, and the interpreter runs the procedure of the same
// name in errordict.
type Error struct {
	// Name is the name of the error, such as typecheck.
	Name string
}

// Error satisfies the error interface.
func (e *Error) Error() string {
	return "postscript error " + e.Name
}

// The errors of PostScript (PLRM 3.11.1).
var (
	ErrConfiguration     = &Error{"configurationerror"}
	ErrDictFull          = &Error{"dictfull"}
	ErrDictStackOverflow = &Error{"dictstackoverflow"}
	ErrDictStackUnder    = &Error{"dictstackunderflow"}
	ErrExecStackOverflow = &Error{"execstackoverflow"}
	ErrInterrupt         = &Error{"interrupt"}
	ErrInvalidAccess     = &Error{"invalidaccess"}
	ErrInvalidExit       = &Error{"invalidexit"}
	ErrInvalidFileAccess = &Error{"invalidfileaccess"}
	ErrInvalidFont       = &Error{"invalidfont"}
	ErrInvalidRestore    = &Error{"invalidrestore"}
	ErrIO                = &Error{"ioerror"}
	ErrLimitcheck        = &Error{"limitcheck"}
	ErrNoCurrentPoint    = &Error{"nocurrentpoint"}
	ErrRangecheck        = &Error{"rangecheck"}
	ErrStackOverflow     = &Error{"stackoverflow"}
	ErrStackUnderflow    = &Error{"stackunderflow"}
	ErrSyntax            = &Error{"syntaxerror"}
	ErrTimeout           = &Error{"timeout"}
	ErrTypecheck         = &Error{"typecheck"}
	ErrUndefined         = &Error{"undefined"}
	ErrUndefinedFilename = &Error{"undefinedfilename"}
	ErrUndefinedResource = &Error{"undefinedresource"}
	ErrUndefinedResult   = &Error{"undefinedresult"}
	ErrUnmatchedMark     = &Error{"unmatchedmark"}
	ErrUnregistered      = &Error{"unregistered"}
	ErrVM                = &Error{"VMerror"}
)

// The short names of the errors in this package.
var (
	errSyntax       = ErrSyntax
	errLimitcheck   = ErrLimitcheck
	errUndefined    = ErrUndefined
	errTypecheck    = ErrTypecheck
	errRangecheck   = ErrRangecheck
	errUnderflow    = ErrStackUnderflow
	errInvalidAcc   = ErrInvalidAccess
	errUndefResult  = ErrUndefinedResult
	errUnmatched    = ErrUnmatchedMark
	errInvalidExit  = ErrInvalidExit
	errExecOverflow = ErrExecStackOverflow
)

// allErrors lists the errors, for errordict.
var allErrors = []*Error{
	ErrConfiguration, ErrDictFull, ErrDictStackOverflow, ErrDictStackUnder,
	ErrExecStackOverflow, ErrInterrupt, ErrInvalidAccess, ErrInvalidExit,
	ErrInvalidFileAccess, ErrInvalidFont, ErrInvalidRestore, ErrIO,
	ErrLimitcheck, ErrNoCurrentPoint, ErrRangecheck, ErrStackOverflow,
	ErrStackUnderflow, ErrSyntax, ErrTimeout, ErrTypecheck, ErrUndefined,
	ErrUndefinedFilename, ErrUndefinedResource, ErrUndefinedResult,
	ErrUnmatchedMark, ErrUnregistered, ErrVM,
}

// The signals that unwind the execution stack. They are errors so that a Go
// function that calls a procedure passes them on.
var (
	// errStop is the signal of stop.
	errStop = errors.New("postscript stop")
	// errExit is the signal of exit.
	errExit = errors.New("postscript exit")
	// errQuit is the signal of quit.
	errQuit = errors.New("postscript quit")
)

// JobError is the error that ends a program: an error of PostScript that no
// stopped caught.
type JobError struct {
	// Name is the name of the error, such as undefined.
	Name string
	// Command is the text of the object that caused it.
	Command string
}

// Error satisfies the error interface.
func (e *JobError) Error() string {
	return fmt.Sprintf("postscript error %s in %s: %s", e.Name, e.Command, ivy.ErrFormat)
}

// Unwrap returns ivy.ErrFormat.
func (e *JobError) Unwrap() error {
	return ivy.ErrFormat
}

// fatal reports whether err ends the program at once: a limit, the end of
// the context, or quit.
func fatal(err error) bool {
	return errors.Is(err, ivy.ErrLimit) || errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errQuit)
}

// ioError returns the error of PostScript for an error of a file. An error
// that wraps ivy.ErrLimit stays as it is.
func ioError(err error) error {
	if fatal(err) {
		return err
	}
	var pe *Error
	if errors.As(err, &pe) {
		return pe
	}
	return ErrIO
}

// limitError returns the error of a limit of the interpreter.
func limitError(what string, n int64) error {
	return fmt.Errorf("postscript %s past %d: %w", what, n, ivy.ErrLimit)
}
