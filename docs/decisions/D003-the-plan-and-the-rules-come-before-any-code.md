# D3. The plan and the rules come before any code

Status: Decided.

No code is written before the plan is ready. Code includes Go source, Go
tests, scripts and the CI workflow. A document, the license, `go.mod` and the
files that git reads, such as `.gitignore`, are not code.

ivy holds many formats, and they share the layers that
[`docs/PLAN.md`](../PLAN.md) describes: the document API, the display list,
the layout of text and the fonts (D10). If the first format comes before
those layers are settled, each later format follows a different rule. okage
D3 and transit D3 made the same rule for the same reason.

The plan is ready when the maintainers answer the open questions at the end
of `docs/PLAN.md` that block phase 1.
