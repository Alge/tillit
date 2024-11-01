package dberrors

import (
	"fmt"
)

// Base error, should not be used directly
type databaseError struct {
	Message string
}

// This should probably be shadowed, but is included here as a sane default
func (e *databaseError) Error() string {
	return e.Message
}

// Error for indicating a object matching the query could not be found
type ObjectNotFoundError struct {
	databaseError
}

func (err *ObjectNotFoundError) Error() string {
	return fmt.Sprintf("No such object found: %s", err.Message)
}

func NewObjectNotFoundError(message string) (err *ObjectNotFoundError) {
	err.Message = message
	return
}
