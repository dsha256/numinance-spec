package cli

import (
	"context"
	"errors"
	"fmt"
)

// Kind classifies a failure of a numi command. It mirrors the kernel error
// kinds without importing them: the engine build maps its own taxonomy to Kind
// through Validate.ErrorKind.
type Kind int

// The kinds. KindNone is success; any other value is reported as KindInternal.
const (
	KindNone Kind = iota
	KindInternal
	KindInvalid
	KindNotFound
	KindConflict
	KindPolicyDenied
	KindUnavailable
	KindTimeout
	// KindUsage is a bad flag or argument. It is last so the numbers of the
	// earlier kinds do not change.
	KindUsage
)

// Exit codes of numi. The table is stable: a code is never reused or renumbered
// in a release with the same major version.
//
//	0  OK
//	1  Internal, and any unknown kind
//	2  Usage (bad flags or arguments)
//	3  Invalid (includes floor violations)
//	4  NotFound
//	5  Conflict
//	6  PolicyDenied
//	7  Unavailable
//	8  Timeout
const (
	ExitOK           = 0
	ExitInternal     = 1
	ExitUsage        = 2
	ExitInvalid      = 3
	ExitNotFound     = 4
	ExitConflict     = 5
	ExitPolicyDenied = 6
	ExitUnavailable  = 7
	ExitTimeout      = 8
)

// ExitCode returns the process exit code of a kind. A value outside the
// declared kinds is ExitInternal.
func ExitCode(k Kind) int {
	switch k {
	case KindNone:
		return ExitOK
	case KindInvalid:
		return ExitInvalid
	case KindNotFound:
		return ExitNotFound
	case KindConflict:
		return ExitConflict
	case KindPolicyDenied:
		return ExitPolicyDenied
	case KindUnavailable:
		return ExitUnavailable
	case KindTimeout:
		return ExitTimeout
	case KindUsage:
		return ExitUsage
	}
	return ExitInternal
}

// Error is an error that carries its Kind. Sources and resolvers outside the
// engine build return it; DefaultErrorKind reads it.
type Error struct {
	Kind Kind
	Msg  string
}

// Errorf returns an *Error of the given kind.
func Errorf(k Kind, format string, args ...any) *Error {
	return &Error{Kind: k, Msg: fmt.Sprintf(format, args...)}
}

// Error implements error.
func (e *Error) Error() string { return e.Msg }

// DefaultErrorKind classifies err for a build without a kernel: an *Error
// carries its own kind, an expired context is KindTimeout and anything else,
// a canceled context included, KindInternal. Exit 7 is never produced for a
// cancel: 7 means "schema pass only" in the open build. A nil err is KindNone.
func DefaultErrorKind(err error) Kind {
	var e *Error
	switch {
	case err == nil:
		return KindNone
	case errors.As(err, &e):
		return e.Kind
	case errors.Is(err, context.DeadlineExceeded):
		return KindTimeout
	case errors.Is(err, context.Canceled):
		return KindInternal
	}
	return KindInternal
}
