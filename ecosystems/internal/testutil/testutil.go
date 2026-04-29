// Package testutil collects test helpers shared across the ecosystem
// adapter packages. Importing this from ecosystems/<name>/*_test.go
// keeps every adapter from redefining the same five-line helpers.
package testutil

import "strings"

// WarningContains reports whether any string in warns has sub as a
// substring. Adapter Parse implementations return a `[]string` of
// non-fatal warnings; tests use this to assert that a particular
// skip ("git source", "no version", etc.) was surfaced without
// pinning the exact wording.
func WarningContains(warns []string, sub string) bool {
	for _, w := range warns {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}
