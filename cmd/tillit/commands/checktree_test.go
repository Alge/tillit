package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alge/tillit/ecosystems"
	"github.com/Alge/tillit/resolver"
)

func mkRow(pkgID, version string, direct bool, status resolver.Status) row {
	return row{
		Pkg:    ecosystems.PackageRef{Ecosystem: "go", PackageID: pkgID, Version: version, Direct: direct},
		Status: status,
	}
}

func TestRenderTree_NestsTransitivesUnderDirect(t *testing.T) {
	rows := []row{
		mkRow("github.com/cloudflare/circl", "v1.6.3", true, resolver.StatusUnknown),
		mkRow("github.com/cespare/xxhash/v2", "v2.3.0", false, resolver.StatusUnknown),
		mkRow("golang.org/x/sync", "v0.10.0", false, resolver.StatusVetted),
		mkRow("golang.org/x/crypto", "v0.30.0", false, resolver.StatusVetted),
	}
	edges := map[string][]string{
		"github.com/cloudflare/circl@v1.6.3": {
			"github.com/cespare/xxhash/v2@v2.3.0",
			"golang.org/x/crypto@v0.30.0",
		},
		"github.com/cespare/xxhash/v2@v2.3.0": {"golang.org/x/sync@v0.10.0"},
	}

	var buf bytes.Buffer
	renderTree(&buf, rows, edges)
	out := buf.String()

	// Direct dep at column 0 — no trailing marker, position conveys it.
	if !strings.Contains(out, "github.com/cloudflare/circl v1.6.3 [unknown]\n") {
		t.Errorf("expected direct root rendered without marker, got:\n%s", out)
	}
	// Transitive nested under direct.
	if !strings.Contains(out, "├── github.com/cespare/xxhash/v2 v2.3.0 [unknown]") {
		t.Errorf("expected nested xxhash branch, got:\n%s", out)
	}
	// Grand-transitive nested two levels deep.
	if !strings.Contains(out, "│   └── golang.org/x/sync v0.10.0 [vetted]") {
		t.Errorf("expected double-nested sync leaf, got:\n%s", out)
	}
	// Last child uses └── instead of ├──
	if !strings.Contains(out, "└── golang.org/x/crypto v0.30.0 [vetted]") {
		t.Errorf("expected last-child └── on crypto, got:\n%s", out)
	}
	// No legend / direct-marker — keep output clean.
	if strings.Contains(out, "(*)") || strings.Contains(out, "* = direct") {
		t.Errorf("expected no markers in clean output, got:\n%s", out)
	}
}

func TestRenderTree_DiamondShownInFullUnderEachParent(t *testing.T) {
	// A and B both depend on C. We now expand fully: C appears under
	// both, and C's own subtree (here: D) is shown both times too.
	rows := []row{
		mkRow("a", "v1", true, resolver.StatusUnknown),
		mkRow("b", "v1", true, resolver.StatusUnknown),
		mkRow("c", "v1", false, resolver.StatusVetted),
		mkRow("d", "v1", false, resolver.StatusVetted),
	}
	edges := map[string][]string{
		"a@v1": {"c@v1"},
		"b@v1": {"c@v1"},
		"c@v1": {"d@v1"},
	}

	var buf bytes.Buffer
	renderTree(&buf, rows, edges)
	out := buf.String()

	if got := strings.Count(out, "c v1 [vetted]"); got != 2 {
		t.Errorf("expected c shown twice (once under each parent), got %d:\n%s", got, out)
	}
	if got := strings.Count(out, "d v1 [vetted]"); got != 2 {
		t.Errorf("expected d expanded under c both times, got %d:\n%s", got, out)
	}
	if strings.Contains(out, "(*)") {
		t.Errorf("dedupe marker should not appear, got:\n%s", out)
	}
}

func TestRenderTree_OrphansListedAtBottom(t *testing.T) {
	// "orphan" has no edge into it but is in the resolved set — must
	// still appear so it's not lost.
	rows := []row{
		mkRow("a", "v1", true, resolver.StatusUnknown),
		mkRow("orphan", "v1", false, resolver.StatusUnknown),
	}
	edges := map[string][]string{}

	var buf bytes.Buffer
	renderTree(&buf, rows, edges)
	out := buf.String()

	if !strings.Contains(out, "(packages not reached from any direct dependency)") {
		t.Errorf("expected orphan section header, got:\n%s", out)
	}
	if !strings.Contains(out, "orphan v1 [unknown]") {
		t.Errorf("expected orphan listed, got:\n%s", out)
	}
}

