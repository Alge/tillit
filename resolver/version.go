package resolver

import (
	"strconv"
	"strings"
)

// CompareVersions orders two version strings using a generic
// numeric-aware comparison: dotted segments are compared numerically
// when both sides parse as integers, lexicographically otherwise. A
// pre-release suffix ("-rc1", "-beta") sorts before the release.
//
// Used as a fallback when no ecosystem-specific comparator is
// available; callers that have the right adapter should prefer
// adapter.CompareVersions.
func CompareVersions(a, b string) int {
	aBase, aPre := splitPreRelease(a)
	bBase, bPre := splitPreRelease(b)

	if c := compareNumericDotted(aBase, bBase); c != 0 {
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
		return strings.Compare(aPre, bPre)
	}
}

func splitPreRelease(v string) (base, pre string) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.Index(v, "-"); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

func compareNumericDotted(a, b string) int {
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
