// Package vault is the Vault boundary (CONTEXT.md, ADR-0005): the single
// interface through which anything requiring spoiler content passes. It is
// the only code path in Tracer that reads the orphan vault ref or runs the
// image built from it — reviewable as one file rather than by vigilance
// across the codebase (STD-009).
//
// At this stage the boundary carries grading only: Build proves the image
// pipeline ADR-0004 and ADR-0005 both rest on, and Grade returns a Verdict —
// pass or fail, nothing else. Later tickets extend the boundary with
// Location claim evaluation and intended-path retrieval.
package vault

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// Verdict is the only thing a grade reveals. No assertion text, test name,
// diff or path ever crosses the Vault boundary (STD-009).
type Verdict struct {
	Passed bool
}

// Client builds and runs the Vault image for one Exercise's spoiler content.
type Client struct {
	// RepoDir is the git repository holding Ref. Ref is never checked out
	// there — Build streams an archive of it straight into the image build.
	RepoDir string
	// Ref is the orphan branch (or other ref) the image is built from.
	Ref string
	// Image is the tag the built image is stored and run under.
	Image string
}

// Build builds the Vault image by streaming a git archive of c.Ref into
// docker build. c.Ref is never checked out — the plaintext it carries never
// lands in the working tree (ADR-0005).
//
// Neither command's stderr is included in a returned error: docker build's
// output routinely echoes COPY/RUN lines and file contents from the build
// context, which is exactly the plaintext STD-009 keeps off a learner-visible
// surface. A caller that needs the raw output for authoring-time debugging
// should shell out to `git archive | docker build` directly rather than
// through this method.
func (c Client) Build() error {
	archive := exec.Command("git", "-C", c.RepoDir, "archive", c.Ref)
	build := exec.Command("docker", "build", "-t", c.Image, "-")

	pr, pw := io.Pipe()
	archive.Stdout = pw
	build.Stdin = pr

	if err := archive.Start(); err != nil {
		return fmt.Errorf("archiving %s: %w", c.Ref, err)
	}
	if err := build.Start(); err != nil {
		return fmt.Errorf("starting docker build: %w", err)
	}

	archiveDone := make(chan error, 1)
	go func() {
		err := archive.Wait()
		pw.CloseWithError(err)
		archiveDone <- err
	}()

	buildErr := build.Wait()
	if err := <-archiveDone; err != nil {
		return fmt.Errorf("archiving %s: %w", c.Ref, err)
	}
	if buildErr != nil {
		return fmt.Errorf("building the Vault image: %w", buildErr)
	}
	return nil
}

// Grade runs the Vault image against tree and returns a Verdict. tree is
// mounted read-only, so the image never gains write access to it. Nothing
// the image writes to stdout or stderr is relayed — the exit code alone
// decides the Verdict, which is what STD-009 requires of a grade result.
func (c Client) Grade(tree string) (Verdict, error) {
	cmd := exec.Command("docker", "run", "--rm", "-v", tree+":/submission:ro", c.Image)

	err := cmd.Run()
	if err == nil {
		return Verdict{Passed: true}, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return Verdict{Passed: false}, nil
	}
	return Verdict{}, fmt.Errorf("running the Vault image: %w", err)
}
