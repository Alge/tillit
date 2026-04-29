package pub

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/Alge/tillit/ecosystems"
)

// PubspecLock is the adapter for Dart / Flutter's `pubspec.lock`
// files. It walks the `packages` map and emits one PackageRef per
// `source: hosted` entry. Git, path, and SDK sources are warn-skipped
// — they live outside the pub.dev registry and have no archive hash
// the trust system can pin against.
type PubspecLock struct{ pubCommon }

func (PubspecLock) Name() string { return "pubspec.lock" }

func (PubspecLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "pubspec.lock"
}

type pubspecLockFile struct {
	Packages map[string]pubspecLockPackage `yaml:"packages"`
}

type pubspecLockPackage struct {
	Dependency  string                 `yaml:"dependency"`
	Description pubspecLockDescription `yaml:"description"`
	Source      string                 `yaml:"source"`
	Version     string                 `yaml:"version"`
}

// pubspecLockDescription accepts both shapes pub uses: a mapping for
// hosted/git/path packages, and a bare scalar string for SDK packages
// (where the description is just the SDK name like "flutter"). The
// custom UnmarshalYAML lets us decode either without crashing.
type pubspecLockDescription struct {
	Name   string
	URL    string
	SHA256 string
}

func (d *pubspecLockDescription) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		// e.g. `description: flutter` for SDK-source packages.
		d.Name = value.Value
		return nil
	case yaml.MappingNode:
		// Standard mapping with name/url/sha256 (and other fields we
		// don't care about for vetting).
		var raw struct {
			Name   string `yaml:"name"`
			URL    string `yaml:"url"`
			SHA256 string `yaml:"sha256"`
		}
		if err := value.Decode(&raw); err != nil {
			return err
		}
		d.Name = raw.Name
		d.URL = raw.URL
		d.SHA256 = raw.SHA256
		return nil
	}
	return fmt.Errorf("pubspec.lock description: unsupported node kind %d", value.Kind)
}

func (PubspecLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc pubspecLockFile
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("parse %s: %w", lockfilePath, err)
	}

	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	// Sorted iteration so output ordering is stable across runs —
	// YAML maps don't preserve insertion order in yaml.v3 either.
	names := make([]string, 0, len(doc.Packages))
	for n := range doc.Packages {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, mapKey := range names {
		entry := doc.Packages[mapKey]
		if entry.Version == "" {
			warnings = append(warnings, fmt.Sprintf("pubspec.lock: skipping %s (no version)", mapKey))
			continue
		}
		switch entry.Source {
		case "hosted":
			// fall through — vetable
		case "git":
			warnings = append(warnings, fmt.Sprintf("pubspec.lock: skipping %s %s (git source)", mapKey, entry.Version))
			continue
		case "path":
			warnings = append(warnings, fmt.Sprintf("pubspec.lock: skipping %s %s (local path source)", mapKey, entry.Version))
			continue
		case "sdk":
			warnings = append(warnings, fmt.Sprintf("pubspec.lock: skipping %s %s (SDK source)", mapKey, entry.Version))
			continue
		default:
			warnings = append(warnings, fmt.Sprintf("pubspec.lock: skipping %s %s (unrecognised source %q)", mapKey, entry.Version, entry.Source))
			continue
		}

		// Prefer the explicit description.name; fall back to the map
		// key when the description omits it (older lockfiles).
		name := entry.Description.Name
		if name == "" {
			name = mapKey
		}
		k := key{Name: name, Version: entry.Version}
		if seen[k] {
			continue
		}
		seen[k] = true
		pkgs = append(pkgs, ecosystems.PackageRef{
			Ecosystem: "pub",
			PackageID: name,
			Version:   entry.Version,
			Hash:      entry.Description.SHA256,
		})
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}
