package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// The pipeline contract (README, "What a pipeline must do"). GitHub starts a workflow_dispatch run on a branch, never at a
// commit, and returns no run id. So the provider dispatches on the repository's default branch, the protected one, which
// means the candidate can never change the pipeline definition, and passes the commit to build as an input. The workflow
// names its run "numinance <operation> <revision>", by which the provider finds the run and reports the revision.
const (
	inputRevision  = "numinance_revision"
	inputOperation = "numinance_operation_id"
	maxParams      = 32
)

var runNameRe = regexp.MustCompile(`^numinance ([0-9a-f]{32}) ([0-9a-f]{40}|[0-9a-f]{64})$`)

type ghRun struct {
	ID           uint64 `json:"id"`
	DisplayTitle string `json:"display_title"`
	HeadBranch   string `json:"head_branch"`
	Path         string `json:"path"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	HTMLURL      string `json:"html_url"`
	StartedAt    string `json:"run_started_at"`
	UpdatedAt    string `json:"updated_at"`
}

func repoPathOf(owner, name string) string {
	return "/repos/" + ghprovider.Seg(owner) + "/" + ghprovider.Seg(name)
}

func opID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:16])
}

// state maps the host's run status and conclusion; an unknown ending is a failure, never a success.
func state(status, conclusion string) civ1.OperationState {
	switch status {
	case "queued", "waiting", "requested", "pending":
		return civ1.OperationState_OPERATION_STATE_QUEUED
	case "in_progress":
		return civ1.OperationState_OPERATION_STATE_RUNNING
	case "completed":
		switch conclusion {
		case "success":
			return civ1.OperationState_OPERATION_STATE_SUCCEEDED
		case "cancelled":
			return civ1.OperationState_OPERATION_STATE_CANCELLED
		case "timed_out":
			return civ1.OperationState_OPERATION_STATE_TIMED_OUT
		}
		return civ1.OperationState_OPERATION_STATE_FAILED
	}
	return civ1.OperationState_OPERATION_STATE_UNSPECIFIED
}

func opIDString(id uint64) string { return strconv.FormatUint(id, 10) }

func parseOp(s string) (uint64, error) {
	if !operationRe.MatchString(s) {
		return 0, invalid()
	}
	id, err := strconv.ParseUint(s, 10, 63)
	if err != nil || id == 0 {
		return 0, invalid()
	}
	return id, nil
}

// errNoRun is findRun's answer when no recent run of the pipeline carries the operation.
var errNoRun = errors.New("no run of the operation")

// runsPage is how many runs one look at the pipeline reads. A full page that does not hold the operation may hold it further back, so
// the look fails closed (LIMIT_EXCEEDED) instead of letting a retry dispatch a second run.
const runsPage = 100

// ofPipeline reports whether run is a run of the customer's pipeline definition on the default branch: dispatched on that branch
// and defined by the workflow file the pipeline names. A run of the same name dispatched on another ref runs that ref's own copy
// of the workflow file, which a candidate controls, and is never adopted.
func ofPipeline(run ghRun, pipe, defaultBranch string) bool {
	file, ref, ok := strings.Cut(run.Path, "@")
	if !ok || file != ".github/workflows/"+pipe {
		return false
	}
	return run.HeadBranch == defaultBranch && (ref == defaultBranch || ref == "refs/heads/"+defaultBranch)
}

// defaultBranch reads the repository's default branch, the one ref a pipeline is dispatched on.
func defaultBranch(ctx context.Context, cl ghprovider.Client, repo string) (string, error) {
	var r struct {
		DefaultBranch string `json:"default_branch"`
	}
	if _, err := cl.Do(ctx, http.MethodGet, repo, nil, &r); err != nil {
		return "", ghprovider.Failure(ctx, err)
	}
	if r.DefaultBranch == "" || len(r.DefaultBranch) > 255 || !printable(r.DefaultBranch) {
		return "", upstream()
	}
	return r.DefaultBranch, nil
}

// findRun looks for the run of an operation among the recent dispatched runs of the pipeline on the default branch.
func findRun(ctx context.Context, cl ghprovider.Client, repo, pipe, op, branch string) (*ghRun, error) {
	var list struct {
		Runs []ghRun `json:"workflow_runs"`
	}
	path := repo + "/actions/workflows/" + ghprovider.Seg(pipe) + "/runs" + ghprovider.Query("branch", branch, "event", "workflow_dispatch", "per_page", strconv.Itoa(runsPage))
	if _, err := cl.Do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	for i := range list.Runs {
		if m := runNameRe.FindStringSubmatch(list.Runs[i].DisplayTitle); len(m) == 3 && m[1] == op && ofPipeline(list.Runs[i], pipe, branch) {
			return &list.Runs[i], nil
		}
	}
	if len(list.Runs) >= runsPage {
		return nil, ghprovider.Fail(pluginv1.Reason_REASON_LIMIT_EXCEEDED)
	}
	return nil, errNoRun
}

func startOperation(poll time.Duration) func(context.Context, *ghprovider.Call, *civ1.StartOperationRequest) (*civ1.StartOperationResponse, error) {
	return func(ctx context.Context, c *ghprovider.Call, m *civ1.StartOperationRequest) (*civ1.StartOperationResponse, error) {
		owner, name, err := repository(m.GetRepository())
		if err != nil {
			return nil, err
		}
		key := c.IdempotencyKey()
		if err := pipeline(m.GetPipeline()); err != nil || !revisionRe.MatchString(m.GetRevision()) || key == "" || len(m.GetParams()) > maxParams {
			return nil, invalid()
		}
		inputs := map[string]string{inputRevision: m.GetRevision()}
		op := opID(key)
		inputs[inputOperation] = op
		for k, v := range m.GetParams() {
			if !paramKeyRe.MatchString(k) || strings.HasPrefix(k, reservedPrefix) || len(v) > 1024 || !printable(v) {
				return nil, invalid()
			}
			inputs[k] = v
		}
		cl, err := c.Client(ctx)
		if err != nil {
			return nil, err
		}
		repo := repoPathOf(owner, name)
		branch, err := defaultBranch(ctx, cl, repo)
		if err != nil {
			return nil, err
		}
		// The operation is found by the key before another is started: a dispatch has no key of its own.
		run, ferr := findRun(ctx, cl, repo, m.GetPipeline(), op, branch)
		switch {
		case ferr == nil:
			return started(ctx, c, m, run, true)
		case !errors.Is(ferr, errNoRun):
			return nil, ferr
		}
		// The workflow file and the revision to build must exist; a dry run stops here.
		if _, err := cl.Do(ctx, http.MethodGet, repo+"/actions/workflows/"+ghprovider.Seg(m.GetPipeline()), nil, nil); err != nil {
			return nil, ghprovider.Failure(ctx, err)
		}
		if _, err := cl.Do(ctx, http.MethodGet, repo+"/commits/"+m.GetRevision(), nil, nil); err != nil {
			return nil, ghprovider.Failure(ctx, err)
		}
		if c.DryRun() {
			return &civ1.StartOperationResponse{State: civ1.OperationState_OPERATION_STATE_QUEUED}, nil
		}
		body := map[string]any{"ref": branch, "inputs": inputs}
		if _, err := cl.Do(ctx, http.MethodPost, repo+"/actions/workflows/"+ghprovider.Seg(m.GetPipeline())+"/dispatches", body, nil); err != nil {
			return nil, ghprovider.Failure(ctx, err)
		}
		c.Wrote(pluginv1.SideEffect_SIDE_EFFECT_DEPLOY, m.GetRepository())
		// The run appears a moment after the dispatch. A run that has not appeared fails the call as unavailable: the
		// host's retry finds the run by the key above and starts no second one.
		for attempt := 0; attempt < 5; attempt++ {
			run, ferr := findRun(ctx, cl, repo, m.GetPipeline(), op, branch)
			switch {
			case ferr == nil:
				return started(ctx, c, m, run, false)
			case !errors.Is(ferr, errNoRun):
				return nil, ferr
			}
			select {
			case <-ctx.Done():
				return nil, ghprovider.Failure(ctx, ctx.Err())
			case <-time.After(poll):
			}
		}
		return nil, ghprovider.Fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
	}
}

func started(ctx context.Context, c *ghprovider.Call, m *civ1.StartOperationRequest, run *ghRun, existing bool) (*civ1.StartOperationResponse, error) {
	st := state(run.Status, run.Conclusion)
	if match := runNameRe.FindStringSubmatch(run.DisplayTitle); st == civ1.OperationState_OPERATION_STATE_UNSPECIFIED || run.ID == 0 || run.ID > 1<<53-1 ||
		len(match) != 3 || match[2] != m.GetRevision() {
		// A run named for another revision is not the one asked for, whatever its operation id says.
		return nil, upstream()
	}
	// The receipt names the run's reference, never the parameters' values or the log.
	_ = c.Emit(ctx, "ci-run-ref", RecordType, &structpb.Struct{Fields: map[string]*structpb.Value{
		"repository":   structpb.NewStringValue(m.GetRepository()),
		"pipeline":     structpb.NewStringValue(m.GetPipeline()),
		"revision":     structpb.NewStringValue(m.GetRevision()),
		"operation_id": structpb.NewStringValue(opIDString(run.ID)),
	}})
	return &civ1.StartOperationResponse{OperationId: opIDString(run.ID), State: st, Existing: existing}, nil
}

// getRun reads a run and accepts it only when it is a run of the default branch's own workflow definition (ofPipeline): a run
// of any other workflow or ref is not an operation of this provider, whatever id it was asked by.
func getRun(ctx context.Context, cl ghprovider.Client, repo string, id uint64) (*ghRun, error) {
	branch, err := defaultBranch(ctx, cl, repo)
	if err != nil {
		return nil, err
	}
	var run ghRun
	if _, err := cl.Do(ctx, http.MethodGet, repo+"/actions/runs/"+opIDString(id), nil, &run); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	file, _, _ := strings.Cut(run.Path, "@")
	if run.ID != id || !strings.HasPrefix(file, ".github/workflows/") || !ofPipeline(run, strings.TrimPrefix(file, ".github/workflows/"), branch) {
		return nil, upstream()
	}
	return &run, nil
}

func getOperation(ctx context.Context, c *ghprovider.Call, m *civ1.GetOperationRequest) (*civ1.GetOperationResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	id, err := parseOp(m.GetOperationId())
	if err != nil {
		return nil, err
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	run, err := getRun(ctx, cl, repoPathOf(owner, name), id)
	if err != nil {
		return nil, err
	}
	st := state(run.Status, run.Conclusion)
	match := runNameRe.FindStringSubmatch(run.DisplayTitle)
	if st == civ1.OperationState_OPERATION_STATE_UNSPECIFIED || match == nil {
		// A run that does not follow the pipeline contract is not an operation of this provider.
		return nil, upstream()
	}
	resp := &civ1.GetOperationResponse{State: st, Revision: match[2], StartedAt: parseTime(run.StartedAt)}
	if st != civ1.OperationState_OPERATION_STATE_QUEUED && st != civ1.OperationState_OPERATION_STATE_RUNNING {
		resp.FinishedAt = parseTime(run.UpdatedAt)
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

func cancelOperation(ctx context.Context, c *ghprovider.Call, m *civ1.CancelOperationRequest) (*civ1.CancelOperationResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	id, err := parseOp(m.GetOperationId())
	if err != nil {
		return nil, err
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	repo := repoPathOf(owner, name)
	run, err := getRun(ctx, cl, repo, id)
	if err != nil {
		return nil, err
	}
	if st := state(run.Status, run.Conclusion); st != civ1.OperationState_OPERATION_STATE_QUEUED && st != civ1.OperationState_OPERATION_STATE_RUNNING {
		// An ended run is answered as it is: cancelling it changes nothing.
		if st == civ1.OperationState_OPERATION_STATE_UNSPECIFIED {
			return nil, upstream()
		}
		return &civ1.CancelOperationResponse{State: st}, nil
	}
	if _, err := cl.Do(ctx, http.MethodPost, repo+"/actions/runs/"+opIDString(id)+"/cancel", nil, nil); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	c.Wrote(pluginv1.SideEffect_SIDE_EFFECT_WRITE, m.GetRepository())
	_ = c.Emit(ctx, "action", ciAction, &structpb.Struct{Fields: map[string]*structpb.Value{
		"capability":   structpb.NewStringValue("ciexecutor.cancel_operation"),
		"repository":   structpb.NewStringValue(m.GetRepository()),
		"operation_id": structpb.NewStringValue(opIDString(id)),
	}})
	after, err := getRun(ctx, cl, repo, id)
	if err != nil {
		return nil, err
	}
	st := state(after.Status, after.Conclusion)
	if st == civ1.OperationState_OPERATION_STATE_UNSPECIFIED {
		return nil, upstream()
	}
	return &civ1.CancelOperationResponse{State: st}, nil
}

func listArtifacts(ctx context.Context, c *ghprovider.Call, m *civ1.ListArtifactsRequest) (*civ1.ListArtifactsResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	id, err := parseOp(m.GetOperationId())
	if err != nil {
		return nil, err
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	var list struct {
		Artifacts []struct {
			Name   string `json:"name"`
			Size   uint64 `json:"size_in_bytes"`
			Digest string `json:"digest"`
		} `json:"artifacts"`
	}
	if _, err := cl.Do(ctx, http.MethodGet, repoPathOf(owner, name)+"/actions/runs/"+opIDString(id)+"/artifacts"+ghprovider.Query("per_page", "100"), nil, &list); err != nil {
		return nil, ghprovider.Failure(ctx, err)
	}
	if len(list.Artifacts) > 256 {
		return nil, ghprovider.Fail(pluginv1.Reason_REASON_LIMIT_EXCEEDED)
	}
	seen := map[string]bool{}
	resp := &civ1.ListArtifactsResponse{}
	for _, a := range list.Artifacts {
		if a.Name == "" || len(a.Name) > 256 || !printable(a.Name) || seen[a.Name] || (a.Digest != "" && !digestRe.MatchString(a.Digest)) {
			return nil, upstream()
		}
		seen[a.Name] = true
		resp.Artifacts = append(resp.Artifacts, &civ1.Artifact{Name: a.Name, SizeBytes: a.Size, Digest: a.Digest})
	}
	sort.SliceStable(resp.Artifacts, func(i, j int) bool { return resp.Artifacts[i].Name < resp.Artifacts[j].Name })
	return resp, nil
}

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func getLogReference(ctx context.Context, c *ghprovider.Call, m *civ1.GetLogReferenceRequest) (*civ1.GetLogReferenceResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	id, err := parseOp(m.GetOperationId())
	if err != nil {
		return nil, err
	}
	cl, err := c.Client(ctx)
	if err != nil {
		return nil, err
	}
	run, err := getRun(ctx, cl, repoPathOf(owner, name), id)
	if err != nil {
		return nil, err
	}
	u, err := httpsURL(run.HTMLURL)
	if err != nil {
		return nil, err
	}
	if pu, perr := url.Parse(u); perr != nil || pu.Host != "github.com" {
		return nil, upstream()
	}
	return &civ1.GetLogReferenceResponse{Url: u}, nil
}
