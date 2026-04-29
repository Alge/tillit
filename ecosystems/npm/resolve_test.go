package npm

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func setEnv(t *testing.T, key, val string) func() {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() {
		if had {
			os.Setenv(key, prev)
		} else {
			os.Unsetenv(key)
		}
	}
}

func TestResolveVersion_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"lodash","version":"4.17.21","dist":{"integrity":"sha512-fake-integrity","shasum":"deadbeef"}}`))
	}))
	defer srv.Close()
	defer setEnv(t, "npm_config_registry", srv.URL)()

	info, err := (PackageLock{}).ResolveVersion("lodash", "4.17.21")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "sha512-fake-integrity" {
		t.Errorf("expected sha512-fake-integrity, got %q", info.Hash)
	}
	if info.HashAlgo != "sha512" {
		t.Errorf("expected HashAlgo sha512, got %q", info.HashAlgo)
	}
}

func TestResolveVersion_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "version not found", http.StatusNotFound)
	}))
	defer srv.Close()
	defer setEnv(t, "npm_config_registry", srv.URL)()

	_, err := (PackageLock{}).ResolveVersion("lodash", "9.9.9")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' phrasing, got: %v", err)
	}
}

func TestResolveVersion_FallsBackToShasumWhenNoIntegrity(t *testing.T) {
	// Older registry entries sometimes lack integrity — use sha1.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"old-pkg","version":"1.0.0","dist":{"shasum":"abcdef0123456789"}}`))
	}))
	defer srv.Close()
	defer setEnv(t, "npm_config_registry", srv.URL)()

	info, err := (PackageLock{}).ResolveVersion("old-pkg", "1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "sha1-abcdef0123456789" {
		t.Errorf("expected fallback to sha1, got %q", info.Hash)
	}
	if info.HashAlgo != "sha1" {
		t.Errorf("expected HashAlgo sha1, got %q", info.HashAlgo)
	}
}

func TestResolveVersion_ScopedPackagePathSurvivesURL(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path
		w.Write([]byte(`{"name":"@types/node","version":"20.0.0","dist":{"integrity":"sha512-x"}}`))
	}))
	defer srv.Close()
	defer setEnv(t, "npm_config_registry", srv.URL)()

	if _, err := (PackageLock{}).ResolveVersion("@types/node", "20.0.0"); err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if !strings.HasPrefix(seen, "/@types/node/20.0.0") {
		t.Errorf("expected scoped path '/@types/node/20.0.0' in URL, got %q", seen)
	}
}

func TestParseRegistryResponse_RejectsErrorPayload(t *testing.T) {
	body := []byte(`{"error":"version not found"}`)
	_, err := parseRegistryResponse(body, "x", "1.0.0")
	if err == nil {
		t.Fatal("expected error when registry response has error field")
	}
}

func TestParseRegistryResponse_MissingVersionFails(t *testing.T) {
	body := []byte(`{"name":"x"}`)
	_, err := parseRegistryResponse(body, "x", "1.0.0")
	if err == nil {
		t.Fatal("expected error on missing version field")
	}
}
