package gosum

import "os"

// Tiny shims so resolve_test.go can stay unaware of the os package.
func osLookupEnv(k string) (string, bool) { return os.LookupEnv(k) }
func osSetenv(k, v string) error          { return os.Setenv(k, v) }
func osUnsetenv(k string) error           { return os.Unsetenv(k) }
