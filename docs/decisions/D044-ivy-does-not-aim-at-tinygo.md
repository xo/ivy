# D44. ivy does not aim at TinyGo

Status: Decided.

okage builds with TinyGo. Question 1 of `PLAN.md` asked whether a build of
ivy with TinyGo is a goal that CI tests. Such a goal also forbids `reflect`
and `encoding/json` in the packages that a user imports.

On 2026-11-18 the maintainers decided that ivy builds with the Go toolchain,
as D1 says, and CI does not test a build with TinyGo. A program that needs
TinyGo can ask again.
