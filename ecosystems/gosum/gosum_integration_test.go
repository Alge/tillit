package gosum_test

import (
	"os"
	"testing"

	"github.com/Alge/tillit/ecosystems/gosum"
)

// Sanity check: can we parse this repo's own go.sum (with its sibling
// go.mod) without warnings, and do we correctly label direct deps?
func TestGoSum_ParsesRepoOwnGoSum(t *testing.T) {
	if _, err := os.Stat("../../go.sum"); err != nil {
		t.Skipf("repo go.sum not found: %v", err)
		return
	}
	fsys := os.DirFS("../..")

	res, err := (gosum.GoSum{}).Parse(fsys, "go.sum")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(res.Packages) == 0 {
		t.Fatal("expected at least one package from real go.sum")
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings on real go.sum: %v", res.Warnings)
	}

	directs := 0
	for _, p := range res.Packages {
		if p.PackageID == "" || p.Version == "" {
			t.Errorf("malformed PackageRef: %+v", p)
		}
		if p.Ecosystem != "go" {
			t.Errorf("Ecosystem = %q, want %q", p.Ecosystem, "go")
		}
		if p.Direct {
			directs++
		}
	}
	if directs == 0 {
		t.Error("expected at least one direct dependency in this repo's go.sum")
	}
}
