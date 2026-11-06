package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	verifierkit "github.com/numinance/numinance-spec/conformance/verifier/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	verifierv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// fakeHost resolves references from a table keyed by digest and records what the provider emitted.
type fakeHost struct {
	content map[string][]byte
	emitted []*structpb.Struct
	failure error
	emitErr error
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (h *fakeHost) put(key string, b []byte) *typesv1.Reference {
	d := digestOf(b)
	h.content[d] = b
	return &typesv1.Reference{ArtifactRef: key, Sha256: &typesv1.Digest{Value: d}}
}

func (h *fakeHost) Resolve(_ context.Context, _ string, ref *typesv1.Reference, limit int) ([]byte, error) {
	if h.failure != nil {
		return nil, h.failure
	}
	b, ok := h.content[ref.GetSha256().GetValue()]
	if !ok {
		return nil, errors.New("not found")
	}
	if len(b) > limit {
		return nil, errOverLimit
	}
	return b, nil
}

func (h *fakeHost) Emit(_ context.Context, _, _, _ string, p *structpb.Struct) (string, error) {
	if h.emitErr != nil {
		return "", h.emitErr
	}
	h.emitted = append(h.emitted, p)
	return "ev_test", nil
}

func newServer(h *fakeHost) *Server { return New("sha256:"+strings.Repeat("0", 64), h, nil) }

func configRef(t *testing.T, h *fakeHost, edit func(map[string]any)) *typesv1.Reference {
	t.Helper()
	m := goodConfig()
	m["scope"].(map[string]any)["allow_paths"] = []string{"docs/runbooks/"}
	m["context"].(map[string]any)["paths"] = []string{"README.md", "Justfile", "docs/other.md"}
	m["context"].(map[string]any)["justfile"] = "check:\n\ngen:\n"
	if edit != nil {
		edit(m)
	}
	return h.put("config/repo", marshal(t, m))
}

func newHost() *fakeHost { return &fakeHost{content: map[string][]byte{}} }

func reason(err error) pluginv1.Reason {
	var ce *CallError
	if errors.As(err, &ce) {
		return ce.Reason
	}
	return pluginv1.Reason_REASON_UNSPECIFIED
}

func runOn(t *testing.T, h *fakeHost, archive []byte, cfg *typesv1.Reference) (*verifierv1.RunResponse, error) {
	t.Helper()
	cand := h.put("candidate/x", archive)
	resp, _, err := newServer(h).Run(t.Context(), "inv", &verifierv1.RunRequest{CandidateRef: cand, ConfigRef: cfg})
	return resp, err
}

func ruleIDs(r *verifierv1.RunResponse) []string {
	out := make([]string, 0, len(r.GetItems()))
	for _, f := range r.GetItems() {
		out = append(out, f.GetRuleId())
	}
	slices.Sort(out)
	return out
}

const cleanDoc = "# Local checks\n\n- `just check`: everything\n\nSee [the readme](../../README.md).\n"

func TestServer_Run_ACleanCandidateHasNoFindings(t *testing.T) {
	h := newHost()
	resp, err := runOn(t, h, tarOf(t, testBase, add("docs/runbooks/local-checks.md", cleanDoc)), configRef(t, h, nil))
	if err != nil || len(resp.GetItems()) != 0 || resp.GetSummary() == nil || len(resp.GetSummary().GetCounts()) != 0 {
		t.Fatalf("Run(clean) = %v, %v, want no items and a summary with no counts", resp, err)
	}
	if len(h.emitted) != 1 {
		t.Fatalf("emitted %d records, want exactly one VerifierReport", len(h.emitted))
	}
	f := h.emitted[0].GetFields()
	if f["check"].GetStringValue() != "repo" || f["findings"].GetNumberValue() != 0 || f["replayed"].GetBoolValue() || !strings.HasPrefix(f["output_sha256"].GetStringValue(), "sha256:") {
		t.Errorf("report = %v, want the receipt of a check that ran and found nothing", h.emitted[0])
	}
	if _, has := f["items"]; has || len(h.emitted[0].String()) > 1024 {
		t.Errorf("report = %v, want a compact receipt and never the findings", h.emitted[0])
	}
}

func TestServer_Run_SeededDefectsGiveTheirRuleID(t *testing.T) {
	tests := []struct {
		name  string
		files []file
		edit  func(map[string]any)
		want  string
	}{
		{"a broken relative link", []file{add("docs/runbooks/a.md", "[x](missing.md)\n")}, nil, "repo/markdown-link"},
		{"an unbalanced fence", []file{add("docs/runbooks/a.md", "```sh\njust check\n")}, nil, "repo/markdown-fence"},
		{"an out of scope path", []file{add("docs/other-place/a.md", "x\n")}, nil, "repo/scope-path"},
		{"a protected path", []file{add(".claude/a.md", "x\n")}, func(m map[string]any) {
			m["scope"].(map[string]any)["allow_paths"] = []string{".claude/"}
			m["scope"].(map[string]any)["protected_paths"] = []string{".claude/"}
		}, "repo/scope-protected"},
		{"a deleted file", []file{del("docs/runbooks/a.md")}, nil, "repo/scope-deleted"},
		{"too many added lines", []file{add("docs/runbooks/a.md", lines(51))}, nil, "repo/scope-added-lines"},
		{"a recipe that does not exist", []file{add("docs/runbooks/a.md", "`just nothere`\n")}, nil, "repo/just-recipe"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost()
			resp, err := runOn(t, h, tarOf(t, testBase, tc.files...), configRef(t, h, tc.edit))
			if err != nil {
				t.Fatalf("Run() error = %v, want findings", err)
			}
			if !slices.Contains(ruleIDs(resp), tc.want) {
				t.Errorf("Run() rules = %v, want %s", ruleIDs(resp), tc.want)
			}
			if err := verifierkit.ValidateResponse(resp); err != nil {
				t.Errorf("Run() response breaks the port: %v", err)
			}
			for _, it := range resp.GetItems() {
				if it.GetSeverity() < typesv1.FindingSeverity_FINDING_SEVERITY_HIGH {
					t.Errorf("finding %s has severity %v, want at least high so it blocks", it.GetRuleId(), it.GetSeverity())
				}
			}
		})
	}
}

