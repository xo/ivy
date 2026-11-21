# D65. A program gives LaTeX its TeX files and its folder

Status: Decided.

D62 says that a program gives the classes and the packages of LaTeX
through `ivy.Config`, and it names no field. A LaTeX document also reads
the files of its own folder: the files of `\input` and `\include`, its
figures, and its bibliography. The build of phase 8a needs a name for both.
Question 28 of `PLAN.md` asked the maintainers to choose. The build uses the
names below until they choose.

## The facts

1. The module of ivy holds no file of LaTeX under the LaTeX Project Public
   License (D62). Without a class, the engine sets a document in a plain
   form, and the text of the document is on the pages.
2. A TeX distribution keeps its files in a tree, such as
   `tex/latex/base/article.cls`, and TeX finds a file by its base name. A
   tree of TeX Live holds more than 100,000 files.
3. The documents of a program share the classes and the packages, but each
   document has its own folder.
4. The package `djvu` has the option `WithDir(dir fs.FS)` for the folder of
   an indirect document (D38).
5. The engine of D62 reads no file of the disk and no variable of the
   environment, because a document is input from a stranger (D5). It reads
   each file through a resolver that the package `latex` gives it.

## The decision

1. `ivy.Config` has the field `TeX fs.FS`: the files of TeX that the
   documents of a program share. The package `latex` finds a file by its
   base name, anywhere in the tree, within `Limits`. It walks a tree once
   for each value that a map can hold, such as an `embed.FS` or an
   `os.DirFS`.
2. The package `latex` has the option `WithDir(dir fs.FS)`: the folder of
   one document. The document reads a file that it names by its path
   relative to the folder. A file of the folder comes before a file of the
   tree, as TeX finds a file of the folder of the document first.
3. The module `fonts` has the function `TeX() fs.FS`, which gives Latin
   Modern and Latin Modern Math in the folder `lm`. `FS` does not give
   them: in `FS`, the font database of the other formats took Latin Modern
   for a font of Office, and the page of one file of the corpus,
   `poi/xls/landscape-a4-drawing.xls`, changed. A program gives the fonts of
   TeX with the tree, as a TeX distribution holds them:
   `ivy.Config{Fonts: fonts.FS(), TeX: fonts.TeX()}`.
4. The package `latex` sets the text in Latin Modern and the math in Latin
   Modern Math when `Config.Fonts` or `Config.TeX` holds their files. It
   finds them by their names. Without them, it sets the text in the Go
   fonts and the math in STIX Two Math of okage.

## The questions

1. Is a field of `ivy.Config` the right place for the tree, or an option
   of the package `latex`, as the folder is?
2. Is `TeX` the right name of the field, or `TeXInputs`, after the
   variable TEXINPUTS of TeX? Is `fonts.TeX` the right name of the
   function?
3. A program that gives a tree of a TeX distribution and also wants the
   fonts of the module fonts needs one `fs.FS` of both. Does the field take
   more than one tree?
4. With TeX Live as the tree, the engine fails on the real `amsmath.sty`:
   the text of a document with amsmath goes missing. With the small set of
   files that upstream embeds, it does not. Can a program give a tree of
   TeX Live, or does ivy say which files a tree holds?

## The decision

On 2026-11-21 the maintainers accepted the field `TeX` of `ivy.Config`, the
option `WithDir` of `latex` and the function `TeX` of the module `fonts`, as
the build of phase 8a has them, with one tree of TeX files. They also
accepted that the module `fonts` holds 25 files of Latin Modern with the
GUST Font License and the two MANIFEST files. The engine still fails on the
`amsmath.sty` of TeX Live, and the backlog keeps that item.
