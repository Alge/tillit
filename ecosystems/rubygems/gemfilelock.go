package rubygems

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/Alge/tillit/ecosystems"
)

// GemfileLock is the adapter for Bundler's `Gemfile.lock` files. It
// scans the line-oriented sections (`GEM`, `GIT`, `PATH`, ...) and
// emits one PackageRef per gem listed under a `GEM` block's `specs:`
// list. Packages from `GIT` or `PATH` blocks are warn-skipped — they
// don't have a registry-published archive hash to pin against.
type GemfileLock struct{ rubygemsCommon }

func (GemfileLock) Name() string { return "Gemfile.lock" }

// CanParse matches Bundler's canonical lockfile name. Bundler emits
// `Gemfile.lock` with a capital G; we don't accept lowercase
// variants since picking up a stray file would surprise users.
func (GemfileLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "Gemfile.lock"
}

// gemSpecRE matches a top-level spec line inside a section's
// `specs:` block: four leading spaces, then `name (version)`. The
// version capture includes the platform suffix (e.g.
// `1.15.0-x86_64-linux`) so callers can decide whether to keep
// platform-tagged builds as distinct entries — we do.
//
// Sub-dependencies under each spec use six leading spaces, so the
// `^    ` anchor (exactly four spaces) excludes them.
var gemSpecRE = regexp.MustCompile(`^    ([A-Za-z0-9._-]+) \(([^)]+)\)\s*$`)

func (GemfileLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()

	pkgs, warnings := parseGemfileLock(f)
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}

func parseGemfileLock(r io.Reader) ([]ecosystems.PackageRef, []string) {
	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)

	// Section state. Bundler's section names are top-level
	// uppercase identifiers; the section continues until the next
	// blank line or another section header.
	const (
		sectionNone = iota
		sectionGem
		sectionGit
		sectionPath
		sectionOther // PLATFORMS / DEPENDENCIES / RUBY VERSION / BUNDLED WITH / etc — ignored
	)
	section := sectionNone
	inSpecs := false

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Blank line ends the current section.
			section = sectionNone
			inSpecs = false
			continue
		}

		// Section headers start at column 0.
		if line[0] != ' ' {
			switch line {
			case "GEM":
				section = sectionGem
			case "GIT":
				section = sectionGit
			case "PATH":
				section = sectionPath
			default:
				section = sectionOther
			}
			inSpecs = false
			continue
		}

		// Inside a section. The `specs:` marker (two-space indent)
		// switches us into spec-line mode.
		if line == "  specs:" {
			inSpecs = true
			continue
		}
		if !inSpecs {
			continue
		}

		m := gemSpecRE.FindStringSubmatch(line)
		if m == nil {
			// Likely a sub-dependency line (six-space indent) or
			// something we don't recognise — skip without warning.
			continue
		}
		name, version := m[1], m[2]
		switch section {
		case sectionGem:
			k := key{Name: name, Version: version}
			if seen[k] {
				continue
			}
			seen[k] = true
			pkgs = append(pkgs, ecosystems.PackageRef{
				Ecosystem: "rubygems",
				PackageID: name,
				Version:   version,
			})
		case sectionGit:
			warnings = append(warnings, fmt.Sprintf("Gemfile.lock: skipping %s %s (git source)", name, version))
		case sectionPath:
			warnings = append(warnings, fmt.Sprintf("Gemfile.lock: skipping %s %s (local path source)", name, version))
		}
	}
	return pkgs, warnings
}
