package engine

import "testing"

func TestSVGRootAttrs(t *testing.T) {
	t.Parallel()
	attrs, ok := svgRootAttrs([]byte(`<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50" viewBox="0 0 100 50"><rect/></svg>`))
	if !ok || attrs["width"] != "100" || attrs["viewBox"] != "0 0 100 50" {
		t.Errorf("expected the attributes of the root, got: %v %v", attrs, ok)
	}
	if w, h := svgIntrinsic(attrs); w != 100 || h != 50 {
		t.Errorf("expected a size of 100x50, got: %dx%d", w, h)
	}
	if _, ok := svgRootAttrs([]byte(`<html/>`)); ok {
		t.Error("expected no svg root")
	}
}

func TestParseTransform(t *testing.T) {
	t.Parallel()
	// a parenthesis that closes before the one that opens
	if m := parseTransform("a)b(c"); m != identity() {
		t.Errorf("expected the identity, got: %v", m)
	}
	if m := parseTransform("translate(1,2) scale(3)"); m != (affine{3, 0, 0, 3, 1, 2}) {
		t.Errorf("expected a translation and a scale, got: %v", m)
	}
}
