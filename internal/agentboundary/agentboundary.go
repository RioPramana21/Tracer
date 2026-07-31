// Package agentboundary arms, verifies and disarms the Agent boundary: the rule
// that no agent reads Playground source while an Exercise is open, enforced at
// the harness rather than by an agent's cooperation (CONTEXT.md, ADR-0004).
//
// The boundary is not a wall. The learner owns the machine and can lift it.
// What arming buys is that lifting is visible: the digest recorded at arming
// no longer re-derives, and an Exercise worked with a lifted boundary cannot be
// a Clear.
//
// What the harness does and does not enforce is measured, not assumed — see
// docs/findings/0001-agent-boundary-enforceability.md.
package agentboundary

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Status is what verifying a checkout can conclude about its boundary.
type Status string

const (
	// Intact means the settings carrying the deny rules re-derive to the
	// digest recorded when the boundary was armed.
	Intact Status = "intact"
	// Lifted means the boundary was armed and its settings have since been
	// edited or deleted.
	Lifted Status = "lifted"
	// Unarmed means no boundary was ever armed for this checkout.
	Unarmed Status = "unarmed"
)

// DenyRules are the harness deny rules that carry the Agent boundary. Deny rules
// merge across every settings scope and are evaluated before allow rules, so
// nothing a later scope adds — and no PreToolUse hook — can lift one. The only
// way to lift these is to edit the file they are written to, which is exactly
// what the recorded digest detects.
//
// Two of the three groups below are deliberately broader than "Playground
// paths", and the inconsistency is intentional rather than missed. git reads
// content by ref rather than by path and PowerShell has its own rule namespace,
// so in neither case can the rule language express the intersection with
// Playground. Given STD-009 — a spoiled Exercise cannot be un-spoiled — the
// broad form is taken while the boundary is armed, which is only while an
// Exercise is open. The learner's own shell is unaffected either way: these
// constrain an attached agent's tool calls, not the terminal.
var DenyRules = []string{
	// Read rules are the mechanism. The harness resolves them to a file path
	// and applies them to the Read and Edit tools, best-effort to Grep and
	// Glob, to @file mentions, and to the Bash file commands it recognises
	// (cat, head, tail, sed). The anchored form holds wherever the session was
	// started from; the bare form, being a deny rule, also matches a nested
	// copy at any depth.
	"Read(/Playground/**)",
	"Read(Playground/**)",

	// git plumbing reads blobs rather than working-tree files, so a Read rule
	// never sees a path to match — and read-only git forms run without a
	// prompt, which makes this the cheapest way through the Read rules above.
	"Bash(git show *)",
	"Bash(git cat-file *)",
	"Bash(git grep *)",
	"Bash(git diff *)",
	"Bash(git log -p*)",
	"Bash(git log --patch*)",
	"Bash(git stash show *)",

	// PowerShell is a separate tool with its own rule namespace; Read rules do
	// not cover its file cmdlets. Denied whole rather than enumerated, because
	// enumerating cmdlets is the arms race the finding argues cannot be won.
	"PowerShell",
}

// The rules go in local settings rather than `.claude/settings.json`, which is
// a tracked file: arming must not dirty the fix branch the learner is graded on
// (ADR-0006). Deny rules merge identically from either scope.
const settingsRelPath = ".claude/settings.local.json"

// Boundary manages one Exercise checkout's boundary. RecordPath sits outside
// the checkout: ADR-0006 branches per Exercise and a checkout clobbers files,
// so the record of what was armed cannot live in the tree it describes.
type Boundary struct {
	CheckoutRoot string
	RecordPath   string
}

// Record is what arming leaves behind so that verifying has something to
// re-derive against.
//
// LiftedAt is what makes lifting *visible* rather than merely detectable. Once
// a lift is observed it is stamped here and never cleared, so re-arming cannot
// launder an Exercise back to a clean record — which is the whole guarantee the
// boundary trades for not being a wall. Deleting the record does not help
// either: a checkout with no record verifies Unarmed, which is not a Clear.
type Record struct {
	ArmedAt  time.Time  `json:"armed_at"`
	LiftedAt *time.Time `json:"lifted_at,omitempty"`
	Settings string     `json:"settings"`
	Digest   string     `json:"digest"`
	Rules    []string   `json:"rules"`
}

