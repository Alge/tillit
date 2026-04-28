package gosum

import "testing"

func TestCompareGoVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.2.10", "v1.2.9", 1},  // numeric, not lex
		{"v2.0.0", "v10.0.0", -1}, // numeric, not lex
		{"v1.0.0-rc1", "v1.0.0", -1},
		{"v1.0.0", "v1.0.0-rc1", 1},
		{"v1.0.0-alpha", "v1.0.0-beta", -1},
		{"v1.0.0-rc1", "v1.0.0-rc2", -1},
		{"v1.0.0-rc.1", "v1.0.0-rc.2", -1},
		{"v0.0.0-20250101120000-abcdef", "v0.0.0-20250101120001-abcdef", -1},
		{"v0.0.0-20250101120000-abcdef", "v0.0.1", -1}, // pseudo < release
	}
	for _, tc := range cases {
		got := compareGoVersion(tc.a, tc.b)
		if (got < 0) != (tc.want < 0) || (got > 0) != (tc.want > 0) || (got == 0) != (tc.want == 0) {
			t.Errorf("compareGoVersion(%q, %q) = %d, want sign %d", tc.a, tc.b, got, tc.want)
		}
	}
}
