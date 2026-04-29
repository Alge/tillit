package pypi

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

// defaultPyPIURL is the canonical JSON API endpoint hosted by PyPI.
// Override with TILLIT_PYPI_URL when targeting a private mirror that
// exposes the same JSON shape.
const defaultPyPIURL = "https://pypi.org"

// ResolveVersion verifies that (packageID, version) exists on PyPI
// by fetching the project's JSON record and returns the canonical
// sdist sha256 hash when one is published. A wheel hash is used as
// fallback for wheel-only releases. The package name is normalised
// per PEP 503 before the request — private mirrors without redirect
// logic only resolve the canonical form.
func (pypiCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	base := pickPyPIURL()
	if base == "" {
		return nil, fmt.Errorf("no usable pypi URL (set TILLIT_PYPI_URL)")
	}
	canonName := normalizePackageName(packageID)
	endpoint := fmt.Sprintf("%s/pypi/%s/%s/json",
		strings.TrimRight(base, "/"),
		url.PathEscape(canonName),
		url.PathEscape(version),
	)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("contact pypi: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("pypi: version not found")
	default:
		return nil, fmt.Errorf("pypi returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read pypi response: %w", err)
	}

	var doc pypiResponse
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse pypi JSON: %w", err)
	}

	info := &ecosystems.VersionInfo{
		PackageID: canonName,
		Version:   version,
	}
	if hash := pickArtifactHash(doc.URLs); hash != "" {
		info.Hash = hash
		info.HashAlgo = "sha256"
	}
	return info, nil
}

type pypiResponse struct {
	Info struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"info"`
	URLs []pypiArtifact `json:"urls"`
}

type pypiArtifact struct {
	PackageType string `json:"packagetype"`
	Digests     struct {
		SHA256 string `json:"sha256"`
	} `json:"digests"`
}

// pickArtifactHash returns the canonical sdist hash when one is
// published; otherwise the first wheel hash. PyPI only returns
// digests for hosted artifacts, so wheel-only releases are still
// vetted by their wheel content rather than failing the call.
func pickArtifactHash(urls []pypiArtifact) string {
	for _, u := range urls {
		if u.PackageType == "sdist" && u.Digests.SHA256 != "" {
			return u.Digests.SHA256
		}
	}
	for _, u := range urls {
		if u.Digests.SHA256 != "" {
			return u.Digests.SHA256
		}
	}
	return ""
}

func pickPyPIURL() string {
	if v := os.Getenv("TILLIT_PYPI_URL"); v != "" {
		return v
	}
	return defaultPyPIURL
}
