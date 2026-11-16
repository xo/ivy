# D12. Encrypted files take a password, and DRM is decoded where it is possible

Status: Decided.

## Passwords

ivy opens a file that a password protects, when the format uses a public
method of encryption:

1. PDF: RC4, and AES with keys of 128 and 256 bits.
2. Office Open XML: the Agile and the Standard encryption of MS-OFFCRYPTO.
3. The binary Office formats: RC4, RC4 CryptoAPI, and the XOR obfuscation of
   Excel.
4. OpenDocument: AES with PBKDF2 or Argon2id, and the older Blowfish.
5. Zip: ZipCrypto and the AES of WinZip, for a comic book archive or another
   container.

A program gives the password with an option. A program that asks a person for
the password gives a function instead, which ivy calls once for each attempt.
If the password is missing or wrong, ivy returns `ErrPassword`, so that the
program can ask again.

## DRM

DRM is a restriction on copying that a seller or an author adds to a file.
ivy decodes DRM where it is possible.

1. ivy undoes the obfuscation of fonts in EPUB, by the IDPF method and by
   the Adobe method, and the obfuscated fonts of XPS. A renderer must undo it
   to draw the text.
2. ivy renders a PDF file that has an owner password and no user password,
   and it ignores the flags that forbid printing or copying.
3. The schemes that need a key that the person holds, such as Adobe ADEPT,
   the DRM of Kindle and KFX, and Readium LCP, go in a module of their own
   (D6). A program that does not import it does not download it or link it.
   iv does not import it by default.
4. A program gives the key with an option. ivy never searches for a key by
   itself, for example a key of a device on the disk or a key in the files of
   another program.
5. The decoders of the formats do not import the module of DRM. A program
   connects the two, with an option that takes a function that decrypts.

If a file holds DRM that ivy does not decode, ivy returns `ErrDRM`, and it
can still return the cover or a preview where the file holds one in the
clear.

In the United States, section 1201 of the DMCA forbids the distribution of a
tool that circumvents DRM. Article 6 of the EU Copyright Directive does the
same. Both can apply when the person who runs the tool owns the file. The
maintainers chose items 3 to 5 with that risk known.
