package gosum

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// ResolveVersion confirms that (packageID, version) exists by
// shelling out to `go mod download -json`. The command is run from a
// throwaway temporary directory containing a stub go.mod, so the
// caller doesn't need to be inside a Go module — sign and friends
// can resolve any version reference from anywhere.
//
// On success the returned VersionInfo carries the sumdb-validated
// h1: hash of the module zip, which the Go toolchain has already
// reconciled with the checksum database. Callers can compare it
// against an entry in go.sum to detect local-go.sum tampering.
func (GoSum) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	tmp, err := os.MkdirTemp("", "tillit-resolve-")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	if err := os.WriteFile(filepath.Join(tmp, "go.mod"),
		[]byte("module tillit-resolve\n\ngo 1.21\n"), 0o644); err != nil {
		return nil, fmt.Errorf("write stub go.mod: %w", err)
	}

	cmd := exec.Command("go", "mod", "download", "-json", packageID+"@"+version)
	cmd.Dir = tmp
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	// `go mod download` exits non-zero on a not-found version too,
	// but it still writes a JSON response to stdout with an Error
	// field — that's where we get the structured failure message.
	// Fall back to stderr if there's no parsable JSON.
	if stdout.Len() == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("go mod download failed: %v: %s", runErr, strings.TrimSpace(stderr.String()))
		}
		return nil, errors.New("go mod download produced no output")
	}
	return parseModDownload(stdout.Bytes(), packageID, version)
}

// parseModDownload extracts a VersionInfo (or an error) from a
// `go mod download -json` response body. Split out from
// ResolveVersion for testability.
func parseModDownload(raw []byte, packageID, version string) (*ecosystems.VersionInfo, error) {
	var resp struct {
		Path    string
		Version string
		Sum     string
		Error   string
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse go mod download output: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("go mod download: %s", resp.Error)
	}
	info := &ecosystems.VersionInfo{
		PackageID: packageID,
		Version:   version,
	}
	if resp.Sum != "" {
		info.Hash = resp.Sum
		// Go's Sum field is always prefixed with the algorithm tag
		// ("h1:" today) — split it so callers compare apples to
		// apples with go.sum entries.
		if i := strings.Index(resp.Sum, ":"); i > 0 {
			info.HashAlgo = resp.Sum[:i]
		}
	}
	return info, nil
}
