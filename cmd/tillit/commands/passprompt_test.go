package commands

import "testing"

// withConfirmResponses installs a fake confirmReader that returns
// the given responses in order. See withPasswordResponses for the
// pattern.
func withConfirmResponses(t *testing.T, responses ...string) func() {
	t.Helper()
	original := confirmReader
	idx := 0
	confirmReader = func(prompt string) ([]byte, error) {
		if idx >= len(responses) {
			t.Fatalf("confirm prompt %q triggered with no response queued", prompt)
		}
		r := responses[idx]
		idx++
		return []byte(r), nil
	}
	return func() {
		confirmReader = original
	}
}

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
