package evidenceverify

import (
	"context"
	"errors"
	"fmt"
)

// Kind classifies an error that stops a verification before a report exists.
type Kind int

// The kinds of Error. The command maps them to exit codes.
const (
	// KindInternal is an error that is none of the others.
	KindInternal Kind = iota + 1
	// KindUsage is an invalid option or argument.
	KindUsage
	// KindInvalid is a directory path that is refused before any report.
	KindInvalid
	// KindNotFound is a directory path that does not exist.
	KindNotFound
	// KindUnavailable is an I/O error that is not a refusal.
	KindUnavailable
	// KindTimeout is an expired deadline.
	KindTimeout
)

// Error is an error with a Kind.
type Error struct {
	Kind Kind
	Msg  string
	Err  error
}

// Error implements error.
func (e *Error) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

// Unwrap returns the cause.
func (e *Error) Unwrap() error { return e.Err }

func errf(k Kind, format string, args ...any) *Error {
	return &Error{Kind: k, Msg: fmt.Sprintf(format, args...)}
}

// KindOf returns the Kind of err: the Kind of an *Error in its chain, Timeout
// for an expired deadline, Internal for anything else. A nil error has no
// kind and returns 0.
func KindOf(err error) Kind {
	var e *Error
	switch {
	case err == nil:
		return 0
	case errors.As(err, &e):
		return e.Kind
	case errors.Is(err, context.DeadlineExceeded):
		return KindTimeout
	}
	return KindInternal
}

// ExitCode is the exit code of numi evidence verify (verification.md section
// 5): the report's when there is one, otherwise the code of the error's kind.
func ExitCode(rep *Report, err error) int {
	if err == nil && rep != nil {
		return rep.ExitCode()
	}
	switch KindOf(err) {
	case KindUsage:
		return 2
	case KindInvalid:
		return 3
	case KindNotFound:
		return 4
	case KindUnavailable:
		return 7
	case KindTimeout:
		return 8
	}
	return 1
}
