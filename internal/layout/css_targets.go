// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/css/targets.py: the targets of target-counter,
// target-counters and target-text.
//
// TargetCollector provides the counter values of the targets of
// target-counter(), target-counters() and target-text(), and what is needed
// to build the pending targets later, when the layout of all the targeted
// anchors is done.

package layout

// anchorNameFromToken is anchor_name_from_token in
// weasyprint/css/targets.py. It returns the name of an anchor from a string
// or a url token, and false for None.
func anchorNameFromToken(anchorToken Value) (string, bool) {
	t, ok := anchorToken.(Tuple)
	if !ok || len(t) != 2 {
		return "", false
	}
	switch {
	case isStr(t[0], "string"):
		if s := str(t[1]); len(s) > 0 && s[0] == '#' {
			return s[1:], true
		}
	case isStr(t[0], "url"):
		if u, ok := t[1].(Tuple); ok && len(u) == 2 && isStr(u[0], "internal") {
			return str(u[1]), true
		}
	}
	return "", false
}

// Target is Target in weasyprint/css/targets.py. It controls the pending
// targets and the target counters that depend on the pages.
type Target struct {
	// State is "pending", "up-to-date" or "undefined".
	State string
	// TargetBox is the box of the target. target-counter() and
	// target-counters() read its cached counter values, and target-text()
	// its text.
	TargetBox *Box
	// ParseAgainFunctions are the functions that check the pending targets.
	ParseAgainFunctions []func(map[string][]int)
	// PageMakerIndex is the index of the page of the anchor during the
	// pagination, or -1 for None.
	PageMakerIndex int
	// CachedPageCounterValues are the page counters of the target box
	// during the pagination.
	CachedPageCounterValues map[string][]int
}

// newTarget is Target.__init__ in weasyprint/css/targets.py.
func newTarget(state string) *Target {
	return &Target{State: state, PageMakerIndex: -1, CachedPageCounterValues: map[string][]int{}}
}

// TargetCounter is Counter in weasyprint/css/targets.py. It controls the
// counters that depend on the pages.
type TargetCounter struct {
	// ParseAgain is the function that checks the pending counter.
	ParseAgain func(map[string][]int)
	// MissingCounters and MissingTargetCounters are the missing counters
	// and target counters, in the order that they are found.
	MissingCounters       []string
	MissingTargetCounters map[string][]string
	// PageMakerIndex is the index of the page of the box during the
	// pagination, or -1 for None.
	PageMakerIndex int
	// Pending marks the counter for make_page.
	Pending bool
	// CachedPageCounterValues are the page counters of the targeting box
	// during the pagination.
	CachedPageCounterValues map[string][]int
}

// TargetCollector is TargetCollector in weasyprint/css/targets.py: the
// collector of the HTML targets that the CSS content with target-* uses.
type TargetCollector struct {
	// Targets are the targets, by anchor name.
	Targets map[string]*Target
	// HadPendingTargets is set when a target is needed but not seen yet.
	// CheckPendingTargets then calls the needed parse_again functions.
	HadPendingTargets bool

	// Counters are the counters that depend on the pages, by the box that
	// needs them and its CSS token.
	Counters map[CounterKey]*TargetCounter

	// order holds the names of Targets in the order of CollectAnchor, the
	// order of the dict of upstream.
	order []string
	// counterOrder holds the keys of Counters in the order of the dict of
	// upstream.
	counterOrder []CounterKey
	log          *logger
}

// CounterKey is the key of TargetCollector.Counters: the box that needs the
// counters and its CSS token, such as "content".
type CounterKey struct {
	Box      *Box
	CSSToken string
}

// NewTargetCollector is TargetCollector.__init__ in
// weasyprint/css/targets.py.
func NewTargetCollector(log *logger) *TargetCollector {
	return &TargetCollector{Targets: map[string]*Target{}, Counters: map[CounterKey]*TargetCounter{}, log: log}
}

// CollectAnchor is TargetCollector.collect_anchor in
// weasyprint/css/targets.py. It makes a Target for an anchor name.
func (tc *TargetCollector) CollectAnchor(anchorName string) {
	if _, ok := tc.Targets[anchorName]; ok {
		tc.log.warning("Anchor defined twice: %q", anchorName)
		return
	}
	tc.Targets[anchorName] = newTarget("pending")
	tc.order = append(tc.order, anchorName)
}

