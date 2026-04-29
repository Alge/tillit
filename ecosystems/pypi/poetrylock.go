package pypi

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/Alge/tillit/ecosystems"
)

// PoetryLock is the adapter for `poetry.lock` files produced by
// Poetry. Packages with a non-PyPI source (git, file, directory,
// direct URL) are warn-skipped; absent or "legacy" (private index)
// sources are vetable.
type PoetryLock struct{ pypiCommon }

func (PoetryLock) Name() string { return "poetry.lock" }

func (PoetryLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "poetry.lock"
}

type poetryLockFile struct {
	Packages []poetryLockPackage `toml:"package"`
}

type poetryLockPackage struct {
	Name    string           `toml:"name"`
	Version string           `toml:"version"`
	Source  poetryLockSource `toml:"source"`
}

// poetryLockSource captures Poetry's `source` table. Type drives the
// vetability decision; url/reference/resolved_reference are kept on
// the struct only because TOML decoding fails silently when an
// unknown subkey is encountered? No — BurntSushi/toml is lenient.
// Kept anyway to make the schema obvious.
type poetryLockSource struct {
	Type      string `toml:"type"`
	URL       string `toml:"url"`
	Reference string `toml:"reference"`
}

func (PoetryLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc poetryLockFile
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
		if reason := poetryNonRegistrySource(p.Source); reason != "" {
			warnings = append(warnings, fmt.Sprintf("poetry.lock: skipping %s %s (%s)", p.Name, p.Version, reason))
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

// poetryNonRegistrySource returns a reason string when the source is
// something other than the implicit PyPI default or a "legacy"
// (PEP 503 simple-index) private mirror. Empty when vetable.
func poetryNonRegistrySource(s poetryLockSource) string {
	switch s.Type {
	case "", "legacy":
		return ""
	case "git":
		return "git source"
	case "file":
		return "file source"
	case "directory":
		return "directory source"
	case "url":
		return "direct URL source"
	default:
		return fmt.Sprintf("unsupported source type %q", s.Type)
	}
}
