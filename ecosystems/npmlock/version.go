package npmlock

import "github.com/Alge/tillit/ecosystems/internal/semver"

// ValidateVersion accepts the strict semver 2.0.0 form npm uses for
// resolved versions in lockfiles: MAJOR.MINOR.PATCH with optional
// pre-release (-alpha.1) and build metadata (+sha.deadbeef).
func (NpmLock) ValidateVersion(v string) error { return semver.Validate(v) }

// CompareVersions implements semver 2.0.0 precedence (§11). Build
// metadata is ignored for ordering (§10).
func (NpmLock) CompareVersions(a, b string) int { return semver.Compare(a, b) }
