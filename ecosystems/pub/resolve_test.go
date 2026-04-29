package pub

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type fakePub struct {
	srv      *httptest.Server
	lastPath string
}

func newFakePub(t *testing.T, h http.Handler) (*fakePub, func()) {
	t.Helper()
	env := &fakePub{}
	env.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.lastPath = r.URL.Path
		h.ServeHTTP(w, r)
	}))
	prev := setPubEnv(t, env.srv.URL)
	return env, func() {
		env.srv.Close()
		prev()
	}
}

func setPubEnv(t *testing.T, val string) func() {
	t.Helper()
	prev, had := os.LookupEnv("TILLIT_PUB_URL")
	if err := os.Setenv("TILLIT_PUB_URL", val); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() {
		if had {
			_ = os.Setenv("TILLIT_PUB_URL", prev)
		} else {
			_ = os.Unsetenv("TILLIT_PUB_URL")
		}
	}
}

func TestResolveVersion_Found(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"version": "1.1.0",
			"archive_sha256": "deadbeefcafef00d",
			"archive_url": "https://pub.dev/packages/http/versions/1.1.0.tar.gz"
		}`))
	})
	env, cleanup := newFakePub(t, h)
	defer cleanup()

	info, err := (pubCommon{}).ResolveVersion("http", "1.1.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.PackageID != "http" || info.Version != "1.1.0" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
	if info.Hash != "deadbeefcafef00d" {
		t.Errorf("expected archive_sha256, got %q", info.Hash)
	}
	if info.HashAlgo != "sha256" {
		t.Errorf("expected sha256 algo, got %q", info.HashAlgo)
	}
	if !strings.Contains(env.lastPath, "/api/packages/http/versions/1.1.0") {
		t.Errorf("expected /api/packages/<name>/versions/<ver> path, got %q", env.lastPath)
	}
}

func TestResolveVersion_NotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := newFakePub(t, h)
	defer cleanup()

	_, err := (pubCommon{}).ResolveVersion("http", "999.0.0")
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
	_, cleanup := newFakePub(t, h)
	defer cleanup()

	_, err := (pubCommon{}).ResolveVersion("http", "1.0.0")
	if err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestResolveVersion_NoHashOmitted(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version": "1.0.0"}`))
	})
	_, cleanup := newFakePub(t, h)
	defer cleanup()

	info, err := (pubCommon{}).ResolveVersion("http", "1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "" {
		t.Errorf("expected empty hash, got %q", info.Hash)
	}
}
