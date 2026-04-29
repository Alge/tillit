package nuget

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type fakeNuget struct {
	srv      *httptest.Server
	lastPath string
}

func newFakeNuget(t *testing.T, h http.Handler) (*fakeNuget, func()) {
	t.Helper()
	env := &fakeNuget{}
	env.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.lastPath = r.URL.Path
		h.ServeHTTP(w, r)
	}))
	prev := setNugetEnv(t, env.srv.URL)
	return env, func() {
		env.srv.Close()
		prev()
	}
}

func setNugetEnv(t *testing.T, val string) func() {
	t.Helper()
	prev, had := os.LookupEnv("TILLIT_NUGET_URL")
	if err := os.Setenv("TILLIT_NUGET_URL", val); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() {
		if had {
			_ = os.Setenv("TILLIT_NUGET_URL", prev)
		} else {
			_ = os.Unsetenv("TILLIT_NUGET_URL")
		}
	}
}

func TestResolveVersion_Found(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The flat container exposes a tiny text file with the
		// base64 sha512 of the .nupkg.
		w.Write([]byte("ppPFpBcvxdsfUonNcvITKqLl3bqxWbDCZIzDWHzjpdAHRFfZe0Dw9HmA0+za13IdyrgJwpkDTDA9fHaxOrt20A=="))
	})
	env, cleanup := newFakeNuget(t, h)
	defer cleanup()

	info, err := (nugetCommon{}).ResolveVersion("Newtonsoft.Json", "13.0.1")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.PackageID != "Newtonsoft.Json" || info.Version != "13.0.1" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
	if info.Hash == "" || !strings.HasPrefix(info.Hash, "ppPFp") {
		t.Errorf("expected base64 sha512 hash, got %q", info.Hash)
	}
	if info.HashAlgo != "sha512" {
		t.Errorf("expected sha512 algo, got %q", info.HashAlgo)
	}
	// The flat container path uses the lowercased package id and
	// includes the version twice — once as a directory, once as the
	// filename prefix.
	if !strings.Contains(env.lastPath, "/v3-flatcontainer/newtonsoft.json/13.0.1/newtonsoft.json.13.0.1.nupkg.sha512") {
		t.Errorf("expected lowercased flat-container path, got %q", env.lastPath)
	}
}

func TestResolveVersion_NotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := newFakeNuget(t, h)
	defer cleanup()

	_, err := (nugetCommon{}).ResolveVersion("Newtonsoft.Json", "999.0.0")
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
	_, cleanup := newFakeNuget(t, h)
	defer cleanup()

	_, err := (nugetCommon{}).ResolveVersion("Newtonsoft.Json", "13.0.1")
	if err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestResolveVersion_TrimsWhitespace(t *testing.T) {
	// Some servers return the hash with a trailing newline. Trim so
	// callers comparing against another hash literal don't see a
	// false mismatch.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("abcdef\n"))
	})
	_, cleanup := newFakeNuget(t, h)
	defer cleanup()

	info, err := (nugetCommon{}).ResolveVersion("any", "1.0.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.Hash != "abcdef" {
		t.Errorf("expected trimmed hash, got %q", info.Hash)
	}
}
