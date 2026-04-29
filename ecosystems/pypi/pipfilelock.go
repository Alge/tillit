package pypi

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

// PipfileLock is the adapter for `Pipfile.lock` files produced by
// Pipenv. Both the `default` and `develop` sections are walked.
// Entries that pin to a registry version (`"version": "==X"`) are
// vetable; entries that point to a git/path/file source are
// warn-skipped.
type PipfileLock struct{ pypiCommon }

func (PipfileLock) Name() string { return "Pipfile.lock" }

func (PipfileLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "Pipfile.lock"
}

// pipfileLockFile mirrors the parts of Pipfile.lock we read. Entries
// in default/develop are kept as raw JSON so we can pick out only
// the fields we care about — Pipenv's schema accepts a wide range
// of optional keys per entry.
type pipfileLockFile struct {
	Default map[string]pipfileEntry `json:"default"`
	Develop map[string]pipfileEntry `json:"develop"`
}

type pipfileEntry struct {
	Version  string `json:"version"`
	Git      string `json:"git"`
	Path     string `json:"path"`
	File     string `json:"file"`
	Editable bool   `json:"editable"`
}

func (PipfileLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	var doc pipfileLockFile
	if err := json.Unmarshal(body, &doc); err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("parse %s: %w", lockfilePath, err)
	}

	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	// Walk default first, then develop. Iterate keys in sorted
	// order so output is deterministic — Go map iteration would
	// otherwise give a different order each run, which makes diffs
	// noisy and tests flaky.
	for _, section := range []map[string]pipfileEntry{doc.Default, doc.Develop} {
		names := make([]string, 0, len(section))
		for n := range section {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, rawName := range names {
			entry := section[rawName]
			if reason := pipfileNonRegistry(entry); reason != "" {
				warnings = append(warnings, fmt.Sprintf("Pipfile.lock: skipping %s (%s)", rawName, reason))
				continue
			}
			version := strings.TrimPrefix(entry.Version, "==")
			if version == "" {
				warnings = append(warnings, fmt.Sprintf("Pipfile.lock: skipping %s (no pinned version)", rawName))
				continue
			}
			name := normalizePackageName(rawName)
			k := key{Name: name, Version: version}
			if seen[k] {
				continue
			}
			seen[k] = true
			pkgs = append(pkgs, ecosystems.PackageRef{
				Ecosystem: "pypi",
				PackageID: name,
				Version:   version,
			})
		}
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}

func pipfileNonRegistry(e pipfileEntry) string {
	switch {
	case e.Git != "":
		return "git source"
	case e.Path != "":
		return "local path source"
	case e.File != "":
		return "direct file source"
	case e.Editable:
		return "editable source"
	}
	return ""
}
