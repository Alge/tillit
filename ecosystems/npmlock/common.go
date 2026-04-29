package npmlock

import (
	"github.com/Alge/tillit/ecosystems/internal/semver"
)

// npmCommon carries the methods shared by every npm-registry-backed
// adapter in this package: identity, version comparison, version
// validation, and registry-side existence/hash resolution. Per-
// format adapters (NpmLock, YarnLock, ...) embed it so they only
// need to implement Name, CanParse, and Parse.
type npmCommon struct{}

func (npmCommon) Ecosystem() string { return "npm" }

func (npmCommon) CompareVersions(a, b string) int { return semver.Compare(a, b) }

func (npmCommon) ValidateVersion(v string) error { return semver.Validate(v) }
