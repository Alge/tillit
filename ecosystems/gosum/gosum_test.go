package gosum_test

import (
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/gosum"
)

func TestGoSum_Ecosystem(t *testing.T) {
	if got := (gosum.GoSum{}).Ecosystem(); got != "go" {
		t.Errorf("Ecosystem() = %q, want %q", got, "go")
	}
}

func TestGoSum_CanParse(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"go.sum", true},
		{"./go.sum", true},
		{"/some/path/go.sum", true},
		{"go.mod", false},
		{"requirements.txt", false},
		{"go.sum.bak", false},
		{"", false},
	}
	a := gosum.GoSum{}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := a.CanParse(tc.path); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func parse(t *testing.T, files map[string]string) (pkgs []gosumPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, content := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(content)}
	}
	res, err := (gosum.GoSum{}).Parse(fsys, "go.sum")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		pkgs = append(pkgs, gosumPackage{p.PackageID, p.Version, p.Hash, p.Direct})
	}
	return pkgs, res.Warnings
}

type gosumPackage struct {
	ID, Version, Hash string
	Direct            bool
}

func TestGoSum_Parse_BasicEntry(t *testing.T) {
	pkgs, _ := parse(t, map[string]string{
		"go.sum": `github.com/foo/bar v1.2.3 h1:abc=
github.com/foo/bar v1.2.3/go.mod h1:def=
`})
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].ID != "github.com/foo/bar" || pkgs[0].Version != "v1.2.3" {
		t.Errorf("unexpected package: %+v", pkgs[0])
	}
}

func TestGoSum_Parse_DedupsGoModLines(t *testing.T) {
	pkgs, _ := parse(t, map[string]string{
		"go.sum": `github.com/a/b v1.0.0 h1:zip=
github.com/a/b v1.0.0/go.mod h1:mod=
`})
	if len(pkgs) != 1 {
		t.Errorf("expected 1 package after dedup, got %d", len(pkgs))
	}
}

func TestGoSum_Parse_MultipleVersionsOfSameModule(t *testing.T) {
	pkgs, _ := parse(t, map[string]string{
		"go.sum": `github.com/a/b v1.0.0 h1:x=
github.com/a/b v1.0.0/go.mod h1:y=
github.com/a/b v1.1.0 h1:z=
github.com/a/b v1.1.0/go.mod h1:w=
`})
	if len(pkgs) != 2 {
		t.Errorf("expected 2 packages, got %d", len(pkgs))
	}
}

func TestGoSum_Parse_PreservesHash(t *testing.T) {
	pkgs, _ := parse(t, map[string]string{
		"go.sum": `github.com/a/b v1.0.0 h1:abc123=
github.com/a/b v1.0.0/go.mod h1:def456=
`})
	if pkgs[0].Hash != "h1:abc123=" {
		t.Errorf("Hash = %q, want %q", pkgs[0].Hash, "h1:abc123=")
	}
}

func TestGoSum_Parse_SkipsBlankLines(t *testing.T) {
	pkgs, _ := parse(t, map[string]string{
		"go.sum": `
github.com/a/b v1.0.0 h1:x=

github.com/a/b v1.0.0/go.mod h1:y=
`})
	if len(pkgs) != 1 {
		t.Errorf("expected 1 package, got %d", len(pkgs))
	}
}

func TestGoSum_Parse_MalformedLineWarns(t *testing.T) {
	pkgs, warnings := parse(t, map[string]string{
		"go.sum": `github.com/good/one v1.0.0 h1:ok=
github.com/good/one v1.0.0/go.mod h1:ok=
this is not a go.sum line
`})
	if len(pkgs) != 1 {
		t.Errorf("expected 1 valid package, got %d", len(pkgs))
	}
	foundMalformed := false
	for _, w := range warnings {
		if w != "" && containsAll(w, "line", "expected 3 fields") {
			foundMalformed = true
		}
	}
	if !foundMalformed {
		t.Errorf("expected a warning for malformed line, got: %v", warnings)
	}
}

func TestGoSum_Parse_PseudoVersion(t *testing.T) {
	pkgs, _ := parse(t, map[string]string{
		"go.sum": `github.com/a/b v0.0.0-20231201120000-abc123def456 h1:x=
github.com/a/b v0.0.0-20231201120000-abc123def456/go.mod h1:y=
`})
	if pkgs[0].Version != "v0.0.0-20231201120000-abc123def456" {
		t.Errorf("Version = %q, want pseudo-version", pkgs[0].Version)
	}
}

func TestGoSum_Parse_OnlyGoModLine(t *testing.T) {
	pkgs, _ := parse(t, map[string]string{
		"go.sum": `github.com/indirect v1.0.0/go.mod h1:abc=
`})
	if len(pkgs) != 1 || pkgs[0].Version != "v1.0.0" {
		t.Errorf("unexpected packages: %+v", pkgs)
	}
}

// --- Direct/indirect labelling via sibling go.mod ----------------------

func TestGoSum_Parse_LabelsDirectFromGoMod(t *testing.T) {
	pkgs, warnings := parse(t, map[string]string{
		"go.sum": `github.com/direct/dep v1.0.0 h1:x=
github.com/direct/dep v1.0.0/go.mod h1:y=
github.com/indirect/dep v1.0.0 h1:a=
github.com/indirect/dep v1.0.0/go.mod h1:b=
`,
		"go.mod": `module example.com/me

go 1.21

require (
	github.com/direct/dep v1.0.0
	github.com/indirect/dep v1.0.0 // indirect
)
`,
	})
	if len(warnings) != 0 {
		t.Errorf("expected no warnings when go.mod is present, got: %v", warnings)
	}
	byID := map[string]gosumPackage{}
	for _, p := range pkgs {
		byID[p.ID] = p
	}
	if !byID["github.com/direct/dep"].Direct {
		t.Errorf("direct/dep should be Direct=true, got %+v", byID["github.com/direct/dep"])
	}
	if byID["github.com/indirect/dep"].Direct {
		t.Errorf("indirect/dep should be Direct=false, got %+v", byID["github.com/indirect/dep"])
	}
}

func TestGoSum_Parse_GoModMissing_AllIndirect(t *testing.T) {
	pkgs, warnings := parse(t, map[string]string{
		"go.sum": `github.com/foo/bar v1.0.0 h1:x=
github.com/foo/bar v1.0.0/go.mod h1:y=
`,
	})
	if len(pkgs) != 1 || pkgs[0].Direct {
		t.Errorf("expected Direct=false when go.mod is missing, got %+v", pkgs)
	}
	if len(warnings) == 0 {
		t.Error("expected a warning when go.mod is unavailable")
	}
}

func TestGoSum_Parse_SingleLineRequire(t *testing.T) {
	pkgs, _ := parse(t, map[string]string{
		"go.sum": `github.com/foo/bar v1.0.0 h1:x=
github.com/foo/bar v1.0.0/go.mod h1:y=
`,
		"go.mod": `module example.com/me
go 1.21
require github.com/foo/bar v1.0.0
`,
	})
	if !pkgs[0].Direct {
		t.Errorf("expected Direct=true for single-line require, got %+v", pkgs[0])
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, ss := range substrs {
		if !contains(s, ss) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
