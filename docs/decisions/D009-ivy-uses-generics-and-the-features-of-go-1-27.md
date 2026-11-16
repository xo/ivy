# D9. ivy uses generics and the features of Go 1.27

Status: Decided.

ivy uses generics and the newer features of Go where they make the code
simpler or faster. Go 1.27 is the oldest release that ivy builds with.

Examples of where they fit:

1. Generics: a buffer of pixels with 8, 16 or 32 bits for each channel, as
   PSD and TIFF hold. A reader of binary records that decodes a struct of
   any type. A cache of fonts or of images, keyed by any type.
2. Iterators of `iter`: the pages of a document, the entries of a container
   and the records of a metafile.
3. SIMD, behind `GOEXPERIMENT=simd`: the conversion of color spaces, such as
   CMYK and YCbCr to RGB. The planar channels of PSD into interleaved
   pixels. The inverse wavelet transforms of DjVu and JPEG 2000.

## SIMD

Go 1.27 has SIMD only behind `GOEXPERIMENT=simd`, so a build without the
experiment must still work.

1. Each SIMD path has a scalar path that gives the same result in every
   build. The scalar path is the reference.
2. The SIMD path and the scalar path agree within the tolerance of a review
   (D5). They do not have to give the same bytes, as okage D16 requires.
3. A test runs each such loop in both paths and compares them.
4. Only the files of the SIMD path carry a build tag for SIMD or for an
   architecture, as in okage D15.
