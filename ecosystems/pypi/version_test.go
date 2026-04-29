package pypi

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// Equality.
		{"1.0", "1.0", 0},
		{"1.0", "1.0.0", 0},     // trailing zeros don't change order
		{"v1.2.3", "1.2.3", 0},  // leading 'v' is cosmetic
		{"1.0a1", "1.0.a.1", 0}, // separator variants normalize the same
		{"1.0alpha1", "1.0a1", 0},
		{"1.0beta2", "1.0b2", 0},
		{"1.0c1", "1.0rc1", 0},
		{"1.0pre1", "1.0rc1", 0},
		{"1.0preview1", "1.0rc1", 0},

		// Numeric ordering of release segments.
		{"1.0", "1.1", -1},
		{"1.2.3", "1.2.10", -1},
		{"1.10", "1.2", 1},

		// Pre-release < final.
		{"1.0a1", "1.0", -1},
		{"1.0b1", "1.0", -1},
		{"1.0rc1", "1.0", -1},

		// Within pre-release: a < b < rc.
		{"1.0a9", "1.0b1", -1},
		{"1.0b9", "1.0rc1", -1},

		// Pre-release number ordering.
		{"1.0a1", "1.0a2", -1},
		{"1.0a10", "1.0a2", 1},

		// Final < post.
		{"1.0", "1.0.post1", -1},
		{"1.0", "1.0-1", -1}, // implicit post

		// Post number ordering.
		{"1.0.post1", "1.0.post2", -1},

		// Dev-only is the very lowest within a release.
		{"1.0.dev0", "1.0a0", -1},
		{"1.0.dev0", "1.0", -1},
		{"1.0.dev1", "1.0.dev2", -1},

		// Dev makes a pre/post lower (pre.dev < pre).
		{"1.0a1.dev0", "1.0a1", -1},
		{"1.0.post1.dev0", "1.0.post1", -1},

		// Final < post.dev (post phase outranks final).
		{"1.0", "1.0.post1.dev0", -1},

		// Epoch dominates: any explicit epoch outranks the default 0.
		{"1!1.0", "2.0", 1},
		{"2!1.0", "1!2.0", 1},
		{"1!1.0", "1!1.0", 0},

		// Local versions sort after the same release without local.
		{"1.0", "1.0+local", -1},
		{"1.0+abc", "1.0+abd", -1},
	}
	for _, tc := range cases {
		got := (Requirements{}).CompareVersions(tc.a, tc.b)
		if !sameSign(got, tc.want) {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
		// Antisymmetry: cmp(a, b) == -cmp(b, a).
		if rev := (Requirements{}).CompareVersions(tc.b, tc.a); !sameSign(rev, -tc.want) {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d (antisym)", tc.b, tc.a, rev, -tc.want)
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
