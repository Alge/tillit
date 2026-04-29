package npmlock

import (
	"fmt"
	"strconv"
	"strings"
)

// ValidateVersion accepts the strict semver 2.0.0 form npm uses for
// resolved versions in lockfiles: MAJOR.MINOR.PATCH with optional
// pre-release (-alpha.1) and build metadata (+sha.deadbeef). No
// leading 'v' (that's a Go convention).
func (NpmLock) ValidateVersion(v string) error {
	if v == "" {
		return fmt.Errorf("version is empty")
	}
	if strings.HasPrefix(v, "v") {
		return fmt.Errorf("npm version %q must not have a leading 'v'", v)
	}
	main, pre, build := splitSemver(v)
	if main == "" {
		return fmt.Errorf("version %q has no main numeric part", v)
	}
	parts := strings.Split(main, ".")
	if len(parts) != 3 {
		return fmt.Errorf("version %q must have MAJOR.MINOR.PATCH (got %d segment(s))", v, len(parts))
	}
	for _, p := range parts {
		if p == "" {
			return fmt.Errorf("version %q has an empty numeric segment", v)
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return fmt.Errorf("version %q has non-numeric character %q in main", v, r)
			}
		}
	}
	if strings.Contains(v, "-") && pre == "" {
		return fmt.Errorf("version %q has empty pre-release after '-'", v)
	}
	if pre != "" && !validSemverIdentifierRun(pre) {
		return fmt.Errorf("version %q has invalid pre-release %q", v, pre)
	}
	if strings.Contains(v, "+") && build == "" {
		return fmt.Errorf("version %q has empty build metadata after '+'", v)
	}
	if build != "" && !validSemverIdentifierRun(build) {
		return fmt.Errorf("version %q has invalid build metadata %q", v, build)
	}
	return nil
}

// CompareVersions implements semver 2.0.0 precedence (§11):
// numeric-segment compare, release-beats-pre-release, then dot-
// separated identifier compare for pre-release. Build metadata is
// ignored for ordering.
func (NpmLock) CompareVersions(a, b string) int {
	aMain, aPre, _ := splitSemver(a)
	bMain, bPre, _ := splitSemver(b)
	if c := compareNumericTriple(aMain, bMain); c != 0 {
		return c
	}
	switch {
	case aPre == "" && bPre == "":
		return 0
	case aPre == "":
		return 1
	case bPre == "":
		return -1
	default:
		return compareSemverPreRelease(aPre, bPre)
	}
}

// splitSemver pulls a semver string apart into (main, pre, build).
// Build follows '+'; pre-release follows '-'. Empty strings for
// missing parts.
func splitSemver(v string) (main, pre, build string) {
	rest := v
	if i := strings.Index(rest, "+"); i >= 0 {
		build = rest[i+1:]
		rest = rest[:i]
	}
	if i := strings.Index(rest, "-"); i >= 0 {
		pre = rest[i+1:]
		rest = rest[:i]
	}
	main = rest
	return
}

func compareNumericTriple(a, b string) int {
	aSegs := strings.Split(a, ".")
	bSegs := strings.Split(b, ".")
	for i := 0; i < 3 && i < len(aSegs) && i < len(bSegs); i++ {
		ai, errA := strconv.Atoi(aSegs[i])
		bi, errB := strconv.Atoi(bSegs[i])
		if errA != nil || errB != nil {
			if c := strings.Compare(aSegs[i], bSegs[i]); c != 0 {
				return c
			}
			continue
		}
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}

// compareSemverPreRelease compares dot-separated identifiers per
// semver §11.4: numeric < non-numeric for cross-type, and shorter
// prefix < longer prefix when one runs out first.
func compareSemverPreRelease(a, b string) int {
	aSegs := strings.Split(a, ".")
	bSegs := strings.Split(b, ".")
	n := len(aSegs)
	if len(bSegs) > n {
		n = len(bSegs)
	}
	for i := 0; i < n; i++ {
		if i >= len(aSegs) {
			return -1
		}
		if i >= len(bSegs) {
			return 1
		}
		ai, errA := strconv.Atoi(aSegs[i])
		bi, errB := strconv.Atoi(bSegs[i])
		switch {
		case errA == nil && errB == nil:
			if ai != bi {
				if ai < bi {
					return -1
				}
				return 1
			}
		case errA == nil:
			return -1 // numeric ranks below non-numeric
		case errB == nil:
			return 1
		default:
			if c := strings.Compare(aSegs[i], bSegs[i]); c != 0 {
				return c
			}
		}
	}
	return 0
}

// validSemverIdentifierRun checks that s is dot-separated alphanumeric
// (or hyphen) identifiers, no empty parts.
func validSemverIdentifierRun(s string) bool {
	if s == "" {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if part == "" {
			return false
		}
		for _, r := range part {
			ok := (r >= '0' && r <= '9') ||
				(r >= 'a' && r <= 'z') ||
				(r >= 'A' && r <= 'Z') ||
				r == '-'
			if !ok {
				return false
			}
		}
	}
	return true
}
