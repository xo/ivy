# D32. A program chooses the paper of a document without one

Status: Decided. Amended by D33.

A file of Word, Excel or OpenDocument gives the size and the orientation of
its pages, and ivy uses them. A text file, a Markdown file, a CSV file, an
HTML file and an ebook give no paper. ivy puts them on pages of US Letter in
portrait, and a program cannot change that. A reader outside North America
expects A4, and a wide table reads better on a page in landscape. The visual
review also compares these formats on US Letter only, so a fault of another
paper or of landscape stays hidden.

On 2026-11-17 the maintainers decided:

1. The package `ivy` has the type `Paper`, the size of a page in points,
   and the field `Paper` of `Config`:

   ```go
   // Paper is the size of a page in points, 1/72 inch. The zero Paper is the
   // default of the format.
   type Paper struct {
   	Width, Height float32
   }

   // The common papers, in portrait.
   var (
   	Letter = Paper{Width: 612, Height: 792}
   	Legal  = Paper{Width: 612, Height: 1008}
   	A4     = Paper{Width: 595.28, Height: 841.89}
   	A5     = Paper{Width: 419.53, Height: 595.28}
   )

   // Landscape returns p with its longer side across.
   func (p Paper) Landscape() Paper

   // Portrait returns p with its longer side down.
   func (p Paper) Portrait() Paper
   ```

   ```go
   type Config struct {
   	// ...
   	// Paper is the page of a document whose file gives none, such as a
   	// text file, an HTML file or an ebook. The zero Paper keeps the default
   	// of the format, US Letter in portrait.
   	Paper Paper
   }
   ```

2. A format uses `Config.Paper` only for a document whose file gives no
   page size. A file that gives its paper keeps it, in its orientation.
   A file whose specification gives a default paper when the file says
   nothing, such as RTF, Word and Excel, takes `Config.Paper` in place of
   that default. The format keeps its own margins on the new paper.
3. The visual review renders each document of the formats without a paper
   on US Letter in portrait, on A4 in portrait and on A4 in landscape, and
   gives the same paper to the reference program.
4. The corpus of the review holds files of Office and OpenDocument on A4,
   on US Legal and in landscape, and the review marks a page whose size
   differs from the size that LibreOffice gives.
