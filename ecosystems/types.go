// Package ecosystems defines the lockfile-adapter contract.
//
// Each package manager has one or more lockfile formats; an Adapter parses a
// single format into the canonical PackageRef tuple used by the trust graph
// resolver. Adapters that produce the same Ecosystem() value are
// interchangeable as far as decisions are concerned — a vetting of
// (go, github.com/foo/bar, v1.2.3) applies whether the package was seen via
// go.sum, go.mod, or any future Go-ecosystem format.
package ecosystems

import "io"

// PackageRef is one (ecosystem, package, version) tuple extracted from a
// lockfile. The resolver keys on Ecosystem+PackageID+Version; Hash and Source
// are exposed for adapters that have them but the resolver may ignore them.
type PackageRef struct {
	Ecosystem string
	PackageID string
	Version   string

	// Optional metadata. Adapters fill what they can; consumers may ignore
	// what they don't need. None of these participate in the trust lookup
	// key today.
	Hash   string // artifact hash, e.g. "h1:abc=", "sha512-..."
	Source string // registry URL or "local"/"git" for non-registry sources
}

// ParseResult separates fatal errors (returned via err) from non-fatal
// issues like unrecognised lines or local-path entries we can't vet. Callers
// decide whether to surface warnings to the user or ignore them.
type ParseResult struct {
	Packages []PackageRef
	Warnings []string
}

// Adapter parses one lockfile format. Each format gets its own adapter;
// adapters serving the same package ecosystem return the same Ecosystem()
// value.
type Adapter interface {
	// Ecosystem is the canonical ecosystem identifier used in signatures —
	// "go", "pip", "npm", etc. Drives the trust lookup key.
	Ecosystem() string

	// Name is a short human-readable description of the format the adapter
	// handles, e.g. "go.sum" or "Pipfile.lock". Used in CLI output.
	Name() string

	// CanParse reports whether this adapter recognises the given path.
	// Typically a filename match. The caller is expected to call CanParse
	// before Parse.
	CanParse(path string) bool

	// Parse reads the lockfile content and returns the canonical package
	// list plus any non-fatal warnings.
	Parse(r io.Reader) (ParseResult, error)
}
