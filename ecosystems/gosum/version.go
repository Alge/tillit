package gosum

import (
	"strconv"
	"strings"
)

// compareGoVersion orders two Go-module version strings. It handles the
// common cases the Go toolchain produces:
//
//   - Standard semver: v1.2.3, v1.2.10
//   - Pre-release: v1.0.0-rc1, v1.0.0-alpha.2
//   - Pseudo-versions: v0.0.0-20250101120000-abcdef123456
//
// Pre-release / pseudo-version suffixes sort BEFORE the corresponding
// release. Numeric segments are compared numerically (so v1.2.10 > v1.2.9).
// Returns -1, 0, or 1.
//
// This is a deliberately compact implementation rather than a full
// semver parser — it covers what real go.sum files contain. Edge cases
// (build metadata, "+" suffix) are compared lexicographically.
func compareGoVersion(a, b string) int {
	aMain, aPre := splitVersion(a)
	bMain, bPre := splitVersion(b)

	if c := compareDottedNumeric(aMain, bMain); c != 0 {
		return c
	}
	switch {
	case aPre == "" && bPre == "":
		return 0
	case aPre == "":
		return 1 // release > pre-release of same base
	case bPre == "":
		return -1
	default:
		return comparePreRelease(aPre, bPre)
	}
}

// splitVersion strips a leading "v" and splits on the first "-" so the
// caller can compare the numeric main and the pre-release suffix
// independently.
func splitVersion(v string) (main, pre string) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.Index(v, "-"); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

func compareDottedNumeric(a, b string) int {
	aSegs := strings.Split(a, ".")
	bSegs := strings.Split(b, ".")
	n := len(aSegs)
	if len(bSegs) > n {
		n = len(bSegs)
	}
	for i := 0; i < n; i++ {
		var aSeg, bSeg string
		if i < len(aSegs) {
			aSeg = aSegs[i]
		}
		if i < len(bSegs) {
			bSeg = bSegs[i]
		}
		aN, aErr := strconv.Atoi(aSeg)
		bN, bErr := strconv.Atoi(bSeg)
		switch {
		case aErr == nil && bErr == nil:
			if aN != bN {
				if aN < bN {
					return -1
				}
				return 1
			}
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		default:
			if c := strings.Compare(aSeg, bSeg); c != 0 {
				return c
			}
		}
	}
	return 0
}

// comparePreRelease compares pre-release suffixes segment by segment, the
// dot-separated identifiers (per semver §11). Numeric identifiers compare
// numerically; mixed compare lexicographically with numeric < non-numeric.
func comparePreRelease(a, b string) int {
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
		aSeg, bSeg := aSegs[i], bSegs[i]
		aN, aErr := strconv.Atoi(aSeg)
		bN, bErr := strconv.Atoi(bSeg)
		switch {
		case aErr == nil && bErr == nil:
			if aN != bN {
				if aN < bN {
					return -1
				}
				return 1
			}
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		default:
			if c := strings.Compare(aSeg, bSeg); c != 0 {
				return c
			}
		}
	}
	return 0
}
