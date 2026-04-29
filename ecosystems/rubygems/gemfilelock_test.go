package rubygems_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/rubygems"
)

type gemPackage struct {
	ID, Version string
}

func parseGemfile(t *testing.T, content string) (pkgs []gemPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"Gemfile.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (rubygems.GemfileLock{}).Parse(fsys, "Gemfile.lock")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "rubygems" {
			t.Errorf("Ecosystem = %q, want rubygems", p.Ecosystem)
		}
		pkgs = append(pkgs, gemPackage{p.PackageID, p.Version})
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

func TestGemfileLock_Identity(t *testing.T) {
	a := rubygems.GemfileLock{}
	if a.Ecosystem() != "rubygems" {
		t.Errorf("Ecosystem() = %q, want rubygems", a.Ecosystem())
	}
	if a.Name() != "Gemfile.lock" {
		t.Errorf("Name() = %q, want Gemfile.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"Gemfile.lock", true},
		{"./Gemfile.lock", true},
		{"/some/path/Gemfile.lock", true},
		{"Gemfile", false},
		{"gemfile.lock", false}, // case-sensitive — Bundler always emits uppercase G
		{"go.sum", false},
		{"Gemfile.lock.bak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestGemfileLock_Parse_Basic(t *testing.T) {
	pkgs, warnings := parseGemfile(t, `GEM
  remote: https://rubygems.org/
  specs:
    actionmailer (7.0.0)
      actionview (= 7.0.0)
      activejob (= 7.0.0)
    actionview (7.0.0)
      activesupport (= 7.0.0)
    rails (7.0.0)
      actionmailer (= 7.0.0)
      actionview (= 7.0.0)

PLATFORMS
  ruby

DEPENDENCIES
  rails

BUNDLED WITH
   2.4.10
`)
	if len(pkgs) != 3 {
		t.Fatalf("expected 3 packages, got %d: %+v", len(pkgs), pkgs)
	}
	got := map[string]string{}
	for _, p := range pkgs {
		got[p.ID] = p.Version
	}
	if got["rails"] != "7.0.0" || got["actionmailer"] != "7.0.0" || got["actionview"] != "7.0.0" {
		t.Errorf("unexpected: %+v", got)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestGemfileLock_Parse_SkipsGitSection(t *testing.T) {
	pkgs, warnings := parseGemfile(t, `GIT
  remote: https://github.com/foo/bar.git
  revision: abc123
  specs:
    from-git (0.1.0)

GEM
  remote: https://rubygems.org/
  specs:
    rails (7.0.0)
`)
	if len(pkgs) != 1 || pkgs[0].ID != "rails" {
		t.Errorf("expected only registry pkg, got: %+v", pkgs)
	}
	if !anyContains(warnings, "from-git") {
		t.Errorf("expected warning, got: %v", warnings)
	}
}

func TestGemfileLock_Parse_SkipsPathSection(t *testing.T) {
	pkgs, warnings := parseGemfile(t, `PATH
  remote: ./vendor/local
  specs:
    local-gem (0.1.0)

GEM
  remote: https://rubygems.org/
  specs:
    rails (7.0.0)
`)
	if len(pkgs) != 1 || pkgs[0].ID != "rails" {
		t.Errorf("expected only registry pkg, got: %+v", pkgs)
	}
	if !anyContains(warnings, "local-gem") {
		t.Errorf("expected warning, got: %v", warnings)
	}
}

func TestGemfileLock_Parse_HandlesPlatformSuffix(t *testing.T) {
	// Some gems publish per-platform builds; the spec line includes
	// the platform after the version: `nokogiri (1.15.0-x86_64-linux)`.
	// We treat platform-tagged builds as a separate version (so the
	// trust store can pin platform-specific binaries) — the lockfile
	// records them distinctly so the user vetted them distinctly.
	pkgs, _ := parseGemfile(t, `GEM
  remote: https://rubygems.org/
  specs:
    nokogiri (1.15.0)
    nokogiri (1.15.0-x86_64-linux)
`)
	if len(pkgs) != 2 {
		t.Errorf("expected both platform variants, got: %+v", pkgs)
	}
}

func TestGemfileLock_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parseGemfile(t, `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.0.0)
    rails (7.0.0)
`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestGemfileLock_Parse_KeepsDifferentVersions(t *testing.T) {
	pkgs, _ := parseGemfile(t, `GEM
  remote: https://rubygems.org/
  specs:
    nokogiri (1.14.0)
    nokogiri (1.15.0)
`)
	if len(pkgs) != 2 {
		t.Errorf("expected 2 versions, got: %+v", pkgs)
	}
}

func TestGemfileLock_Parse_PreReleaseGems(t *testing.T) {
	pkgs, _ := parseGemfile(t, `GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.0.rc1)
    nokogiri (1.16.0.beta.1)
`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got: %+v", pkgs)
	}
	got := map[string]string{}
	for _, p := range pkgs {
		got[p.ID] = p.Version
	}
	if got["rails"] != "7.1.0.rc1" || got["nokogiri"] != "1.16.0.beta.1" {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestGemfileLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (rubygems.GemfileLock{}).Parse(fstest.MapFS{}, "Gemfile.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestGemfileLock_Parse_EmptyLockfile(t *testing.T) {
	pkgs, _ := parseGemfile(t, "")
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
}

func TestGemfileLock_Parse_MultipleGemRemotes(t *testing.T) {
	// Apps that draw from a custom mirror plus rubygems.org end up
	// with two GEM blocks in the lockfile. Both should be parsed.
	pkgs, _ := parseGemfile(t, `GEM
  remote: https://internal.example.com/
  specs:
    private-gem (1.0.0)

GEM
  remote: https://rubygems.org/
  specs:
    rails (7.0.0)
`)
	if len(pkgs) != 2 {
		t.Errorf("expected packages from both blocks, got: %+v", pkgs)
	}
}
