package pypi

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// version is the parsed PEP 440 form of a version string. -1 in the
// optional integer fields means "absent" so the comparator can
// distinguish "no post" from "post 0".
type version struct {
	epoch   int
	release []int
	preKind string // "", "a", "b", "rc"
	preNum  int
	postNum int // -1 if absent
	devNum  int // -1 if absent
	local   string
}

var (
	preRE          = regexp.MustCompile(`^[._-]?(alpha|beta|preview|pre|rc|a|b|c)[._-]?(\d+)`)
	postExplicitRE = regexp.MustCompile(`^[._-]?(post|rev|r)[._-]?(\d+)`)
	postImplicitRE = regexp.MustCompile(`^-(\d+)`)
	devRE          = regexp.MustCompile(`^[._-]?dev[._-]?(\d+)`)
	localRE        = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)
)

// parseVersion converts a PEP 440 version string into the structured
// form used for comparison. It is deliberately strict on suffix
// numbering — pip's implicit zero (e.g. "1.0a" → "1.0a0") is
// rejected here so the trust store doesn't end up with two visually-
// distinct strings that compare equal.
func parseVersion(in string) (*version, error) {
	if in == "" {
		return nil, fmt.Errorf("version is empty")
	}
	s := strings.ToLower(in)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return nil, fmt.Errorf("version %q has nothing after 'v'", in)
	}

	v := &version{postNum: -1, devNum: -1}

	// Local version label, if any (everything after the first '+').
	if i := strings.Index(s, "+"); i >= 0 {
		local := s[i+1:]
		s = s[:i]
		if local == "" {
			return nil, fmt.Errorf("version %q has empty local segment after '+'", in)
		}
		if !localRE.MatchString(local) {
			return nil, fmt.Errorf("version %q has invalid local segment %q", in, local)
		}
		v.local = local
	}

	// Epoch, if any: leading <digits>!.
	if i := strings.Index(s, "!"); i >= 0 {
		epochStr := s[:i]
		if epochStr == "" {
			return nil, fmt.Errorf("version %q has empty epoch before '!'", in)
		}
		n, err := strconv.Atoi(epochStr)
		if err != nil {
			return nil, fmt.Errorf("version %q has non-numeric epoch %q", in, epochStr)
		}
		v.epoch = n
		s = s[i+1:]
	}

	// Release: required, dot-separated digits. Stop at the first
	// non-digit so `1.0.post1` doesn't pull the trailing dot into
	// the release.
	end := 0
	for {
		segStart := end
		for end < len(s) && s[end] >= '0' && s[end] <= '9' {
			end++
		}
		if end == segStart {
			return nil, fmt.Errorf("version %q has empty release segment", in)
		}
		n, _ := strconv.Atoi(s[segStart:end])
		v.release = append(v.release, n)
		// Continue only if the next char is '.' AND the one after
		// is a digit — otherwise leave the dot for the suffix
		// matchers (`.post1`, `.dev1`).
		if end < len(s)-1 && s[end] == '.' && s[end+1] >= '0' && s[end+1] <= '9' {
			end++
			continue
		}
		break
	}
	if len(v.release) == 0 {
		return nil, fmt.Errorf("version %q has no release segment", in)
	}
	s = s[end:]

	// Pre-release.
	if m := preRE.FindStringSubmatch(s); m != nil {
		v.preKind = canonPreKind(m[1])
		n, _ := strconv.Atoi(m[2])
		v.preNum = n
		s = s[len(m[0]):]
	}

	// Post-release: explicit form first so a literal "-1" doesn't
	// snatch the dash separator from a "post" / "rev" / "r" tag.
	if m := postExplicitRE.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[2])
		v.postNum = n
		s = s[len(m[0]):]
	} else if m := postImplicitRE.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		v.postNum = n
		s = s[len(m[0]):]
	}

	// Dev.
	if m := devRE.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		v.devNum = n
		s = s[len(m[0]):]
	}

	if s != "" {
		return nil, fmt.Errorf("version %q has trailing junk %q", in, s)
	}
	return v, nil
}

// canonPreKind collapses the long aliases PEP 440 permits into the
// three canonical kinds used for comparison: a, b, rc.
func canonPreKind(s string) string {
	switch s {
	case "a", "alpha":
		return "a"
	case "b", "beta":
		return "b"
	case "c", "rc", "pre", "preview":
		return "rc"
	}
	return s
}

// comparePEP440 orders two version strings per PEP 440. Strings that
// fail to parse fall back to a lexicographic comparison so the
// comparator stays total — the resolver uses this for output ordering
// only, where surfacing a malformed version is preferable to panicking.
func comparePEP440(a, b string) int {
	pa, errA := parseVersion(a)
	pb, errB := parseVersion(b)
	switch {
	case errA != nil && errB != nil:
		return cmp.Compare(a, b)
	case errA != nil:
		return -1
	case errB != nil:
		return 1
	}
	return cmpVersion(pa, pb)
}

