# D29. ivy interprets PostScript

Status: Decided.

The package `ps` gives only the previews that an EPS file carries. A
PostScript file, and an EPS file without a preview, gives no page. The plan
kept the interpreter of PostScript in the long tail, until a user asks for
it (D1). On 2026-11-17 the visual review showed seven such files in the
corpus, and the maintainers asked for the interpreter.

The maintainers decided:

1. `ps` interprets PostScript Level 2, with the parts of Level 3 that real
   files use, such as the shadings, and draws each page onto the display
   list (D10), as `pdf` does. An EPS file gives its page from its
   BoundingBox, and its preview stays a preview.
2. The interpreter is in `ps/internal/`. It reuses what ivy holds: the fonts
   (Type 1, CFF, TrueType and Type 3), the filters and the image decoders,
   the color spaces and the shadings of the port of hayro. A part that only
   `pdf/internal/` holds and that `ps` needs moves to `internal/`, so that
   both packages import it.
3. [llgcode/ps](https://github.com/llgcode/ps) (BSD-2-Clause) and
   [seehuhn/go-postscript](https://github.com/seehuhn/go-postscript)
   (BSD-3-Clause) can be read, and copied or ported in part, with their
   licenses (D7). Ghostscript is under the AGPL, so it runs only as an
   oracle in the test module (D13), and no code of it is read.
4. A file must never crash ivy or make it hang (D5). The interpreter bounds
   its operand, dictionary and execution stacks, its memory, the depth of
   its procedures and its loops, and the number of operators that it runs,
   from `ivy.Config`, and it checks the context.
5. A file that starts with `%!` is PostScript, as the language says. `ps`
   does not need `%!PS`.
