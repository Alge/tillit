package swiftpm

import "github.com/Alge/tillit/ecosystems"

// ResolveVersion is a no-op for Swift Package Manager. Unlike npm,
// rubygems, and the rest of the registry-backed ecosystems, Swift
// has no widely-adopted registry that exposes a canonical hash
// per (package, version) tuple — packages live as git tags on
// arbitrary hosts, and the Swift Package Registry that would change
// this is not yet ubiquitous.
//
// Rather than making an unreliable network call, we return the input
// (packageID, version) as a VersionInfo with no Hash. Sign-time
// validation that the version exists falls back to human review
// — the user has presumably built against this version and is
// signing it deliberately.
//
// TODO: when a swift package is hosted on github.com, hit the
// /repos/<owner>/<repo>/git/refs/tags/<version> API to at least
// verify the tag exists. Out of scope for the first cut.
func (swiftpmCommon) ResolveVersion(packageID, version string) (*ecosystems.VersionInfo, error) {
	return &ecosystems.VersionInfo{
		PackageID: packageID,
		Version:   version,
	}, nil
}
