package pypi_test

import (
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/internal/testutil"
	"github.com/Alge/tillit/ecosystems/pypi"
)

func TestPdmLock_Identity(t *testing.T) {
	a := pypi.PdmLock{}
	if a.Ecosystem() != "pypi" {
		t.Errorf("Ecosystem() = %q, want pypi", a.Ecosystem())
	}
	if a.Name() != "pdm.lock" {
		t.Errorf("Name() = %q, want pdm.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"pdm.lock", true},
		{"./pdm.lock", true},
		{"poetry.lock", false},
		{"uv.lock", false},
		{"pdm.lock.bak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func parsePdm(t *testing.T, content string) (pkgs []pypiPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"pdm.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (pypi.PdmLock{}).Parse(fsys, "pdm.lock")
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

func TestPdmLock_Parse_BasicPackages(t *testing.T) {
	pkgs, warnings := parsePdm(t, `
[metadata]
groups = ["default"]
lock_version = "4.4"

[[package]]
name = "requests"
version = "2.31.0"
requires_python = ">=3.7"

[[package]]
name = "flask"
version = "3.0.0"
`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestPdmLock_Parse_NormalizesPackageName(t *testing.T) {
	pkgs, _ := parsePdm(t, `
[[package]]
name = "Django_Project"
version = "4.2"
`)
	if len(pkgs) != 1 || pkgs[0].ID != "django-project" {
		t.Errorf("expected normalized name, got: %+v", pkgs)
	}
}

func TestPdmLock_Parse_SkipsGitRevision(t *testing.T) {
	pkgs, warnings := parsePdm(t, `
[[package]]
name = "ok"
version = "1.0.0"

[[package]]
name = "from-git"
version = "0.1.0"
revision = "abc123"
`)
	if len(pkgs) != 1 || pkgs[0].ID != "ok" {
		t.Errorf("expected only registry pkg, got: %+v", pkgs)
	}
	if !testutil.WarningContains(warnings, "from-git") {
		t.Errorf("expected warning, got: %v", warnings)
	}
}

func TestPdmLock_Parse_SkipsPathAndURL(t *testing.T) {
	pkgs, warnings := parsePdm(t, `
[[package]]
name = "from-path"
version = "0.1.0"
path = "../local"

[[package]]
name = "from-url"
version = "0.1.0"
url = "https://example.com/foo.tar.gz"
`)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
	if len(warnings) < 2 {
		t.Errorf("expected two warnings, got: %v", warnings)
	}
}

func TestPdmLock_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parsePdm(t, `
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

func TestPdmLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (pypi.PdmLock{}).Parse(fstest.MapFS{}, "pdm.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestPdmLock_Parse_MalformedTOMLErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"pdm.lock": &fstest.MapFile{Data: []byte("not [[ valid toml")},
	}
	_, err := (pypi.PdmLock{}).Parse(fsys, "pdm.lock")
	if err == nil {
		t.Error("expected error on malformed TOML")
	}
}
