package pypi

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/Alge/tillit/ecosystems"
)

// Parse reads a requirements.txt-style file from fsys. Each fully
// pinned requirement (`name==version`) becomes one PackageRef. Loose
// specs, editable installs, VCS URLs, includes, and unparseable
// lines are surfaced as warnings rather than fatal errors so the user
// sees what was skipped.
//
// requirements.txt has no notion of direct vs. transitive dependencies
// (pip-compile output mixes both freely), so Direct stays false and
// the CLI renders a flat list rather than a tree.
func (Requirements) Parse(fsys fs.FS, lockfilePath string) (ecosystems.ParseResult, error) {
	f, err := fsys.Open(lockfilePath)
	if err != nil {
		return ecosystems.ParseResult{}, fmt.Errorf("open %s: %w", lockfilePath, err)
	}
	defer f.Close()

	pkgs, warnings, err := parseRequirements(f)
	if err != nil {
		return ecosystems.ParseResult{}, err
	}
	return ecosystems.ParseResult{
		Packages: pkgs,
		Warnings: warnings,
	}, nil
}

func parseRequirements(r io.Reader) ([]ecosystems.PackageRef, []string, error) {
	type key struct{ Name, Version string }
	seen := map[key]bool{}
	var pkgs []ecosystems.PackageRef
	var warnings []string

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var pending strings.Builder
	pendingLineNo := 0
	currentLine := 0

	flush := func(text string, lineNo int) {
		text = stripComment(text)
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		// Strip environment marker (`; python_version >= "3.8"`).
		if i := strings.Index(text, ";"); i >= 0 {
			text = strings.TrimSpace(text[:i])
		}
		// Drop trailing --hash=... and other inline options.
		text = stripInlineOptions(text)
		if text == "" {
			return
		}
		switch {
		case strings.HasPrefix(text, "-"):
			warnings = append(warnings, fmt.Sprintf("requirements.txt line %d: skipping option %q", lineNo, text))
			return
		case looksLikeURL(text):
			warnings = append(warnings, fmt.Sprintf("requirements.txt line %d: skipping URL/VCS install %q", lineNo, text))
			return
		}
		name, version, ok := splitPinnedRequirement(text)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("requirements.txt line %d: not a pinned requirement (need name==version): %q", lineNo, text))
			return
		}
		k := key{Name: name, Version: version}
		if seen[k] {
			return
		}
		seen[k] = true
		pkgs = append(pkgs, ecosystems.PackageRef{
			Ecosystem: "pypi",
			PackageID: name,
			Version:   version,
		})
	}

	for scanner.Scan() {
		currentLine++
		raw := scanner.Text()
		// Continuation: a trailing backslash joins with the next
		// physical line.
		if strings.HasSuffix(raw, "\\") {
			if pending.Len() == 0 {
				pendingLineNo = currentLine
			}
			pending.WriteString(strings.TrimSuffix(raw, "\\"))
			pending.WriteString(" ")
			continue
		}
		if pending.Len() > 0 {
			pending.WriteString(raw)
			flush(pending.String(), pendingLineNo)
			pending.Reset()
			pendingLineNo = 0
			continue
		}
		flush(raw, currentLine)
	}
	if pending.Len() > 0 {
		flush(pending.String(), pendingLineNo)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read requirements: %w", err)
	}
	return pkgs, warnings, nil
}

// stripComment drops everything from the first '#' onward.
// requirements.txt treats '#' as a line comment everywhere.
func stripComment(s string) string {
	if i := strings.Index(s, "#"); i >= 0 {
		return s[:i]
	}
	return s
}

// stripInlineOptions removes a trailing run of `--<flag>` arguments
// (notably `--hash=sha256:...`) that pip allows after the
// requirement itself. The first whitespace-separated field that
// starts with `--` and everything after it is dropped.
func stripInlineOptions(s string) string {
	fields := strings.Fields(s)
	for i, f := range fields {
		if strings.HasPrefix(f, "--") {
			return strings.Join(fields[:i], " ")
		}
	}
	return s
}

// splitPinnedRequirement extracts (name, version) from a `name==version`
// line. Returns ok=false for any spec that isn't a single `==` pin.
// Extras (`name[extra]==1.0`) are stripped before returning the name.
func splitPinnedRequirement(s string) (name, version string, ok bool) {
	idx := strings.Index(s, "==")
	if idx < 0 {
		return "", "", false
	}
	rawName := strings.TrimSpace(s[:idx])
	version = strings.TrimSpace(s[idx+2:])
	if rawName == "" || version == "" {
		return "", "", false
	}
	// `===` (PEP 440 arbitrary equality) leaves a leading '=' in
	// version after splitting on the first `==`. Treat as
	// not-a-normal-pin so the caller surfaces a warning.
	if strings.HasPrefix(version, "=") {
		return "", "", false
	}
	if i := strings.Index(rawName, "["); i >= 0 {
		rawName = strings.TrimSpace(rawName[:i])
	}
	if rawName == "" {
		return "", "", false
	}
	return normalizePackageName(rawName), version, true
}

// normalizePackageName applies PEP 503 normalization: lowercase, and
// runs of [-_.] collapsed to a single hyphen. The trust store keys on
// this so `Django`, `django`, and `DJANGO` share decisions.
func normalizePackageName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSep := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			prevSep = false
		case r == '-' || r == '_' || r == '.':
			if !prevSep && b.Len() > 0 {
				b.WriteByte('-')
				prevSep = true
			}
		default:
			b.WriteRune(r)
			prevSep = false
		}
	}
	return strings.TrimRight(b.String(), "-")
}

func looksLikeURL(s string) bool {
	for _, prefix := range []string{
		"http://", "https://", "ftp://", "file://",
		"git+", "hg+", "svn+", "bzr+",
	} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
