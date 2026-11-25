# D68. Chinese and Japanese text keeps its lines

Status: Decided. Amends D34.

D34 makes `text` join the lines of a paragraph of a text file, and start a
new paragraph at a line that starts with white space. On 2026-11-25 a
maintainer found two faults in the review of `gutenberg/txt/pg24264.txt`
and `gutenberg/epub/pg24264-images-3.epub`, a Chinese book of Project
Gutenberg.

## The facts

1. The text file wraps each line at about 35 characters. Each paragraph
   starts with two ideographic spaces, U+3000. calibre keeps each line of
   the file, so its pages show the lines of the file. `text` joins them, so
   a line of ivy fills the width of the page, as wide as 90 characters on
   A4 in landscape.
2. The EPUB is made from the same text. Each file of the book holds one
   `p` element of about 250 KB. The text keeps the line feeds of the text
   file, and each paragraph starts after a line feed with two ideographic
   spaces. CSS turns each line feed into a space, so the paragraphs run
   together, and each pair of ideographic spaces is a gap inside a line.
   calibre and a browser show the book in the same way.

## The decision

On 2026-11-25 the maintainers decided:

1. `text` keeps each line of a text file in Chinese or Japanese, as calibre
   does. A file is in Chinese or Japanese when more than half of its lines
   that are not blank are mostly made of the letters and the punctuation of
   Chinese or Japanese. A line of such a file that is longer than the page
   still wraps. The other text files keep the rule of D34 and of D67.
2. The layout starts a new line at a line feed that one or more ideographic
   spaces follow, in text whose `white-space` is `normal`. The spaces and
   the tabs before the ideographic spaces go, as around any line feed. The
   paragraphs of a book such as the EPUB above then start on lines of their
   own, with their indent. This is not a rule of CSS, and calibre does not
   do it. It applies to every format that the layout puts on pages.
3. The layout removes a line feed whose characters before and after are
   characters of Chinese or Japanese, with the spaces and the tabs around
   it, as CSS Text 3 says in its rules for the line feeds of the source.
   WeasyPrint turns such a line feed into a space, so the EPUB above showed
   a small gap at each line feed of its source. A browser such as Firefox
   removes it too. Korean keeps the space, because it separates its words
   with spaces.

Point 3 was not one of the choices of the question. It came from the same
review, and it follows CSS.
