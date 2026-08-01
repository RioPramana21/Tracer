// Package gitrepo wraps the handful of git operations the Exercise loop
// drives: cloning a Playground checkout, cutting a fix branch from a fixed
// baseline, and keeping the progress record's own history. It names no
// Tracer concept, so it carries no domain vocabulary (STD-011) — it is
// plumbing under internal/exercise and internal/record, not a concept
// CONTEXT.md defines.
package gitrepo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Clone clones src into dst with full history, so the clone carries the
// Playground's simulated timeline and any commit named as a Baseline.
func Clone(src, dst string) error {
	return run("", "clone", src, dst)
}

// CreateBranch cuts branch from ref inside repoDir and checks it out. ref
// names a commit already present in repoDir's history — cloning first, as
// Clone does, is what puts it there.
func CreateBranch(repoDir, branch, ref string) error {
	return run(repoDir, "checkout", "-b", branch, ref)
}

// CurrentBranch reports the branch checked out in repoDir.
func CurrentBranch(repoDir string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse --abbrev-ref HEAD: %w: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// EnsureRepo makes dir a git repository if it is not already one. Idempotent:
// a dir that already has a .git is left untouched.
func EnsureRepo(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	return run(dir, "init")
}

// CommitAll stages every change in dir and commits it. A dir with nothing to
// commit is left alone rather than failed over, so callers can commit after
// every write without tracking whether that write actually changed anything.
func CommitAll(dir, message string) error {
	if err := run(dir, "add", "-A"); err != nil {
		return err
	}
	diff := exec.Command("git", "-C", dir, "diff", "--cached", "--quiet")
	if err := diff.Run(); err == nil {
		return nil // nothing staged
	}
	// An explicit identity, rather than the ambient git config, so a commit
	// to the progress record never depends on the machine it runs on having
	// user.name/user.email set.
	return run(dir, "-c", "user.name=tracer", "-c", "user.email=tracer@localhost",
		"commit", "-m", message)
}

func run(dir string, args ...string) error {
	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", full...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
	}
	return nil
}
