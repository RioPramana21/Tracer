# 1. Spoiler containment: one repo, two surfaces

Date: 2026-07-30

## Status

Accepted

## Context

Tracer trains one person — the same person who commissions the content. Every exercise
depends on that person not knowing where the defect is, yet the repository they own
contains the answer.

The leaks are ambient rather than deliberate, which is what makes willpower an inadequate
defence:

- Commit messages naming seeded defects, visible from a single incidental `git log`.
- File names in the editor sidebar (`exercises/012-stale-cache/answer-key.json`).
- Hidden tests, which must exist on disk to run, and which describe the defect they prove.
- Issue and PR titles, since this repo drives work through `gh`.
- Coding agents operating in the repo, which can read and summarise the vault on request.

A second constraint pulls in the opposite direction: `git log` and `git bisect` are
themselves high-value tracing techniques the learner must train. That requires the
Playground's history to be a plausible sequence of feature commits, one of which innocently
introduced the defect — not a single "seed all bugs" commit.

Options considered:

1. **Two repositories.** Vault private and separate; Playground generated from it.
2. **One repo, two surfaces.** Vault colocated but rendered unreadable in practice.
3. **One repo, honour system.** Everything plain, discipline as the only wall.
4. **Agent policy only.** Rules in `CLAUDE.md` plus permission denies on vault paths.

## Decision

Option 2, with option 4 layered on top.

- Answer keys are stored **hashed**. Grading compares hashes of normalised answers, so the
  key is present and functional but not legible.
- Hidden tests run from a **prebuilt container image**, not from readable source in the
  working tree.
- Playground commits are written to read as innocent feature work. Authoring history and
  Playground history are kept distinct.
- Seeding issues are kept off the exercise board.
- Agent-facing rules and path denies are added as a second, softer layer.

Docker is therefore adopted on day one — as plumbing for the test harness and database,
explicitly *not* as exercise surface. Infrastructure does not become a place defects live
until much later in the ramp.

## Consequences

- Ambient leaks are closed structurally rather than behaviourally. Hashing removes the
  answer-key leak outright; the container removes the hidden-test leak.
- The cost of a content pipeline and a second repository is avoided at a stage where the
  learner has not yet cleared a single exercise. Infrastructure-first is a common cause of
  death for solo projects.
- Agent policy is understood as policy, not a wall: it stops an honest agent, not a
  deliberately-pushed one. Accepted knowingly.
- Docker sits in the stack before the learner is ready to debug it. Its failures are
  therefore a support burden rather than a training opportunity, at least initially.
- Reversible upward: if leakage proves unmanageable, the vault can be extracted into a
  separate repository without changing the Playground.
