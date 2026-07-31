# Standards changelog — `Tracer`

Every change to `standards.md` beside this file, newest first. **This is the only history the
rubric has** — an unlogged edit is a rule that appeared from nowhere.

**Log an entry for:** adding a rule, amending its wording or scope, promoting or demoting its
tag, retiring it, or deciding *not* to add one after a challenge (that decision is worth
keeping — it stops the same rejected rule coming back in three weeks).

**Never renumber, never reuse an ID.** A retired rule keeps its number and stays in this log;
`STD-014` must mean one thing forever, or every citation of it rots.

**Entry format:**

```markdown
## YYYY-MM-DD · STD-### · <added | amended | promoted | demoted | retired | rejected>

**Tag:** `[MINE]` → `[HOUSE]`   *(omit if unchanged)*
**Rule:** one-line restatement, so this log is readable without the rubric open
**Source:** who proposed it — me / <name> in review / verifier / a PR comment / an audit
**Why:** the reason it changed. Not "improve quality" — the specific thing that prompted it
**Evidence:** file paths + line numbers, the ticket, or the verbatim quote. Required for
`[HOUSE]` and `[TOLD]`
**Challenge:** result of the challenge protocol — which tier, and what was decided.
`n/a` if the rule raised no conflict with established practice
```

---

## 2026-07-31 · STD-011 · added

**Tag:** `[MINE]`
**Rule:** Code naming a Tracer concept uses the term as `CONTEXT.md` defines it — packages,
types, CLI verbs and test names all count.
**Source:** Rio, at rubric creation
**Why:** The glossary was written before any code exists, which is the only moment this rule
is cheap. Once a package is named `challenge` and the glossary says `Exercise`, the synonym is
load-bearing and the rename is a refactor nobody schedules.
**Evidence:** `CONTEXT.md`, 20 terms, committed 05a1861
**Challenge:** n/a — no conflict with established practice; ubiquitous language is the
mainstream position.

## 2026-07-31 · STD-010 · added

**Tag:** `[GUARD]`
**Rule:** The progress record is never reachable from the Playground — no shared database,
schema, connection or migration path.
**Source:** Rio, at rubric creation
**Why:** Seeded defects will include data-corruption classes (double-decrement, bad migration,
over-wide cascade delete). If the training history shares a store, a seeded defect can destroy
it — and `Path log` and `Time to locate` data cannot be reconstructed. The secondary failure is
worse than the primary: the learner would be unable to tell whether the tracker is lying to
them, which destroys trust in every result the tool reports.
**Evidence:** design decision Q8 of the 2026-07-30 grilling session; `CONTEXT.md` → *Path log*,
*Time to locate*
**Challenge:** tier 1 (data integrity), resolved by adopting the rule. Silent data loss with no
recovery path.

## 2026-07-31 · STD-009 · added

**Tag:** `[GUARD]`
**Rule:** Spoilers never reach a learner-visible surface — not in commit messages, branch
names, issue or PR titles, filenames, test names, log lines or Tickets. Answer keys hashed,
never plaintext.
**Source:** Rio, at rubric creation
**Why:** Every exercise depends on the learner not knowing where the defect is, and the learner
owns the repo. The leaks are ambient rather than deliberate — an incidental `git log`, a
filename in the editor sidebar — which is exactly why willpower is not a defence. A spoiled
exercise cannot be un-spoiled for the one person this repo exists for.
**Evidence:** ADR-0001 (spoiler containment), committed 05a1861
**Challenge:** tier 1 by analogy (integrity of the artefact the repo exists to produce),
resolved by adopting the rule.

## 2026-07-31 · STD-008 · added

**Tag:** `[MINE]`
**Rule:** The rubric grades the Tooling — CLI, tracker, vault harness, build and container
config. It does not grade the Playground, which is deliberately bad. Ambiguous cases are
Tooling.
**Source:** Claude, proposed at rubric creation; accepted by Rio
**Why:** Tracer's central deliverable is code that is *supposed* to violate a quality bar —
inconsistent, misleadingly named, stale-commented, patchily instrumented. Without an explicit
scope boundary the rubric either forbids the design or gets routinely ignored, and a rubric
that is routinely ignored is worse than none. Placed as the first rule because every rule below
it depends on knowing what it applies to.
**Evidence:** `CONTEXT.md` → *Realistic mess*, *Patchy observability*, *Fairness*
**Challenge:** n/a — the rule is itself a scoping decision, not a technical claim.

## 2026-07-31 · rubric created

**Source:** Rio, after the initial `/grill-with-docs` session settled the design and before any
Playground code is generated
**Why:** Agents will author a large codebase that will not be reviewed line by line, and the
design deliberately calls for mess. Without a written bar and an explicit scope boundary, there
is no way to tell *designed* mess from slop — and slop breaks the fairness constraint that makes
an exercise solvable at all. The rubric is cheapest to write now, while the repo has no code to
retrofit.
**Seeded from:** the generic template — machinery only, zero `[HOUSE]` rules. `[HOUSE]` and
`[TOLD]` entries deleted as required for a new rubric. Everything observed about this codebase
gets added from here, with evidence.
