package ebook

import (
	"slices"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
)

func TestTOC(t *testing.T) {
	toc := NewTOC(ivy.Limits{MaxObjects: 3, MaxDepth: 2})
	for _, e := range []layout.TOCEntry{
		{Label: " One\n two ", Level: 1, Target: "a.xhtml"},
		{Label: "", Level: 1, Target: "a.xhtml"},
		{Label: "No target", Level: 1},
		{Label: "Deep", Level: 3, Target: "a.xhtml"},
		{Label: "Zero", Level: 0, Target: "a.xhtml"},
		{Label: "Two", Level: 2, Target: "a.xhtml#x"},
		{Label: "Three", Level: 1, Target: "b.xhtml"},
	} {
		if !toc.Add(e.Label, e.Level, e.Target) {
			t.Fatalf("expected room for %+v", e)
		}
	}
	if !toc.Full() || toc.Add("Four", 1, "c.xhtml") {
		t.Error("expected a full table")
	}
	want := []layout.TOCEntry{
		{Label: "One two", Level: 1, Target: "a.xhtml"},
		{Label: "Two", Level: 2, Target: "a.xhtml#x"},
		{Label: "Three", Level: 1, Target: "b.xhtml"},
	}
	if !slices.Equal(toc.Entries, want) {
		t.Errorf("expected %+v, got: %+v", want, toc.Entries)
	}
}

func TestResolveTarget(t *testing.T) {
	for _, tt := range []struct {
		dir, href, want string
		ok              bool
	}{
		{"OEBPS", "text/ch1.xhtml", "OEBPS/text/ch1.xhtml", true},
		{"OEBPS/nav", "../text/ch%201.xhtml#a%20b", "OEBPS/text/ch 1.xhtml#a b", true},
		{"OEBPS", "ch1.xhtml#", "OEBPS/ch1.xhtml", true},
		{"OEBPS", "/top.xhtml#x", "top.xhtml#x", true},
		{"OEBPS", "#x", "", false},
		{"OEBPS", "https://example.com/a#b", "", false},
		{"", "../out.xhtml", "", false},
	} {
		got, ok := ResolveTarget(tt.dir, tt.href)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ResolveTarget(%q, %q): expected %q %v, got: %q %v", tt.dir, tt.href, tt.want, tt.ok, got, ok)
		}
	}
}
