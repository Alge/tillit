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

// renderTree prints a cargo-tree-style hierarchy of all packages,
// rooted at each direct dependency. Diamond deps are shown in full at
// first appearance; subsequent appearances render as a leaf with `(*)`.
// Cycles (shouldn't exist in Go modules but we're defensive) are broken
// by the same visited set.
func renderTree(w io.Writer, rows []row, edges map[string][]string) {
	infos := indexRows(rows)
	resolvedKey := buildResolvedKeyMap(rows)
	edges = canonicaliseEdges(edges, infos, resolvedKey)
	directs := directRoots(rows)

	visited := map[string]bool{}
	for i, root := range directs {
		if i > 0 {
			fmt.Fprintln(w)
		}
		printRoot(w, root, edges, infos, visited)
	}

	// Anything we couldn't reach via the graph (orphans, or fallback
	// when edges is missing) gets listed flat at the bottom.
	if leftovers := unvisited(rows, visited); len(leftovers) > 0 {
		if len(directs) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "(packages not reached from any direct dependency)")
		}
		for _, key := range leftovers {
			info := infos[key]
			fmt.Fprintf(w, "%s %s [%s]%s\n",
				info.Label, info.Version, info.Status, directMarker(info.Direct))
		}
	}
	fmt.Fprintln(w, "\n(* = direct dependency)")
}

func printRoot(w io.Writer, key string, edges map[string][]string, infos map[string]nodeInfo, visited map[string]bool) {
	info := infos[key]
	visited[key] = true
	fmt.Fprintf(w, "%s %s [%s]%s\n",
		info.Label, info.Version, info.Status, directMarker(info.Direct))
	children := sortedChildren(edges[key], infos)
	for i, c := range children {
		printChild(w, c, "", i == len(children)-1, edges, infos, visited)
	}
}

func printChild(w io.Writer, key, prefix string, last bool, edges map[string][]string, infos map[string]nodeInfo, visited map[string]bool) {
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

	if visited[key] {
		fmt.Fprintf(w, "%s%s%s %s [%s] (*)\n",
			prefix, branch, info.Label, info.Version, info.Status)
		return
	}
	visited[key] = true
	fmt.Fprintf(w, "%s%s%s %s [%s]%s\n",
		prefix, branch, info.Label, info.Version, info.Status, directMarker(info.Direct))

	children := sortedChildren(edges[key], infos)
	for i, c := range children {
		printChild(w, c, nextPrefix, i == len(children)-1, edges, infos, visited)
	}
}

func directMarker(direct bool) string {
	if direct {
		return " *"
	}
	return ""
}

// indexRows builds a "module@version" -> nodeInfo map from the resolved
// rows. We also accept "module" alone (no @version) as an alias for
// rows whose package id matches — useful for graphs whose root node is
// the bare main-module path.
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
		// A "from" of just "mainmod" with no version is the project root;
		// we keep its key as-is since we use it via Direct=true rows in
		// directRoots() rather than walking from the synthetic root.
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

func unvisited(rows []row, visited map[string]bool) []string {
	var out []string
	for _, r := range rows {
		key := r.Pkg.PackageID + "@" + r.Pkg.Version
		if !visited[key] {
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
