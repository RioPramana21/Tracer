# Standards — how I write code in `Tracer`

**What this file is:** the bar my own diffs meet in **this repo**, and the rubric the
`verifier` subagent grades against before I open a PR. Loaded at session start; read before
writing code, not only at review.

**Marked sections are author process, not review criteria.** Everything between
`<!-- verifier:skip-start -->` and `<!-- verifier:skip-end -->` is skipped by the `verifier`;
everything outside them it reads in full. **A rule meant to grade a diff must live outside the
markers** — one parked inside is a rule that never fires. *(A real marker starts its own line at
column 0; that is how it is told apart from this sentence, which merely names it.)*

**What it is not:** a wishlist for the codebase. Anything I think *the repo* should change
but have no standing to change unilaterally goes to `docs/improvements.md`. The boundary is
one question:

> **Can I do this inside my own diff, without asking anyone, without widening the ticket?**
> Yes → here. No → `docs/improvements.md`.

**Scope:** this repo only. `[HOUSE]` rules are observations about *one team's* code and are
actively misleading applied anywhere else. A new repo gets a new copy of this template with
every `[HOUSE]`/`[TOLD]` entry deleted.

**Changelog:** every change to this file is logged in `standards-changelog.md` beside it.
No silent edits — that file is the only history this rubric has.

**Created:** 2026-07-31 · **Last amended:** 2026-07-31

---

## STD-008 `[MINE]` — what this rubric does and does not grade

This repo deliberately contains code that is *supposed* to be bad. Read this before applying
anything below it.

**Graded — the Tooling.** Everything Tracer uses to run itself: the grading CLI, the tracker
and its store, the vault harness, build and container config. This code must be trustworthy,
because it is what tells the learner whether they were right.

**Not graded — the Playground.** The seeded application is inconsistent, misleadingly named,
stale-commented and patchily instrumented **on purpose** (see `CONTEXT.md` → *Realistic mess*,
*Patchy observability*). Grading it against a quality bar would destroy the thing it is for.

The Playground is not ungoverned; it is governed by a different document. Its constraints are
*plausibility, archaeology and fairness*, not cleanliness, and they live in `CONTEXT.md` — not
here. A `verifier` run over Playground code is a category error; say so rather than filing
findings.

**Where the boundary is unclear, it is the Tooling.** Hidden tests, answer-key hashing, the
seeding scripts and anything that decides a pass or fail are Tooling, however close to the
Playground they sit.

---

## How to read an entry

Every rule carries an **ID** and a **source tag**. The ID is permanent and never reused —
it is what the changelog and PR comments cite.

**ID prefix for this rubric: `STD`.** IDs are scoped to this file.

| Tag | Means | Authority |
|---|---|---|
| `[GUARD]` | A safety floor: security, data integrity, secret handling | **Absolute.** Outranks everything, including observed house code. Never silently violated, not even to match the surrounding file |
| `[TOLD]` | A human on this team told me directly. Records who and when | **High.** Cite the person if challenged in review |
| `[HOUSE]` | Observed in this codebase, ≥2 sightings or confirmed by a human here | **High.** Beats my preferences. Follow even if I disagree |
| `[MINE]` | Industry default applied because the codebase has no convention either way | **Weakest.** Applies only where the repo is silent. Never cite as "the standard here" |

**Conflict rule:** `[GUARD]` > `[TOLD]`/`[HOUSE]` > `[MINE]`. Where an industry best practice
contradicts an observed house convention, follow the house and file the disagreement in
`docs/improvements.md` — do not quietly deviate inside a ticket. **Unless the challenge
protocol below says otherwise.**

---

## Challenge protocol — every rule here is challengeable

A rubric records what was **observed**, which is not proof it is **right**. Two sightings of
a mistake is a mistake sighted twice. So before applying any entry, check it against
established practice — and act by what the conflict actually costs:

