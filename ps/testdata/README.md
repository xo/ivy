# Test files of ps

| File | Source |
| --- | --- |
| `plain.eps` | ImageMagick 7: a gradient of 16 by 8 from red to blue, `magick grad.png eps2:plain.eps`. It holds no preview |
| `tiff4-preview.eps` | ImageMagick 7: the same gradient, `magick grad.png ept:tiff4-preview.eps`. Its TIFF preview has a palette of 4 bits |
| `tiff8-preview.eps` | `plain.eps` with a DOS EPS header and a TIFF preview of 8 bits that ImageMagick wrote: `magick grad.png -depth 8 -type TrueColor -compress none p8.tif` |
| `half.epsi` | ps2epsi of Ghostscript, from an EPS of 32 by 16 whose left half is black |
| `shapes.ps` | Written by hand for the tests of the pages: fills, strokes with caps, joins and dashes, arcs, curves, a clip, colors in DeviceRGB, DeviceCMYK, HSB, Separation and Indexed, an image, an axial shading and a tiling pattern |
| `text.ps` | Written by hand for the tests of the pages: the standard fonts, a font with ISOLatin1Encoding, ashow, widthshow, charpath, stringwidth, rotated text and a Type 3 font |
