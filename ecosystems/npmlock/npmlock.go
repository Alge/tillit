// Package npmlock parses npm package-lock.json files (lockfileVersion
// 2 and 3) into the canonical PackageRef shape consumed by the trust
// graph resolver. It also provides ResolveVersion against the npm
// registry's per-version metadata endpoint so sign-time checks work
// the same way they do for Go modules.
package npmlock

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// NpmLock is the package-lock.json adapter. Implements
// ecosystems.Adapter and ecosystems.GraphResolver (the latter is
// satisfied here without shelling out — the lockfile already
// contains all the edges we need).
type NpmLock struct{}

func (NpmLock) Ecosystem() string { return "npm" }
func (NpmLock) Name() string      { return "package-lock.json" }

func (NpmLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "package-lock.json"
}

// rawLockfile mirrors the slice of package-lock.json we care about.
// We only consume v2/v3; v1's nested "dependencies" tree is
// converted by `npm install` itself in modern toolchains so we don't
// support it here. Lockfile-version 1 lockfiles will yield an empty
// PackageRef list with a warning.
type rawLockfile struct {
	LockfileVersion int                    `json:"lockfileVersion"`
	Packages        map[string]rawLockNode `json:"packages"`
}

// rawLockNode is one entry under "packages". The empty-string key
// represents the project root; every other key is "node_modules/..."
// (or nested "node_modules/.../node_modules/..." for hoisted v3).
type rawLockNode struct {
	Name            string            `json:"name,omitempty"`
	Version         string            `json:"version,omitempty"`
	Resolved        string            `json:"resolved,omitempty"`
	Integrity       string            `json:"integrity,omitempty"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
	OptionalDeps    map[string]string `json:"optionalDependencies,omitempty"`
	PeerDeps        map[string]string `json:"peerDependencies,omitempty"`
	Dev             bool              `json:"dev,omitempty"`
	Optional        bool              `json:"optional,omitempty"`
}

func (NpmLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()

	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}
	var lock rawLockfile
	if err := json.Unmarshal(body, &lock); err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("parse %s: %w", lockfilePath, err)
	}

	if lock.LockfileVersion < 2 {
		return ecosystems.ParseResult{
			Warnings: []string{
				fmt.Sprintf("package-lock.json lockfileVersion=%d not supported (need v2 or v3); run `npm install` with npm 7+ to upgrade",
					lock.LockfileVersion),
			},
		}, nil
	}

	root, hasRoot := lock.Packages[""]
	directs := map[string]bool{}
	if hasRoot {
		for name := range root.Dependencies {
			directs[name] = true
		}
		for name := range root.DevDependencies {
			directs[name] = true
		}
		for name := range root.OptionalDeps {
			directs[name] = true
		}
	}

	pkgs := make([]ecosystems.PackageRef, 0, len(lock.Packages))
	edges := map[string][]string{}
	// Build a name → "name@version" lookup so we can rewrite the
	// dependency-name keys (which hold version constraints, not
	// resolved versions) onto resolved keys.
	nameToKey := map[string]string{}
	for path, node := range lock.Packages {
		if path == "" {
			continue
		}
		name := nodePackageName(path, node)
		if name == "" || node.Version == "" {
			continue
		}
		nameToKey[name] = name + "@" + node.Version
	}

	var warnings []string
	for path, node := range lock.Packages {
		if path == "" {
			continue
		}
		name := nodePackageName(path, node)
		if name == "" || node.Version == "" {
			warnings = append(warnings,
				fmt.Sprintf("package-lock entry %q has no resolvable name/version; skipped", path))
			continue
		}
		ref := ecosystems.PackageRef{
			Ecosystem: "npm",
			PackageID: name,
			Version:   node.Version,
			Direct:    directs[name],
			Hash:      node.Integrity,
			Source:    node.Resolved,
		}
		pkgs = append(pkgs, ref)

		nodeKey := name + "@" + node.Version
		var deps []string
		for dep := range node.Dependencies {
			if k, ok := nameToKey[dep]; ok {
				deps = append(deps, k)
			}
		}
		for dep := range node.OptionalDeps {
			if k, ok := nameToKey[dep]; ok {
				deps = append(deps, k)
			}
		}
		if len(deps) > 0 {
			sort.Strings(deps)
			edges[nodeKey] = deps
		}
	}

	// Stable output ordering — by package id, then version.
	sort.Slice(pkgs, func(i, j int) bool {
		if pkgs[i].PackageID != pkgs[j].PackageID {
			return pkgs[i].PackageID < pkgs[j].PackageID
		}
		return pkgs[i].Version < pkgs[j].Version
	})

	return ecosystems.ParseResult{
		Packages: pkgs,
		Edges:    edges,
		Warnings: warnings,
	}, nil
}

// nodePackageName extracts the package name from a packages-map key
// like "node_modules/lodash" or
// "node_modules/@scope/pkg/node_modules/inner". Hoisted v3 lockfiles
// nest these. The Name field on the node, when present, wins.
func nodePackageName(path string, node rawLockNode) string {
	if node.Name != "" {
		return node.Name
	}
	// Take the segment after the last "node_modules/".
	idx := strings.LastIndex(path, "node_modules/")
	if idx < 0 {
		return ""
	}
	rest := path[idx+len("node_modules/"):]
	// Scoped packages: "@scope/name" — keep both segments.
	if strings.HasPrefix(rest, "@") {
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) < 2 {
			return ""
		}
		return parts[0] + "/" + parts[1]
	}
	if i := strings.Index(rest, "/"); i >= 0 {
		return rest[:i]
	}
	return rest
}
