# D70. markdown-it in Chrome is the reference of Markdown

Status: Decided. Amends D69.

D69 gives Markdown the reference of calibre, as for the ebooks. calibre
draws a Markdown file as a book: no rules under the headings, no borders of
cells, no background behind code and no colors of code. A person reads a
Markdown file on GitHub, or in a preview such as the one of VS Code, which
draw all of them. On 2026-11-25 the review of a README of GitHub showed the
difference.

The maintainers looked at two references. Blitz, through `github.com/xo/
blitz`, draws Markdown with a style sheet of GitHub, but it has no
fragmentation: it draws a document as one tall picture, and its PDF cuts
that picture anywhere, even through a line of text. markdown-it, a parser of
Markdown in JavaScript, draws the HTML that the preview of VS Code draws.
With the style sheet `github-markdown-css` it looks like GitHub. Chrome
prints that HTML to PDF on pages, and breaks them between lines, as ivy
does.

## The decision

On 2026-11-25 the maintainers decided:

1. The reference of Markdown is markdown-it in Chrome, through `chromedp`
   (D69). The page holds markdown-it with the plugins for the alerts of
   GitHub (`> [!NOTE]` and the others), task lists, footnotes and emoji,
   Shiki for the colors of code, KaTeX for math, and the style sheet
   `github-markdown-css`. The review fetches these files once into its
   cache, as it fetches the corpus, and runs them without the network. They
   are in the test module only (D13).
2. Chrome prints the page to PDF with an `@page` rule of the paper and the
   margins of ivy, and MuPDF draws each page, as for the other references of
   PDF (D69). Page n of ivy is compared with page n of Chrome.
3. calibre stays a reference of Markdown under `-refs all` of D69. Blitz is
   not used.
4. `markdown` draws a document as GitHub does, within its pages: its style
   sheet follows `github-markdown-css` for the headings, the code, the
   tables, the quotes, the alerts, the lists and the rules. A program can
   give it a highlighter of code, which the review gives with transit. This
   answers the question of the build of 2026-11-25 on which reference wins
   for the Markdown parts of a README: GitHub.
