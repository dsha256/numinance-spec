package verifierkit_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	verifierkit "github.com/numinance/numinance-spec/conformance/verifier/v1"
	"github.com/numinance/numinance-spec/plugins/official/verifier-python/provider"
	"github.com/numinance/numinance-spec/plugins/official/verifier-python/replay"
	verifierv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// host resolves the kit's config references from a table, as a host would resolve them from the artifact store.
type host struct{ configs map[string][]byte }

func (h host) Resolve(_ context.Context, _ string, ref *typesv1.Reference, _ int) ([]byte, error) {
	b, ok := h.configs[ref.GetSha256().GetValue()]
	if !ok {
		return nil, errors.New("not found")
	}
	return b, nil
}

func (host) Emit(context.Context, string, string, string, *structpb.Struct) (string, error) {
	return "ev_kit", nil
}

// direct reaches the provider with no host process: the kit's replay of the replaying build of the reference provider.
type direct struct{ s *provider.Server }

func (d direct) Run(ctx context.Context, r *verifierv1.RunRequest) (*verifierv1.RunResponse, error) {
	resp, _, err := d.s.Run(ctx, "inv", r)
	var ce *provider.CallError
	if errors.As(err, &ce) {
		return nil, &verifierkit.CallError{Reason: ce.Reason}
	}
	return resp, err
}

func subject(check string) verifierkit.Subject {
	h := host{configs: map[string][]byte{}}
	cfg := func(c string) *typesv1.Reference {
		b, _ := json.Marshal(provider.Config{Check: c})
		ref := replay.CandidateRef("config-" + c)
		h.configs[ref.GetSha256().GetValue()] = b
		return ref
	}
	d := direct{provider.New(provider.ReplayPluginID, "sha256:0", replay.Runner{}, h, nil)}
	fx := func(cand string) verifierkit.Fixture {
		return verifierkit.Fixture{Candidate: replay.CandidateRef(cand), Config: cfg(check)}
	}
	return verifierkit.Subject{
		Target: d, Clean: fx(replay.Clean), Seeded: fx(replay.Seeded), Suppressed: fx(replay.Suppressed),
		Crashing: fx(replay.Crashing), Malformed: fx(replay.Malformed), UnknownCheck: cfg("clippy"),
	}
}

func TestKit_TheReplayingProviderPasses(t *testing.T) {
	s := subject("mypy")
	s.Expect = []verifierkit.Expected{{RuleID: "mypy/return-value", Severity: typesv1.FindingSeverity_FINDING_SEVERITY_MEDIUM, Path: "shop/domain/pricing.py"}}
	verifierkit.Run(t, s)
}

// lying reports nothing for any candidate: a provider that passes everything.
type lying struct{}

func (lying) Run(context.Context, *verifierv1.RunRequest) (*verifierv1.RunResponse, error) {
	return &verifierv1.RunResponse{Summary: &verifierv1.FindingsSummary{}}, nil
}

func TestKit_FailsAProviderThatPassesEverything(t *testing.T) {
	s := subject("mypy")
	s.Target = lying{}
	failed := 0
	for _, c := range verifierkit.Cases() {
		if c.Check(t.Context(), s) != nil {
			failed++
		}
	}
	if failed < 4 {
		t.Errorf("the kit failed %d cases of a provider that reports nothing and fails nothing, want at least 4", failed)
	}
}

func TestValidateResponse_RefusesTheRulesOfTheHost(t *testing.T) {
	item := func(rule string, sev typesv1.FindingSeverity, fp string) *verifierv1.Finding {
		return &verifierv1.Finding{RuleId: rule, Severity: sev, Message: "m", Fingerprint: fp}
	}
	high, low := typesv1.FindingSeverity_FINDING_SEVERITY_HIGH, typesv1.FindingSeverity_FINDING_SEVERITY_LOW
	sum := func(c ...*verifierv1.SeverityCount) *verifierv1.FindingsSummary {
		return &verifierv1.FindingsSummary{Counts: c}
	}
	count := func(s typesv1.FindingSeverity, n uint64) *verifierv1.SeverityCount {
		return &verifierv1.SeverityCount{Severity: s, Count: n}
	}
	tests := map[string]*verifierv1.RunResponse{
		"no summary":                          {},
		"a repeated fingerprint":              {Items: []*verifierv1.Finding{item("a", high, "f"), item("b", high, "f")}, Summary: sum(count(high, 2))},
		"not most severe first":               {Items: []*verifierv1.Finding{item("a", low, "f1"), item("b", high, "f2")}, Summary: sum(count(high, 1), count(low, 1))},
		"an unknown severity":                 {Items: []*verifierv1.Finding{item("a", 9, "f")}, Summary: sum(count(9, 1))},
		"a count below its items":             {Items: []*verifierv1.Finding{item("a", high, "f1"), item("b", high, "f2")}, Summary: sum(count(high, 1))},
		"fewer items than the total":          {Items: []*verifierv1.Finding{item("a", high, "f")}, Summary: sum(count(high, 1), count(low, 5))},
		"a rule id out of bounds":             {Items: []*verifierv1.Finding{item("-a", high, "f")}, Summary: sum(count(high, 1))},
		"a path that climbs out":              {Items: []*verifierv1.Finding{{RuleId: "a", Severity: high, Message: "m", Fingerprint: "f", Location: &verifierv1.Location{Path: "a/../b", StartLine: 1, EndLine: 1}}}, Summary: sum(count(high, 1))},
		"an end line before the start":        {Items: []*verifierv1.Finding{{RuleId: "a", Severity: high, Message: "m", Fingerprint: "f", Location: &verifierv1.Location{Path: "a", StartLine: 3, EndLine: 2}}}, Summary: sum(count(high, 1))},
		"a count of zero":                     {Summary: sum(count(high, 0))},
		"two counts of one severity":          {Items: []*verifierv1.Finding{item("a", high, "f")}, Summary: sum(count(high, 1), count(high, 1))},
		"an item of a severity with no count": {Items: []*verifierv1.Finding{item("a", high, "f")}, Summary: sum(count(low, 1))},
	}
	for name, resp := range tests {
		if err := verifierkit.ValidateResponse(resp); err == nil {
			t.Errorf("ValidateResponse(%s) = nil, want an error", name)
		}
	}
	good := &verifierv1.RunResponse{Items: []*verifierv1.Finding{item("a", high, "f1"), item("b", low, "f2")}, Summary: sum(count(high, 1), count(low, 1))}
	if err := verifierkit.ValidateResponse(good); err != nil {
		t.Errorf("ValidateResponse(good) = %v, want nil", err)
	}
}