func TestFormatSummary_SplitsDirectAndIndirect(t *testing.T) {
	rows := []row{
		mkRow("a", "v1", true, resolver.StatusUnknown),
		mkRow("b", "v1", true, resolver.StatusVetted),
		mkRow("c", "v1", false, resolver.StatusAllowed),
		mkRow("d", "v1", false, resolver.StatusUnknown),
		mkRow("e", "v1", false, resolver.StatusUnknown),
	}
	got := formatSummary(rows)

	if !strings.Contains(got, "Direct") {
		t.Errorf("expected a Direct line, got:\n%s", got)
	}
	if !strings.Contains(got, "Indirect") {
		t.Errorf("expected an Indirect line, got:\n%s", got)
	}
	// Direct: 1 unknown, 1 vetted
	if !strings.Contains(got, "1 unknown") || !strings.Contains(got, "1 vetted") {
		t.Errorf("expected direct counts in summary, got:\n%s", got)
	}
	// Indirect: 1 allowed, 2 unknown
	if !strings.Contains(got, "2 unknown") || !strings.Contains(got, "1 allowed") {
		t.Errorf("expected indirect counts, got:\n%s", got)
	}
}

func TestFormatSummary_AlwaysShowsAllStatuses(t *testing.T) {
	rows := []row{
		mkRow("a", "v1", true, resolver.StatusUnknown),
	}
	got := formatSummary(rows)
	for _, want := range []string{"0 rejected", "1 unknown", "0 allowed", "0 vetted"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in summary, got:\n%s", want, got)
		}
	}
}

func TestFindLockfile_SingleMatch(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "go.sum", "")
	mustWrite(t, dir, "README.md", "") // ignored

	got, err := findLockfile(dir, adapters)
	if err != nil {
		t.Fatalf("findLockfile failed: %v", err)
	}
	if filepath.Base(got) != "go.sum" {
		t.Errorf("expected go.sum, got %q", got)
	}
}

func TestFindLockfile_NoMatch(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "README.md", "")

	_, err := findLockfile(dir, adapters)
	if err == nil {
		t.Error("expected error when no lockfile present")
	}
}

func TestFindLockfile_IgnoresSubdirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "go.sum"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := findLockfile(dir, adapters)
	if err == nil {
		t.Error("a subdirectory named go.sum should not count as a lockfile")
	}
}

func TestParseCheckArgs(t *testing.T) {
	cases := []struct {
		name           string
		args           []string
		wantEcosystem  string
		wantTarget     string
		wantErr        bool
	}{
		{"empty defaults", nil, "", ".", false},
		{"path only", []string{"./somedir"}, "", "./somedir", false},
		{"flag short", []string{"-e", "go"}, "go", ".", false},
		{"flag long", []string{"--ecosystem", "go"}, "go", ".", false},
		{"flag long equals", []string{"--ecosystem=go"}, "go", ".", false},
		{"flag short equals", []string{"-e=go"}, "go", ".", false},
		{"flag plus path", []string{"-e", "go", "./d"}, "go", "./d", false},
		{"path then flag", []string{"./d", "-e", "go"}, "go", "./d", false},
		{"missing flag value", []string{"-e"}, "", "", true},
		{"unknown flag", []string{"--bogus"}, "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eco, tgt, err := parseCheckArgs(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if eco != tc.wantEcosystem {
				t.Errorf("ecosystem = %q, want %q", eco, tc.wantEcosystem)
			}
			if tgt != tc.wantTarget {
				t.Errorf("target = %q, want %q", tgt, tc.wantTarget)
			}
		})
	}
}

func mustWrite(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestRenderTree_HandlesCycle(t *testing.T) {
	// Defensive: cycle a→b→a should not infinite-loop. Each side gets
	// printed once at the top of the cycle and recursion stops.
	rows := []row{
		mkRow("a", "v1", true, resolver.StatusUnknown),
		mkRow("b", "v1", false, resolver.StatusUnknown),
	}
	edges := map[string][]string{
		"a@v1": {"b@v1"},
		"b@v1": {"a@v1"},
	}

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		renderTree(&buf, rows, edges)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("renderTree appears to be in an infinite loop")
	}
	out := buf.String()
	if !strings.Contains(out, "a v1 [unknown]") || !strings.Contains(out, "└── b v1 [unknown]") {
		t.Errorf("expected cycle to render once before stopping, got:\n%s", out)
	}
}
