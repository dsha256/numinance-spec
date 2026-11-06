package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	scmv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/scm/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

const (
	// pageSize and maxPages bound what is read of a list the host pages: pageSize*maxPages entries, and a full last page is
	// LIMIT_EXCEEDED, never a silent cut.
	pageSize = 100
	maxPages = 5
	// maxApprovers bounds the permission lookups of one status.
	maxApprovers = 32
)

// maxPullNumber is the largest pull request number scm.proto allows (2^53-1).
const maxPullNumber = 1<<53 - 1

type ghPull struct {
	Number         uint64 `json:"number"`
	State          string `json:"state"`
	Merged         bool   `json:"merged"`
	MergedAt       string `json:"merged_at"`
	MergeCommitSHA string `json:"merge_commit_sha"`
	HTMLURL        string `json:"html_url"`
	Mergeable      *bool  `json:"mergeable"`
	User           struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

// pullRequest converts and checks the host's pull request: every field is untrusted and a violation is UPSTREAM_ERROR.
func pullRequest(p ghPull) (*scmv1.PullRequest, error) {
	if p.Number < 1 || p.Number > maxPullNumber {
		return nil, upstream()
	}
	var state scmv1.PullRequestState
	switch {
	case p.Merged || p.MergedAt != "":
		state = scmv1.PullRequestState_PULL_REQUEST_STATE_MERGED
	case p.State == "open":
		state = scmv1.PullRequestState_PULL_REQUEST_STATE_OPEN
	case p.State == "closed":
		state = scmv1.PullRequestState_PULL_REQUEST_STATE_CLOSED
	default:
		return nil, upstream()
	}
	head, err := commit(p.Head.SHA)
	if err != nil {
		return nil, err
	}
	if branch(p.Head.Ref) != nil || branch(p.Base.Ref) != nil {
		return nil, upstream()
	}
	u, err := httpsURL(p.HTMLURL)
	if err != nil {
		return nil, err
	}
	return &scmv1.PullRequest{Number: p.Number, State: state, HeadBranch: p.Head.Ref, HeadRevision: head, BaseBranch: p.Base.Ref, Url: u}, nil
}

func repoPathOf(owner, name string) string {
	return "/repos/" + ghprovider.Seg(owner) + "/" + ghprovider.Seg(name)
}

// refPath escapes a branch name segment by segment, keeping the slashes the host accepts in a ref path.
func refPath(b string) string {
	segs := strings.Split(b, "/")
	for i, s := range segs {
		segs[i] = ghprovider.Seg(s)
	}
	return strings.Join(segs, "/")
}

func getRepository(ctx context.Context, c *ghprovider.Call, m *scmv1.GetRepositoryRequest) (*scmv1.GetRepositoryResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	var repo struct {
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
		Archived      bool   `json:"archived"`
	}
	if _, err := cl.Do(ctx, http.MethodGet, repoPathOf(owner, name), nil, &repo); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	if branch(repo.DefaultBranch) != nil {
		return nil, upstream()
	}
	var br struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if _, err := cl.Do(ctx, http.MethodGet, repoPathOf(owner, name)+"/branches/"+refPath(repo.DefaultBranch), nil, &br); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	head, err := commit(br.Commit.SHA)
	if err != nil {
		return nil, err
	}
	return &scmv1.GetRepositoryResponse{DefaultBranch: repo.DefaultBranch, DefaultBranchHead: head, Private: repo.Private, Archived: repo.Archived}, nil
}

func getTree(ctx context.Context, c *ghprovider.Call, m *scmv1.GetTreeRequest) (*scmv1.GetTreeResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	if revision(m.GetRevision()) != nil || repoPath(m.GetPath(), true) != nil || m.GetMaxEntries() < 1 || m.GetMaxEntries() > 1024 {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	var tree struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size uint64 `json:"size"`
		} `json:"tree"`
	}
	if _, err := cl.Do(ctx, http.MethodGet, repoPathOf(owner, name)+"/git/trees/"+m.GetRevision()+ghprovider.Query("recursive", "1"), nil, &tree); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	prefix := ""
	if m.GetPath() != "" {
		prefix = m.GetPath() + "/"
	}
	resp := &scmv1.GetTreeResponse{Truncated: tree.Truncated}
	found := prefix == ""
	for _, e := range tree.Tree {
		if e.Path == m.GetPath() && e.Type == "tree" {
			found = true
		}
		rest, ok := strings.CutPrefix(e.Path, prefix)
		if !ok || rest == "" || (!m.GetRecursive() && strings.Contains(rest, "/")) {
			continue
		}
		if len(resp.Entries) == int(m.GetMaxEntries()) {
			resp.Truncated = true
			break
		}
		if repoPath(e.Path, false) != nil || !revisionRe.MatchString(e.SHA) {
			return nil, upstream()
		}
		resp.Entries = append(resp.Entries, &scmv1.TreeEntry{Path: e.Path, Directory: e.Type == "tree", ObjectId: e.SHA, Size: e.Size})
	}
	if !found {
		return nil, ghprovider.Fail(notFoundReason)
	}
	sort.SliceStable(resp.Entries, func(i, j int) bool { return resp.Entries[i].Path < resp.Entries[j].Path })
	return resp, nil
}

