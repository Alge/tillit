package nuget

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

// PackagesLock is the adapter for .NET's `packages.lock.json` files.
// It walks every target-framework block and emits one PackageRef per
// (name, version) pair, deduplicating across frameworks. Project-type
// entries (workspace references to local .csproj files) are skipped
// silently — they're a normal part of multi-project solutions and
// shouldn't produce noise.
type PackagesLock struct{ nugetCommon }

func (PackagesLock) Name() string { return "packages.lock.json" }

func (PackagesLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "packages.lock.json"
}

type packagesLockFile struct {
	Version      int                                     `json:"version"`
	Dependencies map[string]map[string]packagesLockEntry `json:"dependencies"`
}

type packagesLockEntry struct {
	Type        string `json:"type"`
	Resolved    string `json:"resolved"`
	ContentHash string `json:"contentHash"`
}

func (PackagesLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc packagesLockFile
	if err := json.Unmarshal(body, &doc); err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("parse %s: %w", lockfilePath, err)
	}

	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	// Iterate frameworks and packages in sorted order so the output
	// is stable across runs — Go's map iteration would otherwise give
	// a different order each time and make diffs noisy.
	frameworks := make([]string, 0, len(doc.Dependencies))
	for fw := range doc.Dependencies {
		frameworks = append(frameworks, fw)
	}
	sort.Strings(frameworks)

	for _, fw := range frameworks {
		entries := doc.Dependencies[fw]
		names := make([]string, 0, len(entries))
		for n := range entries {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, rawName := range names {
			entry := entries[rawName]
			if entry.Type == "Project" {
				// Workspace reference to a local .csproj — skip
				// silently; these are normal in multi-project
				// solutions and aren't backed by a registry artifact.
				continue
			}
			if entry.Resolved == "" {
				warnings = append(warnings, fmt.Sprintf("packages.lock.json: skipping %s (no resolved version)", rawName))
				continue
			}
			name := strings.ToLower(rawName)
			k := key{Name: name, Version: entry.Resolved}
			if seen[k] {
				continue
			}
			seen[k] = true
			pkgs = append(pkgs, ecosystems.PackageRef{
				Ecosystem: "nuget",
				PackageID: name,
				Version:   entry.Resolved,
				Hash:      entry.ContentHash,
			})
		}
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}
