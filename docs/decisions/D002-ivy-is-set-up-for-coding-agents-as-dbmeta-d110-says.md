# D2. ivy is set up for coding agents as dbmeta D110 says

Status: Decided.

ivy takes the structure of okage, which takes the structure of dbmeta and
transit. dbmeta D110 sets the layout and the base rules for coding agents in
every `xo` repository, and this decision adopts it. ivy writes its own rules
in the same form, and it does not copy the text of okage word for word. Its
`README.md` has the badges and the links to the other `xo` projects, as the
other `xo` repositories have. A logo comes later.

The repository has these files in its root:

- `README.md`, for a person who uses ivy.
- `AGENTS.md`, which holds the rules for a coding agent and for a person.
- `CLAUDE.md`, which holds one line, `@AGENTS.md`.
- `CONTRIBUTING.md`, for a person who changes ivy.
- `LICENSE`, `go.mod`, `skills-lock.json`, `.gitignore` and `.gitattributes`.

The `docs/` folder holds the plan, the design of the API, the survey of the
formats, the backlog and the decisions. The `simple-english` and
`go-pedantry` skills are ordinary folders in `.agents/skills/` and in
`.claude/skills/`, as dbmeta D89 says.
They are copies of the skills of okage, at the hashes that
`skills-lock.json` records.

`AGENTS.md` opens with the three standing rules of dbmeta D110. The first rule
is to stage every change for review, and to commit only when a maintainer says
so.

The other `xo` repositories have `skills_test.go` and `docs_test.go`, which
make sure that the skills are copies, that the root holds only four
documents, and that each decision reference exists. They are Go code, so they
come in phase 1 (D3).
