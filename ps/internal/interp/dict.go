package interp

import "math"

// key is the key of an entry of a dictionary. A string key is a name, and a
// real key with an integer value is an integer (PLRM 3.3.9).
type key struct {
	s string
	i int64
	t Type
}

// entry is an entry of a dictionary.
type entry struct {
	k, v Object
	live bool
}

// Dict is a dictionary. It keeps the order in which its keys were defined,
// so that forall runs in the same order on each run.
type Dict struct {
	// names maps the keys that are names, the most common keys, to their
	// entries: a map with string keys is the fastest map of Go. m maps the
	// other keys.
	names   map[string]int
	m       map[key]int
	entries []entry
	live    int
	max     int
	access  Access
	global  bool
	id      uint64
	saved   int
	vm      *vm
}

// keyOf returns the key of an object, and false for null and for an
// object that cannot be a key.
func keyOf(o Object) (key, bool) {
	switch o.typ {
	case TName:
		return key{t: TName, s: o.NameVal()}, true
	case TString:
		return key{t: TName, s: o.Text()}, true
	case TInt:
		return key{t: TInt, i: o.i}, true
	case TReal:
		if o.r == math.Trunc(o.r) && math.Abs(o.r) < 1<<53 {
			return key{t: TInt, i: int64(o.r)}, true
		}
		return key{t: TReal, i: int64(math.Float64bits(o.r))}, true
	case TBool:
		return key{t: TBool, i: o.i}, true
	case TArray, TPacked:
		return key{t: TArray, i: int64(o.ref.(*arrBuf).id)}, true
	case TDict:
		return key{t: TDict, i: int64(o.Dict().id)}, true
	case TOperator:
		return key{t: TOperator, s: o.Op().Name}, true
	case TFontID:
		return key{t: TFontID, i: o.i}, true
	case TNull:
		return key{}, false
	}
	return key{t: o.typ}, true
}

// storedKey returns the key object that a dictionary keeps: a name for a
// string, and an integer for a real with an integer value.
func storedKey(o Object) Object {
	switch o.typ {
	case TString:
		return NameObj(o.Text())
	case TReal:
		if o.r == math.Trunc(o.r) && math.Abs(o.r) < 1<<31 {
			return Int(int(o.r))
		}
	}
	o.exec = false
	return o
}

// index returns the entry of a key.
func (d *Dict) index(k key) (int, bool) {
	if k.t == TName {
		i, ok := d.names[k.s]
		return i, ok
	}
	i, ok := d.m[k]
	return i, ok
}

// setIndex sets the entry of a key.
func (d *Dict) setIndex(k key, i int) {
	if k.t == TName {
		if d.names == nil {
			d.names = make(map[string]int)
		}
		d.names[k.s] = i
		return
	}
	if d.m == nil {
		d.m = make(map[key]int)
	}
	d.m[k] = i
}

// deleteIndex removes the entry of a key.
func (d *Dict) deleteIndex(k key) {
	if k.t == TName {
		delete(d.names, k.s)
		return
	}
	delete(d.m, k)
}

// Len returns the number of the entries.
func (d *Dict) Len() int {
	return d.live
}

// MaxLen returns the capacity of the dictionary, as maxlength gives it.
func (d *Dict) MaxLen() int {
	return max(d.max, d.live)
}

// Get returns the value of a key.
func (d *Dict) Get(k Object) (Object, bool) {
	kk, ok := keyOf(k)
	if !ok {
		return Null, false
	}
	i, ok := d.index(kk)
	if !ok {
		return Null, false
	}
	return d.entries[i].v, true
}

// GetName returns the value of the name s.
func (d *Dict) GetName(s string) (Object, bool) {
	i, ok := d.names[s]
	if !ok {
		return Null, false
	}
	return d.entries[i].v, true
}

// Known reports whether the dictionary holds a key.
func (d *Dict) Known(k Object) bool {
	_, ok := d.Get(k)
	return ok
}

// touch keeps the contents of the dictionary in the journal of the save
// level before the dictionary changes.
func (d *Dict) touch() {
	if d.vm == nil || d.global || d.saved >= d.vm.level {
		return
	}
	m := make(map[key]int, len(d.m))
	for k, v := range d.m {
		m[k] = v
	}
	names := make(map[string]int, len(d.names))
	for k, v := range d.names {
		names[k] = v
	}
	d.vm.journal = append(d.vm.journal, journalEntry{
		level:   d.vm.level,
		dict:    d,
		m:       m,
		names:   names,
		entries: append([]entry(nil), d.entries...),
		live:    d.live,
		saved:   d.saved,
	})
	d.saved = d.vm.level
}

// Put sets the value of a key. It returns false for a key that cannot be a
// key, such as null.
func (d *Dict) Put(k, v Object) bool {
	kk, ok := keyOf(k)
	if !ok {
		return false
	}
	d.touch()
	if i, ok := d.index(kk); ok {
		d.entries[i].v = v
		return true
	}
	if d.vm != nil {
		d.vm.used += 96
	}
	d.setIndex(kk, len(d.entries))
	d.entries = append(d.entries, entry{k: storedKey(k), v: v, live: true})
	d.live++
	return true
}

// PutName sets the value of the name s.
func (d *Dict) PutName(s string, v Object) {
	d.Put(NameObj(s), v)
}

// Delete removes a key.
func (d *Dict) Delete(k Object) {
	kk, ok := keyOf(k)
	if !ok {
		return
	}
	i, ok := d.index(kk)
	if !ok {
		return
	}
	d.touch()
	d.deleteIndex(kk)
	d.entries[i] = entry{}
	d.live--
	// a dictionary that loses most of its entries packs them
	if len(d.entries) > 64 && d.live < len(d.entries)/4 {
		d.compact()
	}
}

// compact drops the entries that Delete freed.
func (d *Dict) compact() {
	entries := make([]entry, 0, d.live)
	for _, e := range d.entries {
		if e.live {
			kk, _ := keyOf(e.k)
			d.setIndex(kk, len(entries))
			entries = append(entries, e)
		}
	}
	d.entries = entries
}

// Each calls fn for each entry, in the order of definition, until fn
// returns false. fn can change the dictionary.
func (d *Dict) Each(fn func(k, v Object) bool) {
	for i := 0; i < len(d.entries); i++ {
		e := d.entries[i]
		if e.live && !fn(e.k, e.v) {
			return
		}
	}
}

// Keys returns the keys, in the order of definition.
func (d *Dict) Keys() []Object {
	keys := make([]Object, 0, d.live)
	for _, e := range d.entries {
		if e.live {
			keys = append(keys, e.k)
		}
	}
	return keys
}

// SetAccess sets the access of the dictionary.
func (d *Dict) SetAccess(a Access) {
	d.access = a
}

// Writable reports whether a program can change the dictionary.
func (d *Dict) Writable() bool {
	return d.access == Unlimited
}
