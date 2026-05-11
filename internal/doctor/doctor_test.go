package doctor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// passing returns a check function that always succeeds with the given detail.
func passing(detail string) func() (string, error) {
	return func() (string, error) { return detail, nil }
}

// failing returns a check function that always fails with the given message.
func failing(msg string) func() (string, error) {
	return func() (string, error) { return "", errors.New(msg) }
}

func TestRunChecks_DownstreamChecksAreSkippedWhenDependencyFails(t *testing.T) {
	checks := []check{
		{name: "a", fn: passing("ok")},
		{name: "b", fn: failing("boom"), dependsOn: "a"},
		{name: "c", fn: passing("ok"), dependsOn: "b"},
		{name: "d", fn: passing("ok")}, // independent — should still run
	}
	results := runChecks(checks)

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	if !results[0].ok {
		t.Errorf("a should have passed")
	}
	if results[1].ok || results[1].skipped {
		t.Errorf("b should have failed (not skipped); got ok=%v skipped=%v", results[1].ok, results[1].skipped)
	}
	if !results[2].skipped {
		t.Errorf("c should be skipped (its dependency b failed)")
	}
	if !results[3].ok {
		t.Errorf("d is independent and should have passed")
	}
}

func TestRunChecks_AllPassWhenChainIsHealthy(t *testing.T) {
	checks := []check{
		{name: "a", fn: passing("a-ok")},
		{name: "b", fn: passing("b-ok"), dependsOn: "a"},
		{name: "c", fn: passing("c-ok"), dependsOn: "b"},
	}
	results := runChecks(checks)
	for _, r := range results {
		if !r.ok {
			t.Errorf("expected %s to pass, got ok=%v skipped=%v err=%v", r.name, r.ok, r.skipped, r.err)
		}
	}
}

func TestFirstFailure_PicksTheRootCauseNotADownstreamSkip(t *testing.T) {
	results := []*result{
		{name: "a", ok: true},
		{name: "b", err: errors.New("boom")},
		{name: "c", skipped: true},
	}
	first := firstFailure(results)
	if first == nil || first.name != "b" {
		t.Errorf("expected first failure to be 'b', got %v", first)
	}
}

func TestFirstFailure_ReturnsNilWhenAllPass(t *testing.T) {
	results := []*result{
		{name: "a", ok: true},
		{name: "b", ok: true},
	}
	if first := firstFailure(results); first != nil {
		t.Errorf("expected nil, got %v", first)
	}
}

func TestRenderReport_SuccessShowsAllChecksPassed(t *testing.T) {
	results := []*result{
		{name: "alpha", ok: true, detail: "looks great"},
		{name: "beta", ok: true, detail: "also great"},
	}
	var buf bytes.Buffer
	if err := renderReport(&buf, results); err != nil {
		t.Fatalf("renderReport returned err: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "looks great") {
		t.Errorf("missing alpha line: %q", out)
	}
	if !strings.Contains(out, "All checks passed.") {
		t.Errorf("missing success footer: %q", out)
	}
}

func TestRenderReport_FailureSurfacesProblemAndHint(t *testing.T) {
	results := []*result{
		{name: "transmission-daemon installed", ok: true, detail: "/usr/bin/transmission-daemon"},
		{name: "daemon running", err: errors.New("not running"), hint: "Start it:\n    brew services start transmission-cli"},
		{name: "RPC reachable", skipped: true},
	}
	var buf bytes.Buffer
	err := renderReport(&buf, results)
	if err == nil {
		t.Fatalf("expected error from renderReport on failure")
	}
	out := buf.String()

	if !strings.Contains(out, "Transmission daemon is not running.") {
		t.Errorf("expected human-readable problem line in output, got:\n%s", out)
	}
	if !strings.Contains(out, "brew services start transmission-cli") {
		t.Errorf("expected actionable hint in output, got:\n%s", out)
	}
	if !strings.Contains(out, "skipped") {
		t.Errorf("expected downstream check to appear as skipped, got:\n%s", out)
	}
}

func TestPrecheckChecks_BailsOnFirstStartupFailureWithHint(t *testing.T) {
	checks := []check{
		{name: "daemon running", fn: failing("not running"), hint: "Start it:\n    brew services start transmission-cli", requiredAtStartup: true},
		{name: "RPC reachable", fn: passing("ok"), requiredAtStartup: true},
	}
	msg, err := precheckChecks(checks)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(msg, "Transmission daemon is not running.") {
		t.Errorf("missing problem line: %q", msg)
	}
	if !strings.Contains(msg, "brew services start transmission-cli") {
		t.Errorf("missing hint command: %q", msg)
	}
	if !strings.Contains(msg, "lazytorrent --doctor") {
		t.Errorf("expected pointer to --doctor: %q", msg)
	}
}

func TestPrecheckChecks_IgnoresNonStartupChecks(t *testing.T) {
	// A non-startup check that would fail should NOT cause Precheck to fail.
	checks := []check{
		{name: "daemon running", fn: passing("ok"), requiredAtStartup: true},
		{name: "config file", fn: failing("missing"), requiredAtStartup: false},
	}
	msg, err := precheckChecks(checks)
	if err != nil {
		t.Errorf("expected no error (non-startup checks shouldn't block startup), got err=%v msg=%q", err, msg)
	}
}

func TestPrecheckChecks_AllPassReturnsEmpty(t *testing.T) {
	checks := []check{
		{name: "a", fn: passing("ok"), requiredAtStartup: true},
		{name: "b", fn: passing("ok"), requiredAtStartup: true},
	}
	msg, err := precheckChecks(checks)
	if err != nil || msg != "" {
		t.Errorf("expected ('', nil), got (%q, %v)", msg, err)
	}
}

func TestProblemLine_KnownChecksProduceHumanText(t *testing.T) {
	cases := map[string]string{
		"transmission-daemon installed": "transmission-daemon is not installed.",
		"daemon running":                "Transmission daemon is not running.",
		"RPC reachable":                 "Transmission RPC is not reachable.",
		"RPC authenticated":             "Transmission RPC authentication failed.",
		"default download directory":    "Default download directory is unusable.",
	}
	for name, want := range cases {
		got := problemLine(&result{name: name, err: errors.New("ignored")})
		if got != want {
			t.Errorf("problemLine(%q) = %q; want %q", name, got, want)
		}
	}
}

func TestExtractVersion(t *testing.T) {
	cases := map[string]string{
		"transmission-daemon 4.1.1 (56442e2929)": "4.1.1",
		"transmission-daemon (unknown)":          "",
		"":                                      "",
		"4.0.0":                                  "4.0.0",
	}
	for in, want := range cases {
		if got := extractVersion(in); got != want {
			t.Errorf("extractVersion(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024 * 1024 * 412, "412.0 GiB"},
	}
	for _, tc := range cases {
		if got := humanBytes(tc.in); got != tc.want {
			t.Errorf("humanBytes(%d) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestIndent_MultilineStringsIndentEveryLine(t *testing.T) {
	got := indent("foo\nbar", "    ")
	want := "    foo\n    bar"
	if got != want {
		t.Errorf("indent: got %q want %q", got, want)
	}
}
