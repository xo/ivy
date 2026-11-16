# D4. Each decision is a file of its own

Status: Decided.

dbmeta D111 says that a large project keeps each decision in a file of its
own under `docs/decisions/`, and that a small library keeps its decisions in
its plan. ivy is a large project by that measure. It holds dozens of formats,
several ports and copies of outside code, and a test module with outside
programs as oracles (D13).

Each decision is `docs/decisions/D<nnn>-<title>.md`, and
[`README.md`](README.md) in this folder is the index. The decisions were
consolidated on 2026-11-16, before the first commit, into the 15 of this
folder.
