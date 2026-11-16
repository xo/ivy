# D16. The port of hayro uses Go, and ports a subset of moxcms

Status: Decided. Amends D14. Amended by D17. Amended by D20.

hayro and moxcms are about 111,000 lines of Rust. The maintainers cut the port
of D14 on 2026-11-16, so that it ports only the code that ivy has no other way
to get.

1. The code of hayro that the Go standard library or `golang.org/x/image`
   already holds is replaced, not ported: MD5, SHA-256, SHA-384, SHA-512, AES
   and RC4 of `hayro-syntax`, its inflate and its JPEG decoder, its LZW with
   early change, which `golang.org/x/image/tiff/lzw` decodes, and the crate
   `hayro-ccitt`, which `golang.org/x/image/ccitt` replaces.
2. `hayro-font` reads TrueType and CFF with skrifa. ivy reads them with
   go-text (D7), and ports only Type 1, Type 3, the encodings, the names of
   the glyphs, and the metrics of the 14 standard fonts.
3. ivy ports the subset of moxcms that hayro uses: the reading of an ICC
   profile, the transforms of 8 bits to sRGB, and Lab. The subset is in
   `internal/moxcms`, so that other formats can use it.
4. The codecs `hayro-jbig2` and `hayro-jpeg2000` are ported, in
   `internal/jbig2` and `internal/jpeg2000`. The packages `jbig2` and `jpx` of
   dejapdf only encode, so they cannot replace them.
5. hayro uses the geometry of kurbo. ivy uses the port of kurbo in okage,
   which okage exports for it (D7).

The packages of the port, and the crate that each one ports:

| Package | Crate |
| --- | --- |
| `pdf/internal/syntax` | `hayro-syntax` |
| `pdf/internal/postscript` | `hayro-postscript` |
| `pdf/internal/cmap` | `hayro-cmap` |
| `pdf/internal/pdffont` | `hayro-font` |
| `pdf/internal/interpret` | `hayro-interpret` |
| `internal/jbig2` | `hayro-jbig2` |
| `internal/jpeg2000` | `hayro-jpeg2000` |
| `internal/moxcms` | the subset of moxcms |

The ports run in parallel, in two waves: first the crates that need no other
crate of the port, then `hayro-font` and `hayro-interpret`. The crate `hayro`
itself is not ported, because the device of the display list takes its place
(D14).
