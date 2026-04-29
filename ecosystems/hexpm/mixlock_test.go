package hexpm_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/hexpm"
)

type hexpmPackage struct {
	ID, Version string
}

func parseMix(t *testing.T, content string) (pkgs []hexpmPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"mix.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (hexpm.MixLock{}).Parse(fsys, "mix.lock")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "hexpm" {
			t.Errorf("Ecosystem = %q, want hexpm", p.Ecosystem)
		}
		pkgs = append(pkgs, hexpmPackage{p.PackageID, p.Version})
	}
	return pkgs, res.Warnings
}

func anyContains(warns []string, sub string) bool {
	for _, w := range warns {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

func TestMixLock_Identity(t *testing.T) {
	a := hexpm.MixLock{}
	if a.Ecosystem() != "hexpm" {
		t.Errorf("Ecosystem() = %q, want hexpm", a.Ecosystem())
	}
	if a.Name() != "mix.lock" {
		t.Errorf("Name() = %q, want mix.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"mix.lock", true},
		{"./mix.lock", true},
		{"/some/path/mix.lock", true},
		{"mix.exs", false},
		{"go.sum", false},
		{"mix.lock.bak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestMixLock_Parse_BasicHexPackage(t *testing.T) {
	// One-line entry (rare in real lockfiles but handy for tests).
	pkgs, warnings := parseMix(t, `%{
  "phoenix": {:hex, :phoenix, "1.7.10", "innerhash", [:mix], [], "hexpm", "outerhash"},
}
`)
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].ID != "phoenix" || pkgs[0].Version != "1.7.10" {
		t.Errorf("unexpected: %+v", pkgs[0])
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestMixLock_Parse_MultiLineEntry(t *testing.T) {
	// Real mix.lock entries usually wrap across many lines.
	pkgs, _ := parseMix(t, `%{
  "phoenix": {:hex, :phoenix, "1.7.10",
   "02189140a61b2ce85bb633a9b6fd02dff705a5f1ab4b9ede29ce91a3eb9d1b25",
   [:mix],
   [{:phoenix_pubsub, "~> 2.1", [hex: :phoenix_pubsub, repo: "hexpm", optional: false]}],
   "hexpm",
   "outer..."},
  "ecto": {:hex, :ecto, "3.10.3",
   "innerhash",
   [:mix],
   [],
   "hexpm",
   "outerhash"},
}
`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].ID != "phoenix" || pkgs[0].Version != "1.7.10" {
		t.Errorf("first pkg: %+v", pkgs[0])
	}
	if pkgs[1].ID != "ecto" || pkgs[1].Version != "3.10.3" {
		t.Errorf("second pkg: %+v", pkgs[1])
	}
}

func TestMixLock_Parse_SkipsGitSource(t *testing.T) {
	pkgs, warnings := parseMix(t, `%{
  "ok": {:hex, :ok, "1.0.0", "h", [:mix], [], "hexpm", "o"},
  "from_git": {:git, "https://github.com/foo/bar.git", "abc123def", []},
}
`)
	if len(pkgs) != 1 || pkgs[0].ID != "ok" {
		t.Errorf("expected only hex pkg, got: %+v", pkgs)
	}
	if !anyContains(warnings, "from_git") {
		t.Errorf("expected warning mentioning git pkg, got: %v", warnings)
	}
}

func TestMixLock_Parse_SkipsPathSource(t *testing.T) {
	pkgs, warnings := parseMix(t, `%{
  "from_path": {:path, "../local_lib"},
}
`)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
	if !anyContains(warnings, "from_path") {
		t.Errorf("expected warning, got: %v", warnings)
	}
}

func TestMixLock_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parseMix(t, `%{
  "phoenix": {:hex, :phoenix, "1.7.10", "h", [:mix], [], "hexpm", "o"},
  "phoenix": {:hex, :phoenix, "1.7.10", "h", [:mix], [], "hexpm", "o"},
}
`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestMixLock_Parse_HandlesEmptyFile(t *testing.T) {
	pkgs, _ := parseMix(t, `%{}
`)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
}

func TestMixLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (hexpm.MixLock{}).Parse(fstest.MapFS{}, "mix.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestMixLock_Parse_MalformedEntryWarns(t *testing.T) {
	// Garbage between valid entries shouldn't take the file down —
	// just warn and skip.
	_, warnings := parseMix(t, `%{
  "ok": {:hex, :ok, "1.0.0", "h", [:mix], [], "hexpm", "o"},
  "junk": this is not a valid entry,
}
`)
	if !anyContains(warnings, "junk") {
		t.Errorf("expected warning about junk entry, got: %v", warnings)
	}
}
