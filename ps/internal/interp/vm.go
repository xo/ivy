package interp

import (
	"fmt"

	"github.com/xo/ivy"
)

// vm is the virtual memory (PLRM 3.7). It counts the bytes that the
// program allocates, and it keeps the journal of save and restore.
//
// save does not copy the memory. Before an array or a dictionary of local
// VM changes for the first time after a save, the journal keeps its
// contents. restore puts back the contents of the journal. As the PLRM
// allows, restore does not put back the contents of strings.
type vm struct {
	level   int
	journal []journalEntry
	saves   []*saveRec
	// global is true while the program allocates in global VM.
	global bool
	used   int64
	max    int64
	nextID uint64
}

// journalEntry holds the contents of an array or a dictionary before it
// changed at a level of save.
type journalEntry struct {
	level int
	saved int

	arr *arrBuf
	old []Object

	dict    *Dict
	m       map[key]int
	names   map[string]int
	entries []entry
	live    int
}

// saveRec is the value of a save object.
type saveRec struct {
	level int
	used  int64
	valid bool
	// state is the graphics state that the hook of save returned.
	state any
}

// alloc counts n bytes of memory. It returns an error that wraps
// ivy.ErrLimit past the limit of memory.
func (v *vm) alloc(n int64) error {
	if n < 0 || v.used+n > v.max {
		return fmt.Errorf("postscript memory of %d bytes: %w", v.max, ivy.ErrLimit)
	}
	v.used += n
	return nil
}

// id returns a new identity of a composite object.
func (v *vm) id() uint64 {
	v.nextID++
	return v.nextID
}

// touchArray keeps the contents of an array in the journal before it
// changes.
func (v *vm) touchArray(b *arrBuf) {
	if b.global || b.saved >= v.level {
		return
	}
	v.journal = append(v.journal, journalEntry{
		level: v.level,
		saved: b.saved,
		arr:   b,
		old:   append([]Object(nil), b.a...),
	})
	b.saved = v.level
}

// save starts a level of save.
func (v *vm) save(state any) *saveRec {
	v.level++
	s := &saveRec{level: v.level, used: v.used, valid: true, state: state}
	v.saves = append(v.saves, s)
	return s
}

// restore ends the levels of save up to and including s, and puts back the
// contents that changed. It returns the saves that it ended, the last one
// first.
func (v *vm) restore(s *saveRec) []*saveRec {
	var ended []*saveRec
	for len(v.saves) > 0 {
		last := v.saves[len(v.saves)-1]
		v.saves = v.saves[:len(v.saves)-1]
		last.valid = false
		ended = append(ended, last)
		if last == s {
			break
		}
	}
	i := len(v.journal)
	for i > 0 && v.journal[i-1].level >= s.level {
		i--
		e := v.journal[i]
		switch {
		case e.arr != nil:
			copy(e.arr.a, e.old)
			e.arr.saved = e.saved
		case e.dict != nil:
			e.dict.m = e.m
			e.dict.names = e.names
			e.dict.entries = e.entries
			e.dict.live = e.live
			e.dict.saved = e.saved
		}
	}
	clear(v.journal[i:])
	v.journal = v.journal[:i]
	v.level = s.level - 1
	v.used = min(v.used, s.used)
	return ended
}
