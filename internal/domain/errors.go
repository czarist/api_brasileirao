package domain

import "errors"

var (
	// ErrNotFound is returned when a competition, season, or match does not exist.
	ErrNotFound = errors.New("not found")
	// ErrInvalid is a caller or payload error that is safe to describe.
	ErrInvalid = errors.New("invalid")
	// ErrIncomplete is a collection that parsed but is too small to publish.
	ErrIncomplete = errors.New("incomplete collection")
	// ErrBusy means another sync holds the advisory lock.
	ErrBusy = errors.New("sync already running")
	// ErrUnavailable means the database cannot serve the request.
	ErrUnavailable = errors.New("database is unavailable")
)

// ValidationError is a parameter error with a client-safe message.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) Unwrap() error { return ErrInvalid }
