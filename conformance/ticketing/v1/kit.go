// Package ticketingkit is the runnable replay conformance kit for a provider of numinance.ports.ticketing.v1, version 1.2.0
// (README.md in this directory lists the case table). It is the acceptance harness for a ticketing provider: the harness
// reaches the provider however it likes (a host dispatching Invoke, a replay, a direct call) through Target, and the cases
// judge only what comes back. It imports the public spec only.
//
// The kit is generic: a publisher supplies a Subject, the project and issues its provider can be asked about. The response
// rules (ValidateIssue, ValidateSummary) are the port's own and are also what a host enforces on any provider. The package
// also holds the fixture scan of ADR 0060 decision 10 (ScanFixtures), which keeps real hosts and tenant names out of the
// public mirror.
package ticketingkit

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"
)

// EvidenceType is the one record type every write verb declares.
const EvidenceType = "ticketing-action"

// CallError is the error a Target returns for a call the provider answered with FAILED; Reason is the reason of the
// diagnostic it carried and Message its text. Any other error means the harness could not get an answer.
type CallError struct {
	Reason  pluginv1.Reason
	Message string
}

// Error implements error.
func (f *CallError) Error() string { return "provider failed the call: " + f.Reason.String() }

// Options are what the host puts in the ActionContext of a call.
type Options struct {
	// DryRun is ActionContext.dry_run.
	DryRun bool
	// Key is ActionContext.idempotency_key.
	Key string
}

// Evidence is one record a provider emitted through HostService.EmitEvidence during a call.
type Evidence struct {
	Type, ID string
	// Fields are the other values of the record, flattened to strings (the verb, the project, the issue key, the result).
	Fields []string
}

// Target is the provider under test.
type Target interface {
	GetIssue(context.Context, Options, *ticketingv1.GetIssueRequest) (*ticketingv1.GetIssueResponse, error)
	SearchIssues(context.Context, Options, *ticketingv1.SearchIssuesRequest) (*ticketingv1.SearchIssuesResponse, error)
	CommentOnIssue(context.Context, Options, *ticketingv1.CommentOnIssueRequest) (*ticketingv1.CommentOnIssueResponse, []Evidence, error)
	TransitionIssue(context.Context, Options, *ticketingv1.TransitionIssueRequest) (*ticketingv1.TransitionIssueResponse, []Evidence, error)
	LinkIssue(context.Context, Options, *ticketingv1.LinkIssueRequest) (*ticketingv1.LinkIssueResponse, []Evidence, error)
}

// Subject is what a publisher supplies for the kit. Every issue key is a key of Project unless stated.
type Subject struct {
	Target Target
	// Project exists. OtherProject is another project the credential can see.
	Project, OtherProject string
	// Found is an issue in the state FoundState, carrying exactly the operation ids FoundOperationIDs.
	Found             string
	FoundState        ticketingv1.IssueStateCategory
	FoundOperationIDs []string
	// Long is an issue whose body is at least 64 bytes and holds a multi-byte character within its first 32 bytes.
	Long string
	// Dirty is an issue whose raw text in the tracker holds control, bidirectional and zero-width characters in its
	// title, body, type, a label and an operation id.
	Dirty string
	// Moved was a key of Project and is now an issue of OtherProject, under the same key there.
	Moved string
	// Hidden exists but the credential may not see it; Missing does not exist.
	Hidden, Missing string
	// SearchText matches at least three OPEN issues of Project and at least one DONE issue of Project, and also
	// matches OtherProjectHit, an issue of OtherProject.
	SearchText      string
	OtherProjectHit string
	// TransitionOpen is an OPEN issue from which DONE, IN_PROGRESS and OPEN are reachable.
	TransitionOpen string
	// TransitionStuck is an issue from which UnreachableTarget is not reachable.
	TransitionStuck   string
	UnreachableTarget ticketingv1.IssueStateCategory
	// LinkBlocker and LinkBlocked are two issues of Project with no link between them.
	LinkBlocker, LinkBlocked string
	// CrossBlocked is an issue of OtherProject with no link from LinkBlocker.
	CrossBlocked string
	// ForbiddenProject is a project the credential may not use at all, and ForbiddenIssue an issue in it.
	ForbiddenProject, ForbiddenIssue string
	// HiddenMatch is an issue of Project that the credential may not see and that matches SearchText; HiddenMatchTitle is
	// a word of its title that no visible issue holds.
	HiddenMatch, HiddenMatchTitle string
	// TrackerTexts are titles and body words of the fixture issues (the hidden and moved ones included). No error message
	// may contain any of them.
	TrackerTexts []string
}

