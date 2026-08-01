package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDebriefAgainstTheRealVaultImage is the one real, Docker-gated CLI test
// for debrief (issue #1's testing decisions: "the pipeline gets a real test
// ... tested for real, once, rather than faked everywhere"), mirroring
// TestSubmitAgainstTheRealVaultImage in submit_test.go. It drives the built
// binary end to end — start, forfeit, debrief — against the actual image
// built from the orphan "vault" branch's Debrief mode (ticket #8).
func TestDebriefAgainstTheRealVaultImage(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found on PATH — skipping the real Vault debrief test")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not reachable — skipping the real Vault debrief test")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot = filepath.Join(repoRoot, "..", "..") // cmd/tracer -> repo root

	playground := t.TempDir()
	runGit(t, playground, "init")
	if err := os.WriteFile(filepath.Join(playground, "app.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, playground, "add", "-A")
	runGit(t, playground, "-c", "user.name=fixture", "-c", "user.email=fixture@localhost",
		"commit", "-m", "Initial commit")
	baseline := strings.TrimSpace(runGitOutput(t, playground, "rev-parse", "HEAD"))

	catalogPath := writeCatalog(t, catalogEntry{
		ID: "bug-001", Ticket: "The nightly export file is empty three days out of five.", Baseline: baseline,
	})
	record := filepath.Join(t.TempDir(), "record")
	checkout := filepath.Join(t.TempDir(), "checkout")

	image := "tracer-vault-debrief-cli-test:test"
	t.Cleanup(func() {
		exec.Command("docker", "rmi", "-f", image).Run()
	})

	if got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout); got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	// A Debrief while the Exercise is still open is refused, at the Vault
	// boundary's own interface.
	beforeClose := runTracer(t, "exercise", "debrief",
		"--record", record, "--exercise", "bug-001", "--vault-repo", repoRoot, "--vault-ref", "vault", "--vault-image", image)
	if beforeClose.code != 1 {
		t.Fatalf("debrief on an open Exercise exited %d, want 1; output %s%s", beforeClose.code, beforeClose.stdout, beforeClose.stderr)
	}

	if got := runTracer(t, "exercise", "forfeit", "--record", record); got.code != 0 {
		t.Fatalf("forfeit exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	afterClose := runTracer(t, "exercise", "debrief",
		"--record", record, "--exercise", "bug-001", "--vault-repo", repoRoot, "--vault-ref", "vault", "--vault-image", image)
	if afterClose.code != 0 {
		t.Fatalf("debrief after a Forfeit exited %d, want 0; output %s%s", afterClose.code, afterClose.stdout, afterClose.stderr)
	}
	if !strings.Contains(afterClose.stdout, "intended path") {
		t.Errorf("debrief did not print the intended path: %q", afterClose.stdout)
	}
}
