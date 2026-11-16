# D6. Each decoder and each encoder is a package of its own

Status: Decided.

A program links only the formats that it imports. iv imports decoders and no
encoders, and its binary must not grow for an encoder or for a format that it
does not use.

## Packages

1. Each decoder is a package of its own. Each encoder is a package of its
   own, apart from the decoder of the same format.
2. The decoder of a format is the package `<format>` in the folder
   `<format>/`, such as `github.com/xo/ivy/pdf`. Its encoder is the package
   `<format>enc` in the folder `<format>/<format>enc/`, such as
   `github.com/xo/ivy/pdf/pdfenc`. Each package has a name of its own, so a
   program that imports both needs no rename. An import of `pdf/pdfenc` does
   not link `pdf`, unless `pdfenc` imports it.
3. The root package `ivy` never imports the package of a format. It holds
   only what every format shares: the document API, the configuration and
   the errors (D11).
4. A shared layer, such as the display list or the layout (D10), is a
   package of its own under `internal/`. A format imports it only if it uses
   it.
5. No package registers every format. A program that wants every format
   imports each package. A package that only imports the others, as
   `decoder/all` of iv does, can come later, and only a program that asks for
   it links it.

## Modules

Go links only the packages that a program imports, so one module keeps a
binary small. A program still downloads each module that the `go.mod` of ivy
requires.

1. ivy is the module `github.com/xo/ivy`.
2. If a package needs a heavy outside dependency, such as a decoder of RAR or
   of 7z, the package is a module of its own, with its own `go.mod`, as
   `magic/compress/xz` is in magic. The maintainers decide which dependency
   is heavy, when they approve it (D7).
3. The DRM that needs a key is a module of its own (D12).
4. The test module in `test/` is a module of its own (D13).