// Case is one conformance case: an id of the README table and the check.
type Case struct {
	ID, Name string
	Check    func(ctx context.Context, s Subject) error
}

// Cases returns the cases this kit runs, in README order.
func Cases() []Case {
	return []Case{
		{"TICKETING-001", "GetIssue: an issue within the port's bounds", t001},
		{"TICKETING-002", "GetIssue of an issue that does not exist is NOT_FOUND", t002},
		{"TICKETING-003", "GetIssue of an issue that moved to another project is NOT_FOUND under the old project", t003},
		{"TICKETING-004", "no permission and does not exist are one answer", t004},
		{"TICKETING-005", "the body is bounded by max_body_bytes and truncated says so", t005},
		{"TICKETING-006", "the state category is the issue's and filters a search", t006},
		{"TICKETING-007", "no control, bidirectional or zero-width character is returned", t007},
		{"TICKETING-008", "the operation ids recorded on the issue are returned as they are", t008},
		{"TICKETING-009", "a malformed project, key or bound is INVALID_ARGUMENT", t009},
		{"TICKETING-010", "SearchIssues is bounded by max_results", t010},
		{"TICKETING-011", "the search text is a parameter, never a query", t011},
		{"TICKETING-012", "CommentOnIssue under one key adds one comment", t012},
		{"TICKETING-013", "CommentOnIssue: bounds, and hidden or moved issues are NOT_FOUND", t013},
		{"TICKETING-014", "TransitionIssue: dry run changes nothing, the call reports the category before, a repeat is done", t014},
		{"TICKETING-015", "TransitionIssue to an unreachable category is a conflict; a bad target is INVALID_ARGUMENT", t015},
		{"TICKETING-016", "LinkIssue is idempotent and refuses a self link", t016},
		{"TICKETING-017", "every write emits one ticketing-action record, a dry run none", t017},
		{"TICKETING-018", "the port declares risk, dry run, evidence and the resource marks as the port requires", t018},
		{"TICKETING-019", "malformed and hostile input is INVALID_ARGUMENT on every verb and never echoed", t019},
		{"TICKETING-020", "SearchIssues never lists, counts or names an issue the credential cannot see", t020},
		{"TICKETING-021", "LinkIssue is authorised against both projects", t021},
		{"TICKETING-022", "an evidence record carries no title, body or comment text", t022},
		{"TICKETING-023", "a key reused for another issue does not answer the first issue's comment", t023},
		{"TICKETING-024", "a comment is sent as plain text with no active mention", t024},
	}
}

// ErrNotApplicable is returned (wrapped) by a case that cannot run against this Target; Run reports the case as skipped
// with the reason, never as passed.
var ErrNotApplicable = errors.New("case not applicable to this target")

