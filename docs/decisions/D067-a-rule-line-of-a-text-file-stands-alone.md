# D67. A rule line of a text file stands alone

Status: Decided. Amends D34.

D34 makes `text` join the lines of a paragraph of a text file, as calibre
does, because a program such as Project Gutenberg wraps each line. A blank
line ends a paragraph, and a line that starts with white space keeps the
break before it. A maintainer found on 2026-11-21 that the Chinese book
`gutenberg/txt/pg24264.txt` runs a heading, a line of dashes and the first
line of a chapter together, as "…閨秀-----…-----此開卷第一回也．作者自云：…".
Question 29 of `PLAN.md` asked the maintainers to choose.

## The facts

These were measured on 2026-11-21 on the two text files of the corpus of
the review. A width is in the columns of a terminal, where a wide character
of Chinese or Japanese takes two columns.

1. In `pg24264.txt`, each chapter starts with a heading, such as
   `第一回　甄士隱夢幻識通靈　賈雨村風塵怀閨秀`, a line of 66 dashes, and the
   first line of the text, with no blank line between them. The file holds
   122 lines that repeat one character of punctuation, and 43 of them touch
   a line of text. `text` joins each of them with the lines around it.
2. `pg11.txt`, a book in English, holds no such line.
3. calibre and other programs that unwrap text also end a paragraph at a
   line that is much shorter than the width of the wrap. In `pg24264.txt`
   the width of the wrap is not the same from line to line. Many lines in
   the middle of a paragraph are 50 to 52 columns wide, and others are 66
   to 70. A rule that ends a paragraph at a line shorter than 75 percent of
   the usual width ends 3,977 lines of this book, and in a sample nearly
   all of them are in the middle of a sentence. The heading of the example
   is 42 columns wide, too close to those lines to tell apart.
4. A line that starts with an ideographic space already starts a new line,
   as D34 says. So the paragraphs of `pg24264.txt` keep their starts.

## The choices

1. A rule line stands alone. A line that holds only one character of
   punctuation repeated four times or more, such as `----`, `====`, `****`,
   `____`, `~~~~` or `────`, with white space around it, keeps the break
   before it and after it. It ends the paragraph before it and starts a new
   one after it, and `text` writes the line as it is in the file, on a line
   of its own. This is the recommendation.
2. Choice 1, and the line becomes a rule of HTML, `<hr>`, in place of its
   characters. This draws a line across the page, but it loses the look of
   the file, such as a line of `*` between scenes.
3. Choice 1, and a short line also ends a paragraph, as fact 3 describes.
   The measurement shows that this breaks thousands of lines in the middle
   of a sentence in `pg24264.txt`, so it is not recommended.
4. No change. D34 stays as it is.

## The cost

Choice 1 is about 30 lines of Go in `toHTML` of `text/text.go`, with a
test of a heading, a rule line and a paragraph without blank lines, a
test of a rule line between two paragraphs, and a test that a line of
mixed punctuation, such as `-=-=`, does not count. The pages of
`pg24264.txt` change, and `pg11.txt` does not.

## The decision

On 2026-11-21 the maintainers answered question 29 with choice 1. A rule
line keeps the break before it and after it, and `text` writes it as it is
in the file, as a paragraph of its own. Only a text file whose lines `text`
joins does this. A TXTZ file keeps each break of a line already. A short
line does not end a paragraph.
