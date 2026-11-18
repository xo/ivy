# D39. pdf refuses a file larger than the memory limit

Status: Decided.

`pdf.Open` reads the whole file into memory, up to its size, and did not
check it against `Limits.MaxMemory`. A reader that pages through the
`io.ReaderAt` of the source needs a rewrite of the reader and the xref
layer of the port of hayro. Question 14 of `PLAN.md` asked what to do.

On 2026-11-18 the maintainers decided:

1. `pdf.Open` gives `ErrLimit` for a file larger than `Limits.MaxMemory`,
   before it reads the file.
2. A reader that pages through the source stays in the backlog. When it
   exists, the limit applies to what it holds in memory, not to the size
   of the file.
3. A memory map is not used, because a file that shrinks while it is mapped
   crashes the program.
