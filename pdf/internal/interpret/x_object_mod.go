// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-interpret src/x_object/mod.rs.

package interpret

import "github.com/xo/ivy/pdf/internal/syntax"

// xObject is XObject in hayro-interpret src/x_object/mod.rs. It is a form
// or an image.
//
// Port: upstream is an enum. Go sets one of the two fields.
type xObject struct {
	form  *formXObject
	image *imageXObject
}

// newXObject is XObject::new in hayro-interpret src/x_object/mod.rs.
func newXObject(stream syntax.Stream, resolveCS func(syntax.Name) (ColorSpace, bool), settings *InterpreterSettings, cache *Cache, transferFunction *ActiveTransferFunction) (xObject, bool) {
	subtype, ok := stream.Dict().Get[syntax.Name](syntax.KeySubtype)
	if !ok {
		return xObject{}, false
	}
	switch subtype {
	case syntax.KeyImage:
		img, ok := newImageXObject(stream, resolveCS, settings, cache, transferFunction)
		if !ok {
			return xObject{}, false
		}
		return xObject{image: img}, true
	case syntax.KeyForm:
		form, ok := newFormXObject(stream)
		if !ok {
			return xObject{}, false
		}
		return xObject{form: form}, true
	}
	return xObject{}, false
}

// draw is XObject::draw in hayro-interpret src/x_object/mod.rs.
func (x xObject) draw(resources *syntax.Resources, ctx *Context, device Device) {
	if x.form != nil {
		x.form.draw(resources, ctx, device)
		return
	}
	x.image.draw(ctx, device)
}

// xobjectOC is xobject_oc in hayro-interpret src/x_object/mod.rs. It enters
// the optional content of an external object, and reports whether it did.
func xobjectOC(dict syntax.Dict, ctx *Context) bool {
	ocDict, ok := dict.Get[syntax.Dict](syntax.KeyOC)
	if !ok {
		return false
	}
	if ocRef, ok := dict.GetRef(syntax.KeyOC); ok {
		ctx.ocgState.beginOcg(ocDict, ocRef.ID())
	} else {
		ctx.ocgState.beginOcmd(ocDict)
	}
	return true
}
