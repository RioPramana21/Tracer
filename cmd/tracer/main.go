// Command tracer is the local CLI the Exercise loop is driven through
// (ADR-0004). It carries the Agent boundary verbs and the Exercise loop's
// next / start / status verbs.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/RioPramana21/Tracer/internal/agentboundary"
	"github.com/RioPramana21/Tracer/internal/exercise"
	"github.com/RioPramana21/Tracer/internal/record"
	"github.com/RioPramana21/Tracer/internal/vault"
)

const usage = `tracer — the Exercise loop

Usage:
  tracer boundary arm     --checkout <dir> --record <file>
  tracer boundary verify  --checkout <dir> --record <file>
  tracer boundary disarm  --checkout <dir> --record <file>

  tracer exercise next    --catalog <file> --record <dir>
  tracer exercise start   --catalog <file> --record <dir> --playground <repo> --checkout <dir>
  tracer exercise status  --record <dir>
  tracer exercise submit  --record <dir> --vault-repo <repo> --vault-ref <ref> --vault-image <tag>

  tracer pathlog file     --record <dir>

The Agent boundary is armed on an Exercise checkout by writing harness deny
rules over Playground paths, and a digest of the settings carrying them is
recorded outside the checkout. Verifying re-derives that digest.

Asking for the next Exercise prints its Ticket. Starting cuts a fix branch
from the Exercise's fixed baseline into a clone of --playground at
<checkout>/Playground, arms the Agent boundary over --checkout, and opens an
Attempt in the progress record at --record. Status reports the open Attempt,
if any, and withholds Probes to locate and Time to locate while it is open.

Filing a Path log entry opens $EDITOR over the open Exercise's Attempt: a
hypothesis, a because, and an optional Location claim. Rejected if the
because is empty. A Location claim is recorded with its Probe index and
timestamp and answered with silence — nothing about its correctness is
shown while the Exercise is open.

Submitting builds the Vault image from --vault-ref in --vault-repo and grades
the open Exercise's fix branch against it. A passing grade with an intact
Agent boundary records a Clear; anything else leaves the Exercise open, so
submitting again costs nothing. A failed submission reports failure only —
no assertion text, test name, diff or path ever crosses the Vault boundary.

Exit codes:
  0  the command succeeded — for boundary verify, the boundary is intact; for
     exercise next and exercise status, this is their only non-error outcome,
     including "nothing to report" (no Exercise left, or none open); for
     exercise submit, the Exercise was Cleared
  1  a substantive refusal: boundary verify found the boundary lifted or
     never armed, exercise start was refused (an Exercise is already open, or
     the Catalog has no Exercise left to offer), pathlog file was refused (no
     Exercise is open, or the because was empty), exercise submit was
     refused (no Exercise is open, or the checkout is not on the fix
     branch), or exercise submit ran but did not Clear (the hidden test
     failed, or the Agent boundary was not intact)
  2  the command could not be run
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	stdout, stderr := os.Stdout, os.Stderr

	if len(args) < 2 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "boundary":
		return runBoundary(args[1], args[2:], stdout, stderr)
	case "exercise":
		return runExercise(args[1], args[2:], stdout, stderr)
	case "pathlog":
		return runPathLog(args[1], args[2:], stdout, stderr)
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func runBoundary(verb string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("boundary "+verb, flag.ContinueOnError)
	flags.SetOutput(stderr)
	checkoutRoot := flags.String("checkout", ".", "the Exercise checkout the boundary applies to")
	recordPath := flags.String("record", "", "where the digest of the armed settings is recorded")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *recordPath == "" {
		fmt.Fprintln(stderr, "tracer: --record is required")
		return 2
	}

	boundary := agentboundary.Boundary{CheckoutRoot: *checkoutRoot, RecordPath: *recordPath}

	switch verb {
	case "arm":
		record, err := boundary.Arm()
		if err != nil {
			fmt.Fprintf(stderr, "tracer: arming the Agent boundary: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "Agent boundary armed over %d rules\n", len(record.Rules))
		fmt.Fprintf(stdout, "  settings: %s\n", record.Settings)
		fmt.Fprintf(stdout, "  digest:   %s\n", record.Digest)
		fmt.Fprintln(stdout, "  note:     an agent session already running against this checkout")
		fmt.Fprintln(stdout, "            does not pick this up — start or restart the session first")
		return 0

	case "verify":
		status, err := boundary.Verify()
		if err != nil {
			fmt.Fprintf(stderr, "tracer: verifying the Agent boundary: %v\n", err)
			return 2
		}
		switch status {
		case agentboundary.Intact:
			fmt.Fprintln(stdout, "Agent boundary intact — the armed rules are still the ones in force")
			return 0
		case agentboundary.Lifted:
			fmt.Fprintln(stdout, "Agent boundary lifted — the settings no longer re-derive to the recorded digest")
			return 1
		default:
			fmt.Fprintln(stdout, "Agent boundary unarmed — no boundary was armed for this checkout")
			return 1
		}

	case "disarm":
		if err := boundary.Disarm(); err != nil {
			fmt.Fprintf(stderr, "tracer: disarming the Agent boundary: %v\n", err)
			return 2
		}
		fmt.Fprintln(stdout, "Agent boundary disarmed")
		return 0

	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func runExercise(verb string, args []string, stdout, stderr io.Writer) int {
	switch verb {
	case "next":
		flags := flag.NewFlagSet("exercise next", flag.ContinueOnError)
		flags.SetOutput(stderr)
		catalogPath := flags.String("catalog", "", "the Catalog file to draw the next Exercise from")
		recordDir := flags.String("record", "", "the progress record directory")
		if err := flags.Parse(args); err != nil {
			return 2
		}
		if *catalogPath == "" || *recordDir == "" {
			fmt.Fprintln(stderr, "tracer: --catalog and --record are required")
			return 2
		}

		loop, err := exercise.Load(*catalogPath, *recordDir)
		if err != nil {
			fmt.Fprintf(stderr, "tracer: loading the Exercise loop: %v\n", err)
			return 2
		}
		entry, ok, err := loop.Next()
		if err != nil {
			fmt.Fprintf(stderr, "tracer: finding the next Exercise: %v\n", err)
			return 2
		}
		if !ok {
			fmt.Fprintln(stdout, "The Catalog has no Exercise left to offer.")
			return 0
		}
		fmt.Fprintf(stdout, "Ticket %s:\n\n  %s\n", entry.ID, entry.Ticket)
		return 0

	case "start":
		flags := flag.NewFlagSet("exercise start", flag.ContinueOnError)
		flags.SetOutput(stderr)
		catalogPath := flags.String("catalog", "", "the Catalog file to draw the next Exercise from")
		recordDir := flags.String("record", "", "the progress record directory")
		playground := flags.String("playground", "", "the Playground repository to clone the fix branch from")
		checkoutDir := flags.String("checkout", "", "where the Exercise checkout is created")
		if err := flags.Parse(args); err != nil {
			return 2
		}
		if *catalogPath == "" || *recordDir == "" || *playground == "" || *checkoutDir == "" {
			fmt.Fprintln(stderr, "tracer: --catalog, --record, --playground and --checkout are all required")
			return 2
		}

		loop, err := exercise.Load(*catalogPath, *recordDir)
		if err != nil {
			fmt.Fprintf(stderr, "tracer: loading the Exercise loop: %v\n", err)
			return 2
		}
		attempt, boundary, err := loop.Start(*playground, *checkoutDir)
		switch {
		case errors.Is(err, exercise.ErrAlreadyOpen):
			fmt.Fprintf(stderr, "tracer: an Exercise is already open — start refused\n")
			return 1
		case errors.Is(err, exercise.ErrCatalogExhausted):
			fmt.Fprintf(stderr, "tracer: %v — start refused\n", err)
			return 1
		case err != nil:
			fmt.Fprintf(stderr, "tracer: starting the Exercise: %v\n", err)
			return 2
		}

		fmt.Fprintf(stdout, "Exercise %s started\n", attempt.ExerciseID)
		fmt.Fprintf(stdout, "  ticket:   %s\n", attempt.Ticket)
		fmt.Fprintf(stdout, "  branch:   %s\n", attempt.Branch)
		fmt.Fprintf(stdout, "  checkout: %s\n", attempt.Checkout)
		fmt.Fprintf(stdout, "  Agent boundary armed over %d rules, digest %s\n", len(boundary.Rules), boundary.Digest)
		return 0

	case "status":
		flags := flag.NewFlagSet("exercise status", flag.ContinueOnError)
		flags.SetOutput(stderr)
		recordDir := flags.String("record", "", "the progress record directory")
		if err := flags.Parse(args); err != nil {
			return 2
		}
		if *recordDir == "" {
			fmt.Fprintln(stderr, "tracer: --record is required")
			return 2
		}

		loop := exercise.Loop{Record: record.Store{Dir: *recordDir}}
		attempt, open, err := loop.Status()
		if err != nil {
			fmt.Fprintf(stderr, "tracer: reading Exercise status: %v\n", err)
			return 2
		}
		if !open {
			fmt.Fprintln(stdout, "No Exercise is open.")
			return 0
		}
		fmt.Fprintf(stdout, "Exercise %s is open\n", attempt.ExerciseID)
		fmt.Fprintf(stdout, "  ticket:   %s\n", attempt.Ticket)
		fmt.Fprintf(stdout, "  branch:   %s\n", attempt.Branch)
		fmt.Fprintf(stdout, "  checkout: %s\n", attempt.Checkout)
		fmt.Fprintf(stdout, "  started:  %s\n", attempt.StartedAt.Format("2006-01-02 15:04:05 MST"))
		return 0

	case "submit":
		flags := flag.NewFlagSet("exercise submit", flag.ContinueOnError)
		flags.SetOutput(stderr)
		recordDir := flags.String("record", "", "the progress record directory")
		vaultRepo := flags.String("vault-repo", "", "the git repository the Vault ref lives in")
		vaultRef := flags.String("vault-ref", "vault", "the orphan ref the Vault image is built from")
		vaultImage := flags.String("vault-image", "", "the tag the Vault image is built and run under")
		if err := flags.Parse(args); err != nil {
			return 2
		}
		if *recordDir == "" || *vaultRepo == "" || *vaultImage == "" {
			fmt.Fprintln(stderr, "tracer: --record, --vault-repo and --vault-image are all required")
			return 2
		}

		loop := exercise.Loop{Record: record.Store{Dir: *recordDir}}

		// Checked before the Vault image is built, so refusing costs no
		// Docker invocation — the same "refusal leaves no partial state"
		// shape Start already follows.
		if _, open, err := loop.Status(); err != nil {
			fmt.Fprintf(stderr, "tracer: reading Exercise status: %v\n", err)
			return 2
		} else if !open {
			fmt.Fprintln(stderr, "tracer: no Exercise is open — nothing to submit")
			return 1
		}

		client := vault.Client{RepoDir: *vaultRepo, Ref: *vaultRef, Image: *vaultImage}
		if err := client.Build(); err != nil {
			fmt.Fprintf(stderr, "tracer: building the Vault image: %v\n", err)
			return 2
		}

		result, err := loop.Submit(client)
		switch {
		case errors.Is(err, exercise.ErrNoExerciseOpen):
			fmt.Fprintln(stderr, "tracer: no Exercise is open — nothing to submit")
			return 1
		case errors.Is(err, exercise.ErrWrongBranch):
			fmt.Fprintf(stderr, "tracer: %v — submit refused\n", err)
			return 1
		case err != nil:
			fmt.Fprintf(stderr, "tracer: submitting the fix: %v\n", err)
			return 2
		}

		if result.Cleared {
			fmt.Fprintln(stdout, "Cleared — the hidden test passed with the Agent boundary intact")
			return 0
		}
		fmt.Fprintln(stdout, "Not cleared")
		if result.Boundary != agentboundary.Intact {
			fmt.Fprintf(stdout, "  Agent boundary: %s\n", result.Boundary)
		}
		return 1

	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}
