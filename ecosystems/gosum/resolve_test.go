package gosum

import (
	"strings"
	"testing"
)

// TestParseModDownload_Success: a successful `go mod download -json`
// response yields a Hash with the h1: prefix.
func TestParseModDownload_Success(t *testing.T) {
	js := `{"Path":"github.com/google/uuid","Version":"v1.6.0","Sum":"h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0="}`
	info, err := parseModDownload([]byte(js), "github.com/google/uuid", "v1.6.0")
	if err != nil {
		t.Fatalf("parseModDownload: %v", err)
	}
	if info.Hash != "h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=" {
		t.Errorf("unexpected Hash: %q", info.Hash)
	}
	if info.HashAlgo != "h1" {
		t.Errorf("expected HashAlgo h1, got %q", info.HashAlgo)
	}
	if info.PackageID != "github.com/google/uuid" || info.Version != "v1.6.0" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
}

// TestParseModDownload_ErrorField: when go mod download returns an
// Error, the parser surfaces it as a Go error.
func TestParseModDownload_ErrorField(t *testing.T) {
	js := `{"Path":"github.com/does/not/exist","Version":"v1.0.0","Error":"invalid version"}`
	_, err := parseModDownload([]byte(js), "github.com/does/not/exist", "v1.0.0")
	if err == nil {
		t.Fatal("expected error when Error field is present")
	}
	if !strings.Contains(err.Error(), "invalid version") {
		t.Errorf("expected error to surface 'invalid version', got: %v", err)
	}
}

// TestParseModDownload_NoSum: a successful response without a Sum
// field (rare, but possible for the main module) returns Hash="".
// Existence is the primary signal; hash is a bonus.
func TestParseModDownload_NoSum(t *testing.T) {
	js := `{"Path":"github.com/foo","Version":"v1.0.0"}`
	info, err := parseModDownload([]byte(js), "github.com/foo", "v1.0.0")
	if err != nil {
		t.Fatalf("parseModDownload: %v", err)
	}
	if info.Hash != "" {
		t.Errorf("expected empty Hash when Sum is absent, got %q", info.Hash)
	}
	if info.HashAlgo != "" {
		t.Errorf("expected empty HashAlgo when no hash, got %q", info.HashAlgo)
	}
}

func TestParseModDownload_MalformedJSON(t *testing.T) {
	_, err := parseModDownload([]byte(`{not json}`), "x", "v1")
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}
