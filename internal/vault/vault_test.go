package vault

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This is the pipeline proof ADR-0004 and ADR-0005 rest on (ticket #2): a
// real orphan branch, archived without being checked out, built into a real
// image, and run for real against a known-good and a known-bad tree. No
// fake stands in anywhere in this file.

func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found on PATH — skipping the Vault image pipeline test")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not reachable — skipping the Vault image pipeline test")
	}
}

func TestBuildAndGradeAgainstTheRealVaultImage(t *testing.T) {
	requireDocker(t)

	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoDir = filepath.Join(repoDir, "..", "..") // internal/vault -> repo root

	client := Client{
		RepoDir: repoDir,
		Ref:     "vault",
		Image:   "tracer-vault-pipeline-proof:test",
	}

	if !refExists(t, repoDir, "vault") {
		t.Fatal(`the orphan "vault" branch does not exist in this checkout`)
	}
	if checkedOut(t, repoDir, "vault") {
		t.Fatal(`the "vault" branch is checked out — it must never be`)
	}

	if err := client.Build(); err != nil {
		t.Fatalf("building the Vault image: %v", err)
	}
	t.Cleanup(func() {
		exec.Command("docker", "rmi", "-f", client.Image).Run()
	})

	if checkedOut(t, repoDir, "vault") {
		t.Fatal(`building the Vault image checked out the "vault" branch`)
	}

	// The known-good and known-bad marker content live only on the vault
	// ref's fixtures/ tree, fetched here through `git show` rather than
	// duplicated as a literal in this file — the same plaintext, copied into
	// a file on the reviewable branch, would undercut the boundary ADR-0005
	// draws around the orphan ref.
	goodTree := submissionTree(t, repoDir, "vault:fixtures/good/marker.txt")
	goodVerdict, err := client.Grade(goodTree)
	if err != nil {
		t.Fatalf("grading the known-good tree: %v", err)
	}
	if !goodVerdict.Passed {
		t.Error("the known-good tree was graded as failing")
	}

	badTree := submissionTree(t, repoDir, "vault:fixtures/bad/marker.txt")
	badVerdict, err := client.Grade(badTree)
	if err != nil {
		t.Fatalf("grading the known-bad tree: %v", err)
	}
	if badVerdict.Passed {
		t.Error("the known-bad tree was graded as passing")
	}
}

// submissionTree builds a submission tree carrying marker.txt with the
// content of markerRef (a vault-ref path, e.g. "vault:fixtures/good/marker.txt").
func submissionTree(t *testing.T, repoDir, markerRef string) string {
	t.Helper()
	content, err := exec.Command("git", "-C", repoDir, "show", markerRef).Output()
	if err != nil {
		t.Fatalf("reading %s: %v", markerRef, err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDebriefRefusesAnOpenExerciseWithoutTouchingDocker proves the refusal
// lives at Debrief's own interface (issue #8's acceptance criteria): closed
// is checked before Docker is ever invoked, so this needs no Docker at all —
// unlike every other test in this file, it runs unconditionally.
func TestDebriefRefusesAnOpenExerciseWithoutTouchingDocker(t *testing.T) {
	// An Image that does not exist: if Debrief reached Docker at all with
	// closed=false, this would fail with a Docker error rather than
	// ErrExerciseOpen, exposing that the check came too late.
	client := Client{Image: "tracer-vault-image-that-does-not-exist:test"}

	_, err := client.Debrief(false, []Claim{{ProbeIndex: 1, Location: "somewhere"}})

	if !errors.Is(err, ErrExerciseOpen) {
		t.Errorf("Debrief err = %v, want ErrExerciseOpen", err)
	}
}

func TestDebriefAgainstTheRealVaultImage(t *testing.T) {
	requireDocker(t)

	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoDir = filepath.Join(repoDir, "..", "..") // internal/vault -> repo root

	client := Client{
		RepoDir: repoDir,
		Ref:     "vault",
		Image:   "tracer-vault-debrief-pipeline-proof:test",
	}
	if err := client.Build(); err != nil {
		t.Fatalf("building the Vault image: %v", err)
	}
	t.Cleanup(func() {
		exec.Command("docker", "rmi", "-f", client.Image).Run()
	})

	debrief, err := client.Debrief(true, []Claim{{ProbeIndex: 1, Location: "wherever"}})
	if err != nil {
		t.Fatalf("Debrief: %v", err)
	}
	if debrief.IntendedPath == "" {
		t.Error("Debrief returned no intended-path narrative")
	}
}

func refExists(t *testing.T, repoDir, ref string) bool {
	t.Helper()
	return exec.Command("git", "-C", repoDir, "rev-parse", "--verify", ref).Run() == nil
}

func checkedOut(t *testing.T, repoDir, ref string) bool {
	t.Helper()
	out, err := exec.Command("git", "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out) == ref+"\n"
}
