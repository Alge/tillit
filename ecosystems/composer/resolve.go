package composer

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

// defaultPackagistURL is the canonical Packagist v2 metadata root.
// Override with TILLIT_COMPOSER_URL when targeting a private mirror
// that exposes the same JSON shape under /p2/<vendor>/<name>.json.
const defaultPackagistURL = "https://repo.packagist.org"

// ResolveVersion verifies that (packageID, version) exists on
// Packagist by fetching the package metadata document and scanning
// its versions list. The Packagist v2 API returns metadata for ALL
// versions of a package in one shot, so we have to find the
// requested version inside the array.
//
// The hash returned is whatever `dist.shasum` Packagist publishes;
// historically that's a SHA-1 of the zip dist (not SHA-256), so
// HashAlgo is reported as "sha1". Composer doesn't yet emit SHA-256
// in metadata, but if it ever does we can pick that up later.
func (composerCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	base := pickComposerURL()
	if base == "" {
		return nil, fmt.Errorf("no usable packagist URL (set TILLIT_COMPOSER_URL)")
	}
	endpoint := fmt.Sprintf("%s/p2/%s.json",
		strings.TrimRight(base, "/"),
		escapePackageName(packageID),
	)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("contact packagist: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("packagist: package not found")
	default:
		return nil, fmt.Errorf("packagist returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read packagist response: %w", err)
	}

	var doc packagistResponse
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse packagist JSON: %w", err)
	}

	versions, ok := doc.Packages[packageID]
	if !ok {
		// Packagist sometimes returns a single key whose case differs
		// — vendor names are case-insensitive. Try a case-insensitive
		// lookup so resolving doesn't fail on capitalisation drift.
		for k, v := range doc.Packages {
			if strings.EqualFold(k, packageID) {
				versions = v
				ok = true
				break
			}
		}
	}
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("packagist: package not found")
	}

	want := strings.TrimPrefix(version, "v")
	for _, v := range versions {
		got := strings.TrimPrefix(v.Version, "v")
		if got != want {
			continue
		}
		info := &ecosystems.VersionInfo{
			PackageID: packageID,
			Version:   version,
		}
		if v.Dist != nil && v.Dist.Shasum != "" {
			info.Hash = v.Dist.Shasum
			info.HashAlgo = "sha1"
		}
		return info, nil
	}
	return nil, fmt.Errorf("packagist: version not found")
}

type packagistResponse struct {
	Packages map[string][]packagistVersion `json:"packages"`
}

type packagistVersion struct {
	Name    string             `json:"name"`
	Version string             `json:"version"`
	Dist    *composerLockBlock `json:"dist,omitempty"`
}

// escapePackageName URL-escapes the path segments of a Composer
// package name (which is always `vendor/package`). PathEscape would
// turn the slash into %2F, which Packagist doesn't accept here, so
// we escape each segment individually and rejoin.
func escapePackageName(p string) string {
	parts := strings.SplitN(p, "/", 2)
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

func pickComposerURL() string {
	if v := os.Getenv("TILLIT_COMPOSER_URL"); v != "" {
		return v
	}
	return defaultPackagistURL
}
