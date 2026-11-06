package provider_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	verifierkit "github.com/numinance/numinance-spec/conformance/verifier/v1"
	"github.com/numinance/numinance-spec/plugins/official/verifier-python/provider"
	"github.com/numinance/numinance-spec/plugins/official/verifier-python/replay"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	verifierv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// fakeHost resolves config references from a table and records what the provider emitted.
type fakeHost struct {
	configs map[string][]byte
	emitted []*structpb.Struct
	over    bool
}

func (h *fakeHost) Resolve(_ context.Context, _ string, ref *typesv1.Reference, _ int) ([]byte, error) {
	if h.over {
		return nil, errors.New("over")
	}
	b, ok := h.configs[ref.GetSha256().GetValue()]
	if !ok {
		return nil, errors.New("not found")
	}
	return b, nil
}

func (h *fakeHost) Emit(_ context.Context, _, _, _ string, p *structpb.Struct) (string, error) {
	h.emitted = append(h.emitted, p)
	return "ev_test", nil
}

func configRef(h *fakeHost, check string) *typesv1.Reference {
	b, _ := json.Marshal(provider.Config{Check: check, Settings: ""})
	ref := replay.CandidateRef("config-" + check)
	ref.ArtifactRef, ref.ArtifactType = "config/"+check, "config"
	h.configs[ref.GetSha256().GetValue()] = b
	return ref
}

func newTarget(t *testing.T) (*fakeHost, *provider.Server) {
	t.Helper()
	h := &fakeHost{configs: map[string][]byte{}}
	return h, provider.New(provider.ReplayPluginID, "sha256:"+string(make([]byte, 0)), replay.Runner{}, h, nil)
}

func run(t *testing.T, s *provider.Server, cand string, cfg *typesv1.Reference) (*verifierv1.RunResponse, error) {
	t.Helper()
	resp, _, err := s.Run(t.Context(), "inv", &verifierv1.RunRequest{CandidateRef: replay.CandidateRef(cand), ConfigRef: cfg})
	return resp, err
}

func reason(err error) pluginv1.Reason {
	var ce *provider.CallError
	if errors.As(err, &ce) {
		return ce.Reason
	}
	return pluginv1.Reason_REASON_UNSPECIFIED
}

func TestServer_Run_SeededDefectsOfEveryCheckAreCaught(t *testing.T) {
	want := map[string]string{
		"ruff": "ruff/F401", "mypy": "mypy/return-value", "semgrep": "semgrep/no-eval", "import-linter": "import-linter/domain-does-not-import-infra", "pytest": "pytest/test-failed",
	}
	for check, rule := range want {
		h, s := newTarget(t)
		resp, err := run(t, s, replay.Seeded, configRef(h, check))
		if err != nil {
			t.Errorf("Run(%s, seeded) error = %v, want findings", check, err)
			continue
		}
		if !slices.ContainsFunc(resp.GetItems(), func(f *verifierv1.Finding) bool { return f.GetRuleId() == rule }) {
			t.Errorf("Run(%s, seeded) = %v, want a finding of %s", check, resp.GetItems(), rule)
		}
		if err := verifierkit.ValidateResponse(resp); err != nil {
			t.Errorf("Run(%s, seeded): %v", check, err)
		}
		if len(h.emitted) != 1 || h.emitted[0].GetFields()["replayed"].GetBoolValue() != true || h.emitted[0].GetFields()["check"].GetStringValue() != check {
			t.Errorf("Run(%s, seeded) emitted %v, want one VerifierReport that says it was replayed", check, h.emitted)
		}
	}
}

func TestServer_Run_ACleanCandidateHasNoFindings(t *testing.T) {
	for _, check := range []string{"ruff", "mypy", "semgrep", "import-linter", "pytest"} {
		h, s := newTarget(t)
		resp, err := run(t, s, replay.Clean, configRef(h, check))
		if err != nil || len(resp.GetItems()) != 0 || resp.GetSummary() == nil {
			t.Errorf("Run(%s, clean) = %v, %v, want no items and a summary", check, resp, err)
		}
	}
}

