# Ports and copies

This document holds the rules for code that ivy takes from another project
(D7). A port translates code from another language into Go. A copy takes Go
code as it is, with the changes that ivy needs. Before either one, look in
okage and in magic, and use their code if they have it (D7).

## The list of upstreams

[`upstream/upstreams.tsv`](upstream/upstreams.tsv) lists each project that
ivy ports or copies. It has one row for each project, with these columns:

| Column | What it holds |
| --- | --- |
| `name` | the name of the project, and the name of its folder under `upstream/` in the root |
| `url` | the URL of its repository |
| `kind` | `port` or `copy` |
| `license` | the SPDX identifier of its license |
| `base` | the commit that ivy took the code from |
| `matched` | the newest commit whose changes ivy has, or the base if there is none yet |
| `path` | the folder in ivy that holds the code |

Add the row before the first commit of the code. Each new project needs the
approval of the maintainers (D7).

## The checkouts

The work is made from a checkout of each project in `upstream/<name>/`, in
the root of the repository. Git ignores the folder. Do not edit a file in a
checkout. If a checkout is missing, clone it at the base commit, as in this
example:

```bash
git clone https://github.com/oov/psd.git upstream/psd
```

```bash
git -C upstream/psd checkout a5d50ec0acac
```

## A copy

1. The copy goes in a folder of its own under `internal/`, or under the
   `internal/` folder of the one format that uses it, such as
   `psd/internal/oovpsd`.
2. The folder holds the `LICENSE` file of the project, byte for byte.
3. Each copied file opens with a comment that names the project, the file
   and the base commit, before the package clause.
4. A change to a copied file is marked with a comment that starts with
   `ivy:`, and it says why. A change keeps the code of the project where it
   can, so that a later commit of the project still applies.
5. The copy drops what ivy does not use, such as a file for one platform or a
   command. The `README.md` of the folder lists the files that it holds and
   the changes.
6. golangci-lint does not check a copy. `.golangci.yml` excludes its folder.

## A port

1. A port follows its source file by file and function by function, as
   okage `docs/UPSTREAM.md` says. A name that collides takes the name of its
   module, as okage D10 says.
2. A port keeps a ledger of the commits of its project after the base commit,
   in `upstream/<name>.tsv`, as okage D12 says. The ledger has the columns
   `commit`, `date`, `subject` and `status`. The status is `ported`,
   `skipped`, with the reason, or `open`.
3. Each ported file opens with the notice of the source file and names it.
4. A deliberate difference from the source is a decision of the
   maintainers.

## A new commit of a project

1. Read the commits of the project after `matched`.
2. For a port, add each one to its ledger and port it, or skip it with the
   reason.
3. For a copy, apply each change that ivy needs, and keep the changes of
   ivy.
4. Move `matched` in `upstream/upstreams.tsv` to the newest commit that ivy
   has.
