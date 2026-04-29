package npm

import (
	"sort"
	"testing"
	"testing/fstest"
)

const samplePackageLockV3 = `{
  "name": "myproj",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": {
      "name": "myproj",
      "version": "1.0.0",
      "dependencies": {
        "lodash": "^4.17.21",
        "@scope/widget": "1.0.0"
      },
      "devDependencies": {
        "jest": "^29.0.0"
      }
    },
    "node_modules/lodash": {
      "version": "4.17.21",
      "resolved": "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz",
      "integrity": "sha512-fake-lodash-integrity"
    },
    "node_modules/@scope/widget": {
      "version": "1.0.0",
      "resolved": "https://registry.npmjs.org/@scope/widget/-/widget-1.0.0.tgz",
      "integrity": "sha512-fake-widget-integrity",
      "dependencies": {
        "lodash": "^4.0.0"
      }
    },
    "node_modules/jest": {
      "version": "29.5.0",
      "resolved": "https://registry.npmjs.org/jest/-/jest-29.5.0.tgz",
      "integrity": "sha512-fake-jest-integrity",
      "dev": true
    }
  }
}`

func TestParse_LockfileVersion3(t *testing.T) {
	fsys := fstest.MapFS{
		"package-lock.json": &fstest.MapFile{Data: []byte(samplePackageLockV3)},
	}
	res, err := (PackageLock{}).Parse(fsys, "package-lock.json")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Packages) != 3 {
		t.Errorf("expected 3 packages, got %d", len(res.Packages))
	}

	byID := map[string]int{}
	for i, p := range res.Packages {
		byID[p.PackageID] = i
	}
	for _, want := range []string{"lodash", "@scope/widget", "jest"} {
		if _, ok := byID[want]; !ok {
			t.Errorf("expected package %q in result, got: %+v", want, res.Packages)
		}
	}

	// Direct vs indirect: lodash and @scope/widget are listed in
	// the root's dependencies; jest is in devDependencies; all
	// three are direct as far as tillit cares.
	for _, p := range res.Packages {
		if !p.Direct {
			t.Errorf("expected %s to be Direct, got %+v", p.PackageID, p)
		}
	}

	// Hash threading.
	for _, p := range res.Packages {
		if p.Hash == "" || p.Hash[:7] != "sha512-" {
			t.Errorf("expected sha512 integrity for %s, got %q", p.PackageID, p.Hash)
		}
	}

	// Edges: @scope/widget depends on lodash.
	wantKey := "@scope/widget@1.0.0"
	deps := res.Edges[wantKey]
	if len(deps) != 1 || deps[0] != "lodash@4.17.21" {
		t.Errorf("expected widget→lodash edge, got %v", deps)
	}
}

func TestParse_LockfileVersion1IsRejectedNonFatally(t *testing.T) {
	fsys := fstest.MapFS{
		"package-lock.json": &fstest.MapFile{Data: []byte(`{"lockfileVersion":1}`)},
	}
	res, err := (PackageLock{}).Parse(fsys, "package-lock.json")
	if err != nil {
		t.Fatalf("expected non-fatal warning, got error: %v", err)
	}
	if len(res.Packages) != 0 {
		t.Errorf("expected no packages from v1 lockfile, got %+v", res.Packages)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected a warning about lockfile version")
	}
}

func TestNpmLock_CanParse(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"package-lock.json", true},
		{"./package-lock.json", true},
		{"some/dir/package-lock.json", true},
		{"yarn.lock", false},
		{"package.json", false},
		{"", false},
	}
	for _, tc := range tests {
		got := (PackageLock{}).CanParse(tc.path)
		if got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestParse_StablePackageOrder(t *testing.T) {
	fsys := fstest.MapFS{
		"package-lock.json": &fstest.MapFile{Data: []byte(samplePackageLockV3)},
	}
	res, _ := (PackageLock{}).Parse(fsys, "package-lock.json")
	got := make([]string, 0, len(res.Packages))
	for _, p := range res.Packages {
		got = append(got, p.PackageID)
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("package list should be sorted by PackageID, got %v", got)
	}
}
