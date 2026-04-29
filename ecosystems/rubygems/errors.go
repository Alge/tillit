package rubygems

import "errors"

// Sentinel errors for the gem version validator. Kept compact so
// every check returns the same instance rather than freshly-formatted
// strings — handy when callers grep error messages.
var (
	errEmptyVersion = errors.New("version is empty")
	errEmptySegment = errors.New("version has empty dot-separated segment")
	errInvalidChar  = errors.New("version contains a character that isn't a digit or letter")
)
