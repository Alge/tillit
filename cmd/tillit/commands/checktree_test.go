package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Alge/tillit/ecosystems"
	"github.com/Alge/tillit/resolver"
)

func mkRow(pkgID, version string, direct bool, status resolver.Status) row {
	return row{
		Pkg:    ecosystems.PackageRef{Ecosystem: "go", PackageID: pkgID, Version: version, Direct: direct},
		Status: status,
	}
}

func TestRenderTree_NestsTransitivesUnderDirect(t *testing.T) {
	rows := []row{
		mkRow("github.com/cloudflare/circl", "v1.6.3", true, resolver.StatusUnknown),
		mkRow("github.com/cespare/xxhash/v2", "v2.3.0", false, resolver.StatusUnknown),
		mkRow("golang.org/x/sync", "v0.10.0", false, resolver.StatusVetted),
		mkRow("golang.org/x/crypto", "v0.30.0", false, resolver.StatusVetted),
	}
	edges := map[string][]string{
		"github.com/cloudflare/circl@v1.6.3": {
			"github.com/cespare/xxhash/v2@v2.3.0",
			"golang.org/x/crypto@v0.30.0",
		},
		"github.com/cespare/xxhash/v2@v2.3.0": {"golang.org/x/sync@v0.10.0"},
	}

	var buf bytes.Buffer
	renderTree(&buf, rows, edges)
	out := buf.String()

	// Direct dep at column 0 with the * marker.
	if !strings.Contains(out, "github.com/cloudflare/circl v1.6.3 [unknown] *") {
		t.Errorf("expected direct root with marker, got:\n%s", out)
	}
	// Transitive nested under direct.
	if !strings.Contains(out, "├── github.com/cespare/xxhash/v2 v2.3.0 [unknown]") {
		t.Errorf("expected nested xxhash branch, got:\n%s", out)
	}
	// Grand-transitive nested two levels deep.
	if !strings.Contains(out, "│   └── golang.org/x/sync v0.10.0 [vetted]") {
		t.Errorf("expected double-nested sync leaf, got:\n%s", out)
	}
	// Last child uses └── instead of ├──
	if !strings.Contains(out, "└── golang.org/x/crypto v0.30.0 [vetted]") {
		t.Errorf("expected last-child └── on crypto, got:\n%s", out)
	}
	// Footer.
	if !strings.Contains(out, "* = direct dependency") {
		t.Errorf("expected legend footer, got:\n%s", out)
	}
}

func TestRenderTree_DiamondShownOnceWithStarMarker(t *testing.T) {
	// A and B are both direct; both depend on C. C should appear in
	// full under one of them and as `(*)` under the other.
	rows := []row{
		mkRow("a", "v1", true, resolver.StatusUnknown),
		mkRow("b", "v1", true, resolver.StatusUnknown),
		mkRow("c", "v1", false, resolver.StatusVetted),
	}
	edges := map[string][]string{
		"a@v1": {"c@v1"},
		"b@v1": {"c@v1"},
	}

	var buf bytes.Buffer
	renderTree(&buf, rows, edges)
	out := buf.String()

	if strings.Count(out, "(*)") != 1 {
		t.Errorf("expected one (*) dedupe marker, got:\n%s", out)
	}
}

func TestRenderTree_OrphansListedAtBottom(t *testing.T) {
	// "orphan" has no edge into it but is in the resolved set — must
	// still appear so it's not lost.
	rows := []row{
		mkRow("a", "v1", true, resolver.StatusUnknown),
		mkRow("orphan", "v1", false, resolver.StatusUnknown),
	}
	edges := map[string][]string{}

	var buf bytes.Buffer
	renderTree(&buf, rows, edges)
	out := buf.String()

	if !strings.Contains(out, "(packages not reached from any direct dependency)") {
		t.Errorf("expected orphan section header, got:\n%s", out)
	}
	if !strings.Contains(out, "orphan v1 [unknown]") {
		t.Errorf("expected orphan listed, got:\n%s", out)
	}
}

func TestRenderTree_HandlesCycle(t *testing.T) {
	// Defensive: cycle a→b→a should not infinite-loop.
	rows := []row{
		mkRow("a", "v1", true, resolver.StatusUnknown),
		mkRow("b", "v1", false, resolver.StatusUnknown),
	}
	edges := map[string][]string{
		"a@v1": {"b@v1"},
		"b@v1": {"a@v1"},
	}

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		renderTree(&buf, rows, edges)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("renderTree appears to be in an infinite loop")
	}
	if !strings.Contains(buf.String(), "(*)") {
		t.Errorf("expected cycle-break (*) marker, got:\n%s", buf.String())
	}
}
