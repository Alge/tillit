package pypi_test

import (
	"testing"

	"github.com/Alge/tillit/ecosystems/pypi"
)

func TestRequirements_Ecosystem(t *testing.T) {
	if got := (pypi.Requirements{}).Ecosystem(); got != "pypi" {
		t.Errorf("Ecosystem() = %q, want %q", got, "pypi")
	}
}

func TestRequirements_Name(t *testing.T) {
	if got := (pypi.Requirements{}).Name(); got != "requirements.txt" {
		t.Errorf("Name() = %q, want %q", got, "requirements.txt")
	}
}

func TestRequirements_CanParse(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"requirements.txt", true},
		{"./requirements.txt", true},
		{"/some/path/requirements.txt", true},
		{"requirements-dev.txt", true},
		{"requirements-prod.txt", true},
		{"go.sum", false},
		{"Pipfile.lock", false},
		{"requirements.txt.bak", false},
		{"requirements.in", false},
		{"requirements", false},
		{"", false},
	}
	a := pypi.Requirements{}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := a.CanParse(tc.path); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
