// Package catalog holds the fixed, ordered Catalog of authored Exercises
// (ADR-0002, CONTEXT.md). It is Tooling, not Vault: a Ticket names no file,
// module or symbol, so the Catalog carries it directly rather than reaching it
// through the Vault boundary.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Entry is one authored Exercise: its id, the Ticket presented to the
// learner, and the Playground commit its fix branch is cut from.
type Entry struct {
	ID       string `json:"id"`
	Ticket   string `json:"ticket"`
	Baseline string `json:"baseline"`
}

// Catalog is the fixed, ordered set of Entries.
type Catalog struct {
	Entries []Entry `json:"entries"`
}

// Load reads a Catalog from a JSON file.
//
// Validated here, at the trust boundary (STD-005), rather than left to fail
// wherever an Entry is first used: an Entry's id becomes both a branch name
// and a progress-record filename, and a Baseline that doesn't check out is
// only discovered mid-Start — after the Playground has already been cloned,
// leaving a checkout no retry can reuse.
func Load(path string) (Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("reading catalog: %w", err)
	}
	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return Catalog{}, fmt.Errorf("catalog at %s is not valid JSON: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return Catalog{}, fmt.Errorf("catalog at %s: %w", path, err)
	}
	return c, nil
}

func (c Catalog) validate() error {
	seen := make(map[string]bool, len(c.Entries))
	for i, entry := range c.Entries {
		if entry.ID == "" {
			return fmt.Errorf("entry %d: id is required", i)
		}
		if strings.ContainsAny(entry.ID, `/\`) {
			return fmt.Errorf("entry %d: id %q contains a path separator — it names both a branch and a record file", i, entry.ID)
		}
		if seen[entry.ID] {
			return fmt.Errorf("entry %d: id %q is not unique", i, entry.ID)
		}
		seen[entry.ID] = true
		if entry.Ticket == "" {
			return fmt.Errorf("entry %d (%s): ticket is required", i, entry.ID)
		}
		if entry.Baseline == "" {
			return fmt.Errorf("entry %d (%s): baseline is required", i, entry.ID)
		}
	}
	return nil
}

// Next returns the first Entry, in Catalog order, whose id is not in used —
// the next Exercise to offer someone who has already attempted every id in
// used, regardless of how those attempts ended.
func (c Catalog) Next(used map[string]bool) (Entry, bool) {
	for _, entry := range c.Entries {
		if !used[entry.ID] {
			return entry, true
		}
	}
	return Entry{}, false
}
