package hexpm

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/Alge/tillit/ecosystems"
)

// RebarLock is the adapter for Erlang's `rebar.lock` files (rebar3).
// rebar3 publishes to hex.pm — the same registry Mix and Gleam use —
// so this adapter shares Ecosystem() == "hexpm" with the others in
// this package.
//
// rebar.lock is an Erlang term file:
//
//	{"VERSION", [PACKAGES]}.
//	[
//	  {pkg_hash, [{<<"name">>, <<"hex_hash">>}, ...]},
//	  {pkg_hash_ext, [{<<"name">>, <<"outer_hash">>}, ...]}
//	].
//
// Each package entry shape varies by source:
//
//	{<<"alias">>, {pkg, <<"name">>, <<"version">>}, level}     // hex.pm
//	{<<"alias">>, {git, "url", {ref, "..."}}, level}            // git
//	{<<"alias">>, {raw, "..."}, level}                          // arbitrary
//
// We emit hex-sourced entries and warn-skip the rest. Note the
// alias and the registry name can differ — the project may depend
// on a hex package under a custom local atom — so we key signing on
// the registry name (the inner `{pkg, <<"name">>, ...}`), not the
// alias.
type RebarLock struct{ hexpmCommon }

func (RebarLock) Name() string { return "rebar.lock" }

func (RebarLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "rebar.lock"
}

// rebarPkgEntryRE matches a hex-sourced package tuple. The capture
// order is (alias, registry-name, version). The (?s) flag lets the
// inner whitespace span lines so multi-line entries match.
var rebarPkgEntryRE = regexp.MustCompile(`(?s)\{<<"([^"]+)">>\s*,\s*\{pkg\s*,\s*<<"([^"]+)">>\s*,\s*<<"([^"]+)">>\s*\}`)

// rebarOtherSourceRE matches a non-hex package tuple. Captures
// (alias, source-atom). Run after the pkg pattern so we only catch
// what the pkg regex didn't.
var rebarOtherSourceRE = regexp.MustCompile(`(?s)\{<<"([^"]+)">>\s*,\s*\{([a-z_][\w]*)\s*,`)

// rebarPkgHashSectionRE isolates the body of a `{pkg_hash, [ ... ]}`
// block so we can scan its entries without picking up other lists.
// The (?s) flag is needed because the section can wrap.
var rebarPkgHashSectionRE = regexp.MustCompile(`(?s)\{pkg_hash\s*,\s*\[(.*?)\]\s*\}`)

// rebarHashEntryRE matches a single hash binding inside the
// pkg_hash block: `{<<"alias">>, <<"HASH">>}`.
var rebarHashEntryRE = regexp.MustCompile(`\{<<"([^"]+)">>\s*,\s*<<"([^"]+)">>\s*\}`)

func (RebarLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}
	src := string(body)

	// Build alias → hash map from the pkg_hash section. Hashes are
	// keyed on the package alias the project used, not the registry
	// name — same convention rebar3 itself follows.
	hashByAlias := map[string]string{}
	if m := rebarPkgHashSectionRE.FindStringSubmatch(src); m != nil {
		for _, hm := range rebarHashEntryRE.FindAllStringSubmatch(m[1], -1) {
			hashByAlias[hm[1]] = hm[2]
		}
	}

	// Pull every hex-sourced entry. Track the matched byte ranges
	// so the second pass (non-hex) can skip them — otherwise the
	// permissive rebarOtherSourceRE would match every hex entry too
	// and produce false warnings.
	var hexSpans []matchSpan

	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	pkgAliases := map[string]bool{}
	for _, m := range rebarPkgEntryRE.FindAllStringSubmatchIndex(src, -1) {
		hexSpans = append(hexSpans, matchSpan{m[0], m[1]})
		alias := src[m[2]:m[3]]
		name := src[m[4]:m[5]]
		version := src[m[6]:m[7]]
		pkgAliases[alias] = true

		k := key{Name: name, Version: version}
		if seen[k] {
			continue
		}
		seen[k] = true
		pkgs = append(pkgs, ecosystems.PackageRef{
			Ecosystem: "hexpm",
			PackageID: name,
			Version:   version,
			Hash:      hashByAlias[alias],
		})
	}

	// Second pass: find non-hex entries. Skip any match whose start
	// falls inside an already-matched hex span (because the inner
	// `{pkg, ...}` tuple matches rebarOtherSourceRE too).
	for _, m := range rebarOtherSourceRE.FindAllStringSubmatchIndex(src, -1) {
		start := m[0]
		if insideAny(start, hexSpans) {
			continue
		}
		alias := src[m[2]:m[3]]
		source := src[m[4]:m[5]]
		// `pkg` shouldn't reach here (the first pass caught it),
		// and project aliases that double as package names would
		// already be in pkgAliases.
		if source == "pkg" || pkgAliases[alias] {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("rebar.lock: skipping %s (%s source)", alias, source))
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}

type matchSpan struct{ start, end int }

func insideAny(pos int, spans []matchSpan) bool {
	for _, s := range spans {
		if pos >= s.start && pos < s.end {
			return true
		}
	}
	return false
}
