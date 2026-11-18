# D36. csv has an option for its separator, and does not guess dates

Status: Decided.

`csv` finds its separator in the first records of the file, because a
source has no file name. Question 9 of `PLAN.md` asked whether a program
can choose it, and whether `csv` reads dates.

On 2026-11-18 the maintainers decided:

1. `csv.WithSeparator(r rune) Option` gives the separator. Without it,
   the package finds it in the first records, as before. A quote, a line
   break or a rune that is not valid gives an error that wraps
   `ErrUnsupported`.
2. `csv` shows each cell as the file writes it, and does not guess dates. A
   wrong guess changes the data that a person reviews.
