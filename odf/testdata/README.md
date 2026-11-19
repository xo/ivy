# Test files

LibreOffice 26.8 wrote these files with `soffice --headless --convert-to`,
from a short text, a CSV file of two rows, or a flat ODP file of one slide.
LibreOffice writes a thumbnail into an OpenDocument file, and none into an
Office Open XML file.

LibreOffice 26.8 wrote the encrypted files with the password `secret`, from
a short text, through its API:

- `encrypted_blowfish.odt`: the version ODF 1.1, with Blowfish and SHA-1.
- `encrypted_aes.odt`: the version ODF 1.2, with AES-256 and SHA-256.
- `encrypted_argon2id.odt`: the default version, ODF 1.4, which encrypts the
  whole package with AES-GCM and Argon2id.

LibreOffice writes no thumbnail into an encrypted file. The tests encrypt a
thumbnail with the fixture of `internal/fixture`.

LibreOffice 26.8 wrote the files `float-f-late.odt` and `float-f-bottom.odt`
from the files of the same names of `docx/testdata`, with `soffice --headless
--convert-to odt`. They hold a frame with `style:vertical-rel` page-content
and wrap, 1 inch below the top of the margin or at its bottom (D60).
