package nuget_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/nuget"
)

type nugetPackage struct {
	ID, Version string
}

func parseNuget(t *testing.T, content string) (pkgs []nugetPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"packages.lock.json": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (nuget.PackagesLock{}).Parse(fsys, "packages.lock.json")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "nuget" {
			t.Errorf("Ecosystem = %q, want nuget", p.Ecosystem)
		}
		pkgs = append(pkgs, nugetPackage{p.PackageID, p.Version})
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

func TestPackagesLock_Identity(t *testing.T) {
	a := nuget.PackagesLock{}
	if a.Ecosystem() != "nuget" {
		t.Errorf("Ecosystem() = %q, want nuget", a.Ecosystem())
	}
	if a.Name() != "packages.lock.json" {
		t.Errorf("Name() = %q, want packages.lock.json", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"packages.lock.json", true},
		{"./packages.lock.json", true},
		{"/some/path/packages.lock.json", true},
		{"package-lock.json", false}, // npm
		{"packages.json", false},
		{"packages.config", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestPackagesLock_Parse_Basic(t *testing.T) {
	pkgs, warnings := parseNuget(t, `{
  "version": 1,
  "dependencies": {
    "net6.0": {
      "Newtonsoft.Json": {
        "type": "Direct",
        "requested": "[13.0.1, )",
        "resolved": "13.0.1",
        "contentHash": "ppPFpBcvxdsfUonNcvITKqLl3bqxWbDCZIzDWHzjpdAHRFfZe0Dw9HmA0+za13IdyrgJwpkDTDA9fHaxOrt20A=="
      },
      "Microsoft.Extensions.DependencyInjection": {
        "type": "Transitive",
        "resolved": "6.0.0",
        "contentHash": "abc"
      }
    }
  }
}`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	got := map[string]string{}
	for _, p := range pkgs {
		got[p.ID] = p.Version
	}
	if got["newtonsoft.json"] != "13.0.1" || got["microsoft.extensions.dependencyinjection"] != "6.0.0" {
		t.Errorf("unexpected: %+v", got)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestPackagesLock_Parse_LowercasesPackageName(t *testing.T) {
	// NuGet IDs are case-insensitive; the canonical form for the
	// registration API is lowercased. Normalising here keeps the
	// trust store from fragmenting on case-only differences.
	pkgs, _ := parseNuget(t, `{
  "version": 1,
  "dependencies": {
    "net6.0": {
      "MyOrg.MyPackage": {"type": "Direct", "resolved": "1.0.0", "contentHash": "a"}
    }
  }
}`)
	if len(pkgs) != 1 || pkgs[0].ID != "myorg.mypackage" {
		t.Errorf("expected lowercase id, got: %+v", pkgs)
	}
}

func TestPackagesLock_Parse_SkipsProjectType(t *testing.T) {
	// Project entries are workspace references to other csproj files
	// in the same solution — they live on disk, not on a registry.
	pkgs, _ := parseNuget(t, `{
  "version": 1,
  "dependencies": {
    "net6.0": {
      "MyApp.Core": {"type": "Project"},
      "Newtonsoft.Json": {"type": "Direct", "resolved": "13.0.1", "contentHash": "a"}
    }
  }
}`)
	if len(pkgs) != 1 || pkgs[0].ID != "newtonsoft.json" {
		t.Errorf("expected only the registry pkg, got: %+v", pkgs)
	}
}

func TestPackagesLock_Parse_DedupsAcrossFrameworks(t *testing.T) {
	// Multi-target projects list the same packages under each
	// framework. Same (name, version) → one entry.
	pkgs, _ := parseNuget(t, `{
  "version": 1,
  "dependencies": {
    "net6.0": {
      "Newtonsoft.Json": {"type": "Direct", "resolved": "13.0.1", "contentHash": "a"}
    },
    "netstandard2.0": {
      "Newtonsoft.Json": {"type": "Direct", "resolved": "13.0.1", "contentHash": "a"}
    }
  }
}`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup across frameworks, got: %+v", pkgs)
	}
}

func TestPackagesLock_Parse_KeepsDifferentVersionsAcrossFrameworks(t *testing.T) {
	pkgs, _ := parseNuget(t, `{
  "version": 1,
  "dependencies": {
    "net6.0": {
      "Newtonsoft.Json": {"type": "Direct", "resolved": "13.0.1", "contentHash": "a"}
    },
    "netstandard2.0": {
      "Newtonsoft.Json": {"type": "Direct", "resolved": "12.0.3", "contentHash": "b"}
    }
  }
}`)
	if len(pkgs) != 2 {
		t.Errorf("expected both versions, got: %+v", pkgs)
	}
}

func TestPackagesLock_Parse_SkipsEntryWithoutResolved(t *testing.T) {
	_, warnings := parseNuget(t, `{
  "version": 1,
  "dependencies": {
    "net6.0": {
      "WithoutResolved": {"type": "Direct", "contentHash": "a"}
    }
  }
}`)
	if !anyContains(warnings, "WithoutResolved") {
		t.Errorf("expected warning about missing resolved version, got: %v", warnings)
	}
}

func TestPackagesLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (nuget.PackagesLock{}).Parse(fstest.MapFS{}, "packages.lock.json")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestPackagesLock_Parse_MalformedJSONErrors(t *testing.T) {
	fsys := fstest.MapFS{
		"packages.lock.json": &fstest.MapFile{Data: []byte("not valid json {")},
	}
	_, err := (nuget.PackagesLock{}).Parse(fsys, "packages.lock.json")
	if err == nil {
		t.Error("expected error on malformed JSON")
	}
}

func TestPackagesLock_Parse_EmptyLockfile(t *testing.T) {
	pkgs, _ := parseNuget(t, `{"version": 1, "dependencies": {}}`)
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
}
