package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestForfeitClosesTheOpenExerciseAsNotCleared(t *testing.T) {
	record := startExercise(t)

	got := runTracer(t, "exercise", "forfeit", "--record", record)

	if got.code != 0 {
		t.Fatalf("forfeit exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "bug-001") {
		t.Errorf("forfeit did not name the Exercise: %q", got.stdout)
	}
	if !strings.Contains(got.stdout, "not cleared") {
		t.Errorf("forfeit did not report itself as not cleared: %q", got.stdout)
	}

	status := runTracer(t, "exercise", "status", "--record", record)
	if !strings.Contains(status.stdout, "No Exercise is open") {
		t.Errorf("an Exercise is still reported open after forfeit: %q", status.stdout)
	}
}

func TestForfeitRefusesWhenNoExerciseIsOpen(t *testing.T) {
	record := filepath.Join(t.TempDir(), "record")

	got := runTracer(t, "exercise", "forfeit", "--record", record)

	if got.code != 1 {
		t.Errorf("forfeit with no Exercise open exited %d, want 1; output %s%s", got.code, got.stdout, got.stderr)
	}
}
