# Context

The ubiquitous language for Tracer. Glossary only — no implementation details, no specs.

## Core

### Tracer

The whole system: a deliberately-seeded web application plus the tooling that assigns,
grades, and tracks work against it. Its purpose is training a human in tracing and
debugging unfamiliar code — not shipping a product.

### Playground

The application under investigation. A plausible product with real business logic, real
entanglement, and defects seeded into it. Nothing inside the Playground is labelled as
training material; its commit history is written to look like an ordinary product's
history.

Contrast with [[vault]].

Its subject matter is a generic **order-to-cash** flow — order placed, stock reserved,
priced, invoiced, paid, refunded — chosen for depth rather than breadth, and kept clear of
any real employer's proprietary process. The Playground's own domain terms (reservation,
credit note, period close, and so on) belong to the fictional product and are glossed
separately as they are introduced.

### Vault

Everything that would spoil an [[exercise]]: defect specifications, answer keys, hidden
tests, technique notes, the difficulty ramp. Colocated with the Playground but never read
casually — keys are stored hashed and tests run from a prebuilt image, so possession does
not imply readability.

### Exercise

One unit of trainable work. Always presented as a [[ticket]], always has a falsifiable
pass condition, always carries an intended technique. Comes in exactly two kinds:
[[bug]] and [[trace]].

Deliberately *not* called a "lesson" or a "challenge" — an Exercise is both at once.

### Bug

An [[exercise]] whose pass condition is a hidden test flipping from red to green. The
learner must locate the defect and repair it.

### Trace

An [[exercise]] whose pass condition is a structured answer checked against a hidden key —
naming a call path, an ordering, or the specific function responsible. No repair is
involved. Trains reading a feature you are about to *extend*, not one that is broken.

"Trace" never means self-assessed comprehension. If it cannot be marked wrong, it is not
a Trace.

### Ticket

The presentation of an [[exercise]] to the learner: a symptom, written the way a
non-engineer reports it. Never names a file, module, or location. This is the only view of
an unstarted Exercise that exists.

### Catalog

The fixed, ordered set of authored [[exercise]]s. Grown in small batches so that each batch
is informed by working the previous one. Not generated on demand.

The Catalog is authored **chronologically**: the [[playground]] is built as a sequence of
plausible feature commits across a simulated multi-year timeline, with defects introduced
inside those commits at the point in history where they would naturally have appeared. The
learner receives a frozen application whose history nonetheless reads as grown — which is
what makes bisecting and regression archaeology trainable.

### Symptom / Cause

Two deliberately separated locations. The **symptom** is what the [[ticket]] describes —
always something visible on a screen, because that is who writes bug reports. The **cause**
is wherever the defect actually lives. The distance between them is the tracing problem.

### Defect surface

The set of layers a cause (see [[symptom-cause]]) is permitted to hide in. Widened deliberately across batches
of the [[catalog]]: Go backend only at first, then the React frontend, then the database
and migrations, then configuration and infrastructure.

Each widening is **announced**, never silent — the learner is told when a guarantee they
have been relying on is removed. Config-borne defects come last because the source code is
correct and reading it will never reveal the problem; suspecting the environment is a
late-stage move that should be trained rather than sprung.

### Technique

A named, repeatable debugging move an [[exercise]] is built to train (e.g. reading data
flow backwards from a symptom, bisecting history). Techniques are the axis progress is
measured along — not the count of Exercises cleared.

### Clear

To pass an [[exercise]] on its own terms — hidden test green, or answer matching the key —
without having forfeited. Only a Clear advances the [[reveal ramp]].

### Forfeit

To give up an open [[exercise]] explicitly. The intended path is revealed and a [[debrief]]
is offered, but the Exercise is recorded as **not cleared** and the ramp does not advance.

Forfeiting is a supported, dignified action, not a failure state to be avoided. Its
purpose is to make quiet cheating unnecessary: without a Forfeit, a stuck learner corrupts
their own record instead of leaving a mark on it.

### Path log

The written record of hypotheses made during an [[exercise]], one line each, written
*before* the probe that tests them: what is believed, and why.

Its purpose is not bookkeeping. Acting without a hypothesis — opening files hoping
something looks wrong, changing things to see what happens — is the defining behaviour of
an untrained debugger, and a required written "because" makes it impossible to do
accidentally. The Path log is also what gives a [[debrief]] something specific to critique.

Mandatory for the first batch of the [[catalog]], when the habit is either formed or lost.

### Time to locate

The interval from starting an [[exercise]] to correctly identifying the defect's location —
tracked separately from time to fix, because locating is the skill being trained.

Recorded and shown in [[debrief]]s and trends, never displayed while an Exercise is in
progress.

### Debrief

The retrospective that follows a [[clear]] or a [[forfeit]]: what the intended path was,
which signals in the [[ticket]] should have pointed where, and where time was lost. Most
of the learning lives here rather than in the solve.

### Agent boundary

The rule governing coding agents during an open [[exercise]]: no agent reads
[[playground]] source. Method may be discussed in the abstract; on early Exercises the
named [[technique]] may be discussed too. Once an Exercise is cleared or forfeited, agents
are unrestricted and run the [[debrief]].

Enforced mechanically — Exercises are worked without an agent attached — rather than by
intention alone.

### Realistic mess

The deliberate disorder of the [[playground]]: inconsistent patterns between modules, dead
code that looks live, misleading names, stale comments describing behaviour that changed
long ago, half-finished migrations. Held at full strength from the first [[exercise]] —
mess is the *medium* Tracer trains against, not a difficulty dial.

Realistic mess always has **archaeology**: it is readable as history, so the learner can
ask "which path is current?" rather than "why is this insane?". Randomly generated noise is
not realistic mess.

### Patchy observability

The [[playground]]'s deliberate unevenness of runtime visibility: some modules
well-instrumented, others silent, log levels used inconsistently, and some log lines
actively misleading — describing behaviour the code no longer has.

The runtime twin of the stale comment in [[realistic mess]]. Teaches that observability
output is an unverified claim, and forces the technique of adding instrumentation
deliberately and temporarily to a silent module.

### Fairness

The standing constraint on every [[exercise]]: the defect must be findable by correct
method within a bounded search. An Exercise solvable only by luck teaches that debugging is
luck, and is defective regardless of how realistic it looks.

### Reveal ramp

The controlled withdrawal of support as the learner climbs. Early Exercises name their
[[technique]] and offer worked hints; late Exercises present the [[ticket]] alone. The
ramp *is* the difficulty curve.
