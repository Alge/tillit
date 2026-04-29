package npmlock

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

// defaultRegistry is queried when npm_config_registry isn't set.
const defaultRegistry = "https://registry.npmjs.org"

// maxResponseBytes caps the per-version metadata read. The npm
// registry's /<name>/<ver> endpoint returns one release's JSON —
// typically a few KB even for popular packages — so 1 MiB is a
// comfortable ceiling that protects against a misbehaving mirror.
const maxResponseBytes = 1 << 20

// ResolveVersion confirms that (packageID, version) exists in the
// configured npm registry and returns the canonical sha512
// integrity hash from the registry's per-version metadata. The
// query uses the per-version endpoint so the response is small even
// for popular packages.
//
// Honours the npm_config_registry environment variable, which both
// `npm` and `pnpm` set when the user has configured a private
// mirror; otherwise falls back to registry.npmjs.org.
func (npmCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	registry := pickRegistry()
	if registry == "" {
		return nil, fmt.Errorf("no usable npm registry (set npm_config_registry or use --skip-verify)")
	}

	// Scoped packages contain '/' which has special meaning in URL
	// paths; encode the package name path-segment-safely. The
	// version is appended raw (semver chars are URL-safe).
	endpoint := fmt.Sprintf("%s/%s/%s",
		strings.TrimRight(registry, "/"),
		encodePackageName(packageID),
		url.PathEscape(version))

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("contact npm registry: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("npm registry: version not found")
	default:
		return nil, fmt.Errorf("npm registry returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read registry response: %w", err)
	}
	return parseRegistryResponse(body, packageID, version)
}

// parseRegistryResponse turns the per-version metadata JSON into a
// VersionInfo. Split out from ResolveVersion so the JSON-shape edge
// cases — `{"error": ...}` payloads and responses missing the
// `version` field — can be unit-tested without spinning up an
// httptest server. Other adapters in the project inline the parse
// step because their per-version JSON is straightforward; npm's is
// the irregular one (integrity-vs-shasum branching, an inline error
// field instead of a 404), so the extra surface earns its keep.
func parseRegistryResponse(body []byte, packageID, version string) (*ecosystems.VersionInfo, error) {
	var v struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Dist    struct {
			Integrity string `json:"integrity"`
			Shasum    string `json:"shasum"`
		} `json:"dist"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("parse registry response: %w", err)
	}
	if v.Error != "" {
		return nil, fmt.Errorf("npm registry: %s", v.Error)
	}
	if v.Version == "" {
		return nil, fmt.Errorf("npm registry response had no version field")
	}
	info := &ecosystems.VersionInfo{
		PackageID: packageID,
		Version:   version,
	}
	// Prefer integrity (sha512, what package-lock.json records).
	// Fall back to shasum (sha1) when the registry doesn't expose
	// integrity for an old entry.
	switch {
	case v.Dist.Integrity != "":
		info.Hash = v.Dist.Integrity
		// integrity has the form "<algo>-<base64>" — extract algo
		// for the HashAlgo field so callers can reason about it.
		if i := strings.Index(v.Dist.Integrity, "-"); i > 0 {
			info.HashAlgo = v.Dist.Integrity[:i]
		}
	case v.Dist.Shasum != "":
		info.Hash = "sha1-" + v.Dist.Shasum
		info.HashAlgo = "sha1"
	}
	return info, nil
}

// pickRegistry honours npm_config_registry (set by `npm config set
// registry ...` and exported on every npm/pnpm invocation), falling
// back to the public registry when unset.
func pickRegistry() string {
	if r := os.Getenv("npm_config_registry"); r != "" {
		return r
	}
	if r := os.Getenv("NPM_CONFIG_REGISTRY"); r != "" {
		return r
	}
	return defaultRegistry
}

// encodePackageName URL-encodes a package name for use as a path
// segment. Scoped packages (@scope/name) need the leading '@' and
// internal '/' preserved; npm registry URLs accept that as-is, so
// we just encode the ones that aren't path-safe.
func encodePackageName(name string) string {
	// The npm registry accepts scoped paths verbatim
	// ("/@types/node/20.0.0"). The components themselves are URL-
	// safe alphanumerics + dashes/underscores in practice. Pass
	// through unchanged.
	return name
}
