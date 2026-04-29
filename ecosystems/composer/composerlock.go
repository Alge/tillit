package composer

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// ComposerLock is the adapter for PHP's `composer.lock` files. It
// walks both the prod (`packages`) and dev (`packages-dev`) arrays,
// emits one PackageRef per registry-published entry, and warn-skips
// entries that are dev-branch refs or VCS-only (no `dist` block).
type ComposerLock struct{ composerCommon }

func (ComposerLock) Name() string { return "composer.lock" }

func (ComposerLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "composer.lock"
}

type composerLockFile struct {
	Packages    []composerLockPackage `json:"packages"`
	PackagesDev []composerLockPackage `json:"packages-dev"`
}

type composerLockPackage struct {
	Name    string             `json:"name"`
	Version string             `json:"version"`
	Source  *composerLockBlock `json:"source,omitempty"`
	Dist    *composerLockBlock `json:"dist,omitempty"`
}

type composerLockBlock struct {
	URL    string `json:"url,omitempty"`
	Shasum string `json:"shasum,omitempty"`
}

func (ComposerLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc composerLockFile
	if err := json.Unmarshal(body, &doc); err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("parse %s: %w", lockfilePath, err)
	}

	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	process := func(entries []composerLockPackage) {
		for _, p := range entries {
			if p.Name == "" || p.Version == "" {
				continue
			}
			if strings.HasPrefix(p.Version, "dev-") {
				warnings = append(warnings, fmt.Sprintf("composer.lock: skipping %s %s (dev-branch ref, not a release)", p.Name, p.Version))
				continue
			}
			if p.Dist == nil || p.Dist.Shasum == "" && p.Dist.URL == "" {
				warnings = append(warnings, fmt.Sprintf("composer.lock: skipping %s %s (no dist block — VCS-only install)", p.Name, p.Version))
				continue
			}
			version := strings.TrimPrefix(p.Version, "v")
			k := key{Name: p.Name, Version: version}
			if seen[k] {
				continue
			}
			seen[k] = true
			hash := ""
			if p.Dist != nil {
				hash = p.Dist.Shasum
			}
			pkgs = append(pkgs, ecosystems.PackageRef{
				Ecosystem: "composer",
				PackageID: p.Name,
				Version:   version,
				Hash:      hash,
			})
		}
	}
	process(doc.Packages)
	process(doc.PackagesDev)
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}
