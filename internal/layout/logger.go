// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/logger.py.

package layout

import (
	"context"
	"fmt"
	"log/slog"
)

// logger is LOGGER in weasyprint/logger.py. Upstream logs to a global logger
// of Python. The port passes a logger to each function that logs, so that two
// documents can be laid out at the same time. A nil logger logs nothing.
//
// The levels follow upstream: an error for a resource that cannot be read or
// used, a warning for HTML or CSS that is not valid or not supported, and a
// debug message for what a reader does not need to see. PROGRESS_LOGGER is
// not ported.
type logger struct {
	log *slog.Logger
}

// newLogger returns a logger that writes to log, or that discards each
// message when log is nil.
func newLogger(log *slog.Logger) *logger {
	return &logger{log: log}
}

// logf writes a message at a level.
func (l *logger) logf(level slog.Level, format string, args ...any) {
	if l == nil || l.log == nil {
		return
	}
	l.log.Log(context.Background(), level, fmt.Sprintf(format, args...))
}

// debug writes a debug message.
func (l *logger) debug(format string, args ...any) {
	l.logf(slog.LevelDebug, format, args...)
}

// warning writes a warning.
func (l *logger) warning(format string, args ...any) {
	l.logf(slog.LevelWarn, format, args...)
}

// error writes an error.
func (l *logger) error(format string, args ...any) {
	l.logf(slog.LevelError, format, args...)
}
