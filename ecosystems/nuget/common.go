// Package nuget parses .NET lockfile formats whose packages live on
// nuget.org (or a compatible v3 mirror) into the canonical PackageRef
// shape consumed by the trust graph resolver. Each lockfile format
// gets its own adapter type inside this package; they all share
// Ecosystem() == "nuget".
package nuget

import "github.com/Alge/tillit/ecosystems/internal/semver"

// nugetCommon carries the methods shared by every NuGet lockfile
// adapter in this package. Per-format adapters embed it so they only
// need to implement Name, CanParse, and Parse.
type nugetCommon struct{}

func (nugetCommon) Ecosystem() string { return "nuget" }

func (nugetCommon) CompareVersions(a, b string) int { return semver.Compare(a, b) }

func (nugetCommon) ValidateVersion(v string) error { return semver.Validate(v) }
