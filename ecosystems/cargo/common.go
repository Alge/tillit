// Package cargo parses Rust lockfile formats whose packages live on
// crates.io (or a compatible registry exposing the same JSON shape)
// into the canonical PackageRef shape consumed by the trust graph
// resolver. Each lockfile format gets its own adapter type inside
// this package; they all share Ecosystem() == "cargo".
package cargo

import "github.com/Alge/tillit/ecosystems/internal/semver"

// cargoCommon carries the methods shared by every Cargo lockfile
// adapter in this package: identity, version comparison, version
// validation, and registry-side existence/hash resolution. Per-
// format adapters embed it so they only need to implement Name,
// CanParse, and Parse.
type cargoCommon struct{}

func (cargoCommon) Ecosystem() string { return "cargo" }

func (cargoCommon) CompareVersions(a, b string) int { return semver.Compare(a, b) }

func (cargoCommon) ValidateVersion(v string) error { return semver.Validate(v) }
