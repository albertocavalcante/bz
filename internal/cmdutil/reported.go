package cmdutil

import "errors"

// reportedError marks an error whose details the command has already shown to
// the user. The top-level runner exits non-zero without printing it again.
type reportedError struct{ err error }

func (e *reportedError) Error() string { return e.err.Error() }
func (e *reportedError) Unwrap() error { return e.err }

// AlreadyReported wraps err to signal that the command already printed its own
// failure output, so only the exit status remains to be reported.
// It returns nil when err is nil.
func AlreadyReported(err error) error {
	if err == nil {
		return nil
	}
	return &reportedError{err: err}
}

// IsAlreadyReported reports whether err (or any error it wraps) was marked
// with AlreadyReported.
func IsAlreadyReported(err error) bool {
	var r *reportedError
	return errors.As(err, &r)
}
