# Test files of msxls

`plain.xls` and `rc4cryptoapi_password.xls` come from the folder
`tests/inputs` of nolze/msoffcrypto-tool, under the MIT license, at the
commit `bdb71ea5dfbdee2f3cd86f8852e74be7a6f15667`. The license is in
`LICENSE-msoffcrypto-tool.txt`. Excel for the Mac wrote them, with one sheet
of two cells. RC4 CryptoAPI encrypts `rc4cryptoapi_password.xls`, with the
password `Password1234_`.

LibreOffice 26.8 wrote the other files through its API. The workbook holds
two sheets. The first sheet holds text, numbers with number formats, a
date, a formula of a number, a formula of text, a formula of a boolean and
two merged cells. Its title is "Ivy test sheet", and its author is "Ivy
Tester".

| File | What it holds |
| --- | --- |
| `sheet.xls` | the workbook in the clear |
| `rc4_password.xls` | the same workbook, which RC4 encrypts with the password `secret` |
