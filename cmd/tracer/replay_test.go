package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplayResetsTheFixBranchAndRecordsIt(t *testing.T) {
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t, catalogEntry{
		ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline,
	})
	record := filepath.Join(t.TempDir(), "record")
	checkout := filepath.Join(t.TempDir(), "checkout")
	if got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout); got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	playgroundCheckout := filepath.Join(checkout, "Playground")
	if err := os.WriteFile(filepath.Join(playgroundCheckout, "app.go"), []byte("package main\n\n// an attempted fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, playgroundCheckout, "add", "-A")
	runGit(t, playgroundCheckout, "-c", "user.name=fixture", "-c", "user.email=fixture@localhost",
		"commit", "-m", "an attempted fix")

	if got := runTracer(t, "exercise", "forfeit", "--record", record); got.code != 0 {
		t.Fatalf("forfeit exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	got := runTracer(t, "exercise", "replay", "--record", record, "--exercise", "bug-001")

	if got.code != 0 {
		t.Fatalf("replay exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "bug-001") {
		t.Errorf("replay did not name the Exercise: %q", got.stdout)
	}

	head := strings.TrimSpace(runGitOutput(t, playgroundCheckout, "rev-parse", "HEAD"))
	if head != baseline {
		t.Errorf("fix branch HEAD after replay is %q, want the baseline %q", head, baseline)
	}

	attempts, err := filepath.Glob(filepath.Join(record, "bug-001*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 {
		t.Errorf("progress record has %d attempt files for bug-001, want 2 (the Forfeit and the Replay): %v", len(attempts), attempts)
	}

	status := runTracer(t, "exercise", "status", "--record", record)
	if !strings.Contains(status.stdout, "replayed") {
		t.Errorf("status does not report the Exercise as being replayed: %q", status.stdout)
	}
}

func TestReplayRefusesWhenTheExerciseWasNotForfeited(t *testing.T) {
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t, catalogEntry{
		ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline,
	})
	record := filepath.Join(t.TempDir(), "record")
	checkout := filepath.Join(t.TempDir(), "checkout")
	if got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout); got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	got := runTracer(t, "exercise", "replay", "--record", record, "--exercise", "bug-001")

	if got.code != 1 {
		t.Errorf("replay of an open (not Forfeited) Exercise exited %d, want 1; output %s%s", got.code, got.stdout, got.stderr)
	}
}