// Run runs every case against the subject as subtests of t.
func Run(t *testing.T, s Subject) {
	t.Helper()
	for _, c := range Cases() {
		t.Run(c.ID, func(t *testing.T) {
			err := c.Check(t.Context(), s)
			if errors.Is(err, ErrNotApplicable) {
				t.Skipf("%s (%s): %v", c.ID, c.Name, err)
			}
			if err != nil {
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

// size converts a length the kit measured itself (at most a few hundred bytes) to a request bound.
func size(n int) uint32 { return uint32(n) } //nolint:gosec // the lengths are measured from fixtures, far below 2^32

const (
	maxBody = 65536
	small   = 4096
)

func get(ctx context.Context, s Subject, project, key string, limit uint32) (*ticketingv1.GetIssueResponse, error) {
	return s.Target.GetIssue(ctx, Options{}, &ticketingv1.GetIssueRequest{Project: project, IssueKey: key, MaxBodyBytes: limit})
}

func t001(ctx context.Context, s Subject) error {
	r, err := get(ctx, s, s.Project, s.Found, small)
	if err != nil {
		return err
	}
	if r.GetIssueKey() != s.Found || r.GetProject() != s.Project {
		return fmt.Errorf("answer is for %s/%s, want %s/%s", r.GetProject(), r.GetIssueKey(), s.Project, s.Found)
	}
	return ValidateIssue(r, small)
}

func t002(ctx context.Context, s Subject) error {
	_, err := get(ctx, s, s.Project, s.Missing, small)
	_, e := notFound(err, s.secrets(s.Project, s.Missing)...)
	return e
}

func t003(ctx context.Context, s Subject) error {
	_, err := get(ctx, s, s.Project, s.Moved, small)
	if _, e := notFound(err, s.secrets(s.Project, s.Moved)...); e != nil {
		return fmt.Errorf("under the old project: %w", e)
	}
	r, err := get(ctx, s, s.OtherProject, s.Moved, small)
	if err != nil {
		return fmt.Errorf("under the new project: %w", err)
	}
	if r.GetProject() != s.OtherProject {
		return fmt.Errorf("project = %q, want the issue's current project %q", r.GetProject(), s.OtherProject)
	}
	return nil
}

func t004(ctx context.Context, s Subject) error {
	probes := s.notFoundProbes(ctx)
	first := ""
	for i, p := range probes {
		msg, err := notFound(p.err, s.secrets(p.project, p.key)...)
		if err != nil {
			return fmt.Errorf("%s: %w", p.name, err)
		}
		if i == 0 {
			first = msg
		}
		if msg != first {
			return fmt.Errorf("%s answered %q and %s answered %q, want one byte-identical message for every NOT_FOUND: the port must not tell a missing, a hidden, a moved, a wrong-project and a forbidden-project issue apart",
				probes[0].name, first, p.name, msg)
		}
	}
	return nil
}

func t005(ctx context.Context, s Subject) error {
	whole, err := get(ctx, s, s.Project, s.Long, maxBody)
	if err != nil {
		return err
	}
	full := whole.GetBody()
	if whole.GetTruncated() || len(full) < 64 {
		return fmt.Errorf("max_body_bytes %d answered truncated=%v and %d body bytes, want the whole body", maxBody, whole.GetTruncated(), len(full))
	}
	exact, err := get(ctx, s, s.Project, s.Long, size(len(full)))
	if err != nil {
		return err
	}
	if exact.GetTruncated() || exact.GetBody() != full {
		return fmt.Errorf("max_body_bytes equal to the body (%d) answered truncated=%v, want the whole body and not truncated", len(full), exact.GetTruncated())
	}
	under, err := get(ctx, s, s.Project, s.Long, size(len(full)-1))
	if err != nil {
		return err
	}
	if !under.GetTruncated() {
		return errors.New("max_body_bytes one under the body length answered truncated=false")
	}
	for limit := 1; limit <= 40; limit++ {
		cut, err := get(ctx, s, s.Project, s.Long, uint32(limit))
		if err != nil {
			return err
		}
		wantLen := 0
		for i := range full {
			if i <= limit {
				wantLen = i
			}
		}
		if !cut.GetTruncated() || !utf8.ValidString(cut.GetBody()) || len(cut.GetBody()) != wantLen || !strings.HasPrefix(full, cut.GetBody()) {
			return fmt.Errorf("max_body_bytes %d answered truncated=%v and %d bytes (valid UTF-8 %v), want truncated and the longest prefix of %d bytes that ends at a character boundary",
				limit, cut.GetTruncated(), len(cut.GetBody()), utf8.ValidString(cut.GetBody()), wantLen)
		}
	}
	return nil
}

func t006(ctx context.Context, s Subject) error {
	r, err := get(ctx, s, s.Project, s.Found, small)
	if err != nil {
		return err
	}
	if r.GetState() != s.FoundState {
		return fmt.Errorf("state = %s, want %s", r.GetState(), s.FoundState)
	}
	for _, st := range []ticketingv1.IssueStateCategory{ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE} {
		res, err := s.Target.SearchIssues(ctx, Options{}, &ticketingv1.SearchIssuesRequest{Project: s.Project, Text: s.SearchText, State: st, MaxResults: 50})
		if err != nil {
			return err
		}
		if len(res.GetIssues()) == 0 {
			return fmt.Errorf("a search in state %s found nothing, want the fixture's issues", st)
		}
		for _, h := range res.GetIssues() {
			if h.GetState() != st {
				return fmt.Errorf("a search in state %s returned %s in state %s", st, h.GetIssueKey(), h.GetState())
			}
		}
	}
	return nil
}

func t007(ctx context.Context, s Subject) error {
	for _, key := range []string{s.Dirty, s.Found} {
		r, err := get(ctx, s, s.Project, key, maxBody)
		if err != nil {
			return err
		}
		if err := ValidateIssue(r, maxBody); err != nil {
			return fmt.Errorf("issue %s: %w", key, err)
		}
		strs := append([]string{r.GetTitle(), r.GetIssueType()}, r.GetLabels()...)
		strs = append(strs, r.GetOperationIds()...)
		for _, v := range strs {
			if bad, ok := forbidden(v, false); ok {
				return fmt.Errorf("issue %s: a string holds %U, which a provider strips", key, bad)
			}
		}
		if bad, ok := forbidden(r.GetBody(), true); ok {
			return fmt.Errorf("issue %s: the body holds %U, which a provider strips", key, bad)
		}
	}
	return nil
}

func t008(ctx context.Context, s Subject) error {
	r, err := get(ctx, s, s.Project, s.Found, small)
	if err != nil {
		return err
	}
	if !equalStrings(r.GetOperationIds(), s.FoundOperationIDs) {
		return fmt.Errorf("operation_ids = %q, want %q", r.GetOperationIds(), s.FoundOperationIDs)
	}
	return nil
}

func t009(ctx context.Context, s Subject) error {
	badProjects := []string{"", "../x", "a b", "-a", strings.Repeat("a", 101)}
	badKeys := []string{"", "PAY 1", "PAY#1", "-1", strings.Repeat("a", 65)}
	for _, p := range badProjects {
		_, err := get(ctx, s, p, s.Found, small)
		if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
			return fmt.Errorf("project %q: %w", p, e)
		}
	}
	for _, k := range badKeys {
		_, err := get(ctx, s, s.Project, k, small)
		if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
			return fmt.Errorf("key %q: %w", k, e)
		}
	}
	for _, m := range []uint32{0, maxBody + 1} {
		_, err := get(ctx, s, s.Project, s.Found, m)
		if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
			return fmt.Errorf("max_body_bytes %d: %w", m, e)
		}
	}
	return nil
}

func search(ctx context.Context, s Subject, text string, limit uint32) (*ticketingv1.SearchIssuesResponse, error) {
	return s.Target.SearchIssues(ctx, Options{}, &ticketingv1.SearchIssuesRequest{Project: s.Project, Text: text, MaxResults: limit})
}

func t010(ctx context.Context, s Subject) error {
	for _, m := range []uint32{0, 51} {
		_, err := search(ctx, s, s.SearchText, m)
		if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
			return fmt.Errorf("max_results %d: %w", m, e)
		}
	}
	_, err := search(ctx, s, strings.Repeat("a", 257), 10)
	if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
		return fmt.Errorf("text of 257 bytes: %w", e)
	}
	two, err := search(ctx, s, s.SearchText, 2)
	if err != nil {
		return err
	}
	if len(two.GetIssues()) != 2 || !two.GetTruncated() {
		return fmt.Errorf("max_results 2 answered %d hits, truncated=%v, want 2 hits and truncated", len(two.GetIssues()), two.GetTruncated())
	}
	all, err := search(ctx, s, s.SearchText, 50)
	if err != nil {
		return err
	}
	if len(all.GetIssues()) < 4 || all.GetTruncated() {
		return fmt.Errorf("max_results 50 answered %d hits, truncated=%v, want every match and not truncated", len(all.GetIssues()), all.GetTruncated())
	}
	for _, h := range all.GetIssues() {
		if err := ValidateSummary(h); err != nil {
			return err
		}
		if h.GetIssueKey() == s.OtherProjectHit {
			return fmt.Errorf("a search of %s returned %s, an issue of %s", s.Project, h.GetIssueKey(), s.OtherProject)
		}
	}
	return nil
}

