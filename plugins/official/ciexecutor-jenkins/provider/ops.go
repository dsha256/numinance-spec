package provider

import (
	"context"
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

// queueLocationRe is the Location Jenkins answers a build request with: the queue item. Only its shape is checked; the host in
// it is Jenkins's own root URL setting and is never followed.
var queueLocationRe = regexp.MustCompile(`/queue/item/(\d{1,15})/?$`)

// impl is the provider over one Jenkins server.
type impl struct {
	cfg Config
	// poll is how long CancelOperation waits between looks at a run it asked Jenkins to stop; a replay passes zero.
	poll time.Duration
	// seen is the queue items of the runs this process has seen.
	seen *queueIndex
}

// client builds the client of a call: it asks the host for the credential handle first (a replay has none).
func (p impl) client(ctx context.Context, c *ghprovider.Call) (client, error) {
	gc, err := c.Client(ctx)
	if err != nil {
		return client{}, err
	}
	return client{rt: gc.Transport, handle: gc.Handle, cfg: p.cfg}, nil
}

func (p impl) target(repo, opID string) (job, op string, err error) {
	owner, name, err := repository(repo)
	if err != nil {
		return "", "", err
	}
	op, pipe, err := parseOperation(opID)
	if err != nil {
		return "", "", err
	}
	return jobPath(owner, name, pipe), op, nil
}

func (p impl) start(ctx context.Context, c *ghprovider.Call, m *civ1.StartOperationRequest) (*civ1.StartOperationResponse, error) {
	owner, name, err := repository(m.GetRepository())
	if err != nil {
		return nil, err
	}
	key := c.IdempotencyKey()
	if err := pipeline(m.GetPipeline()); err != nil || !revisionRe.MatchString(m.GetRevision()) || key == "" {
		return nil, invalid()
	}
	if err := params(m.GetParams()); err != nil {
		return nil, err
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	job := jobPath(owner, name, m.GetPipeline())
	if err := p.checkJob(ctx, cl, job, m.GetParams()); err != nil {
		return nil, err
	}
	op := opDigest(key, m.GetRepository(), m.GetPipeline())
	id := operationID(op, m.GetPipeline())
	// The run is found by the key before another is started: Jenkins has no key of its own.
	switch run, ferr := p.locate(ctx, cl, job, op); {
	case ferr == nil:
		if run.revision != m.GetRevision() {
			// A run of this key for another commit is not the one asked for.
			return nil, upstream()
		}
		if c.DryRun() {
			// A dry run has no effect to record: no receipt, and no operation id (the port answers none for a dry run).
			return &civ1.StartOperationResponse{State: run.state, Existing: true}, nil
		}
		return p.started(ctx, c, m, id, run.state, true), nil
	case !errors.Is(ferr, errNoRun):
		return nil, ferr
	}
	if c.DryRun() {
		return &civ1.StartOperationResponse{State: civ1.OperationState_OPERATION_STATE_QUEUED}, nil
	}
	crumb, err := p.crumb(ctx, cl)
	if err != nil {
		return nil, err
	}
	form := url.Values{paramRevision: {m.GetRevision()}, paramOperation: {op}}
	for k, v := range m.GetParams() {
		form.Set(k, v)
	}
	rep, err := cl.do(ctx, request{method: http.MethodPost, path: job + "/buildWithParameters", form: form, header: crumb}, nil)
	if err == nil && rep.status == http.StatusCreated {
		// Jenkins accepts a build with 201 and the queue item in Location.
		if qid, ok := queueItemID(rep.header.Get("Location")); ok {
			p.seen.put(op, qid)
			c.Wrote(pluginv1.SideEffect_SIDE_EFFECT_DEPLOY, m.GetRepository())
			return p.started(ctx, c, m, id, civ1.OperationState_OPERATION_STATE_QUEUED, false), nil
		}
	}
	// Anything else does not say whether Jenkins took the build: a timeout, a reset or a 5xx from a proxy in front may follow an
	// accepted request, and a 2xx without the queue item is odd. Neither "wrote" nor "did not write" is claimed from the answer;
	// Jenkins is asked for the run by its key. A run found is the effect of this very request (nothing was found before it).
	if buildMayHaveBeenTaken(err) && ctx.Err() == nil {
		run, ferr := p.locate(ctx, cl, job, op)
		if ferr == nil && run.revision == m.GetRevision() {
			c.Wrote(pluginv1.SideEffect_SIDE_EFFECT_DEPLOY, m.GetRepository())
			return p.started(ctx, c, m, id, run.state, false), nil
		}
	}
	// Not found, or not asked: the outcome is not known, so the call fails with the reason of the error and claims no effect. The
	// host's retry under the same key finds a run that was started after all and starts no second one.
	if err != nil {
		return nil, failure(ctx, err)
	}
	return nil, upstream()
}

// buildMayHaveBeenTaken reports whether the error of a build request leaves it open that Jenkins took the build: no error (an odd
// answer), a transport error after the request was written, an answer over the limit or a 5xx. A refusal (4xx, 3xx) and a request
// the transport would not send mean it did not.
func buildMayHaveBeenTaken(err error) bool {
	var ae *ghprovider.APIError
	switch {
	case err == nil:
		return true
	case errors.Is(err, ghprovider.ErrNoEgress):
		return false
	case errors.As(err, &ae):
		return ae.Status >= 500
	}
	return true
}

// queueItemID reads the id of the queue item from the Location of an accepted build request. Only its shape is checked.
func queueItemID(loc string) (int64, bool) {
	u, err := url.Parse(loc)
	if err != nil || len(loc) > 1024 {
		return 0, false
	}
	m := queueLocationRe.FindStringSubmatch(u.Path)
	if m == nil {
		return 0, false
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	return id, err == nil && id > 0
}

func (p impl) started(ctx context.Context, c *ghprovider.Call, m *civ1.StartOperationRequest, id string, st civ1.OperationState, existing bool) *civ1.StartOperationResponse {
	// The receipt names the run's reference, never the parameters' values or the log.
	receipt(ctx, c, "ci-run-ref", RecordType, &structpb.Struct{Fields: map[string]*structpb.Value{
		"repository":   structpb.NewStringValue(m.GetRepository()),
		"pipeline":     structpb.NewStringValue(m.GetPipeline()),
		"revision":     structpb.NewStringValue(m.GetRevision()),
		"operation_id": structpb.NewStringValue(id),
	}})
	return &civ1.StartOperationResponse{OperationId: id, State: st, Existing: existing}
}

// receipt submits one evidence record. The run has already happened, so a record the host would not take does not undo it and
// does not fail the call: the call ends SUCCEEDED with no id for that record and a WARNING, and the host derives PARTIAL from the
// declared evidence that is missing. A retry under the same key records the receipt again.
func receipt(ctx context.Context, c *ghprovider.Call, key, recordType string, payload *structpb.Struct) {
	if err := c.Emit(ctx, key, recordType, payload); err != nil {
		c.Warn(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
	}
}

// find locates the run of an operation id; a run Jenkins does not show is NOT_FOUND.
func (p impl) find(ctx context.Context, cl client, job, op string) (*located, error) {
	run, err := p.locate(ctx, cl, job, op)
	if errors.Is(err, errNoRun) {
		return nil, ghprovider.Fail(pluginv1.Reason_REASON_NOT_FOUND)
	}
	return run, err
}

func (p impl) get(ctx context.Context, c *ghprovider.Call, m *civ1.GetOperationRequest) (*civ1.GetOperationResponse, error) {
	job, op, err := p.target(m.GetRepository(), m.GetOperationId())
	if err != nil {
		return nil, err
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	run, err := p.find(ctx, cl, job, op)
	if err != nil {
		return nil, err
	}
	resp := &civ1.GetOperationResponse{State: run.state, Revision: run.revision}
	if !run.started.IsZero() {
		resp.StartedAt = timestamppb.New(run.started)
	}
	if ended(run.state) {
		if run.finished.IsZero() {
			// The port promises an end time for an ended run; Jenkins did not give the times to derive it.
			return nil, upstream()
		}
		resp.FinishedAt = timestamppb.New(run.finished)
	}
	return resp, nil
}

func (p impl) cancel(ctx context.Context, c *ghprovider.Call, m *civ1.CancelOperationRequest) (*civ1.CancelOperationResponse, error) {
	job, op, err := p.target(m.GetRepository(), m.GetOperationId())
	if err != nil {
		return nil, err
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	run, err := p.find(ctx, cl, job, op)
	if err != nil {
		return nil, err
	}
	if ended(run.state) {
		// An ended run is answered as it is: cancelling it changes nothing.
		return &civ1.CancelOperationResponse{State: run.state}, nil
	}
	crumb, err := p.crumb(ctx, cl)
	if err != nil {
		return nil, err
	}
	cancelledQueued, emitted := false, false
	for attempt := 0; ; attempt++ {
		req := request{method: http.MethodPost, header: crumb, okRedirect: true}
		// Only what this very request cancelled counts: a queue item that has since become a build is not "cancelled by Jenkins".
		cancelledQueued = run.queueID > 0
		if cancelledQueued {
			req.path, req.query = "/queue/cancelItem", url.Values{"id": {itoa(run.queueID)}}
		} else {
			req.path = job + "/" + itoa(run.number) + "/stop"
		}
		if _, err := cl.do(ctx, req, nil); err != nil {
			return nil, failure(ctx, err)
		}
		c.Wrote(pluginv1.SideEffect_SIDE_EFFECT_WRITE, m.GetRepository())
		if !emitted {
			emitted = true
			receipt(ctx, c, "action", ciAction, &structpb.Struct{Fields: map[string]*structpb.Value{
				"capability":   structpb.NewStringValue("ciexecutor.cancel_operation"),
				"repository":   structpb.NewStringValue(m.GetRepository()),
				"operation_id": structpb.NewStringValue(m.GetOperationId()),
			}})
		}
		// Jenkins stops a build asynchronously: the result becomes ABORTED a moment after the request.
		select {
		case <-ctx.Done():
			return nil, ghprovider.Failure(ctx, ctx.Err())
		case <-time.After(p.poll):
		}
		after, err := p.locate(ctx, cl, job, op)
		switch {
		case errors.Is(err, errNoRun) && cancelledQueued:
			// A cancelled queue item leaves no trace in the lists read here: not in the queue and never a build.
			return &civ1.CancelOperationResponse{State: civ1.OperationState_OPERATION_STATE_CANCELLED}, nil
		case cancelledQueued && isLimitExceeded(err):
			// All buildsPages pages of builds are full and none holds the run. After our cancel of its queue item that is the answer
			// too: a run that had left the queue first would head the first page, being the newest build.
			return &civ1.CancelOperationResponse{State: civ1.OperationState_OPERATION_STATE_CANCELLED}, nil
		case err != nil && errors.Is(err, errNoRun):
			return nil, upstream()
		case err != nil:
			return nil, err
		case ended(after.state) || attempt >= 2:
			return &civ1.CancelOperationResponse{State: after.state}, nil
		}
		run = after
	}
}

func isLimitExceeded(err error) bool {
	var ce *ghprovider.CallError
	return errors.As(err, &ce) && ce.Reason == pluginv1.Reason_REASON_LIMIT_EXCEEDED
}

func (p impl) artifacts(ctx context.Context, c *ghprovider.Call, m *civ1.ListArtifactsRequest) (*civ1.ListArtifactsResponse, error) {
	job, op, err := p.target(m.GetRepository(), m.GetOperationId())
	if err != nil {
		return nil, err
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	run, err := p.find(ctx, cl, job, op)
	if err != nil {
		return nil, err
	}
	resp := &civ1.ListArtifactsResponse{}
	if run.number == 0 {
		return resp, nil // still queued: nothing produced yet
	}
	build := job + "/" + itoa(run.number)
	var list struct {
		Artifacts []struct {
			RelativePath string `json:"relativePath"`
		} `json:"artifacts"`
	}
	if _, err := cl.do(ctx, request{method: http.MethodGet, path: build + "/api/json", query: url.Values{"tree": {"artifacts[relativePath]"}}}, &list); err != nil {
		return nil, failure(ctx, err)
	}
	if len(list.Artifacts) > 256 {
		return nil, ghprovider.Fail(pluginv1.Reason_REASON_LIMIT_EXCEEDED)
	}
	seen := map[string]bool{}
	for _, a := range list.Artifacts {
		segs := strings.Split(a.RelativePath, "/")
		ok := a.RelativePath != "" && len(a.RelativePath) <= 256 && !seen[a.RelativePath]
		for _, s := range segs {
			ok = ok && segment(s)
		}
		if !ok {
			return nil, upstream()
		}
		seen[a.RelativePath] = true
		size, err := p.artifactSize(ctx, cl, build, segs)
		if err != nil {
			return nil, err
		}
		// Jenkins reports no sha256 of an artifact (its fingerprints are MD5), so the digest stays empty.
		resp.Artifacts = append(resp.Artifacts, &civ1.Artifact{Name: a.RelativePath, SizeBytes: size})
	}
	sort.SliceStable(resp.Artifacts, func(i, j int) bool { return resp.Artifacts[i].Name < resp.Artifacts[j].Name })
	return resp, nil
}

// artifactSize reads the size of an artifact from the Content-Length of a HEAD request: the build's JSON lists no size. The
// content is never fetched. An answer with no usable length fails the call: the size is never guessed.
func (p impl) artifactSize(ctx context.Context, cl client, build string, segs []string) (uint64, error) {
	var b strings.Builder
	b.WriteString(build + "/artifact")
	for _, s := range segs {
		b.WriteString("/" + url.PathEscape(s))
	}
	rep, err := cl.do(ctx, request{method: http.MethodHead, path: b.String()}, nil)
	if err != nil {
		return 0, failure(ctx, err)
	}
	n, perr := strconv.ParseUint(rep.header.Get("Content-Length"), 10, 63)
	if perr != nil {
		return 0, upstream()
	}
	return n, nil
}

func (p impl) logReference(ctx context.Context, c *ghprovider.Call, m *civ1.GetLogReferenceRequest) (*civ1.GetLogReferenceResponse, error) {
	job, op, err := p.target(m.GetRepository(), m.GetOperationId())
	if err != nil {
		return nil, err
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	run, err := p.find(ctx, cl, job, op)
	if err != nil {
		return nil, err
	}
	if run.number == 0 {
		return nil, ghprovider.Fail(pluginv1.Reason_REASON_CONFLICT) // queued: there is no log yet
	}
	// Built from the configured server and the job path, never from the url Jenkins answers: its root URL setting is not a fact
	// this provider relies on, and a log reference must not point anywhere the manifest did not name.
	u := "https://" + p.cfg.Authority() + p.cfg.Prefix + job + "/" + itoa(run.number) + "/console"
	if len(u) > 1024 {
		return nil, upstream()
	}
	return &civ1.GetLogReferenceResponse{Url: u}, nil
}
