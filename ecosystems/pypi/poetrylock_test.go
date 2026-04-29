package pypi_test

import (
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/pypi"
)

func TestPoetryLock_Identity(t *testing.T) {
	a := pypi.PoetryLock{}
	if a.Ecosystem() != "pypi" {
		t.Errorf("Ecosystem() = %q, want pypi", a.Ecosystem())
	}
	if a.Name() != "poetry.lock" {
		t.Errorf("Name() = %q, want poetry.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"poetry.lock", true},
		{"./poetry.lock", true},
		{"/some/path/poetry.lock", true},
		{"uv.lock", false},
		{"requirements.txt", false},
		{"poetry.lock.bak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func parsePoetry(t *testing.T, content string) (pkgs []pypiPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"poetry.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (pypi.PoetryLock{}).Parse(fsys, "poetry.lock")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "pypi" {
			t.Errorf("Ecosystem = %q, want pypi", p.Ecosystem)
		}
		pkgs = append(pkgs, pypiPackage{p.PackageID, p.Version})
	}
	return pkgs, res.Warnings
}

func TestPoetryLock_Parse_DefaultPyPISource(t *testing.T) {
	pkgs, warnings := parsePoetry(t, `
[[package]]
name = "requests"
version = "2.31.0"
description = "Python HTTP for Humans."
optional = false
python-versions = ">=3.7"

[[package]]
name = "flask"
version = "3.0.0"
description = "A simple framework for building complex web applications."
optional = false
python-versions = ">=3.8"
`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].ID != "requests" || pkgs[0].Version != "2.31.0" {
		t.Errorf("first pkg: %+v", pkgs[0])
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestPoetryLock_Parse_NormalizesPackageName(t *testing.T) {
	pkgs, _ := parsePoetry(t, `
[[package]]
name = "Django_Project"
version = "4.2"
`)
	if len(pkgs) != 1 || pkgs[0].ID != "django-project" {
		t.Errorf("expected normalized name, got: %+v", pkgs)
	}
}

func TestPoetryLock_Parse_AcceptsLegacyPrivateIndex(t *testing.T) {
	// Private indexes that use the simple HTML protocol are still
	// PyPI-shaped — the JSON API will work as long as the user
	// points TILLIT_PYPI_URL at one that exposes /pypi/...
	pkgs, warnings := parsePoetry(t, `
[[package]]
name = "internal-package"
version = "1.0.0"
description = "..."
optional = false
python-versions = ">=3.8"

[package.source]
type = "legacy"
url = "https://internal.example.com/simple"
reference = "internal"
`)
	if len(pkgs) != 1 || pkgs[0].ID != "internal-package" {
		t.Errorf("expected legacy/simple to be vetable, got: %+v pkgs warnings: %v", pkgs, warnings)
	}
}

func TestPoetryLock_Parse_SkipsGitSource(t *testing.T) {
	pkgs, warnings := parsePoetry(t, `
[[package]]
name = "ok"
version = "1.0.0"

[[package]]
name = "from-git"
version = "0.1.0"

[package.source]
type = "git"
url = "https://github.com/foo/bar.git"
reference = "main"
resolved_reference = "abc123"
`)
	if len(pkgs) != 1 || pkgs[0].ID != "ok" {
		t.Errorf("expected only registry pkg, got: %+v", pkgs)
	}
	if !anyContains(warnings, "from-git") {
		t.Errorf("expected warning mentioning skipped git pkg, got: %v", warnings)
	}
}

func TestPoetryLock_Parse_SkipsFileAndDirectorySources(t *testing.T) {
	pkgs, warnings := parsePoetry(t, `
[[package]]
name = "from-file"
version = "0.1.0"

[package.source]
type = "file"
url = "../local/foo.tar.gz"

[[package]]
name = "from-directory"
version = "0.1.0"

[package.source]
type = "directory"
url = "../local-pkg"

[[package]]
name = "from-url"
version = "0.1.0"

[package.source]
type = "url"
url = "https://example.com/foo.tar.gz"
`)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
	if len(warnings) < 3 {
		t.Errorf("expected three warnings, got: %v", warnings)
	}
}

func TestPoetryLock_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parsePoetry(t, `
[[package]]
name = "requests"
version = "2.31.0"

[[package]]
name = "requests"
version = "2.31.0"
`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestPoetryLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (pypi.PoetryLock{}).Parse(fstest.MapFS{}, "poetry.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestPoetryLock_Parse_MalformedTOMLErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"poetry.lock": &fstest.MapFile{Data: []byte("not [[ valid toml")},
	}
	_, err := (pypi.PoetryLock{}).Parse(fsys, "poetry.lock")
	if err == nil {
		t.Error("expected error on malformed TOML")
	}
}
