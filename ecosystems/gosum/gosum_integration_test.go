package gosum_test

import (
	"os"
	"testing"

	"github.com/Alge/tillit/ecosystems/gosum"
)

// Sanity check: can we parse this repo's own go.sum without warnings?
func TestGoSum_ParsesRepoOwnGoSum(t *testing.T) {
	f, err := os.Open("../../go.sum")
	if err != nil {
		t.Skipf("repo go.sum not found: %v", err)
		return
	}
	defer f.Close()

	res, err := (gosum.GoSum{}).Parse(f)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(res.Packages) == 0 {
		t.Fatal("expected at least one package from real go.sum")
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings on real go.sum: %v", res.Warnings)
	}

	// Spot-check: every package should have a non-empty PackageID and Version.
	for _, p := range res.Packages {
		if p.PackageID == "" || p.Version == "" {
			t.Errorf("malformed PackageRef: %+v", p)
		}
		if p.Ecosystem != "go" {
			t.Errorf("Ecosystem = %q, want %q", p.Ecosystem, "go")
		}
	}
}
