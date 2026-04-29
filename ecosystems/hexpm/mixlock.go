package hexpm

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// MixLock is the adapter for Elixir's `mix.lock` files. It parses
// the Elixir-literal map at the top of the file, picks out hex-
// sourced entries, and warn-skips entries that point to git, path,
// or other non-registry sources.
//
// mix.lock is Elixir source: a `%{...}` map of `"key" => {tuple}`
// pairs (older format) or `"key": {tuple}` (newer). Each tuple's
// first three positions are `{:source_atom, :name_atom, "version"}`
// for hex packages; other source atoms (`:git`, `:path`, `:url`)
// signal non-vetable installs.
type MixLock struct{ hexpmCommon }

func (MixLock) Name() string { return "mix.lock" }

func (MixLock) CanParse(p string) bool {
	if p == "" {
		return false
	}
	return filepath.Base(p) == "mix.lock"
}

func (MixLock) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("read %s: %w", lockfilePath, err)
	}

	pkgs, warnings, err := parseMixLock(string(body))
	if err != nil {
		return ecosystems.ParseResult{}, err
	}
	return ecosystems.ParseResult{Packages: pkgs, Warnings: warnings}, nil
}

// hexEntryRE captures a hex-sourced entry's package atom and version
// from the first three tuple positions: `{:hex, :NAME, "VERSION"`.
// The (?s) flag lets the prefix wrap across lines, which real
// mix.lock entries usually do.
var hexEntryRE = regexp.MustCompile(`(?s)\A\s*\{\s*:hex\s*,\s*:([A-Za-z_][\w]*)\s*,\s*"([^"]+)"`)

// otherSourceRE captures the source atom for any other variant —
// :git, :path, :url, etc.
var otherSourceRE = regexp.MustCompile(`(?s)\A\s*\{\s*:([A-Za-z_][\w]*)`)

func parseMixLock(s string) ([]ecosystems.PackageRef, []string, error) {
	// Locate the outer `%{` map; everything we care about lives
	// between it and its matching `}`.
	outer := strings.Index(s, "%{")
	if outer < 0 {
		return nil, nil, fmt.Errorf("mix.lock: expected '%%{' to open the map")
	}
	openBrace := outer + 1 // index of the '{'
	closeBrace, err := matchBrace(s, openBrace)
	if err != nil {
		return nil, nil, fmt.Errorf("mix.lock: %w", err)
	}
	body := s[openBrace+1 : closeBrace]

	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	pos := 0
	for pos < len(body) {
		pos = skipSpace(body, pos)
		if pos >= len(body) {
			break
		}
		if body[pos] != '"' {
			// Stray character (most likely a trailing comma we
			// already consumed plus whitespace). Skip.
			pos++
			continue
		}
		keyEnd, ok := findStringEnd(body, pos)
		if !ok {
			return nil, nil, fmt.Errorf("mix.lock: unterminated key string at offset %d", pos)
		}
		entryKey := body[pos+1 : keyEnd]
		pos = keyEnd + 1

		pos = skipSpace(body, pos)
		// mix.lock keys are followed by either ':' (newer Elixir map
		// shorthand) or '=>' (older arrow form). Accept both.
		switch {
		case pos < len(body) && body[pos] == ':':
			pos++
		case pos+1 < len(body) && body[pos] == '=' && body[pos+1] == '>':
			pos += 2
		default:
			warnings = append(warnings, fmt.Sprintf("mix.lock: missing ':' or '=>' after key %q", entryKey))
			pos = advanceToTopComma(body, pos)
			continue
		}
		pos = skipSpace(body, pos)

		// The value should start with '{' for a tuple. Anything
		// else is malformed; warn and recover.
		if pos >= len(body) || body[pos] != '{' {
			warnings = append(warnings, fmt.Sprintf("mix.lock: %q is not a tuple value, skipping", entryKey))
			pos = advanceToTopComma(body, pos)
			continue
		}
		valueEnd, err := matchBrace(body, pos)
		if err != nil {
			return nil, nil, fmt.Errorf("mix.lock: value for %q: %w", entryKey, err)
		}
		value := body[pos : valueEnd+1]
		pos = valueEnd + 1

		// Skip trailing comma between entries.
		pos = skipSpace(body, pos)
		if pos < len(body) && body[pos] == ',' {
			pos++
		}

		if m := hexEntryRE.FindStringSubmatch(value); m != nil {
			name, version := m[1], m[2]
			k := key{Name: name, Version: version}
			if seen[k] {
				continue
			}
			seen[k] = true
			pkgs = append(pkgs, ecosystems.PackageRef{
				Ecosystem: "hexpm",
				PackageID: name,
				Version:   version,
			})
			continue
		}
		if m := otherSourceRE.FindStringSubmatch(value); m != nil {
			warnings = append(warnings, fmt.Sprintf("mix.lock: skipping %s (%s source)", entryKey, m[1]))
			continue
		}
		warnings = append(warnings, fmt.Sprintf("mix.lock: %q has unrecognised tuple shape, skipping", entryKey))
	}
	return pkgs, warnings, nil
}

// matchBrace returns the index of the '}' that closes the '{' at
// openIdx. It tracks string boundaries and escape sequences so a
// '{' or '}' inside a "..." literal doesn't throw off the depth.
func matchBrace(s string, openIdx int) (int, error) {
	if openIdx >= len(s) || s[openIdx] != '{' {
		return -1, fmt.Errorf("expected '{' at offset %d", openIdx)
	}
	depth := 0
	inString := false
	for i := openIdx; i < len(s); i++ {
		c := s[i]
		if inString {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return -1, fmt.Errorf("unmatched '{' starting at offset %d", openIdx)
}

// findStringEnd returns the index of the closing '"' for a string
// literal that opens at openIdx, respecting backslash escapes.
func findStringEnd(s string, openIdx int) (int, bool) {
	if openIdx >= len(s) || s[openIdx] != '"' {
		return -1, false
	}
	for i := openIdx + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 < len(s) {
				i++
			}
		case '"':
			return i, true
		}
	}
	return -1, false
}

// skipSpace advances past whitespace and `# ...` comments. Mix.lock
// is sometimes hand-edited; comments are unusual but legal Elixir.
func skipSpace(s string, pos int) int {
	for pos < len(s) {
		c := s[pos]
		switch {
		case c == ' ', c == '\t', c == '\n', c == '\r':
			pos++
		case c == '#':
			// Comment to end of line.
			for pos < len(s) && s[pos] != '\n' {
				pos++
			}
		default:
			return pos
		}
	}
	return pos
}

// advanceToTopComma moves the cursor past the next top-level comma
// so the loop can recover from a malformed entry.
func advanceToTopComma(s string, pos int) int {
	depth := 0
	inString := false
	for i := pos; i < len(s); i++ {
		c := s[i]
		if inString {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}
