# D42. A rewrite of the bytes of HTML is not a second reader of HTML

Status: Decided. Amends D7.

D7 says that ivy reads HTML with the parser of okage, and that a second
way to read HTML must not stay. `mobi` finds the filepos anchors of a book
in the bytes of its HTML, and `chm` finds the meta element of the charset.
Both change the text at byte positions, which the tree of a parser cannot
keep. Question 18 of `PLAN.md` asked whether D7 covers them.

On 2026-11-18 the maintainers decided:

1. A rewrite that finds a few marks in the bytes of HTML, and changes the
   text at their positions, without building a tree, is not a reader of
   HTML. D7 allows it.
2. Code that reads the structure of HTML, such as the elements, their
   order or their text, uses the parser of okage, as D7 says.
