package hexpm

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

// defaultHexpmURL is the canonical hex.pm API root. Override with
// TILLIT_HEXPM_URL when targeting a private mirror that exposes the
// same JSON shape under /api/packages/<name>/releases/<version>.
const defaultHexpmURL = "https://hex.pm"

// ResolveVersion verifies that (packageID, version) exists on hex.pm
// by fetching the per-release JSON and returns the canonical sha256
// checksum the response advertises. Hex publishes one tarball per
// release, so a single call yields both existence and content hash.
func (hexpmCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	base := pickHexpmURL()
	if base == "" {
		return nil, fmt.Errorf("no usable hex.pm URL (set TILLIT_HEXPM_URL)")
	}
	endpoint := fmt.Sprintf("%s/api/packages/%s/releases/%s",
		strings.TrimRight(base, "/"),
		url.PathEscape(packageID),
		url.PathEscape(version),
	)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("contact hex.pm: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("hex.pm: version not found")
	default:
		return nil, fmt.Errorf("hex.pm returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read hex.pm response: %w", err)
	}
	var doc hexpmRelease
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse hex.pm JSON: %w", err)
	}

	info := &ecosystems.VersionInfo{
		PackageID: packageID,
		Version:   version,
	}
	if doc.Checksum != "" {
		info.Hash = doc.Checksum
		info.HashAlgo = "sha256"
	}
	return info, nil
}

type hexpmRelease struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

func pickHexpmURL() string {
	if v := os.Getenv("TILLIT_HEXPM_URL"); v != "" {
		return v
	}
	return defaultHexpmURL
}
