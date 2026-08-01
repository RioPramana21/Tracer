package exercise

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RioPramana21/Tracer/internal/catalog"
	"github.com/RioPramana21/Tracer/internal/record"
	"github.com/RioPramana21/Tracer/internal/vault"
)

// Submit's logic — the Clear decision, the boundary check, that a failed or
// boundary-lifted submission costs nothing — is tested here against a fake
// Grader (issue #1's testing decisions: "every other test substitutes a fake
// at the Vault boundary"). The real vault.Client is exercised only by
// internal/vault's slow, Docker-gated pipeline test and by the one real
// end-to-end CLI test.

type fakeGrader struct {
	passed bool
}

func (f fakeGrader) Grade(tree string) (vault.Verdict, error) {
	return vault.Verdict{Passed: f.passed}, nil
}

// fakeDebriefer records the closed flag and claims it was called with, and
// returns a canned Debrief. Real gating — refusing an open Exercise before
// Docker ever runs — is vault.Client's own job, covered by
// internal/vault's tests; this fake exists to prove exercise.Loop.Debrief
// passes the right closed flag and claims through to it (issue #1's
// testing decisions: every other test substitutes a fake at the Vault
// boundary).
type fakeDebriefer struct {
	gotClosed bool
	gotClaims []vault.Claim
	// verdicts, if set, is returned verbatim as the Debrief's Claims —
	// letting a test control which claim (if any) the Vault judges Correct
	// (issue #8: Probes/Time to locate are Tracer's own count over that
	// judgement, not the Vault's).
	verdicts []vault.ClaimVerdict
}

