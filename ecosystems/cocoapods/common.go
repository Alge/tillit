// Package cocoapods parses CocoaPods lockfile formats whose pods
// live on the Trunk registry (https://trunk.cocoapods.org/) into the
// canonical PackageRef shape consumed by the trust graph resolver.
package cocoapods

import "github.com/Alge/tillit/ecosystems/internal/semver"

// cocoapodsCommon carries the methods shared by every CocoaPods
// lockfile adapter in this package: identity, version comparison,
// version validation, and registry-side existence/hash resolution.
// Per-format adapters embed it so they only need to implement Name,
// CanParse, and Parse.
type cocoapodsCommon struct{}

func (cocoapodsCommon) Ecosystem() string { return "cocoapods" }

func (cocoapodsCommon) CompareVersions(a, b string) int { return semver.Compare(a, b) }

func (cocoapodsCommon) ValidateVersion(v string) error { return semver.Validate(v) }
