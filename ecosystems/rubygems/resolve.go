package rubygems

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

// defaultRubygemsURL is the canonical rubygems.org API root. Override
// with TILLIT_RUBYGEMS_URL when targeting a private mirror (e.g.
// Gemfury, packagecloud) that exposes the same endpoints.
const defaultRubygemsURL = "https://rubygems.org"

// ResolveVersion verifies that (packageID, version) exists on
// rubygems.org by fetching the per-gem versions list and scanning
// for the requested number. The list endpoint returns metadata for
// every released version of the gem, so we must walk the array to
// find the one we want.
func (rubygemsCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	base := pickRubygemsURL()
	if base == "" {
		return nil, fmt.Errorf("no usable rubygems.org URL (set TILLIT_RUBYGEMS_URL)")
	}
	endpoint := fmt.Sprintf("%s/api/v1/versions/%s.json",
		strings.TrimRight(base, "/"),
		url.PathEscape(packageID),
	)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("contact rubygems.org: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("rubygems.org: gem not found")
	default:
		return nil, fmt.Errorf("rubygems.org returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read rubygems.org response: %w", err)
	}
	var versions []rubygemsVersion
	if err := json.Unmarshal(body, &versions); err != nil {
		return nil, fmt.Errorf("parse rubygems.org JSON: %w", err)
	}

	for _, v := range versions {
		if v.Number != version {
			continue
		}
		info := &ecosystems.VersionInfo{
			PackageID: packageID,
			Version:   version,
		}
		if v.SHA != "" {
			info.Hash = v.SHA
			info.HashAlgo = "sha256"
		}
		return info, nil
	}
	return nil, fmt.Errorf("rubygems.org: version not found")
}

type rubygemsVersion struct {
	Number string `json:"number"`
	SHA    string `json:"sha"`
}

func pickRubygemsURL() string {
	if v := os.Getenv("TILLIT_RUBYGEMS_URL"); v != "" {
		return v
	}
	return defaultRubygemsURL
}
