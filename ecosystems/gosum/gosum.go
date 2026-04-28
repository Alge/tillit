// Package gosum parses Go module checksum files (go.sum) into the canonical
// PackageRef shape consumed by the trust graph resolver.
package gosum

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// GoSum is the adapter for go.sum files. It implements ecosystems.Adapter.
type GoSum struct{}

func (GoSum) Ecosystem() string { return "go" }

func (GoSum) Name() string { return "go.sum" }

func (GoSum) CanParse(path string) bool {
	if path == "" {
		return false
	}
	return filepath.Base(path) == "go.sum"
}

// Parse reads a go.sum-formatted stream and returns one PackageRef per
// (module, version) pair. The /go.mod hash lines are folded into the
// matching module-zip entry (or seed a new entry if the zip line is absent).
func (GoSum) Parse(r io.Reader) (ecosystems.ParseResult, error) {
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
			warnings = append(warnings, fmt.Sprintf("line %d: expected 3 fields, got %d: %q", lineNo, len(fields), line))
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
		return ecosystems.ParseResult{}, fmt.Errorf("read go.sum: %w", err)
	}

	pkgs := make([]ecosystems.PackageRef, 0, len(order))
	for _, k := range order {
		pkgs = append(pkgs, *entries[k])
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}
