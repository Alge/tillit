package dberrors

import "testing"

func TestNewObjectNotFoundError(t *testing.T) {
	err := NewObjectNotFoundError("test message")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Error() != "No such object found: test message" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}
