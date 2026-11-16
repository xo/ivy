# Test files of psd

| File | Source |
| --- | --- |
| `rgba8.psd` | ImageMagick 7: `magick -size 64x48 gradient:red-blue -alpha set -channel A -evaluate set 75% +channel -depth 8 rgba8.psd` |
| `cmyk8.psd` | ImageMagick 7: `magick -size 64x48 gradient:red-blue -colorspace CMYK -depth 8 cmyk8.psd` |
| `deep16.psd` | ImageMagick 7: `magick -size 96x64 gradient:yellow-navy -depth 16 -compress RLE deep16.psd`. Upstream oov/psd fails on it |
| `rgb8bit.psd`, `psd_compat.psd`, `psd_nocompat.psd` | the test files of github.com/oov/psd at `a5d50ec0acac`, under the MIT license in `../internal/oovpsd/LICENSE` |
