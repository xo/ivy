# D17. ivy ports all of hayro-ccitt, and decodes JPEG with Go

Status: Decided. Amends D16.

D16 replaced the crate `hayro-ccitt` with `golang.org/x/image/ccitt`, and
the JPEG decoder of hayro with `image/jpeg`. The ports of wave 1 found where
each replacement falls short. The maintainers decided on 2026-11-16:

1. `golang.org/x/image/ccitt` cannot decode Group 3 2D streams, nor the rows
   of Group 3 without EOL codes, and PDF files hold both. It cannot report how
   many bytes it read, and the halftone regions of JBIG2 need that number. So
   ivy ports all of `hayro-ccitt`, about 1,100 lines, into `internal/ccitt`.
   The filter CCITTFaxDecode of `pdf/internal/syntax` and the MMR coding of
   `internal/jbig2` use it, and JBIG2 holds no copy of its own.
2. `image/jpeg` fails on a truncated JPEG, on a JPEG of 12 bits, and on one
   with arithmetic coding, where the decoder of hayro decodes them. ivy keeps
   `image/jpeg` for now. An image that fails leaves its page as an
   `*ivy.PartialError` (D5). A lenient decoder of JPEG is later work in the
   backlog.
