# 5. The Vault is a timing gate, not a secret store

Date: 2026-07-31

## Status

Accepted. Supersedes the answer-key mechanism of ADR-0001.

## Context

ADR-0001 stores answer keys **hashed**: grading compares hashes of normalised answers, so the
key is present and functional but not legible. That closes the leak. It does not survive
contact with the rest of the loop.

**A hash cannot reveal.** A `Debrief` is defined as *what the intended path was, which signals
in the `Ticket` should have pointed where, and where time was lost*; a `Forfeit` reveals the
intended path outright. Both require the plaintext to be *retrievable after the fact*.
Irreversibility — the exact property hashing was chosen for — is wrong for the half of the
Vault whose purpose is eventual disclosure. ADR-0001 solved the leak and left the reveal
unaddressed.

**A hash cannot say "close".** With `Location claim`s now the instrument behind
`Probes to locate`, the highest-value Debrief line is *you claimed `invoice.go:applyDiscount`,
the cause was `pricing.go:roundHalfEven` — you stopped one call short*. Equality-on-hash cannot
produce it, and cannot even distinguish right-file-wrong-symbol from entirely wrong.

**Hashing pushes brittleness onto the learner.** Matching hashed free text means
`internal/billing/invoice.go` versus `billing/invoice.go`, `applyDiscount` versus
`(*Invoicer).applyDiscount`, casing, separators. Every near-miss reads as the tool being
broken, and it lands hardest at a low baseline, where "I was wrong" and "it didn't understand
me" are indistinguishable and only one of them teaches anything.

The mechanism that solves all three is already adopted. ADR-0001 runs hidden tests from a
prebuilt container image specifically so they are not readable in the working tree. That image
can equally hold plaintext keys, accepted aliases, arbitrary matching logic, and the written
intended-path narrative.

An image is only a wall if its source is not in the checkout. Building from a `vault/`
directory rebuilds the honour system ADR-0001 rejected — the image would protect nothing a
sidebar filename had already given away.

## Decision

The Vault's readable half is the container image, and it holds plaintext.

- Spoiler content — hidden tests, location keys, accepted aliases, matching logic, the
  intended-path narrative — lives in the image, not on disk in readable form.
- Answer keys are **not hashed**. Grading and revelation are both `docker run` against the
  image, gated on Exercise state: nothing is legible before a `Clear` or a `Forfeit`, all of it
  is legible after.
- The image's source lives on an **orphan `vault` branch**, never checked out. Built with
  `git archive vault | docker build -`, so the plaintext never lands in the working tree.
- No registry and no CI on day one. A registry-pulled digest is an upgrade the loop does not
  need to change to accept.

STD-009's clause *"Answer keys are stored hashed, never in plaintext"* is amended accordingly;
the rest of STD-009 is unaffected.

## Consequences

- The Vault is understood correctly for the first time: what governs it is **timing**, not
  secrecy. Half of it exists to be shown.
- Near-miss feedback becomes possible, which is most of what makes a `Location claim` worth
  filing.
- Every failed `Trace` or claim is a wrong answer rather than a typo. The brittleness that
  would have been charged to the learner is absorbed at authoring time, where a human is
  already checking fairness per exercise (ADR-0002).
- `git show vault:...` reaches the plaintext. This is a deliberate push, not an ambient leak —
  the same tier ADR-0001 knowingly accepted for agent policy. The threat model remains the
  learner's carelessness, not the learner's determination.
- The blobs are in `.git/objects` of the repo the learner clones. A second machine, or a
  re-clone, carries the spoilers with it. Accepted; the alternative is the content pipeline
  ADR-0001 declined.
- Docker moves from useful to load-bearing. It was already adopted on day one for the test
  harness; it is now also on the path for every `Debrief`.
