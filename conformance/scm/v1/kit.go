// Package scmkit is the runnable skeleton of the scm conformance kit, version 1.0.0 (README.md in this directory lists the
// full case table; this package runs the subset a provider can pass with the fixtures a publisher supplies). It is the
// acceptance harness for a provider of numinance.ports.scm.v1: the harness reaches the provider however it likes (a host
// dispatching Invoke, a replay, a direct call) through Target, and the cases judge only what comes back. It imports the
// public spec only.
//
// The kit is generic: a publisher supplies a Subject, the repository and pull requests its provider can be asked about.
// The response rules (ValidateRepository, ValidatePullRequest, ValidateStatus) are the port's own and are also what a host
// enforces on any provider.
package scmkit

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
	scmv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/scm/v1"
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
	GetRepository(context.Context, Options, *scmv1.GetRepositoryRequest) (*scmv1.GetRepositoryResponse, error)
	GetBranchProtection(context.Context, Options, *scmv1.GetBranchProtectionRequest) (*scmv1.GetBranchProtectionResponse, error)
	OpenPullRequest(context.Context, Options, *scmv1.OpenPullRequestRequest) (*scmv1.OpenPullRequestResponse, error)
	GetPullRequestStatus(context.Context, Options, *scmv1.GetPullRequestStatusRequest) (*scmv1.GetPullRequestStatusResponse, error)
	MergePullRequest(context.Context, Options, *scmv1.MergePullRequestRequest) (*scmv1.MergePullRequestResponse, error)
	CommentOnPullRequest(context.Context, Options, *scmv1.CommentOnPullRequestRequest) (*scmv1.CommentOnPullRequestResponse, error)
}

// Subject is what a publisher supplies for the kit.
type Subject struct {
	Target Target
	// Repository exists and Missing does not.
	Repository, Missing string
	// Protected is a protected branch of Repository and Unprotected one that is not.
	Protected, Unprotected string
	// Open is a candidate branch with no pull request yet: the call opens one, and a second call answers it. DryRunOpen is
	// another such branch, which only a dry run touches.
	Open, DryRunOpen Candidate
	// Moved names a branch that does not point at Revision: opening a pull request for it is refused.
	Moved Candidate
	// Status is a pull request that is open and has checks and reviews.
	Status uint64
	// Merge is a pull request that merges at Head; MovedHead is one whose head is not Head; Closed is one that is closed.
	Merge          MergeFixture
	MovedHead      MergeFixture
	Closed         MergeFixture
	CommentTarget  uint64
	CommentKey     string
	CommentContent string
}

// Candidate is a branch, the commit it must point at and the branch the pull request merges into.
type Candidate struct {
	Branch, Revision, Base, Title string
}

// MergeFixture is a pull request number and the head the caller verified.
type MergeFixture struct {
	Number uint64
	Head   string
	// Base is the branch the pull request was opened into.
	Base string
}

// Case is one conformance case: an id of the README table and the check.
type Case struct {
	ID, Name string
	Check    func(ctx context.Context, s Subject) error
}

