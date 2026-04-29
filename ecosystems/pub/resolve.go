package pub

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

// defaultPubURL is the canonical pub.dev API root. Override with
// TILLIT_PUB_URL when targeting a private mirror.
const defaultPubURL = "https://pub.dev"

// maxResponseBytes caps the per-version metadata read. pub.dev's
// /api/packages/<name>/versions/<ver> response is a single release —
// typically a few KB — so 1 MiB is a comfortable ceiling that
// protects against a malicious or misbehaving mirror.
const maxResponseBytes = 1 << 20

// ResolveVersion verifies that (packageID, version) exists on pub.dev
// by fetching the per-version JSON record. Pub publishes one tarball
// per release with `archive_sha256` set to its SHA-256 hash, so a
// single GET yields existence + the canonical content hash.
func (pubCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	base := pickPubURL()
	if base == "" {
		return nil, fmt.Errorf("no usable pub.dev URL (set TILLIT_PUB_URL)")
	}
	endpoint := fmt.Sprintf("%s/api/packages/%s/versions/%s",
		strings.TrimRight(base, "/"),
		url.PathEscape(packageID),
		url.PathEscape(version),
	)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("contact pub.dev: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("pub.dev: version not found")
	default:
		return nil, fmt.Errorf("pub.dev returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read pub.dev response: %w", err)
	}
	var doc pubResponse
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse pub.dev JSON: %w", err)
	}

	info := &ecosystems.VersionInfo{
		PackageID: packageID,
		Version:   version,
	}
	if doc.ArchiveSHA256 != "" {
		info.Hash = doc.ArchiveSHA256
		info.HashAlgo = "sha256"
	}
	return info, nil
}

type pubResponse struct {
	Version       string `json:"version"`
	ArchiveSHA256 string `json:"archive_sha256"`
	ArchiveURL    string `json:"archive_url"`
}

func pickPubURL() string {
	if v := os.Getenv("TILLIT_PUB_URL"); v != "" {
		return v
	}
	return defaultPubURL
}
