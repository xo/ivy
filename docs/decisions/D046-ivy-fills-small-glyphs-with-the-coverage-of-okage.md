# D46. ivy fills small glyphs with the coverage of okage

Status: Decided. Amends D28.

Point 3 of D28 fills a glyph of at most 48 pixels to the em at twice its
size into a mask. Then it averages each block of 2 by 2 pixels, so that the
coverage has a grid of 8 by 8 samples in each pixel. okage v0.11.0 gives
`Paint.Coverage` (okage D54). With `okage.Coverage8x8`, a fill of okage
takes 8 by 8 samples in each pixel, the same grid as the fill of D28. The
maintainers asked on 2026-11-18 to use it in place of the fill of D28.

A measurement on 2026-11-18 filled a run of 800 glyphs of Go Regular, at 13
pixels to the em, on a pixmap of 660 by 180 pixels:

1. The fill of D28 took 19.9 ms. The fill with `Coverage8x8` took 13.9 ms.
   The fill with the default grid of 4 by 4 samples took 12.1 ms.
2. The mean difference of the alpha of a pixel from the fill of D28 was 0.44
   of 255 with `Coverage8x8`, and 2.11 with the grid of 4 by 4 samples.
3. No golden file of the tests changed.

The maintainers decided:

1. `FillGlyphs` and `FillText` of `internal/display` fill a glyph of at most
   48 pixels to the em with `Paint.Coverage` set to `okage.Coverage8x8`.
   This replaces the fill at twice the size of point 3 of D28. A larger
   glyph fills as a path, with the grid of 4 by 4 samples, as before.
2. The finer coverage applies to every shader, and to every blend mode and
   clip of the device. The fill of D28 applied only to a solid color.
   Without antialiasing, a glyph fills as a path.
3. ivy has no limit of its own on the size of such a fill. okage takes the
   grid of 4 by 4 samples for a fill wider or higher than 4095 pixels (okage
   D54).
