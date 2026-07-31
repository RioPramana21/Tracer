package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Fixture Playground: a temp git repo with one commit, whose SHA is the
// Baseline every fixture Catalog entry cuts a fix branch from.

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

type catalogEntry struct {
	ID       string `json:"id"`
	Ticket   string `json:"ticket"`
	Baseline string `json:"baseline"`
}

func writeCatalog(t *testing.T, entries ...catalogEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.json")
	doc := struct {
		Entries []catalogEntry `json:"entries"`
	}{Entries: entries}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNextPrintsTheTicketAndNothingElse(t *testing.T) {
	_, baseline := initPlayground(t)
	catalogPath := writeCatalog(t, catalogEntry{
		ID:       "bug-001",
		Ticket:   "Customers say the payment page spins forever and never confirms the order.",
		Baseline: baseline,
	})
	record := filepath.Join(t.TempDir(), "record")

	got := runTracer(t, "exercise", "next", "--catalog", catalogPath, "--record", record)

	if got.code != 0 {
		t.Fatalf("next exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "Customers say the payment page spins forever and never confirms the order.") {
		t.Errorf("next did not print the Ticket: %q", got.stdout)
	}
	if strings.Contains(got.stdout, baseline) {
		t.Errorf("next leaked the baseline commit: %q", got.stdout)
	}
	for _, leak := range []string{"checkout", "branch", "Playground"} {
		if strings.Contains(got.stdout, leak) {
			t.Errorf("next leaked %q beyond the Ticket: %q", leak, got.stdout)
		}
	}
}

func TestStartCutsTheBranchFromTheBaselineLeavingItUntouched(t *testing.T) {
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t, catalogEntry{
		ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline,
	})
	record := filepath.Join(t.TempDir(), "record")
	checkout := filepath.Join(t.TempDir(), "checkout")

	got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout)
	if got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	playgroundCheckout := filepath.Join(checkout, "Playground")
	branch := strings.TrimSpace(runGitOutput(t, playgroundCheckout, "rev-parse", "--abbrev-ref", "HEAD"))
	if branch != "exercise/bug-001" {
		t.Errorf("checkout is on branch %q, want exercise/bug-001", branch)
	}
	head := strings.TrimSpace(runGitOutput(t, playgroundCheckout, "rev-parse", "HEAD"))
	if head != baseline {
		t.Errorf("checkout HEAD is %q, want the baseline %q", head, baseline)
	}

	sourceHead := strings.TrimSpace(runGitOutput(t, playground, "rev-parse", "HEAD"))
	if sourceHead != baseline {
		t.Errorf("starting moved the source Playground's HEAD to %q", sourceHead)
	}
	if strings.Contains(runGitOutput(t, playground, "branch"), "exercise/bug-001") {
		t.Errorf("starting created the fix branch in the source Playground, not just the clone")
	}
}

func TestStartArmsTheAgentBoundaryAndRecordsADigest(t *testing.T) {
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t, catalogEntry{
		ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline,
	})
	record := filepath.Join(t.TempDir(), "record")
	checkout := filepath.Join(t.TempDir(), "checkout")

	got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout)
	if got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	rules := denyRules(t, checkout)
	if !slices.Contains(rules, "Read(Playground/**)") {
		t.Errorf("start did not arm the Playground deny rule: %v", rules)
	}

	raw, err := os.ReadFile(filepath.Join(record, "bug-001.boundary.json"))
	if err != nil {
		t.Fatalf("reading boundary record: %v", err)
	}
	var recorded struct {
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(recorded.Digest, "sha256:") {
		t.Errorf("digest %q is not a sha256 digest", recorded.Digest)
	}
}

