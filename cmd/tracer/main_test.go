package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The CLI process boundary is the seam. Every test here drives the built binary
// the way the learner does and asserts on printed output and on the files the
// Agent boundary leaves behind.

var tracerBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tracer-bin")
	if err != nil {
		panic(err)
	}
	tracerBin = filepath.Join(dir, "tracer")
	if runtime.GOOS == "windows" {
		tracerBin += ".exe"
	}
	build := exec.Command("go", "build", "-o", tracerBin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("building tracer: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type result struct {
	stdout string
	stderr string
	code   int
}

func runTracer(t *testing.T, args ...string) result {
	t.Helper()
	cmd := exec.Command(tracerBin, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("running %v: %v", args, err)
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: cmd.ProcessState.ExitCode()}
}

// checkout returns a fixture Exercise checkout and the path the digest is
// recorded at, which sits outside the checkout as ADR-0004 requires.
func checkout(t *testing.T) (root, record string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "checkout")
	if err := os.MkdirAll(filepath.Join(root, "Playground"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(base, "record", "boundary.json")
}

func arm(t *testing.T, root, record string) result {
	t.Helper()
	got := runTracer(t, "boundary", "arm", "--checkout", root, "--record", record)
	if got.code != 0 {
		t.Fatalf("arm failed: %d %s%s", got.code, got.stdout, got.stderr)
	}
	return got
}

func settingsPath(root string) string {
	return filepath.Join(root, ".claude", "settings.local.json")
}

func readSettings(t *testing.T, root string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(settingsPath(root))
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("settings is not valid JSON: %v", err)
	}
	return settings
}

func denyRules(t *testing.T, root string) []string {
	t.Helper()
	settings := readSettings(t, root)
	permissions, ok := settings["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("settings carries no permissions object: %v", settings)
	}
	raw, ok := permissions["deny"].([]any)
	if !ok {
		t.Fatalf("permissions carries no deny array: %v", permissions)
	}
	rules := make([]string, 0, len(raw))
	for _, rule := range raw {
		rules = append(rules, rule.(string))
	}
	return rules
}

func TestArmDeniesSourceReadingOverPlaygroundPaths(t *testing.T) {
	root, record := checkout(t)

	arm(t, root, record)

	rules := denyRules(t, root)
	// Read rules are what the harness resolves to a file path: they cover the
	// Read and Edit tools, Grep and Glob best-effort, @file mentions, and the
	// Bash file commands it recognises.
	for _, want := range []string{"Read(/Playground/**)", "Read(Playground/**)"} {
		if !slices.Contains(rules, want) {
			t.Errorf("armed boundary does not deny %s; rules were %v", want, rules)
		}
	}
}

func TestArmDeniesReadPathsThatResolveToNoFilePath(t *testing.T) {
	root, record := checkout(t)

	arm(t, root, record)

	rules := denyRules(t, root)
	// git plumbing reads blobs rather than working-tree files, so a Read rule
	// never sees a path to match; read-only git forms also run unprompted.
	// PowerShell cmdlets are matched by PowerShell rules, which Read rules do
	// not cover, so the tool is denied whole.
	for _, want := range []string{"Bash(git show *)", "Bash(git cat-file *)", "PowerShell"} {
		if !slices.Contains(rules, want) {
			t.Errorf("armed boundary does not deny %s; rules were %v", want, rules)
		}
	}
}

func TestArmRecordsADigestOfTheSettingsCarryingTheRules(t *testing.T) {
	root, record := checkout(t)

	arm(t, root, record)

	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("reading record: %v", err)
	}
	var recorded struct {
		Digest   string `json:"digest"`
		Settings string `json:"settings"`
	}
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatalf("record is not valid JSON: %v", err)
	}
	if !strings.HasPrefix(recorded.Digest, "sha256:") {
		t.Errorf("digest %q is not a sha256 digest", recorded.Digest)
	}
	if recorded.Settings != settingsPath(root) {
		t.Errorf("record names settings at %q, want %q", recorded.Settings, settingsPath(root))
	}
}

