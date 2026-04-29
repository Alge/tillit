package pypi_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/pypi"
)

type pypiPackage struct {
	ID, Version string
}

func parseReq(t *testing.T, files map[string]string) (pkgs []pypiPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, content := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	res, err := (pypi.Requirements{}).Parse(fsys, "requirements.txt")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "pypi" {
			t.Errorf("Ecosystem = %q, want %q", p.Ecosystem, "pypi")
		}
		pkgs = append(pkgs, pypiPackage{p.PackageID, p.Version})
	}
	return pkgs, res.Warnings
}

func TestRequirements_Parse_BasicPin(t *testing.T) {
	pkgs, warnings := parseReq(t, map[string]string{
		"requirements.txt": "requests==2.31.0\n",
	})
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].ID != "requests" || pkgs[0].Version != "2.31.0" {
		t.Errorf("unexpected package: %+v", pkgs[0])
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestRequirements_Parse_NormalizesPackageName(t *testing.T) {
	// PEP 503 says these all share the same project; we must
	// normalize to the canonical form so the trust store keys
	// don't fragment.
	cases := []string{"Django==4.2", "DJANGO==4.2", "django.project==4.2", "django_project==4.2", "django-project==4.2"}
	want := []string{"django", "django", "django-project", "django-project", "django-project"}
	for i, line := range cases {
		pkgs, _ := parseReq(t, map[string]string{
			"requirements.txt": line + "\n",
		})
		if len(pkgs) != 1 {
			t.Fatalf("%q: expected 1 package, got %d", line, len(pkgs))
		}
		if pkgs[0].ID != want[i] {
			t.Errorf("%q: ID = %q, want %q", line, pkgs[0].ID, want[i])
		}
	}
}

func TestRequirements_Parse_SkipsCommentsAndBlanks(t *testing.T) {
	pkgs, _ := parseReq(t, map[string]string{
		"requirements.txt": `# top comment
requests==2.31.0  # trailing comment

flask==3.0.0
`,
	})
	if len(pkgs) != 2 {
		t.Errorf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
}

func TestRequirements_Parse_StripsExtras(t *testing.T) {
	pkgs, _ := parseReq(t, map[string]string{
		"requirements.txt": "requests[security,socks]==2.31.0\n",
	})
	if len(pkgs) != 1 || pkgs[0].ID != "requests" || pkgs[0].Version != "2.31.0" {
		t.Errorf("unexpected: %+v", pkgs)
	}
}

func TestRequirements_Parse_StripsEnvironmentMarker(t *testing.T) {
	pkgs, _ := parseReq(t, map[string]string{
		"requirements.txt": `requests==2.31.0 ; python_version >= "3.8"`,
	})
	if len(pkgs) != 1 || pkgs[0].ID != "requests" || pkgs[0].Version != "2.31.0" {
		t.Errorf("unexpected: %+v", pkgs)
	}
}

func TestRequirements_Parse_StripsHashOption(t *testing.T) {
	pkgs, _ := parseReq(t, map[string]string{
		"requirements.txt": "requests==2.31.0 --hash=sha256:abcdef\n",
	})
	if len(pkgs) != 1 || pkgs[0].ID != "requests" {
		t.Errorf("unexpected: %+v", pkgs)
	}
}

func TestRequirements_Parse_HandlesContinuationLines(t *testing.T) {
	// pip-compile output uses trailing backslashes to spread the
	// requirement and its --hash pins across multiple lines.
	pkgs, _ := parseReq(t, map[string]string{
		"requirements.txt": `requests==2.31.0 \
    --hash=sha256:aaaa \
    --hash=sha256:bbbb
flask==3.0.0
`,
	})
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].ID != "requests" || pkgs[0].Version != "2.31.0" {
		t.Errorf("first pkg: %+v", pkgs[0])
	}
	if pkgs[1].ID != "flask" || pkgs[1].Version != "3.0.0" {
		t.Errorf("second pkg: %+v", pkgs[1])
	}
}

func TestRequirements_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parseReq(t, map[string]string{
		"requirements.txt": `requests==2.31.0
requests==2.31.0
`,
	})
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got %d: %+v", len(pkgs), pkgs)
	}
}

func TestRequirements_Parse_KeepsDifferentVersions(t *testing.T) {
	pkgs, _ := parseReq(t, map[string]string{
		"requirements.txt": `requests==2.30.0
requests==2.31.0
`,
	})
	if len(pkgs) != 2 {
		t.Errorf("expected 2 packages for two versions, got %d", len(pkgs))
	}
}

func TestRequirements_Parse_WarnsOnLooseSpec(t *testing.T) {
	_, warnings := parseReq(t, map[string]string{
		"requirements.txt": "requests>=2.0\n",
	})
	if !anyContains(warnings, "pinned") {
		t.Errorf("expected warning about non-pinned requirement, got: %v", warnings)
	}
}

func TestRequirements_Parse_WarnsOnURLInstall(t *testing.T) {
	_, warnings := parseReq(t, map[string]string{
		"requirements.txt": "git+https://github.com/foo/bar.git@v1.0.0#egg=bar\n",
	})
	if !anyContains(warnings, "URL") {
		t.Errorf("expected URL/VCS warning, got: %v", warnings)
	}
}

func TestRequirements_Parse_WarnsOnIncludeOption(t *testing.T) {
	_, warnings := parseReq(t, map[string]string{
		"requirements.txt": "-r other-requirements.txt\n",
	})
	if !anyContains(warnings, "option") && !anyContains(warnings, "-r") {
		t.Errorf("expected option warning, got: %v", warnings)
	}
}

func TestRequirements_Parse_WarnsOnEditable(t *testing.T) {
	_, warnings := parseReq(t, map[string]string{
		"requirements.txt": "-e .\n",
	})
	if !anyContains(warnings, "option") && !anyContains(warnings, "-e") {
		t.Errorf("expected editable warning, got: %v", warnings)
	}
}

func TestRequirements_Parse_MissingFileErrors(t *testing.T) {
	res, err := (pypi.Requirements{}).Parse(fstest.MapFS{}, "requirements.txt")
	if err == nil {
		t.Errorf("expected error when file missing, got: %+v", res)
	}
}

func anyContains(warns []string, sub string) bool {
	for _, w := range warns {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}
