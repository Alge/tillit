package npmlock

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// YarnLock is the adapter for Yarn Classic (v1) `yarn.lock` files.
// The format is custom flat-text: blocks separated by blank lines,
// each starting with one or more quoted constraint headers and
// followed by indented `key value` pairs (`version`, `resolved`,
// `integrity`, `dependencies`, ...).
//
// Yarn Berry (v2+) uses a different YAML-shaped lockfile that we
// don't decode here; that format includes `__metadata` and uses
// `npm:` resolution prefixes.
type YarnLock struct{ npmCommon }

func (YarnLock) Name() string { return "yarn.lock" }

func (YarnLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "yarn.lock"
}

func (YarnLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()

	pkgs, warnings := parseYarnLock(f)
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}

func parseYarnLock(r io.Reader) ([]ecosystems.PackageRef, []string) {
	type block struct {
		headers   []string
		version   string
		integrity string
		resolved  string
	}

	var blocks []block
	var current *block

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if line == "" {
			if current != nil {
				blocks = append(blocks, *current)
				current = nil
			}
			continue
		}

		// Header lines (start a new block) sit at column 0 and end
		// with a colon. Indented lines (start with two spaces) are
		// the body of the current block.
		if line[0] != ' ' {
			if current != nil {
				blocks = append(blocks, *current)
			}
			current = &block{headers: parseYarnHeaders(line)}
			continue
		}

		if current == nil {
			continue
		}
		body := strings.TrimLeft(line, " ")
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		// Sub-keys (4-space indent) are the contents of `dependencies`/
		// `optionalDependencies` blocks; we ignore them.
		if strings.HasPrefix(line, "    ") {
			continue
		}
		switch {
		case strings.HasPrefix(body, "version "):
			current.version = unquote(strings.TrimPrefix(body, "version "))
		case strings.HasPrefix(body, "integrity "):
			current.integrity = unquote(strings.TrimPrefix(body, "integrity "))
		case strings.HasPrefix(body, "resolved "):
			current.resolved = unquote(strings.TrimPrefix(body, "resolved "))
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}

	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	for _, b := range blocks {
		if len(b.headers) == 0 {
			continue
		}
		// Use the first header to derive the package name. All
		// headers in a single block refer to the same resolved
		// package, so any one is fine.
		name, constraint := splitYarnHeader(b.headers[0])
		if name == "" {
			continue
		}
		if isYarnNonRegistryConstraint(constraint) {
			warnings = append(warnings, fmt.Sprintf("yarn.lock: skipping %s (%s)", name, classifyYarnNonRegistry(constraint)))
			continue
		}
		if b.version == "" {
			warnings = append(warnings, fmt.Sprintf("yarn.lock: skipping %s (no version)", name))
			continue
		}
		// `resolved` may also encode a non-registry source (git+, file:,
		// etc.) even when the constraint is a plain semver range.
		if isYarnNonRegistryConstraint(b.resolved) {
			warnings = append(warnings, fmt.Sprintf("yarn.lock: skipping %s %s (%s)", name, b.version, classifyYarnNonRegistry(b.resolved)))
			continue
		}
		k := key{Name: name, Version: b.version}
		if seen[k] {
			continue
		}
		seen[k] = true
		pkgs = append(pkgs, ecosystems.PackageRef{
			Ecosystem: "npm",
			PackageID: name,
			Version:   b.version,
			Hash:      b.integrity,
			Source:    b.resolved,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool {
		if pkgs[i].PackageID != pkgs[j].PackageID {
			return pkgs[i].PackageID < pkgs[j].PackageID
		}
		return pkgs[i].Version < pkgs[j].Version
	})
	return pkgs, warnings
}

// parseYarnHeaders splits a header line into its constituent
// quoted constraints. Yarn collapses identical resolved versions
// into one block whose header is `"a@^1.0", "a@^2.0":`.
func parseYarnHeaders(line string) []string {
	line = strings.TrimSuffix(strings.TrimSpace(line), ":")
	parts := strings.Split(line, ", ")
	for i, p := range parts {
		parts[i] = unquote(strings.TrimSpace(p))
	}
	return parts
}

// splitYarnHeader extracts (name, constraint) from a header like
// `left-pad@^1.3.0` or `@babel/code-frame@^7.10.4`. The split is on
// the LAST `@`, since scoped packages start with `@`.
func splitYarnHeader(h string) (name, constraint string) {
	if h == "" {
		return "", ""
	}
	// For scoped packages, ignore the leading '@' for the split.
	searchFrom := 0
	if strings.HasPrefix(h, "@") {
		searchFrom = 1
	}
	idx := strings.LastIndex(h[searchFrom:], "@")
	if idx < 0 {
		return h, ""
	}
	idx += searchFrom
	return h[:idx], h[idx+1:]
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// isYarnNonRegistryConstraint reports whether the given constraint
// or resolved-URL string points outside the npm registry (git URL,
// local file, protocol-prefixed package). Plain semver ranges and
// HTTPS registry tarball URLs return false.
func isYarnNonRegistryConstraint(s string) bool {
	for _, prefix := range []string{
		"git+", "git://", "git@",
		"file:",
		"http://", // unusual; HTTPS is the norm, plain HTTP is typically a custom mirror
		"link:", "portal:", "patch:", "exec:",
	} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func classifyYarnNonRegistry(s string) string {
	switch {
	case strings.HasPrefix(s, "git+"), strings.HasPrefix(s, "git://"), strings.HasPrefix(s, "git@"):
		return "git source"
	case strings.HasPrefix(s, "file:"):
		return "file source"
	case strings.HasPrefix(s, "link:"), strings.HasPrefix(s, "portal:"):
		return "workspace link"
	case strings.HasPrefix(s, "patch:"):
		return "patched package"
	case strings.HasPrefix(s, "exec:"):
		return "exec source"
	}
	return "non-registry source"
}
