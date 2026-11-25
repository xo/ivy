# D66. A program gives LaTeX the job name of a document

Status: Decided.

TeX names the files that a run writes and reads after the job name of the
document: `\jobname.aux`, `\jobname.toc` and `\jobname.bbl`. pdfLaTeX takes
the job name from the name of the input file. The package `latex` reads a
document from an `ivy.Source`, which has no name, so the engine uses its
default job name, `texput`. A document of the review corpus,
`repo/tex/report.tex`, then looks for `texput.bbl`, finds no file, and
loses its bibliography and the numbers of its `\cite` commands. The review
of LaTeX of 2026-11-21 found this ("Found by the review of LaTeX" in
`BACKLOG.md`).

## The options

1. A new option of `latex`, `WithName`, gives the job name. A program that
   opens `report.tex` gives `WithName("report")`. The name is part of the
   public API (D11).
2. `latex` looks in the folder of the document for one `.bbl` file and
   uses it. This needs no new API, but it guesses, and it does not help the
   other files that a run reads by the job name, such as `.aux` and `.toc`
   when a program gives them.

## The decision

On 2026-11-21 the maintainers chose option 1. The package `latex` gets this
option:

```go
// WithName gives the job name of the document: the name of its input file
// without the extension, such as "report" for report.tex. The engine reads
// the files of the run that carry the job name, such as report.bbl, from the
// folder of WithDir. Without it, the job name is "texput", as in TeX.
func WithName(name string) Option // package latex
```

A name that is empty, or that holds a slash, is the same as no name. The
review gives each TeX document of the corpus the name of its file.

## The build

The option was built on 2026-11-25. The engine now reads the files of the
job name, such as `report.bbl`, through the resolver of the files of the
document.
