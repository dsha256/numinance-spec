package provider_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	cikit "github.com/numinance/numinance-spec/conformance/ciexecutor/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	civ1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ciexecutor/v1"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// This file is a harness for the first run against a real Jenkins controller. It is skipped unless the operator opts in, and it holds
// no credential: the provider never does. The operator runs their own egress proxy (or the broker's) that holds the Jenkins user
// and API token, injects it for the handle named below and re-originates the request to the controller over TLS; the test only
// points the live transport at that proxy. It has not been run: no controller was available where it was written.
//
// Starting a build is a side effect on a real system, so it is a second opt-in. Without NUMINANCE_JENKINS_LIVE_START the test
// only does a dry run of StartOperation: it checks that the job exists, can be built and declares the numinance_* parameters, and
// starts nothing.
//
// Variables (all required once NUMINANCE_JENKINS_LIVE is "1"):
//
//	NUMINANCE_JENKINS_LIVE           "1" opts in
//	NUMINANCE_JENKINS_LIVE_URL       the controller, as `-url` of the live build (https://host[/context])
//	NUMINANCE_JENKINS_LIVE_PROXY     the proxy's ip:port that accepts the handle
//	NUMINANCE_JENKINS_LIVE_HANDLE    the credential handle the proxy accepts (an opaque handle, not the token)
//	NUMINANCE_JENKINS_LIVE_REPO      <owner>/<name>, the folder path of the repository's jobs
//	NUMINANCE_JENKINS_LIVE_PIPELINE  the job under that folder (a parameterized job with the numinance_* parameters)
//	NUMINANCE_JENKINS_LIVE_REVISION  a full commit id (40 or 64 hex) the job may be built for
//	NUMINANCE_JENKINS_LIVE_START     "1" additionally runs the steps that start, read, cancel and locate a real build

const liveStartTimeout = 2 * time.Minute

type controllerEnv struct {
	cfg                            provider.Config
	proxy, handle, repo, pipe, rev string
	start                          bool
}

// parseControllerEnv reads the harness variables through get. It reports opted false (and nothing else) when the operator has not
// opted in, and an error when one is empty or the URL is not a valid -url.
func parseControllerEnv(get func(string) string) (e controllerEnv, opted bool, err error) {
	if get("NUMINANCE_JENKINS_LIVE") != "1" {
		return controllerEnv{}, false, nil
	}
	raw := get("NUMINANCE_JENKINS_LIVE_URL")
	e = controllerEnv{
		proxy: get("NUMINANCE_JENKINS_LIVE_PROXY"), handle: get("NUMINANCE_JENKINS_LIVE_HANDLE"), repo: get("NUMINANCE_JENKINS_LIVE_REPO"),
		pipe: get("NUMINANCE_JENKINS_LIVE_PIPELINE"), rev: get("NUMINANCE_JENKINS_LIVE_REVISION"), start: get("NUMINANCE_JENKINS_LIVE_START") == "1",
	}
	if raw == "" || e.proxy == "" || e.handle == "" || e.repo == "" || e.pipe == "" || e.rev == "" {
		return controllerEnv{}, true, errors.New("NUMINANCE_JENKINS_LIVE is set but a NUMINANCE_JENKINS_LIVE_{URL,PROXY,HANDLE,REPO,PIPELINE,REVISION} variable is empty")
	}
	if e.cfg, err = provider.ParseURL(raw); err != nil {
		return controllerEnv{}, true, fmt.Errorf("NUMINANCE_JENKINS_LIVE_URL is not a valid controller url: %w", err)
	}
	return e, true, nil
}

