# D53. A metafile draws glyph indexes through their text

Status: Decided.

A record of EMF or EMF+ can name its glyphs by index, as ETO_GLYPH_INDEX
does. The indexes belong to the font of Windows that the metafile names,
such as Arial or Calibri. ivy draws such text with a font that has the same
metrics, such as Liberation Sans or Carlito, and that font orders its
glyphs in another way. So the indexes draw the wrong glyphs. The
maintainers asked on 2026-11-18 how ivy handles this text.

The maintainers decided:

1. When a record also gives the text of its glyphs, such as the string of
   EMF+ DrawDriverString, or text in a code page that ivy knows, ivy draws
   the text with its own font and does not use the indexes.
2. When a record gives only the indexes, ivy draws them in its own font, as
   before.
3. ivy holds no tables of the glyphs of the fonts of Windows.
