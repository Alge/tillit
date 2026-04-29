package pypi

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/Alge/tillit/ecosystems"
)

// PdmLock is the adapter for `pdm.lock` files produced by PDM.
// Packages with a VCS revision, local path, or direct URL are
// warn-skipped; the remainder are vetable through PyPI.
type PdmLock struct{ pypiCommon }

func (PdmLock) Name() string { return "pdm.lock" }

func (PdmLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "pdm.lock"
}

type pdmLockFile struct {
	Packages []pdmLockPackage `toml:"package"`
}

type pdmLockPackage struct {
	Name     string `toml:"name"`
	Version  string `toml:"version"`
	Revision string `toml:"revision"`
	Path     string `toml:"path"`
	URL      string `toml:"url"`
	Editable bool   `toml:"editable"`
}

func (PdmLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc pdmLockFile
	if _, err := toml.Decode(string(body), &doc); err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("parse %s: %w", lockfilePath, err)
	}

	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	for _, p := range doc.Packages {
		if p.Name == "" || p.Version == "" {
			continue
		}
		if reason := pdmNonRegistry(p); reason != "" {
			warnings = append(warnings, fmt.Sprintf("pdm.lock: skipping %s %s (%s)", p.Name, p.Version, reason))
			continue
		}
		name := normalizePackageName(p.Name)
		k := key{Name: name, Version: p.Version}
		if seen[k] {
			continue
		}
		seen[k] = true
		pkgs = append(pkgs, ecosystems.PackageRef{
			Ecosystem: "pypi",
			PackageID: name,
			Version:   p.Version,
		})
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}

func pdmNonRegistry(p pdmLockPackage) string {
	switch {
	case p.Revision != "":
		return "VCS revision"
	case p.Path != "":
		return "local path source"
	case p.URL != "":
		return "direct URL source"
	case p.Editable:
		return "editable source"
	}
	return ""
}
