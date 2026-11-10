package provider_test

import (
	"context"
	"os"
	"testing"
	"time"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
	"github.com/numinance/numinance-spec/plugins/official/ticketing-jira/provider"
)

// This file is a harness for the first run against a real tenant. It is skipped unless the operator opts in, and it holds no
// credential: the provider never does. The operator runs their own egress proxy (or the broker's) that holds the read-scoped
// token, injects it for the handle named below and re-originates the request to the instance over TLS; the test only points the
// live transport at that proxy. It reads one issue and checks that a write is refused by the transport (read-only default).
//
// Variables (all required once NUMINANCE_JIRA_LIVE is "1"):
//
//	NUMINANCE_JIRA_LIVE         "1" opts in
//	NUMINANCE_JIRA_LIVE_HOST    the instance host, as `-host` of the live build
//	NUMINANCE_JIRA_LIVE_PROXY   the proxy's ip:port that accepts the handle
//	NUMINANCE_JIRA_LIVE_HANDLE  the credential handle the proxy accepts (an opaque handle, not the token)
//	NUMINANCE_JIRA_LIVE_PROJECT and NUMINANCE_JIRA_LIVE_ISSUE  a project and issue key the token may read

type tenantEnv struct{ host, proxy, handle, project, issue string }

func tenantFromEnv(t *testing.T) tenantEnv {
	t.Helper()
	if os.Getenv("NUMINANCE_JIRA_LIVE") != "1" {
		t.Skip("set NUMINANCE_JIRA_LIVE=1 and the NUMINANCE_JIRA_LIVE_* variables to run against a real tenant (see provider/live_tenant_test.go)")
	}
	e := tenantEnv{
		host: os.Getenv("NUMINANCE_JIRA_LIVE_HOST"), proxy: os.Getenv("NUMINANCE_JIRA_LIVE_PROXY"), handle: os.Getenv("NUMINANCE_JIRA_LIVE_HANDLE"),
		project: os.Getenv("NUMINANCE_JIRA_LIVE_PROJECT"), issue: os.Getenv("NUMINANCE_JIRA_LIVE_ISSUE"),
	}
	if e.host == "" || e.proxy == "" || e.handle == "" || e.project == "" || e.issue == "" {
		t.Fatal("NUMINANCE_JIRA_LIVE is set but a NUMINANCE_JIRA_LIVE_{HOST,PROXY,HANDLE,PROJECT,ISSUE} variable is empty")
	}
	if err := provider.ValidHost(e.host); err != nil {
		t.Fatalf("NUMINANCE_JIRA_LIVE_HOST is not a valid instance host: %v", err)
	}
	return e
}

// fixedHandleHost hands out the one handle the operator's proxy accepts.
type fixedHandleHost struct {
	*recordingHost
	handle string
}

func (h fixedHandleHost) Credential(context.Context, string, string) (string, error) {
	return h.handle, nil
}

func TestLiveTenant_ReadsOneIssueAndRefusesAWrite(t *testing.T) {
	e := tenantFromEnv(t)
	routes := ghprovider.NewRoutes(time.Now)
	if err := routes.Set(e.handle, e.proxy, time.Now().Add(5*time.Minute)); err != nil {
		t.Fatalf("NUMINANCE_JIRA_LIVE_PROXY is not an ip:port: %v", err)
	}
	host := fixedHandleHost{recordingHost: &recordingHost{}, handle: e.handle}
	srv, err := provider.New(provider.PluginID, e.host, "sha256:0", provider.Transport{RT: provider.NewLive(e.host, routes, false)}, host, nil)
	if err != nil {
		t.Fatal(err)
	}
	tg := &target{srv: srv, host: host.recordingHost, tap: &tap{}}

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	got, err := tg.GetIssue(ctx, ticketingkit.Options{}, &ticketingv1.GetIssueRequest{Project: e.project, IssueKey: e.issue, MaxBodyBytes: 1024})
	if err != nil {
		t.Fatalf("GetIssue() = %v, want the issue", err)
	}
	if got.GetIssueKey() != e.issue {
		t.Errorf("GetIssue().IssueKey = %q, want the key asked for", got.GetIssueKey())
	}
	// Never log the title or body: they are the tenant's data.

	_, _, err = tg.CommentOnIssue(ctx, ticketingkit.Options{Key: "live-harness"}, &ticketingv1.CommentOnIssueRequest{Project: e.project, IssueKey: e.issue, Body: "must not be sent"})
	if err == nil {
		t.Error("CommentOnIssue() succeeded on a read-only build, want it refused before anything was sent")
	}
}
