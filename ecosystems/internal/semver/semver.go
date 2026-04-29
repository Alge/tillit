// Package semver provides strict semver 2.0.0 parsing, validation,
// and ordering for ecosystem adapters whose registries use it
// (hex.pm, npm, crates.io, NuGet, pub.dev, ...).
//
// The implementation is deliberately strict: leading zeros in
// numeric segments, missing pre-release numbers, and other
// looseness commonly accepted by parsers are rejected here.
// Adapters whose ecosystem accepts a looser superset (or a
// different scheme entirely, like PEP 440 or Go pseudo-versions)
// must implement their own.
package semver

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

// version is the parsed form of a semver string. Pre-release and
// build identifiers are stored as their dot-separated identifier
// runs so the comparator can apply §11.4 segment rules.
type version struct {
	major, minor, patch int
	pre                 []string
	build               []string
}

// Validate reports whether v conforms to strict semver 2.0.0.
// Returns nil when valid; otherwise an error describing the first
// problem found.
func Validate(v string) error {
	_, err := parse(v)
	return err
}

// Compare orders a and b per semver §11. Build metadata is ignored
// (§10). Strings that fail to parse fall back to a lexicographic
// comparison so the comparator stays total — callers may surface
// invalid input upstream via Validate.
func Compare(a, b string) int {
	pa, errA := parse(a)
	pb, errB := parse(b)
	switch {
	case errA != nil && errB != nil:
		return cmp.Compare(a, b)
	case errA != nil:
		return -1
	case errB != nil:
		return 1
	}
	if c := cmp.Compare(pa.major, pb.major); c != 0 {
		return c
	}
	if c := cmp.Compare(pa.minor, pb.minor); c != 0 {
		return c
	}
	if c := cmp.Compare(pa.patch, pb.patch); c != 0 {
		return c
	}
	switch {
	case len(pa.pre) == 0 && len(pb.pre) == 0:
		return 0
	case len(pa.pre) == 0:
		return 1 // release > pre-release
	case len(pb.pre) == 0:
		return -1
	}
	return cmpPreRelease(pa.pre, pb.pre)
}

func parse(in string) (*version, error) {
	if in == "" {
		return nil, fmt.Errorf("version is empty")
	}
	s := in

	// Build metadata: everything after a single '+'.
	var build []string
	if i := strings.Index(s, "+"); i >= 0 {
		buildStr := s[i+1:]
		s = s[:i]
		if buildStr == "" {
			return nil, fmt.Errorf("version %q has empty build metadata after '+'", in)
		}
		ids, err := splitIdents(buildStr, "build")
		if err != nil {
			return nil, fmt.Errorf("version %q: %w", in, err)
		}
		build = ids
	}

	// Pre-release: everything after the first '-'.
	var pre []string
	if i := strings.Index(s, "-"); i >= 0 {
		preStr := s[i+1:]
		s = s[:i]
		if preStr == "" {
			return nil, fmt.Errorf("version %q has empty pre-release after '-'", in)
		}
		ids, err := splitIdents(preStr, "pre-release")
		if err != nil {
			return nil, fmt.Errorf("version %q: %w", in, err)
		}
		// Numeric pre-release identifiers must have no leading zero
		// (other than the bare "0") per semver §9.
		for _, id := range ids {
			if isAllDigits(id) && len(id) > 1 && id[0] == '0' {
				return nil, fmt.Errorf("version %q has numeric pre-release identifier %q with leading zero", in, id)
			}
		}
		pre = ids
	}

	// Main: exactly three dot-separated unsigned integers, no leading zeros.
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("version %q must have major.minor.patch (got %d segments)", in, len(parts))
	}
	v := &version{pre: pre, build: build}
	for i, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("version %q has empty numeric segment", in)
		}
		if len(p) > 1 && p[0] == '0' {
			return nil, fmt.Errorf("version %q has leading zero in numeric segment %q", in, p)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("version %q has non-numeric segment %q", in, p)
		}
		switch i {
		case 0:
			v.major = n
		case 1:
			v.minor = n
		case 2:
			v.patch = n
		}
	}
	return v, nil
}

// splitIdents splits a dot-separated identifier run, validating
// that each identifier is non-empty and uses only alphanumerics
// and hyphens. Used for both pre-release and build segments.
func splitIdents(s, label string) ([]string, error) {
	parts := strings.Split(s, ".")
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("empty %s identifier", label)
		}
		for _, r := range p {
			ok := (r >= '0' && r <= '9') ||
				(r >= 'a' && r <= 'z') ||
				(r >= 'A' && r <= 'Z') ||
				r == '-'
			if !ok {
				return nil, fmt.Errorf("%s identifier %q has invalid character %q", label, p, r)
			}
		}
	}
	return parts, nil
}

// cmpPreRelease compares two pre-release identifier runs per
// semver §11.4. Numeric identifiers compare numerically;
// alphanumeric compare lexicographically; numeric < alphanumeric
// when types differ. A run with more identifiers wins ties on
// shared prefixes (§11.4.4).
func cmpPreRelease(a, b []string) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if i >= len(a) {
			return -1
		}
		if i >= len(b) {
			return 1
		}
		aIsNum := isAllDigits(a[i])
		bIsNum := isAllDigits(b[i])
		switch {
		case aIsNum && bIsNum:
			an, _ := strconv.Atoi(a[i])
			bn, _ := strconv.Atoi(b[i])
			if c := cmp.Compare(an, bn); c != 0 {
				return c
			}
		case aIsNum:
			return -1
		case bIsNum:
			return 1
		default:
			if c := cmp.Compare(a[i], b[i]); c != 0 {
				return c
			}
		}
	}
	return 0
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
