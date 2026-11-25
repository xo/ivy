// This file of ivy replaces lmfonts.go and texmf.go of go-tex/engine (D62).
// Upstream embeds Latin Modern and the classes of LaTeX. ivy embeds
// neither: the module of ivy holds no file under the LaTeX Project Public
// License, and the fonts are in the module fonts (D24). A program gives
// them, and the run passes them in its options.

package engine

// The default faces of upstream. They are nil, and a run of ivy always
// names its faces and its math renderer, so api.go and math.go never take
// them.
var (
	lmRomanRegular []byte
	lmRomanBold    []byte
	lmRomanItalic  []byte
	lmMonoRegular  []byte
	lmMath         []byte
)

// hostTeXFile resolves one exact file name with the resolver of the
// options. Upstream also looks in the files that it embeds, and ivy embeds
// none.
func (e *Engine) hostTeXFile(name string) ([]byte, string, bool) {
	if e.resolve != nil {
		if data, ok := e.resolve(name); ok {
			return data, "<host>/" + name, true
		}
	}
	return nil, "", false
}

// readJobFile reads a file of the run that a document names, such as
// \jobname.bbl or the .bib of \bibliography. Upstream reads it from the
// disk with readDocFile. A run of ivy has no disk, so it reads the file
// with the resolver of the options, which reads the folder of the document
// first.
func (e *Engine) readJobFile(name string) ([]byte, error) {
	data, err := readDocFile(name)
	if err == nil {
		return data, nil
	}
	if data, _, ok := e.hostTeXFile(name); ok {
		return data, nil
	}
	return nil, err
}
