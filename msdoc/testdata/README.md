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
