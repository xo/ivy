package latex

import (
	"io/fs"
	"path"
	"reflect"
	"strings"
	"sync"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
	"github.com/xo/ivy/latex/internal/engine"
)

// maxIndex bounds the files of a tree that an index holds, so that a tree
// that is too large does not take the memory.
const maxIndex = 1 << 20

// indexes keeps the index of each tree whose value a map can hold, such as
// an embed.FS or an os.DirFS, so that the documents of a program walk each
// tree once.
var indexes sync.Map

// treeIndex maps the base name of each file of a tree to its path. The
// first path of a name in the order of fs.WalkDir wins.
type treeIndex struct {
	once  sync.Once
	paths map[string]string
}

// indexOf returns the index of the tree fsys.
func indexOf(fsys fs.FS, lim ivy.Limits) map[string]string {
	if fsys == nil {
		return nil
	}
	var idx *treeIndex
	if reflect.TypeOf(fsys).Comparable() {
		v, _ := indexes.LoadOrStore(fsys, new(treeIndex))
		idx = v.(*treeIndex)
	} else {
		idx = new(treeIndex)
	}
	idx.once.Do(func() {
		idx.paths = walkIndex(fsys, lim)
	})
	return idx.paths
}

// walkIndex walks the tree fsys, within the limits.
func walkIndex(fsys fs.FS, lim ivy.Limits) map[string]string {
	paths := make(map[string]string)
	n := 0
	limit := min(lim.MaxObjects, maxIndex)
	_ = fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			// a folder that the tree cannot read is left out
			return nil //nolint:nilerr // the walk goes on
		}
		if d.IsDir() {
			if strings.Count(name, "/") >= lim.MaxDepth {
				return fs.SkipDir
			}
			return nil
		}
		n++
		if n > limit {
			return fs.SkipAll
		}
		base := path.Base(name)
		if _, ok := paths[base]; !ok {
			paths[base] = name
		}
		return nil
	})
	return paths
}

// readFile reads a file of fsys, within the memory of the limits.
func readFile(fsys fs.FS, name string, lim ivy.Limits) ([]byte, bool) {
	if st, err := fs.Stat(fsys, name); err != nil || st.IsDir() || st.Size() > lim.MaxMemory {
		return nil, false
	}
	data, err := fs.ReadFile(fsys, name)
	if err != nil || int64(len(data)) > lim.MaxMemory {
		return nil, false
	}
	return data, true
}

// resolver gives the engine the files that a document names: first from
// the folder of the document, by the path, and then from the tree of TeX
// files, by the base name.
type resolver struct {
	dir  fs.FS
	tex  fs.FS
	idx  map[string]string
	lim  ivy.Limits
	read int64
}

// newResolver returns the resolver of a run.
func newResolver(dir, tex fs.FS, lim ivy.Limits) *resolver {
	return &resolver{dir: dir, tex: tex, idx: indexOf(tex, lim), lim: lim}
}

// resolve returns the file name. The total of the files of a run is
// bounded by the memory of the limits, ten times.
func (r *resolver) resolve(name string) ([]byte, bool) {
	data, ok := r.find(name)
	if !ok {
		return nil, false
	}
	r.read += int64(len(data))
	if r.read > 10*r.lim.MaxMemory {
		return nil, false
	}
	return data, true
}

// find finds the file name.
func (r *resolver) find(name string) ([]byte, bool) {
	clean := strings.TrimPrefix(path.Clean(strings.ReplaceAll(name, `\`, "/")), "./")
	if r.dir != nil && fs.ValidPath(clean) {
		if data, ok := readFile(r.dir, clean, r.lim); ok {
			return data, true
		}
	}
	if r.tex != nil {
		if p, ok := r.idx[path.Base(clean)]; ok {
			return readFile(r.tex, p, r.lim)
		}
	}
	return nil, false
}

// faceCache keeps the faces of each pair of trees whose values a map can
// hold.
var faceCache sync.Map

// treePair is the key of faceCache: the font tree and the tree of TeX.
type treePair struct {
	fonts, tex fs.FS
}

// cachedFaces is the faces of a font tree, read once.
type cachedFaces struct {
	once  sync.Once
	faces engine.Faces
}

// keyable reports whether a map can hold fsys as a key.
func keyable(fsys fs.FS) bool {
	return fsys == nil || reflect.TypeOf(fsys).Comparable()
}

// loadFaces returns the faces of a run: Latin Modern from the font tree
// fonts or from the tree of TeX tex, as fonts.TeX of the module fonts and a
// TeX distribution hold it, or the Go fonts and STIX Two Math without it.
func loadFaces(fonts, tex fs.FS, lim ivy.Limits) engine.Faces {
	if fonts == nil && tex == nil {
		return goFaces()
	}
	var c *cachedFaces
	if keyable(fonts) && keyable(tex) {
		v, _ := faceCache.LoadOrStore(treePair{fonts, tex}, new(cachedFaces))
		c = v.(*cachedFaces)
	} else {
		c = new(cachedFaces)
	}
	c.once.Do(func() {
		c.faces = latinModern([]fs.FS{fonts, tex}, lim)
	})
	return c.faces
}

// latinModern reads the files of Latin Modern from the first tree that
// holds each one.
func latinModern(trees []fs.FS, lim ivy.Limits) engine.Faces {
	files := make(map[string][]byte)
	for _, fsys := range trees {
		if fsys == nil {
			continue
		}
		idx := indexOf(fsys, lim)
		for _, name := range engine.LatinModernFiles() {
			if _, ok := files[name]; ok {
				continue
			}
			p, ok := idx[name+".otf"]
			if !ok {
				continue
			}
			if data, ok := readFile(fsys, p, lim); ok {
				files[name] = data
			}
		}
	}
	lm := engine.RegisterLatinModern(files)
	if lm.Regular == nil {
		faces := goFaces()
		faces.Math = lm.Math
		return faces
	}
	return engine.Faces{
		Regular: lm.Regular,
		Bold:    lm.Bold,
		Italic:  lm.Italic,
		Mono:    lm.Mono,
		Math:    lm.Math,

		Sans:           lm.Sans,
		SansBold:       lm.SansBold,
		SansItalic:     lm.SansItalic,
		SansBoldItalic: lm.SansBoldItalic,
	}
}

// goFaces returns the Go fonts. The math takes STIX Two Math of okage.
func goFaces() engine.Faces {
	return engine.Faces{
		Regular: goregular.TTF,
		Bold:    gobold.TTF,
		Italic:  goitalic.TTF,
		Mono:    gomono.TTF,
	}
}
