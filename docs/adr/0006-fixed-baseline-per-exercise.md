# 6. Exercises branch from a fixed baseline; the Playground never accumulates fixes

Date: 2026-07-31

## Status

Accepted

## Context

A `Bug` is cleared by editing the Playground. Nothing said what happens to those edits when the
next Exercise starts, and the two answers are not equally safe.

Letting fixes accumulate is the realistic option — a codebase you maintain, getting better as
you work it. It breaks ADR-0002. That ADR chose a fixed catalog specifically so *fairness is
verified once per exercise, by hand, at authoring time*, and that verification is performed
against a **known tree**. If the learner's own fixes reshape the Playground, every downstream
fairness check silently expires: Exercise 20 was verified against a tree that no longer exists.

The failure is invisible in the worst possible way. An Exercise that has become unfair looks
exactly like an Exercise the learner is too weak for — which is the precise confusion ADR-0003
was written to eliminate, arriving through a different door.

There is a sharper version. A `Bug` clears when a hidden test goes green, and a green test does
not mean a good fix. A defect special-cased into passing leaves a landmine that later Exercises
are authored in ignorance of.

## Decision

Every Exercise is worked on its own branch, cut from a single fixed baseline commit. Fixes are
never merged into the baseline.

The Playground's simulated multi-year history sits below the baseline, so `git log` and
`git bisect` remain fully trainable (ADR-0001).

A forfeited Exercise can be `Replay`ed by resetting its branch. Replays are recorded but never
`Clear`, and produce no `Probes to locate` or `Time to locate` — there is no search once the
answer is known.

## Consequences

- Every Exercise starts from the exact tree its fairness was verified against, for the life of
  the catalog. The guarantee ADR-0002 bought is actually kept.
- Fix branches are a record. What was changed, and how badly, is available at `Debrief` and
  afterwards.
- The Playground never improves. The learner will fix the same class of thing twice and the
  application will still be broken both times. The satisfaction of maintenance, and the realism
  of owning a codebase, are given up deliberately — neither is what Tracer trains, and buying
  them with the fairness guarantee is a bad trade.
- The progress record cannot live in the Tracer working tree: constant checkouts clobber files.
  This forces the record's location, and STD-010 is satisfied structurally as a result
  (ADR-0004).
- If a batch ever needs to be authored against a tree that includes earlier fixes, the baseline
  must be re-cut *before* that batch is written, never after. Re-baselining mid-batch
  invalidates exactly what this decision protects.
