package cocoapods_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Alge/tillit/ecosystems/cocoapods"
)

type podPackage struct {
	ID, Version, Hash string
}

func parsePodfile(t *testing.T, content string) (pkgs []podPackage, warnings []string) {
	t.Helper()
	fsys := fstest.MapFS{
		"Podfile.lock": &fstest.MapFile{Data: []byte(content)},
	}
	res, err := (cocoapods.PodfileLock{}).Parse(fsys, "Podfile.lock")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, p := range res.Packages {
		if p.Ecosystem != "cocoapods" {
			t.Errorf("Ecosystem = %q, want cocoapods", p.Ecosystem)
		}
		pkgs = append(pkgs, podPackage{p.PackageID, p.Version, p.Hash})
	}
	return pkgs, res.Warnings
}

func TestPodfileLock_Identity(t *testing.T) {
	a := cocoapods.PodfileLock{}
	if a.Ecosystem() != "cocoapods" {
		t.Errorf("Ecosystem() = %q, want cocoapods", a.Ecosystem())
	}
	if a.Name() != "Podfile.lock" {
		t.Errorf("Name() = %q, want Podfile.lock", a.Name())
	}
	cases := []struct {
		path string
		want bool
	}{
		{"Podfile.lock", true},
		{"./Podfile.lock", true},
		{"/some/path/Podfile.lock", true},
		{"Podfile", false},
		{"podfile.lock", false}, // case-sensitive
		{"go.sum", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := a.CanParse(tc.path); got != tc.want {
			t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestPodfileLock_Parse_Basic(t *testing.T) {
	pkgs, warnings := parsePodfile(t, `PODS:
  - Alamofire (5.8.0)
  - SwiftyJSON (5.0.1)

DEPENDENCIES:
  - Alamofire
  - SwiftyJSON

SPEC REPOS:
  trunk:
    - Alamofire
    - SwiftyJSON

SPEC CHECKSUMS:
  Alamofire: abc123def456
  SwiftyJSON: deadbeefcafef00d

PODFILE CHECKSUM: 0123456789

COCOAPODS: 1.13.0
`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %+v", len(pkgs), pkgs)
	}
	got := map[string]podPackage{}
	for _, p := range pkgs {
		got[p.ID] = p
	}
	if got["Alamofire"].Version != "5.8.0" || got["Alamofire"].Hash != "abc123def456" {
		t.Errorf("Alamofire: %+v", got["Alamofire"])
	}
	if got["SwiftyJSON"].Version != "5.0.1" || got["SwiftyJSON"].Hash != "deadbeefcafef00d" {
		t.Errorf("SwiftyJSON: %+v", got["SwiftyJSON"])
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestPodfileLock_Parse_PodsWithSubdeps(t *testing.T) {
	// Pods that have sub-dependencies use the map form: the pod
	// itself sits at the same indent, then its deps follow at +2.
	pkgs, _ := parsePodfile(t, `PODS:
  - Alamofire (5.8.0)
  - Networking (2.0.0):
    - Alamofire (~> 5.8)
    - SwiftyJSON

SPEC CHECKSUMS:
  Alamofire: aaa
  Networking: bbb
`)
	if len(pkgs) != 2 {
		t.Errorf("expected 2 top-level pods, got %d: %+v", len(pkgs), pkgs)
	}
	got := map[string]string{}
	for _, p := range pkgs {
		got[p.ID] = p.Version
	}
	if got["Alamofire"] != "5.8.0" || got["Networking"] != "2.0.0" {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestPodfileLock_Parse_PodsWithSubspec(t *testing.T) {
	// Subspecs use slash notation in pod names: "Foo/Bar".
	pkgs, _ := parsePodfile(t, `PODS:
  - Firebase/Core (10.0.0)
  - Firebase/Messaging (10.0.0)

SPEC CHECKSUMS:
  Firebase: abc
`)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got: %+v", pkgs)
	}
}

func TestPodfileLock_Parse_DedupsRepeatEntries(t *testing.T) {
	pkgs, _ := parsePodfile(t, `PODS:
  - Alamofire (5.8.0)
  - Alamofire (5.8.0)

SPEC CHECKSUMS:
  Alamofire: abc
`)
	if len(pkgs) != 1 {
		t.Errorf("expected dedup, got: %+v", pkgs)
	}
}

func TestPodfileLock_Parse_KeepsDifferentVersions(t *testing.T) {
	pkgs, _ := parsePodfile(t, `PODS:
  - Alamofire (5.8.0)
  - Alamofire (5.7.0)

SPEC CHECKSUMS:
  Alamofire: abc
`)
	if len(pkgs) != 2 {
		t.Errorf("expected both versions, got: %+v", pkgs)
	}
}

func TestPodfileLock_Parse_HashOptional(t *testing.T) {
	// SPEC CHECKSUMS is sometimes missing entries (very old
	// Podfile.lock files). The pod should still be emitted, just
	// without a hash.
	pkgs, _ := parsePodfile(t, `PODS:
  - HashlessPod (1.0.0)
`)
	if len(pkgs) != 1 || pkgs[0].Hash != "" {
		t.Errorf("expected pkg with empty hash, got: %+v", pkgs)
	}
}

func TestPodfileLock_Parse_WarnsOnExternalSource(t *testing.T) {
	// CHECKOUT OPTIONS lists pods installed straight from a git URL
	// or a local path — those aren't on trunk.
	_, warnings := parsePodfile(t, `PODS:
  - PrivatePod (1.0.0)

CHECKOUT OPTIONS:
  PrivatePod:
    :git: https://github.com/foo/private.git
    :commit: abc123

SPEC CHECKSUMS:
  PrivatePod: abc
`)
	if !contains(warnings, "PrivatePod") {
		t.Errorf("expected warning about external pod, got: %v", warnings)
	}
}

func TestPodfileLock_Parse_MissingFileErrors(t *testing.T) {
	_, err := (cocoapods.PodfileLock{}).Parse(fstest.MapFS{}, "Podfile.lock")
	if err == nil {
		t.Error("expected error when file missing")
	}
}

func TestPodfileLock_Parse_EmptyLockfile(t *testing.T) {
	pkgs, _ := parsePodfile(t, "")
	if len(pkgs) != 0 {
		t.Errorf("expected no packages, got: %+v", pkgs)
	}
}

func contains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}
