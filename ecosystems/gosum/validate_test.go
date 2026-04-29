package gosum

import "testing"

func TestValidateVersion_Accepts(t *testing.T) {
	good := []string{
		"v0.0.0",
		"v1.2.3",
		"v1.2.10",
		"v2.0.0-rc1",
		"v1.0.0-alpha.2",
		"v0.0.0-20250101120000-abcdef123456", // pseudo-version
		"v1.0.0+build.1",
		"v1.2.3-pre+meta",
	}
	for _, v := range good {
		if err := (GoSum{}).ValidateVersion(v); err != nil {
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
		{"v", "no main"},
		{"3.0.0", "missing v prefix"},
		{"v.3.0.0", "leading dot in main"},
		{"v3..0", "empty segment"},
		{"v3.0.", "trailing dot"},
		{"v3.0.0-", "empty pre-release"},
		{"v3.0.0+", "empty build"},
		{"v3.a.0", "non-numeric main segment"},
		{"V1.2.3", "uppercase v"},
		{"v 1.2.3", "embedded space"},
	}
	for _, tc := range cases {
		if err := (GoSum{}).ValidateVersion(tc.in); err == nil {
			t.Errorf("ValidateVersion(%q) = nil, want error (%s)", tc.in, tc.reason)
		}
	}
}
