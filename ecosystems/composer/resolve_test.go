package composer

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type fakePackagist struct {
	srv      *httptest.Server
	lastPath string
}

func newFakePackagist(t *testing.T, h http.Handler) (*fakePackagist, func()) {
	t.Helper()
	env := &fakePackagist{}
	env.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.lastPath = r.URL.Path
		h.ServeHTTP(w, r)
	}))
	prev := setComposerEnv(t, env.srv.URL)
	return env, func() {
		env.srv.Close()
		prev()
	}
}

func setComposerEnv(t *testing.T, val string) func() {
	t.Helper()
	prev, had := os.LookupEnv("TILLIT_COMPOSER_URL")
	if err := os.Setenv("TILLIT_COMPOSER_URL", val); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() {
		if had {
			_ = os.Setenv("TILLIT_COMPOSER_URL", prev)
		} else {
			_ = os.Unsetenv("TILLIT_COMPOSER_URL")
		}
	}
}

func TestResolveVersion_Found(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"packages": {
				"guzzlehttp/guzzle": [
					{"name": "guzzlehttp/guzzle", "version": "7.8.0",
					 "dist": {"type": "zip", "url": "...", "shasum": "deadbeef"}},
					{"name": "guzzlehttp/guzzle", "version": "7.7.0",
					 "dist": {"type": "zip", "url": "...", "shasum": "cafef00d"}}
				]
			}
		}`))
	})
	env, cleanup := newFakePackagist(t, h)
	defer cleanup()

	info, err := (composerCommon{}).ResolveVersion("guzzlehttp/guzzle", "7.8.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.PackageID != "guzzlehttp/guzzle" || info.Version != "7.8.0" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
	if info.Hash != "deadbeef" {
		t.Errorf("expected dist.shasum 'deadbeef', got %q", info.Hash)
	}
	if info.HashAlgo != "sha1" {
		// Packagist's `shasum` for zip dists is historically sha1.
		// We record the algo explicitly so callers can compare against
		// lockfile-recorded hashes correctly.
		t.Errorf("expected sha1 algo, got %q", info.HashAlgo)
	}
	if !strings.Contains(env.lastPath, "/p2/guzzlehttp/guzzle.json") {
		t.Errorf("expected /p2/<name>.json path, got %q", env.lastPath)
	}
}

func TestResolveVersion_AcceptsLeadingV(t *testing.T) {
	// Some packages publish their tags with a leading 'v'. Packagist
	// normalises away the prefix, but matching should still work.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"packages": {
				"vendor/pkg": [
					{"name": "vendor/pkg", "version": "v1.2.3", "dist": {"shasum": "abc"}}
				]
			}
		}`))
	})
	_, cleanup := newFakePackagist(t, h)
	defer cleanup()

	info, err := (composerCommon{}).ResolveVersion("vendor/pkg", "1.2.3")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "abc" {
		t.Errorf("expected hash 'abc', got %q", info.Hash)
	}
}

func TestResolveVersion_NotFound_404(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := newFakePackagist(t, h)
	defer cleanup()

	_, err := (composerCommon{}).ResolveVersion("vendor/pkg", "1.0.0")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' phrasing, got: %v", err)
	}
}

func TestResolveVersion_PackageFoundVersionMissing(t *testing.T) {
	// Packagist returns 200 + the package metadata even when the
	// requested version doesn't exist. We have to scan the array.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"packages": {
				"vendor/pkg": [
					{"name": "vendor/pkg", "version": "1.0.0", "dist": {"shasum": "a"}},
					{"name": "vendor/pkg", "version": "2.0.0", "dist": {"shasum": "b"}}
				]
			}
		}`))
	})
	_, cleanup := newFakePackagist(t, h)
	defer cleanup()

	_, err := (composerCommon{}).ResolveVersion("vendor/pkg", "999.0.0")
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
	_, cleanup := newFakePackagist(t, h)
	defer cleanup()

	_, err := (composerCommon{}).ResolveVersion("vendor/pkg", "1.0.0")
	if err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestResolveVersion_NoShasumOmitted(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"packages": {
				"vendor/pkg": [
					{"name": "vendor/pkg", "version": "1.0.0", "dist": {"url": "..."}}
				]
			}
		}`))
	})
	_, cleanup := newFakePackagist(t, h)
	defer cleanup()

	info, err := (composerCommon{}).ResolveVersion("vendor/pkg", "1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "" {
		t.Errorf("expected empty hash when shasum absent, got %q", info.Hash)
	}
}

func TestEscapePackageName(t *testing.T) {
	// Packagist's package URL accepts the literal `/` between vendor
	// and package, so url.PathEscape (which would emit `%2F`) is the
	// wrong tool for the whole name. escapePackageName must escape
	// each side of the slash independently and rejoin with a literal
	// `/`. This test pins that contract so a future "simplify with
	// PathEscape" refactor doesn't silently break Packagist lookups.
	cases := []struct {
		in, want string
	}{
		// Normal lowercase name — no escaping needed; the slash
		// must remain literal.
		{"guzzlehttp/guzzle", "guzzlehttp/guzzle"},
		{"symfony/console", "symfony/console"},

		// Names with characters that PathEscape would escape — the
		// segments are encoded but the joining slash is preserved.
		{"vendor/with space", "vendor/with%20space"},
		{"vendor/with+plus", "vendor/with+plus"}, // '+' is path-safe per RFC 3986

		// Single-segment input (no slash) is rare/invalid for
		// Composer but must not panic — it should pass through
		// PathEscape on its own.
		{"single", "single"},
	}
	for _, tc := range cases {
		if got := escapePackageName(tc.in); got != tc.want {
			t.Errorf("escapePackageName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveVersion_PreservesSlashInScopedName(t *testing.T) {
	// Integration-level guard for the same contract: a vendor/package
	// name must end up in the URL with a literal slash, not %2F. If
	// this ever regresses Packagist returns 404 in production.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"packages": {"my-vendor/my-pkg": [
			{"name": "my-vendor/my-pkg", "version": "1.0.0", "dist": {"shasum": "abc"}}
		]}}`))
	})
	env, cleanup := newFakePackagist(t, h)
	defer cleanup()

	if _, err := (composerCommon{}).ResolveVersion("my-vendor/my-pkg", "1.0.0"); err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if !strings.Contains(env.lastPath, "/p2/my-vendor/my-pkg.json") {
		t.Errorf("expected literal slash in path, got %q", env.lastPath)
	}
	if strings.Contains(env.lastPath, "%2F") {
		t.Errorf("path must not URL-encode the vendor/package separator, got %q", env.lastPath)
	}
}