func TestControllerEnv(t *testing.T) {
	t.Parallel()
	full := map[string]string{
		"NUMINANCE_JENKINS_LIVE": "1", "NUMINANCE_JENKINS_LIVE_URL": "https://jenkins.example.test", "NUMINANCE_JENKINS_LIVE_PROXY": "127.0.0.1:9",
		"NUMINANCE_JENKINS_LIVE_HANDLE": "h", "NUMINANCE_JENKINS_LIVE_REPO": "acme/shop", "NUMINANCE_JENKINS_LIVE_PIPELINE": "release",
		"NUMINANCE_JENKINS_LIVE_REVISION": rev,
	}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for kk, vv := range full {
			m[kk] = vv
		}
		m[k] = v
		return m
	}
	for _, tc := range []struct {
		name      string
		env       map[string]string
		wantOpted bool
		wantErr   bool
		wantStart bool
	}{
		{name: "skips without the opt-in even when every other variable is set", env: with("NUMINANCE_JENKINS_LIVE", ""), wantOpted: false},
		{name: "skips on no variables at all", env: nil, wantOpted: false},
		{name: "any other opt-in value is not an opt-in", env: with("NUMINANCE_JENKINS_LIVE", "true"), wantOpted: false},
		{name: "reads a full environment, start off by default", env: full, wantOpted: true},
		{name: "the start flag is a second opt-in", env: with("NUMINANCE_JENKINS_LIVE_START", "1"), wantOpted: true, wantStart: true},
		{name: "an empty variable is an error once opted in", env: with("NUMINANCE_JENKINS_LIVE_HANDLE", ""), wantOpted: true, wantErr: true},
		{name: "an empty revision is an error", env: with("NUMINANCE_JENKINS_LIVE_REVISION", ""), wantOpted: true, wantErr: true},
		{name: "an http url is an error", env: with("NUMINANCE_JENKINS_LIVE_URL", "http://jenkins.example.test"), wantOpted: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, opted, err := parseControllerEnv(func(k string) string { return tc.env[k] })
			if opted != tc.wantOpted || (err != nil) != tc.wantErr {
				t.Fatalf("parseControllerEnv() = opted %v, err %v; want opted %v, error %v", opted, err, tc.wantOpted, tc.wantErr)
			}
			if err == nil && opted && (e.start != tc.wantStart || e.repo != "acme/shop") {
				t.Errorf("parseControllerEnv() = start %v repo %q, want start %v and the repo given", e.start, e.repo, tc.wantStart)
			}
		})
	}
}

// fixedHandleHost hands out the one handle the operator's proxy accepts, and keeps its route to that proxy alive for the run.
type fixedHandleHost struct {
	routes        *ghprovider.Routes
	proxy, handle string
}

func (h fixedHandleHost) Credential(context.Context, string, string) (string, error) {
	if err := h.routes.Set(h.handle, h.proxy, time.Now().Add(5*time.Minute)); err != nil {
		return "", err
	}
	return h.handle, nil
}

func (h fixedHandleHost) Emit(context.Context, string, string, string, *structpb.Struct) (string, error) {
	return "ev_live", nil
}

// liveServer is the provider on the live transport for the controller cfg, through the proxy of the operator.
func liveServer(t *testing.T, cfg provider.Config, host ghprovider.Host, routes *ghprovider.Routes) *kitTarget {
	t.Helper()
	srv := provider.New(provider.PluginID, "sha256:0", cfg, provider.Transport{RT: provider.NewLive(cfg, routes)}, time.Second, host, nil)
	return &kitTarget{s: srv}
}

func TestLiveController_DryRunAndOptionalBuild(t *testing.T) {
	e, opted, err := parseControllerEnv(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if !opted {
		t.Skip("set NUMINANCE_JENKINS_LIVE=1 and the NUMINANCE_JENKINS_LIVE_* variables to run against a real controller (see provider/live_controller_test.go)")
	}
	routes := ghprovider.NewRoutes(time.Now)
	host := fixedHandleHost{routes: routes, proxy: e.proxy, handle: e.handle}
	if _, err := host.Credential(t.Context(), "", ""); err != nil {
		t.Fatalf("NUMINANCE_JENKINS_LIVE_PROXY is not an ip:port: %v", err)
	}
	tg := liveServer(t, e.cfg, host, routes)
	if !e.start {
		t.Log("NUMINANCE_JENKINS_LIVE_START is not 1: dry run only, no build is started")
	}
	runLiveScenario(t, tg, liveScenario{repo: e.repo, pipe: e.pipe, dryPipe: e.pipe, rev: e.rev, start: e.start, host: e.cfg.Authority()})
}

// liveScenario is the run both live harnesses make against a real controller (the one of the operator, and the container one).
type liveScenario struct {
	repo, pipe, dryPipe, rev string
	// start runs the steps that start a build; without it only the dry run is made.
	start bool
	// wantRunning demands that the started build reaches RUNNING, that its cancel answers CANCELLED and that it has no artifacts (the
	// container fixture's job sleeps); the operator's job may finish at once, so there any end state is accepted.
	wantRunning bool
	// every is the pause between two looks at the run (2 s when zero).
	every time.Duration
	// host is the authority of the controller, for the log reference.
	host string
}

func uniqueKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "live-harness/" + hex.EncodeToString(b)
}

