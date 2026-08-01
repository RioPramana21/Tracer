// Package exercise is the Exercise loop (cmd/tracer's own doc comment,
// ADR-0004): the orchestrator that composes the Catalog, the progress
// record, the Playground checkout, and the Agent boundary into next, start
// and status. It is where the loop's decisions live, the same way
// agentboundary.Boundary — not main.go — owns the boundary's decisions.
package exercise

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/RioPramana21/Tracer/internal/agentboundary"
	"github.com/RioPramana21/Tracer/internal/catalog"
	"github.com/RioPramana21/Tracer/internal/gitrepo"
	"github.com/RioPramana21/Tracer/internal/record"
	"github.com/RioPramana21/Tracer/internal/vault"
)

// ErrAlreadyOpen is returned by Start when an Exercise is already open.
var ErrAlreadyOpen = errors.New("an Exercise is already open")

// ErrCatalogExhausted is returned by Start when the Catalog has no Exercise
// left to offer.
var ErrCatalogExhausted = errors.New("the Catalog has no Exercise left to offer")

// ErrNoExerciseOpen is returned by Submit when no Exercise is open to grade.
var ErrNoExerciseOpen = errors.New("no Exercise is open")

// ErrWrongBranch is returned by Submit when the Playground checkout is not
// on the open Exercise's fix branch.
var ErrWrongBranch = errors.New("the checkout is not on the Exercise's fix branch")

// Grader grades a submission tree against the open Exercise's hidden test.
// vault.Client satisfies this: production wiring passes the real Vault
// image, and a test substitutes a fake so most of Submit's logic never
// touches Docker (issue #1's testing decisions).
type Grader interface {
	Grade(tree string) (vault.Verdict, error)
}

// Debriefer judges Location claims against the Vault's near-miss bands and
// reveals the intended path, gated on the closed flag it is passed.
// vault.Client satisfies this, refusing with vault.ErrExerciseOpen when
// closed is false before ever touching Docker.
type Debriefer interface {
	Debrief(closed bool, claims []vault.Claim) (vault.Debrief, error)
}

// DebriefResult is what Loop.Debrief reports: the Vault's Debrief content,
// plus Probes to locate and Time to locate. The two counts are Tracer's own
// instrumentation over the learner's Path log and elapsed clock, never the
// Vault's — neither is spoiler content (CONTEXT.md) — so they travel
// alongside the Vault's Debrief rather than inside vault.Debrief itself.
//
// Both are nil if no filed Location claim ever judged Correct — an Exercise
// can be Cleared by a fix without the learner ever having named where the
// Cause was (CONTEXT.md's Probes to locate entry: "absent against a Clear
// is a finding about the solve, not a gap in the record").
type DebriefResult struct {
	vault.Debrief
	ProbesToLocate *int
	TimeToLocate   *time.Duration
}

// SubmitResult is what Submit reports back. Deliberately impoverished — no
// assertion text, test name, diff or path ever crosses the Vault boundary
// (STD-009) — but enough to say whether the Exercise was Cleared and, if
// not, whether the hidden test or the Agent boundary is why.
type SubmitResult struct {
	Passed   bool
	Cleared  bool
	Boundary agentboundary.Status
}

// Loop is one learner's view of the Exercise loop: a Catalog to draw from and
// the progress record of what they have already attempted.
type Loop struct {
	Catalog catalog.Catalog
	Record  record.Store
}

// Load reads the Catalog at catalogPath and opens the progress record at
// recordDir.
func Load(catalogPath, recordDir string) (Loop, error) {
	c, err := catalog.Load(catalogPath)
	if err != nil {
		return Loop{}, err
	}
	return Loop{Catalog: c, Record: record.Store{Dir: recordDir}}, nil
}

// Next returns the next Exercise the learner has not yet attempted.
func (l Loop) Next() (catalog.Entry, bool, error) {
	used, err := l.Record.Used()
	if err != nil {
		return catalog.Entry{}, false, err
	}
	entry, ok := l.Catalog.Next(used)
	return entry, ok, nil
}

