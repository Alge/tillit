// Package rubygems parses Ruby lockfile formats whose packages live
// on rubygems.org (or a compatible mirror exposing the same JSON
// shape) into the canonical PackageRef shape consumed by the trust
// graph resolver. Each lockfile format gets its own adapter type
// inside this package; they all share Ecosystem() == "rubygems".
package rubygems

// rubygemsCommon carries the methods shared by every rubygems
// lockfile adapter. Per-format adapters embed it so they only need
// to implement Name, CanParse, and Parse.
type rubygemsCommon struct{}

func (rubygemsCommon) Ecosystem() string { return "rubygems" }

func (rubygemsCommon) CompareVersions(a, b string) int { return compareGemVersion(a, b) }

func (rubygemsCommon) ValidateVersion(v string) error { return validateGemVersion(v) }
