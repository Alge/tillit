package cocoapods

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// defaultTrunkURL is the canonical CocoaPods Trunk root. Override
// with TILLIT_COCOAPODS_URL when targeting a private spec mirror
// that exposes the same /api/v1/pods endpoints.
const defaultTrunkURL = "https://trunk.cocoapods.org"

// ResolveVersion verifies that (packageID, version) exists on Trunk
// by fetching the podspec for that release. The endpoint returns
// the .podspec JSON we don't otherwise need, so we only inspect the
// HTTP status: 200 → exists, 404 → missing.
//
// Subspecs (`Foo/Bar`) aren't published as independent specs on
// Trunk — they live inside the umbrella pod's spec — so we strip
// the subspec component before the request. The lockfile is trusted
// to record a valid subspec name; what we're verifying here is that
// the umbrella pod has the requested version.
//
// Hash isn't returned: Trunk doesn't expose the .podspec MD5 (which
// is what `SPEC CHECKSUMS` records) without computing it locally
// over the spec content. The PackageRef from Parse already carries
// the hash from the lockfile, which is the authoritative value the
// trust system needs.
func (cocoapodsCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	base := pickCocoapodsURL()
	if base == "" {
		return nil, fmt.Errorf("no usable CocoaPods URL (set TILLIT_COCOAPODS_URL)")
	}
	umbrella := packageID
	if i := strings.Index(umbrella, "/"); i >= 0 {
		umbrella = umbrella[:i]
	}
	endpoint := fmt.Sprintf("%s/api/v1/pods/%s/specs/%s",
		strings.TrimRight(base, "/"),
		url.PathEscape(umbrella),
		url.PathEscape(version),
	)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("contact CocoaPods Trunk: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("CocoaPods Trunk: version not found")
	default:
		return nil, fmt.Errorf("CocoaPods Trunk returned %s", resp.Status)
	}

	// Drain the body so the connection can be reused. We don't need
	// the contents.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	return &ecosystems.VersionInfo{
		PackageID: packageID,
		Version:   version,
	}, nil
}

func pickCocoapodsURL() string {
	if v := os.Getenv("TILLIT_COCOAPODS_URL"); v != "" {
		return v
	}
	return defaultTrunkURL
}
