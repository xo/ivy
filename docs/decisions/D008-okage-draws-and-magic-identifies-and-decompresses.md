# D8. okage draws, and magic identifies and decompresses

Status: Decided.

ivy decodes files. Two other `xo` repositories do work that ivy does not
repeat.

1. okage draws. Every format that ivy renders draws with okage: paths,
   gradients, images, masks, blend modes and text. If ivy needs a feature
   that okage does not have, the work goes to okage. An example is a shading
   that fills a mesh of triangles. ivy does not hold a second renderer.
2. magic identifies. A program finds the type of a file with magic, and then
   opens it with the package of ivy for that type. A package of ivy that must
   find the type of a file inside a container, such as a page of a comic book
   archive, also asks magic. If magic cannot tell two formats apart, such as
   an XPS file and an ordinary zip file, the work goes to magic. ivy does not
   hold a second set of rules for the types of files.
3. magic decompresses. These decoders go to `magic/compress`, and ivy
   imports them from there:

| Decoder | ivy needs it for | Other users |
| --- | --- | --- |
| LZX | CHM and Microsoft Reader | CAB and WIM |
| LZFSE | DMG | other formats of Apple |
| ADC | older DMG files | |
| PalmDoc | MOBI, PRC and PDB | other PalmDB files |
| HUFF/CDIC | MOBI and AZW | |

Until magic has a decoder, the format that needs it waits. The work that ivy
needs from each repository is in [`docs/BACKLOG.md`](../BACKLOG.md). The
section "Peer sessions" of `AGENTS.md` says how to ask for it.
