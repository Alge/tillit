// Package pub parses Dart / Flutter lockfile formats whose packages
// live on pub.dev (or a compatible mirror exposing the same JSON
// shape) into the canonical PackageRef shape consumed by the trust
// graph resolver. Each lockfile format gets its own adapter type
// inside this package; they all share Ecosystem() == "pub".
package pub

import "github.com/Alge/tillit/ecosystems/internal/semver"

// pubCommon carries the methods shared by every pub.dev lockfile
// adapter in this package. Per-format adapters embed it so they only
// need to implement Name, CanParse, and Parse.
type pubCommon struct{}

func (pubCommon) Ecosystem() string { return "pub" }

func (pubCommon) CompareVersions(a, b string) int { return semver.Compare(a, b) }

func (pubCommon) ValidateVersion(v string) error { return semver.Validate(v) }
