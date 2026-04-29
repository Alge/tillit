package commands

import "testing"

// withPasswordResponses installs a fake passwordReader that returns
// the given responses in order. Each prompt consumes one response;
// the test fails if more prompts arrive than responses queued.
// Returns a restore func to defer.
func withPasswordResponses(t *testing.T, responses ...string) func() {
	t.Helper()
	original := passwordReader
	idx := 0
	passwordReader = func(prompt string) ([]byte, error) {
		if idx >= len(responses) {
			t.Fatalf("password prompt %q triggered with no response queued", prompt)
		}
		r := responses[idx]
		idx++
		return []byte(r), nil
	}
	return func() {
		passwordReader = original
	}
}
