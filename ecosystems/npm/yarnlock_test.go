package npm_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/internal/testutil"
	"github.com/Alge/tillit/ecosystems/npm"
)

type yarnPackage struct {
	ID, Version, Hash string
}

func parseYarn(t *testing.T, content string) (pkgs []yarnPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"yarn.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (npm.YarnLock{}).Parse(fsys, "yarn.lock")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "npm" {
			t.Errorf("Ecosystem = %q, want npm", p.Ecosystem)
		}
		pkgs = append(pkgs, yarnPackage{p.PackageID, p.Version, p.Hash})
	}
	return pkgs, res.Warnings
}

func TestYarnLock_Identity(t *testing.T) {
	a := npm.YarnLock{}
	if a.Ecosystem() != "npm" {
		t.Errorf("Ecosystem() = %q, want npm", a.Ecosystem())
	}
	if a.Name() != "yarn.lock" {
		t.Errorf("Name() = %q, want yarn.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"yarn.lock", true},
		{"./yarn.lock", true},
		{"/some/path/yarn.lock", true},
		{"package-lock.json", false},
		{"go.sum", false},
		{"yarn.lock.bak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestYarnLock_Parse_BasicV1(t *testing.T) {
	pkgs, warnings := parseYarn(t, `# yarn lockfile v1


"left-pad@^1.3.0":
  version "1.3.0"
  resolved "https://registry.yarnpkg.com/left-pad/-/left-pad-1.3.0.tgz#5b8a3a7765dfe001261dde915589e782f8c94d1e"
  integrity sha512-XI5MPzVNApjAyhQzphX8BkmKsKUxD4LdyK24iZeQGinBN9yTQT3bFlCBy/aVx2HrNcqQGsdot8ghrjyrvMCoEA==

"is-array@^1.0.1":
  version "1.0.1"
  resolved "..."
  integrity sha512-cafef00d
`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	got := map[string]yarnPackage{}
	for _, p := range pkgs {
		got[p.ID] = p
	}
	if got["left-pad"].Version != "1.3.0" || !strings.HasPrefix(got["left-pad"].Hash, "sha512-XI5MPzV") {
		t.Errorf("left-pad: %+v", got["left-pad"])
	}
	if got["is-array"].Version != "1.0.1" {
		t.Errorf("is-array: %+v", got["is-array"])
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestYarnLock_Parse_ScopedPackage(t *testing.T) {
	// Scoped packages (@scope/name) start with '@', so the package
	// name extends through the second '@' which separates the
	// constraint.
	pkgs, _ := parseYarn(t, `"@babel/code-frame@^7.10.4":
  version "7.10.4"
  resolved "..."
  integrity "sha512-foo"
`)
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].ID != "@babel/code-frame" || pkgs[0].Version != "7.10.4" {
		t.Errorf("unexpected: %+v", pkgs[0])
	}
}

func TestYarnLock_Parse_MultipleConstraintsSameBlock(t *testing.T) {
	// Yarn collapses identical resolved versions of a package into
	// one block with multiple constraint headers (comma-separated).
	// The package should appear only once in the output.
	pkgs, _ := parseYarn(t, `"left-pad@^1.0.0", "left-pad@^1.3.0":
  version "1.3.0"
  resolved "..."
  integrity "sha512-foo"
`)
	if len(pkgs) != 1 || pkgs[0].ID != "left-pad" {
		t.Errorf("expected one entry with merged headers, got: %+v", pkgs)
	}
}

func TestYarnLock_Parse_DedupsRepeatBlocks(t *testing.T) {
	pkgs, _ := parseYarn(t, `"left-pad@^1.3.0":
  version "1.3.0"
  resolved "..."
  integrity "sha512-a"

"left-pad@^1.3.0":
  version "1.3.0"
  resolved "..."
  integrity "sha512-a"
`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestYarnLock_Parse_KeepsDifferentVersions(t *testing.T) {
	pkgs, _ := parseYarn(t, `"left-pad@^1.0.0":
  version "1.0.2"
  resolved "..."
  integrity "sha512-old"

"left-pad@^1.3.0":
  version "1.3.0"
  resolved "..."
  integrity "sha512-new"
`)
	if len(pkgs) != 2 {
		t.Errorf("expected 2 versions, got: %+v", pkgs)
	}
}

func TestYarnLock_Parse_SkipsBlocksWithoutVersion(t *testing.T) {
	// Defensive — a malformed block without a `version` line should
	// be skipped rather than crashing.
	_, warnings := parseYarn(t, `"weird@^1.0.0":
  resolved "..."
  integrity "sha512-foo"
`)
	if !testutil.WarningContains(warnings, "weird") {
		t.Errorf("expected warning, got: %v", warnings)
	}
}

func TestYarnLock_Parse_WarnsOnGitProtocol(t *testing.T) {
	// Yarn supports git+ and file: protocols in the constraint
	// segment. Those installs aren't from the registry — warn.
	_, warnings := parseYarn(t, `"from-git@git+https://github.com/foo/bar.git":
  version "1.0.0"
  resolved "git+https://github.com/foo/bar.git#abc123"
`)
	if !testutil.WarningContains(warnings, "from-git") {
		t.Errorf("expected warning, got: %v", warnings)
	}
}

func TestYarnLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (npm.YarnLock{}).Parse(fstest.MapFS{}, "yarn.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestYarnLock_Parse_EmptyLockfile(t *testing.T) {
	pkgs, _ := parseYarn(t, "# yarn lockfile v1\n\n")
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
}
