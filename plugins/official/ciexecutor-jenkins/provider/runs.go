package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// The pipeline contract (README, "What a pipeline must do"). Jenkins starts a parameterized job and answers with a queue item,
// never with an id that survives the queue, and it has no idempotency key. So the provider passes the commit to build and an
// operation id derived from the host's key as two job parameters (numinance_revision, numinance_operation_id), and finds the run
// again by that parameter in the queue and in the job's recent builds. The job is the folder path <owner>/<name> and the
// pipeline name under it.
const (
	// buildsWindow is how many recent builds one look at the job reads (Jenkins's own window of the builds property). A full
	// window that does not hold the operation may hold it further back, so the look fails closed (LIMIT_EXCEEDED).
	buildsWindow = 100

	treeQueue = "items[id,task[name,url],actions[parameters[name,value]]]"
	treeBuild = "builds[number,building,result,timestamp,duration,actions[parameters[name,value]]]{0,100}"
	treeJob   = "buildable,property[parameterDefinitions[name]]"
)

// errNoRun is locate's answer when neither the queue nor the recent builds carry the operation.
var errNoRun = errors.New("no run of the operation")

// jobPath is the URL path of the job of a pipeline: /job/<owner>/job/<name>/job/<segment>... Each name is one path segment, escaped.
func jobPath(owner, name, pipe string) string {
	var b strings.Builder
	for _, seg := range append([]string{owner, name}, strings.Split(pipe, "/")...) {
		b.WriteString("/job/" + url.PathEscape(seg))
	}
	return b.String()
}

// opDigest derives the 32 hex characters of the operation id from the host's key, bound to the repository and the pipeline so
// one key can never name a run of another job.
func opDigest(key, repo, pipe string) string {
	sum := sha256.Sum256([]byte(key + "\x00" + repo + "\x00" + pipe))
	return hex.EncodeToString(sum[:16])
}

func operationID(op, pipe string) string { return op + "." + strings.ReplaceAll(pipe, "/", ":") }

// parseOperation splits an operation id the provider issued into the digest and the pipeline; anything else is invalid.
func parseOperation(id string) (op, pipe string, err error) {
	m := operationRe.FindStringSubmatch(id)
	if m == nil {
		return "", "", invalid()
	}
	pipe = strings.ReplaceAll(m[2], ":", "/")
	if err := pipeline(pipe); err != nil {
		return "", "", err
	}
	return m[1], pipe, nil
}

// paramsAction is the parameters Jenkins records on a queue item or a build.
type paramsAction struct {
	Parameters []struct {
		Name  string `json:"name"`
		Value any    `json:"value"`
	} `json:"parameters"`
}

