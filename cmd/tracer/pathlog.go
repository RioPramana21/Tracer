package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/RioPramana21/Tracer/internal/record"
)

// pathLogTemplate is what the learner's editor opens on. Lines starting
// with # are stripped before parsing, the same convention git uses for
// commit message templates.
const pathLogTemplate = `# Path log entry — write your hypothesis and why you hold it.
# Lines starting with # are ignored. The entry is rejected if "Because" is
# left empty.
#
# A Location claim is optional: where you believe the Cause lives. It is
# recorded and never answered while the Exercise is open. Leave it blank if
# you have no claim to make.

Hypothesis:

Because:

Location claim:
`

func runPathLog(verb string, args []string, stdout, stderr io.Writer) int {
	switch verb {
	case "file":
		flags := flag.NewFlagSet("pathlog file", flag.ContinueOnError)
		flags.SetOutput(stderr)
		recordDir := flags.String("record", "", "the progress record directory")
		if err := flags.Parse(args); err != nil {
			return 2
		}
		if *recordDir == "" {
			fmt.Fprintln(stderr, "tracer: --record is required")
			return 2
		}

		store := record.Store{Dir: *recordDir}
		attempt, open, err := store.Open()
		if err != nil {
			fmt.Fprintf(stderr, "tracer: reading Exercise status: %v\n", err)
			return 2
		}
		if !open {
			fmt.Fprintln(stderr, "tracer: no Exercise is open — nothing to file a Path log entry against")
			return 1
		}

		tmpPath, err := writePathLogTemplate()
		if err != nil {
			fmt.Fprintf(stderr, "tracer: preparing the Path log entry: %v\n", err)
			return 2
		}
		defer os.Remove(tmpPath)

		if err := openEditor(tmpPath); err != nil {
			fmt.Fprintf(stderr, "tracer: opening the editor: %v\n", err)
			return 2
		}

		raw, err := os.ReadFile(tmpPath)
		if err != nil {
			fmt.Fprintf(stderr, "tracer: reading the Path log entry: %v\n", err)
			return 2
		}
		hypothesis, because, location := parsePathLogInput(string(raw))

		probe, err := store.AppendEntry(attempt.ExerciseID, record.PathLogEntry{
			Hypothesis: hypothesis,
			Because:    because,
			Location:   location,
		})
		switch {
		case errors.Is(err, record.ErrBecauseRequired):
			fmt.Fprintln(stderr, `tracer: Path log entry rejected — "because" is required`)
			return 1
		case err != nil:
			fmt.Fprintf(stderr, "tracer: filing the Path log entry: %v\n", err)
			return 2
		}

		fmt.Fprintf(stdout, "Path log entry filed as probe %d\n", probe)
		if location != "" {
			fmt.Fprintln(stdout, "  Location claim recorded — no indication of correctness while the Exercise is open")
		}
		return 0

	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func writePathLogTemplate() (string, error) {
	tmp, err := os.CreateTemp("", "tracer-pathlog-*.md")
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	if _, err := tmp.WriteString(pathLogTemplate); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

// openEditor opens path in the learner's editor and blocks until it exits.
// $EDITOR is split on whitespace so a value like "code --wait" works; path
// is appended as the final argument.
func openEditor(path string) error {
	parts := strings.Fields(os.Getenv("EDITOR"))
	if len(parts) == 0 {
		if runtime.GOOS == "windows" {
			parts = []string{"notepad"}
		} else {
			parts = []string{"vi"}
		}
	}
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// parsePathLogInput reads back what the learner wrote in the editor. Each
// labelled field runs from its "Label:" line up to the next labelled line
// (or the end of the file), so a hypothesis or a because can span several
// lines of prose. Comment lines are dropped everywhere, including inside a
// field's text.
//
// The three label strings here and pathLogTemplate's must be changed
// together — the template is what the learner's file starts as, and this is
// what reads it back.
func parsePathLogInput(raw string) (hypothesis, because, location string) {
	fields := []struct {
		prefix string
		target *string
	}{
		{"Hypothesis:", &hypothesis},
		{"Because:", &because},
		{"Location claim:", &location},
	}

	var current *string
	var buf strings.Builder
	flush := func() {
		if current != nil {
			*current = strings.TrimSpace(buf.String())
		}
		buf.Reset()
	}

	for line := range strings.SplitSeq(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		matched := false
		for _, f := range fields {
			if strings.HasPrefix(line, f.prefix) {
				flush()
				current = f.target
				buf.WriteString(strings.TrimPrefix(line, f.prefix))
				buf.WriteString("\n")
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if current != nil {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	flush()
	return hypothesis, because, location
}
