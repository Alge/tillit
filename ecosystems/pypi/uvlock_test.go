package pypi_test

import (
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/internal/testutil"
	"github.com/Alge/tillit/ecosystems/pypi"
)

func TestUvLock_Identity(t *testing.T) {
	a := pypi.UvLock{}
	if a.Ecosystem() != "pypi" {
		t.Errorf("Ecosystem() = %q, want pypi", a.Ecosystem())
	}
	if a.Name() != "uv.lock" {
		t.Errorf("Name() = %q, want uv.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"uv.lock", true},
		{"./uv.lock", true},
		{"/some/path/uv.lock", true},
		{"poetry.lock", false},
		{"requirements.txt", false},
		{"uv.lock.bak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func parseUv(t *testing.T, content string) (pkgs []pypiPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"uv.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (pypi.UvLock{}).Parse(fsys, "uv.lock")
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

func TestUvLock_Parse_BasicRegistryPackages(t *testing.T) {
	pkgs, warnings := parseUv(t, `
version = 1
requires-python = ">=3.8"

[[package]]
name = "requests"
version = "2.31.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "flask"
version = "3.0.0"
source = { registry = "https://pypi.org/simple" }
`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].ID != "requests" || pkgs[0].Version != "2.31.0" {
		t.Errorf("first pkg: %+v", pkgs[0])
	}
	if pkgs[1].ID != "flask" || pkgs[1].Version != "3.0.0" {
		t.Errorf("second pkg: %+v", pkgs[1])
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestUvLock_Parse_NormalizesPackageName(t *testing.T) {
	pkgs, _ := parseUv(t, `
[[package]]
name = "Django_Project"
version = "4.2"
source = { registry = "https://pypi.org/simple" }
`)
	if len(pkgs) != 1 || pkgs[0].ID != "django-project" {
		t.Errorf("expected normalized name, got: %+v", pkgs)
	}
}

func TestUvLock_Parse_SkipsGitSource(t *testing.T) {
	pkgs, warnings := parseUv(t, `
[[package]]
name = "ok"
version = "1.0.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "from-git"
version = "0.1.0"
source = { git = "https://github.com/foo/bar.git" }
`)
	if len(pkgs) != 1 || pkgs[0].ID != "ok" {
		t.Errorf("expected only registry pkg, got: %+v", pkgs)
	}
	if !testutil.WarningContains(warnings, "from-git") {
		t.Errorf("expected warning mentioning the skipped git pkg, got: %v", warnings)
	}
}

func TestUvLock_Parse_SkipsEditableAndVirtual(t *testing.T) {
	pkgs, warnings := parseUv(t, `
[[package]]
name = "myapp"
version = "0.1.0"
source = { editable = "." }

[[package]]
name = "workspace-member"
version = "0.1.0"
source = { virtual = "." }
`)
	if len(pkgs) != 0 {
		t.Errorf("expected no registry pkgs, got: %+v", pkgs)
	}
	if len(warnings) < 2 {
		t.Errorf("expected warnings for both skipped packages, got: %v", warnings)
	}
}

func TestUvLock_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parseUv(t, `
[[package]]
name = "requests"
version = "2.31.0"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "requests"
version = "2.31.0"
source = { registry = "https://pypi.org/simple" }
`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestUvLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (pypi.UvLock{}).Parse(fstest.MapFS{}, "uv.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestUvLock_Parse_MalformedTOMLErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"uv.lock": &fstest.MapFile{Data: []byte("this is not [[ valid toml")},
	}
	_, err := (pypi.UvLock{}).Parse(fsys, "uv.lock")
	if err == nil {
		t.Error("expected error on malformed TOML")
	}
}
