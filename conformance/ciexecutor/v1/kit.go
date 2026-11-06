// Package cikit is the runnable skeleton of the ciexecutor conformance kit, version 1.0.0 (README.md in this directory lists
// the full case table; this package runs the subset a provider can pass with the fixtures a publisher supplies). It is the
// acceptance harness for a provider of numinance.ports.ciexecutor.v1: the harness reaches the provider however it likes (a
// host dispatching Invoke, a replay, a direct call) through Target, and the cases judge only what comes back. It imports the
// public spec only.
package cikit

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"
)

// CallError is the error a Target returns for a call the provider answered with FAILED; Reason is the reason of the
// diagnostic it carried. Any other error means the harness could not get an answer.
type CallError struct{ Reason pluginv1.Reason }

// Error implements error.
func (f *CallError) Error() string { return "provider failed the call: " + f.Reason.String() }

// Options are what the host puts in the ActionContext of a call.
type Options struct {
	// DryRun is ActionContext.dry_run.
	DryRun bool
	// Key is ActionContext.idempotency_key.
	Key string
}

// Target is the provider under test.
type Target interface {
	StartOperation(context.Context, Options, *civ1.StartOperationRequest) (*civ1.StartOperationResponse, error)
	GetOperation(context.Context, Options, *civ1.GetOperationRequest) (*civ1.GetOperationResponse, error)
	CancelOperation(context.Context, Options, *civ1.CancelOperationRequest) (*civ1.CancelOperationResponse, error)
	ListArtifacts(context.Context, Options, *civ1.ListArtifactsRequest) (*civ1.ListArtifactsResponse, error)
	GetLogReference(context.Context, Options, *civ1.GetLogReferenceRequest) (*civ1.GetLogReferenceResponse, error)
}

// Subject is what a publisher supplies for the kit.
type Subject struct {
	Target Target
	// Repository exists and Missing does not.
	Repository, Missing string
	// Pipeline and Revision start a run with Params under StartKey; DryRunPipeline is another pipeline that only a dry run touches.
	Pipeline, DryRunPipeline, Revision, StartKey string
	Params                                       map[string]string
	// Progressing is a run that is running at the first look and has ended at the second; Cancellable is a run that is
	// running at the first look and that a cancel ends; WithArtifacts is an ended run that has artifacts and a log.
	Progressing, Cancellable, WithArtifacts string
}

// Case is one conformance case: an id of the README table and the check.
type Case struct {
	ID, Name string
	Check    func(ctx context.Context, s Subject) error
}

// Cases returns the cases this skeleton runs, in README order.
func Cases() []Case {
	return []Case{
		{"CORE-001", "StartOperation starts one run, and a repeat under the same key answers it", core001},
		{"CORE-002", "StartOperation of a repository that does not exist is NOT_FOUND", core002},
		{"CORE-003", "a request that breaks a bound is INVALID_ARGUMENT", core003},
		{"CORE-004", "StartOperation with dry run starts nothing", core004},
		{"CORE-005", "GetOperation: a valid state, the revision that was started, and an end time only after the run ends", core005},
		{"CORE-006", "CancelOperation ends a running run and a repeat answers its end state", core006},
		{"CORE-007", "ListArtifacts: names unique and bounded, digests well formed", core007},
		{"CORE-008", "GetLogReference: a bounded https URL", core008},
	}
}

// Run runs every case against the subject as subtests of t.
func Run(t *testing.T, s Subject) {
	t.Helper()
	for _, c := range Cases() {
		t.Run(c.ID, func(t *testing.T) {
			if err := c.Check(t.Context(), s); err != nil {
				t.Errorf("%s (%s): %v", c.ID, c.Name, err)
			}
		})
	}
}

func wantReason(err error, reason pluginv1.Reason) error {
	var f *CallError
	if !errors.As(err, &f) || f.Reason != reason {
		return fmt.Errorf("error = %w, want a FAILED call with reason %s", err, reason)
	}
	return nil
}

func (s Subject) start() *civ1.StartOperationRequest {
	return &civ1.StartOperationRequest{Repository: s.Repository, Pipeline: s.Pipeline, Revision: s.Revision, Params: s.Params}
}

func core001(ctx context.Context, s Subject) error {
	first, err := s.Target.StartOperation(ctx, Options{Key: s.StartKey}, s.start())
	if err != nil {
		return err
	}
	if !operationRe.MatchString(first.GetOperationId()) || first.GetExisting() || first.GetState() < civ1.OperationState_OPERATION_STATE_QUEUED || first.GetState() > civ1.OperationState_OPERATION_STATE_TIMED_OUT {
		return fmt.Errorf("start = %v, want a new run with a well formed id and a state of the enum", first)
	}
	again, err := s.Target.StartOperation(ctx, Options{Key: s.StartKey}, s.start())
	if err != nil {
		return err
	}
	if again.GetOperationId() != first.GetOperationId() || !again.GetExisting() {
		return fmt.Errorf("a repeat = %v, want run %s as existing: the pipeline was started twice", again, first.GetOperationId())
	}
	return nil
}

func core002(ctx context.Context, s Subject) error {
	req := s.start()
	req.Repository = s.Missing
	_, err := s.Target.StartOperation(ctx, Options{Key: s.StartKey}, req)
	return wantReason(err, pluginv1.Reason_REASON_NOT_FOUND)
}

