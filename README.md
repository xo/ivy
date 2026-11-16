<div align="center">
  <!-- The logo of ivy comes later, at docs/logo/ivy.svg. -->
  <a href="#status" title="Status">Status</a> |
  <a href="#formats" title="Formats">Formats</a> |
  <a href="#documents" title="Documents">Documents</a> |
  <a href="#related-projects" title="Related Projects">Related Projects</a> |
  <a href="https://pkg.go.dev/github.com/xo/ivy" title="Go Reference">Reference</a> |
  <a href="https://github.com/xo/ivy/releases" title="Releases">Releases</a> |
  <a href="CONTRIBUTING.md" title="Contributing">Contributing</a> |
  <a href="#license" title="License">License</a>
</div>

<br/>

[![Unit Tests][ivy-ci-status]][ivy-ci]
[![Go Reference][goref-ivy-status]][goref-ivy]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

[ivy-ci]: https://github.com/xo/ivy/actions/workflows/test.yml "Test CI"
[ivy-ci-status]: https://github.com/xo/ivy/actions/workflows/test.yml/badge.svg "Test CI"
[goref-ivy]: https://pkg.go.dev/github.com/xo/ivy "Go Reference"
[goref-ivy-status]: https://pkg.go.dev/badge/github.com/xo/ivy.svg "Go Reference"
[release-status]: https://img.shields.io/github/v/release/xo/ivy?display_name=tag "Latest Release"
[releases]: https://github.com/xo/ivy/releases "Releases"
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"

# ivy

`ivy` decodes, renders and encodes the file formats that a program shows as
images, in pure Go:

1. Ebooks: EPUB, MOBI, AZW3, FictionBook, CHM, DjVu, comic book archives and
   the other formats that calibre reads.
2. Office documents: Word, Excel and PowerPoint, OpenDocument, RTF and CSV.
3. Page descriptions: PDF, XPS, PostScript, EPS, Adobe Illustrator, EMF and
   WMF.
4. Images with layers: Photoshop, OpenRaster and Krita.
5. Containers that carry an icon or a preview: Apple disk images,
   applications and packages for macOS, iOS, Android, Windows and Linux, and
   camera RAW files.

ivy also opens a file that a password protects, where the format has a public
method of encryption.

A program that imports ivy needs the Go toolchain and nothing else. ivy does
not use cgo and does not run an outside program. Each format is a package of
its own, so a program links only the formats that it imports.

ivy draws each page with [okage][okage], and a program finds the type of a
file with [magic][magic] before it calls ivy. A rendering is good enough for a
person to review the document. It is not an exact copy of the rendering of
the program that made the document.

[okage]: https://github.com/xo/okage
[magic]: https://github.com/xo/magic

## Status

ivy is a plan, and it holds no code yet. The plan, the design of the API,
the survey of the formats and the open questions are in the documents below.
The first phase gives the document API, the previews that the files carry,
and Photoshop.

## Formats

[`docs/FORMATS.md`](docs/FORMATS.md) lists each format that ivy can support,
the way to support it, its source and its cost.
[`docs/PLAN.md`](docs/PLAN.md) gives the order.

## Documents

| Document | What it holds |
| --- | --- |
| [docs/PLAN.md](docs/PLAN.md) | the plan, the architecture, the testing plan, the phases and the open questions |
| [docs/API.md](docs/API.md) | the design of the public API and the expected layout of the packages |
| [docs/FORMATS.md](docs/FORMATS.md) | the survey of the formats, with the estimates of three outside models |
| [docs/BACKLOG.md](docs/BACKLOG.md) | the work that is known and not done, and the work for okage and magic |
| [docs/decisions/](docs/decisions/README.md) | every decision, one file each, with an index |
| [CONTRIBUTING.md](CONTRIBUTING.md) | how to change ivy |
| [AGENTS.md](AGENTS.md) | the rules for a coding agent, which also hold for a person |

## Related projects

`ivy` is one of the `xo` projects. Several of them port a library of C or
Rust to pure Go. Each one is a separate repository:

| Project | What it is |
| --- | --- |
| [okage](https://github.com/xo/okage) | A Go port of tiny-skia and resvg, which draws 2D graphics and SVG images. ivy draws with it |
| [magic](https://github.com/xo/magic) | A Go port of libmagic, which identifies the type of a file |
| [transit](https://github.com/xo/transit) | A Go port of tree-sitter, a parser of source code |
| [rline](https://github.com/xo/rline) | A readline package for Go, which reads a line of text that a person edits. It started as a port of isocline |
| [zuzu](https://github.com/xo/zuzu) | Draws DOT graphs, Mermaid diagrams, and Vega and Vega-Lite charts |
| [ox](https://github.com/xo/ox) | Parses the flags and arguments of a command line |
| [usql](https://github.com/xo/usql) | A command line client for SQL and NoSQL databases |

[iv](https://github.com/kenshaw/iv), an image viewer for the terminal, is the
first program that uses ivy. ivy replaces the outside programs and the
libraries that iv uses through cgo for ebooks, XPS, Office documents, PDF and
HTML.

## License

The code of ivy is under the MIT license, in `LICENSE`. A package that ivy
copies or ports from another project keeps the license of that project, in a
`LICENSE` file in its folder.
