# linebreak

This folder is a copy of
[github.com/go-typeset/linebreak](https://github.com/go-typeset/linebreak)
at the commit `48917899e7b189c44cac60ab22e30bd3d9c6eb7a`, the tag `v1.12.0`,
under the BSD-3-Clause license in `LICENSE` (D7, D62). The rules for a copy
are in [`docs/UPSTREAM.md`](../../../docs/UPSTREAM.md).

It is the line breaking of Knuth and Plass that the engine of TeX in
`latex/internal/engine` calls, through `libs.go` of the engine.

## The files

The folder holds each Go file of the root of upstream, with its tests, with
the same names. The copy leaves out `README.md`, `renovate.json` and
`c.out`.

## The changes

Each file opens with a line that names the file and the commit. The copy
has no other change.
