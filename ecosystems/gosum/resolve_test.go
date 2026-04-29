package gosum

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeGoEnv stages a mock Go module proxy + sumdb on httptest
// servers and points GOPROXY/GOSUMDB at them for the duration of
// the test. Returned cleanup restores prior env values and shuts
// the servers down.
type fakeGoEnv struct {
	proxy *httptest.Server
	sumdb *httptest.Server

	// Latest paths the helpers received — handy for asserting
	// module-path encoding rules.
	lastInfoPath   string
	lastLookupPath string
}

func newFakeGoEnv(t *testing.T, proxyHandler, sumdbHandler http.Handler) (*fakeGoEnv, func()) {
	t.Helper()
	env := &fakeGoEnv{}

	wrapProxy := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			env.lastInfoPath = r.URL.Path
			h.ServeHTTP(w, r)
		})
	}
	wrapSumdb := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			env.lastLookupPath = r.URL.Path
			h.ServeHTTP(w, r)
		})
	}
	env.proxy = httptest.NewServer(wrapProxy(proxyHandler))
	env.sumdb = httptest.NewServer(wrapSumdb(sumdbHandler))

	prevProxy := setEnv(t, "GOPROXY", env.proxy.URL)
	prevSum := setEnv(t, "GOSUMDB", env.sumdb.URL)
	return env, func() {
		env.proxy.Close()
		env.sumdb.Close()
		prevProxy()
		prevSum()
	}
}

func setEnv(t *testing.T, key, val string) func() {
	t.Helper()
	prev, had := lookupEnv(key)
	if err := setenv(key, val); err != nil {
		t.Fatalf("setenv %s: %v", key, err)
	}
	return func() {
		if had {
			_ = setenv(key, prev)
		} else {
			_ = unsetenv(key)
		}
	}
}

// Wrappers around os.* so the helper file doesn't need imports
// duplicated in every test file.
func lookupEnv(k string) (string, bool) { return osLookupEnv(k) }
func setenv(k, v string) error          { return osSetenv(k, v) }
func unsetenv(k string) error           { return osUnsetenv(k) }

func TestResolveVersion_Found(t *testing.T) {
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// .info responses are tiny JSON.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Version":"v1.0.0","Time":"2024-01-01T00:00:00Z"}`))
	})
	sumdb := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("github.com/foo/bar v1.0.0 h1:fakeziphash=\ngithub.com/foo/bar v1.0.0/go.mod h1:fakemodhash=\n"))
	})
	env, cleanup := newFakeGoEnv(t, proxy, sumdb)
	defer cleanup()
	_ = env

	info, err := (GoSum{}).ResolveVersion("github.com/foo/bar", "v1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.PackageID != "github.com/foo/bar" || info.Version != "v1.0.0" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
	if info.Hash != "h1:fakeziphash=" {
		t.Errorf("expected zip hash, got %q", info.Hash)
	}
	if info.HashAlgo != "h1" {
		t.Errorf("expected HashAlgo h1, got %q", info.HashAlgo)
	}
}

func TestResolveVersion_NotFoundReturnsError(t *testing.T) {
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	sumdb := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should never reach here — proxy 404 short-circuits.
		t.Errorf("sumdb hit on a not-found case: %s", r.URL.Path)
	})
	_, cleanup := newFakeGoEnv(t, proxy, sumdb)
	defer cleanup()

	_, err := (GoSum{}).ResolveVersion("github.com/foo/bar", "v999.0.0")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' phrasing, got: %v", err)
	}
}

func TestResolveVersion_HashOptional_ProxyWorksSumdbFails(t *testing.T) {
	// Existence proves out, but sumdb is down. The result must
	// still be returned with an empty Hash — the existence signal
	// is the primary requirement.
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Version":"v1.0.0"}`))
	})
	sumdb := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	})
	_, cleanup := newFakeGoEnv(t, proxy, sumdb)
	defer cleanup()

	info, err := (GoSum{}).ResolveVersion("github.com/foo/bar", "v1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "" {
		t.Errorf("hash should be empty when sumdb failed, got %q", info.Hash)
	}
}

// TestResolveVersion_GOSUMDBOff: GOSUMDB=off skips the hash lookup
// entirely; existence still works.
func TestResolveVersion_GOSUMDBOff(t *testing.T) {
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Version":"v1.0.0"}`))
	})
	sumdbHit := false
	sumdb := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sumdbHit = true
	})
	_, cleanup := newFakeGoEnv(t, proxy, sumdb)
	defer cleanup()
	defer setEnv(t, "GOSUMDB", "off")()

	info, err := (GoSum{}).ResolveVersion("github.com/foo/bar", "v1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if sumdbHit {
		t.Error("GOSUMDB=off must prevent any sumdb HTTP call")
	}
	if info.Hash != "" {
		t.Error("Hash must be empty when sumdb is off")
	}
}

// TestResolveVersion_EncodesUppercaseInModulePath: the proxy URL
// must escape uppercase letters — e.g. github.com/Bar/Baz becomes
// github.com/!bar/!baz.
func TestResolveVersion_EncodesUppercaseInModulePath(t *testing.T) {
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Version":"v1.0.0"}`))
	})
	sumdb := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	env, cleanup := newFakeGoEnv(t, proxy, sumdb)
	defer cleanup()

	if _, err := (GoSum{}).ResolveVersion("github.com/Bar/Baz", "v1.0.0"); err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if !strings.Contains(env.lastInfoPath, "/!bar/!baz/@v/v1.0.0.info") {
		t.Errorf("expected uppercase-escaped path in proxy URL, got %q", env.lastInfoPath)
	}
}

// TestParseLookup_PicksZipHashLineNotGoModLine: the sumdb response
// has TWO h1: hash lines (zip + go.mod). We only want the first.
func TestParseLookup_PicksZipHashLineNotGoModLine(t *testing.T) {
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Version":"v1.0.0"}`))
	})
	sumdb := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Order matches real sumdb: zip first, then go.mod, then
		// tree headers we ignore.
		w.Write([]byte(
			"github.com/foo/bar v1.0.0 h1:THE-ZIP-HASH=\n" +
				"github.com/foo/bar v1.0.0/go.mod h1:WRONG-IF-PICKED=\n" +
				"\ngo.sum database tree\n123\nfakeroot\n",
		))
	})
	_, cleanup := newFakeGoEnv(t, proxy, sumdb)
	defer cleanup()

	info, err := (GoSum{}).ResolveVersion("github.com/foo/bar", "v1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "h1:THE-ZIP-HASH=" {
		t.Errorf("expected zip hash, got %q", info.Hash)
	}
}
