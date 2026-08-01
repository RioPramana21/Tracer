// Package record is the progress record store: plain files, one per Exercise
// Attempt, in a directory that is itself a git repository (ADR-0004,
// CONTEXT.md). It shares no store, schema or connection with the Playground
// (STD-010) — the only thing that reaches this package is the path the
// caller supplies, and that path is never the Playground's.
package record

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RioPramana21/Tracer/internal/agentboundary"
	"github.com/RioPramana21/Tracer/internal/gitrepo"
)

// State is where an Attempt stands. Replay (#9) adds the rest.
type State string

// StateOpen means the Exercise is being worked and has not yet closed.
const StateOpen State = "open"

// StateCleared means a Submission passed the hidden test with the Agent
// boundary intact.
const StateCleared State = "cleared"

// StateForfeited means the learner closed the Exercise explicitly without a
// Clear (CONTEXT.md's Forfeit entry) — recorded as not cleared, visible in
// the learner's own history rather than left open indefinitely.
const StateForfeited State = "forfeited"

// Attempt is one worked instance of an Exercise: the front matter of one
// progress-record file. The prose body below the front matter carries its
// Path log.
type Attempt struct {
	ExerciseID string    `json:"exercise_id"`
	Ticket     string    `json:"ticket"`
	State      State     `json:"state"`
	StartedAt  time.Time `json:"started_at"`
	Baseline   string    `json:"baseline"`
	Branch     string    `json:"branch"`
	Checkout   string    `json:"checkout"`
	// ClockRunningSince is when the elapsed clock most recently started
	// running; nil while paused. Never shown in any output while the
	// Exercise is open (issue #7) — it exists only so Pause/ResumeClock can
	// fold the running interval into ClockWorked.
	ClockRunningSince *time.Time `json:"clock_running_since,omitempty"`
	// ClockWorked is the accumulated elapsed time from every finished worked
	// interval; the interval currently running, if any, is not yet folded
	// in. Recorded for later reporting (Time to locate), never displayed
	// while the Exercise is open.
	ClockWorked time.Duration `json:"clock_worked"`
}

// PathLogEntry is one filed hypothesis: what is believed and why, and
// optionally where the Cause is believed to live. ProbeIndex and FiledAt are
// assigned by Store.AppendEntry at the moment the entry is filed, not by the
// caller — the Probe index is the entry's position among every entry already
// on file for the Attempt, so it cannot be chosen or predicted ahead of time.
type PathLogEntry struct {
	ProbeIndex int
	FiledAt    time.Time
	Hypothesis string
	Because    string
	// Location is the Location claim's text — where the Cause is believed to
	// live. Empty means the entry carries no claim.
	Location string
}

// Submission is one graded attempt against the Exercise's hidden test.
// SubmittedAt is assigned by Store.AppendSubmission at the moment it is
// filed, not by the caller.
type Submission struct {
	SubmittedAt time.Time
	// Passed is the Vault boundary's Verdict — the hidden test's result and
	// nothing else (STD-009).
	Passed bool
	// Boundary is the Agent boundary's status at submission time. Only
	// agentboundary.Intact clears; Lifted or Unarmed is recorded here
	// verbatim rather than collapsed to a bool, so a tampered boundary is
	// legible in the Attempt file itself and not only in the boundary's own
	// digest record.
	Boundary agentboundary.Status
}

// Store manages the progress record at Dir.
type Store struct {
	Dir string
}

// ErrBecauseRequired is returned by AppendEntry when e carries no "because".
// The Path log's discipline — no probe without a written belief — is
// enforced here rather than left to the caller's care.
var ErrBecauseRequired = errors.New(`a Path log entry requires a "because"`)

// ErrClockAlreadyPaused is returned by PauseClock when the Attempt's elapsed
// clock is not currently running.
var ErrClockAlreadyPaused = errors.New("the Exercise clock is already paused")

// ErrClockAlreadyRunning is returned by ResumeClock when the Attempt's
// elapsed clock is already running.
var ErrClockAlreadyRunning = errors.New("the Exercise clock is already running")

func (s Store) path(id string) string {
	return filepath.Join(s.Dir, id+".md")
}

// Used reports the Exercise ids with an Attempt on file, regardless of how
// that Attempt ended.
func (s Store) Used() (map[string]bool, error) {
	attempts, err := s.all()
	if err != nil {
		return nil, err
	}
	used := make(map[string]bool, len(attempts))
	for _, a := range attempts {
		used[a.ExerciseID] = true
	}
	return used, nil
}

// Open returns the one Attempt in state Open, if any.
func (s Store) Open() (Attempt, bool, error) {
	attempts, err := s.all()
	if err != nil {
		return Attempt{}, false, err
	}
	for _, a := range attempts {
		if a.State == StateOpen {
			return a, true, nil
		}
	}
	return Attempt{}, false, nil
}