// Start opens the next Exercise: it cuts a fix branch from the Exercise's
// fixed baseline into a fresh clone of playgroundSrc at
// <checkoutDir>/Playground, arms the Agent boundary over checkoutDir, and
// records a new open Attempt.
//
// Refuses with ErrAlreadyOpen if an Exercise is already open, and with
// ErrCatalogExhausted if there is no next Exercise to start — both checked
// before anything is cloned or armed, so a refusal leaves no partial state.
func (l Loop) Start(playgroundSrc, checkoutDir string) (record.Attempt, agentboundary.Record, error) {
	if _, open, err := l.Record.Open(); err != nil {
		return record.Attempt{}, agentboundary.Record{}, err
	} else if open {
		return record.Attempt{}, agentboundary.Record{}, ErrAlreadyOpen
	}

	entry, ok, err := l.Next()
	if err != nil {
		return record.Attempt{}, agentboundary.Record{}, err
	}
	if !ok {
		return record.Attempt{}, agentboundary.Record{}, ErrCatalogExhausted
	}

	branch := "exercise/" + entry.ID
	playgroundDir := filepath.Join(checkoutDir, "Playground")
	if err := gitrepo.Clone(playgroundSrc, playgroundDir); err != nil {
		return record.Attempt{}, agentboundary.Record{}, fmt.Errorf("cloning the Playground: %w", err)
	}
	if err := gitrepo.CreateBranch(playgroundDir, branch, entry.Baseline); err != nil {
		return record.Attempt{}, agentboundary.Record{}, fmt.Errorf("cutting the fix branch: %w", err)
	}

	// The boundary digest record lives inside the progress record's own
	// directory — "outside the checkout" (ADR-0004) doesn't require its own
	// separate git repo, and putting it here means Record.Write's commit
	// captures the boundary digest and the Attempt together, in one entry
	// in the record's history.
	boundary := agentboundary.Boundary{
		CheckoutRoot: checkoutDir,
		RecordPath:   filepath.Join(l.Record.Dir, entry.ID+".boundary.json"),
	}
	armed, err := boundary.Arm()
	if err != nil {
		return record.Attempt{}, agentboundary.Record{}, fmt.Errorf("arming the Agent boundary: %w", err)
	}

	startedAt := time.Now().UTC()
	attempt := record.Attempt{
		ExerciseID: entry.ID,
		Ticket:     entry.Ticket,
		State:      record.StateOpen,
		StartedAt:  startedAt,
		Baseline:   entry.Baseline,
		Branch:     branch,
		Checkout:   checkoutDir,
		// The elapsed clock starts running immediately; Pause/ResumeClock
		// (issue #7) explicitly stop and restart it from here.
		ClockRunningSince: &startedAt,
	}
	if err := l.Record.Write(attempt); err != nil {
		return record.Attempt{}, agentboundary.Record{}, err
	}
	return attempt, armed, nil
}

// Status returns the open Attempt, if any.
func (l Loop) Status() (record.Attempt, bool, error) {
	return l.Record.Open()
}

// Submit grades the open Exercise's fix branch against grader and records
// the outcome. A Passed grade with an Intact Agent boundary records a Clear;
// a tampered or lifted boundary forecloses one even over a Passed grade.
// Every other outcome leaves the Exercise open, so submitting again costs
// nothing (issue #6: "submitting repeatedly costs nothing").
//
// Refuses with ErrNoExerciseOpen if no Exercise is open, and with
// ErrWrongBranch if the checkout has moved off the Exercise's fix branch —
// grading whatever tree happens to be checked out, rather than the fix
// branch a Clear is supposed to mean, is refused rather than silently done.
func (l Loop) Submit(grader Grader) (SubmitResult, error) {
	attempt, open, err := l.Record.Open()
	if err != nil {
		return SubmitResult{}, err
	}
	if !open {
		return SubmitResult{}, ErrNoExerciseOpen
	}

	playgroundDir := filepath.Join(attempt.Checkout, "Playground")
	branch, err := gitrepo.CurrentBranch(playgroundDir)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("checking the fix branch: %w", err)
	}
	if branch != attempt.Branch {
		return SubmitResult{}, fmt.Errorf("%w: on %q, want %q", ErrWrongBranch, branch, attempt.Branch)
	}

	boundary := agentboundary.Boundary{
		CheckoutRoot: attempt.Checkout,
		RecordPath:   filepath.Join(l.Record.Dir, attempt.ExerciseID+".boundary.json"),
	}
	status, err := boundary.Verify()
	if err != nil {
		return SubmitResult{}, fmt.Errorf("verifying the Agent boundary: %w", err)
	}

	verdict, err := grader.Grade(playgroundDir)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("grading the submission: %w", err)
	}

	cleared, err := l.Record.AppendSubmission(attempt.ExerciseID, record.Submission{
		Passed:   verdict.Passed,
		Boundary: status,
	})
	if err != nil {
		return SubmitResult{}, err
	}

	return SubmitResult{Passed: verdict.Passed, Cleared: cleared, Boundary: status}, nil
}

