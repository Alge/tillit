// Package pypi parses Python lockfile formats whose packages live on
// PyPI into the canonical PackageRef shape consumed by the trust
// graph resolver. Each lockfile format gets its own adapter type
// inside this package; they all share Ecosystem() == "pypi" so a
// vetting of `requests==2.31.0` is portable regardless of which
// tool resolved it.
package pypi

import (
	"path/filepath"
	"strings"
)

// Requirements is the adapter for pip's `requirements.txt` lockfile
// format (including pip-compile output). It implements
// ecosystems.Adapter.
type Requirements struct{ pypiCommon }

func (Requirements) Name() string { return "requirements.txt" }

// CanParse matches the canonical name plus the conventional
// `requirements-<env>.txt` variant that real projects use for
// environment-specific lockfiles.
func (Requirements) CanParse(p string) bool {
	if p == "" {
		return false
	}
	base := filepath.Base(p)
	if base == "requirements.txt" {
		return true
	}
	return strings.HasPrefix(base, "requirements-") && strings.HasSuffix(base, ".txt")
}