// SettingsPath is the settings file the deny rules are written to.
func (b Boundary) SettingsPath() string {
	return filepath.Join(b.CheckoutRoot, filepath.FromSlash(settingsRelPath))
}

func (b Boundary) absSettingsPath() (string, error) {
	abs, err := filepath.Abs(b.SettingsPath())
	if err != nil {
		return "", fmt.Errorf("resolving settings path: %w", err)
	}
	return abs, nil
}

// Arm writes the deny rules into the checkout's settings and records a digest
// of the rules carrying the boundary.
//
// Arming an already-armed checkout is idempotent, with one exception: if the
// boundary was lifted since it was armed, arming observes that first and
// carries the lift forward.
func (b Boundary) Arm() (Record, error) {
	settingsPath, err := b.absSettingsPath()
	if err != nil {
		return Record{}, err
	}

	// Evaluated before anything is overwritten, so that edit-read-rearm does
	// not erase the evidence of the edit.
	liftedAt, err := b.liftCarriedForward()
	if err != nil {
		return Record{}, err
	}

	settings, err := loadSettings(settingsPath)
	if err != nil {
		return Record{}, err
	}
	deny, err := denyRules(settings)
	if err != nil {
		return Record{}, err
	}
	for _, rule := range DenyRules {
		if !slices.Contains(deny, rule) {
			deny = append(deny, rule)
		}
	}
	if err := setDenyRules(settings, deny); err != nil {
		return Record{}, err
	}
	if err := writeSettings(settingsPath, settings); err != nil {
		return Record{}, err
	}

	record := Record{
		ArmedAt:  time.Now().UTC(),
		LiftedAt: liftedAt,
		Settings: settingsPath,
		Digest:   boundaryDigest(deny),
		Rules:    slices.Clone(DenyRules),
	}
	return record, b.writeRecord(record)
}

// liftCarriedForward reports the lift stamp a new record must inherit: an
// existing one if the boundary was already known lifted, a fresh one if it is
// lifted now, and nil if there is nothing to carry.
func (b Boundary) liftCarriedForward() (*time.Time, error) {
	record, err := b.readRecord()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if record.LiftedAt != nil {
		return record.LiftedAt, nil
	}
	status, err := b.evaluate(record)
	if err != nil {
		return nil, err
	}
	if status == Lifted {
		now := time.Now().UTC()
		return &now, nil
	}
	return nil, nil
}

// Verify re-derives the digest and reports whether the rules are still the ones
// that were armed. Observing a lift stamps it into the record, so the finding
// survives a later re-arm.
func (b Boundary) Verify() (Status, error) {
	record, err := b.readRecord()
	if errors.Is(err, os.ErrNotExist) {
		return Unarmed, nil
	}
	if err != nil {
		return "", err
	}
	if record.LiftedAt != nil {
		return Lifted, nil
	}

	status, err := b.evaluate(record)
	if err != nil {
		return "", err
	}
	if status == Lifted {
		now := time.Now().UTC()
		record.LiftedAt = &now
		if err := b.writeRecord(record); err != nil {
			return "", err
		}
	}
	return status, nil
}

// evaluate compares the checkout's current rules against the record, without
// consulting or updating the lift stamp.
func (b Boundary) evaluate(record Record) (Status, error) {
	settingsPath, err := b.absSettingsPath()
	if err != nil {
		return "", err
	}
	if record.Settings != settingsPath {
		return "", fmt.Errorf(
			"record at %s was armed over %s, not over this checkout's %s",
			b.RecordPath, record.Settings, settingsPath)
	}

	settings, err := loadSettings(settingsPath)
	if err != nil {
		return "", err
	}
	deny, err := denyRules(settings)
	if err != nil {
		return "", err
	}
	if boundaryDigest(deny) != record.Digest {
		return Lifted, nil
	}
	return Intact, nil
}