// Get returns the Attempt for exerciseID regardless of its State — the one
// way to reach a closed Attempt, since Open only ever returns one in state
// Open.
func (s Store) Get(exerciseID string) (Attempt, error) {
	raw, err := os.ReadFile(s.path(exerciseID))
	if err != nil {
		return Attempt{}, fmt.Errorf("reading attempt %s: %w", exerciseID, err)
	}
	return parseAttempt(raw)
}

// Write records a, creating the record's own git repository on first use and
// committing the write. ADR-0004: the record is versioned, so editing it is
// possible but never invisible.
func (s Store) Write(a Attempt) error {
	if err := gitrepo.EnsureRepo(s.Dir); err != nil {
		return err
	}
	encoded, err := encodeAttempt(a)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path(a.ExerciseID), encoded, 0o644); err != nil {
		return fmt.Errorf("writing attempt: %w", err)
	}
	return gitrepo.CommitAll(s.Dir, fmt.Sprintf("Attempt %s: %s", a.ExerciseID, a.State))
}

// AppendEntry files e against the Attempt exerciseID: it is assigned the
// next Probe index and the current time, appended to the Attempt file's
// prose body as its Path log, and committed. Returns the assigned Probe
// index.
//
// Refuses with ErrBecauseRequired, and appends nothing, if e carries no
// "because" — the one check that is enforced rather than merely encouraged
// (CONTEXT.md's Path log).
func (s Store) AppendEntry(exerciseID string, e PathLogEntry) (int, error) {
	if strings.TrimSpace(e.Because) == "" {
		return 0, ErrBecauseRequired
	}

	raw, err := os.ReadFile(s.path(exerciseID))
	if err != nil {
		return 0, fmt.Errorf("reading attempt %s: %w", exerciseID, err)
	}
	frontMatter, body, err := splitFrontMatter(raw)
	if err != nil {
		return 0, fmt.Errorf("attempt file for %s: %w", exerciseID, err)
	}

	// Counts renderEntry's own heading text — the two must be changed
	// together, since this is what makes an existing entry findable at all.
	e.ProbeIndex = strings.Count(body, "\n### Probe ") + 1
	e.FiledAt = time.Now().UTC()

	if !strings.Contains(body, "## Path log") {
		body += "\n## Path log\n"
	}
	body += "\n" + renderEntry(e)

	if err := os.WriteFile(s.path(exerciseID), []byte(frontMatter+body), 0o644); err != nil {
		return 0, fmt.Errorf("writing attempt %s: %w", exerciseID, err)
	}
	if err := gitrepo.CommitAll(s.Dir, fmt.Sprintf("Attempt %s: Path log probe %d", exerciseID, e.ProbeIndex)); err != nil {
		return 0, err
	}
	return e.ProbeIndex, nil
}

// PathLog returns the Path log entries filed against the Attempt exerciseID,
// parsed back out of its rendered prose body — the inverse of renderEntry.
// A Debrief needs each entry's Probe index, filed timestamp and Location
// claim: the timestamp is what Time to locate measures against (issue #8).
// Hypothesis and Because are parsed too, for anything that later wants to
// show the Path log back to the learner.
func (s Store) PathLog(exerciseID string) ([]PathLogEntry, error) {
	raw, err := os.ReadFile(s.path(exerciseID))
	if err != nil {
		return nil, fmt.Errorf("reading attempt %s: %w", exerciseID, err)
	}
	_, body, err := splitFrontMatter(raw)
	if err != nil {
		return nil, fmt.Errorf("attempt file for %s: %w", exerciseID, err)
	}

	var entries []PathLogEntry
	var current *PathLogEntry
	for line := range strings.SplitSeq(body, "\n") {
		if rest, ok := strings.CutPrefix(line, "### Probe "); ok {
			if current != nil {
				entries = append(entries, *current)
			}
			index, timestamp, _ := strings.Cut(rest, " — ")
			var probe PathLogEntry
			fmt.Sscanf(index, "%d", &probe.ProbeIndex)
			if filedAt, err := time.Parse(time.RFC3339, timestamp); err == nil {
				probe.FiledAt = filedAt
			}
			current = &probe
			continue
		}
		if current == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Hypothesis: "):
			current.Hypothesis = strings.TrimPrefix(line, "Hypothesis: ")
		case strings.HasPrefix(line, "Because: "):
			current.Because = strings.TrimPrefix(line, "Because: ")
		case strings.HasPrefix(line, "Location claim: "):
			current.Location = strings.TrimPrefix(line, "Location claim: ")
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}
	return entries, nil
}

