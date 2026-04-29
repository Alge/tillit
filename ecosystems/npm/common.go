// Package npm parses Node.js lockfile formats whose packages live on
// the npm registry (npmjs.org or a compatible mirror) into the
// canonical PackageRef shape consumed by the trust graph resolver.
// Each lockfile format gets its own adapter type inside this package
// (PackageLock for package-lock.json, YarnLock for yarn.lock); they
// all share Ecosystem() == "npm" so a vetting of (npm, lodash, X) is
// portable regardless of which Node.js tool resolved it.
package npm

import (
	"github.com/Alge/tillit/ecosystems/internal/semver"
)

// npmCommon carries the methods shared by every npm-registry-backed
// adapter in this package: identity, version comparison, version
// validation, and registry-side existence/hash resolution. Per-
// format adapters (PackageLock, YarnLock, ...) embed it so they only
// need to implement Name, CanParse, and Parse.
type npmCommon struct{}

func (npmCommon) Ecosystem() string { return "npm" }

func (npmCommon) CompareVersions(a, b string) int { return semver.Compare(a, b) }

func (npmCommon) ValidateVersion(v string) error { return semver.Validate(v) }
