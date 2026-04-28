package resolver

import (
	"sort"
	"strconv"
	"strings"
)

// VersionRange is a contiguous run of versions (in sort order) that share
// the same status. From and To are the lowest and highest versions of the
// run; for a single-version run From == To.
type VersionRange struct {
	From      string
	To        string
	Status    Status
	Decisions []ContributingDecision // union of decisions across the run
}

// GroupVersions sorts pv.Versions by version and collapses adjacent
// same-status entries into ranges.
func GroupVersions(pv PackageVerdict) []VersionRange {
	type entry struct {
		Version string
		Verdict Verdict
	}
	entries := make([]entry, 0, len(pv.Versions))
	for v, ver := range pv.Versions {
		entries = append(entries, entry{Version: v, Verdict: ver})
	}
	sort.Slice(entries, func(i, j int) bool {
		return CompareVersions(entries[i].Version, entries[j].Version) < 0
	})

	var groups []VersionRange
	for _, e := range entries {
		if n := len(groups); n > 0 && groups[n-1].Status == e.Verdict.Status {
			groups[n-1].To = e.Version
			groups[n-1].Decisions = append(groups[n-1].Decisions, e.Verdict.Decisions...)
			continue
		}
		groups = append(groups, VersionRange{
			From:      e.Version,
			To:        e.Version,
			Status:    e.Verdict.Status,
			Decisions: append([]ContributingDecision(nil), e.Verdict.Decisions...),
		})
	}
	return groups
}

// CompareVersions orders two version strings. It handles the common
// dotted-numeric case (v1.2.10 > v1.2.9) without a full semver parser:
// strip an optional leading "v", split on dots, compare segments
// numerically when both sides are integers, lexicographically otherwise.
// A pre-release suffix ("-rc1", "-beta") sorts before the release.
//
// Good enough for `tillit query` output ordering across the ecosystems
// we target (Go semver, PEP 440 numeric prefixes, npm semver). Specific
// adapters can plug in stricter comparators later.
func CompareVersions(a, b string) int {
	aBase, aPre := splitPreRelease(a)
	bBase, bPre := splitPreRelease(b)

	if c := compareNumericDotted(aBase, bBase); c != 0 {
		return c
	}
	// Same numeric base: a release (no pre-release) sorts after a
	// pre-release of the same base.
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
			return -1 // numeric < non-numeric
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
