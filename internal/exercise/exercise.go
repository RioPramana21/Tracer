// Package exercise is the Exercise loop (cmd/tracer's own doc comment,
// ADR-0004): the orchestrator that composes the Catalog, the progress
// record, the Playground checkout, and the Agent boundary into next, start
// and status. It is where the loop's decisions live, the same way
// agentboundary.Boundary — not main.go — owns the boundary's decisions.
package exercise

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/RioPramana21/Tracer/internal/agentboundary"
	"github.com/RioPramana21/Tracer/internal/catalog"
	"github.com/RioPramana21/Tracer/internal/gitrepo"
	"github.com/RioPramana21/Tracer/internal/record"
)

// ErrAlreadyOpen is returned by Start when an Exercise is already open.
var ErrAlreadyOpen = errors.New("an Exercise is already open")

// ErrCatalogExhausted is returned by Start when the Catalog has no Exercise
// left to offer.
var ErrCatalogExhausted = errors.New("the Catalog has no Exercise left to offer")

// Loop is one learner's view of the Exercise loop: a Catalog to draw from and
// the progress record of what they have already attempted.
type Loop struct {
	Catalog catalog.Catalog
	Record  record.Store
}

// Load reads the Catalog at catalogPath and opens the progress record at
// recordDir.
func Load(catalogPath, recordDir string) (Loop, error) {
	c, err := catalog.Load(catalogPath)
	if err != nil {
		return Loop{}, err
	}
	return Loop{Catalog: c, Record: record.Store{Dir: recordDir}}, nil
}

// Next returns the next Exercise the learner has not yet attempted.
func (l Loop) Next() (catalog.Entry, bool, error) {
	used, err := l.Record.Used()
	if err != nil {
		return catalog.Entry{}, false, err
	}
	entry, ok := l.Catalog.Next(used)
	return entry, ok, nil
}

// Start opens the next Exercise: it cuts a fix branch from the Exercise's
// fixed baseline into a fresh clone of playgroundSrc at
// <checkoutDir>/Playground, arms the Agent boundary over checkoutDir, and
// records a new open Attempt.
//
// Refuses with ErrAlreadyOpen if an Exercise is already open, and with
// ErrCatalogExhausted if there is no next Exercise to start — both checked
// before anything is cloned or armed, so a refusal leaves no partial state.
func (l Loop) Start(playgroundSrc, checkoutDir string) (record.Attempt, agentboundary.Record, error) {
	if _, open, err := l.Record.Open(); err != nil {
		return record.Attempt{}, agentboundary.Record{}, err
	} else if open {
		return record.Attempt{}, agentboundary.Record{}, ErrAlreadyOpen
	}

	entry, ok, err := l.Next()
	if err != nil {
		return record.Attempt{}, agentboundary.Record{}, err
	}
	if !ok {
		return record.Attempt{}, agentboundary.Record{}, ErrCatalogExhausted
	}

	branch := "exercise/" + entry.ID
	playgroundDir := filepath.Join(checkoutDir, "Playground")
	if err := gitrepo.Clone(playgroundSrc, playgroundDir); err != nil {
		return record.Attempt{}, agentboundary.Record{}, fmt.Errorf("cloning the Playground: %w", err)
	}
	if err := gitrepo.CreateBranch(playgroundDir, branch, entry.Baseline); err != nil {
		return record.Attempt{}, agentboundary.Record{}, fmt.Errorf("cutting the fix branch: %w", err)
	}

	// The boundary digest record lives inside the progress record's own
	// directory — "outside the checkout" (ADR-0004) doesn't require its own
	// separate git repo, and putting it here means Record.Write's commit
	// captures the boundary digest and the Attempt together, in one entry
	// in the record's history.
	boundary := agentboundary.Boundary{
		CheckoutRoot: checkoutDir,
		RecordPath:   filepath.Join(l.Record.Dir, entry.ID+".boundary.json"),
	}
	armed, err := boundary.Arm()
	if err != nil {
		return record.Attempt{}, agentboundary.Record{}, fmt.Errorf("arming the Agent boundary: %w", err)
	}

	attempt := record.Attempt{
		ExerciseID: entry.ID,
		Ticket:     entry.Ticket,
		State:      record.StateOpen,
		StartedAt:  time.Now().UTC(),
		Baseline:   entry.Baseline,
		Branch:     branch,
		Checkout:   checkoutDir,
	}
	if err := l.Record.Write(attempt); err != nil {
		return record.Attempt{}, agentboundary.Record{}, err
	}
	return attempt, armed, nil
}

// Status returns the open Attempt, if any.
func (l Loop) Status() (record.Attempt, bool, error) {
	return l.Record.Open()
}
