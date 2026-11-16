# D23. A program can ask for the fonts of the system

Status: Decided.

The layout draws an ebook with the fonts that the ebook embeds, and with
the Go fonts of `golang.org/x/image` for the other text. okage bundles no
font. ivy did not read the fonts of the system, so a page looked the same
on each machine. A program could not ask for the fonts of the system, for
an ebook that names a font that the system has. This was question 4 of
`docs/PLAN.md`.

On 2026-11-17 the maintainers decided that a program can ask for them:

1. `ivy.Config` has the field `SystemFonts bool`. It is false by default.
2. When it is true, a format that lays out text adds the fonts of the
   system after the fonts that the document embeds and the Go fonts. The
   layout reads the fonts of the system once, with `LoadOSFonts` of okage,
   and the books share them.
3. When it is false, ivy reads no font of the system. A page then looks the
   same on each machine, and the tests do not depend on the machine.

The option is a field of `ivy.Config`, because each format that lays out
text uses it the same way. The ebooks use it now, and the Office formats of
phase 5 will use it too.