// PauseClock stops the elapsed clock on the Attempt exerciseID, folding the
// interval since it last started running into ClockWorked so pausing and
// resuming across separate invocations accumulates only worked intervals
// (issue #7).
//
// Refuses with ErrClockAlreadyPaused if the clock is not currently running.
func (s Store) PauseClock(exerciseID string) error {
	return s.updateClock(exerciseID, "paused", func(a *Attempt, now time.Time) error {
		if a.ClockRunningSince == nil {
			return ErrClockAlreadyPaused
		}
		a.ClockWorked += now.Sub(*a.ClockRunningSince)
		a.ClockRunningSince = nil
		return nil
	})
}

// ResumeClock restarts the elapsed clock on the Attempt exerciseID.
//
// Refuses with ErrClockAlreadyRunning if the clock is already running.
func (s Store) ResumeClock(exerciseID string) error {
	return s.updateClock(exerciseID, "resumed", func(a *Attempt, now time.Time) error {
		if a.ClockRunningSince != nil {
			return ErrClockAlreadyRunning
		}
		a.ClockRunningSince = &now
		return nil
	})
}

// updateClock reads the Attempt exerciseID, applies mutate, and writes and
// commits the result. mutate's error — ErrClockAlreadyPaused or
// ErrClockAlreadyRunning — is returned unchanged and nothing is written, the
// same refuse-before-writing shape AppendEntry follows for
// ErrBecauseRequired.
func (s Store) updateClock(exerciseID, verb string, mutate func(a *Attempt, now time.Time) error) error {
	raw, err := os.ReadFile(s.path(exerciseID))
	if err != nil {
		return fmt.Errorf("reading attempt %s: %w", exerciseID, err)
	}
	attempt, err := parseAttempt(raw)
	if err != nil {
		return fmt.Errorf("attempt file for %s: %w", exerciseID, err)
	}
	_, body, err := splitFrontMatter(raw)
	if err != nil {
		return fmt.Errorf("attempt file for %s: %w", exerciseID, err)
	}

	if err := mutate(&attempt, time.Now().UTC()); err != nil {
		return err
	}

	frontMatter, err := encodeAttempt(attempt)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path(exerciseID), append(frontMatter, []byte(body)...), 0o644); err != nil {
		return fmt.Errorf("writing attempt %s: %w", exerciseID, err)
	}
	return gitrepo.CommitAll(s.Dir, fmt.Sprintf("Attempt %s: clock %s", exerciseID, verb))
}

// AppendSubmission files sub against the Attempt exerciseID: appended to the
// Attempt file's prose body as a record of the graded attempt, and
// committed. A Passed submission with an Intact boundary transitions the
// Attempt to Cleared and reports cleared as true; any other outcome leaves
// the Attempt Open and reports false, so a failed or boundary-lifted
// submission never forecloses trying again (issue #6: "submitting
// repeatedly costs nothing").
func (s Store) AppendSubmission(exerciseID string, sub Submission) (cleared bool, err error) {
	raw, err := os.ReadFile(s.path(exerciseID))
	if err != nil {
		return false, fmt.Errorf("reading attempt %s: %w", exerciseID, err)
	}
	attempt, err := parseAttempt(raw)
	if err != nil {
		return false, fmt.Errorf("attempt file for %s: %w", exerciseID, err)
	}
	_, body, err := splitFrontMatter(raw)
	if err != nil {
		return false, fmt.Errorf("attempt file for %s: %w", exerciseID, err)
	}

	sub.SubmittedAt = time.Now().UTC()
	if !strings.Contains(body, "## Submissions") {
		body += "\n## Submissions\n"
	}
	body += "\n" + renderSubmission(sub)

	cleared = sub.Passed && sub.Boundary == agentboundary.Intact
	if cleared {
		attempt.State = StateCleared
		// The Exercise is closing, so the clock stops with it — a Cleared
		// Attempt's recorded interval must not keep growing after the fact.
		if attempt.ClockRunningSince != nil {
			attempt.ClockWorked += sub.SubmittedAt.Sub(*attempt.ClockRunningSince)
			attempt.ClockRunningSince = nil
		}
	}

	frontMatter, err := encodeAttempt(attempt)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(s.path(exerciseID), append(frontMatter, []byte(body)...), 0o644); err != nil {
		return false, fmt.Errorf("writing attempt %s: %w", exerciseID, err)
	}

	message := fmt.Sprintf("Attempt %s: submission failed", exerciseID)
	if cleared {
		message = fmt.Sprintf("Attempt %s: Cleared", exerciseID)
	}
	if err := gitrepo.CommitAll(s.Dir, message); err != nil {
		return false, err
	}
	return cleared, nil
}

