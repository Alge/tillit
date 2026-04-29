package semver

import "testing"

func TestValidate_Accepts(t *testing.T) {
	good := []string{
		"0.0.0",
		"1.0.0",
		"1.2.3",
		"1.2.10",
		"10.20.30",
		"2.0.0-alpha",
		"2.0.0-alpha.1",
		"2.0.0-rc.1",
		"1.0.0-0.3.7", // numeric pre-release idents with no leading zero
		"1.2.3-pre.alpha",
		"1.0.0+20240101",
		"1.0.0-rc.1+build.7",
		"1.2.3+sha.deadbeef",
	}
	for _, v := range good {
		if err := Validate(v); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", v, err)
		}
	}
}

func TestValidate_Rejects(t *testing.T) {
	cases := []struct {
		in     string
		reason string
	}{
		{"", "empty"},
		{"1", "missing minor and patch"},
		{"1.0", "missing patch"},
		{"v1.0.0", "leading 'v' not part of strict semver"},
		{"1.0.0.0", "four numeric segments"},
		{"1.0.x", "non-numeric main segment"},
		{"1.0.0-", "empty pre-release after dash"},
		{"1.0.0+", "empty build after plus"},
		{"1.0.0-01", "numeric pre-release identifier with leading zero"},
		{"01.0.0", "leading zero in major"},
		{"1.0.0-rc..1", "empty pre-release identifier"},
		{"1..3", "empty middle segment"},
	}
	for _, tc := range cases {
		if err := Validate(tc.in); err == nil {
			t.Errorf("Validate(%q) = nil, want error (%s)", tc.in, tc.reason)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// Equality.
		{"1.0.0", "1.0.0", 0},
		{"1.0.0+build1", "1.0.0+build2", 0}, // build metadata ignored

		// Numeric ordering.
		{"1.0.0", "1.0.1", -1},
		{"1.2.3", "1.2.10", -1},
		{"1.10.0", "1.2.0", 1},
		{"2.0.0", "1.99.99", 1},

		// Pre-release < release.
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0-rc.1", "1.0.0", -1},

		// Pre-release ordering: alphabetic vs numeric, length.
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1}, // numeric < alphanumeric
		{"1.0.0-alpha.beta", "1.0.0-beta", -1},
		{"1.0.0-beta", "1.0.0-beta.2", -1},
		{"1.0.0-beta.2", "1.0.0-beta.11", -1}, // numeric ident compared numerically
		{"1.0.0-rc.1", "1.0.0-rc.2", -1},
	}
	for _, tc := range cases {
		got := Compare(tc.a, tc.b)
		if !sameSign(got, tc.want) {
			t.Errorf("Compare(%q, %q) = %d, want sign %d", tc.a, tc.b, got, tc.want)
		}
		if rev := Compare(tc.b, tc.a); !sameSign(rev, -tc.want) {
			t.Errorf("Compare(%q, %q) = %d, want %d (antisym)", tc.b, tc.a, rev, -tc.want)
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
