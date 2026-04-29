package hexpm

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type fakeHexpm struct {
	srv      *httptest.Server
	lastPath string
}

func newFakeHexpm(t *testing.T, h http.Handler) (*fakeHexpm, func()) {
	t.Helper()
	env := &fakeHexpm{}
	env.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.lastPath = r.URL.Path
		h.ServeHTTP(w, r)
	}))
	prev := setHexpmEnv(t, env.srv.URL)
	return env, func() {
		env.srv.Close()
		prev()
	}
}

func setHexpmEnv(t *testing.T, val string) func() {
	t.Helper()
	prev, had := os.LookupEnv("TILLIT_HEXPM_URL")
	if err := os.Setenv("TILLIT_HEXPM_URL", val); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() {
		if had {
			_ = os.Setenv("TILLIT_HEXPM_URL", prev)
		} else {
			_ = os.Unsetenv("TILLIT_HEXPM_URL")
		}
	}
}

func TestResolveVersion_Found(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"version": "1.7.10",
			"checksum": "02189140a61b2ce85bb633a9b6fd02dff705a5f1ab4b9ede29ce91a3eb9d1b25",
			"url": "https://hex.pm/api/packages/phoenix/releases/1.7.10"
		}`))
	})
	env, cleanup := newFakeHexpm(t, h)
	defer cleanup()

	info, err := (hexpmCommon{}).ResolveVersion("phoenix", "1.7.10")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.PackageID != "phoenix" || info.Version != "1.7.10" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
	if info.Hash != "02189140a61b2ce85bb633a9b6fd02dff705a5f1ab4b9ede29ce91a3eb9d1b25" {
		t.Errorf("expected checksum, got %q", info.Hash)
	}
	if info.HashAlgo != "sha256" {
		t.Errorf("expected sha256 algo, got %q", info.HashAlgo)
	}
	if !strings.Contains(env.lastPath, "/api/packages/phoenix/releases/1.7.10") {
		t.Errorf("expected /api/packages/<name>/releases/<ver> path, got %q", env.lastPath)
	}
}

func TestResolveVersion_NotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := newFakeHexpm(t, h)
	defer cleanup()

	_, err := (hexpmCommon{}).ResolveVersion("phoenix", "999.0.0")
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
	_, cleanup := newFakeHexpm(t, h)
	defer cleanup()

	_, err := (hexpmCommon{}).ResolveVersion("phoenix", "1.0.0")
	if err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestResolveVersion_NoChecksumOmitted(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version": "1.0.0"}`))
	})
	_, cleanup := newFakeHexpm(t, h)
	defer cleanup()

	info, err := (hexpmCommon{}).ResolveVersion("phoenix", "1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "" {
		t.Errorf("expected empty hash when checksum absent, got %q", info.Hash)
	}
}
