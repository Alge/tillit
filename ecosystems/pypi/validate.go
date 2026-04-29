package pypi

import "fmt"

// ValidateVersion accepts the PEP 440 version forms the Python
// packaging ecosystem produces:
//
//   - Standard release: 1.0, 1.2.3 (with optional leading 'v')
//   - Epoch: 1!1.2.3
//   - Pre-release: 1.0a1, 1.0b1, 1.0rc1 (alpha/beta/c/pre/preview aliases)
//   - Implicit post: 1.0-1
//   - Explicit post: 1.0.post1
//   - Dev: 1.0.dev1
//   - Local: 1.0+local.tag
//
// The goal is to reject typos before a signature is recorded, not to
// pass the full PEP 440 grammar — anything close enough that pip
// accepts it must validate here. We deliberately reject pre-release
// segments without an explicit number (e.g. "1.0a"), since pip's
// implicit zero would silently equate to "1.0a0" and confuse users
// inspecting their trust store.
func (pypiCommon) ValidateVersion(v string) error {
	if v == "" {
		return fmt.Errorf("version is empty")
	}
	if _, err := parseVersion(v); err != nil {
		return err
	}
	return nil
}
