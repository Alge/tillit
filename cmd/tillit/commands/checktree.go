package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Alge/tillit/ecosystems"
	"github.com/Alge/tillit/resolver"
)

// nodeInfo is everything we need to render one tree node: the package
// label, version, and resolved status.
type nodeInfo struct {
	Label   string // packageID
	Version string // resolved version
	Status  resolver.Status
	Direct  bool
}

// renderTree prints a hierarchical view of all packages, rooted at each
// direct dependency. Subtrees are expanded fully every time they
// appear — diamond dependencies show up under each of their parents.
// Cycles (shouldn't exist in Go modules but we're defensive) are
// broken by tracking ancestors of the current branch and refusing to
// re-enter one. Position in the tree conveys whether a package is
// direct (depth 0) or transitive.
func renderTree(w io.Writer, rows []row, edges map[string][]string) {
	infos := indexRows(rows)
	resolvedKey := buildResolvedKeyMap(rows)
	edges = canonicaliseEdges(edges, infos, resolvedKey)
	directs := directRoots(rows)

	for i, root := range directs {
		if i > 0 {
			fmt.Fprintln(w)
		}
		printRoot(w, root, edges, infos)
	}

	// Anything not reachable from any direct gets listed flat at the
	// bottom so packages aren't silently dropped.
	if leftovers := unvisitedFromDirects(rows, edges, directs); len(leftovers) > 0 {
		if len(directs) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "(packages not reached from any direct dependency)")
		}
		for _, key := range leftovers {
			info := infos[key]
			fmt.Fprintf(w, "%s %s [%s]\n", info.Label, info.Version, info.Status)
		}
	}
}

func printRoot(w io.Writer, key string, edges map[string][]string, infos map[string]nodeInfo) {
	info := infos[key]
	fmt.Fprintf(w, "%s %s [%s]\n", info.Label, info.Version, info.Status)
	children := sortedChildren(edges[key], infos)
	ancestors := map[string]bool{key: true}
	for i, c := range children {
		printChild(w, c, "", i == len(children)-1, edges, infos, ancestors)
	}
}

func printChild(w io.Writer, key, prefix string, last bool, edges map[string][]string, infos map[string]nodeInfo, ancestors map[string]bool) {
	branch := "├── "
	nextPrefix := prefix + "│   "
	if last {
		branch = "└── "
		nextPrefix = prefix + "    "
	}

	info, known := infos[key]
	if !known {
		// Shouldn't happen after canonicaliseEdges, but guard anyway.
		return
	}

	fmt.Fprintf(w, "%s%s%s %s [%s]\n",
		prefix, branch, info.Label, info.Version, info.Status)

	if ancestors[key] {
		// Cycle break: don't recurse into a node that's an ancestor of
		// the current branch. (This shouldn't happen for Go modules.)
		return
	}
	ancestors[key] = true
	defer delete(ancestors, key)

	children := sortedChildren(edges[key], infos)
	for i, c := range children {
		printChild(w, c, nextPrefix, i == len(children)-1, edges, infos, ancestors)
	}
}

// indexRows builds a "module@version" -> nodeInfo map from the resolved
// rows.
func indexRows(rows []row) map[string]nodeInfo {
	out := map[string]nodeInfo{}
	for _, r := range rows {
		key := r.Pkg.PackageID + "@" + r.Pkg.Version
		out[key] = nodeInfo{
			Label:   r.Pkg.PackageID,
			Version: r.Pkg.Version,
			Status:  r.Status,
			Direct:  r.Pkg.Direct,
		}
	}
	return out
}

func directRoots(rows []row) []string {
	var out []string
	for _, r := range rows {
		if r.Pkg.Direct {
			out = append(out, r.Pkg.PackageID+"@"+r.Pkg.Version)
		}
	}
	sort.Strings(out)
	return out
}

// buildResolvedKeyMap maps each package id to the "id@version" key for
// the version the project actually uses (per go.sum / lockfile). Used
// to retarget graph edges that name an MVS-overridden version onto the
// version the resolver actually evaluated.
func buildResolvedKeyMap(rows []row) map[string]string {
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Pkg.PackageID] = r.Pkg.PackageID + "@" + r.Pkg.Version
	}
	return out
}

// canonicaliseEdges rewrites every edge endpoint to the resolved
// "id@version" we have in infos. Edges referencing meta nodes (e.g.
// "go@1.22.0" toolchain directive) or packages we don't track at all
// are dropped — they'd render as confusing leaves. Self-edges that
// arise from the rewrite are also dropped.
func canonicaliseEdges(edges map[string][]string, infos map[string]nodeInfo, resolved map[string]string) map[string][]string {
	out := make(map[string][]string, len(edges))
	canon := func(key string) string {
		if _, ok := infos[key]; ok {
			return key
		}
		id := strings.SplitN(key, "@", 2)[0]
		if alt, ok := resolved[id]; ok {
			return alt
		}
		return ""
	}
	for from, deps := range edges {
		fromKey := from
		if c := canon(from); c != "" {
			fromKey = c
		}
		seen := map[string]bool{}
		var keep []string
		for _, d := range deps {
			c := canon(d)
			if c == "" || c == fromKey || seen[c] {
				continue
			}
			seen[c] = true
			keep = append(keep, c)
		}
		if len(keep) > 0 {
			out[fromKey] = keep
		}
	}
	return out
}

func sortedChildren(keys []string, infos map[string]nodeInfo) []string {
	cp := append([]string(nil), keys...)
	sort.Slice(cp, func(i, j int) bool {
		ai, aok := infos[cp[i]]
		bi, bok := infos[cp[j]]
		if aok && bok {
			if ai.Label != bi.Label {
				return ai.Label < bi.Label
			}
			return ai.Version < bi.Version
		}
		return cp[i] < cp[j]
	})
	return cp
}

// unvisitedFromDirects walks the graph from the direct roots and
// returns any package keys that weren't visited. Used so we can list
// orphans at the bottom of the tree output.
func unvisitedFromDirects(rows []row, edges map[string][]string, directs []string) []string {
	reachable := map[string]bool{}
	var walk func(key string)
	walk = func(key string) {
		if reachable[key] {
			return
		}
		reachable[key] = true
		for _, c := range edges[key] {
			walk(c)
		}
	}
	for _, d := range directs {
		walk(d)
	}

	var out []string
	for _, r := range rows {
		key := r.Pkg.PackageID + "@" + r.Pkg.Version
		if !reachable[key] {
			out = append(out, key)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

// pkgKey is exported for parity with edge keys: build the same
// "module@version" string that adapters emit in ParseResult.Edges.
func pkgKey(p ecosystems.PackageRef) string { return p.PackageID + "@" + p.Version }
