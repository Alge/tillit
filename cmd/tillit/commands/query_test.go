package commands

import (
	"strings"
	"testing"

	"github.com/Alge/tillit/resolver"
)

func TestVerboseDecisionLine_IncludesShortSignatureID(t *testing.T) {
	d := resolver.ContributingDecision{
		SignerID:    "abcdef0123456789",
		Path:        []string{"abcdef0123456789"},
		Level:       "allowed",
		SignatureID: "a3f9d2c1b8e74f5a6d0e9b2c4f8a1d3e5b7c9d1f2a4b6c8d0e2f4a6b8c0d2e4f",
		Kind:        resolver.KindExact,
		Version:     "v3.0.0",
	}
	out := verboseDecisionLine(d)
	if !strings.Contains(out, "a3f9d2c1") {
		t.Errorf("expected short hash %q in output, got: %q", "a3f9d2c1", out)
	}
}

func TestDecisionsSummary_IncludesShortSignatureID(t *testing.T) {
	ds := []resolver.ContributingDecision{{
		SignerID:    "abcdef0123456789",
		Level:       "allowed",
		SignatureID: "a3f9d2c1b8e74f5a6d0e9b2c4f8a1d3e5b7c9d1f2a4b6c8d0e2f4a6b8c0d2e4f",
		Kind:        resolver.KindExact,
		Version:     "v3.0.0",
	}}
	out := decisionsSummary(ds)
	if !strings.Contains(out, "a3f9d2c1") {
		t.Errorf("expected short hash %q in summary, got: %q", "a3f9d2c1", out)
	}
}