func t011(ctx context.Context, s Subject) error {
	for _, text := range []string{`project = "` + s.OtherProject + `" OR 1=1`, `" OR ""="`, `*`, `~`, `) OR (`, `text ~ "` + s.SearchText + `"`} {
		r, err := search(ctx, s, text, 50)
		if err != nil {
			return fmt.Errorf("text %q: %w", text, err)
		}
		if len(r.GetIssues()) != 0 {
			return fmt.Errorf("text %q matched %d issues, want none: the text must be matched as plain text and never read as a query", text, len(r.GetIssues()))
		}
	}
	return nil
}

func comment(ctx context.Context, s Subject, key, body string) (*ticketingv1.CommentOnIssueResponse, []Evidence, error) {
	return s.Target.CommentOnIssue(ctx, Options{Key: key}, &ticketingv1.CommentOnIssueRequest{Project: s.Project, IssueKey: s.Found, Body: body})
}

func t012(ctx context.Context, s Subject) error {
	first, _, err := comment(ctx, s, "kit-comment-1", "Checks passed.")
	if err != nil {
		return err
	}
	again, _, err := comment(ctx, s, "kit-comment-1", "Checks passed.")
	if err != nil {
		return err
	}
	if first.GetCommentId() == "" || first.GetCommentId() != again.GetCommentId() || first.GetExisting() || !again.GetExisting() {
		return fmt.Errorf("comments (%q, existing %v) and (%q, existing %v), want one comment id, then existing: a comment was posted twice",
			first.GetCommentId(), first.GetExisting(), again.GetCommentId(), again.GetExisting())
	}
	other, _, err := comment(ctx, s, "kit-comment-2", "Checks passed.")
	if err != nil {
		return err
	}
	if other.GetCommentId() == first.GetCommentId() || other.GetExisting() {
		return errors.New("another key answered the first comment, want a new comment")
	}
	return nil
}

