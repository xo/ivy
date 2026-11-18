# D35. ivy.Info has the publisher and the identifier

Status: Decided.

`internal/ebook` reads the publisher and the identifier of a book, and
`ivy.Info` had no field for them. Question 8 of `PLAN.md` asked whether the
API gets them (D11).

On 2026-11-18 the maintainers decided:

1. `ivy.Info` has two fields more:

   ```go
   // Publisher is the publisher of the document, such as the publisher of
   // a book.
   Publisher string
   // Identifier is an identifier of the document that its file gives, such
   // as an ISBN, a UUID or an ASIN, as the file writes it.
   Identifier string
   ```
2. EPUB, MOBI and AZW3 fill them, and a comic fills `Publisher` from its
   ComicInfo.xml. Another format fills them when its file has them.
3. The series and the number of a comic get no field, until a program asks
   for them.
