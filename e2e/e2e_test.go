//go:build e2e

// Package e2e runs end-to-end tests against the compiled cncf-maintainers binary.
// Tests require network access to fetch the live CNCF maintainers CSV.
// Run with: go test -v -tags e2e ./e2e/
package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// knownMaintainer and knownMaintainer2 are GitHub usernames that have been CNCF
// maintainers for many years and are reliably present in project-maintainers.csv.
const (
	knownMaintainer  = "thockin" // Tim Hockin, Kubernetes
	knownMaintainer2 = "liggitt" // Jordan Liggitt, Kubernetes
)

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "cncf-maintainers-e2e-*")
	if err != nil {
		panic("creating temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	binaryPath = filepath.Join(tmp, "cncf-maintainers")
	root := filepath.Join("..")
	out, err := exec.Command("go", "build", "-o", binaryPath, root).CombinedOutput()
	if err != nil {
		panic("building binary: " + string(out))
	}

	os.Exit(m.Run())
}

// run executes the binary with the given arguments and returns stdout+stderr
// combined, plus the exit error (nil on exit 0).
func run(args ...string) (string, error) {
	cmd := exec.Command(binaryPath, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ── validate ─────────────────────────────────────────────────────────────────

func TestValidate_KnownMaintainer(t *testing.T) {
	out, err := run("validate", knownMaintainer)
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "[✓]") {
		t.Errorf("expected confirmed checkmark in output:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), strings.ToLower(knownMaintainer)) {
		t.Errorf("expected %q in output:\n%s", knownMaintainer, out)
	}
}

func TestValidate_UnknownMaintainer(t *testing.T) {
	out, err := run("validate", "this-user-definitely-does-not-exist-xyz123")
	if err == nil {
		t.Fatalf("expected non-zero exit for unknown maintainer, got success\noutput:\n%s", out)
	}
	if !strings.Contains(out, "[✗]") {
		t.Errorf("expected not-found marker in output:\n%s", out)
	}
}

func TestValidate_MultipleUsers_AllKnown(t *testing.T) {
	out, err := run("validate", knownMaintainer, knownMaintainer2)
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Summary:") {
		t.Errorf("expected summary line for multiple users:\n%s", out)
	}
}

func TestValidate_MultipleUsers_MixedResults(t *testing.T) {
	out, err := run("validate", knownMaintainer, "no-such-user-xyz999")
	if err == nil {
		t.Fatalf("expected non-zero exit when at least one user not found\noutput:\n%s", out)
	}
	if !strings.Contains(out, "[✓]") {
		t.Errorf("expected confirmed entry for known user:\n%s", out)
	}
	if !strings.Contains(out, "[✗]") {
		t.Errorf("expected not-found entry for unknown user:\n%s", out)
	}
}

func TestValidate_FileInput(t *testing.T) {
	f, err := os.CreateTemp("", "usernames-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString("# comment line\n\n" + knownMaintainer + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	out, err := run("validate", "--file", f.Name())
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "[✓]") {
		t.Errorf("expected confirmed checkmark:\n%s", out)
	}
}

func TestValidate_FileInput_Empty(t *testing.T) {
	f, err := os.CreateTemp("", "empty-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	out, err := run("validate", "--file", f.Name())
	if err == nil {
		t.Fatalf("expected non-zero exit for empty file\noutput:\n%s", out)
	}
}

func TestValidate_NoArgs(t *testing.T) {
	out, err := run("validate")
	if err == nil {
		t.Fatalf("expected non-zero exit with no arguments\noutput:\n%s", out)
	}
	if !strings.Contains(out, "required") {
		t.Errorf("expected 'required' in error output:\n%s", out)
	}
}

func TestValidate_FileAndPositional_MutuallyExclusive(t *testing.T) {
	f, err := os.CreateTemp("", "usernames-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(knownMaintainer + "\n")
	f.Close()

	out, err := run("validate", "--file", f.Name(), knownMaintainer)
	if err == nil {
		t.Fatalf("expected error when --file and positional args are both supplied\noutput:\n%s", out)
	}
	if !strings.Contains(out, "--file") {
		t.Errorf("expected --file mentioned in error output:\n%s", out)
	}
}

// ── add --dry-run ─────────────────────────────────────────────────────────────

func TestAdd_DryRun_KnownMaintainer(t *testing.T) {
	out, err := run("add", "--dry-run", knownMaintainer)
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected 'dry-run' in output:\n%s", out)
	}
	if !strings.Contains(out, "[✓]") {
		t.Errorf("expected confirmed checkmark for known maintainer:\n%s", out)
	}
}

func TestAdd_DryRun_UnknownMaintainer(t *testing.T) {
	out, err := run("add", "--dry-run", "no-such-user-xyz999")
	if err != nil {
		t.Fatalf("expected exit 0 even for unknown user in dry-run:\n%v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "[✗]") {
		t.Errorf("expected not-found marker:\n%s", out)
	}
}

func TestAdd_DryRun_MultipleUsers_Summary(t *testing.T) {
	out, err := run("add", "--dry-run", knownMaintainer, "no-such-user-xyz999")
	if err != nil {
		t.Fatalf("expected exit 0: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Summary") {
		t.Errorf("expected Summary block for multiple users:\n%s", out)
	}
}

func TestAdd_NoToken_NonDryRun(t *testing.T) {
	// Block all three token sources: env vars and the gh CLI fallback.
	// We prevent the gh fallback by pointing PATH at an empty directory so
	// exec.Command("gh", ...) fails with "executable not found".
	emptyDir := t.TempDir()
	cmd := exec.Command(binaryPath, "add", knownMaintainer)
	cmd.Env = append(
		filterEnv(os.Environ(), "GITHUB_TOKEN", "GH_TOKEN", "PATH"),
		"PATH="+emptyDir,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit without a token\noutput:\n%s", out)
	}
	if !strings.Contains(string(out), "token") {
		t.Errorf("expected token-related error message:\n%s", string(out))
	}
}

// ── misc ──────────────────────────────────────────────────────────────────────

func TestVersion(t *testing.T) {
	out, err := run("--version")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "cncf-maintainers") {
		t.Errorf("expected binary name in version output:\n%s", out)
	}
}

func TestHelp(t *testing.T) {
	out, err := run("--help")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	for _, sub := range []string{"validate", "add"} {
		if !strings.Contains(out, sub) {
			t.Errorf("expected subcommand %q in help output:\n%s", sub, out)
		}
	}
}

// filterEnv returns a copy of env with any entry whose key matches one of the
// given keys removed.
func filterEnv(env []string, keys ...string) []string {
	drop := make(map[string]bool, len(keys))
	for _, k := range keys {
		drop[k] = true
	}
	out := make([]string, 0, len(env))
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		if !drop[k] {
			out = append(out, e)
		}
	}
	return out
}
