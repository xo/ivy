# D13. The test module holds the oracles, and runs by hand

Status: Decided.

The folder `test/` is a Go module of its own, as in okage D22. It is the only
place that can use cgo or run an outside program. It holds:

1. The oracles. These are the programs that iv runs today and other
   renderers: calibre, LibreOffice, libgxps, poppler, MuPDF, Ghostscript and
   DjVuLibre. Their licenses forbid a copy of their code in ivy (D7), but a
   test can run them and compare the result.
2. The comparisons. A page of ivy and a page of an oracle are compared
   within a tolerance that fits a review (D5).
3. The corpus. A script downloads the test files of the sources that ivy
   ports, such as the 1,600 PDF files of hayro. Git ignores the corpus.

The oracles are large to install. CI does not install them and does not run
the test module. CI tests the module that a user imports, with the Go
toolchain alone.

A maintainer runs the test module by hand, on a machine that has the
oracles, before a release and after a change to a renderer. A test of the
test module that cannot find its oracle fails, and it names the program that
is missing.