// Phase ordering within a release: dev-only < pre < final < post.
const (
	phaseDevOnly = 0
	phasePre     = 1
	phaseFinal   = 2
	phasePost    = 3
)

// classifyPhase maps a parsed version onto its PEP 440 phase. The
// phase determines which suffix fields contribute to the comparison
// once the release segment has been ruled equal.
func classifyPhase(v *version) int {
	switch {
	case v.preKind != "":
		return phasePre
	case v.postNum >= 0:
		return phasePost
	case v.devNum >= 0:
		return phaseDevOnly
	default:
		return phaseFinal
	}
}

func cmpVersion(a, b *version) int {
	if c := cmp.Compare(a.epoch, b.epoch); c != 0 {
		return c
	}
	if c := cmpRelease(a.release, b.release); c != 0 {
		return c
	}
	pa, pb := classifyPhase(a), classifyPhase(b)
	if pa != pb {
		return cmp.Compare(pa, pb)
	}
	switch pa {
	case phaseDevOnly:
		if c := cmp.Compare(a.devNum, b.devNum); c != 0 {
			return c
		}
	case phasePre:
		if c := cmp.Compare(preKindRank(a.preKind), preKindRank(b.preKind)); c != 0 {
			return c
		}
		if c := cmp.Compare(a.preNum, b.preNum); c != 0 {
			return c
		}
		// Absent dev (-1) sorts after any present dev — flip the
		// sentinel before comparing.
		if c := cmp.Compare(devForOrder(a.devNum), devForOrder(b.devNum)); c != 0 {
			return c
		}
	case phasePost:
		if c := cmp.Compare(a.postNum, b.postNum); c != 0 {
			return c
		}
		if c := cmp.Compare(devForOrder(a.devNum), devForOrder(b.devNum)); c != 0 {
			return c
		}
	}
	// Local segment is the final tiebreaker. Absent local sorts
	// before any present local.
	switch {
	case a.local == "" && b.local == "":
		return 0
	case a.local == "":
		return -1
	case b.local == "":
		return 1
	default:
		return cmpLocal(a.local, b.local)
	}
}

// preKindRank turns the canonical pre-release kind into a sortable
// integer: "a" < "b" < "rc". Anything unexpected sorts last.
func preKindRank(k string) int {
	switch k {
	case "a":
		return 0
	case "b":
		return 1
	case "rc":
		return 2
	}
	return 99
}

// devForOrder rewrites the dev-absent sentinel (-1) to a value that
// sorts after every real dev number, matching PEP 440's "no dev =
// after any dev" rule.
func devForOrder(n int) int {
	if n < 0 {
		return 1<<30 - 1
	}
	return n
}

// cmpRelease compares release tuples after stripping trailing zeros
// — PEP 440 treats 1.0 and 1.0.0 as equal, so "1.0" must compare
// equal to "1.0.0".
func cmpRelease(a, b []int) int {
	a = stripTrailingZeros(a)
	b = stripTrailingZeros(b)
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if c := cmp.Compare(av, bv); c != 0 {
			return c
		}
	}
	return 0
}

func stripTrailingZeros(seg []int) []int {
	end := len(seg)
	for end > 1 && seg[end-1] == 0 {
		end--
	}
	return seg[:end]
}

// cmpLocal compares local-version labels segment by segment. Per
// PEP 440 separators (`.`, `-`, `_`) are equivalent, numeric segments
// compare numerically, and a numeric segment outranks a string
// segment of the same position. The full PEP 440 rule mixes types
// asymmetrically; we stay simple and correct for the common case
// of either-all-numeric or either-all-string segments, which is what
// real local versions look like.
func cmpLocal(a, b string) int {
	as := splitLocal(a)
	bs := splitLocal(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		if i >= len(as) {
			return -1
		}
		if i >= len(bs) {
			return 1
		}
		ax, aIsNum := asLocalSegment(as[i])
		bx, bIsNum := asLocalSegment(bs[i])
		switch {
		case aIsNum && bIsNum:
			if c := cmp.Compare(ax.(int), bx.(int)); c != 0 {
				return c
			}
		case aIsNum:
			return 1
		case bIsNum:
			return -1
		default:
			if c := cmp.Compare(ax.(string), bx.(string)); c != 0 {
				return c
			}
		}
	}
	return 0
}

func splitLocal(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' || s[i] == '-' || s[i] == '_' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// asLocalSegment reports whether a local segment is numeric and
// returns either its int value or the original string.
func asLocalSegment(s string) (any, bool) {
	for _, r := range s {
		if r < '0' || r > '9' {
			return s, false
		}
	}
	n, _ := strconv.Atoi(s)
	return n, true
}