func TestServer_Run_FailureModes(t *testing.T) {
	tests := []struct {
		name  string
		cand  string
		check string
		want  pluginv1.Reason
	}{
		{"a crashing tool", replay.Crashing, "ruff", pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"output that is not the tool's", replay.Malformed, "mypy", pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"a pytest report that declares an entity", replay.Malformed, "pytest", pluginv1.Reason_REASON_UPSTREAM_ERROR},
		{"a check the provider cannot run", replay.Seeded, "clippy", pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"a candidate with no recording", "never-recorded", "ruff", pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
	}
	for _, tt := range tests {
		h, s := newTarget(t)
		resp, err := run(t, s, tt.cand, configRef(h, tt.check))
		if resp != nil || reason(err) != tt.want {
			t.Errorf("%s: Run = %v, %v, want a FAILED call with reason %v and no findings", tt.name, resp, err, tt.want)
		}
	}
}

func TestServer_Run_AConfigOutOfItsBoundsIsInvalid(t *testing.T) {
	h, s := newTarget(t)
	bad := replay.CandidateRef("bad-config")
	h.configs[bad.GetSha256().GetValue()] = []byte(`{"check":"ruff","unknown":1}`)
	if _, err := run(t, s, replay.Seeded, bad); reason(err) != pluginv1.Reason_REASON_INVALID_ARGUMENT {
		t.Errorf("Run(unknown config field) error = %v, want INVALID_ARGUMENT", err)
	}
	h.over = true
	if _, err := run(t, s, replay.Seeded, configRef(h, "ruff")); reason(err) != pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE {
		t.Errorf("Run(unreadable config) error = %v, want UNAVAILABLE", err)
	}
	h.over = false
	if _, _, err := s.Run(t.Context(), "inv", &verifierv1.RunRequest{}); reason(err) != pluginv1.Reason_REASON_INVALID_ARGUMENT {
		t.Errorf("Run(no references) error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestServer_Run_ASuppressedViolationIsStillReported(t *testing.T) {
	h, s := newTarget(t)
	resp, err := run(t, s, replay.Suppressed, configRef(h, "mypy"))
	if err != nil {
		t.Fatalf("Run(mypy, suppressed) error = %v", err)
	}
	if len(resp.GetItems()) != 1 || resp.GetItems()[0].GetRuleId() != "numinance.directive/type-ignore" {
		t.Errorf("Run(mypy, suppressed) = %v, want the type: ignore directive reported", resp.GetItems())
	}
}

// stubRunner returns one fixed output, and runs Fetch first when fetch is set.
type stubRunner struct {
	out   provider.Output
	fetch bool
}

func (r stubRunner) Run(ctx context.Context, c provider.Call) (provider.Output, error) {
	if r.fetch {
		if _, err := c.Fetch(ctx); err != nil {
			return provider.Output{}, err
		}
	}
	return r.out, nil
}

func TestServer_Run_AToolThatExitsNonZeroWithNothingToShowCrashed(t *testing.T) {
	h := &fakeHost{configs: map[string][]byte{}}
	s := provider.New(provider.ReplayPluginID, "sha256:0", stubRunner{out: provider.Output{Stdout: []byte("[]"), ExitCode: 2}}, h, nil)
	if resp, err := run(t, s, replay.Seeded, configRef(h, "ruff")); resp != nil || reason(err) != pluginv1.Reason_REASON_UPSTREAM_ERROR {
		t.Errorf("Run(exit 2, no findings) = %v, %v, want FAILED with UPSTREAM_ERROR, never a pass", resp, err)
	}
	s = provider.New(provider.ReplayPluginID, "sha256:0", stubRunner{out: provider.Output{Stdout: []byte("[]"), ExitCode: 0}}, h, nil)
	if resp, err := run(t, s, replay.Seeded, configRef(h, "ruff")); err != nil || len(resp.GetItems()) != 0 {
		t.Errorf("Run(exit 0, no findings) = %v, %v, want a clean answer", resp, err)
	}
}

func TestServer_Run_TheFetchedCandidateMustBeTheDigestOfItsReference(t *testing.T) {
	h := &fakeHost{configs: map[string][]byte{}}
	s := provider.New(provider.ReplayPluginID, "sha256:0", stubRunner{fetch: true, out: provider.Output{Stdout: []byte("[]")}}, h, nil)
	cfg := configRef(h, "ruff")
	// the fake host answers the candidate reference with the bytes it holds for that digest: register bytes that do not hash to it
	cand := replay.CandidateRef(replay.Seeded)
	h.configs[cand.GetSha256().GetValue()] = []byte("not the candidate")
	if resp, _, err := s.Run(t.Context(), "inv", &verifierv1.RunRequest{CandidateRef: cand, ConfigRef: cfg}); resp != nil || reason(err) != pluginv1.Reason_REASON_INVALID_ARGUMENT {
		t.Errorf("Run(content of another digest) = %v, %v, want INVALID_ARGUMENT", resp, err)
	}
}
