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

	"github.com/RioPramana21/Tracer/internal/gitrepo"
)

// State is where an Attempt stands. Only StateOpen is reachable in this
// slice; Clear, Forfeit and Replay (#6, #8, #9) add the rest.
type State string

// StateOpen means the Exercise is being worked and has not yet closed.
const StateOpen State = "open"

// Attempt is one worked instance of an Exercise: the front matter of one
// progress-record file. The prose body below the front matter — the Path
// log — is #5's to write.
type Attempt struct {
	ExerciseID string    `json:"exercise_id"`
	Ticket     string    `json:"ticket"`
	State      State     `json:"state"`
	StartedAt  time.Time `json:"started_at"`
	Baseline   string    `json:"baseline"`
	Branch     string    `json:"branch"`
	Checkout   string    `json:"checkout"`
}

// Store manages the progress record at Dir.
type Store struct {
	Dir string
}

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
	// The record is its own git repository (ADR-0004), and this machine's
	// git can rewrite the working tree with CRLF line endings on a checkout,
	// reset, or clone of it (core.autocrlf). Normalising first means a
	// front-matter delimiter still matches regardless of which line ending
	// the filesystem currently holds it in.
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 2 || lines[0] != "---" {
		return Attempt{}, errors.New("does not start with a --- front matter delimiter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return Attempt{}, errors.New("front matter has no closing ---")
	}
	var a Attempt
	if err := json.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &a); err != nil {
		return Attempt{}, fmt.Errorf("front matter is not valid JSON: %w", err)
	}
	return a, nil
}
