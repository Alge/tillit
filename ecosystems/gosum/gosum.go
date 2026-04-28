// Package gosum parses Go module checksum files (go.sum) into the canonical
// PackageRef shape consumed by the trust graph resolver.
//
// The adapter also reads the sibling go.mod (when present) to distinguish
// direct dependencies from transitive ones — go.sum itself is a flat
// checksum list with no direct/indirect distinction.
package gosum

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// GoSum is the adapter for go.sum files. It implements ecosystems.Adapter.
type GoSum struct{}

func (GoSum) Ecosystem() string { return "go" }

func (GoSum) Name() string { return "go.sum" }

func (GoSum) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "go.sum"
}

// Parse reads go.sum (required) and go.mod (optional, sibling file) from
// fsys. It returns one PackageRef per (module, version) pair from go.sum,
// with Direct=true for modules listed without `// indirect` in go.mod.
func (GoSum) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	sumFile, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer sumFile.Close()

	pkgs, sumWarnings, err := parseGoSum(sumFile)
	if err != nil {
		return ecosystems.ParseResult{}, err
	}

	// Try the sibling go.mod for direct/indirect labelling. Missing or
	// unreadable go.mod is non-fatal; everything stays Direct=false.
	modPath := path.Join(path.Dir(lockfilePath), "go.mod")
	directs, modWarnings := readDirectModules(fsys, modPath)
	for i := range pkgs {
		if directs[pkgs[i].PackageID] {
			pkgs[i].Direct = true
		}
	}

	return ecosystems.ParseResult{
		Packages: pkgs,
		Warnings: append(sumWarnings, modWarnings...),
	}, nil
}

func parseGoSum(r io.Reader) ([]ecosystems.PackageRef, []string, error) {
	type key struct{ Module, Version string }
	entries := map[key]*ecosystems.PackageRef{}
	order := []key{}
	var warnings []string

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			warnings = append(warnings, fmt.Sprintf("go.sum line %d: expected 3 fields, got %d: %q", lineNo, len(fields), line))
			continue
		}

		module, versionField, hash := fields[0], fields[1], fields[2]
		isGoModLine := strings.HasSuffix(versionField, "/go.mod")
		version := strings.TrimSuffix(versionField, "/go.mod")

		k := key{Module: module, Version: version}
		entry, exists := entries[k]
		if !exists {
			entry = &ecosystems.PackageRef{
				Ecosystem: "go",
				PackageID: module,
				Version:   version,
			}
			entries[k] = entry
			order = append(order, k)
		}
		// Module-zip line wins for the artifact hash; /go.mod lines only
		// fill the slot when no zip line has set it yet.
		if !isGoModLine || entry.Hash == "" {
			entry.Hash = hash
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read go.sum: %w", err)
	}

	pkgs := make([]ecosystems.PackageRef, 0, len(order))
	for _, k := range order {
		pkgs = append(pkgs, *entries[k])
	}
	return pkgs, warnings, nil
}

// readDirectModules opens the given go.mod path and returns a set of module
// paths declared as direct dependencies (i.e. on a require line that does
// not end with `// indirect`). Errors return an empty set plus a warning so
// the caller can proceed with all-indirect labelling.
func readDirectModules(fsys fs.FS, modPath string) (map[string]bool, []string) {
	directs := map[string]bool{}
	modFile, err := fsys.Open(modPath)
	if err != nil {
		return directs, []string{fmt.Sprintf("go.mod not readable, all dependencies labelled indirect: %v", err)}
	}
	defer modFile.Close()

	var warnings []string
	scanner := bufio.NewScanner(modFile)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	inRequireBlock := false
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// Multi-line require ( ... ) block.
		if !inRequireBlock {
			if line == "require (" {
				inRequireBlock = true
				continue
			}
			// Single-line: `require module v1.2.3 [// indirect]`
			if rest, ok := strings.CutPrefix(line, "require "); ok {
				if mod, direct := parseRequireLine(rest); mod != "" && direct {
					directs[mod] = true
				}
				continue
			}
			continue
		}

		if line == ")" {
			inRequireBlock = false
			continue
		}
		if mod, direct := parseRequireLine(line); mod != "" && direct {
			directs[mod] = true
		}
	}
	if err := scanner.Err(); err != nil {
		warnings = append(warnings, fmt.Sprintf("go.mod read error: %v", err))
	}
	return directs, warnings
}

// parseRequireLine returns the module path and whether the line declares a
// direct dependency. A line like `github.com/foo/bar v1.2.3 // indirect` is
// indirect; without the comment it's direct.
func parseRequireLine(line string) (module string, direct bool) {
	// Strip trailing `// indirect` (anywhere in the comment is treated as
	// indirect — go.mod normalises to `// indirect` exactly, but be lenient).
	commentIdx := strings.Index(line, "//")
	indirect := false
	if commentIdx >= 0 {
		comment := line[commentIdx+2:]
		if strings.Contains(comment, "indirect") {
			indirect = true
		}
		line = strings.TrimSpace(line[:commentIdx])
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", false
	}
	return fields[0], !indirect
}
