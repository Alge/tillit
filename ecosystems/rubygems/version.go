package rubygems

import (
	"cmp"
	"strconv"
	"strings"
)

// RubyGems uses a versioning scheme close to semver but with a few
// quirks: pre-release segments are dot-separated (`1.0.0.pre.1`,
// `1.0.0.alpha`) rather than dash-separated, trailing numeric zeros
// are equivalent (`1.0` == `1.0.0`), and any segment containing a
// letter is treated as a pre-release marker that sorts BEFORE the
// equivalent release.
//
// We implement gem version comparison locally rather than reusing
// the shared semver package because too many real Gemfile.lock
// entries would fail strict-semver validation (`6.1.7.6`,
// `1.0.0.pre`, etc.).

// compareGemVersion orders two gem version strings. Returns -1, 0, 1.
func compareGemVersion(a, b string) int {
	aSegs := splitGemSegments(a)
	bSegs := splitGemSegments(b)

	n := len(aSegs)
	if len(bSegs) > n {
		n = len(bSegs)
	}
	for i := 0; i < n; i++ {
		aHas, bHas := i < len(aSegs), i < len(bSegs)
		switch {
		case !aHas:
			// `a` ran out. The remainder of `b` decides the result:
			//   all-zero numerics → still equal
			//   contains a non-zero number → b is greater
			//   contains a string → b is a pre-release of a → a is greater
			return -classifyGemTrailing(bSegs[i:])
		case !bHas:
			return classifyGemTrailing(aSegs[i:])
		}

		aSeg, bSeg := aSegs[i], bSegs[i]
		aNum, aIsNum := tryAtoi(aSeg)
		bNum, bIsNum := tryAtoi(bSeg)
		switch {
		case aIsNum && bIsNum:
			if c := cmp.Compare(aNum, bNum); c != 0 {
				return c
			}
		case aIsNum:
			// numeric > string → release > pre-release
			return 1
		case bIsNum:
			return -1
		default:
			if c := cmp.Compare(aSeg, bSeg); c != 0 {
				return c
			}
		}
	}
	return 0
}

// validateGemVersion accepts the syntax gems publish in lockfiles —
// dot-separated segments where each segment is either digits or a
// letter-led alphanumeric tag. Empty input or stray characters
// (whitespace, dashes outside known forms, etc.) are rejected so
// typos don't slip into the trust store.
func validateGemVersion(v string) error {
	if v == "" {
		return errEmptyVersion
	}
	for _, seg := range strings.Split(v, ".") {
		if seg == "" {
			return errEmptySegment
		}
		for _, r := range seg {
			ok := (r >= '0' && r <= '9') ||
				(r >= 'a' && r <= 'z') ||
				(r >= 'A' && r <= 'Z')
			if !ok {
				return errInvalidChar
			}
		}
	}
	return nil
}

// splitGemSegments breaks a gem version on dots. A bare leading 'v'
// is allowed — some `Gemfile.lock` entries copy the upstream tag
// verbatim — and stripped before the split so it doesn't show up as
// a leading non-numeric segment.
func splitGemSegments(v string) []string {
	v = strings.TrimPrefix(v, "v")
	return strings.Split(v, ".")
}

// classifyGemTrailing inspects the leftover segments after one side
// runs out: returns 0 when they're all numeric zeros (equivalent
// trailing-zero padding), -1 when at least one is non-numeric (the
// longer side is a pre-release of the shorter), and 1 when at least
// one is a non-zero number (the longer side is strictly greater).
func classifyGemTrailing(segs []string) int {
	preReleaseSeen := false
	for _, s := range segs {
		if n, ok := tryAtoi(s); ok {
			if n != 0 {
				return 1
			}
			continue
		}
		preReleaseSeen = true
	}
	if preReleaseSeen {
		return -1
	}
	return 0
}

func tryAtoi(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}
