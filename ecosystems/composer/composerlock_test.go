package composer_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/composer"
)

type composerPackage struct {
	ID, Version string
}

func parseComposer(t *testing.T, content string) (pkgs []composerPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"composer.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (composer.ComposerLock{}).Parse(fsys, "composer.lock")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "composer" {
			t.Errorf("Ecosystem = %q, want composer", p.Ecosystem)
		}
		pkgs = append(pkgs, composerPackage{p.PackageID, p.Version})
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

func TestComposerLock_Identity(t *testing.T) {
	a := composer.ComposerLock{}
	if a.Ecosystem() != "composer" {
		t.Errorf("Ecosystem() = %q, want composer", a.Ecosystem())
	}
	if a.Name() != "composer.lock" {
		t.Errorf("Name() = %q, want composer.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"composer.lock", true},
		{"./composer.lock", true},
		{"/some/path/composer.lock", true},
		{"composer.json", false},
		{"go.sum", false},
		{"composer.lock.bak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestComposerLock_Parse_Basic(t *testing.T) {
	pkgs, warnings := parseComposer(t, `{
  "_readme": ["..."],
  "content-hash": "abc",
  "packages": [
    {
      "name": "guzzlehttp/guzzle",
      "version": "7.8.0",
      "source": {"type": "git", "url": "https://github.com/guzzle/guzzle.git", "reference": "abc"},
      "dist": {"type": "zip", "url": "...", "shasum": "deadbeef", "reference": "abc"},
      "type": "library"
    },
    {
      "name": "monolog/monolog",
      "version": "3.5.0",
      "dist": {"type": "zip", "url": "...", "shasum": "cafef00d"},
      "type": "library"
    }
  ],
  "packages-dev": []
}`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	got := map[string]string{}
	for _, p := range pkgs {
		got[p.ID] = p.Version
	}
	if got["guzzlehttp/guzzle"] != "7.8.0" || got["monolog/monolog"] != "3.5.0" {
		t.Errorf("unexpected: %+v", got)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestComposerLock_Parse_IncludesPackagesDev(t *testing.T) {
	pkgs, _ := parseComposer(t, `{
  "packages": [
    {"name": "vendor/runtime", "version": "1.0.0", "dist": {"shasum": "a"}}
  ],
  "packages-dev": [
    {"name": "vendor/dev-tool", "version": "2.0.0", "dist": {"shasum": "b"}}
  ]
}`)
	if len(pkgs) != 2 {
		t.Errorf("expected 2 packages from prod + dev, got %d: %+v", len(pkgs), pkgs)
	}
}

func TestComposerLock_Parse_StripsLeadingV(t *testing.T) {
	// Some composer.lock files store the tag literally as "v1.0.0".
	// We normalise to plain semver so the trust store doesn't end up
	// keyed on both "1.0.0" and "v1.0.0" for the same release.
	pkgs, _ := parseComposer(t, `{
  "packages": [
    {"name": "vendor/pkg", "version": "v1.2.3", "dist": {"shasum": "a"}}
  ]
}`)
	if len(pkgs) != 1 || pkgs[0].Version != "1.2.3" {
		t.Errorf("expected version 1.2.3 (stripped), got: %+v", pkgs)
	}
}

func TestComposerLock_Parse_SkipsDevBranchVersions(t *testing.T) {
	// dev-<branch> versions are git refs, not releases. They have no
	// stable hash on packagist and can't be vetted.
	_, warnings := parseComposer(t, `{
  "packages": [
    {"name": "vendor/branch", "version": "dev-master", "dist": {"shasum": "a"}}
  ]
}`)
	if !anyContains(warnings, "dev-master") {
		t.Errorf("expected warning about dev branch, got: %v", warnings)
	}
}

func TestComposerLock_Parse_SkipsEntriesWithoutDist(t *testing.T) {
	// A package with only a `source` block (no dist tarball) means
	// it's installed straight from VCS — no registry artifact to vet.
	_, warnings := parseComposer(t, `{
  "packages": [
    {
      "name": "vendor/source-only",
      "version": "1.0.0",
      "source": {"type": "git", "url": "https://example.com/repo.git", "reference": "abc"}
    }
  ]
}`)
	if !anyContains(warnings, "source-only") {
		t.Errorf("expected warning about missing dist, got: %v", warnings)
	}
}

func TestComposerLock_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parseComposer(t, `{
  "packages": [
    {"name": "vendor/pkg", "version": "1.0.0", "dist": {"shasum": "a"}},
    {"name": "vendor/pkg", "version": "1.0.0", "dist": {"shasum": "a"}}
  ]
}`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestComposerLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (composer.ComposerLock{}).Parse(fstest.MapFS{}, "composer.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestComposerLock_Parse_MalformedJSONErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"composer.lock": &fstest.MapFile{Data: []byte("not valid json {")},
	}
	_, err := (composer.ComposerLock{}).Parse(fsys, "composer.lock")
	if err == nil {
		t.Error("expected error on malformed JSON")
	}
}

func TestComposerLock_Parse_EmptyLockfile(t *testing.T) {
	pkgs, _ := parseComposer(t, `{"packages": [], "packages-dev": []}`)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
}