// Forfeit closes the open Attempt exerciseID explicitly, recording it as
// StateForfeited rather than StateCleared — visible in the learner's own
// history as not cleared (CONTEXT.md's Forfeit entry). The elapsed clock
// stops the same way a Clear stops it (issue #7): a closed Attempt's
// recorded interval must not keep growing after the fact.
func (s Store) Forfeit(exerciseID string) error {
	raw, err := os.ReadFile(s.path(exerciseID))
	if err != nil {
		return fmt.Errorf("reading attempt %s: %w", exerciseID, err)
	}
	attempt, err := parseAttempt(raw)
	if err != nil {
		return fmt.Errorf("attempt file for %s: %w", exerciseID, err)
	}
	_, body, err := splitFrontMatter(raw)
	if err != nil {
		return fmt.Errorf("attempt file for %s: %w", exerciseID, err)
	}

	now := time.Now().UTC()
	attempt.State = StateForfeited
	if attempt.ClockRunningSince != nil {
		attempt.ClockWorked += now.Sub(*attempt.ClockRunningSince)
		attempt.ClockRunningSince = nil
	}

	frontMatter, err := encodeAttempt(attempt)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path(exerciseID), append(frontMatter, []byte(body)...), 0o644); err != nil {
		return fmt.Errorf("writing attempt %s: %w", exerciseID, err)
	}
	return gitrepo.CommitAll(s.Dir, fmt.Sprintf("Attempt %s: Forfeited", exerciseID))
}

// renderSubmission renders sub for the Attempt file's prose body. Carries
// only pass/fail and boundary status — never assertion text, a test name, a
// diff or a path (STD-009).
func renderSubmission(sub Submission) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "### Submission — %s\n", sub.SubmittedAt.Format(time.RFC3339))
	fmt.Fprintf(&buf, "Passed: %t\n", sub.Passed)
	fmt.Fprintf(&buf, "Agent boundary: %s\n", sub.Boundary)
	return buf.String()
}

// renderEntry's heading format is load-bearing: AppendEntry counts it back
// out of the body to assign the next Probe index. Change the two together.
func renderEntry(e PathLogEntry) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "### Probe %d — %s\n", e.ProbeIndex, e.FiledAt.Format(time.RFC3339))
	fmt.Fprintf(&buf, "Hypothesis: %s\n", e.Hypothesis)
	fmt.Fprintf(&buf, "Because: %s\n", e.Because)
	if e.Location != "" {
		fmt.Fprintf(&buf, "Location claim: %s\n", e.Location)
	}
	return buf.String()
}

func (s Store) all() ([]Attempt, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading progress record: %w", err)
	}
	var attempts []Attempt
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.Dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading attempt %s: %w", entry.Name(), err)
		}
		attempt, err := parseAttempt(raw)
		if err != nil {
			return nil, fmt.Errorf("attempt file %s: %w", entry.Name(), err)
		}
		attempts = append(attempts, attempt)
	}
	return attempts, nil
}

// Attempt files are structured front matter plus prose (ADR-0004): a JSON
// object between a pair of "---" lines, followed by the prose body. JSON
// rather than YAML, so the format needs no dependency beyond encoding/json
// and handles multi-line Ticket text without hand-rolled escaping.

func encodeAttempt(a Attempt) ([]byte, error) {
	body, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(body)
	buf.WriteString("\n---\n")
	return buf.Bytes(), nil
}

func parseAttempt(raw []byte) (Attempt, error) {
	frontMatter, _, err := splitFrontMatter(raw)
	if err != nil {
		return Attempt{}, err
	}
	// frontMatter carries the "---\n...\n---\n" delimiters; strip them back
	// off to reach the JSON between them.
	frontMatterJSON := strings.TrimSuffix(strings.TrimPrefix(frontMatter, "---\n"), "---\n")
	var a Attempt
	if err := json.Unmarshal([]byte(frontMatterJSON), &a); err != nil {
		return Attempt{}, fmt.Errorf("front matter is not valid JSON: %w", err)
	}
	return a, nil
}

// splitFrontMatter separates an Attempt file's front matter — including its
// "---" delimiters — from the prose body below it.
func splitFrontMatter(raw []byte) (frontMatter, body string, err error) {
	// The record is its own git repository (ADR-0004), and this machine's
	// git can rewrite the working tree with CRLF line endings on a checkout,
	// reset, or clone of it (core.autocrlf). Normalising first means a
	// front-matter delimiter still matches regardless of which line ending
	// the filesystem currently holds it in.
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 2 || lines[0] != "---" {
		return "", "", errors.New("does not start with a --- front matter delimiter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", "", errors.New("front matter has no closing ---")
	}
	frontMatter = strings.Join(lines[:end+1], "\n") + "\n"
	body = strings.Join(lines[end+1:], "\n")
	return frontMatter, body, nil
}
