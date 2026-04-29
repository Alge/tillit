package rubygems

import "testing"

func TestValidateGemVersion_Accepts(t *testing.T) {
	good := []string{
		"0.0.0",
		"1.0.0",
		"1.2.3",
		"6.1.7.6",      // four-segment numerics are common in Rails-land
		"7.0.0.rc1",    // gem pre-release tag attached without a dot
		"1.0.0.pre",    // bare pre marker
		"1.0.0.pre.1",  // pre marker plus number
		"2.0.0.alpha",  // long alias
		"3.0.0.beta.5", // long alias with number
		"v1.0.0",       // tag-prefixed (rare but seen)
	}
	for _, v := range good {
		if err := validateGemVersion(v); err != nil {
			t.Errorf("validateGemVersion(%q) = %v, want nil", v, err)
		}
	}
}

func TestValidateGemVersion_Rejects(t *testing.T) {
	cases := []struct {
		in     string
		reason string
	}{
		{"", "empty"},
		{"1..0", "empty middle segment"},
		{"1.0.", "trailing dot"},
		{".1.0", "leading dot"},
		{"1.0-rc1", "dash separator (not gem-style)"},
		{"1.0 0", "embedded space"},
		{"1.0+build", "build metadata not part of gem version"},
		{"1.0_rc1", "underscore"},
	}
	for _, tc := range cases {
		if err := validateGemVersion(tc.in); err == nil {
			t.Errorf("validateGemVersion(%q) = nil, want error (%s)", tc.in, tc.reason)
		}
	}
}

func TestCompareGemVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// Equality and trailing zeros.
		{"1.0.0", "1.0.0", 0},
		{"1.0", "1.0.0", 0},     // gem's trailing-zero rule
		{"1.0.0", "1.0.0.0", 0}, // same — extra zero segments are no-ops
		{"v1.0.0", "1.0.0", 0},  // leading v is cosmetic

		// Plain numeric ordering.
		{"1.0.0", "1.0.1", -1},
		{"1.2.10", "1.2.9", 1}, // numeric, not lexical
		{"1.2.0", "1.10.0", -1},

		// Four-segment forms (Rails-style).
		{"6.1.7.6", "6.1.7.5", 1},
		{"6.1.7", "6.1.7.1", -1}, // longer non-zero numeric is greater

		// Pre-release < release.
		{"1.0.0.pre", "1.0.0", -1},
		{"1.0.0.alpha", "1.0.0", -1},
		{"1.0.0.beta", "1.0.0", -1},
		{"1.0.0.rc1", "1.0.0", -1},

		// Within pre-release: alphabetic order, then number.
		{"1.0.0.alpha", "1.0.0.beta", -1},
		{"1.0.0.beta", "1.0.0.rc1", -1},
		{"1.0.0.alpha", "1.0.0.alpha.1", -1},
		{"1.0.0.alpha.1", "1.0.0.alpha.2", -1},
	}
	for _, tc := range cases {
		got := compareGemVersion(tc.a, tc.b)
		if !sameSign(got, tc.want) {
			t.Errorf("compareGemVersion(%q, %q) = %d, want sign %d", tc.a, tc.b, got, tc.want)
		}
		if rev := compareGemVersion(tc.b, tc.a); !sameSign(rev, -tc.want) {
			t.Errorf("compareGemVersion(%q, %q) = %d, want %d (antisym)", tc.b, tc.a, rev, -tc.want)
		}
	}
}

func sameSign(got, want int) bool {
	switch {
	case want < 0:
		return got < 0
	case want > 0:
		return got > 0
	default:
		return got == 0
	}
}
