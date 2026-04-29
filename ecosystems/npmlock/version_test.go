package npmlock

import "testing"

func TestValidateVersion_Accepts(t *testing.T) {
	good := []string{
		"1.2.3",
		"0.0.0",
		"10.20.30",
		"1.2.3-alpha",
		"1.2.3-alpha.1",
		"1.2.3-rc.1+build.5",
		"1.2.3+sha.deadbeef",
	}
	for _, v := range good {
		if err := (NpmLock{}).ValidateVersion(v); err != nil {
			t.Errorf("ValidateVersion(%q) = %v, want nil", v, err)
		}
	}
}

func TestValidateVersion_Rejects(t *testing.T) {
	cases := []struct {
		in     string
		reason string
	}{
		{"", "empty"},
		{"v1.2.3", "leading v not allowed in npm semver"},
		{"1.2", "missing patch"},
		{"1.2.3.4", "too many segments"},
		{"1.a.3", "non-numeric main"},
		{"1.2.3-", "empty pre-release"},
		{"1.2.3+", "empty build"},
		{"1..3", "empty middle segment"},
	}
	for _, tc := range cases {
		if err := (NpmLock{}).ValidateVersion(tc.in); err == nil {
			t.Errorf("ValidateVersion(%q) = nil, want error (%s)", tc.in, tc.reason)
		}
	}
}

func TestCompareVersions_NumericOrdering(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.2.10", "1.2.9", 1}, // numeric, not lexical
		{"1.2.0", "1.10.0", -1},
		{"2.0.0", "1.99.99", 1},
	}
	for _, tc := range cases {
		got := (NpmLock{}).CompareVersions(tc.a, tc.b)
		if (got > 0) != (tc.want > 0) || (got < 0) != (tc.want < 0) || (got == 0) != (tc.want == 0) {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCompareVersions_PreReleaseLowerThanRelease(t *testing.T) {
	if (NpmLock{}).CompareVersions("1.0.0-alpha", "1.0.0") >= 0 {
		t.Error("pre-release should sort BEFORE the release version per semver §11.3")
	}
	if (NpmLock{}).CompareVersions("1.0.0", "1.0.0-rc.1") <= 0 {
		t.Error("release should sort AFTER any of its pre-releases")
	}
}

func TestCompareVersions_PreReleaseIdentifierRules(t *testing.T) {
	// Per semver §11.4: numeric < non-numeric, fewer fields < more fields.
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0-alpha", "1.0.0-alpha.1", -1}, // shorter < longer
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1}, // numeric < non-numeric
		{"1.0.0-rc.1", "1.0.0-rc.2", -1}, // numeric ordering inside identifier
	}
	for _, tc := range cases {
		got := (NpmLock{}).CompareVersions(tc.a, tc.b)
		if (got > 0) != (tc.want > 0) || (got < 0) != (tc.want < 0) || (got == 0) != (tc.want == 0) {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCompareVersions_BuildMetadataIgnored(t *testing.T) {
	// Per semver §10: build metadata MUST be ignored when determining precedence.
	if (NpmLock{}).CompareVersions("1.2.3+a", "1.2.3+b") != 0 {
		t.Error("build metadata must not affect ordering")
	}
}
