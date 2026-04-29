package hexpm

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/Alge/tillit/ecosystems"
)

// GleamManifest is the adapter for Gleam's `manifest.toml` files.
// Gleam packages publish to hex.pm — the same registry Mix uses —
// so this adapter shares Ecosystem() == "hexpm" with MixLock.
//
// The manifest is a TOML document whose top-level `packages` array
// holds inline tables. Entries with `source = "hex"` are vetable;
// `git` and `local` sources are warn-skipped.
type GleamManifest struct{ hexpmCommon }

func (GleamManifest) Name() string { return "manifest.toml" }

func (GleamManifest) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "manifest.toml"
}

type gleamManifestFile struct {
	Packages []gleamManifestPackage `toml:"packages"`
}

type gleamManifestPackage struct {
	Name          string `toml:"name"`
	Version       string `toml:"version"`
	Source        string `toml:"source"`
	OuterChecksum string `toml:"outer_checksum"`
}

func (GleamManifest) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc gleamManifestFile
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
		switch p.Source {
		case "hex", "":
			// `hex` is the explicit form modern manifests use; an
			// empty source is treated as the default (hex.pm) so
			// older or hand-edited manifests still parse.
		case "git":
			warnings = append(warnings, fmt.Sprintf("manifest.toml: skipping %s %s (git source)", p.Name, p.Version))
			continue
		case "local":
			warnings = append(warnings, fmt.Sprintf("manifest.toml: skipping %s %s (local path source)", p.Name, p.Version))
			continue
		default:
			warnings = append(warnings, fmt.Sprintf("manifest.toml: skipping %s %s (unrecognised source %q)", p.Name, p.Version, p.Source))
			continue
		}

		k := key{Name: p.Name, Version: p.Version}
		if seen[k] {
			continue
		}
		seen[k] = true
		pkgs = append(pkgs, ecosystems.PackageRef{
			Ecosystem: "hexpm",
			PackageID: p.Name,
			Version:   p.Version,
			Hash:      p.OuterChecksum,
		})
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}
