package pypi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// fakePyPI stages a mock PyPI JSON server and points the adapter's
// configurable base URL at it for the duration of the test.
type fakePyPI struct {
	srv      *httptest.Server
	lastPath string
}

func newFakePyPI(t *testing.T, h http.Handler) (*fakePyPI, func()) {
	t.Helper()
	env := &fakePyPI{}
	env.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.lastPath = r.URL.Path
		h.ServeHTTP(w, r)
	}))
	prev := setPyPIEnv(t, env.srv.URL)
	return env, func() {
		env.srv.Close()
		prev()
	}
}

func setPyPIEnv(t *testing.T, val string) func() {
	t.Helper()
	prev, had := os.LookupEnv("TILLIT_PYPI_URL")
	if err := os.Setenv("TILLIT_PYPI_URL", val); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() {
		if had {
			_ = os.Setenv("TILLIT_PYPI_URL", prev)
		} else {
			_ = os.Unsetenv("TILLIT_PYPI_URL")
		}
	}
}

func TestResolveVersion_Found(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"info": {"name": "requests", "version": "2.31.0"},
			"urls": [
				{"packagetype": "bdist_wheel", "digests": {"sha256": "wheel-hash"}},
				{"packagetype": "sdist", "digests": {"sha256": "sdist-hash"}}
			]
		}`))
	})
	env, cleanup := newFakePyPI(t, h)
	defer cleanup()

	info, err := (Requirements{}).ResolveVersion("requests", "2.31.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.PackageID != "requests" || info.Version != "2.31.0" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
	// Prefer sdist over wheel — the sdist hash is the canonical one
	// for source-level vetting.
	if info.Hash != "sdist-hash" {
		t.Errorf("expected sdist hash, got %q", info.Hash)
	}
	if info.HashAlgo != "sha256" {
		t.Errorf("expected sha256 algo, got %q", info.HashAlgo)
	}
	if !strings.Contains(env.lastPath, "/pypi/requests/2.31.0/json") {
		t.Errorf("expected /pypi/<name>/<version>/json path, got %q", env.lastPath)
	}
}

func TestResolveVersion_NotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := newFakePyPI(t, h)
	defer cleanup()

	_, err := (Requirements{}).ResolveVersion("requests", "999.0.0")
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
	_, cleanup := newFakePyPI(t, h)
	defer cleanup()

	_, err := (Requirements{}).ResolveVersion("requests", "2.31.0")
	if err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestResolveVersion_NoSdistFallsBackToWheel(t *testing.T) {
	// PyPI sometimes hosts wheel-only releases. We still want a
	// content hash, so use the first wheel.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"info": {"name": "wheelonly", "version": "1.0"},
			"urls": [
				{"packagetype": "bdist_wheel", "digests": {"sha256": "first-wheel"}},
				{"packagetype": "bdist_wheel", "digests": {"sha256": "second-wheel"}}
			]
		}`))
	})
	_, cleanup := newFakePyPI(t, h)
	defer cleanup()

	info, err := (Requirements{}).ResolveVersion("wheelonly", "1.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "first-wheel" {
		t.Errorf("expected first wheel hash, got %q", info.Hash)
	}
}

func TestResolveVersion_NoUrlsHashOmitted(t *testing.T) {
	// Existence still validates even when no artifacts are listed
	// (rare, but possible for yanked or pre-upload entries).
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"info": {"name": "ghost", "version": "1.0"},
			"urls": []
		}`))
	})
	_, cleanup := newFakePyPI(t, h)
	defer cleanup()

	info, err := (Requirements{}).ResolveVersion("ghost", "1.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "" {
		t.Errorf("expected empty hash when no URLs present, got %q", info.Hash)
	}
}

func TestResolveVersion_NormalizesPackageName(t *testing.T) {
	// PyPI redirects e.g. `Django` to `django` itself, but the
	// adapter should send the canonical form so private mirrors
	// without redirect logic still work.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"info":{"name":"django","version":"4.2"},"urls":[]}`))
	})
	env, cleanup := newFakePyPI(t, h)
	defer cleanup()

	if _, err := (Requirements{}).ResolveVersion("Django", "4.2"); err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if !strings.Contains(env.lastPath, "/pypi/django/4.2/json") {
		t.Errorf("expected normalized name in URL, got %q", env.lastPath)
	}
}