// Cases returns the cases this skeleton runs, in README order.
func Cases() []Case {
	return []Case{
		{"CORE-001", "GetRepository: a default branch and the full commit it points to", core001},
		{"CORE-002", "GetRepository of a repository that does not exist is NOT_FOUND", core002},
		{"CORE-003", "a malformed repository name is INVALID_ARGUMENT", core003},
		{"CORE-004", "GetBranchProtection reports a protected and an unprotected branch", core004},
		{"CORE-005", "OpenPullRequest opens one pull request for the candidate and a repeat answers it", core005},
		{"CORE-006", "OpenPullRequest for a branch that does not point at head_revision is a conflict", core006},
		{"CORE-007", "OpenPullRequest with dry run opens nothing", core007},
		{"CORE-008", "GetPullRequestStatus: every field within its bounds", core008},
		{"CORE-009", "MergePullRequest merges the verified head, answers a repeat as done, refuses a moved head and a closed pull request", core009},
		{"CORE-010", "CommentOnPullRequest under one key adds one comment", core010},
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

func core001(ctx context.Context, s Subject) error {
	resp, err := s.Target.GetRepository(ctx, Options{}, &scmv1.GetRepositoryRequest{Repository: s.Repository})
	if err != nil {
		return err
	}
	return ValidateRepository(resp)
}

func core002(ctx context.Context, s Subject) error {
	_, err := s.Target.GetRepository(ctx, Options{}, &scmv1.GetRepositoryRequest{Repository: s.Missing})
	return wantReason(err, pluginv1.Reason_REASON_NOT_FOUND)
}

func core003(ctx context.Context, s Subject) error {
	for _, bad := range []string{"", "a", "a/b/c", "../x", "acme/..", "acme/shop.git", "acme/sh op", strings.Repeat("a", 101) + "/x"} {
		_, err := s.Target.GetRepository(ctx, Options{}, &scmv1.GetRepositoryRequest{Repository: bad})
		if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
			return fmt.Errorf("repository %q: %w", bad, e)
		}
	}
	return nil
}

func core004(ctx context.Context, s Subject) error {
	p, err := s.Target.GetBranchProtection(ctx, Options{}, &scmv1.GetBranchProtectionRequest{Repository: s.Repository, Branch: s.Protected})
	if err != nil {
		return err
	}
	if !p.GetProtected() || len(p.GetRequiredChecks()) > 64 {
		return fmt.Errorf("protected = %v with %d required checks, want a protected branch with at most 64", p.GetProtected(), len(p.GetRequiredChecks()))
	}
	u, err := s.Target.GetBranchProtection(ctx, Options{}, &scmv1.GetBranchProtectionRequest{Repository: s.Repository, Branch: s.Unprotected})
	if err != nil {
		return err
	}
	if u.GetProtected() || u.GetRequiredReviews() != 0 || len(u.GetRequiredChecks()) != 0 {
		return fmt.Errorf("an unprotected branch reported protection %v", u)
	}
	return nil
}

func open(c Candidate, repo string) *scmv1.OpenPullRequestRequest {
	return &scmv1.OpenPullRequestRequest{Repository: repo, HeadBranch: c.Branch, HeadRevision: c.Revision, BaseBranch: c.Base, Title: c.Title, Body: "Candidate."}
}

func core005(ctx context.Context, s Subject) error {
	first, err := s.Target.OpenPullRequest(ctx, Options{Key: "kit-open"}, open(s.Open, s.Repository))
	if err != nil {
		return err
	}
	if err := ValidatePullRequest(first.GetPullRequest()); err != nil {
		return err
	}
	pr := first.GetPullRequest()
	if pr.GetState() != scmv1.PullRequestState_PULL_REQUEST_STATE_OPEN || pr.GetHeadRevision() != s.Open.Revision || pr.GetHeadBranch() != s.Open.Branch || pr.GetBaseBranch() != s.Open.Base {
		return fmt.Errorf("pull request = %v, want an open one for %s at %s into %s", pr, s.Open.Branch, s.Open.Revision, s.Open.Base)
	}
	again, err := s.Target.OpenPullRequest(ctx, Options{Key: "kit-open"}, open(s.Open, s.Repository))
	if err != nil {
		return err
	}
	if again.GetPullRequest().GetNumber() != pr.GetNumber() || !again.GetExisting() {
		return fmt.Errorf("a repeat answered number %d existing=%v, want number %d existing=true: the pull request was opened twice", again.GetPullRequest().GetNumber(), again.GetExisting(), pr.GetNumber())
	}
	return nil
}

func core006(ctx context.Context, s Subject) error {
	_, err := s.Target.OpenPullRequest(ctx, Options{Key: "kit-moved"}, open(s.Moved, s.Repository))
	return wantReason(err, pluginv1.Reason_REASON_CONFLICT)
}

func core007(ctx context.Context, s Subject) error {
	resp, err := s.Target.OpenPullRequest(ctx, Options{DryRun: true, Key: "kit-dry"}, open(s.DryRunOpen, s.Repository))
	if err != nil {
		return err
	}
	if resp.GetPullRequest() != nil {
		return fmt.Errorf("a dry run answered pull request %v, want none: it would open one", resp.GetPullRequest())
	}
	return nil
}

func core008(ctx context.Context, s Subject) error {
	resp, err := s.Target.GetPullRequestStatus(ctx, Options{}, &scmv1.GetPullRequestStatusRequest{Repository: s.Repository, Number: s.Status})
	if err != nil {
		return err
	}
	return ValidateStatus(resp)
}

func core009(ctx context.Context, s Subject) error {
	req := func(f MergeFixture) *scmv1.MergePullRequestRequest {
		return &scmv1.MergePullRequestRequest{Repository: s.Repository, Number: f.Number, ExpectedHeadRevision: f.Head, ExpectedBaseBranch: f.Base}
	}
	_, err := s.Target.MergePullRequest(ctx, Options{Key: "kit-moved"}, req(s.MovedHead))
	if e := wantReason(err, pluginv1.Reason_REASON_CONFLICT); e != nil {
		return fmt.Errorf("moved head: %w", e)
	}
	_, err = s.Target.MergePullRequest(ctx, Options{Key: "kit-closed"}, req(s.Closed))
	if e := wantReason(err, pluginv1.Reason_REASON_CONFLICT); e != nil {
		return fmt.Errorf("closed: %w", e)
	}
	done, err := s.Target.MergePullRequest(ctx, Options{Key: "kit-merge"}, req(s.Merge))
	if err != nil {
		return err
	}
	if !revisionRe.MatchString(done.GetMergeRevision()) || done.GetAlreadyMerged() {
		return fmt.Errorf("merge = %v, want a merge revision and already_merged false", done)
	}
	if done.GetHeadRevision() != s.Merge.Head {
		return fmt.Errorf("merge head = %q, want the verified head %q", done.GetHeadRevision(), s.Merge.Head)
	}
	other := s.Merge
	other.Base = "release"
	_, err = s.Target.MergePullRequest(ctx, Options{Key: "kit-other-base"}, req(other))
	if e := wantReason(err, pluginv1.Reason_REASON_CONFLICT); e != nil {
		return fmt.Errorf("another base: %w", e)
	}
	again, err := s.Target.MergePullRequest(ctx, Options{Key: "kit-merge"}, req(s.Merge))
	if err != nil {
		return err
	}
	if again.GetMergeRevision() != done.GetMergeRevision() || !again.GetAlreadyMerged() || again.GetHeadRevision() != s.Merge.Head {
		return fmt.Errorf("a repeat = %v, want the first merge revision as already merged: a merge was done twice", again)
	}
	return nil
}

func core010(ctx context.Context, s Subject) error {
	req := &scmv1.CommentOnPullRequestRequest{Repository: s.Repository, Number: s.CommentTarget, Body: s.CommentContent}
	first, err := s.Target.CommentOnPullRequest(ctx, Options{Key: s.CommentKey}, req)
	if err != nil {
		return err
	}
	again, err := s.Target.CommentOnPullRequest(ctx, Options{Key: s.CommentKey}, req)
	if err != nil {
		return err
	}
	if first.GetCommentId() == "" || first.GetCommentId() != again.GetCommentId() {
		return fmt.Errorf("comment ids %q and %q, want one comment under one key", first.GetCommentId(), again.GetCommentId())
	}
	return nil
}

var (
	revisionRe = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	branchRe   = regexp.MustCompile(`^[^\x00-\x20\x7f~^:?*\[\\]{1,255}$`)
)

// ValidateRepository applies the response rules of GetRepository.
func ValidateRepository(r *scmv1.GetRepositoryResponse) error {
	if !branchRe.MatchString(r.GetDefaultBranch()) || !revisionRe.MatchString(r.GetDefaultBranchHead()) {
		return errors.New("a repository answer names a default branch that is not a branch name or a head that is not a full commit id")
	}
	return nil
}

// ValidatePullRequest applies the response rules of a pull request: a number in 1 to 2^53-1, a closed state, branch names, a
// full commit id for the head and an https URL of at most 1024 bytes.
func ValidatePullRequest(p *scmv1.PullRequest) error {
	switch {
	case p == nil:
		return errors.New("no pull request")
	case p.GetNumber() < 1 || p.GetNumber() >= 1<<53:
		return errors.New("a pull request number outside 1 to 2^53-1")
	case p.GetState() < scmv1.PullRequestState_PULL_REQUEST_STATE_OPEN || p.GetState() > scmv1.PullRequestState_PULL_REQUEST_STATE_MERGED:
		return errors.New("a pull request state outside the enum")
	case !branchRe.MatchString(p.GetHeadBranch()) || !branchRe.MatchString(p.GetBaseBranch()):
		return errors.New("a pull request branch that is not a branch name")
	case !revisionRe.MatchString(p.GetHeadRevision()):
		return errors.New("a pull request head that is not a full commit id")
	case p.GetUrl() != "" && (len(p.GetUrl()) > 1024 || !strings.HasPrefix(p.GetUrl(), "https://") || !isPrintable(p.GetUrl())):
		return errors.New("a pull request URL that is not a bounded https URL")
	}
	return nil
}

// ValidateStatus applies the response rules of GetPullRequestStatus.
func ValidateStatus(r *scmv1.GetPullRequestStatusResponse) error {
	if err := ValidatePullRequest(r.GetPullRequest()); err != nil {
		return err
	}
	if r.GetMergeability() < scmv1.Mergeability_MERGEABILITY_MERGEABLE || r.GetMergeability() > scmv1.Mergeability_MERGEABILITY_UNKNOWN {
		return errors.New("a mergeability outside the enum")
	}
	if len(r.GetChecks()) > 256 {
		return errors.New("more than 256 checks")
	}
	seen := map[string]bool{}
	for _, c := range r.GetChecks() {
		if c.GetName() == "" || len(c.GetName()) > 256 || !isPrintable(c.GetName()) || seen[c.GetName()] ||
			c.GetConclusion() < scmv1.CheckConclusion_CHECK_CONCLUSION_PENDING || c.GetConclusion() > scmv1.CheckConclusion_CHECK_CONCLUSION_NEUTRAL {
			return errors.New("a check with no name, an unbounded or repeated name, or a conclusion outside the enum")
		}
		seen[c.GetName()] = true
	}
	merged := r.GetPullRequest().GetState() == scmv1.PullRequestState_PULL_REQUEST_STATE_MERGED
	if merged != (r.GetMergeRevision() != "") || (merged && !revisionRe.MatchString(r.GetMergeRevision())) {
		return errors.New("a merge revision that does not match the merged state")
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
