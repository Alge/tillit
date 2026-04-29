package cargo

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type fakeCrates struct {
	srv      *httptest.Server
	lastPath string
}

func newFakeCrates(t *testing.T, h http.Handler) (*fakeCrates, func()) {
	t.Helper()
	env := &fakeCrates{}
	env.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.lastPath = r.URL.Path
		h.ServeHTTP(w, r)
	}))
	prev := setCargoEnv(t, env.srv.URL)
	return env, func() {
		env.srv.Close()
		prev()
	}
}

func setCargoEnv(t *testing.T, val string) func() {
	t.Helper()
	prev, had := os.LookupEnv("TILLIT_CARGO_URL")
	if err := os.Setenv("TILLIT_CARGO_URL", val); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() {
		if had {
			_ = os.Setenv("TILLIT_CARGO_URL", prev)
		} else {
			_ = os.Unsetenv("TILLIT_CARGO_URL")
		}
	}
}

func TestResolveVersion_Found(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"version": {
				"crate": "anyhow",
				"num": "1.0.75",
				"checksum": "a4668cab20f66d8d020e1fbc0ebe47217433c1b6c8f2040faf858554e394ace6"
			}
		}`))
	})
	env, cleanup := newFakeCrates(t, h)
	defer cleanup()

	info, err := (cargoCommon{}).ResolveVersion("anyhow", "1.0.75")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.PackageID != "anyhow" || info.Version != "1.0.75" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
	if info.Hash != "a4668cab20f66d8d020e1fbc0ebe47217433c1b6c8f2040faf858554e394ace6" {
		t.Errorf("expected checksum, got %q", info.Hash)
	}
	if info.HashAlgo != "sha256" {
		t.Errorf("expected sha256 algo, got %q", info.HashAlgo)
	}
	if !strings.Contains(env.lastPath, "/api/v1/crates/anyhow/1.0.75") {
		t.Errorf("expected /api/v1/crates/<name>/<version> path, got %q", env.lastPath)
	}
}

func TestResolveVersion_NotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := newFakeCrates(t, h)
	defer cleanup()

	_, err := (cargoCommon{}).ResolveVersion("anyhow", "999.0.0")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' phrasing, got: %v", err)
	}
}

func TestResolveVersion_ServerError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	_, cleanup := newFakeCrates(t, h)
	defer cleanup()

	_, err := (cargoCommon{}).ResolveVersion("anyhow", "1.0.0")
	if err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestResolveVersion_NoChecksumOmitted(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version": {"crate": "anyhow", "num": "1.0.0"}}`))
	})
	_, cleanup := newFakeCrates(t, h)
	defer cleanup()

	info, err := (cargoCommon{}).ResolveVersion("anyhow", "1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "" {
		t.Errorf("expected empty hash when checksum absent, got %q", info.Hash)
	}
}

func TestResolveVersion_URLEncodesPackageName(t *testing.T) {
	// Crate names are limited to ASCII alphanumerics, dashes, and
	// underscores, so escaping is mostly a no-op — but the encoder
	// should still be in the path so future namespaces don't break.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version": {"crate": "ok", "num": "1.0.0"}}`))
	})
	env, cleanup := newFakeCrates(t, h)
	defer cleanup()

	if _, err := (cargoCommon{}).ResolveVersion("serde_json", "1.0.0"); err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if !strings.Contains(env.lastPath, "/api/v1/crates/serde_json/1.0.0") {
		t.Errorf("expected the name in the URL, got %q", env.lastPath)
	}
}
