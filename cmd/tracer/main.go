// Command tracer is the local CLI the Exercise loop is driven through
// (ADR-0004). It currently carries the Agent boundary verbs only.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/RioPramana21/Tracer/internal/agentboundary"
)

const usage = `tracer — the Exercise loop

Usage:
  tracer boundary arm     --checkout <dir> --record <file>
  tracer boundary verify  --checkout <dir> --record <file>
  tracer boundary disarm  --checkout <dir> --record <file>

The Agent boundary is armed on an Exercise checkout by writing harness deny
rules over Playground paths, and a digest of the settings carrying them is
recorded outside the checkout. Verifying re-derives that digest.

Exit codes:
  0  the command succeeded, and for verify the boundary is intact
  1  the boundary is lifted or was never armed
  2  the command could not be run
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) < 2 || args[0] != "boundary" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	verb := args[1]

	flags := flag.NewFlagSet("boundary "+verb, flag.ContinueOnError)
	flags.SetOutput(stderr)
	checkoutRoot := flags.String("checkout", ".", "the Exercise checkout the boundary applies to")
	recordPath := flags.String("record", "", "where the digest of the armed settings is recorded")
	if err := flags.Parse(args[2:]); err != nil {
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
