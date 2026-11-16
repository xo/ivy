# Test files of offcrypto

Each file is one stream of a Compound File, byte for byte. The tests read
the streams, because the reader of Compound Files comes from magic later in
phase 5. A script with the Python package olefile took the streams out of
the files.

These streams come from the test files of nolze/msoffcrypto-tool, under the
MIT license, at the commit `bdb71ea5dfbdee2f3cd86f8852e74be7a6f15667`. The
license is in `LICENSE-msoffcrypto-tool.txt`. The password of each file is
`Password1234_`.

| Stream | Source file in `tests/inputs` | Stream of the source |
| --- | --- | --- |
| `agile_sha512.info`, `agile_sha512.package` | `example_password.xlsx` | `EncryptionInfo`, `EncryptedPackage` |
| `agile_sha1.info`, `agile_sha1.package` | `ecma376agile_sha1_password.xlsx` | `EncryptionInfo`, `EncryptedPackage` |
| `standard.info`, `standard.package` | `ecma376standard_password.docx` | `EncryptionInfo`, `EncryptedPackage` |
| `rc4cryptoapi_doc.table` | `rc4cryptoapi_password.doc` | `1Table` |
| `rc4cryptoapi_xls.workbook` | `rc4cryptoapi_password.xls` | `Workbook` |

The tests compare the result with the SHA-256 of the files that
msoffcrypto-tool keeps in `tests/outputs`.

LibreOffice 26.8 wrote the other files, from a short text or a sheet of one
cell, with the password `secret`. It encrypts a .docx file with Standard
encryption, and a .doc or .xls file with RC4.

| Stream | Source file | Stream of the source |
| --- | --- | --- |
| `libreoffice_standard.info`, `libreoffice_standard.package` | a .docx file | `EncryptionInfo`, `EncryptedPackage` |
| `libreoffice_rc4_doc.table` | a .doc file | `1Table` |
| `libreoffice_rc4_xls.workbook` | an .xls file | `Workbook` |
