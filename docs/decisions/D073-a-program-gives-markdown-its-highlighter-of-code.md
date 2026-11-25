# D73. A program gives markdown its highlighter of code

Status: Decided.

D70 makes markdown-it in Chrome, with the colors of code of Shiki, the
reference of Markdown. `markdown` draws a block of code in one color. A
comparison with the reference is fair only if the code has colors in ivy
too.

The maintainers decided on 2026-11-25 that ivy holds no highlighter of
code. A highlighter holds a grammar or a lexer for each language, and a
program chooses one, such as `github.com/xo/transit`, which `iv` uses
today, or chroma. The binding of Blitz in `github.com/xo/blitz` takes a
highlighter as a function, and `iv` gives it one with transit.

## The decision

On 2026-11-25 the maintainers decided:

1. The package `markdown` gets this public API (D11):

   ```go
   // Highlighter returns the code of a fenced block in the language lang
   // as HTML: the text of the code, escaped, with a span element around
   // each run of text that has a style. ok is false for a language that
   // it does not color, and the block keeps one color.
   type Highlighter func(lang, code string) (html string, ok bool)

   // WithHighlighter colors the fenced code blocks of a document with h.
   func WithHighlighter(h Highlighter) Option
   ```

   The function has the signature of the highlighter of
   `github.com/xo/blitz`, so that a program can give the same function to
   both.
2. `markdown` gives h the first word of the info string of the block, as
   the language, and the text of the block. It keeps only the `span`
   elements of the result, and only these properties of their `style`
   attributes: `color`, `background-color`, `font-weight`, `font-style` and
   `text-decoration`. It keeps the text of any other element, and drops the
   element. A result whose text is not the text of the block is not used,
   and the block keeps one color.
3. `markdown` calls h only within the limits of `ivy.Config`. A block larger
   than the limits keeps one color. A panic of h is recovered, and the block
   keeps one color.
4. ivy holds no highlighter, and the module of ivy imports none. The test
   module gives the review a highlighter with `github.com/xo/transit` and the
   style "github" of `transit/styles`, as `iv` does, so that the review
   compares the colors of code with markdown-it and Shiki.
