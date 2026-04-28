package gosum_test

import (
	"strings"
	"testing"

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

func TestGoSum_Parse_BasicEntry(t *testing.T) {
	input := `github.com/foo/bar v1.2.3 h1:abc=
github.com/foo/bar v1.2.3/go.mod h1:def=
`
	res, err := (gosum.GoSum{}).Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(res.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d: %+v", len(res.Packages), res.Packages)
	}
	p := res.Packages[0]
	if p.Ecosystem != "go" || p.PackageID != "github.com/foo/bar" || p.Version != "v1.2.3" {
		t.Errorf("unexpected PackageRef: %+v", p)
	}
}

func TestGoSum_Parse_DedupsGoModLines(t *testing.T) {
	// Same (module, version) in module-zip line and go.mod line — should
	// produce one entry, not two.
	input := `github.com/a/b v1.0.0 h1:zip=
github.com/a/b v1.0.0/go.mod h1:mod=
`
	res, _ := (gosum.GoSum{}).Parse(strings.NewReader(input))
	if len(res.Packages) != 1 {
		t.Errorf("expected 1 package after dedup, got %d", len(res.Packages))
	}
}

func TestGoSum_Parse_MultipleVersionsOfSameModule(t *testing.T) {
	// Different versions of the same module are separate entries.
	input := `github.com/a/b v1.0.0 h1:x=
github.com/a/b v1.0.0/go.mod h1:y=
github.com/a/b v1.1.0 h1:z=
github.com/a/b v1.1.0/go.mod h1:w=
`
	res, _ := (gosum.GoSum{}).Parse(strings.NewReader(input))
	if len(res.Packages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(res.Packages))
	}
}

func TestGoSum_Parse_PreservesHash(t *testing.T) {
	input := `github.com/a/b v1.0.0 h1:abc123=
github.com/a/b v1.0.0/go.mod h1:def456=
`
	res, _ := (gosum.GoSum{}).Parse(strings.NewReader(input))
	if len(res.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(res.Packages))
	}
	// The module-zip hash (not the go.mod hash) is the artifact hash.
	if res.Packages[0].Hash != "h1:abc123=" {
		t.Errorf("Hash = %q, want %q", res.Packages[0].Hash, "h1:abc123=")
	}
}

func TestGoSum_Parse_SkipsBlankLines(t *testing.T) {
	input := `
github.com/a/b v1.0.0 h1:x=

github.com/a/b v1.0.0/go.mod h1:y=
`
	res, err := (gosum.GoSum{}).Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(res.Packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(res.Packages))
	}
}

func TestGoSum_Parse_MalformedLineWarns(t *testing.T) {
	input := `github.com/good/one v1.0.0 h1:ok=
github.com/good/one v1.0.0/go.mod h1:ok=
this is not a go.sum line
`
	res, err := (gosum.GoSum{}).Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse should not return fatal error for malformed line: %v", err)
	}
	if len(res.Packages) != 1 {
		t.Errorf("expected 1 valid package, got %d", len(res.Packages))
	}
	if len(res.Warnings) == 0 {
		t.Error("expected a warning for malformed line")
	}
}

func TestGoSum_Parse_PseudoVersion(t *testing.T) {
	input := `github.com/a/b v0.0.0-20231201120000-abc123def456 h1:x=
github.com/a/b v0.0.0-20231201120000-abc123def456/go.mod h1:y=
`
	res, _ := (gosum.GoSum{}).Parse(strings.NewReader(input))
	if len(res.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(res.Packages))
	}
	if res.Packages[0].Version != "v0.0.0-20231201120000-abc123def456" {
		t.Errorf("Version = %q, want pseudo-version", res.Packages[0].Version)
	}
}

func TestGoSum_Parse_OnlyGoModLine(t *testing.T) {
	// Sometimes only the /go.mod line is present (e.g. for indirect modules
	// where Go didn't need to download the zip). Still produces an entry.
	input := `github.com/indirect v1.0.0/go.mod h1:abc=
`
	res, _ := (gosum.GoSum{}).Parse(strings.NewReader(input))
	if len(res.Packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(res.Packages))
	}
	if res.Packages[0].Version != "v1.0.0" {
		t.Errorf("Version = %q, want v1.0.0", res.Packages[0].Version)
	}
}
