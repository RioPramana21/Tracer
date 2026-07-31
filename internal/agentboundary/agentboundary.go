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

// Rules are the harness deny rules that carry the Agent boundary. Deny rules
// merge across every settings scope and are evaluated before allow rules, so
// nothing a later scope adds — and no PreToolUse hook — can lift one. The only
// way to lift these is to edit the file they are written to, which is exactly
// what the recorded digest detects.
var Rules = []string{
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

const settingsRelPath = ".claude/settings.json"

// Boundary manages one Exercise checkout's boundary. RecordPath sits outside
// the checkout: ADR-0006 branches per Exercise and a checkout clobbers files,
// so the record of what was armed cannot live in the tree it describes.
type Boundary struct {
	CheckoutRoot string
	RecordPath   string
}

// Record is what arming leaves behind so that verifying has something to
// re-derive against.
type Record struct {
	ArmedAt  time.Time `json:"armed_at"`
	Settings string    `json:"settings"`
	Digest   string    `json:"digest"`
	Rules    []string  `json:"rules"`
}

// SettingsPath is the settings file the deny rules are written to.
func (b Boundary) SettingsPath() string {
	return filepath.Join(b.CheckoutRoot, filepath.FromSlash(settingsRelPath))
}

// Arm writes the deny rules into the checkout's settings and records a digest
// of the file carrying them. Arming an already-armed checkout is idempotent.
func (b Boundary) Arm() (Record, error) {
	settings, err := loadSettings(b.SettingsPath())
	if err != nil {
		return Record{}, err
	}
	deny, err := denyRules(settings)
	if err != nil {
		return Record{}, err
	}
	for _, rule := range Rules {
		if !slices.Contains(deny, rule) {
			deny = append(deny, rule)
		}
	}
	if err := setDenyRules(settings, deny); err != nil {
		return Record{}, err
	}
	digest, err := writeSettings(b.SettingsPath(), settings)
	if err != nil {
		return Record{}, err
	}

	record := Record{
		ArmedAt:  time.Now().UTC(),
		Settings: b.SettingsPath(),
		Digest:   digest,
		Rules:    Rules,
	}
	return record, b.writeRecord(record)
}

// Verify re-derives the digest of the settings and reports whether the rules
// are still the ones that were armed.
func (b Boundary) Verify() (Status, error) {
	record, err := b.readRecord()
	if errors.Is(err, os.ErrNotExist) {
		return Unarmed, nil
	}
	if err != nil {
		return "", err
	}

	current, err := os.ReadFile(record.Settings)
	if errors.Is(err, os.ErrNotExist) {
		return Lifted, nil
	}
	if err != nil {
		return "", fmt.Errorf("reading settings: %w", err)
	}
	if digestOf(current) != record.Digest {
		return Lifted, nil
	}
	return Intact, nil
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
		if !slices.Contains(Rules, rule) {
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
	} else if _, err := writeSettings(b.SettingsPath(), settings); err != nil {
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

// writeSettings writes the document and returns the digest of the bytes it
// wrote. Go marshals map keys in sorted order, so the same document always
// produces the same bytes and therefore the same digest.
func writeSettings(path string, doc settings) (string, error) {
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating settings directory: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return "", fmt.Errorf("writing settings: %w", err)
	}
	return digestOf(encoded), nil
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