func core003(ctx context.Context, s Subject) error {
	many := map[string]string{}
	for i := range 33 {
		many[fmt.Sprintf("p%02d", i)] = "v"
	}
	mut := map[string]func(*civ1.StartOperationRequest){
		"a short revision":          func(r *civ1.StartOperationRequest) { r.Revision = "abc" },
		"a branch for a revision":   func(r *civ1.StartOperationRequest) { r.Revision = "main" },
		"a pipeline that climbs":    func(r *civ1.StartOperationRequest) { r.Pipeline = "../x.yml" },
		"a reserved parameter":      func(r *civ1.StartOperationRequest) { r.Params = map[string]string{"numinance_revision": "x"} },
		"an upper case parameter":   func(r *civ1.StartOperationRequest) { r.Params = map[string]string{"Env": "x"} },
		"a parameter over its size": func(r *civ1.StartOperationRequest) { r.Params = map[string]string{"env": strings.Repeat("x", 1025)} },
		"more than 32 parameters":   func(r *civ1.StartOperationRequest) { r.Params = many },
		"a malformed repository":    func(r *civ1.StartOperationRequest) { r.Repository = "../x" },
	}
	for name, f := range mut {
		req := s.start()
		f(req)
		_, err := s.Target.StartOperation(ctx, Options{Key: s.StartKey}, req)
		if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
			return fmt.Errorf("%s: %w", name, e)
		}
	}
	for _, bad := range []string{"", "abc", "0", "1 2", strings.Repeat("9", 40)} {
		_, err := s.Target.GetOperation(ctx, Options{}, &civ1.GetOperationRequest{Repository: s.Repository, OperationId: bad})
		if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
			return fmt.Errorf("operation id %q: %w", bad, e)
		}
	}
	return nil
}

func core004(ctx context.Context, s Subject) error {
	req := s.start()
	req.Pipeline = s.DryRunPipeline
	resp, err := s.Target.StartOperation(ctx, Options{DryRun: true, Key: s.StartKey + "/dry"}, req)
	if err != nil {
		return err
	}
	if resp.GetOperationId() != "" {
		return fmt.Errorf("a dry run answered run %q, want none: it would start one", resp.GetOperationId())
	}
	return nil
}

func ended(st civ1.OperationState) bool {
	return st >= civ1.OperationState_OPERATION_STATE_SUCCEEDED && st <= civ1.OperationState_OPERATION_STATE_TIMED_OUT
}

func core005(ctx context.Context, s Subject) error {
	var last civ1.OperationState
	for i := range 2 {
		resp, err := s.Target.GetOperation(ctx, Options{}, &civ1.GetOperationRequest{Repository: s.Repository, OperationId: s.Progressing})
		if err != nil {
			return err
		}
		if err := ValidateOperation(resp); err != nil {
			return fmt.Errorf("look %d: %w", i+1, err)
		}
		if resp.GetRevision() != s.Revision {
			return fmt.Errorf("revision = %s, want the one that was started, %s", resp.GetRevision(), s.Revision)
		}
		last = resp.GetState()
	}
	if !ended(last) {
		return fmt.Errorf("state at the second look = %s, want an end state", last)
	}
	return nil
}

func core006(ctx context.Context, s Subject) error {
	req := &civ1.CancelOperationRequest{Repository: s.Repository, OperationId: s.Cancellable}
	for i := range 2 {
		resp, err := s.Target.CancelOperation(ctx, Options{Key: "kit-cancel"}, req)
		if err != nil {
			return err
		}
		if resp.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED {
			return fmt.Errorf("cancel %d = %s, want CANCELLED", i+1, resp.GetState())
		}
	}
	return nil
}

func core007(ctx context.Context, s Subject) error {
	resp, err := s.Target.ListArtifacts(ctx, Options{}, &civ1.ListArtifactsRequest{Repository: s.Repository, OperationId: s.WithArtifacts})
	if err != nil {
		return err
	}
	if len(resp.GetArtifacts()) == 0 || len(resp.GetArtifacts()) > 256 {
		return fmt.Errorf("artifacts = %d, want 1 to 256", len(resp.GetArtifacts()))
	}
	seen := map[string]bool{}
	for _, a := range resp.GetArtifacts() {
		if a.GetName() == "" || len(a.GetName()) > 256 || !isPrintable(a.GetName()) || seen[a.GetName()] || (a.GetDigest() != "" && !digestRe.MatchString(a.GetDigest())) {
			return errors.New("an artifact with no name, an unbounded or repeated name, or a digest that is not sha256:<hex>")
		}
		seen[a.GetName()] = true
	}
	return nil
}

func core008(ctx context.Context, s Subject) error {
	resp, err := s.Target.GetLogReference(ctx, Options{}, &civ1.GetLogReferenceRequest{Repository: s.Repository, OperationId: s.WithArtifacts})
	if err != nil {
		return err
	}
	if u := resp.GetUrl(); len(u) > 1024 || !strings.HasPrefix(u, "https://") || !isPrintable(u) {
		return errors.New("a log reference that is not a bounded https URL")
	}
	return nil
}

var (
	operationRe = regexp.MustCompile(`^[\x21-\x7e]{1,128}$`)
	revisionRe  = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	digestRe    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// ValidateOperation applies the response rules of GetOperation: a state of the enum, a full commit id and an end time only
// for a run that has ended.
func ValidateOperation(r *civ1.GetOperationResponse) error {
	switch {
	case r.GetState() < civ1.OperationState_OPERATION_STATE_QUEUED || r.GetState() > civ1.OperationState_OPERATION_STATE_TIMED_OUT:
		return errors.New("a state outside the enum")
	case !revisionRe.MatchString(r.GetRevision()):
		return errors.New("a revision that is not a full commit id")
	case (r.GetFinishedAt() != nil) != ended(r.GetState()):
		return errors.New("an end time that does not match the state")
	}
	return nil
}

func isPrintable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r != ' ' && !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsNumber(r) && !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			return false
		}
	}
	return true
}
