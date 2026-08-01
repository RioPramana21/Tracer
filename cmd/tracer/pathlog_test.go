package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setStubEditor makes $EDITOR the stub editor (see
// testdata/stubeditor and TestMain), which overwrites whatever file the
// tracer opens it on with content — standing in for "what the learner typed".
func setStubEditor(t *testing.T, content string) {
	t.Helper()
	contentPath := filepath.Join(t.TempDir(), "editor-content.md")
	if err := os.WriteFile(contentPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", stubEditorBin)
	t.Setenv("TRACER_TEST_EDITOR_CONTENT", contentPath)
}

// startExercise starts the fixture Exercise "bug-001" and returns its
// progress record directory.
func startExercise(t *testing.T) (record string) {
	t.Helper()
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t, catalogEntry{
		ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline,
	})
	record = filepath.Join(t.TempDir(), "record")
	checkout := filepath.Join(t.TempDir(), "checkout")
	if got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout); got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	return record
}

func attemptFile(t *testing.T, record string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(record, "bug-001.md"))
	if err != nil {
		t.Fatalf("reading attempt file: %v", err)
	}
	return string(raw)
}

func TestPathLogFileRecordsHypothesisBecauseAndLocationClaim(t *testing.T) {
	record := startExercise(t)
	setStubEditor(t, "Hypothesis: the retry job double-submits on timeout.\n"+
		"Because: the log shows two identical POSTs 200ms apart.\n"+
		"Location claim: internal/billing/retry\n")

	got := runTracer(t, "pathlog", "file", "--record", record)

	if got.code != 0 {
		t.Fatalf("pathlog file exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "probe 1") {
		t.Errorf("did not report the assigned probe: %q", got.stdout)
	}
	if !strings.Contains(got.stdout, "no indication of correctness") {
		t.Errorf("filing a Location claim did not say it was met with silence: %q", got.stdout)
	}

	body := attemptFile(t, record)
	for _, want := range []string{
		"Hypothesis: the retry job double-submits on timeout.",
		"Because: the log shows two identical POSTs 200ms apart.",
		"Location claim: internal/billing/retry",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("attempt file did not read back %q:\n%s", want, body)
		}
	}
}

func TestPathLogFileRejectsAnEntryWithNoBecause(t *testing.T) {
	record := startExercise(t)
	setStubEditor(t, "Hypothesis: the retry job double-submits on timeout.\n\nBecause:\n")

	got := runTracer(t, "pathlog", "file", "--record", record)

	if got.code != 1 {
		t.Errorf("exited %d, want 1; output %s%s", got.code, got.stdout, got.stderr)
	}
	body := attemptFile(t, record)
	if strings.Contains(body, "Probe") {
		t.Errorf("rejected entry was recorded anyway:\n%s", body)
	}
}

func TestPathLogFileWithoutALocationClaimOmitsTheLine(t *testing.T) {
	record := startExercise(t)
	setStubEditor(t, "Hypothesis: the retry job double-submits on timeout.\n"+
		"Because: the log shows two identical POSTs 200ms apart.\n"+
		"Location claim:\n")

	got := runTracer(t, "pathlog", "file", "--record", record)

	if got.code != 0 {
		t.Fatalf("pathlog file exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if strings.Contains(got.stdout, "no indication of correctness") {
		t.Errorf("reported a Location claim that was never filed: %q", got.stdout)
	}
	if strings.Contains(attemptFile(t, record), "Location claim:") {
		t.Errorf("wrote a Location claim line for an entry with none")
	}
}

func TestPathLogFileAssignsIncrementingProbeIndices(t *testing.T) {
	record := startExercise(t)

	setStubEditor(t, "Hypothesis: first guess.\nBecause: first reason.\n")
	first := runTracer(t, "pathlog", "file", "--record", record)
	if first.code != 0 {
		t.Fatalf("first pathlog file exited %d; output %s%s", first.code, first.stdout, first.stderr)
	}
	if !strings.Contains(first.stdout, "probe 1") {
		t.Errorf("first entry not filed as probe 1: %q", first.stdout)
	}

	setStubEditor(t, "Hypothesis: second guess.\nBecause: second reason.\n")
	second := runTracer(t, "pathlog", "file", "--record", record)
	if second.code != 0 {
		t.Fatalf("second pathlog file exited %d; output %s%s", second.code, second.stdout, second.stderr)
	}
	if !strings.Contains(second.stdout, "probe 2") {
		t.Errorf("second entry not filed as probe 2: %q", second.stdout)
	}

	body := attemptFile(t, record)
	if !strings.Contains(body, "first reason") || !strings.Contains(body, "second reason") {
		t.Errorf("both entries were not both retained:\n%s", body)
	}
}

func TestPathLogFileIsRefusedWithNoExerciseOpen(t *testing.T) {
	record := filepath.Join(t.TempDir(), "record")
	setStubEditor(t, "Hypothesis: a guess.\nBecause: a reason.\n")

	got := runTracer(t, "pathlog", "file", "--record", record)

	if got.code != 1 {
		t.Errorf("exited %d, want 1; output %s%s", got.code, got.stdout, got.stderr)
	}
}

func TestPathLogFileEntryIsCommittedToTheProgressRecord(t *testing.T) {
	record := startExercise(t)
	setStubEditor(t, "Hypothesis: a guess.\nBecause: a reason.\n")

	if got := runTracer(t, "pathlog", "file", "--record", record); got.code != 0 {
		t.Fatalf("pathlog file exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	log := runGitOutput(t, record, "log", "--oneline")
	if !strings.Contains(log, "probe 1") {
		t.Errorf("progress record history has no commit for the filed entry: %q", log)
	}
}
