# 2. Fixed catalog over generated exercises

Date: 2026-07-30

## Status

Accepted

## Context

Exercises could be authored ahead of time as a fixed set, or generated on demand by an
agent each time the learner asks for the next one.

Generation is attractive: content is effectively infinite and the up-front authoring cost
approaches zero. It has one serious problem. An agent asked to plant a defect will readily
produce something trivial, something that is not actually a defect, or something reachable
only by luck — and the last of these violates the fairness constraint, which matters most
precisely when the learner is least able to tell an unfair exercise from their own
incompetence.

Generation is therefore only safe behind an automated gate: the hidden test must fail
before the seed and pass after a known-good fix, the defect must be reachable from the
ticket's symptom by the named technique within a bounded number of steps, and the ticket
text must name no file, module, or symbol. Anything failing these checks is discarded
unseen.

That gate is the most technically demanding component in the system — harder than the
Playground, the tracker, or the UI. It is also squarely inside the skill set the learner
does not yet have, which makes it both the riskiest thing to build and the thing least
likely to be built correctly on a first attempt.

A fixed catalog inverts the trade. Fairness is verified once per exercise, by hand, at
authoring time — cheap per exercise, and requiring no general-purpose machinery. The cost
is finiteness: the catalog is exhausted eventually, and each batch costs real authoring
effort.

## Decision

Author a fixed, ordered catalog. No generation.

Batches are small — eight exercises initially — so that each batch is informed by having
worked the previous one. Fairness is checked per exercise at authoring time rather than by
a general gate.

Generated instances remain a plausible later feature, layered on top of an existing
catalog once the exercise format has been proven in use.

## Consequences

- The project's highest-risk component is not built, and is not on the critical path to the
  first exercise.
- Exercise quality is bounded by authoring care rather than by gate coverage, which is the
  more reliable of the two at small volumes.
- Content is finite. Exhausting the catalog requires an authoring round, not a button.
- Calibration of the difficulty ramp is empirical: small batches mean a miscalibrated ramp
  is discovered after eight exercises rather than after forty.
- If generation is added later, the gate must still be built — this decision defers that
  cost rather than eliminating it, and the existing catalog then serves as a corpus of
  known-good examples to validate the gate against.
