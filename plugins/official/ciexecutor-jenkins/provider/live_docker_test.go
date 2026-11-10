//go:build integration

package provider_test

// UNRUN IN THE AUTHORING ENVIRONMENT. This test was written and compiled (go vet -tags integration) where no Docker daemon was
// running, so it has never executed. Treat the first run as the first evidence, and expect to fix the fixture
// (testdata/jenkins-docker) before the provider: the Groovy init script and the fixed API token are the parts not checked.
//
// It builds a throwaway Jenkins controller (testdata/jenkins-docker, pinned by digest), runs it on a loopback port, and drives the
// live build of the provider against it for the reserved host jenkins.example.test through a small in-test plain HTTP egress proxy
// that accepts only that host on 443 and a known handle, strips the handle and injects Basic credentials built from a random
// per-run admin token (never a literal in the repo). Needs Docker and network access to pull the base image and the
// cloudbees-folder plugin.
//
// Run it with:
//
//	cd numinance-spec && NUMINANCE_JENKINS_DOCKER=1 go test -tags integration -run TestDockerJenkins ./plugins/official/ciexecutor-jenkins/provider/
//
// Without NUMINANCE_JENKINS_DOCKER=1, or when `docker info` fails, it is skipped.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/numinance/numinance-spec/plugins/official/ciexecutor-jenkins/provider"
	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

const dockerRevision = "c0ffee00c0ffee00c0ffee00c0ffee00c0ffee02"

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// docker runs the docker client with extra environment variables (so a secret is passed by name, never on the command line).
func docker(ctx context.Context, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...) //nolint:gosec // constant client name; the arguments are made by this test
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// dockerProxy is the egress proxy of the test. It refuses anything but the reserved host on 443 with a known handle, and anything
// that arrives with a header that authenticates (the provider holds none), then forwards over plain HTTP to the container.
type dockerProxy struct {
	ln       net.Listener
	handle   string
	target   string // 127.0.0.1:<port>
	user     string
	token    string
	mu       sync.Mutex
	refusals []string
	forwards int
}

func (p *dockerProxy) refuse(w http.ResponseWriter, why string) {
	p.mu.Lock()
	p.refusals = append(p.refusals, why)
	p.mu.Unlock()
	w.WriteHeader(http.StatusForbidden)
}

func (p *dockerProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Scheme != "http" || r.URL.Host != exampleHost+":443":
		p.refuse(w, "destination")
		return
	case r.Header.Get(ghprovider.HandleHeader) != p.handle:
		p.refuse(w, "handle")
		return
	case r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Proxy-Authorization") != "":
		p.refuse(w, "the provider sent a credential header")
		return
	}
	out, err := http.NewRequestWithContext(r.Context(), r.Method, "http://"+p.target+r.URL.RequestURI(), r.Body) //nolint:gosec // the target is the loopback container this test started
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	for k, v := range r.Header {
		if !strings.EqualFold(k, ghprovider.HandleHeader) {
			out.Header[k] = v
		}
	}
	out.SetBasicAuth(p.user, p.token)
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(out) //nolint:gosec // the destination is the loopback container this test started
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	p.mu.Lock()
	p.forwards++
	p.mu.Unlock()
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// startJenkinsContainer builds and runs the fixture and returns the loopback address of its HTTP port.
func startJenkinsContainer(t *testing.T, user, token string) string {
	t.Helper()
	ctx := t.Context()
	image := "numinance-jenkins-fixture:" + randomHex(t, 4)
	if out, err := docker(ctx, nil, "build", "-t", image, "../testdata/jenkins-docker"); err != nil {
		t.Fatalf("docker build: %v\n%s", err, out)
	}
	t.Cleanup(func() { _, _ = docker(context.WithoutCancel(ctx), nil, "rmi", "-f", image) })

	env := []string{"NUMINANCE_TEST_ADMIN_USER=" + user, "NUMINANCE_TEST_ADMIN_TOKEN=" + token}
	id, err := docker(ctx, env, "run", "-d", "-p", "127.0.0.1:0:8080", "-e", "NUMINANCE_TEST_ADMIN_USER", "-e", "NUMINANCE_TEST_ADMIN_TOKEN", image)
	if err != nil {
		t.Fatalf("docker run: %v\n%s", err, id)
	}
	id = strings.Fields(id)[len(strings.Fields(id))-1]
	t.Cleanup(func() { _, _ = docker(context.WithoutCancel(ctx), nil, "rm", "-f", id) })

	mapped, err := docker(ctx, nil, "port", id, "8080/tcp")
	if err != nil {
		t.Fatalf("docker port: %v\n%s", err, mapped)
	}
	line, _, _ := strings.Cut(mapped, "\n")
	_, port, err := net.SplitHostPort(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("docker port answered %q, want host:port", line)
	}
	addr := net.JoinHostPort("127.0.0.1", port)

	deadline := time.Now().Add(3 * time.Minute)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/login", http.NoBody)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return addr
			}
		}
		if time.Now().After(deadline) {
			logs, _ := docker(ctx, nil, "logs", "--tail", "40", id)
			t.Fatalf("Jenkins did not answer /login with 200 within 3 minutes; container log tail:\n%s", logs)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for Jenkins: %v", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func TestDockerJenkins_StartRepeatRunCancel(t *testing.T) {
	if os.Getenv("NUMINANCE_JENKINS_DOCKER") != "1" {
		t.Skip("set NUMINANCE_JENKINS_DOCKER=1 to run against a Jenkins container (see the header of provider/live_docker_test.go)")
	}
	if out, err := docker(t.Context(), nil, "info"); err != nil {
		t.Skipf("docker info failed, no usable Docker daemon: %v (%.80s)", err, out)
	}

	user, token := "numinance-"+randomHex(t, 4), "11"+randomHex(t, 16)
	addr := startJenkinsContainer(t, user, token)

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	px := &dockerProxy{ln: ln, handle: "docker-" + randomHex(t, 8), target: addr, user: user, token: token}
	srv := &http.Server{Handler: px, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	cfg, err := provider.ParseURL("https://" + exampleHost)
	if err != nil {
		t.Fatal(err)
	}
	routes := ghprovider.NewRoutes(time.Now)
	host := fixedHandleHost{routes: routes, proxy: ln.Addr().String(), handle: px.handle}
	tg := liveServer(t, cfg, host, routes)

	runLiveScenario(t, tg, liveScenario{
		repo: "acme/widgets", pipe: "release", dryPipe: "dry", rev: dockerRevision, start: true, wantRunning: true, host: cfg.Authority(),
	})

	px.mu.Lock()
	defer px.mu.Unlock()
	if len(px.refusals) != 0 {
		t.Errorf("the proxy refused %d requests (%v), want none", len(px.refusals), px.refusals)
	}
	if px.forwards < 10 {
		t.Errorf("the proxy forwarded %d requests, want the whole scenario", px.forwards)
	}
}
