package pypi

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/Alge/tillit/ecosystems"
)

// UvLock is the adapter for `uv.lock` files produced by Astral's uv
// package manager. Only packages whose source is a registry (PyPI or
// a private mirror exposing the same shape) are vetable; git, path,
// editable, and virtual sources are warn-skipped.
type UvLock struct{ pypiCommon }

func (UvLock) Name() string { return "uv.lock" }

func (UvLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "uv.lock"
}

// uvLockFile mirrors the parts of uv.lock we read. Fields we don't
// care about are dropped during decoding.
type uvLockFile struct {
	Packages []uvLockPackage `toml:"package"`
}

type uvLockPackage struct {
	Name    string       `toml:"name"`
	Version string       `toml:"version"`
	Source  uvLockSource `toml:"source"`
}

// uvLockSource captures the variant tags uv emits. Exactly one of
// these fields is set per package; the rest stay zero.
type uvLockSource struct {
	Registry  string `toml:"registry"`
	URL       string `toml:"url"`
	Git       string `toml:"git"`
	Path      string `toml:"path"`
	Editable  string `toml:"editable"`
	Virtual   string `toml:"virtual"`
	Directory string `toml:"directory"`
}

func (UvLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc uvLockFile
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
		if reason := uvNonRegistrySource(p.Source); reason != "" {
			warnings = append(warnings, fmt.Sprintf("uv.lock: skipping %s %s (%s)", p.Name, p.Version, reason))
			continue
		}
		if p.Source.Registry == "" {
			// No source block at all is unusual but not strictly
			// disallowed in older uv versions; treat as registry by
			// default rather than dropping silently.
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

// uvNonRegistrySource returns a short reason string when the package
// source is something other than a registry (PyPI / mirror); empty
// when the package is vetable through the PyPI JSON API.
func uvNonRegistrySource(s uvLockSource) string {
	switch {
	case s.Git != "":
		return "git source"
	case s.URL != "":
		return "direct URL source"
	case s.Path != "":
		return "local path source"
	case s.Editable != "":
		return "editable source"
	case s.Virtual != "":
		return "virtual workspace source"
	case s.Directory != "":
		return "directory source"
	}
	return ""
}
