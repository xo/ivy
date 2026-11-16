# D18. A port can guard against hostile input

Status: Decided.

docs/UPSTREAM.md says that a deliberate difference of a port from its source
is a decision of the maintainers. The ports of hayro and moxcms add guards
against hostile input that their sources lack. The maintainers accepted them
as a class on 2026-11-16, so that each one needs no decision of its own.

A port can add a guard against hostile input when it gives the same result
as its source on valid input:

1. A limit on memory, pixels, objects, nesting or time, from `ivy.Limits` or
   from the context (D5).
2. A cap on a recursion or a loop that the source lets a crafted file make
   endless or exponential, such as a CMap that names itself.
3. A cache of a result that the source computes again, such as the search
   for `endstream`, when the result is the same.
4. A `recover` at a public function, which turns a panic into an error that
   wraps `ivy.ErrFormat`. A fuzz test still fails on the panic, so that the
   cause is fixed.
5. A shortcut that skips work whose result is empty, such as the empty
   subtrees of a tag tree of JPEG 2000.

Each guard is marked with a comment that starts with `Port:` and says why,
and the README of its package lists it. A difference that changes the result
on valid input is not a guard, and it is still a decision of its own.
