package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startedExercise starts one fixture Exercise and returns its progress
// record directory and Exercise id.
func startedExercise(t *testing.T) (recordDir, exerciseID string) {
	t.Helper()
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t, catalogEntry{
		ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline,
	})
	recordDir = filepath.Join(t.TempDir(), "record")
	checkoutDir := filepath.Join(t.TempDir(), "checkout")
	if got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", recordDir, "--playground", playground, "--checkout", checkoutDir); got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	return recordDir, "bug-001"
}

// clockFrontMatter reads the clock fields out of an Attempt file's front
// matter — the same "read the file, not the internal representation"
// approach TestStartArmsTheAgentBoundaryAndRecordsADigest uses for the
// boundary record, applied here because the accumulated interval landing in
// the Attempt file is itself the acceptance criterion (issue #7).
func clockFrontMatter(t *testing.T, recordDir, exerciseID string) (workedNanos int64, running bool) {
	t.Helper()
	raw := readAttemptFile(t, recordDir, exerciseID)
	var fm struct {
		ClockWorked       int64   `json:"clock_worked"`
		ClockRunningSince *string `json:"clock_running_since"`
	}
	if err := json.Unmarshal([]byte(raw), &fm); err != nil {
		t.Fatalf("attempt front matter is not valid JSON: %v\n%s", err, raw)
	}
	return fm.ClockWorked, fm.ClockRunningSince != nil
}

func readAttemptFile(t *testing.T, recordDir, exerciseID string) string {
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
	return strings.Join(lines[1:end], "\n")
}

func TestPauseAndResumeAccumulateOnlyWorkedIntervals(t *testing.T) {
	recordDir, exerciseID := startedExercise(t)

	// Every CLI invocation here spawns a real process, so this test's
	// tolerances are set against wall-clock elapsed rather than a fixed
	// millisecond budget — spawn overhead is unpredictable, but the gap
	// below is a deliberate "dinner" the recorded interval must mostly
	// exclude regardless of how slow spawning a process is on this machine.
	wallStart := time.Now()

	if got := runTracer(t, "exercise", "pause", "--record", recordDir); got.code != 0 {
		t.Fatalf("pause exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	const gap = 2 * time.Second
	time.Sleep(gap)

	if got := runTracer(t, "exercise", "resume", "--record", recordDir); got.code != 0 {
		t.Fatalf("resume exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if got := runTracer(t, "exercise", "pause", "--record", recordDir); got.code != 0 {
		t.Fatalf("second pause exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	wallElapsed := time.Since(wallStart)

	workedNanos, running := clockFrontMatter(t, recordDir, exerciseID)
	worked := time.Duration(workedNanos)
	if running {
		t.Error("clock is still recorded as running after a pause")
	}
	if worked <= 0 {
		t.Error("no worked interval was recorded at all")
	}
	// The gap dominates wallElapsed; a worked interval anywhere close to
	// wallElapsed means the paused gap leaked into the record instead of
	// being excluded from it.
	if worked > wallElapsed-gap/2 {
		t.Errorf("worked interval %s is too close to the wall-clock elapsed %s (gap %s) — the paused gap leaked into the record",
			worked, wallElapsed, gap)
	}
}

func TestPauseRefusesWhenAlreadyPaused(t *testing.T) {
	recordDir, _ := startedExercise(t)
	if got := runTracer(t, "exercise", "pause", "--record", recordDir); got.code != 0 {
		t.Fatalf("first pause exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	got := runTracer(t, "exercise", "pause", "--record", recordDir)

	if got.code != 1 {
		t.Errorf("second pause exited %d, want 1; output %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestResumeRefusesWhenAlreadyRunning(t *testing.T) {
	recordDir, _ := startedExercise(t)

	got := runTracer(t, "exercise", "resume", "--record", recordDir)

	if got.code != 1 {
		t.Errorf("resume on an already-running clock exited %d, want 1; output %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestPauseRefusesWhenNoExerciseIsOpen(t *testing.T) {
	recordDir := filepath.Join(t.TempDir(), "record")

	got := runTracer(t, "exercise", "pause", "--record", recordDir)

	if got.code != 1 {
		t.Errorf("pause with no Exercise open exited %d, want 1; output %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestResumeRefusesWhenNoExerciseIsOpen(t *testing.T) {
	recordDir := filepath.Join(t.TempDir(), "record")

	got := runTracer(t, "exercise", "resume", "--record", recordDir)

	if got.code != 1 {
		t.Errorf("resume with no Exercise open exited %d, want 1; output %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestPauseAndResumeStateSurvivesAcrossInvocations(t *testing.T) {
	recordDir, _ := startedExercise(t)

	// Each runTracer call is its own process — pausing in one invocation and
	// resuming in a later, unrelated one is the only way this CLI can be
	// driven, so a resume succeeding here is exactly issue #7's "survives
	// across separate invocations".
	if got := runTracer(t, "exercise", "pause", "--record", recordDir); got.code != 0 {
		t.Fatalf("pause exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if got := runTracer(t, "exercise", "resume", "--record", recordDir); got.code != 0 {
		t.Errorf("resume in a fresh invocation exited %d, want 0; output %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestClockNeverAppearsInAnyOutputWhileOpen(t *testing.T) {
	recordDir, _ := startedExercise(t)

	next := runTracer(t, "exercise", "status", "--record", recordDir)
	pause := runTracer(t, "exercise", "pause", "--record", recordDir)
	resume := runTracer(t, "exercise", "resume", "--record", recordDir)

	for _, got := range []result{next, pause, resume} {
		for _, withheld := range []string{"Probes to locate", "Time to locate"} {
			if strings.Contains(got.stdout, withheld) || strings.Contains(got.stderr, withheld) {
				t.Errorf("output printed %q while the Exercise is open: stdout %q stderr %q", withheld, got.stdout, got.stderr)
			}
		}
	}
}
