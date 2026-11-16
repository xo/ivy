# D22. A program can give the folder of a Markdown or HTML file

Status: Decided.

A Markdown file links to its images by paths relative to its folder. An
HTML file links to its style sheets, its images and its fonts in the same
way. An `ivy.Source` gives the bytes of a file and not its folder. So
`markdown` drew the alternate text of each image, and the layout read no
file that a plain HTML file links to. This was question 3 of
`docs/PLAN.md`.

On 2026-11-17 the maintainers decided that a program can give the folder:

1. `markdown.WithDir(dir fs.FS)` gives the folder of a Markdown file. The
   package reads each image from it, within the limits of the
   configuration, and copies the image into the document.
2. `html.WithDir(dir fs.FS)` gives the folder of an HTML file. The layout
   reads the files that the document links to from it. The document is at
   the root of the folder. An HTMLZ file holds its own files, and it
   ignores the folder.
3. Neither package reads a link out of the folder, an absolute path of the
   system or a URL with a scheme other than `data`. `fs.FS` refuses a path
   out of the folder.
4. Without the option, each package works as before.

The option is an option of each format, and not a field of `ivy.Config`.
Only a format whose file links to other files by path needs it. A program
that does not know the folder gives no option.