func (f *fakeDebriefer) Debrief(closed bool, claims []vault.Claim) (vault.Debrief, error) {
	f.gotClosed = closed
	f.gotClaims = claims
	if !closed {
		return vault.Debrief{}, vault.ErrExerciseOpen
	}
	return vault.Debrief{IntendedPath: "the intended path", TicketSignals: "the signal", Claims: f.verdicts}, nil
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func initPlayground(t *testing.T) (repo, baseline string) {
	t.Helper()
	repo = t.TempDir()
	runGit(t, repo, "init")
	if err := os.WriteFile(filepath.Join(repo, "app.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "user.name=fixture", "-c", "user.email=fixture@localhost",
		"commit", "-m", "Initial commit")
	baseline = strings.TrimSpace(runGitOutput(t, repo, "rev-parse", "HEAD"))
	return repo, baseline
}

func writeCatalog(t *testing.T, entries ...catalog.Entry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.json")
	doc := catalog.Catalog{Entries: entries}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// startedLoop starts one Exercise against a fixture Playground and returns
// the Loop and the started Attempt.
func startedLoop(t *testing.T) (Loop, record.Attempt) {
	t.Helper()
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t, catalog.Entry{
		ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline,
	})
	recordDir := filepath.Join(t.TempDir(), "record")
	checkoutDir := filepath.Join(t.TempDir(), "checkout")

	loop, err := Load(catalogPath, recordDir)
	if err != nil {
		t.Fatal(err)
	}
	attempt, _, err := loop.Start(playground, checkoutDir)
	if err != nil {
		t.Fatal(err)
	}
	return loop, attempt
}

func TestSubmitPassingWithIntactBoundaryRecordsAClear(t *testing.T) {
	loop, _ := startedLoop(t)

	result, err := loop.Submit(fakeGrader{passed: true})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if !result.Passed {
		t.Error("result.Passed = false, want true")
	}
	if !result.Cleared {
		t.Error("result.Cleared = false, want true — a Passed grade with an intact boundary should Clear")
	}

	if _, open, err := loop.Record.Open(); err != nil {
		t.Fatal(err)
	} else if open {
		t.Error("an Exercise is still reported open after a Clear")
	}
}

func TestSubmitFailingHiddenTestLeavesTheExerciseOpen(t *testing.T) {
	loop, _ := startedLoop(t)

	result, err := loop.Submit(fakeGrader{passed: false})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if result.Passed {
		t.Error("result.Passed = true, want false")
	}
	if result.Cleared {
		t.Error("result.Cleared = true, want false — a failing grade must never Clear")
	}

	if _, open, err := loop.Record.Open(); err != nil {
		t.Fatal(err)
	} else if !open {
		t.Error("a failed submission closed the Exercise — it should stay open for another try")
	}
}

func TestSubmitRepeatedlyCostsNothing(t *testing.T) {
	loop, _ := startedLoop(t)

	if _, err := loop.Submit(fakeGrader{passed: false}); err != nil {
		t.Fatalf("first Submit: %v", err)
	}
	if _, err := loop.Submit(fakeGrader{passed: false}); err != nil {
		t.Fatalf("second Submit: %v", err)
	}
	result, err := loop.Submit(fakeGrader{passed: true})
	if err != nil {
		t.Fatalf("third Submit: %v", err)
	}

	if !result.Cleared {
		t.Error("a Passed submission after two failures did not Clear")
	}
}

func TestSubmitWithALiftedBoundaryForecloseAClearEvenOverAPass(t *testing.T) {
	loop, attempt := startedLoop(t)

	// Tamper with the armed settings the same way a learner lifting the
	// boundary would — editing the deny rules the digest was taken over.
	settingsPath := filepath.Join(attempt.Checkout, ".claude", "settings.local.json")
	if err := os.WriteFile(settingsPath, []byte(`{"permissions":{"deny":["Read(/nothing/**)"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := loop.Submit(fakeGrader{passed: true})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if result.Cleared {
		t.Error("result.Cleared = true, want false — a lifted boundary must foreclose a Clear")
	}
	if !result.Passed {
		t.Error("result.Passed = false, want true — the hidden test itself passed")
	}

	if _, open, err := loop.Record.Open(); err != nil {
		t.Fatal(err)
	} else if !open {
		t.Error("a boundary-lifted submission closed the Exercise — it should stay open")
	}
}

// attemptClockFields reads the clock fields straight out of the Attempt
// file's front matter — record.Store has no getter for a closed Attempt
// (Open only returns one in state Open), and this is the one place that
// needs to see behind the Clear (issue #7: the recorded interval must stop
// growing once the Exercise closes).
func attemptClockFields(t *testing.T, recordDir, exerciseID string) (workedNanos int64, running bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(recordDir, exerciseID+".md"))
	if err != nil {
		t.Fatalf("reading attempt file: %v", err)
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || lines[0] != "---" {
		t.Fatalf("attempt file does not start with front matter: %q", content)
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		t.Fatalf("attempt file front matter has no closing ---: %q", content)
	}
	var fm struct {
		ClockWorked       int64   `json:"clock_worked"`
		ClockRunningSince *string `json:"clock_running_since"`
	}
	if err := json.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fm); err != nil {
		t.Fatalf("attempt front matter is not valid JSON: %v", err)
	}
	return fm.ClockWorked, fm.ClockRunningSince != nil
}

func TestSubmitClearingStopsTheElapsedClock(t *testing.T) {
	loop, attempt := startedLoop(t)

	if _, running := attemptClockFields(t, loop.Record.Dir, attempt.ExerciseID); !running {
		t.Fatal("sanity check failed: the clock is not recorded as running right after Start")
	}

	if _, err := loop.Submit(fakeGrader{passed: true}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	worked, running := attemptClockFields(t, loop.Record.Dir, attempt.ExerciseID)
	if running {
		t.Error("the clock is still recorded as running after a Clear")
	}
	if worked <= 0 {
		t.Error("no worked interval was recorded across the Clear")
	}
}

func TestSubmitRefusesWhenNoExerciseIsOpen(t *testing.T) {
	loop := Loop{Record: record.Store{Dir: filepath.Join(t.TempDir(), "record")}}

	_, err := loop.Submit(fakeGrader{passed: true})

	if !errors.Is(err, ErrNoExerciseOpen) {
		t.Errorf("Submit err = %v, want ErrNoExerciseOpen", err)
	}
}

func TestSubmitRefusesWhenTheCheckoutIsNotOnTheFixBranch(t *testing.T) {
	loop, attempt := startedLoop(t)
	playgroundDir := filepath.Join(attempt.Checkout, "Playground")
	runGit(t, playgroundDir, "checkout", attempt.Baseline)

	_, err := loop.Submit(fakeGrader{passed: true})

	if !errors.Is(err, ErrWrongBranch) {
		t.Errorf("Submit err = %v, want ErrWrongBranch", err)
	}
	if _, open, err := loop.Record.Open(); err != nil {
		t.Fatal(err)
	} else if !open {
		t.Error("a wrong-branch submission closed the Exercise — it should stay open")
	}
}

func TestForfeitClosesTheOpenExerciseAsNotCleared(t *testing.T) {
	loop, attempt := startedLoop(t)

	forfeited, err := loop.Forfeit()
	if err != nil {
		t.Fatalf("Forfeit: %v", err)
	}
	if forfeited.ExerciseID != attempt.ExerciseID {
		t.Errorf("Forfeit closed %q, want %q", forfeited.ExerciseID, attempt.ExerciseID)
	}
	if forfeited.State != record.StateForfeited {
		t.Errorf("Forfeit left State %q, want %q", forfeited.State, record.StateForfeited)
	}

	if _, open, err := loop.Record.Open(); err != nil {
		t.Fatal(err)
	} else if open {
		t.Error("an Exercise is still reported open after a Forfeit")
	}

	closed, err := loop.Record.Get(attempt.ExerciseID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.State != record.StateForfeited {
		t.Errorf("the recorded Attempt State is %q, want %q", closed.State, record.StateForfeited)
	}
}

func TestForfeitStopsTheElapsedClock(t *testing.T) {
	loop, attempt := startedLoop(t)

	if _, err := loop.Forfeit(); err != nil {
		t.Fatalf("Forfeit: %v", err)
	}

	worked, running := attemptClockFields(t, loop.Record.Dir, attempt.ExerciseID)
	if running {
		t.Error("the clock is still recorded as running after a Forfeit")
	}
	if worked <= 0 {
		t.Error("no worked interval was recorded across the Forfeit")
	}
}

func TestForfeitRefusesWhenNoExerciseIsOpen(t *testing.T) {
	loop := Loop{Record: record.Store{Dir: filepath.Join(t.TempDir(), "record")}}

	_, err := loop.Forfeit()

	if !errors.Is(err, ErrNoExerciseOpen) {
		t.Errorf("Forfeit err = %v, want ErrNoExerciseOpen", err)
	}
}

func TestDebriefPassesTheClosedStateAndFiledClaimsToTheDebriefer(t *testing.T) {
	loop, attempt := startedLoop(t)
	if _, err := loop.Record.AppendEntry(attempt.ExerciseID, record.PathLogEntry{
		Hypothesis: "a guess", Because: "a reason", Location: "internal/billing/retry",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Record.AppendEntry(attempt.ExerciseID, record.PathLogEntry{
		Hypothesis: "no claim here", Because: "just a hunch",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Forfeit(); err != nil {
		t.Fatalf("Forfeit: %v", err)
	}

	debriefer := &fakeDebriefer{}
	if _, err := loop.Debrief(attempt.ExerciseID, debriefer); err != nil {
		t.Fatalf("Debrief: %v", err)
	}

	if !debriefer.gotClosed {
		t.Error("Debrief passed closed=false for a Forfeited Exercise")
	}
	if len(debriefer.gotClaims) != 1 {
		t.Fatalf("Debrief passed %d claims, want 1 (only the entry carrying a Location claim)", len(debriefer.gotClaims))
	}
	if debriefer.gotClaims[0].Location != "internal/billing/retry" {
		t.Errorf("claim location = %q, want %q", debriefer.gotClaims[0].Location, "internal/billing/retry")
	}
	if debriefer.gotClaims[0].ProbeIndex != 1 {
		t.Errorf("claim probe index = %d, want 1", debriefer.gotClaims[0].ProbeIndex)
	}
}

func TestDebriefPassesClosedFalseForAnOpenExercise(t *testing.T) {
	loop, attempt := startedLoop(t)

	debriefer := &fakeDebriefer{}
	_, err := loop.Debrief(attempt.ExerciseID, debriefer)

	if !errors.Is(err, vault.ErrExerciseOpen) {
		t.Errorf("Debrief err = %v, want vault.ErrExerciseOpen — the refusal is the Debriefer's own, not a check Loop.Debrief short-circuits before calling it", err)
	}
	if debriefer.gotClosed {
		t.Error("Debrief passed closed=true for an open Exercise")
	}
}

func TestDebriefAfterAClearReportsClosedTrue(t *testing.T) {
	loop, attempt := startedLoop(t)
	if _, err := loop.Submit(fakeGrader{passed: true}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	debriefer := &fakeDebriefer{}
	if _, err := loop.Debrief(attempt.ExerciseID, debriefer); err != nil {
		t.Fatalf("Debrief: %v", err)
	}
	if !debriefer.gotClosed {
		t.Error("Debrief passed closed=false for a Cleared Exercise")
	}
}

func TestDebriefReportsProbesAndTimeToLocateAgainstTheFirstCorrectClaim(t *testing.T) {
	loop, attempt := startedLoop(t)
	if _, err := loop.Record.AppendEntry(attempt.ExerciseID, record.PathLogEntry{
		Hypothesis: "first guess", Because: "a reason", Location: "wrong/place",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Record.AppendEntry(attempt.ExerciseID, record.PathLogEntry{
		Hypothesis: "second guess", Because: "a better reason", Location: "internal/billing/retry",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Forfeit(); err != nil {
		t.Fatalf("Forfeit: %v", err)
	}

	debriefer := &fakeDebriefer{verdicts: []vault.ClaimVerdict{
		{ProbeIndex: 1, Location: "wrong/place", Band: "adjacent-call"},
		{ProbeIndex: 2, Location: "internal/billing/retry", Band: vault.CorrectBand},
	}}
	result, err := loop.Debrief(attempt.ExerciseID, debriefer)
	if err != nil {
		t.Fatalf("Debrief: %v", err)
	}

	if result.ProbesToLocate == nil {
		t.Fatal("ProbesToLocate is nil, want the Probe index of the first correct claim")
	}
	if *result.ProbesToLocate != 2 {
		t.Errorf("ProbesToLocate = %d, want 2", *result.ProbesToLocate)
	}
	if result.TimeToLocate == nil {
		t.Fatal("TimeToLocate is nil, want the interval to the first correct claim")
	}
	if *result.TimeToLocate < 0 {
		t.Errorf("TimeToLocate = %s, want non-negative", *result.TimeToLocate)
	}
}

func TestReplayResetsTheFixBranchToTheBaseline(t *testing.T) {
	loop, attempt := startedLoop(t)
	playgroundDir := filepath.Join(attempt.Checkout, "Playground")
	if err := os.WriteFile(filepath.Join(playgroundDir, "app.go"), []byte("package main\n\n// a fix attempt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, playgroundDir, "add", "-A")
	runGit(t, playgroundDir, "-c", "user.name=fixture", "-c", "user.email=fixture@localhost",
		"commit", "-m", "an attempted fix")
	if _, err := loop.Forfeit(); err != nil {
		t.Fatalf("Forfeit: %v", err)
	}

	replay, err := loop.Replay(attempt.ExerciseID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	head := strings.TrimSpace(runGitOutput(t, playgroundDir, "rev-parse", "HEAD"))
	if head != attempt.Baseline {
		t.Errorf("fix branch HEAD after Replay is %q, want the baseline %q", head, attempt.Baseline)
	}
	branch := strings.TrimSpace(runGitOutput(t, playgroundDir, "rev-parse", "--abbrev-ref", "HEAD"))
	if branch != attempt.Branch {
		t.Errorf("Replay left the checkout on branch %q, want %q", branch, attempt.Branch)
	}
	if replay.State != record.StateReplaying {
		t.Errorf("Replay attempt State = %q, want %q", replay.State, record.StateReplaying)
	}
}

func TestReplayNeverTouchesTheForfeitedAttemptsOwnFile(t *testing.T) {
	loop, attempt := startedLoop(t)
	if _, err := loop.Record.AppendEntry(attempt.ExerciseID, record.PathLogEntry{
		Hypothesis: "a guess", Because: "a reason", Location: "internal/billing/retry",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Forfeit(); err != nil {
		t.Fatalf("Forfeit: %v", err)
	}
	beforeReplay, err := loop.Record.Get(attempt.ExerciseID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := loop.Replay(attempt.ExerciseID); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	afterReplay, err := loop.Record.Get(attempt.ExerciseID)
	if err != nil {
		t.Fatal(err)
	}
	if afterReplay.State != record.StateForfeited {
		t.Errorf("the Forfeited attempt's own State changed to %q after a Replay", afterReplay.State)
	}
	if afterReplay != beforeReplay {
		t.Errorf("Replay altered the Forfeited attempt: before %+v, after %+v", beforeReplay, afterReplay)
	}

	debriefer := &fakeDebriefer{}
	if _, err := loop.Debrief(attempt.ExerciseID, debriefer); err != nil {
		t.Fatalf("Debrief: %v", err)
	}
	if len(debriefer.gotClaims) != 1 {
		t.Errorf("the Forfeited attempt's Debrief sees %d claims after a Replay, want 1 — unchanged", len(debriefer.gotClaims))
	}
}

func TestReplaySubmissionNeverRecordsAClearEvenWhenPassed(t *testing.T) {
	loop, attempt := startedLoop(t)
	if _, err := loop.Forfeit(); err != nil {
		t.Fatalf("Forfeit: %v", err)
	}
	if _, err := loop.Replay(attempt.ExerciseID); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	result, err := loop.Submit(fakeGrader{passed: true})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if !result.Passed {
		t.Error("result.Passed = false, want true — the hidden test itself passed")
	}
	if result.Cleared {
		t.Error("result.Cleared = true, want false — a Replay must never record a Clear")
	}
}

func TestReplayProducesNoProbesToLocateBecausePathLogRefusesDuringIt(t *testing.T) {
	loop, attempt := startedLoop(t)
	if _, err := loop.Forfeit(); err != nil {
		t.Fatalf("Forfeit: %v", err)
	}
	if _, err := loop.Replay(attempt.ExerciseID); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	// Store.Open only ever returns a StateOpen Attempt; a Replay's is
	// StateReplaying, so there is structurally no Attempt to file a Path log
	// entry — and therefore a Location claim — against while replaying
	// (issue #9: "there is no search once the answer is known").
	if _, open, err := loop.Record.Open(); err != nil {
		t.Fatal(err)
	} else if open {
		t.Error("Store.Open reports an open Attempt during a Replay — Path log filing should be structurally unavailable")
	}
}

func TestReplayRefusesWhenTheExerciseWasNotForfeited(t *testing.T) {
	loop, attempt := startedLoop(t)
	if _, err := loop.Submit(fakeGrader{passed: true}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	_, err := loop.Replay(attempt.ExerciseID)

	if !errors.Is(err, record.ErrNotForfeited) {
		t.Errorf("Replay err = %v, want record.ErrNotForfeited", err)
	}
}

func TestReplayRefusesWhenAnExerciseIsAlreadyActive(t *testing.T) {
	loop, attempt := startedLoop(t)
	if _, err := loop.Forfeit(); err != nil {
		t.Fatalf("Forfeit: %v", err)
	}
	if _, err := loop.Replay(attempt.ExerciseID); err != nil {
		t.Fatalf("first Replay: %v", err)
	}

	_, err := loop.Replay(attempt.ExerciseID)

	if !errors.Is(err, ErrAlreadyOpen) {
		t.Errorf("second Replay err = %v, want ErrAlreadyOpen", err)
	}
}

func TestStartRefusesWhileAReplayIsActive(t *testing.T) {
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t,
		catalog.Entry{ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline},
		catalog.Entry{ID: "bug-002", Ticket: "The dashboard totals stop updating after lunchtime.", Baseline: baseline},
	)
	recordDir := filepath.Join(t.TempDir(), "record")
	checkoutDir := filepath.Join(t.TempDir(), "checkout")
	loop, err := Load(catalogPath, recordDir)
	if err != nil {
		t.Fatal(err)
	}
	attempt, _, err := loop.Start(playground, checkoutDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Forfeit(); err != nil {
		t.Fatalf("Forfeit: %v", err)
	}
	if _, err := loop.Replay(attempt.ExerciseID); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	_, _, err = loop.Start(playground, filepath.Join(t.TempDir(), "checkout2"))

	if !errors.Is(err, ErrAlreadyOpen) {
		t.Errorf("Start err = %v, want ErrAlreadyOpen while a Replay is active", err)
	}
}

func TestDebriefReportsNoProbesOrTimeToLocateWithoutACorrectClaim(t *testing.T) {
	loop, attempt := startedLoop(t)
	if _, err := loop.Record.AppendEntry(attempt.ExerciseID, record.PathLogEntry{
		Hypothesis: "a guess", Because: "a reason", Location: "wrong/place",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Submit(fakeGrader{passed: true}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	debriefer := &fakeDebriefer{verdicts: []vault.ClaimVerdict{
		{ProbeIndex: 1, Location: "wrong/place", Band: "adjacent-call"},
	}}
	result, err := loop.Debrief(attempt.ExerciseID, debriefer)
	if err != nil {
		t.Fatalf("Debrief: %v", err)
	}

	if result.ProbesToLocate != nil {
		t.Errorf("ProbesToLocate = %d, want nil — no claim was Correct", *result.ProbesToLocate)
	}
	if result.TimeToLocate != nil {
		t.Errorf("TimeToLocate = %s, want nil — no claim was Correct", *result.TimeToLocate)
	}
}