| Conflict class | What happens | Who decides |
|---|---|---|
| **Security / data integrity** — injection, secret or PII exposure, authz bypass, silent data loss, unvalidated boundary input | **Stop. Do not write it.** Name the concrete failure — the input, the state, the damage — and the compliant alternative, *before* any code. A `[HOUSE]` tag does not override this; "the surrounding code does it that way" is not a defence | The human, explicitly, on the record. Never assumed |
| **Performance defect** — cost that grows with data: N+1, unbounded query or render, work in a hot loop, blocking the main thread | Flag **before** writing, with the cost stated concretely (what grows, with what). Follow the house only if the cost is bounded and I can say by how much | The human, after seeing the cost |
| **Style, architecture, taste** | Follow the house, silently. File the disagreement in `docs/improvements.md` | Nobody. It just goes in the file |

**The line between tier 2 and tier 3 is growth.** A slower loop is taste. A loop that gets
slower as the table grows is a defect.

**A challenge needs evidence too.** "No performance claim without a number" cuts both ways:
if I cannot name what grows and with what, my objection is a preference wearing a defect's
clothes — that is tier 3, and it goes in `docs/improvements.md` like any other opinion.

**Challenging is not deviating.** The output of a tier-2 or tier-3 challenge is a sentence to
the human plus a filed entry — never a quiet deviation inside a ticket. Tier 1 is the one
exception: there, *not writing the unsafe code* is the correct action, and it gets surfaced
immediately rather than after the fact.

**Admission check.** A rule must survive this same protocol *before* it is admitted. If the
two sightings that would make something `[HOUSE]` are both an injection risk, what I observed
is a bug pattern, not a convention — it goes to `docs/improvements.md`, never into this file.

---

## Scope discipline (the rule that protects every other rule)

**STD-001** `[MINE]` — a PR gets rejected for size and surprise far more often than for style.

- Change what the ticket asks for. Nothing adjacent, however tempting.
- Something wrong outside the blast radius → `docs/improvements.md`, not the diff.
- Deliberate inconsistencies stay deliberate. Note them in the PR so a reviewer doesn't
  think I missed one.
- No new files in this repo unless the ticket needs them.

---

## `[GUARD]` — the floor

These hold regardless of what the surrounding code does.

- **STD-002** `[GUARD]` Parameterised queries only. No string-concatenated SQL, no template
  literal carrying user input, ever.
- **STD-003** `[GUARD]` Never log or echo a token, password, connection string, key, or full
  request body on an auth path.
- **STD-004** `[GUARD]` Secrets files (`.env`, credential stores, cert/key material) are never
  read, copied, printed, or pasted.
- **STD-005** `[GUARD]` Validate input at the trust boundary. Never trust a request field's
  presence, type, or range.
- **STD-006** `[GUARD]` Errors returned to a client say what went wrong without leaking
  internals — no stack traces, no SQL, no file paths in a response body.
- **STD-009** `[GUARD]` **Spoilers never leak into a learner-visible surface.** A defect's
  location, the file or symbol it lives in, or the intended solution must never appear in a
  Playground commit message, a branch name, an issue or PR title, a filename, a test name, a
  log line, or a `Ticket`. Spoiler content lives only inside the Vault image and is legible
  only after a `Clear` or a `Forfeit` — never in the working tree, and never in a reply to an
  open Exercise. Losing this is unrecoverable: a spoiled exercise cannot be un-spoiled for the
  one person the repo exists for. See ADR-0001 and ADR-0005.
- **STD-010** `[GUARD]` **The progress record is never reachable from the Playground.** No
  shared database, schema, connection, or migration path. A seeded defect must be structurally
  incapable of corrupting the learner's history — including the `Path log` and `Time to locate`
  data, which cannot be reconstructed once lost.

---

## Performance claims

**STD-007** `[MINE]` — methodology. Applies to code, CVs, PR descriptions, and interviews alike.

- **No performance claim without a number I produced.** "Faster", "more efficient", "reduced
  renders" are unsayable until measured.
- **Benchmark the loop that actually runs**, not the function I edited. A microbenchmark
  around a changed function measures the *cost* side of a trade while being structurally
  blind to the *benefit* side.
