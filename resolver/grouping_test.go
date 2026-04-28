package resolver

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.0.0", "v0.9.9", 1},
		{"v1.2.10", "v1.2.9", 1},  // numeric, not lex
		{"v2.0.0", "v10.0.0", -1}, // numeric, not lex
		{"v1.0.0-rc1", "v1.0.0", -1},
		{"v1.0.0", "v1.0.0-rc1", 1},
		{"v1.0.0-alpha", "v1.0.0-beta", -1},
	}
	for _, tc := range cases {
		got := CompareVersions(tc.a, tc.b)
		if (got < 0) != (tc.want < 0) || (got > 0) != (tc.want > 0) || (got == 0) != (tc.want == 0) {
			t.Errorf("CompareVersions(%q, %q) = %d, want sign %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestGroupVersions_AdjacentSameStatusCollapse(t *testing.T) {
	pv := PackageVerdict{
		Versions: map[string]Verdict{
			"v1.0.0": {Status: StatusVetted},
			"v1.0.1": {Status: StatusVetted},
			"v1.0.2": {Status: StatusVetted},
			"v1.0.3": {Status: StatusRejected},
			"v1.1.0": {Status: StatusVetted},
		},
	}
	groups := GroupVersions(pv)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d: %+v", len(groups), groups)
	}
	if groups[0].From != "v1.0.0" || groups[0].To != "v1.0.2" || groups[0].Status != StatusVetted {
		t.Errorf("group 0 = %+v", groups[0])
	}
	if groups[1].From != "v1.0.3" || groups[1].To != "v1.0.3" || groups[1].Status != StatusRejected {
		t.Errorf("group 1 = %+v", groups[1])
	}
	if groups[2].From != "v1.1.0" || groups[2].To != "v1.1.0" || groups[2].Status != StatusVetted {
		t.Errorf("group 2 = %+v", groups[2])
	}
}

func TestGroupVersions_NonSemverFallsThrough(t *testing.T) {
	// Non-semver strings shouldn't crash the comparator.
	pv := PackageVerdict{
		Versions: map[string]Verdict{
			"v1.0.0": {Status: StatusVetted},
			"main":   {Status: StatusVetted},
		},
	}
	groups := GroupVersions(pv)
	if len(groups) != 1 {
		t.Errorf("expected 1 group (both vetted), got %d", len(groups))
	}
}
