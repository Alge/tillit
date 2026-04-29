package nuget

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// defaultNugetURL is the canonical nuget.org v3 flat container root.
// Override with TILLIT_NUGET_URL when targeting a private feed that
// exposes the same paths.
const defaultNugetURL = "https://api.nuget.org"

// ResolveVersion verifies that (packageID, version) exists on
// nuget.org by fetching the .nupkg.sha512 sibling file the flat
// container publishes for every release. The endpoint is small
// (the file contains exactly the base64-encoded SHA-512 of the
// .nupkg) and a 404 means the version doesn't exist.
//
// NuGet IDs are case-insensitive but the v3 flat container requires
// lowercase in the path. We normalise both the ID and the version
// the same way the official client does.
func (nugetCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	base := pickNugetURL()
	if base == "" {
		return nil, fmt.Errorf("no usable nuget URL (set TILLIT_NUGET_URL)")
	}
	id := strings.ToLower(packageID)
	ver := strings.ToLower(version)
	endpoint := fmt.Sprintf("%s/v3-flatcontainer/%s/%s/%s.%s.nupkg.sha512",
		strings.TrimRight(base, "/"),
		url.PathEscape(id),
		url.PathEscape(ver),
		url.PathEscape(id),
		url.PathEscape(ver),
	)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("contact nuget: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("nuget: version not found")
	default:
		return nil, fmt.Errorf("nuget returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("read nuget response: %w", err)
	}

	info := &ecosystems.VersionInfo{
		PackageID: packageID,
		Version:   version,
	}
	hash := strings.TrimSpace(string(body))
	if hash != "" {
		info.Hash = hash
		info.HashAlgo = "sha512"
	}
	return info, nil
}

func pickNugetURL() string {
	if v := os.Getenv("TILLIT_NUGET_URL"); v != "" {
		return v
	}
	return defaultNugetURL
}