// runLiveScenario never logs a title, a parameter or log content: they are the controller's data.
func runLiveScenario(t *testing.T, tg cikit.Target, sc liveScenario) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), liveStartTimeout+3*time.Minute)
	defer cancel()

	dry, err := tg.StartOperation(ctx, cikit.Options{DryRun: true, Key: uniqueKey(t)}, &civ1.StartOperationRequest{Repository: sc.repo, Pipeline: sc.dryPipe, Revision: sc.rev})
	if err != nil {
		t.Fatalf("StartOperation(dry run) = %v, want the job checked (parameterized with the numinance_* parameters)", err)
	}
	if dry.GetExisting() || dry.GetOperationId() != "" {
		t.Errorf("StartOperation(dry run) = existing %v id %q, want a plan that starts nothing", dry.GetExisting(), dry.GetOperationId())
	}
	if !sc.start {
		return
	}

	every := sc.every
	if every == 0 {
		every = 2 * time.Second
	}
	key := uniqueKey(t)
	req := &civ1.StartOperationRequest{Repository: sc.repo, Pipeline: sc.pipe, Revision: sc.rev}
	first, err := tg.StartOperation(ctx, cikit.Options{Key: key}, req)
	if err != nil {
		t.Fatalf("StartOperation() = %v, want a build requested", err)
	}
	id := first.GetOperationId()
	if id == "" || first.GetExisting() {
		t.Fatalf("StartOperation() = id %q existing %v, want a new run", id, first.GetExisting())
	}
	// From here the build exists: whatever fails, try to stop it.
	cancelled := false
	t.Cleanup(func() {
		if cancelled {
			return
		}
		cctx, ccancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer ccancel()
		_, _ = tg.CancelOperation(cctx, cikit.Options{Key: key + "/cleanup"}, &civ1.CancelOperationRequest{Repository: sc.repo, OperationId: id})
	})

	again, err := tg.StartOperation(ctx, cikit.Options{Key: key}, req)
	if err != nil {
		t.Fatalf("StartOperation(repeat) = %v, want the same run", err)
	}
	if !again.GetExisting() || again.GetOperationId() != id {
		t.Errorf("StartOperation(repeat) = existing %v same id %v, want existing and the same operation id (no second build)", again.GetExisting(), again.GetOperationId() == id)
	}

	state := first.GetState()
	deadline := time.Now().Add(liveStartTimeout)
	for state == civ1.OperationState_OPERATION_STATE_QUEUED && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for the build to leave the queue: %v", ctx.Err())
		case <-time.After(every):
		}
		got, err := tg.GetOperation(ctx, cikit.Options{}, &civ1.GetOperationRequest{Repository: sc.repo, OperationId: id})
		if err != nil {
			t.Fatalf("GetOperation() = %v, want the run", err)
		}
		state = got.GetState()
	}
	t.Logf("state after waiting: %s", state)
	if sc.wantRunning && state != civ1.OperationState_OPERATION_STATE_RUNNING {
		t.Errorf("GetOperation().State = %s within %s, want RUNNING", state, liveStartTimeout)
	}

	logRef, logErr := tg.GetLogReference(ctx, cikit.Options{}, &civ1.GetLogReferenceRequest{Repository: sc.repo, OperationId: id})
	if state == civ1.OperationState_OPERATION_STATE_QUEUED {
		// A queued run has no log yet.
		var ce *cikit.CallError
		if !errors.As(logErr, &ce) || ce.Reason != pluginv1.Reason_REASON_CONFLICT {
			t.Errorf("GetLogReference(queued) = %v, want CONFLICT", logErr)
		}
	} else {
		if logErr != nil {
			t.Fatalf("GetLogReference() = %v, want the console url", logErr)
		}
		if u := logRef.GetUrl(); !strings.HasPrefix(u, "https://"+sc.host+"/") || !strings.HasSuffix(u, "/console") {
			t.Errorf("GetLogReference().Url has the wrong shape, want https://%s/.../console", sc.host)
		}
	}

	if sc.wantRunning {
		arts, err := tg.ListArtifacts(ctx, cikit.Options{}, &civ1.ListArtifactsRequest{Repository: sc.repo, OperationId: id})
		if err != nil {
			t.Errorf("ListArtifacts() = %v, want none", err)
		} else if len(arts.GetArtifacts()) != 0 {
			t.Errorf("ListArtifacts() = %d artifacts, want none for a running build", len(arts.GetArtifacts()))
		}
	}

	cancelled = true
	stopped, err := tg.CancelOperation(ctx, cikit.Options{Key: key + "/cancel"}, &civ1.CancelOperationRequest{Repository: sc.repo, OperationId: id})
	if err != nil {
		cancelled = false
		t.Fatalf("CancelOperation() = %v, want the run stopped", err)
	}
	endState := stopped.GetState()
	t.Logf("state after cancel: %s", endState)
	if sc.wantRunning && endState != civ1.OperationState_OPERATION_STATE_CANCELLED {
		t.Errorf("CancelOperation().State = %s, want CANCELLED", endState)
	}
	wasQueued := state == civ1.OperationState_OPERATION_STATE_QUEUED
	for end := time.Now().Add(30 * time.Second); ; {
		got, err := tg.GetOperation(ctx, cikit.Options{}, &civ1.GetOperationRequest{Repository: sc.repo, OperationId: id})
		var ce *cikit.CallError
		if wasQueued && errors.As(err, &ce) && ce.Reason == pluginv1.Reason_REASON_NOT_FOUND {
			return // a cancelled queue item leaves no trace
		}
		if err != nil {
			t.Fatalf("GetOperation(after cancel) = %v, want the end state", err)
		}
		if got.GetState() >= civ1.OperationState_OPERATION_STATE_SUCCEEDED {
			if sc.wantRunning && got.GetState() != civ1.OperationState_OPERATION_STATE_CANCELLED {
				t.Errorf("GetOperation(after cancel).State = %s, want CANCELLED", got.GetState())
			}
			return
		}
		if time.Now().After(end) {
			t.Fatalf("GetOperation(after cancel).State = %s after 30s, want an end state", got.GetState())
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for the end state: %v", ctx.Err())
		case <-time.After(every):
		}
	}
}

// advancing lets the queued run become a running build after the first look at it, as a free executor does.
type advancing struct {
	cikit.Target
	f *fake
}

func (a advancing) GetOperation(ctx context.Context, o cikit.Options, r *civ1.GetOperationRequest) (*civ1.GetOperationResponse, error) {
	resp, err := a.Target.GetOperation(ctx, o, r)
	a.f.mu.Lock()
	queued := len(a.f.queue) > 0
	a.f.mu.Unlock()
	if queued {
		a.f.run()
	}
	return resp, err
}

// The scenario of both live harnesses, run against the stateful fake so that its own logic is executed.
func TestLiveScenario_AgainstTheFake(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.extraJobs = []string{"dry"}
	tg := advancing{Target: &kitTarget{s: f.server(&host{})}, f: f}
	runLiveScenario(t, tg, liveScenario{repo: repo, pipe: "release", dryPipe: "dry", rev: rev, start: true, wantRunning: true, every: time.Millisecond, host: "jenkins.example.test"})
	if got := f.posts(); got != 2 {
		t.Errorf("POSTs = %d, want the build request and the stop", got)
	}
}