// Forfeit closes the open Exercise explicitly, recording it as not cleared
// (CONTEXT.md's Forfeit entry) — a dignified exit for being stuck, visible
// in the learner's own history rather than the Exercise sitting open
// indefinitely.
//
// Refuses with ErrNoExerciseOpen if no Exercise is open.
func (l Loop) Forfeit() (record.Attempt, error) {
	attempt, open, err := l.Record.Open()
	if err != nil {
		return record.Attempt{}, err
	}
	if !open {
		return record.Attempt{}, ErrNoExerciseOpen
	}

	if err := l.Record.Forfeit(attempt.ExerciseID); err != nil {
		return record.Attempt{}, err
	}
	attempt.State = record.StateForfeited
	return attempt, nil
}

// Debrief judges exerciseID's filed Location claims against debriefer and
// returns the intended path alongside them. Which of the two endings closed
// the Exercise makes no difference to what is revealed (CONTEXT.md's
// Debrief entry: it follows either one) — what does is state itself, and
// that gating is left to debriefer rather than checked here first, so it is
// enforced at the Vault boundary's own interface and not merely by this
// call site's care (issue #8's acceptance criteria).
func (l Loop) Debrief(exerciseID string, debriefer Debriefer) (DebriefResult, error) {
	attempt, err := l.Record.Get(exerciseID)
	if err != nil {
		return DebriefResult{}, err
	}

	entries, err := l.Record.PathLog(exerciseID)
	if err != nil {
		return DebriefResult{}, err
	}
	var claims []vault.Claim
	for _, e := range entries {
		if e.Location != "" {
			claims = append(claims, vault.Claim{ProbeIndex: e.ProbeIndex, Location: e.Location})
		}
	}

	debrief, err := debriefer.Debrief(attempt.State != record.StateOpen, claims)
	if err != nil {
		return DebriefResult{}, err
	}
	result := DebriefResult{Debrief: debrief}

	// The first correct claim by Probe index — Path log entries are already
	// filed in ascending Probe order, but Claims came back from the Vault in
	// whatever order it chose to answer them, so that ordering is not
	// assumed here.
	verdicts := slices.Clone(debrief.Claims)
	slices.SortFunc(verdicts, func(a, b vault.ClaimVerdict) int { return a.ProbeIndex - b.ProbeIndex })
	for _, c := range verdicts {
		if c.Band != vault.CorrectBand {
			continue
		}
		probes := c.ProbeIndex
		result.ProbesToLocate = &probes
		for _, e := range entries {
			if e.ProbeIndex == c.ProbeIndex {
				// FiledAt round-trips through the Attempt file at second
				// precision (renderEntry's RFC3339 timestamp), while
				// StartedAt keeps its original sub-second value — so an
				// entry filed within the same second Start recorded can
				// parse back a hair earlier. Clamped to zero rather than
				// reported as a small negative interval, which reads only
				// as "found immediately".
				elapsed := max(e.FiledAt.Sub(attempt.StartedAt), 0)
				result.TimeToLocate = &elapsed
				break
			}
		}
		break
	}

	return result, nil
}