// paramValue returns the string value of the named parameter, or "" when there is none or it is not a string.
func paramValue(actions []paramsAction, name string) string {
	for _, a := range actions {
		for _, p := range a.Parameters {
			if p.Name != name {
				continue
			}
			if s, ok := p.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

// located is a run of the operation: in the queue (queueID > 0) or a build (number > 0).
type located struct {
	state             civ1.OperationState
	queueID, number   int64
	revision          string
	started, finished time.Time
}

func ended(st civ1.OperationState) bool {
	return st >= civ1.OperationState_OPERATION_STATE_SUCCEEDED && st <= civ1.OperationState_OPERATION_STATE_TIMED_OUT
}

// buildState maps the build and result Jenkins reports. A build without a result is running; only SUCCESS is a success, ABORTED is a
// cancel, and everything else (FAILURE, UNSTABLE, NOT_BUILT, a result this code does not know) is a failure, never a success.
// Jenkins reports a timeout (the Pipeline timeout step, the build-timeout plugin) as ABORTED, so TIMED_OUT is never produced.
func buildState(building bool, result *string) civ1.OperationState {
	switch {
	case building || result == nil:
		return civ1.OperationState_OPERATION_STATE_RUNNING
	case *result == "SUCCESS":
		return civ1.OperationState_OPERATION_STATE_SUCCEEDED
	case *result == "ABORTED":
		return civ1.OperationState_OPERATION_STATE_CANCELLED
	}
	return civ1.OperationState_OPERATION_STATE_FAILED
}

// jobName is the short name of the job a path names: the last /job/<segment>, unescaped.
func jobName(job string) string {
	seg := job[strings.LastIndex(job, "/")+1:]
	name, err := url.PathUnescape(seg)
	if err != nil {
		return ""
	}
	return name
}

// itemIsJob reports whether a queue item is a run of the job. The queue is global (every job of the server), so the job of the item
// decides. The task URL of Jenkins is <root URL>/job/<a>/job/<b>, and its root part is Jenkins's own setting (a reverse proxy may mount
// it at another path than -url), so the item is ours when the URL path ENDS with our job path and what precedes that is the
// configured context path, or a root mount without a /job/ segment. A path that does not end with ours is another job. A path that ends
// with ours but has a /job/ segment before it is ambiguous (a parent folder, or a root mount that contains /job/ and is not the
// configured one): that is an error, never an adoption and never a silent skip, so no second run is started on a doubt. Without a
// task URL the task's short name is compared; with neither, the item is not ours.
func (p impl) itemIsJob(taskURL, taskName, job string) (bool, error) {
	if taskURL == "" {
		return taskName != "" && taskName == jobName(job), nil
	}
	u, err := url.Parse(taskURL)
	if err != nil {
		return false, upstream() // our operation id on an item whose job can not be read: fail closed
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	if !strings.HasSuffix(path, job) {
		return false, nil
	}
	before := strings.TrimSuffix(path, job)
	if before == "" || before == p.cfg.Prefix || !strings.Contains(before+"/", "/job/") {
		return true, nil
	}
	return false, upstream()
}

func epoch(ms int64) time.Time {
	if ms <= 0 || ms > 1<<53 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

// locate looks for the run of an operation: first in the queue, then in the job's recent builds. The order matters: a run leaves the
// queue for a build, never the other way, so a run missed in the queue is already a build by the second look.
func (p impl) locate(ctx context.Context, cl client, job, op string) (*located, error) {
	var q struct {
		Items []struct {
			ID   int64 `json:"id"`
			Task struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"task"`
			Actions []paramsAction `json:"actions"`
		} `json:"items"`
	}
	if _, err := cl.do(ctx, request{method: http.MethodGet, path: "/queue/api/json", query: url.Values{"tree": {treeQueue}}}, &q); err != nil {
		return nil, failure(ctx, err)
	}
	for _, it := range q.Items {
		// Both must match: our operation id (the id binds the repository and the pipeline, but is not secret) and the job of the item.
		// An item of another job that carries our value is skipped, never adopted and never an error, so it can neither shadow the run
		// nor block a call. A forgery by someone who can build this very job is an accepted limit (README).
		if paramValue(it.Actions, paramOperation) != op {
			continue
		}
		ours, err := p.itemIsJob(it.Task.URL, it.Task.Name, job)
		if err != nil {
			return nil, err
		}
		if !ours {
			continue
		}
		rev := paramValue(it.Actions, paramRevision)
		if it.ID <= 0 || !revisionRe.MatchString(rev) {
			return nil, upstream()
		}
		return &located{state: civ1.OperationState_OPERATION_STATE_QUEUED, queueID: it.ID, revision: rev}, nil
	}
	var l struct {
		Builds []struct {
			Number    int64          `json:"number"`
			Building  bool           `json:"building"`
			Result    *string        `json:"result"`
			Timestamp int64          `json:"timestamp"`
			Duration  int64          `json:"duration"`
			Actions   []paramsAction `json:"actions"`
		} `json:"builds"`
	}
	if _, err := cl.do(ctx, request{method: http.MethodGet, path: job + "/api/json", query: url.Values{"tree": {treeBuild}}}, &l); err != nil {
		return nil, failure(ctx, err)
	}
	for _, b := range l.Builds {
		if paramValue(b.Actions, paramOperation) != op {
			continue
		}
		rev := paramValue(b.Actions, paramRevision)
		if b.Number <= 0 || b.Number > 1<<53 || !revisionRe.MatchString(rev) {
			return nil, upstream()
		}
		out := &located{state: buildState(b.Building, b.Result), number: b.Number, revision: rev, started: epoch(b.Timestamp)}
		if ended(out.state) && !out.started.IsZero() && b.Duration >= 0 {
			out.finished = out.started.Add(time.Duration(b.Duration) * time.Millisecond)
		}
		return out, nil
	}
	if len(l.Builds) >= buildsWindow {
		return nil, ghprovider.Fail(pluginv1.Reason_REASON_LIMIT_EXCEEDED)
	}
	return nil, errNoRun
}

// checkJob reads the job and fails unless it exists, can be built and declares every parameter the provider will send.
func (p impl) checkJob(ctx context.Context, cl client, job string, given map[string]string) error {
	var info struct {
		Buildable bool `json:"buildable"`
		Property  []struct {
			Definitions []struct {
				Name string `json:"name"`
			} `json:"parameterDefinitions"`
		} `json:"property"`
	}
	if _, err := cl.do(ctx, request{method: http.MethodGet, path: job + "/api/json", query: url.Values{"tree": {treeJob}}}, &info); err != nil {
		return failure(ctx, err)
	}
	defined := map[string]bool{}
	for _, pr := range info.Property {
		for _, d := range pr.Definitions {
			defined[d.Name] = true
		}
	}
	ok := info.Buildable && defined[paramRevision] && defined[paramOperation]
	for k := range given {
		ok = ok && defined[k]
	}
	if !ok {
		// A folder, a disabled job, or a job that does not declare the parameters the contract needs: not a pipeline of this provider.
		return ghprovider.Fail(pluginv1.Reason_REASON_CONFLICT)
	}
	return nil
}

// crumb asks Jenkins for a CSRF crumb. A server with crumbs off answers 404 and none is sent. What it answers is a header, so
// both parts are checked against a narrow grammar.
func (p impl) crumb(ctx context.Context, cl client) (map[string]string, error) {
	var c struct {
		Crumb string `json:"crumb"`
		Field string `json:"crumbRequestField"`
	}
	if _, err := cl.do(ctx, request{method: http.MethodGet, path: "/crumbIssuer/api/json"}, &c); err != nil {
		if isStatus(err, http.StatusNotFound) {
			return map[string]string{}, nil
		}
		return nil, failure(ctx, err)
	}
	if !crumbFieldRe.MatchString(c.Field) || reservedHeader(c.Field) || !crumbRe.MatchString(c.Crumb) {
		return nil, upstream()
	}
	return map[string]string{c.Field: c.Crumb}, nil
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
