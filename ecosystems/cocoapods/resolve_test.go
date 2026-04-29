package cocoapods

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type fakeTrunk struct {
	srv      *httptest.Server
	lastPath string
}

func newFakeTrunk(t *testing.T, h http.Handler) (*fakeTrunk, func()) {
	t.Helper()
	env := &fakeTrunk{}
	env.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.lastPath = r.URL.Path
		h.ServeHTTP(w, r)
	}))
	prev := setCocoapodsEnv(t, env.srv.URL)
	return env, func() {
		env.srv.Close()
		prev()
	}
}

func setCocoapodsEnv(t *testing.T, val string) func() {
	t.Helper()
	prev, had := os.LookupEnv("TILLIT_COCOAPODS_URL")
	if err := os.Setenv("TILLIT_COCOAPODS_URL", val); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	return func() {
		if had {
			_ = os.Setenv("TILLIT_COCOAPODS_URL", prev)
		} else {
			_ = os.Unsetenv("TILLIT_COCOAPODS_URL")
		}
	}
}

func TestResolveVersion_Found(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Trunk returns the podspec content. We don't parse it; a
		// 200 just confirms (name, version) exists.
		w.Write([]byte(`{"name": "Alamofire", "version": "5.8.0"}`))
	})
	env, cleanup := newFakeTrunk(t, h)
	defer cleanup()

	info, err := (cocoapodsCommon{}).ResolveVersion("Alamofire", "5.8.0")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if info.PackageID != "Alamofire" || info.Version != "5.8.0" {
		t.Errorf("unexpected pkg/ver: %+v", info)
	}
	if !strings.Contains(env.lastPath, "/api/v1/pods/Alamofire/specs/5.8.0") {
		t.Errorf("expected /api/v1/pods/<name>/specs/<ver> path, got %q", env.lastPath)
	}
}

func TestResolveVersion_NotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := newFakeTrunk(t, h)
	defer cleanup()

	_, err := (cocoapodsCommon{}).ResolveVersion("Alamofire", "999.0.0")
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
	_, cleanup := newFakeTrunk(t, h)
	defer cleanup()

	_, err := (cocoapodsCommon{}).ResolveVersion("Alamofire", "5.8.0")
	if err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestResolveVersion_SubspecResolvesAgainstParent(t *testing.T) {
	// Subspecs are keyed under the umbrella pod on trunk — they
	// don't have their own /specs endpoint. We resolve the umbrella
	// instead and trust the lockfile to record the right subspec
	// version.
	var caught string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caught = r.URL.Path
		w.Write([]byte(`{}`))
	})
	_, cleanup := newFakeTrunk(t, h)
	defer cleanup()

	if _, err := (cocoapodsCommon{}).ResolveVersion("Firebase/Core", "10.0.0"); err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if !strings.Contains(caught, "/pods/Firebase/specs/10.0.0") {
		t.Errorf("expected umbrella-pod URL, got %q", caught)
	}
}