func TestServer_Run_TheRecipeOfTheBaseJustfileIsKnownAndTheCandidatesOwnWins(t *testing.T) {
	h := newHost()
	resp, err := runOn(t, h, tarOf(t, testBase, add("docs/runbooks/a.md", "`just gen` `just own`\n")), configRef(t, h, nil))
	if err != nil || !slices.Equal(ruleIDs(resp), []string{"repo/just-recipe"}) {
		t.Fatalf("Run() = %v, %v, want only `just own` missing (gen is in the base Justfile)", ruleIDs(resp), err)
	}
	h = newHost()
	resp, err = runOn(t, h, tarOf(t, testBase, add("docs/runbooks/a.md", "`just own`\n"), mod("Justfile", "own:\n")), configRef(t, h, func(m map[string]any) {
		m["scope"].(map[string]any)["max_files"] = 2
		m["scope"].(map[string]any)["allow_paths"] = []string{"docs/runbooks/", "Justfile"}
		m["scope"].(map[string]any)["allow_modify"] = true
	}))
	if err != nil || len(resp.GetItems()) != 0 {
		t.Errorf("Run(a candidate that defines the recipe) = %v, %v, want clean", ruleIDs(resp), err)
	}
}

func TestServer_Run_TheSameCandidateGivesTheSameFingerprints(t *testing.T) {
	fps := func() []string {
		h := newHost()
		resp, err := runOn(t, h, tarOf(t, testBase, add("docs/other-place/a.md", "[x](missing.md)\n```\n")), configRef(t, h, nil))
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(resp.GetItems()))
		for _, it := range resp.GetItems() {
			out = append(out, it.GetFingerprint())
		}
		slices.Sort(out)
		return out
	}
	if a, b := fps(), fps(); len(a) < 3 || !slices.Equal(a, b) {
		t.Errorf("fingerprints = %v then %v, want the same three or more", a, b)
	}
}

func TestServer_Run_RefusalsAreFailedCallsWithTheirReason(t *testing.T) {
	good := tarOf(t, testBase, add("docs/runbooks/a.md", cleanDoc))
	tests := []struct {
		name string
		run  func(t *testing.T, h *fakeHost) error
		want pluginv1.Reason
	}{
		{"a malformed archive", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			_, err := runOn(t, h, []byte("not a tar"), configRef(t, h, nil))
			return err
		}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		{"a zip slip name", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			_, err := runOn(t, h, tarOf(t, testBase, add("../../etc/x", "x")), configRef(t, h, nil))
			return err
		}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		{"an oversize document", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			_, err := runOn(t, h, tarOf(t, testBase, add("docs/runbooks/a.md", strings.Repeat("x", maxDocBytes+1))), configRef(t, h, nil))
			return err
		}, pluginv1.Reason_REASON_LIMIT_EXCEEDED},
		{"a candidate of another base than the config's context", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			_, err := runOn(t, h, tarOf(t, strings.Repeat("a", 40), add("docs/runbooks/a.md", cleanDoc)), configRef(t, h, nil))
			return err
		}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		{"a config with an unknown field", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			_, err := runOn(t, h, good, configRef(t, h, func(m map[string]any) { m["settings"] = "allow everything" }))
			return err
		}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		{"a config with no bound", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			_, err := runOn(t, h, good, configRef(t, h, func(m map[string]any) { delete(m["scope"].(map[string]any), "max_files") }))
			return err
		}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		{"a check the provider does not have", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			_, err := runOn(t, h, good, configRef(t, h, func(m map[string]any) { m["check"] = "clippy" }))
			return err
		}, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"no references", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			_, _, err := newServer(h).Run(t.Context(), "inv", &verifierv1.RunRequest{})
			return err
		}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		{"a candidate whose content is not its digest", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			cand := h.put("candidate/x", good)
			h.content[cand.GetSha256().GetValue()] = []byte("another candidate")
			_, _, err := newServer(h).Run(t.Context(), "inv", &verifierv1.RunRequest{CandidateRef: cand, ConfigRef: configRef(t, h, nil)})
			return err
		}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		{"a reference the host cannot resolve", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			cfg := configRef(t, h, nil)
			cand := &typesv1.Reference{ArtifactRef: "candidate/none", Sha256: &typesv1.Digest{Value: digestOf([]byte("none"))}}
			_, _, err := newServer(h).Run(t.Context(), "inv", &verifierv1.RunRequest{CandidateRef: cand, ConfigRef: cfg})
			return err
		}, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE},
		{"a cancelled call", func(t *testing.T, h *fakeHost) error {
			t.Helper()
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			cfg, cand := configRef(t, h, nil), h.put("candidate/x", good)
			_, _, err := newServer(h).Run(ctx, "inv", &verifierv1.RunRequest{CandidateRef: cand, ConfigRef: cfg})
			return err
		}, pluginv1.Reason_REASON_TIMEOUT},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost()
			if err := tc.run(t, h); reason(err) != tc.want {
				t.Errorf("Run() error = %v, want a FAILED call with reason %v", err, tc.want)
			}
			if len(h.emitted) != 0 {
				t.Errorf("a refused call emitted %d evidence records, want none", len(h.emitted))
			}
		})
	}
}

