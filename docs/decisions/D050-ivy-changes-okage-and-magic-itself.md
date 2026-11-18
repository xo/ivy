# D50. ivy changes okage and magic itself

Status: Decided. Amends D8.

D8 sends the work that ivy needs from okage and magic to those repositories.
The section "Peer sessions" of `AGENTS.md` said how to ask for it: send a
message to a Claude Code session of okage, magic or iv. On 2026-11-18 the
maintainers said that these sessions do not exist. They also said that iv
uses ivy for its documents now, so the work for iv in `docs/BACKLOG.md` is
done.

The maintainers decided:

1. No peer sessions exist for okage, magic or iv. A session of ivy does not
   wait for one and does not ask one.
2. If ivy needs a change in okage or in magic, a session of ivy can make the
   change when a maintainer asks for it. It works in a checkout or a
   worktree of that repository, and it follows the `AGENTS.md` and the
   decisions of that repository.
3. The session commits, tags and pushes okage or magic only when a
   maintainer says so, as standing rule 1 says for ivy.
4. The work that ivy needs from okage and magic stays in "Work for okage"
   and "Work for magic" of `docs/BACKLOG.md` until it is done. The list
   "Work for iv" is deleted.

The split of D8 does not change: okage draws, and magic identifies and
decompresses. ivy does not hold a second renderer, a second set of rules for
the types of files, or a decoder that belongs in `magic/compress`.
