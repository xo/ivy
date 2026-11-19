# Test files of msdoc

`rc4cryptoapi_password.doc` comes from the folder `tests/inputs` of
nolze/msoffcrypto-tool, under the MIT license, at the commit
`bdb71ea5dfbdee2f3cd86f8852e74be7a6f15667`. The license is in
`LICENSE-msoffcrypto-tool.txt`. RC4 CryptoAPI encrypts the file, with the
password `Password1234_`.

LibreOffice 26.8 wrote the other files through its API. The document holds a
heading, a paragraph with bold and italic text, a large centered paragraph, a
table of two rows and two columns, and a last paragraph. Its title is "Ivy
test document", and its author is "Ivy Tester".

| File | What it holds |
| --- | --- |
| `text.doc` | the document in the clear |
| `rc4_password.doc` | the same document, which RC4 encrypts with the password `secret` |

LibreOffice 26.8 wrote the files `float-*.doc` from the files `float-*.docx`
of `docx/testdata`, with `soffice --headless --convert-to "doc:MS Word 97"`.
They hold the page floats of D60. LibreOffice writes each table with
sprmTPc and sprmTDyaAbs, and each frame and the picture as a shape, with its
place from the margin. `docx/testdata/README.md` describes the floats.

| File | The float |
| --- | --- |
| `float-t-late.doc` | a table 1 inch below the top of the margin, anchored before P12 |
| `float-t-page2.doc` | the same table, anchored before P26, on page 2 |
| `float-f-late.doc` | a frame 1 inch below the top of the margin, anchored after P11 |
| `float-s-late.doc` | a picture 1 inch below the top of the margin, anchored in P12 |
| `float-f-bottom.doc` | the frame at the bottom of the margin, anchored after P01 |
| `float-t-bottom.doc` | the table at the bottom of the margin, anchored before P02 |
| `float-t-two.doc` | two tables at the same place |
| `float-t-loop3.doc` | a table of 12 rows at the top of the margin, anchored before P10 |
| `float-t-loop2.doc` | the same table, anchored before P12 |
| `float-t-nofit.doc` | a table of 6 rows 9.5 inches below the top of the page, anchored before P06 |