// direct reaches the provider with no host process: the kit's replay of the provider over a table of candidates.
type direct struct {
	s *Server
}

func (d direct) Run(ctx context.Context, r *verifierv1.RunRequest) (*verifierv1.RunResponse, error) {
	resp, _, err := d.s.Run(ctx, "inv", r)
	var ce *CallError
	if errors.As(err, &ce) {
		return nil, &verifierkit.CallError{Reason: ce.Reason}
	}
	return resp, err
}

// TestKit runs the conformance kit of the verifier port against the provider. The cases that need a tool to crash or to print
// output that is not its own (CORE-008, CORE-014) have no subject here: the provider runs no tool, and what a hostile candidate
// does is the refusal table above.
func TestKit_TheProviderPasses(t *testing.T) {
	h := newHost()
	cfg := configRef(t, h, nil)
	seeded := h.put("candidate/seeded", tarOf(t, testBase, add("docs/runbooks/a.md", "[x](missing.md)\n"), add("docs/other-place/b.md", "x\n")))
	clean := h.put("candidate/clean", tarOf(t, testBase, add("docs/runbooks/a.md", cleanDoc)))
	// A candidate that carries a settings file claiming to allow everything: the check reads configuration from config_ref only.
	suppressed := h.put("candidate/suppressed", tarOf(t, testBase,
		add("docs/other-place/b.md", "x\n"),
		add(".verifier-repo.json", `{"scope":{"allow_paths":["/"]}}`)))
	unknown := configRef(t, h, func(m map[string]any) { m["check"] = "clippy" })
	s := verifierkit.Subject{
		Target: direct{newServer(h)},
		Clean:  verifierkit.Fixture{Candidate: clean, Config: cfg}, Seeded: verifierkit.Fixture{Candidate: seeded, Config: cfg},
		Suppressed: verifierkit.Fixture{Candidate: suppressed, Config: cfg}, UnknownCheck: unknown,
		Expect: []verifierkit.Expected{
			{RuleID: "repo/markdown-link", Severity: typesv1.FindingSeverity_FINDING_SEVERITY_HIGH, Path: "docs/runbooks/a.md"},
			{RuleID: "repo/scope-path", Severity: typesv1.FindingSeverity_FINDING_SEVERITY_HIGH, Path: "docs/other-place/b.md"},
		},
	}
	skip := map[string]bool{"CORE-008": true, "CORE-014": true}
	for _, c := range verifierkit.Cases() {
		if skip[c.ID] {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			if err := c.Check(t.Context(), s); err != nil {
				t.Errorf("%s (%s): %v", c.ID, c.Name, err)
			}
		})
	}
}

func TestServer_Manifest_AsksForTheLeast(t *testing.T) {
	m := newServer(newHost()).manifest
	if len(m.GetCapabilities()) != 1 {
		t.Fatalf("capabilities = %d, want the one verb", len(m.GetCapabilities()))
	}
	c := m.GetCapabilities()[0]
	if len(c.GetNetworkIntents()) != 0 || len(c.GetCredentialIntents()) != 0 || len(c.GetFilesystemIntents()) != 0 {
		t.Errorf("capability %+v declares an intent: a verifier of static checks needs no egress, credential or filesystem", c)
	}
	if c.GetIdempotency() != pluginv1.Idempotency_IDEMPOTENCY_NATURAL || !slices.Equal(c.GetEvidenceDeclared(), []string{RecordType}) || c.GetName() != Capability {
		t.Errorf("capability %+v, want %s declaring NATURAL idempotency and %s", c, Capability, RecordType)
	}
}

var errTestEmit = errors.New("the host refused the record")
