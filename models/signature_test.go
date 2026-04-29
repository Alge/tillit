package models

import "testing"

func TestSignatureID_Deterministic(t *testing.T) {
	payload := `{"type":"decision","signer":"abc","ecosystem":"go","package_id":"foo","version":"v1.0.0","level":"allowed"}`
	sig := "QUJDREVGRw"

	a := SignatureID(payload, sig)
	b := SignatureID(payload, sig)
	if a != b {
		t.Fatalf("SignatureID not deterministic: %q vs %q", a, b)
	}
	if a == "" {
		t.Fatal("SignatureID returned empty string")
	}
}

func TestSignatureID_DifferentSig(t *testing.T) {
	payload := `{"type":"decision","signer":"abc","ecosystem":"go","package_id":"foo","version":"v1.0.0","level":"allowed"}`
	if SignatureID(payload, "sigA") == SignatureID(payload, "sigB") {
		t.Fatal("different sigs produced same ID")
	}
}

func TestSignatureID_DifferentPayload(t *testing.T) {
	sig := "QUJDREVGRw"
	if SignatureID(`{"a":1}`, sig) == SignatureID(`{"a":2}`, sig) {
		t.Fatal("different payloads produced same ID")
	}
}

// SignatureID must not be susceptible to length-extension/concatenation
// ambiguity: payload="ab" + sig="cd" must not collide with payload="abc" + sig="d".
func TestSignatureID_NoConcatCollision(t *testing.T) {
	a := SignatureID("ab", "cd")
	b := SignatureID("abc", "d")
	if a == b {
		t.Fatalf("concat-ambiguity collision: %q == %q", a, b)
	}
}
