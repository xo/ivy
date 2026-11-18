# D36. csv has an option for its separator, and does not guess dates

Status: Decided.

`csv` takes its separator from the extension of the file: a comma for
`.csv`, and a tab for `.tsv` and `.tab`. Question 9 of `PLAN.md` asked
whether a program can choose it, and whether `csv` reads dates.

On 2026-11-18 the maintainers decided:

1. `csv.WithSeparator(r rune) Option` gives the separator. Without it,
   the package takes it from the extension, as before.
2. `csv` shows each cell as the file writes it, and does not guess dates. A
   wrong guess changes the data that a person reviews.