func t013(ctx context.Context, s Subject) error {
	for _, body := range []string{"", strings.Repeat("a", 8193)} {
		_, _, err := comment(ctx, s, "kit-comment-bound", body)
		if e := wantInvalid(err, body); e != nil {
			return fmt.Errorf("body of %d bytes: %w", len(body), e)
		}
	}
	for _, key := range []string{s.Hidden, s.Moved, s.Missing} {
		_, _, err := s.Target.CommentOnIssue(ctx, Options{Key: "kit-comment-nf"}, &ticketingv1.CommentOnIssueRequest{Project: s.Project, IssueKey: key, Body: "x"})
		if _, e := notFound(err, s.secrets(s.Project, key)...); e != nil {
			return fmt.Errorf("issue %s: %w", key, e)
		}
	}
	return nil
}

func transition(ctx context.Context, s Subject, o Options, key string, target ticketingv1.IssueStateCategory) (*ticketingv1.TransitionIssueResponse, []Evidence, error) {
	return s.Target.TransitionIssue(ctx, o, &ticketingv1.TransitionIssueRequest{Project: s.Project, IssueKey: key, Target: target})
}

func t014(ctx context.Context, s Subject) error {
	const (
		open = ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN
		done = ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE
	)
	state := func() (ticketingv1.IssueStateCategory, error) {
		r, err := get(ctx, s, s.Project, s.TransitionOpen, small)
		return r.GetState(), err
	}
	if st, err := state(); err != nil || st != open {
		return fmt.Errorf("fixture issue %s is in %s (error %w), want OPEN", s.TransitionOpen, st, err)
	}
	dry, ev, err := transition(ctx, s, Options{DryRun: true, Key: "kit-tr-dry"}, s.TransitionOpen, done)
	if err != nil {
		return fmt.Errorf("dry run: %w", err)
	}
	if dry.GetPrevious() != open || dry.GetState() != done || !dry.GetChanged() || len(ev) != 0 {
		return fmt.Errorf("dry run = %v with %d evidence records, want previous OPEN, state DONE, changed, no evidence", dry, len(ev))
	}
	if st, err := state(); err != nil || st != open {
		return fmt.Errorf("after a dry run the issue is in %s (error %w), want OPEN: a dry run changed the issue", st, err)
	}
	moved, _, err := transition(ctx, s, Options{Key: "kit-tr-1"}, s.TransitionOpen, done)
	if err != nil {
		return err
	}
	if moved.GetPrevious() != open || moved.GetState() != done || !moved.GetChanged() {
		return fmt.Errorf("transition = %v, want previous OPEN, state DONE, changed", moved)
	}
	if st, err := state(); err != nil || st != done {
		return fmt.Errorf("after the call the issue is in %s (error %w), want DONE", st, err)
	}
	again, _, err := transition(ctx, s, Options{Key: "kit-tr-1"}, s.TransitionOpen, done)
	if err != nil {
		return err
	}
	if again.GetPrevious() != done || again.GetState() != done || again.GetChanged() {
		return fmt.Errorf("a repeat = %v, want previous DONE, state DONE, not changed", again)
	}
	back, _, err := transition(ctx, s, Options{Key: "kit-tr-back"}, s.TransitionOpen, moved.GetPrevious())
	if err != nil {
		return fmt.Errorf("the compensating transition: %w", err)
	}
	if back.GetState() != open || !back.GetChanged() {
		return fmt.Errorf("compensation = %v, want state OPEN, changed", back)
	}
	return nil
}

