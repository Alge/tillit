package cargo

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/Alge/tillit/ecosystems"
)

// CargoLock is the adapter for Rust's `Cargo.lock` files. It reads
// the TOML `[[package]]` array and returns the registry-sourced
// entries; git, path, and workspace-member entries are filtered out
// (workspace members silently, others with a warning).
type CargoLock struct{ cargoCommon }

func (CargoLock) Name() string { return "Cargo.lock" }

// CanParse matches Cargo's canonical lockfile name. Cargo only
// recognises `Cargo.lock` (capital C) — lowercase `cargo.lock` is
// not produced by `cargo` and we deliberately don't accept it,
// because picking up a stray file would surprise users.
func (CargoLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "Cargo.lock"
}

type cargoLockFile struct {
	Packages []cargoLockPackage `toml:"package"`
}

type cargoLockPackage struct {
	Name     string `toml:"name"`
	Version  string `toml:"version"`
	Source   string `toml:"source"`
	Checksum string `toml:"checksum"`
}

func (CargoLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc cargoLockFile
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
		switch {
		case p.Source == "":
			// Workspace member or local crate — skip silently.
			// Surfacing these would create noise on every project
			// since the project's own crates always show up here.
			continue
		case strings.HasPrefix(p.Source, "registry+"):
			// fall through — vetable
		case strings.HasPrefix(p.Source, "git+"):
			warnings = append(warnings, fmt.Sprintf("Cargo.lock: skipping %s %s (git source)", p.Name, p.Version))
			continue
		case strings.HasPrefix(p.Source, "path+"):
			warnings = append(warnings, fmt.Sprintf("Cargo.lock: skipping %s %s (local path source)", p.Name, p.Version))
			continue
		default:
			warnings = append(warnings, fmt.Sprintf("Cargo.lock: skipping %s %s (unrecognised source %q)", p.Name, p.Version, p.Source))
			continue
		}

		k := key{Name: p.Name, Version: p.Version}
		if seen[k] {
			continue
		}
		seen[k] = true
		pkgs = append(pkgs, ecosystems.PackageRef{
			Ecosystem: "cargo",
			PackageID: p.Name,
			Version:   p.Version,
			Hash:      p.Checksum,
		})
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}
