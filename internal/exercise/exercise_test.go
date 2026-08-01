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