func getFile(ctx context.Context, c *ghprovider.Call, m *scmv1.GetFileRequest) (*scmv1.GetFileResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	if revision(m.GetRevision()) != nil || repoPath(m.GetPath(), false) != nil || m.GetMaxBytes() < 1 || m.GetMaxBytes() > 262144 {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if _, err := cl.Do(ctx, http.MethodGet, repoPathOf(owner, name)+"/contents/"+refPath(m.GetPath())+ghprovider.Query("ref", m.GetRevision()), nil, &raw); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	var f struct {
		Type     string `json:"type"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
		SHA      string `json:"sha"`
		Size     uint64 `json:"size"`
	}
	// A directory answers a list, which is not a file; an unknown type is not a file either.
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		return nil, ghprovider.Fail(notFoundReason)
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, upstream()
	}
	if f.Type != "file" || !revisionRe.MatchString(f.SHA) {
		return nil, ghprovider.Fail(notFoundReason)
	}
	if f.Encoding != "base64" {
		return nil, ghprovider.Fail(limitReason)
	}
	content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
	if err != nil || uint64(len(content)) != f.Size {
		return nil, upstream()
	}
	resp := &scmv1.GetFileResponse{ObjectId: f.SHA, Size: f.Size, Content: content}
	if len(content) > int(m.GetMaxBytes()) {
		resp.Content, resp.Truncated = content[:m.GetMaxBytes()], true
	}
	return resp, nil
}

func listBranches(ctx context.Context, c *ghprovider.Call, m *scmv1.ListBranchesRequest) (*scmv1.ListBranchesResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	page := uint64(1)
	if t := m.GetPageToken(); t != "" {
		p, perr := strconv.ParseUint(t, 10, 32)
		if perr != nil || p < 1 || len(t) > 6 {
			return nil, invalid()
		}
		page = p
	}
	if m.GetPageSize() < 1 || m.GetPageSize() > 256 {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	var list []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	path := repoPathOf(owner, name) + "/branches" + ghprovider.Query("per_page", strconv.Itoa(int(m.GetPageSize())), "page", strconv.FormatUint(page, 10))
	if _, err := cl.Do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	if len(list) > int(m.GetPageSize()) {
		return nil, upstream()
	}
	resp := &scmv1.ListBranchesResponse{}
	for _, b := range list {
		head, herr := commit(b.Commit.SHA)
		if herr != nil || branch(b.Name) != nil {
			return nil, upstream()
		}
		resp.Branches = append(resp.Branches, &scmv1.Branch{Name: b.Name, HeadRevision: head})
	}
	if len(list) == int(m.GetPageSize()) {
		resp.NextPageToken = strconv.FormatUint(page+1, 10)
	}
	return resp, nil
}

func createBranch(ctx context.Context, c *ghprovider.Call, m *scmv1.CreateBranchRequest) (*scmv1.CreateBranchResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	if branch(m.GetName()) != nil || revision(m.GetFromRevision()) != nil {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	repo := repoPathOf(owner, name)
	_, err = cl.Do(ctx, http.MethodPost, repo+"/git/refs", map[string]string{"ref": "refs/heads/" + m.GetName(), "sha": m.GetFromRevision()}, nil)
	switch {
	case err == nil:
		c.Wrote(sideWrite, m.GetRepository())
		receipt(ctx, c, "scm.create_branch", m.GetRepository(), m.GetName())
	case isStatus(err, http.StatusUnprocessableEntity):
		// The name is taken: a replay at the same commit answers the existing branch, another commit is a conflict.
		sha, rerr := branchHead(ctx, cl, repo, m.GetName())
		if rerr != nil {
			return nil, ghprovider.Failure(ctx, rerr)
		}
		if sha != m.GetFromRevision() {
			return nil, ghprovider.Fail(conflictReason)
		}
	default:
		return nil, ghprovider.Failure(ctx, err)
	}
	return &scmv1.CreateBranchResponse{Branch: &scmv1.Branch{Name: m.GetName(), HeadRevision: m.GetFromRevision()}}, nil
}

// branchHead reads the commit a branch points to through the ref API.
func branchHead(ctx context.Context, cl ghprovider.Client, repo, b string) (string, error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if _, err := cl.Do(ctx, http.MethodGet, repo+"/git/ref/heads/"+refPath(b), nil, &ref); err != nil {
		return "", err
	}
	if !revisionRe.MatchString(ref.Object.SHA) {
		return "", upstream()
	}
	return ref.Object.SHA, nil
}

func getBranchProtection(ctx context.Context, c *ghprovider.Call, m *scmv1.GetBranchProtectionRequest) (*scmv1.GetBranchProtectionResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	if branch(m.GetBranch()) != nil {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	var p struct {
		Reviews *struct {
			Count uint32 `json:"required_approving_review_count"`
		} `json:"required_pull_request_reviews"`
		Checks *struct {
			Contexts []string `json:"contexts"`
		} `json:"required_status_checks"`
		Admins *struct {
			Enabled bool `json:"enabled"`
		} `json:"enforce_admins"`
		Force *struct {
			Enabled bool `json:"enabled"`
		} `json:"allow_force_pushes"`
	}
	_, err = cl.Do(ctx, http.MethodGet, repoPathOf(owner, name)+"/branches/"+refPath(m.GetBranch())+"/protection", nil, &p)
	var ae *ghprovider.APIError
	if asAPI(err, &ae) && ae.Status == http.StatusNotFound && ae.Message == "Branch not protected" {
		return &scmv1.GetBranchProtectionResponse{}, nil
	}
	if err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	resp := &scmv1.GetBranchProtectionResponse{Protected: true}
	if p.Reviews != nil {
		resp.RequiredReviews = p.Reviews.Count
	}
	if p.Checks != nil {
		if len(p.Checks.Contexts) > 64 {
			return nil, ghprovider.Fail(limitReason)
		}
		seen := map[string]bool{}
		for _, ctxName := range p.Checks.Contexts {
			if cerr := clean(ctxName, 256); cerr != nil || ctxName == "" || seen[ctxName] {
				return nil, upstream()
			}
			seen[ctxName] = true
			resp.RequiredChecks = append(resp.RequiredChecks, ctxName)
		}
	}
	resp.EnforceAdmins = p.Admins != nil && p.Admins.Enabled
	resp.AllowForcePush = p.Force != nil && p.Force.Enabled
	return resp, nil
}

// errNoOpenPullRequest is findOpen's answer when the host has no open pull request for the head and base.
var errNoOpenPullRequest = errors.New("no open pull request")

// findOpen returns the open pull request for head and base, or errNoOpenPullRequest.
func findOpen(ctx context.Context, cl ghprovider.Client, owner, name, head, base string) (*ghPull, error) {
	var list []ghPull
	q := ghprovider.Query("state", "open", "head", owner+":"+head, "base", base)
	if _, err := cl.Do(ctx, http.MethodGet, repoPathOf(owner, name)+"/pulls"+q, nil, &list); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errNoOpenPullRequest
	}
	return &list[0], nil
}

func openPullRequest(ctx context.Context, c *ghprovider.Call, m *scmv1.OpenPullRequestRequest) (*scmv1.OpenPullRequestResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	if branch(m.GetHeadBranch()) != nil || branch(m.GetBaseBranch()) != nil || revision(m.GetHeadRevision()) != nil ||
		text(m.GetTitle(), maxTitle, false) != nil || (m.GetBody() != "" && text(m.GetBody(), maxBody, true) != nil) {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	repo := repoPathOf(owner, name)
	// The pull request is the candidate that was verified, not whatever the branch holds later.
	head, err := branchHead(ctx, cl, repo, m.GetHeadBranch())
	if err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	if head != m.GetHeadRevision() {
		return nil, ghprovider.Fail(conflictReason)
	}
	existing, err := findOpen(ctx, cl, owner, name, m.GetHeadBranch(), m.GetBaseBranch())
	if err != nil && !errors.Is(err, errNoOpenPullRequest) {
		return nil, ghprovider.Failure(ctx, err)
	}
	if existing == nil && c.DryRun() {
		return &scmv1.OpenPullRequestResponse{}, nil
	}
	isNew := false
	if existing == nil {
		var created ghPull
		_, err = cl.Do(ctx, http.MethodPost, repo+"/pulls", map[string]string{"title": m.GetTitle(), "body": m.GetBody(), "head": m.GetHeadBranch(), "base": m.GetBaseBranch()}, &created)
		switch {
		case err == nil:
			existing, isNew = &created, true
			c.Wrote(sideWrite, m.GetRepository())
		case isStatus(err, http.StatusUnprocessableEntity):
			// A pull request appeared between the read and the create: answer it.
			if existing, err = findOpen(ctx, cl, owner, name, m.GetHeadBranch(), m.GetBaseBranch()); err != nil {
				return nil, ghprovider.Fail(conflictReason)
			}
		default:
			return nil, ghprovider.Failure(ctx, err)
		}
	}
	pr, err := pullRequest(*existing)
	if err != nil {
		return nil, err
	}
	if pr.GetHeadRevision() != m.GetHeadRevision() {
		return nil, ghprovider.Fail(conflictReason)
	}
	// A receipt of the reference, never the title or body. A failed submission leaves the call PARTIAL, which the release
	// context treats as no receipt.
	_ = c.Emit(ctx, "pr-ref", "pr-ref", prRef(m.GetRepository(), pr))
	return &scmv1.OpenPullRequestResponse{PullRequest: pr, Existing: !isNew}, nil
}

func updatePullRequest(ctx context.Context, c *ghprovider.Call, m *scmv1.UpdatePullRequestRequest) (*scmv1.UpdatePullRequestResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	if m.GetNumber() < 1 || m.GetNumber() > maxPullNumber || (m.GetTitle() != "" && text(m.GetTitle(), maxTitle, false) != nil) ||
		(m.GetBody() != "" && text(m.GetBody(), maxBody, true) != nil) ||
		(m.GetState() != scmv1.PullRequestState_PULL_REQUEST_STATE_UNSPECIFIED && m.GetState() != scmv1.PullRequestState_PULL_REQUEST_STATE_CLOSED) ||
		(m.GetTitle() == "" && m.GetBody() == "" && m.GetState() == scmv1.PullRequestState_PULL_REQUEST_STATE_UNSPECIFIED) {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	patch := map[string]string{}
	if m.GetTitle() != "" {
		patch["title"] = m.GetTitle()
	}
	if m.GetBody() != "" {
		patch["body"] = m.GetBody()
	}
	if m.GetState() == scmv1.PullRequestState_PULL_REQUEST_STATE_CLOSED {
		patch["state"] = "closed"
	}
	var p ghPull
	if _, err := cl.Do(ctx, http.MethodPatch, repoPathOf(owner, name)+"/pulls/"+strconv.FormatUint(m.GetNumber(), 10), patch, &p); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	c.Wrote(sideWrite, m.GetRepository())
	receipt(ctx, c, "scm.update_pull_request", m.GetRepository(), "pull/"+strconv.FormatUint(m.GetNumber(), 10))
	pr, err := pullRequest(p)
	if err != nil {
		return nil, err
	}
	return &scmv1.UpdatePullRequestResponse{PullRequest: pr}, nil
}

func commentOnPullRequest(ctx context.Context, c *ghprovider.Call, m *scmv1.CommentOnPullRequestRequest) (*scmv1.CommentOnPullRequestResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	key := c.IdempotencyKey()
	if m.GetNumber() < 1 || m.GetNumber() > maxPullNumber || text(m.GetBody(), maxBody, true) != nil || key == "" {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	// The host's key names the comment: a hidden marker lets a retry find the first one instead of adding another.
	marker := "<!-- numinance:" + shortDigest(key) + " -->"
	path := repoPathOf(owner, name) + "/issues/" + strconv.FormatUint(m.GetNumber(), 10) + "/comments"
	var existing []struct {
		ID   uint64 `json:"id"`
		Body string `json:"body"`
	}
	if _, err := cl.Do(ctx, http.MethodGet, path+ghprovider.Query("per_page", strconv.Itoa(pageSize)), nil, &existing); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	for _, e := range existing {
		if strings.Contains(e.Body, marker) {
			return &scmv1.CommentOnPullRequestResponse{CommentId: strconv.FormatUint(e.ID, 10)}, nil
		}
	}
	// A full page that does not hold the marker may hold it on the next: fail closed rather than comment twice.
	if len(existing) >= pageSize {
		return nil, ghprovider.Fail(limitReason)
	}
	var made struct {
		ID uint64 `json:"id"`
	}
	if _, err := cl.Do(ctx, http.MethodPost, path, map[string]string{"body": m.GetBody() + "\n\n" + marker}, &made); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	c.Wrote(sideWrite, m.GetRepository())
	receipt(ctx, c, "scm.comment_on_pull_request", m.GetRepository(), "pull/"+strconv.FormatUint(m.GetNumber(), 10))
	return &scmv1.CommentOnPullRequestResponse{CommentId: strconv.FormatUint(made.ID, 10)}, nil
}

func getPullRequestStatus(ctx context.Context, c *ghprovider.Call, m *scmv1.GetPullRequestStatusRequest) (*scmv1.GetPullRequestStatusResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	if m.GetNumber() < 1 || m.GetNumber() > maxPullNumber {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	repo := repoPathOf(owner, name)
	num := strconv.FormatUint(m.GetNumber(), 10)
	var p ghPull
	if _, err := cl.Do(ctx, http.MethodGet, repo+"/pulls/"+num, nil, &p); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	pr, err := pullRequest(p)
	if err != nil {
		return nil, err
	}
	resp := &scmv1.GetPullRequestStatusResponse{PullRequest: pr, Mergeability: mergeability(p.Mergeable)}
	if pr.GetState() == scmv1.PullRequestState_PULL_REQUEST_STATE_MERGED {
		if resp.MergeRevision, err = commit(p.MergeCommitSHA); err != nil {
			return nil, err
		}
	}
	if resp.Checks, err = checks(ctx, cl, repo, pr.GetHeadRevision()); err != nil {
		return nil, err
	}
	resp.Approvals, resp.ChangesRequested, err = reviews(ctx, cl, repo, num, pr.GetHeadRevision(), p.User.Login)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func mergeability(b *bool) scmv1.Mergeability {
	switch {
	case b == nil:
		return scmv1.Mergeability_MERGEABILITY_UNKNOWN
	case *b:
		return scmv1.Mergeability_MERGEABILITY_MERGEABLE
	}
	return scmv1.Mergeability_MERGEABILITY_CONFLICTING
}

// checks reads the check runs of a head and folds runs of one name into the worst of them.
func checks(ctx context.Context, cl ghprovider.Client, repo, head string) ([]*scmv1.CheckRun, error) {
	type run struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	}
	var runs []run
	for page := 1; ; page++ {
		var cr struct {
			Total int   `json:"total_count"`
			Runs  []run `json:"check_runs"`
		}
		path := repo + "/commits/" + head + "/check-runs" + ghprovider.Query("per_page", strconv.Itoa(pageSize), "page", strconv.Itoa(page))
		if _, err := cl.Do(ctx, http.MethodGet, path, nil, &cr); err != nil {
			return nil, ghprovider.Failure(ctx, err)
		}
		runs = append(runs, cr.Runs...)
		if len(cr.Runs) < pageSize || len(runs) >= cr.Total {
			break
		}
		// The checks a head has are all read or the call is refused: a failing check on a page never read is not a pass.
		if page == maxPages {
			return nil, ghprovider.Fail(limitReason)
		}
	}
	worst := map[string]scmv1.CheckConclusion{}
	var order []string
	for _, r := range runs {
		if err := clean(r.Name, 256); err != nil || r.Name == "" {
			return nil, upstream()
		}
		got := conclusion(r.Status, r.Conclusion)
		prev, seen := worst[r.Name]
		if !seen {
			order = append(order, r.Name)
		}
		if !seen || rank(got) > rank(prev) {
			worst[r.Name] = got
		}
	}
	if len(order) > 256 {
		return nil, ghprovider.Fail(limitReason)
	}
	var out []*scmv1.CheckRun
	for _, n := range order {
		out = append(out, &scmv1.CheckRun{Name: n, Conclusion: worst[n]})
	}
	return out, nil
}

func conclusion(status, concl string) scmv1.CheckConclusion {
	if status != "completed" {
		return scmv1.CheckConclusion_CHECK_CONCLUSION_PENDING
	}
	switch concl {
	case "success":
		return scmv1.CheckConclusion_CHECK_CONCLUSION_SUCCESS
	case "neutral", "skipped", "cancelled":
		return scmv1.CheckConclusion_CHECK_CONCLUSION_NEUTRAL
	}
	// failure, timed_out, action_required, startup_failure, stale and anything new: not a pass.
	return scmv1.CheckConclusion_CHECK_CONCLUSION_FAILURE
}

// rank orders conclusions from the best to the worst, so a name with a failing and a passing run reads as failing.
func rank(c scmv1.CheckConclusion) int {
	switch c {
	case scmv1.CheckConclusion_CHECK_CONCLUSION_FAILURE:
		return 3
	case scmv1.CheckConclusion_CHECK_CONCLUSION_PENDING:
		return 2
	case scmv1.CheckConclusion_CHECK_CONCLUSION_NEUTRAL:
		return 1
	}
	return 0
}

// reviews counts the reviewers whose latest review approves the head and reports whether any latest review asks for changes.
// reviews counts the reviewers whose latest review approves the head, who are not the pull request's author and who hold write
// access to the repository (the host is asked of each approver, at most maxApprovers), and reports whether any latest review asks
// for changes. Every page of reviews is read or the call is refused.
func reviews(ctx context.Context, cl ghprovider.Client, repo, num, head, author string) (approvals uint32, changes bool, err error) {
	type review struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		State    string `json:"state"`
		CommitID string `json:"commit_id"`
	}
	var list []review
	for page := 1; ; page++ {
		var got []review
		path := repo + "/pulls/" + num + "/reviews" + ghprovider.Query("per_page", strconv.Itoa(pageSize), "page", strconv.Itoa(page))
		if _, err := cl.Do(ctx, http.MethodGet, path, nil, &got); err != nil {
			return 0, false, ghprovider.Failure(ctx, err)
		}
		list = append(list, got...)
		if len(got) < pageSize {
			break
		}
		if page == maxPages {
			return 0, false, ghprovider.Fail(limitReason)
		}
	}
	type verdict struct{ approve, changes bool }
	latest := map[string]verdict{}
	var order []string
	for _, r := range list {
		if r.User.Login == "" {
			return 0, false, upstream()
		}
		if _, seen := latest[r.User.Login]; !seen {
			order = append(order, r.User.Login)
		}
		switch r.State {
		case "APPROVED":
			latest[r.User.Login] = verdict{approve: r.CommitID == head}
		case "CHANGES_REQUESTED":
			latest[r.User.Login] = verdict{changes: true}
		case "DISMISSED":
			latest[r.User.Login] = verdict{}
		default:
			if _, ok := latest[r.User.Login]; !ok {
				latest[r.User.Login] = verdict{}
			}
		}
	}
	checked := 0
	for _, login := range order {
		v := latest[login]
		changes = changes || v.changes
		if !v.approve || login == author {
			continue
		}
		if checked == maxApprovers {
			continue
		}
		checked++
		var perm struct {
			Permission string `json:"permission"`
		}
		_, err := cl.Do(ctx, http.MethodGet, repo+"/collaborators/"+ghprovider.Seg(login)+"/permission", nil, &perm)
		var ae *ghprovider.APIError
		switch {
		case errors.As(err, &ae) && ae.Status == http.StatusNotFound:
			continue // not a collaborator: an approval that counts for nothing
		case err != nil:
			return 0, false, ghprovider.Failure(ctx, err)
		}
		if perm.Permission == "admin" || perm.Permission == "write" || perm.Permission == "maintain" {
			approvals++
		}
	}
	return approvals, changes, nil
}

func mergePullRequest(ctx context.Context, c *ghprovider.Call, m *scmv1.MergePullRequestRequest) (*scmv1.MergePullRequestResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	if m.GetNumber() < 1 || m.GetNumber() > maxPullNumber || revision(m.GetExpectedHeadRevision()) != nil || branch(m.GetExpectedBaseBranch()) != nil {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	repo := repoPathOf(owner, name)
	path := repo + "/pulls/" + strconv.FormatUint(m.GetNumber(), 10)
	var p ghPull
	if _, err := cl.Do(ctx, http.MethodGet, path, nil, &p); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	pr, err := pullRequest(p)
	if err != nil {
		return nil, err
	}
	// The pull request that was verified is the one merged: the same head and the same base, whatever its state.
	if pr.GetBaseBranch() != m.GetExpectedBaseBranch() || pr.GetHeadRevision() != m.GetExpectedHeadRevision() {
		return nil, ghprovider.Fail(conflictReason)
	}
	switch pr.GetState() {
	case scmv1.PullRequestState_PULL_REQUEST_STATE_MERGED:
		// Reconciliation of a lost response: the merge was done, and it is answered as done, never done again.
		rev, rerr := commit(p.MergeCommitSHA)
		if rerr != nil {
			return nil, rerr
		}
		return &scmv1.MergePullRequestResponse{MergeRevision: rev, AlreadyMerged: true, MergedAt: parseTime(p.MergedAt), HeadRevision: pr.GetHeadRevision()}, nil
	case scmv1.PullRequestState_PULL_REQUEST_STATE_CLOSED:
		return nil, ghprovider.Fail(conflictReason)
	}
	if p.Mergeable != nil && !*p.Mergeable {
		return nil, ghprovider.Fail(conflictReason)
	}
	if c.DryRun() {
		return &scmv1.MergePullRequestResponse{}, nil
	}
	// merge_method is squash: the host's own default for the repository is not read, and a repository that forbids it
	// refuses the merge (405), which is a conflict here.
	var done struct {
		SHA    string `json:"sha"`
		Merged bool   `json:"merged"`
	}
	if _, err := cl.Do(ctx, http.MethodPut, path+"/merge", map[string]string{"sha": m.GetExpectedHeadRevision(), "merge_method": "squash"}, &done); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	if !done.Merged {
		return nil, ghprovider.Fail(conflictReason)
	}
	c.Wrote(sideWrite, m.GetRepository())
	rev, err := commit(done.SHA)
	if err != nil {
		return nil, err
	}
	receipt(ctx, c, "scm.merge_pull_request", m.GetRepository(), rev)
	resp := &scmv1.MergePullRequestResponse{MergeRevision: rev, HeadRevision: m.GetExpectedHeadRevision()}
	var after ghPull
	if _, err := cl.Do(ctx, http.MethodGet, path, nil, &after); err == nil {
		resp.MergedAt = parseTime(after.MergedAt)
	}
	return resp, nil
}

func parseTime(s string) *timestamppb.Timestamp {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return timestamppb.New(t)
}

func searchCode(ctx context.Context, c *ghprovider.Call, m *scmv1.SearchCodeRequest) (*scmv1.SearchCodeResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	q := m.GetQuery()
	// The text goes in as one quoted phrase: a qualifier ("repo:", "org:") inside it would otherwise widen the search to
	// another repository, which the grant's resource scope never named.
	if text(q, 256, false) != nil || strings.ContainsAny(q, "\"\\") || m.GetMaxResults() < 1 || m.GetMaxResults() > 100 {
		return nil, invalid()
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	var res struct {
		Items []struct {
			Path       string `json:"path"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		} `json:"items"`
	}
	query := ghprovider.Query("q", "\""+q+"\" repo:"+owner+"/"+name, "per_page", strconv.Itoa(int(m.GetMaxResults())))
	if _, err := cl.Do(ctx, http.MethodGet, "/search/code"+query, nil, &res); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	resp := &scmv1.SearchCodeResponse{}
	for _, it := range res.Items {
		// A hit in another repository is dropped, not reported: the answer never names a resource the call did not.
		if !strings.EqualFold(it.Repository.FullName, owner+"/"+name) {
			continue
		}
		if repoPath(it.Path, false) != nil {
			return nil, upstream()
		}
		if len(resp.Paths) < int(m.GetMaxResults()) {
			resp.Paths = append(resp.Paths, it.Path)
		}
	}
	return resp, nil
}
