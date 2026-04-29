package cargo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// defaultCargoURL is the canonical crates.io API root. Override with
// TILLIT_CARGO_URL when targeting a private mirror that exposes the
// same JSON shape under /api/v1/crates/<name>/<version>.
const defaultCargoURL = "https://crates.io"

// maxResponseBytes caps the per-version metadata read. crates.io's
// /api/v1/crates/<name>/<version> response is a single release —
// typically a few KB — so 1 MiB is a comfortable ceiling that
// protects against a malicious or misbehaving mirror.
const maxResponseBytes = 1 << 20

// ResolveVersion verifies that (packageID, version) exists on
// crates.io by fetching the per-version JSON and returns the canonical
// sha256 checksum the response advertises. crates.io publishes one
// `.crate` tarball per release, so a single call yields both
// existence and the content hash.
func (cargoCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	base := pickCargoURL()
	if base == "" {
		return nil, fmt.Errorf("no usable crates.io URL (set TILLIT_CARGO_URL)")
	}
	endpoint := fmt.Sprintf("%s/api/v1/crates/%s/%s",
		strings.TrimRight(base, "/"),
		url.PathEscape(packageID),
		url.PathEscape(version),
	)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("contact crates.io: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("crates.io: version not found")
	default:
		return nil, fmt.Errorf("crates.io returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read crates.io response: %w", err)
	}
	var doc cratesResponse
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse crates.io JSON: %w", err)
	}

	info := &ecosystems.VersionInfo{
		PackageID: packageID,
		Version:   version,
	}
	if doc.Version.Checksum != "" {
		info.Hash = doc.Version.Checksum
		info.HashAlgo = "sha256"
	}
	return info, nil
}

type cratesResponse struct {
	Version struct {
		Crate    string `json:"crate"`
		Num      string `json:"num"`
		Checksum string `json:"checksum"`
	} `json:"version"`
}

func pickCargoURL() string {
	if v := os.Getenv("TILLIT_CARGO_URL"); v != "" {
		return v
	}
	return defaultCargoURL
}