func t015(ctx context.Context, s Subject) error {
	before, err := get(ctx, s, s.Project, s.TransitionStuck, small)
	if err != nil {
		return err
	}
	for _, dry := range []bool{true, false} {
		_, _, err := transition(ctx, s, Options{DryRun: dry, Key: "kit-tr-stuck"}, s.TransitionStuck, s.UnreachableTarget)
		if e := wantReason(err, pluginv1.Reason_REASON_CONFLICT); e != nil {
			return fmt.Errorf("an unreachable category, dry run %v: %w", dry, e)
		}
	}
	after, err := get(ctx, s, s.Project, s.TransitionStuck, small)
	if err != nil {
		return err
	}
	if after.GetState() != before.GetState() {
		return fmt.Errorf("a refused transition moved the issue from %s to %s", before.GetState(), after.GetState())
	}
	for _, target := range []ticketingv1.IssueStateCategory{ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED, 99} {
		_, _, err := transition(ctx, s, Options{Key: "kit-tr-bad"}, s.TransitionOpen, target)
		if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
			return fmt.Errorf("target %d: %w", target, e)
		}
	}
	for _, key := range []string{s.Hidden, s.Moved} {
		_, _, err := transition(ctx, s, Options{Key: "kit-tr-nf"}, key, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE)
		if _, e := notFound(err, s.secrets(s.Project, key)...); e != nil {
			return fmt.Errorf("issue %s: %w", key, e)
		}
	}
	return nil
}

func link(ctx context.Context, s Subject, key, blocker, blocked string, lt ticketingv1.IssueLinkType) (*ticketingv1.LinkIssueResponse, []Evidence, error) {
	return s.Target.LinkIssue(ctx, Options{Key: key}, &ticketingv1.LinkIssueRequest{
		Project: s.Project, IssueKey: blocker, BlockedProject: s.Project, BlockedIssueKey: blocked, LinkType: lt,
	})
}