- **An unstable ratio is the finding.** When small-N runs disagree between passes, the honest
  report is "below this machine's noise floor", not either number.
- **Say what I did not measure.** Recording something as unmeasured beats implying it was checked.
- **When measurement comes back flat, that is the deliverable.** "I measured it, it was noise
  at real data sizes, so I optimised for correctness instead" is stronger than a number I'd
  have to defend — and it's true.

---

## Domain language

**STD-011** `[MINE]` — code that names a Tracer concept uses the term as `CONTEXT.md` defines
it: `Playground`, `Vault`, `Exercise`, `Bug`, `Trace`, `Ticket`, `Catalog`, `Technique`,
`Clear`, `Forfeit`, `Replay`, `Debrief`, `Path log`, `Location claim`, `Probes to locate`,
`Time to locate`. Package names, types, CLI verbs and test names all count.

A concept the glossary does not have is a signal, not a licence — either the wrong word is
being invented, or the glossary has a real gap worth filling before the code hardens around a
synonym.

---

<!-- verifier:skip-start
  Author process, not review criteria: the pre-PR checklist, any commit/PR convention entries,
  and this file's own intake and pruning rules. None of it can grade a diff — the verifier
  reviews `git diff`, not a commit message, a PR body, or a rubric's upkeep. Keep real rules
  above this marker.
-->

## Before I open a PR

1. Build/test command — `test -z "$(gofmt -l .)" && go vet ./... && go test ./...`, run from
   the repo root. Paste the output rather than asserting it. (`gofmt -l` exits 0 even when it
   names files, so the `test -z` is what makes formatting a gate rather than a report.)
2. `git status` — clean of anything personal.
3. `git diff` read start to finish, by me. Anything I can't explain does not ship.
4. `verifier` subagent over the diff against this file — **Tooling diffs only** (STD-008).
5. Deliberate inconsistencies noted in the PR description.
6. Commit message matches this repo's conventions. **No AI attribution anywhere** — no
   `Co-Authored-By`, no "generated with" footers, in commits, PR bodies, comments or docs.

---

## Keeping this file honest

**Intake — a rule may only be added when it has a real source:**

| Route | Becomes | Bar |
|---|---|---|
| Security or data-integrity floor | `[GUARD]` | Immediate, no sightings needed. It holds everywhere |
| A human here corrected me in review | `[TOLD]` | Immediate. Record who + when + their words verbatim |
| I observed the same convention twice or more | `[HOUSE]` | Two sightings, with file paths |
| I observed it once | not here — a learning note | One sighting is an anecdote, not a convention |
| I read it in a blog / already knew it | `[MINE]` | Only where the repo is genuinely silent |

**No rule enters this file from a confident guess about this system.** A fabricated house
convention is worse than none — it gets cited in review.

**Sightings in the Playground are not sightings.** Its patterns were authored to be wrong.
`[HOUSE]` rules may only be drawn from Tooling code (STD-008).

**Every admission, amendment, promotion, demotion, and deletion is logged** in
`standards-changelog.md` with the ID, the date, who proposed it, and why. An unlogged edit is
a rule with no provenance, and a rule with no provenance cannot be defended or retired.

**Demotion and pruning — run this at each checkpoint:**

- A `[MINE]` rule contradicted by observed house code → delete it, file the disagreement.
- A rule that never once applied across ~3 tickets → cut it. A rubric nobody's diff touches
  makes review noisier without making code better.
- A rule I keep violating → either it's wrong, or it belongs in a hook instead of prose.
- Anything now enforced mechanically (linter, CI) → delete from here. Duplicated rules drift.
- **Exception: `[GUARD]` rules are never pruned for disuse.** A floor that never triggered is
  a floor doing its job.

**Review trigger:** whenever a PR comes back with a comment I didn't predict. That comment is
the highest-quality standards input available — a real reviewer saying what this team actually
cares about. Log it the same day, `[TOLD]`, verbatim.

<!-- verifier:skip-end -->

*(Anything added below this marker **is** read by the `verifier`. New review criteria go in the
rule sections above the skipped block, never inside it.)*
