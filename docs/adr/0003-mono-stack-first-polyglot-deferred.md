# 3. Mono-stack first, polyglot deferred

Date: 2026-07-30

## Status

Accepted

## Context

Tracer exists to prepare its learner for a workplace whose backend varies by project —
Node, Laravel, Python, Go. The obvious inference is that the Playground should be polyglot
from the start, since being dropped into an unfamiliar language is exactly the situation
being trained for.

Against that: tracing skill is largely language-independent. Reading data flow backwards
from a symptom, bisecting the search surface, instrumenting one hypothesis at a time — these
are identical across languages. What differs is syntax, idiom, and where a given framework
hides its indirection.

A learner starting at roughly 1/10 who works in an unfamiliar language therefore faces two
independent failure sources with no way to separate them. When stuck, they cannot answer
whether the method failed or the language did. That ambiguity destroys the feedback signal
the project exists to produce, and it is most damaging exactly when the baseline is being
established.

Deferring polyglot also turns out to buy something rather than merely postponing a cost.
Introducing a second language later as a *separate service* adds a network boundary, and
cross-service tracing — symptom surfacing in one runtime, cause living in another, with
only a correlation ID connecting them — is a distinct and higher-value skill than reading a
second syntax. It also puts existing observability experience (Grafana, Loki, Jaeger,
OpenTelemetry) to work.

## Decision

The Playground is mono-stack for the first phase: Go backend, React/TypeScript/Vite
frontend, PostgreSQL.

A second language is introduced later, at roughly the midpoint of the learner's progression,
and specifically as an additional service across a network boundary rather than as a rewrite
or a parallel implementation.

## Consequences

- The first phase is deliberately more comfortable than the workplace it trains for. The
  intended discomfort is "I cannot find it", not "I cannot read it".
- Failures during early exercises are attributable: a stuck learner can be confident the
  method is what needs work.
- Polyglot capability is not dropped, and arrives carrying cross-service tracing with it —
  two skills for the cost of one addition.
- Language-specific tracing hazards (framework magic, dynamic dispatch, service containers)
  go untrained for the whole first phase. These are real and will need their own rungs.
- Reversible: adding a service is additive and requires no change to existing exercises.