func t016(ctx context.Context, s Subject) error {
	const blocks = ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS
	first, _, err := link(ctx, s, "kit-link-1", s.LinkBlocker, s.LinkBlocked, blocks)
	if err != nil {
		return err
	}
	again, _, err := link(ctx, s, "kit-link-2", s.LinkBlocker, s.LinkBlocked, blocks)
	if err != nil {
		return err
	}
	if first.GetExisting() || !again.GetExisting() {
		return fmt.Errorf("existing = %v then %v, want false then true: the link was made twice", first.GetExisting(), again.GetExisting())
	}
	for name, c := range map[string]struct {
		blocker, blocked string
		lt               ticketingv1.IssueLinkType
		reason           pluginv1.Reason
	}{
		"a self link":         {s.LinkBlocker, s.LinkBlocker, blocks, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		"no link type":        {s.LinkBlocker, s.LinkBlocked, ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_UNSPECIFIED, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		"a missing blocked":   {s.LinkBlocker, s.Missing, blocks, pluginv1.Reason_REASON_NOT_FOUND},
		"a hidden blocker":    {s.Hidden, s.LinkBlocked, blocks, pluginv1.Reason_REASON_NOT_FOUND},
		"a moved blocked key": {s.LinkBlocker, s.Moved, blocks, pluginv1.Reason_REASON_NOT_FOUND},
	} {
		_, _, err := link(ctx, s, "kit-link-bad", c.blocker, c.blocked, c.lt)
		var e error
		if c.reason == pluginv1.Reason_REASON_NOT_FOUND {
			_, e = notFound(err, s.secrets(s.Project, c.blocked)...)
		} else {
			e = wantReason(err, c.reason)
		}
		if e != nil {
			return fmt.Errorf("%s: %w", name, e)
		}
	}
	return nil
}

func t017(ctx context.Context, s Subject) error {
	one := func(what string, ev []Evidence, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		if len(ev) != 1 || ev[0].Type != EvidenceType || ev[0].ID == "" {
			return fmt.Errorf("%s emitted %v, want exactly one %s record with an id: a call without it is PARTIAL", what, ev, EvidenceType)
		}
		return nil
	}
	for i := range 2 {
		_, ev, err := comment(ctx, s, "kit-ev-comment", "Evidence check.")
		if e := one(fmt.Sprintf("CommentOnIssue (call %d)", i+1), ev, err); e != nil {
			return e
		}
		_, ev, err = transition(ctx, s, Options{Key: "kit-ev-transition"}, s.TransitionOpen, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN)
		if e := one(fmt.Sprintf("TransitionIssue (call %d)", i+1), ev, err); e != nil {
			return e
		}
		_, ev, err = link(ctx, s, "kit-ev-link", s.LinkBlocker, s.LinkBlocked, ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS)
		if e := one(fmt.Sprintf("LinkIssue (call %d)", i+1), ev, err); e != nil {
			return e
		}
	}
	_, ev, err := transition(ctx, s, Options{DryRun: true, Key: "kit-ev-dry"}, s.TransitionOpen, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_IN_PROGRESS)
	if err != nil || len(ev) != 0 {
		return fmt.Errorf("a dry run emitted %v (error %w), want none", ev, err)
	}
	return nil
}

func t018(context.Context, Subject) error {
	return CheckPort(ticketingv1.File_numinance_ports_ticketing_v1_ticketing_proto)
}

var issueKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidateIssue applies the response rules of GetIssue: a key in the key grammar, a title of 1 to 1024 bytes without a
// line feed or a tab, a body of at most maxBodyBytes bytes of valid UTF-8, a state of the enum, at most 128 type bytes, at most
// 64 labels of 1 to 128 bytes and at most 64 operation ids of at most 128 bytes, all printable.
func ValidateIssue(r *ticketingv1.GetIssueResponse, maxBodyBytes int) error {
	switch {
	case !issueKeyRe.MatchString(r.GetIssueKey()):
		return errors.New("an issue key that is not in the key grammar")
	case len(r.GetTitle()) < 1 || len(r.GetTitle()) > 1024 || !isPrintable(r.GetTitle()):
		return errors.New("a title that is empty, over 1024 bytes or not printable")
	case len(r.GetBody()) > maxBodyBytes || !utf8.ValidString(r.GetBody()):
		return errors.New("a body over max_body_bytes or not valid UTF-8")
	case r.GetState() < ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN || r.GetState() > ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED:
		return errors.New("a state outside the four categories")
	case len(r.GetIssueType()) > 128 || !isPrintable(r.GetIssueType()):
		return errors.New("an issue type over 128 bytes or not printable")
	case len(r.GetLabels()) > 64 || len(r.GetOperationIds()) > 64:
		return errors.New("more than 64 labels or operation ids")
	}
	for _, l := range r.GetLabels() {
		if len(l) < 1 || len(l) > 128 || !isPrintable(l) {
			return errors.New("a label that is empty, over 128 bytes or not printable")
		}
	}
	for _, o := range r.GetOperationIds() {
		if len(o) > 128 || !isPrintable(o) {
			return errors.New("an operation id over 128 bytes or not printable")
		}
	}
	return nil
}

// ValidateSummary applies the response rules of one search hit.
func ValidateSummary(h *ticketingv1.IssueSummary) error {
	if !issueKeyRe.MatchString(h.GetIssueKey()) || len(h.GetTitle()) < 1 || len(h.GetTitle()) > 1024 || !isPrintable(h.GetTitle()) ||
		h.GetState() < ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN || h.GetState() > ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED {
		return fmt.Errorf("a search hit %q that has a malformed key, a title out of bounds or a state outside the four categories", h.GetIssueKey())
	}
	return nil
}

// forbidden returns the first character a provider must strip: a control character (a line feed and a tab are allowed
// when allowLayout is set), a bidirectional control or a zero-width character.
func forbidden(s string, allowLayout bool) (rune, bool) {
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			if !allowLayout {
				return r, true
			}
		case unicode.Is(unicode.Cc, r),
			r == 0x061C, r == 0x200E, r == 0x200F, r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069,
			r >= 0x200B && r <= 0x200D, r == 0x2060, r == 0xFEFF:
			return r, true
		}
	}
	return 0, false
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

func equalStrings(a, b []string) bool {
	return slices.Equal(a, b)
}

// want is what the port requires of one RPC.
type want struct {
	risk         annotationsv1.RiskClass
	dryRun       bool
	evidence     bool
	compensating string
	// resources are the request fields that carry the resource_name mark, and no other field of the request does.
	resources []string
}

// CheckPort checks a descriptor of the ticketing proto against what the port requires: five verbs and no other, the risk
// class, dry run, evidence and compensation of each, and the resource_name mark on project (and on blocked_project of
// LinkIssue) of every request and on no other field. It takes the descriptor so that a mutated copy can be checked.
func CheckPort(file protoreflect.FileDescriptor) error {
	table := map[string]want{
		"GetIssue":        {annotationsv1.RiskClass_RISK_CLASS_READ, false, false, "", []string{"project"}},
		"SearchIssues":    {annotationsv1.RiskClass_RISK_CLASS_READ, false, false, "", []string{"project"}},
		"CommentOnIssue":  {annotationsv1.RiskClass_RISK_CLASS_LOW, false, true, "", []string{"project"}},
		"TransitionIssue": {annotationsv1.RiskClass_RISK_CLASS_MEDIUM, true, true, "TicketingService.TransitionIssue", []string{"project"}},
		"LinkIssue":       {annotationsv1.RiskClass_RISK_CLASS_LOW, false, true, "", []string{"project", "blocked_project"}},
	}
	svc := file.Services().ByName("TicketingService")
	if svc == nil || svc.Methods().Len() != len(table) {
		return errors.New("TicketingService is missing or does not have exactly the five verbs of the port (no VendorCall in 1.x)")
	}
	for name, w := range table {
		m := svc.Methods().ByName(protoreflect.Name(name))
		if m == nil {
			return fmt.Errorf("verb %s is missing", name)
		}
		a, _ := proto.GetExtension(m.Options(), annotationsv1.E_Action).(*annotationsv1.ActionOptions)
		if a == nil {
			return fmt.Errorf("%s has no action annotation", name)
		}
		ev := len(a.GetEvidence()) == 1 && a.GetEvidence()[0] == EvidenceType
		if a.GetRisk() != w.risk || a.GetSupportsDryRun() != w.dryRun || !a.GetIdempotent() || a.GetExec() != annotationsv1.ExecMode_EXEC_MODE_SYNC ||
			ev != w.evidence || (!w.evidence && len(a.GetEvidence()) != 0) || a.GetCompensatingAction() != w.compensating ||
			a.GetLlmDescription() == "" || a.GetSince() == "" || a.GetCapability() != "" {
			return fmt.Errorf("%s is annotated %v, want risk %s, SYNC, idempotent, dry run %v, evidence %q %v, compensation %q, a description, a since and no capability",
				name, a, w.risk, w.dryRun, EvidenceType, w.evidence, w.compensating)
		}
		var marked []string
		fields := m.Input().Fields()
		for i := range fields.Len() {
			f := fields.Get(i)
			if fa, _ := proto.GetExtension(f.Options(), annotationsv1.E_Field).(*annotationsv1.FieldAnnotations); fa.GetResourceName() {
				marked = append(marked, string(f.Name()))
			}
		}
		if !equalStrings(marked, w.resources) {
			return fmt.Errorf("%s marks %q as resource_name, want exactly %q: a grant scopes by these fields", name, marked, w.resources)
		}
	}
	return nil
}
