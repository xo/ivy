# D34. A text file joins the lines of a paragraph

Status: Decided. Amended by D67. Amended by D68.

calibre joins the lines of a paragraph of a text file that a program
wrapped, such as a book of Project Gutenberg. ivy kept each line. Question 7
of `PLAN.md` asked whether ivy reflows them.

On 2026-11-18 the maintainers decided:

1. `text` joins the lines between two empty lines into one paragraph, which
   the layout wraps to the page.
2. A line that starts with a space or a tab keeps its break, and so do the
   lines of a block of such lines, such as code or a poem with an indent.
3. A TXTZ file keeps the HTML of its own text.
