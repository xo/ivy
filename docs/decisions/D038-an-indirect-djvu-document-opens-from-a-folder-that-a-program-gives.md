# D38. An indirect DjVu document opens from a folder that a program gives

Status: Decided. Amends D26.

An indirect DjVu document keeps its pages and its shared parts in files of
their own, which its index names. D26 left it as `ErrUnsupported`. D22 lets
a program give the folder of a Markdown or HTML file. Question 12 of
`PLAN.md` asked whether DjVu does the same.

On 2026-11-18 the maintainers decided:

1. `djvu.WithDir(dir fs.FS) Option` gives the folder of an indirect
   document. The package reads each file that the index names from it,
   within the limits of the configuration.
2. Without the option, an indirect document gives `ErrUnsupported`, as
   before.
