# Contributing to ivy

Before you change anything, read three things:

1. [`AGENTS.md`](AGENTS.md) holds the rules. It is written for a coding agent,
   and every rule in it holds for a person too. `CLAUDE.md` holds one line
   that imports it, for Claude Code.
2. [`docs/PLAN.md`](docs/PLAN.md) holds the plan, and the open questions at
   its end.
3. [`docs/decisions/`](docs/decisions/README.md) holds every decision, one
   file each. Read the status of a decision before the decision.

Do not decide an open question on your own. Add it to the end of
`docs/PLAN.md`, and ask a maintainer.

On 2026-11-16 ivy holds the plan and no code. The survey of the formats is
in [`docs/FORMATS.md`](docs/FORMATS.md), and the design of the API is in
[`docs/API.md`](docs/API.md).

## Four rules that a change can break

1. ivy is pure Go. A package that a user imports does not use cgo and does
   not run an outside program (D1).
2. Each decoder and each encoder is a package of its own, and the root
   package imports none of them (D6).
3. ivy uses what okage and magic hold before it ports or copies code (D7).
4. ivy never holds code under the GPL, the LGPL or the AGPL. Copied code
   keeps its license in its own folder (D7).

## The upstream checkouts

ivy ports or copies code from other projects. The work is made from a
checkout of each project in `upstream/<repository>/`, in the root of the
repository. Git ignores the folder. `docs/UPSTREAM.md` will list each project
and say how to port or copy it. Do not edit a file in a checkout.

## Agent skills

The repository carries two agent skills. A skill is a set of instructions
that a coding agent loads for a task. `simple-english` sets how prose is
written, and `go-pedantry` sets how Go is written.

`skills-lock.json` names the source of each skill. The `skills` command from
npm writes that file, and version 1.7.0 is the one that the other `xo`
repositories measured. It writes each skill into two folders. Codex and the
other agents read `.agents/skills/<name>`, and Claude Code reads
`.claude/skills/<name>`.

To add a skill or to update one, run the command for it in the repository
root:

```bash
npx skills@1.7.0 add AminBlg/SimpleEnglish --skill simple-english --agent codex claude-code --copy -y
```

```bash
npx skills@1.7.0 add oborchers/fractional-cto --skill go-pedantry --agent codex claude-code --copy -y
```

Keep `--copy`. Without it, the command writes `.claude/skills/<name>` as a
symbolic link. A Windows checkout writes a symbolic link as a text file, and
Claude Code then loads no skill and reports nothing. See dbmeta D89.

`.claude/settings.local.json` holds the Claude Code permissions of one
person. The root `.gitignore` ignores it.

## Before you send a change

Run the checks under "Before you stage" in [`AGENTS.md`](AGENTS.md). Then
stage the change for review. A maintainer commits.

## Releases

A maintainer makes each release.