// LookupTarget is TargetCollector.lookup_target in
// weasyprint/css/targets.py. It returns the Target of an anchor token. When
// a previous anchor element filled it, its state is "up-to-date". Else it is
// "pending", and the whole tree must be parsed again.
func (tc *TargetCollector) LookupTarget(anchorToken Value, sourceBox *Box, _ string, parseAgain func(map[string][]int)) *Target {
	if t, ok := anchorToken.(Tuple); ok && len(t) == 2 && isStr(t[0], "attr()") {
		if args, ok := t[1].(Tuple); ok && len(args) == 3 {
			attrName, typ, fallback := str(args[0]), str(args[1]), str(args[2])
			attrValue := fallback
			if sourceBox.Element != nil {
				attrValue = sourceBox.Element.GetDefault(attrName, fallback)
			}
			switch typ {
			case "string":
				anchorToken = Tuple{String("string"), String(attrValue)}
			case "url":
				anchorToken = Tuple{String("url"), getURLTuple(attrValue, sourceBox.Style.BaseURL)}
			}
		}
	}
	anchorName, _ := anchorNameFromToken(anchorToken)
	item, ok := tc.Targets[anchorName]
	if !ok {
		item = newTarget("undefined")
	}

	if item.State == "pending" {
		tc.HadPendingTargets = true
		item.ParseAgainFunctions = append(item.ParseAgainFunctions, parseAgain)
	}

	if item.State == "undefined" {
		tc.log.error("Content discarded: target points to undefined anchor %s", ValueString(anchorToken))
	}

	return item
}

// CheckPendingTargets is TargetCollector.check_pending_targets in
// weasyprint/css/targets.py.
func (tc *TargetCollector) CheckPendingTargets() {
	if !tc.HadPendingTargets {
		return
	}
	for _, name := range tc.order {
		for _, parseAgain := range tc.Targets[name].ParseAgainFunctions {
			parseAgain(map[string][]int{})
		}
	}
	tc.HadPendingTargets = false
}

// StoreTarget is TargetCollector.store_target in weasyprint/css/targets.py.
// It stores the target of an anchor name. A pending Target is updated. Only
// the anchors that CollectAnchor collected are stored.
func (tc *TargetCollector) StoreTarget(anchorName string, targetCounterValues map[string][]int, targetBox *Box) {
	item := tc.Targets[anchorName]
	if item != nil && item.State == "pending" {
		item.State = "up-to-date"
		item.TargetBox = targetBox
		// Store the counter_values in the target_box like
		// compute_content_list does.
		if targetBox.CachedCounterValues == nil {
			targetBox.CachedCounterValues = copyCounterValues(targetCounterValues)
		}
	}
}

// CollectMissingCounters is TargetCollector.collect_missing_counters in
// weasyprint/css/targets.py. It collects the missing counters, which
// probably depend on the pages, while the boxes are built. The pagination
// uses the missing counters again.
//
// The attribute MissingLink of the parent box connects the boxes of the
// pages to the parent box that they come from.
func (tc *TargetCollector) CollectMissingCounters(parentBox *Box, cssToken string, parseAgainFunction func(map[string][]int), missingCounters []string, missingTargetCounters map[string][]string) {
	// No need to add empty miss-lists.
	if len(missingCounters) > 0 || len(missingTargetCounters) > 0 {
		if parentBox.MissingLink == nil {
			parentBox.MissingLink = parentBox
		}
		key := CounterKey{parentBox, cssToken}
		if _, ok := tc.Counters[key]; !ok {
			tc.counterOrder = append(tc.counterOrder, key)
		}
		tc.Counters[key] = &TargetCounter{
			ParseAgain:              parseAgainFunction,
			MissingCounters:         missingCounters,
			MissingTargetCounters:   missingTargetCounters,
			PageMakerIndex:          -1,
			CachedPageCounterValues: map[string][]int{},
		}
	}
}

// CacheTargetPageCounters is TargetCollector.cache_target_page_counters in
// weasyprint/css/targets.py. It stores the current page maker index and the
// page counter values of a target, and updates the boxes that target it.
// pageMakerIndex is -1 for None, and pageMakerLen is the length of the page
// maker of the layout.
func (tc *TargetCollector) CacheTargetPageCounters(anchorName string, pageCounterValues map[string][]int, pageMakerIndex, pageMakerLen int) {
	item := tc.Targets[anchorName]
	if item == nil || item.State != "up-to-date" {
		return
	}
	if counterValuesEqual(item.CachedPageCounterValues, pageCounterValues) {
		return
	}

	item.PageMakerIndex = pageMakerIndex
	item.CachedPageCounterValues = copyCounterValues(pageCounterValues)

	// Spread the news: update boxes affected by a change in the anchor's
	// page counter values.
	for _, key := range tc.counterOrder {
		counter := tc.Counters[key]
		// Only update items that need counters in their content.
		if key.CSSToken != "content" {
			continue
		}

		// Don't update if item has no missing target counter.
		if _, ok := counter.MissingTargetCounters[anchorName]; !ok {
			continue
		}

		// Pending marker for make_page.
		if pageMakerIndex < 0 || pageMakerIndex >= pageMakerLen {
			counter.Pending = true
		}
	}
}

// counterValuesEqual reports whether two maps of counter values are equal,
// as the operator == of Python does for dicts.
func counterValuesEqual(a, b map[string][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		other, ok := b[key]
		if !ok || len(value) != len(other) {
			return false
		}
		for i := range value {
			if value[i] != other[i] {
				return false
			}
		}
	}
	return true
}
