# D41. A fuzz test makes an input smaller for 2 seconds at most

Status: Decided.

After each input that finds new code, the fuzzer of Go makes the input
smaller for up to 60 seconds. For a seed of tens of kilobytes and a run of
some milliseconds, a fuzz run of 1 minute does nothing else, and seems to
hang. Question 17 of `PLAN.md` asked how each package bounds it.

On 2026-11-18 the maintainers decided:

1. The fuzz command of `AGENTS.md` and of CI gives `-fuzzminimizetime 2s`,
   such as `go test -run '^$' -fuzz FuzzOpen -fuzztime 1m
   -fuzzminimizetime 2s ./psd`.
2. A package does not set the flag itself. The `TestMain` of `msppt` and
   `msxls` that sets it goes.