func TestStartWritesAnAttemptFileOutsideTheCheckoutInItsOwnGitRepo(t *testing.T) {
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t, catalogEntry{
		ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline,
	})
	record := filepath.Join(t.TempDir(), "record")
	checkout := filepath.Join(t.TempDir(), "checkout")

	got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout)
	if got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	// Asserts what AC#4 actually requires — a file was written, outside the
	// checkout, in its own versioned git repository — without reaching into
	// the front matter's internal shape (issue #1's own testing decisions:
	// a test that breaks when the record's file layout changes but the
	// learner's experience does not is testing the wrong thing).
	attempts, err := filepath.Glob(filepath.Join(record, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 {
		t.Errorf("progress record has %d attempt files, want 1: %v", len(attempts), attempts)
	}

	if _, err := os.Stat(filepath.Join(record, ".git")); err != nil {
		t.Errorf("progress record is not its own git repository: %v", err)
	}
	if strings.TrimSpace(runGitOutput(t, record, "log", "--oneline")) == "" {
		t.Errorf("the attempt was not committed to the progress record")
	}
}

func TestStatusReportsTheOpenExerciseAndWithholdsProbesAndTime(t *testing.T) {
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

	got := runTracer(t, "exercise", "status", "--record", record)

	if got.code != 0 {
		t.Fatalf("status exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "bug-001") {
		t.Errorf("status did not name the open Exercise: %q", got.stdout)
	}
	if !strings.Contains(got.stdout, "exercise/bug-001") {
		t.Errorf("status did not report the branch: %q", got.stdout)
	}
	for _, withheld := range []string{"Probes to locate", "Time to locate"} {
		if strings.Contains(got.stdout, withheld) {
			t.Errorf("status printed %q while the Exercise is open: %q", withheld, got.stdout)
		}
	}
}

func TestStatusReportsNoExerciseOpenBeforeStarting(t *testing.T) {
	record := filepath.Join(t.TempDir(), "record")

	got := runTracer(t, "exercise", "status", "--record", record)

	if got.code != 0 {
		t.Fatalf("status exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "No Exercise is open") {
		t.Errorf("status said %q, want it to report no Exercise open", got.stdout)
	}
}

func TestStartingASecondExerciseIsRefused(t *testing.T) {
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t,
		catalogEntry{ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline},
		catalogEntry{ID: "bug-002", Ticket: "The dashboard totals stop updating after lunchtime.", Baseline: baseline},
	)
	record := filepath.Join(t.TempDir(), "record")
	checkout1 := filepath.Join(t.TempDir(), "checkout1")
	if first := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout1); first.code != 0 {
		t.Fatalf("first start exited %d; output %s%s", first.code, first.stdout, first.stderr)
	}

	checkout2 := filepath.Join(t.TempDir(), "checkout2")
	second := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout2)

	if second.code != 1 {
		t.Errorf("second start exited %d, want 1; output %s%s", second.code, second.stdout, second.stderr)
	}
	if _, err := os.Stat(filepath.Join(record, "bug-002.md")); !os.IsNotExist(err) {
		t.Errorf("second start wrote an attempt file despite being refused")
	}
	if _, err := os.Stat(checkout2); !os.IsNotExist(err) {
		t.Errorf("second start created a checkout despite being refused")
	}
}

func TestNextSkipsAnExerciseAlreadyAttempted(t *testing.T) {
	playground, baseline := initPlayground(t)
	catalogPath := writeCatalog(t,
		catalogEntry{ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline},
		catalogEntry{ID: "bug-002", Ticket: "The dashboard totals stop updating after lunchtime.", Baseline: baseline},
	)
	record := filepath.Join(t.TempDir(), "record")
	checkout := filepath.Join(t.TempDir(), "checkout")
	if got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout); got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	got := runTracer(t, "exercise", "next", "--catalog", catalogPath, "--record", record)

	if got.code != 0 {
		t.Fatalf("next exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "The dashboard totals stop updating after lunchtime.") {
		t.Errorf("next did not skip the already-attempted Exercise: %q", got.stdout)
	}
	if strings.Contains(got.stdout, "The nightly export file is empty three days out of five.") {
		t.Errorf("next re-offered an already-attempted Exercise: %q", got.stdout)
	}
}
