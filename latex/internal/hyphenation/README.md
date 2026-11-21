# hyphenation

This folder is a copy of
[github.com/go-typeset/hyphenation](https://github.com/go-typeset/hyphenation)
at the commit `d9d3c0ca2e61e69ca9fa7252ec732164d6b6e436`, the tag `v1.0.1`,
under the BSD-3-Clause license in `LICENSE` (D7, D62). The rules for a copy
are in [`docs/UPSTREAM.md`](../../../docs/UPSTREAM.md).

It is the algorithm of Liang, which finds where a hyphen can fall in a word.
The engine of TeX in `latex/internal/engine` calls it, through `libs.go` of
the engine, with the patterns that the engine loads.

## The files

The folder holds each Go file of the root of upstream, with its tests, with
the same names. The copy leaves out `README.md`, `renovate.json` and
`c.out`. Upstream holds no file of patterns.

## The changes

Each file opens with a line that names the file and the commit. The copy
has no other change.