// checkoutWithSettings is a checkout that already carries settings unrelated
// to the boundary, so tests can assert arming or disarming leaves them alone.
func checkoutWithSettings(t *testing.T, existing string) (root, record string) {
	t.Helper()
	root, record = checkout(t)
	if err := os.MkdirAll(filepath.Dir(settingsPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath(root), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, record
}

func TestArmLeavesUnrelatedSettingsAlone(t *testing.T) {
	root, record := checkoutWithSettings(t, `{"model":"opus","permissions":{"deny":["Bash(rm -rf *)"]}}`)

	arm(t, root, record)

	settings := readSettings(t, root)
	if settings["model"] != "opus" {
		t.Errorf("arming dropped an unrelated setting: %v", settings)
	}
	if !slices.Contains(denyRules(t, root), "Bash(rm -rf *)") {
		t.Errorf("arming dropped an unrelated deny rule: %v", denyRules(t, root))
	}
}

func TestVerifyReportsAnUnbrokenBoundary(t *testing.T) {
	root, record := checkout(t)
	arm(t, root, record)

	got := runTracer(t, "boundary", "verify", "--checkout", root, "--record", record)

	if got.code != 0 {
		t.Errorf("verify exited %d, want 0; output %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "intact") {
		t.Errorf("verify said %q, want it to report an intact boundary", got.stdout)
	}
}

func TestVerifyReportsALiftedBoundaryWhenARuleIsEdited(t *testing.T) {
	root, record := checkout(t)
	arm(t, root, record)
	settings := readSettings(t, root)
	settings["permissions"] = map[string]any{"deny": []any{"Read(/nothing/**)"}}
	edited, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath(root), edited, 0o644); err != nil {
		t.Fatal(err)
	}

	got := runTracer(t, "boundary", "verify", "--checkout", root, "--record", record)

	if got.code == 0 {
		t.Errorf("verify exited 0 over an edited boundary; output %s%s", got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "lifted") {
		t.Errorf("verify said %q, want it to report a lifted boundary", got.stdout)
	}
}

func TestVerifyReportsALiftedBoundaryWhenTheSettingsAreDeleted(t *testing.T) {
	root, record := checkout(t)
	arm(t, root, record)
	if err := os.Remove(settingsPath(root)); err != nil {
		t.Fatal(err)
	}

	got := runTracer(t, "boundary", "verify", "--checkout", root, "--record", record)

	if got.code == 0 {
		t.Errorf("verify exited 0 with the settings deleted; output %s%s", got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "lifted") {
		t.Errorf("verify said %q, want it to report a lifted boundary", got.stdout)
	}
}

func TestVerifyDistinguishesNeverArmedFromLifted(t *testing.T) {
	root, record := checkout(t)

	got := runTracer(t, "boundary", "verify", "--checkout", root, "--record", record)

	if got.code == 0 {
		t.Errorf("verify exited 0 with no boundary armed; output %s%s", got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "unarmed") {
		t.Errorf("verify said %q, want it to report an unarmed boundary", got.stdout)
	}
	if strings.Contains(got.stdout, "lifted") {
		t.Errorf("verify called a never-armed boundary lifted: %q", got.stdout)
	}
}

func TestVerifyIgnoresAnUnrelatedSettingAddedAfterArming(t *testing.T) {
	root, record := checkout(t)
	arm(t, root, record)
	settings := readSettings(t, root)
	settings["outputStyle"] = "concise"
	settings["permissions"].(map[string]any)["allow"] = []any{"Bash(go test *)"}
	edited, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath(root), edited, 0o644); err != nil {
		t.Fatal(err)
	}

	got := runTracer(t, "boundary", "verify", "--checkout", root, "--record", record)

	if got.code != 0 {
		t.Errorf("verify exited %d over an unrelated settings addition; output %s%s", got.code, got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "intact") {
		t.Errorf("verify said %q, want it to still report an intact boundary", got.stdout)
	}
}

func editDenyRules(t *testing.T, root string, deny []string) {
	t.Helper()
	settings := readSettings(t, root)
	settings["permissions"] = map[string]any{"deny": deny}
	edited, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath(root), edited, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyKeepsALiftedRecordAcrossARearm(t *testing.T) {
	root, record := checkout(t)
	arm(t, root, record)
	editDenyRules(t, root, []string{"Read(/nothing/**)"})
	if got := runTracer(t, "boundary", "verify", "--checkout", root, "--record", record); got.code == 0 {
		t.Fatalf("sanity check failed: verify did not observe the lift; output %s%s", got.stdout, got.stderr)
	}

	arm(t, root, record) // restores every rule the boundary armed

	got := runTracer(t, "boundary", "verify", "--checkout", root, "--record", record)
	if got.code == 0 {
		t.Errorf("re-arming a lifted boundary produced a clean record; output %s%s", got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "lifted") {
		t.Errorf("verify said %q after a re-arm, want it to still report the earlier lift", got.stdout)
	}
}

func TestArmObservesALiftEvenWithoutAnInterveningVerify(t *testing.T) {
	root, record := checkout(t)
	arm(t, root, record)
	editDenyRules(t, root, []string{"Read(/nothing/**)"})

	arm(t, root, record) // no verify call between the edit and this re-arm

	got := runTracer(t, "boundary", "verify", "--checkout", root, "--record", record)
	if got.code == 0 {
		t.Errorf("re-arming after an unverified lift produced a clean record; output %s%s", got.stdout, got.stderr)
	}
	if !strings.Contains(got.stdout, "lifted") {
		t.Errorf("verify said %q, want arm to have already caught the lift itself", got.stdout)
	}
}

func TestDisarmRemovesTheRules(t *testing.T) {
	root, record := checkout(t)
	arm(t, root, record)

	got := runTracer(t, "boundary", "disarm", "--checkout", root, "--record", record)

	if got.code != 0 {
		t.Fatalf("disarm exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}
	if _, err := os.Stat(settingsPath(root)); !os.IsNotExist(err) {
		t.Errorf("disarm left settings behind that arming created: %v", err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Errorf("disarm left the digest record behind: %v", err)
	}
}

func TestDisarmKeepsUnrelatedSettings(t *testing.T) {
	root, record := checkoutWithSettings(t, `{"model":"opus","permissions":{"deny":["Bash(rm -rf *)"]}}`)
	arm(t, root, record)

	if got := runTracer(t, "boundary", "disarm", "--checkout", root, "--record", record); got.code != 0 {
		t.Fatalf("disarm exited %d; output %s%s", got.code, got.stdout, got.stderr)
	}

	settings := readSettings(t, root)
	if settings["model"] != "opus" {
		t.Errorf("disarm dropped an unrelated setting: %v", settings)
	}
	rules := denyRules(t, root)
	if !slices.Contains(rules, "Bash(rm -rf *)") {
		t.Errorf("disarm dropped an unrelated deny rule: %v", rules)
	}
	if slices.Contains(rules, "Read(/Playground/**)") {
		t.Errorf("disarm left a boundary rule behind: %v", rules)
	}
}
