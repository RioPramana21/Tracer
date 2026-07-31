# 4. The exercise loop is a local CLI over plain files

Date: 2026-07-31

## Status

Accepted

## Context

An Exercise has to be received, worked, and submitted somewhere. The obvious candidate was a
board: a `Ticket` is defined as a symptom written the way a non-engineer reports it, ADR-0001
already uses the phrase "exercise board", and this repo drives its own construction through
`gh`. A Jira-shaped ticket arriving in a tracker is what the workplace actually looks like.

Two things count against it.

ADR-0001 lists issue and PR titles as an **ambient leak vector**, precisely because this repo
uses `gh`. Putting exercises on a board means the surface carrying the `Ticket` is the same
surface carrying the seeding work — the leak the ADR was written to close, reintroduced at the
point of delivery.

More decisively, a board cannot grade. Clearing a `Bug` means running a hidden test from a
container (ADR-0001), which is a local invocation. Something local must exist regardless, so a
board is an *additional* surface rather than an alternative one.

The remaining question was what the progress record is made of. The `Path log` is the deciding
factor: an entry is prose the learner writes — a hypothesis and a because. A database forces
that prose through CLI flags; files let it be written in an editor. And STD-010 names the
record as unreconstructable if lost, which argues for the most boring, most recoverable format
available rather than a binary file with a schema and a migration history.

The record's *location* is forced rather than chosen. ADR-0006 branches per Exercise, and a
checkout clobbers files, so the record cannot live in the Tracer working tree at all.

## Decision

The loop is a local CLI. No board, no server, no network dependency.

The progress record is plain files — one per Exercise attempt — outside the Tracer repo, in a
directory that is itself a git repository. `Path log` entries are written through `$EDITOR`.

Realism is not what this trades away. The Playground already supplies mess, archaeology and
patchy observability; a ticket in a tracker adds atmosphere, not difficulty, and costs a
spoiler surface.

A board projection over the same record remains possible later, but nothing is designed toward
it now.

## Consequences

- STD-010 holds structurally rather than by construction: a Postgres defect cannot reach a
  markdown file the Playground has no path to.
- The record survives the tooling. A CLI that will not build, a schema migrated wrongly, a
  format changed — none of them cost history that cannot be rebuilt.
- Versioning the record makes retroactive self-flattery visible. It does not prevent editing;
  it means editing has to be meant. This matters because `Forfeit` is only worth choosing if
  the record it protects is trustworthy.
- Cross-Exercise trend queries — probes by `Technique`, by `Defect surface` — must be written
  as aggregation over files rather than as SQL. At batches of eight this is cheap; a
  rebuildable SQLite index can be added later without the files changing.
- The `Agent boundary` mechanics this ADR assumes were measured against the harness rather
  than taken on trust. The digest scheme holds and every ordinary read path is closed, but
  deny rules gate operations on a path and do not taint the data, and OS-level sandboxing —
  the only thing that would — does not run on native Windows. See
  `docs/findings/0001-agent-boundary-enforceability.md`. Whether Tracer should require WSL2
  for Exercise work is left open there and needs its own ADR.
- No ticket ever arrives unbidden. Exercises are summoned, which removes "unfamiliar code at
  unwelcome timing" from what Tracer trains. Accepted knowingly.
