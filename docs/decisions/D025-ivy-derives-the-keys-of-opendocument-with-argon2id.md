# D25. ivy derives the keys of OpenDocument with Argon2id

Status: Decided.

LibreOffice 26.8 writes ODF 1.4 by default. It encrypts the whole package
with AES-256-GCM, with a key that Argon2id (RFC 9106) derives from the
SHA-256 of the password. D12 lists Argon2id for OpenDocument. D24 approved
`golang.org/x/crypto` for Blowfish only, and the standard library of Go has
no Argon2. So `odf` could not open such a file. This was question 5 of
`docs/PLAN.md`.

On 2026-11-17 the maintainers decided that ivy uses
`golang.org/x/crypto/argon2`, from the same module and under the same
license as Blowfish (D7):

1. `odf` reads the key derivation Argon2id by its name in ODF 1.4 and by the
   experimental name that LibreOffice writes. It reads the passes, the
   memory and the lanes from the attributes of the manifest, in the
   namespace of the manifest or of LibreOffice.
2. A file sets the memory of Argon2id, so `ivy.Limits.MaxMemory` bounds it,
   and a file with more than 64 passes returns an error that wraps
   `ivy.ErrLimit`. LibreOffice writes 64 MiB and 3 passes.
3. The data of an entry that AES-GCM encrypts starts with its IV, as XML
   Encryption 1.1 says and as LibreOffice writes. The tag of 16 bytes ends
   it.