// boundaryDigest digests the boundary's own rules as they currently stand in
// the settings, in a fixed order, rather than the whole settings file. A rule
// removed or altered changes the digest; an unrelated setting added alongside
// does not. Digesting the file would be simpler and would report Lifted for an
// edit that never touched the boundary — and since a lift forecloses a Clear,
// that false positive costs the learner an Exercise they earned.
func boundaryDigest(deny []string) string {
	present := make([]string, 0, len(DenyRules))
	for _, rule := range DenyRules {
		if slices.Contains(deny, rule) {
			present = append(present, rule)
		}
	}
	return digestOf([]byte(strings.Join(present, "\n")))
}

// Disarm removes the rules and the digest record, leaving any settings the
// checkout already carried untouched.
func (b Boundary) Disarm() error {
	settings, err := loadSettings(b.SettingsPath())
	if err != nil {
		return err
	}
	deny, err := denyRules(settings)
	if err != nil {
		return err
	}
	kept := deny[:0]
	for _, rule := range deny {
		if !slices.Contains(DenyRules, rule) {
			kept = append(kept, rule)
		}
	}
	if err := setDenyRules(settings, kept); err != nil {
		return err
	}

	if len(settings) == 0 {
		if err := removeIfExists(b.SettingsPath()); err != nil {
			return err
		}
		// Only tidies away a directory arming created; a non-empty one fails
		// and is ignored.
		os.Remove(filepath.Dir(b.SettingsPath()))
	} else if err := writeSettings(b.SettingsPath(), settings); err != nil {
		return err
	}
	return removeIfExists(b.RecordPath)
}

// settings is a settings document held as raw JSON per key, so that arming a
// checkout that already has settings rewrites the permissions and nothing else.
type settings map[string]json.RawMessage

func loadSettings(path string) (settings, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading settings: %w", err)
	}
	doc := settings{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("settings at %s is not valid JSON: %w", path, err)
	}
	return doc, nil
}

func denyRules(doc settings) ([]string, error) {
	permissions, err := permissionsOf(doc)
	if err != nil {
		return nil, err
	}
	rawDeny, ok := permissions["deny"]
	if !ok {
		return nil, nil
	}
	var deny []string
	if err := json.Unmarshal(rawDeny, &deny); err != nil {
		return nil, fmt.Errorf("permissions.deny is not an array of rules: %w", err)
	}
	return deny, nil
}

func setDenyRules(doc settings, deny []string) error {
	permissions, err := permissionsOf(doc)
	if err != nil {
		return err
	}
	if len(deny) == 0 {
		delete(permissions, "deny")
	} else {
		encoded, err := json.Marshal(deny)
		if err != nil {
			return err
		}
		permissions["deny"] = encoded
	}

	if len(permissions) == 0 {
		delete(doc, "permissions")
		return nil
	}
	encoded, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	doc["permissions"] = encoded
	return nil
}

func permissionsOf(doc settings) (map[string]json.RawMessage, error) {
	raw, ok := doc["permissions"]
	if !ok {
		return map[string]json.RawMessage{}, nil
	}
	permissions := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &permissions); err != nil {
		return nil, fmt.Errorf("permissions is not an object: %w", err)
	}
	return permissions, nil
}

func writeSettings(path string, doc settings) error {
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating settings directory: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	return nil
}

func (b Boundary) writeRecord(record Record) error {
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(b.RecordPath), 0o755); err != nil {
		return fmt.Errorf("creating record directory: %w", err)
	}
	if err := os.WriteFile(b.RecordPath, encoded, 0o644); err != nil {
		return fmt.Errorf("writing record: %w", err)
	}
	return nil
}

func (b Boundary) readRecord() (Record, error) {
	raw, err := os.ReadFile(b.RecordPath)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(raw, &record); err != nil {
		return Record{}, fmt.Errorf("record at %s is not valid JSON: %w", b.RecordPath, err)
	}
	return record, nil
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func digestOf(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}
