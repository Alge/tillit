package gosum

import (
	"fmt"
	"strings"
)

// ValidateVersion accepts the Go-module version forms the toolchain
// produces:
//
//   - Standard semver: v1.2.3 (with optional extra dotted segments)
//   - Pre-release: v1.0.0-rc1, v1.0.0-alpha.2
//   - Pseudo-versions: v0.0.0-20250101120000-abcdef123456
//   - Build metadata: v1.2.3+build.1
//
// Each non-empty numeric segment in the main part is required; the pre-
// release and build suffixes may contain alphanumerics, dots, and
// hyphens but cannot be empty after their introducing character. The
// goal is to reject typos like "v.3.0.0" or "3.0.0" before a signature
// is written, not to pass the full semver 2.0.0 spec — anything close
// enough that the Go toolchain accepts it must validate here.
func (GoSum) ValidateVersion(v string) error {
	if v == "" {
		return fmt.Errorf("version is empty")
	}
	if !strings.HasPrefix(v, "v") {
		return fmt.Errorf("version %q must start with 'v' (e.g. v1.2.3)", v)
	}
	rest := v[1:]
	if rest == "" {
		return fmt.Errorf("version %q has no number after 'v'", v)
	}

	// Split off build metadata (+) first so a "+" inside pre-release
	// can't slip through. Semver: at most one "+".
	main, build, _ := cutAt(rest, "+")
	if strings.Contains(build, "+") {
		return fmt.Errorf("version %q has multiple '+' separators", v)
	}
	if strings.Contains(rest, "+") && build == "" {
		return fmt.Errorf("version %q has empty build metadata after '+'", v)
	}
	if build != "" && !validIdentifierRun(build) {
		return fmt.Errorf("version %q has invalid build metadata %q", v, build)
	}

	// Then split off pre-release (-).
	num, pre, _ := cutAt(main, "-")
	if strings.Contains(main, "-") && pre == "" {
		return fmt.Errorf("version %q has empty pre-release after '-'", v)
	}
	if pre != "" && !validIdentifierRun(pre) {
		return fmt.Errorf("version %q has invalid pre-release %q", v, pre)
	}

	// Numeric main: at least one segment, every segment non-empty digits.
	if num == "" {
		return fmt.Errorf("version %q has no numeric segments", v)
	}
	for _, seg := range strings.Split(num, ".") {
		if seg == "" {
			return fmt.Errorf("version %q has an empty numeric segment", v)
		}
		for _, r := range seg {
			if r < '0' || r > '9' {
				return fmt.Errorf("version %q has non-numeric character %q in main part", v, r)
			}
		}
	}
	return nil
}

// cutAt splits s on the first occurrence of sep. Returns (before,
// after, found). Mirrors strings.Cut (Go 1.18+) inline so we don't have
// to thread a stdlib alias through.
func cutAt(s, sep string) (string, string, bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

// validIdentifierRun checks that s is a non-empty sequence of dot-
// separated identifiers, each composed of alphanumerics or hyphens.
// Empty identifiers between dots are rejected.
func validIdentifierRun(s string) bool {
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
