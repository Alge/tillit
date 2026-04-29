package rubygems

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type fakeRubygems struct {
	srv      *httptest.Server
	lastPath string
}

func newFakeRubygems(t *testing.T, h http.Handler) (*fakeRubygems, func()) {
	t.Helper()
	env := &fakeRubygems{}
	env.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.lastPath = r.URL.Path
		h.ServeHTTP(w, r)
	}))
	prev := setRubygemsEnv(t, env.srv.URL)
	return env, func() {
		env.srv.Close()
		prev()
	}
}

func setRubygemsEnv(t *testing.T, val string) func() {
	t.Helper()
	prev, had := os.LookupEnv("TILLIT_RUBYGEMS_URL")
	if err := os.Setenv("TILLIT_RUBYGEMS_URL", val); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() {
		if had {
			_ = os.Setenv("TILLIT_RUBYGEMS_URL", prev)
		} else {
			_ = os.Unsetenv("TILLIT_RUBYGEMS_URL")
		}
	}
}

func TestResolveVersion_Found(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"number": "7.1.0", "sha": "newer-hash"},
			{"number": "7.0.0", "sha": "deadbeefcafef00d"},
			{"number": "6.1.0", "sha": "older-hash"}
		]`))
	})
	env, cleanup := newFakeRubygems(t, h)
	defer cleanup()

	info, err := (rubygemsCommon{}).ResolveVersion("rails", "7.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.PackageID != "rails" || info.Version != "7.0.0" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
	if info.Hash != "deadbeefcafef00d" {
		t.Errorf("expected matched-version hash, got %q", info.Hash)
	}
	if info.HashAlgo != "sha256" {
		t.Errorf("expected sha256 algo, got %q", info.HashAlgo)
	}
	if !strings.Contains(env.lastPath, "/api/v1/versions/rails.json") {
		t.Errorf("expected /api/v1/versions/<name>.json path, got %q", env.lastPath)
	}
}

func TestResolveVersion_NotFound_404(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := newFakeRubygems(t, h)
	defer cleanup()

	_, err := (rubygemsCommon{}).ResolveVersion("nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' phrasing, got: %v", err)
	}
}

func TestResolveVersion_PackageFoundVersionMissing(t *testing.T) {
	// RubyGems returns 200 + the full version array even when the
	// requested version doesn't exist. The adapter has to scan the
	// list and surface "not found" itself.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[
			{"number": "1.0.0", "sha": "a"},
			{"number": "2.0.0", "sha": "b"}
		]`))
	})
	_, cleanup := newFakeRubygems(t, h)
	defer cleanup()

	_, err := (rubygemsCommon{}).ResolveVersion("rails", "999.0.0")
	if err == nil {
		t.Fatal("expected error when version not in list")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' phrasing, got: %v", err)
	}
}

func TestResolveVersion_ServerError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	_, cleanup := newFakeRubygems(t, h)
	defer cleanup()

	_, err := (rubygemsCommon{}).ResolveVersion("rails", "7.0.0")
	if err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestResolveVersion_NoShaOmitted(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"number": "1.0.0"}]`))
	})
	_, cleanup := newFakeRubygems(t, h)
	defer cleanup()

	info, err := (rubygemsCommon{}).ResolveVersion("rails", "1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "" {
		t.Errorf("expected empty hash, got %q", info.Hash)
	}
}
