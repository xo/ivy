# D45. The backlog drops ivy.Cache, keeps JPEG XR, and waits on JPEG and SVM

Status: Decided.

Four questions of `PLAN.md` asked about items of the backlog.

On 2026-11-18 the maintainers decided:

1. The backlog drops the item of `ivy.Cache` for the format packages
   (question 10). `internal/fontdb` holds the fonts. A format that needs a
   cache asks again.
2. The backlog keeps JPEG XR of XPS, though no permissive decoder in Go is
   known (question 13).
3. A decoder of the JPEG files that `image/jpeg` refuses waits (question
   15). The item stays.
4. A decoder of SVM pictures waits (question 20). The item stays.
