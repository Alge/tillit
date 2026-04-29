package pypi_test

import (
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/pypi"
)

func TestPipfileLock_Identity(t *testing.T) {
	a := pypi.PipfileLock{}
	if a.Ecosystem() != "pypi" {
		t.Errorf("Ecosystem() = %q, want pypi", a.Ecosystem())
	}
	if a.Name() != "Pipfile.lock" {
		t.Errorf("Name() = %q, want Pipfile.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"Pipfile.lock", true},
		{"./Pipfile.lock", true},
		{"/some/path/Pipfile.lock", true},
		{"poetry.lock", false},
		{"requirements.txt", false},
		{"Pipfile", false},
		{"Pipfile.lock.bak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func parsePipfile(t *testing.T, content string) (pkgs []pypiPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"Pipfile.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (pypi.PipfileLock{}).Parse(fsys, "Pipfile.lock")
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

func TestPipfileLock_Parse_DefaultSection(t *testing.T) {
	pkgs, warnings := parsePipfile(t, `{
  "_meta": {"sources": [{"name": "pypi", "url": "https://pypi.org/simple"}]},
  "default": {
    "requests": {"index": "pypi", "version": "==2.31.0", "hashes": ["sha256:aaa"]},
    "flask": {"index": "pypi", "version": "==3.0.0", "hashes": ["sha256:bbb"]}
  },
  "develop": {}
}`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	got := map[string]string{}
	for _, p := range pkgs {
		got[p.ID] = p.Version
	}
	if got["requests"] != "2.31.0" || got["flask"] != "3.0.0" {
		t.Errorf("unexpected: %+v", got)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestPipfileLock_Parse_IncludesDevelopSection(t *testing.T) {
	pkgs, _ := parsePipfile(t, `{
  "default": {
    "requests": {"version": "==2.31.0"}
  },
  "develop": {
    "pytest": {"version": "==7.4.0"}
  }
}`)
	if len(pkgs) != 2 {
		t.Errorf("expected 2 packages from default + develop, got %d: %+v", len(pkgs), pkgs)
	}
}

func TestPipfileLock_Parse_NormalizesPackageName(t *testing.T) {
	pkgs, _ := parsePipfile(t, `{
  "default": {
    "Django_Project": {"version": "==4.2"}
  }
}`)
	if len(pkgs) != 1 || pkgs[0].ID != "django-project" {
		t.Errorf("expected normalized name, got: %+v", pkgs)
	}
}

func TestPipfileLock_Parse_StripsEqualEqualPrefix(t *testing.T) {
	pkgs, _ := parsePipfile(t, `{
  "default": {
    "requests": {"version": "==2.31.0"}
  }
}`)
	if len(pkgs) != 1 || pkgs[0].Version != "2.31.0" {
		t.Errorf("expected version with == stripped, got: %+v", pkgs)
	}
}

func TestPipfileLock_Parse_SkipsGitEntry(t *testing.T) {
	pkgs, warnings := parsePipfile(t, `{
  "default": {
    "ok": {"version": "==1.0"},
    "from-git": {"git": "https://github.com/foo/bar.git", "ref": "abc"}
  }
}`)
	if len(pkgs) != 1 || pkgs[0].ID != "ok" {
		t.Errorf("expected only registry pkg, got: %+v", pkgs)
	}
	if !anyContains(warnings, "from-git") {
		t.Errorf("expected warning mentioning skipped git pkg, got: %v", warnings)
	}
}

func TestPipfileLock_Parse_SkipsPathAndFileEntries(t *testing.T) {
	pkgs, warnings := parsePipfile(t, `{
  "default": {
    "from-path": {"path": "../local/foo"},
    "from-file": {"file": "https://example.com/foo.tar.gz"}
  }
}`)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
	if len(warnings) < 2 {
		t.Errorf("expected two warnings, got: %v", warnings)
	}
}

func TestPipfileLock_Parse_SkipsEntryWithoutVersion(t *testing.T) {
	_, warnings := parsePipfile(t, `{
  "default": {
    "weird": {"hashes": ["sha256:..."]}
  }
}`)
	if !anyContains(warnings, "weird") {
		t.Errorf("expected warning about missing version, got: %v", warnings)
	}
}

func TestPipfileLock_Parse_DedupsAcrossSections(t *testing.T) {
	pkgs, _ := parsePipfile(t, `{
  "default": {
    "requests": {"version": "==2.31.0"}
  },
  "develop": {
    "requests": {"version": "==2.31.0"}
  }
}`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestPipfileLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (pypi.PipfileLock{}).Parse(fstest.MapFS{}, "Pipfile.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestPipfileLock_Parse_MalformedJSONErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"Pipfile.lock": &fstest.MapFile{Data: []byte("not valid json {{{")},
	}
	_, err := (pypi.PipfileLock{}).Parse(fsys, "Pipfile.lock")
	if err == nil {
		t.Error("expected error on malformed JSON")
	}
}
