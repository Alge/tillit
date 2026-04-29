package pypi

import "testing"

func TestValidateVersion_Accepts(t *testing.T) {
	good := []string{
		"0",
		"1.0",
		"1.0.0",
		"1.2.3",
		"v1.2.3",    // PEP 440 allows a leading 'v'
		"1.0a1",     // pre-release: alpha
		"1.0.a1",    // dot separator
		"1.0-a1",    // dash separator
		"1.0_a1",    // underscore separator
		"1.0alpha1", // long alias for a
		"1.0beta2",  // long alias for b
		"1.0c1",     // 'c' alias for rc
		"1.0rc1",
		"1.0pre1",     // alias for rc
		"1.0preview1", // alias for rc
		"1.0.post1",
		"1.0-1", // implicit post-release
		"1.0.dev1",
		"1.0a1.dev1",
		"1.0.post1.dev1",
		"1!1.0",     // epoch
		"2!1.0.0a1", // epoch + pre-release
		"1.0+local",
		"1.0+local.1",
		"1.0+local-1.2",
		"1.0.0a0", // zero numbered pre
	}
	for _, v := range good {
		if err := (Requirements{}).ValidateVersion(v); err != nil {
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
		{"abc", "no numeric main"},
		{"1.0.x", "non-numeric main segment"},
		{"1..0", "empty numeric segment"},
		{"1.0.", "trailing dot in main"},
		{".1.0", "leading dot in main"},
		{"1.0xyz", "trailing junk"},
		{"1.0+", "empty local"},
		{"1.0+ ", "whitespace local"},
		{"1.0+!", "invalid local char"},
		{"1!", "empty release after epoch"},
		{"!1.0", "empty epoch"},
		{"1.0a", "pre-release without number ok actually — rejected here for strictness"},
	}
	// Note: PEP 440 strictly allows "1.0a" (number defaults to 0), but
	// for tillit we want to reject typos like that — every signature
	// records the version verbatim, and "1.0a" vs "1.0a0" comparing
	// equal would be confusing. Keep this in the reject list.
	for _, tc := range cases {
		if err := (Requirements{}).ValidateVersion(tc.in); err == nil {
			t.Errorf("ValidateVersion(%q) = nil, want error (%s)", tc.in, tc.reason)
		}
	}
}
