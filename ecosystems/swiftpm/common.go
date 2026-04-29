// Package swiftpm parses Swift Package Manager lockfile formats into
// the canonical PackageRef shape consumed by the trust graph
// resolver.
//
// Unlike npm or rubygems, Swift packages don't share a single
// authoritative registry — most are git URLs (typically GitHub),
// with the Swift Package Registry (api.swiftpackageregistry.com)
// only beginning to see adoption. This package therefore performs
// no canonical-hash resolution: ResolveVersion returns the
// (identity, version) tuple as-is so the trust store can record
// signatures, but no automated existence check or hash verification
// is performed at sign time.
package swiftpm

import "github.com/Alge/tillit/ecosystems/internal/semver"

// swiftpmCommon carries the methods shared by every Swift Package
// Manager lockfile adapter in this package: identity, version
// comparison, and version validation. Unlike the other ecosystems,
// the embedded ResolveVersion here is a no-op (see resolve.go) —
// Swift has no widely-adopted canonical-hash registry. Per-format
// adapters embed it so they only need to implement Name, CanParse,
// and Parse.
type swiftpmCommon struct{}

func (swiftpmCommon) Ecosystem() string { return "swiftpm" }

func (swiftpmCommon) CompareVersions(a, b string) int { return semver.Compare(a, b) }

func (swiftpmCommon) ValidateVersion(v string) error { return semver.Validate(v) }
