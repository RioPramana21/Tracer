package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSubmitAgainstTheRealVaultImage is the one real, Docker-gated CLI test
// for submit (issue #1's testing decisions: "the pipeline gets a real test
// ... tested for real, once, rather than faked everywhere"). It drives the
// built binary end to end — Start, edit the fix branch, submit — against the
// actual image built from the orphan "vault" branch (proved by ticket #2),
// and asserts the CLI's own output never carries what that image's grade.sh
// reads or writes: the AC's "no output ... contains the defect's location or
// the intended path", concretely anchored to this batch's only Vault content.
//
// The marker content itself is read through `git show vault:...`, the same
// way internal/vault/vault_test.go's submissionTree does — never duplicated
// as a literal here, which would put the plaintext ADR-0005 keeps off the
// orphan ref onto this reviewable branch instead (STD-009).
func TestSubmitAgainstTheRealVaultImage(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found on PATH — skipping the real Vault submit test")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not reachable — skipping the real Vault submit test")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot = filepath.Join(repoRoot, "..", "..") // cmd/tracer -> repo root

	badMarker := vaultMarkerContent(t, repoRoot, "fixtures/bad/marker.txt")
	goodMarker := vaultMarkerContent(t, repoRoot, "fixtures/good/marker.txt")

	playground := t.TempDir()
	runGit(t, playground, "init")
	if err := os.WriteFile(filepath.Join(playground, "marker.txt"), []byte(badMarker), 0o644); err != nil {
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

	image := "tracer-vault-submit-test:test"
	t.Cleanup(func() {
		exec.Command("docker", "rmi", "-f", image).Run()
	})

	if got := runTracer(t, "exercise", "start",
		"--catalog", catalogPath, "--record", record, "--playground", playground, "--checkout", checkout); got.code != 0 {
		t.Fatalf("start exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	// A submission before the fix must fail, and leak neither marker
	// content grade.sh checks for nor any word describing why it failed.
	first := runTracer(t, "exercise", "submit",
		"--record", record, "--vault-repo", repoRoot, "--vault-ref", "vault", "--vault-image", image)
	if first.code != 1 {
		t.Fatalf("submit before the fix exited %d, want 1; output %s%s", first.code, first.stdout, first.stderr)
	}
	assertNoSpoilerLeak(t, first, badMarker, goodMarker)

	// Apply the fix the vault ref's grade.sh actually checks for.
	playgroundCheckout := filepath.Join(checkout, "Playground")
	if err := os.WriteFile(filepath.Join(playgroundCheckout, "marker.txt"), []byte(goodMarker), 0o644); err != nil {
		t.Fatal(err)
	}

	second := runTracer(t, "exercise", "submit",
		"--record", record, "--vault-repo", repoRoot, "--vault-ref", "vault", "--vault-image", image)
	if second.code != 0 {
		t.Fatalf("submit after the fix exited %d, want 0; output %s%s", second.code, second.stdout, second.stderr)
	}
	if !strings.Contains(second.stdout, "Cleared") {
		t.Errorf("submit after the fix did not report a Clear: %q", second.stdout)
	}
	assertNoSpoilerLeak(t, second, badMarker, goodMarker)
}

// vaultMarkerContent reads a fixture marker's plaintext through `git show
// vault:<path>` rather than duplicating it as a literal in this file — the
// same plaintext, copied onto this reviewable branch, would undercut the
// boundary ADR-0005 draws around the orphan ref.
func vaultMarkerContent(t *testing.T, repoRoot, path string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoRoot, "show", "vault:"+path).Output()
	if err != nil {
		t.Fatalf("reading vault:%s: %v", path, err)
	}
	return string(out)
}

func assertNoSpoilerLeak(t *testing.T, got result, spoilers ...string) {
	t.Helper()
	for _, spoiler := range append(spoilers, "marker.txt") {
		if strings.Contains(got.stdout, spoiler) || strings.Contains(got.stderr, spoiler) {
			t.Errorf("submit output leaked %q: stdout %q stderr %q", spoiler, got.stdout, got.stderr)
		}
	}
}
