// Package ecosystems defines the lockfile-adapter contract.
//
// Each package manager has one or more lockfile formats; an Adapter parses a
// single format into the canonical PackageRef tuple used by the trust graph
// resolver. Adapters that produce the same Ecosystem() value are
// interchangeable as far as decisions are concerned — a vetting of
// (go, github.com/foo/bar, v1.2.3) applies whether the package was seen via
// go.sum, go.mod, or any future Go-ecosystem format.
package ecosystems

import "io/fs"

// PackageRef is one (ecosystem, package, version) tuple extracted from a
// lockfile. The resolver keys on Ecosystem+PackageID+Version; the other
// fields are descriptive metadata adapters fill in when they can.
type PackageRef struct {
	Ecosystem string
	PackageID string
	Version   string

	// Direct reports whether the project declares this as a direct
	// dependency (as opposed to inherited transitively). For ecosystems
	// where the distinction requires a separate file (Go's go.mod, npm's
	// package.json, etc.), Direct is best-effort: false when the project
	// file isn't available.
	Direct bool

	// Optional metadata. None of these participate in the trust lookup key.
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
//
// Adapters take an fs.FS rather than an io.Reader so they can also read
// sibling files — most ecosystems need the project manifest (go.mod,
// package.json, pyproject.toml) alongside the lockfile to distinguish
// direct from transitive dependencies. Tests use fstest.MapFS; production
// callers use os.DirFS rooted at the project directory.
type Adapter interface {
	// Ecosystem is the canonical ecosystem identifier used in signatures —
	// "go", "pip", "npm", etc. Drives the trust lookup key.
	Ecosystem() string

	// Name is a short human-readable description of the format the adapter
	// handles, e.g. "go.sum" or "Pipfile.lock". Used in CLI output.
	Name() string

	// CanParse reports whether this adapter recognises the given path
	// (relative to whichever fs.FS the caller uses). Typically a filename
	// match. The caller is expected to call CanParse before Parse.
	CanParse(path string) bool

	// Parse reads the lockfile (and any sibling project file the adapter
	// needs) from the filesystem and returns the canonical package list
	// plus any non-fatal warnings.
	Parse(fsys fs.FS, lockfilePath string) (ParseResult, error)

	// CompareVersions orders two version strings using this ecosystem's
	// version semantics (Go semver, PEP 440, npm semver, etc.). Returns
	// -1, 0, or 1 if a is less than, equal to, or greater than b.
	// Adapters serving the same ecosystem must agree on the comparison
	// rule. Used by the resolver for output ordering and by sign-diff
	// to verify that FromVersion precedes ToVersion.
	CompareVersions(a, b string) int
}
