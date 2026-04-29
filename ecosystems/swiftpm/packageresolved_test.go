package swiftpm_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/swiftpm"
)

type swiftPackage struct {
	ID, Version string
}

func parseSwift(t *testing.T, content string) (pkgs []swiftPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"Package.resolved": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (swiftpm.PackageResolved{}).Parse(fsys, "Package.resolved")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "swiftpm" {
			t.Errorf("Ecosystem = %q, want swiftpm", p.Ecosystem)
		}
		pkgs = append(pkgs, swiftPackage{p.PackageID, p.Version})
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

func TestPackageResolved_Identity(t *testing.T) {
	a := swiftpm.PackageResolved{}
	if a.Ecosystem() != "swiftpm" {
		t.Errorf("Ecosystem() = %q, want swiftpm", a.Ecosystem())
	}
	if a.Name() != "Package.resolved" {
		t.Errorf("Name() = %q, want Package.resolved", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"Package.resolved", true},
		{"./Package.resolved", true},
		{"/some/path/Package.resolved", true},
		{"Package.swift", false},
		{"package.resolved", false}, // case-sensitive
		{"go.sum", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestPackageResolved_Parse_V2_TaggedReleases(t *testing.T) {
	pkgs, warnings := parseSwift(t, `{
  "originHash": "...",
  "pins": [
    {
      "identity": "alamofire",
      "kind": "remoteSourceControl",
      "location": "https://github.com/Alamofire/Alamofire.git",
      "state": {"revision": "abc123", "version": "5.8.0"}
    },
    {
      "identity": "swift-log",
      "kind": "remoteSourceControl",
      "location": "https://github.com/apple/swift-log.git",
      "state": {"revision": "def456", "version": "1.5.0"}
    }
  ],
  "version": 2
}`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	got := map[string]string{}
	for _, p := range pkgs {
		got[p.ID] = p.Version
	}
	if got["alamofire"] != "5.8.0" || got["swift-log"] != "1.5.0" {
		t.Errorf("unexpected: %+v", got)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestPackageResolved_Parse_V1Format(t *testing.T) {
	// V1 nested pins under `object.pins` instead of top-level. Older
	// Xcode projects still emit it.
	pkgs, _ := parseSwift(t, `{
  "object": {
    "pins": [
      {
        "package": "Alamofire",
        "repositoryURL": "https://github.com/Alamofire/Alamofire.git",
        "state": {"branch": null, "revision": "abc", "version": "5.8.0"}
      }
    ]
  },
  "version": 1
}`)
	if len(pkgs) != 1 || pkgs[0].Version != "5.8.0" {
		t.Errorf("expected one v1 entry, got: %+v", pkgs)
	}
	// V1 uses `package` (display name) — we keep it as the identity
	// since v1 lockfiles predate the canonical-identity convention.
	if pkgs[0].ID != "Alamofire" {
		t.Errorf("expected v1 package field as id, got %q", pkgs[0].ID)
	}
}

func TestPackageResolved_Parse_SkipsBranchPin(t *testing.T) {
	_, warnings := parseSwift(t, `{
  "pins": [
    {
      "identity": "branch-pinned",
      "kind": "remoteSourceControl",
      "location": "https://github.com/foo/bar.git",
      "state": {"branch": "main", "revision": "abc"}
    }
  ],
  "version": 2
}`)
	if !anyContains(warnings, "branch-pinned") {
		t.Errorf("expected warning for branch pin, got: %v", warnings)
	}
}

func TestPackageResolved_Parse_SkipsRevisionPin(t *testing.T) {
	// A pin with only a revision (no version, no branch) is a raw
	// commit pin — not a tagged release, so not vetable through any
	// version-keyed registry.
	_, warnings := parseSwift(t, `{
  "pins": [
    {
      "identity": "commit-pinned",
      "kind": "remoteSourceControl",
      "location": "https://github.com/foo/bar.git",
      "state": {"revision": "abc123def"}
    }
  ],
  "version": 2
}`)
	if !anyContains(warnings, "commit-pinned") {
		t.Errorf("expected warning, got: %v", warnings)
	}
}

func TestPackageResolved_Parse_SkipsFileSystemKind(t *testing.T) {
	_, warnings := parseSwift(t, `{
  "pins": [
    {
      "identity": "local-package",
      "kind": "fileSystem",
      "location": "../local"
    }
  ],
  "version": 2
}`)
	if !anyContains(warnings, "local-package") {
		t.Errorf("expected warning, got: %v", warnings)
	}
}

func TestPackageResolved_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parseSwift(t, `{
  "pins": [
    {"identity": "alamofire", "kind": "remoteSourceControl",
     "state": {"version": "5.8.0", "revision": "abc"}},
    {"identity": "alamofire", "kind": "remoteSourceControl",
     "state": {"version": "5.8.0", "revision": "abc"}}
  ],
  "version": 2
}`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestPackageResolved_Parse_MissingFileErrors(t *testing.T) {
	_, err := (swiftpm.PackageResolved{}).Parse(fstest.MapFS{}, "Package.resolved")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestPackageResolved_Parse_MalformedJSONErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"Package.resolved": &fstest.MapFile{Data: []byte("not valid json {")},
	}
	_, err := (swiftpm.PackageResolved{}).Parse(fsys, "Package.resolved")
	if err == nil {
		t.Error("expected error on malformed JSON")
	}
}

func TestPackageResolved_Parse_EmptyPinsList(t *testing.T) {
	pkgs, _ := parseSwift(t, `{"pins": [], "version": 2}`)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
}
