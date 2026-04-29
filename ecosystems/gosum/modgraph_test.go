package gosum

import (
	"strings"
	"testing"
)

func TestParseModGraph_BuildsEdges(t *testing.T) {
	input := `mainmod golang.org/x/mod@v0.27.0
mainmod golang.org/x/sync@v0.10.0
golang.org/x/mod@v0.27.0 golang.org/x/tools@v0.5.0
`
	edges, warnings := parseModGraph(strings.NewReader(input))
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	roots := edges["mainmod"]
	if len(roots) != 2 {
		t.Fatalf("mainmod should have 2 deps, got %v", roots)
	}
	if got := edges["golang.org/x/mod@v0.27.0"]; len(got) != 1 || got[0] != "golang.org/x/tools@v0.5.0" {
		t.Errorf("expected mod->tools edge, got %v", got)
	}
}

func TestParseModGraph_DedupesIdenticalEdges(t *testing.T) {
	input := `a b
a b
a c
`
	edges, _ := parseModGraph(strings.NewReader(input))
	if got := edges["a"]; len(got) != 2 {
		t.Errorf("expected 2 unique edges from a, got %v", got)
	}
}

func TestParseModGraph_SkipsMalformedLines(t *testing.T) {
	input := "good line\njust-one-token\nthree word lines wrong\nx y\n"
	edges, _ := parseModGraph(strings.NewReader(input))
	// Only "good line" and "x y" should be parsed.
	if len(edges) != 2 {
		t.Errorf("expected 2 from-nodes after skipping bad lines, got %v", edges)
	}
}
