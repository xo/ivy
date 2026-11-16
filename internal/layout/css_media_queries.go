// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/media_queries.py. See
// https://www.w3.org/TR/mediaqueries-4/.

package layout

import (
	"slices"

	"github.com/xo/okage/tinycss2"
)

// evaluateMediaQuery is evaluate_media_query in
// weasyprint/css/media_queries.py. It reports whether the query list matches
// the media type of the device.
func evaluateMediaQuery(queryList []string, deviceMediaType string) bool {
	// TODO: actual support for media queries, not just media types
	return slices.Contains(queryList, "all") || slices.Contains(queryList, deviceMediaType)
}

// parseMediaQuery is parse_media_query in weasyprint/css/media_queries.py.
// It returns the media types of a query, and false for None.
func parseMediaQuery(log *logger, tokens []tinycss2.Node) ([]string, bool) {
	tokens = removeWhitespace(tokens)
	if len(tokens) == 0 {
		return []string{"all"}, true
	}
	var media []string
	if ident, ok := tokens[0].(*tinycss2.IdentToken); ok && ident.LowerValue == "only" {
		tokens = tokens[1:]
	}
	for _, part := range splitOnComma(tokens) {
		if len(part) == 1 {
			if ident, ok := part[0].(*tinycss2.IdentToken); ok {
				media = append(media, ident.LowerValue)
				continue
			}
		}
		log.warning("Expected a media type, got %q", tinycss2.Serialize(part))
		return nil, false
	}
	return media, true
}
