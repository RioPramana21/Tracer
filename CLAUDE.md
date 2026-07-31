# Tracer

## Coding standards

The rubric for this repo is `.claude/standards.md`. It grades Tracer's own tooling only —
the Playground is deliberately bad code and is governed by `CONTEXT.md` instead. See STD-008.

## Agent skills

### Issue tracker

Issues and PRDs live as GitHub issues, driven through the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical roles, used verbatim: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — one `CONTEXT.md` and one `docs/adr/` at the repo root. See `docs/agents/domain.md`.

## Finishing a ticket

`/implement` ends at a commit. After that commit, also:

1. Push the branch: `git push -u origin HEAD`
2. Open a PR with `gh pr create`, body stating what the ticket asked for,
   what the two review axes found, and the verification actually run
   (gofmt/vet/test output, plus any hand demonstration).
3. Link the issue with `Closes #<n>`.
4. Stop there. Do not merge — that's a human decision.