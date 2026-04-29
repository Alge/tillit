// Package composer parses PHP lockfile formats whose packages live on
// Packagist (or a compatible mirror exposing the same JSON shape) into
// the canonical PackageRef shape consumed by the trust graph resolver.
// Each lockfile format gets its own adapter type inside this package;
// they all share Ecosystem() == "composer".
package composer

import "github.com/Alge/tillit/ecosystems/internal/semver"

// composerCommon carries the methods shared by every Composer lockfile
// adapter in this package. Per-format adapters embed it so they only
// need to implement Name, CanParse, and Parse.
type composerCommon struct{}

func (composerCommon) Ecosystem() string { return "composer" }

func (composerCommon) CompareVersions(a, b string) int { return semver.Compare(a, b) }

func (composerCommon) ValidateVersion(v string) error { return semver.Validate(v) }
