# Test files of latex

The files are the work of ivy, under the MIT license of ivy in `LICENSE` in
the root of the repository. The tests of the package read them, and the
test module compares their pages with the pages of pdfLaTeX (D13). The
review corpus, `test/review/corpus.tsv`, holds each `.tex` file of this
folder.

| File | What it holds |
| --- | --- |
| `article.tex` | the class article: a title, sections, the styles of the text, ligatures, math in the text and in a display, a list, a table, a rule, a color and a link |
| `math.tex` | formulas of amsmath: a fraction, a root, an integral, a matrix and a numbered alignment |
| `boxes.tex` | the boxes of graphicx and of LaTeX: a scale, a rotation, a reflection, frames, an underline, the color boxes and the leaders |
| `plain.tex` | plain TeX without a class: rules, a box in a frame and skips |
| `livre.tex` | the class book in French with babel: the front matter, a table of contents, guillemets, a footnote, a list and `thebibliography` |
| `report/report.tex` | the class report with the headers of fancyhdr, a table of booktabs with `\multicolumn`, lists, footnotes, cross-references, a bibliography of BibTeX, figures and the transforms of graphicx |
| `report/report.bib`, `report/report.bbl` | the bibliography of `report.tex`, and the file that BibTeX made from it with the style plain |
| `report/figures/` | the figures of `report.tex`: `shapes.png` and `plasma.jpg`, which ImageMagick drew, and `vector.pdf`, which Ghostscript made from a small EPS file |
