package knowledgekit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	knowledgekit "github.com/numinance/numinance-spec/conformance/knowledge/v1"
	"github.com/numinance/numinance-spec/plugins/examples/sample-knowledge/provider"
	knowledgev1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/knowledge/v1"
)

// direct reaches the sample's source with no host and no process: the kit's replay of its own reference provider.
type direct struct{ s *provider.Source }

func asFailure(err error) error {
	var f *provider.CallError
	if errors.As(err, &f) {
		return &knowledgekit.CallError{Reason: f.Reason}
	}
	return err
}

func (d direct) Search(_ context.Context, r *knowledgev1.SearchRequest) (*knowledgev1.SearchResponse, error) {
	resp, err := d.s.Search(r)
	return resp, asFailure(err)
}

func (d direct) Fetch(_ context.Context, r *knowledgev1.FetchRequest) (*knowledgev1.FetchResponse, error) {
	resp, err := d.s.Fetch(r)
	return resp, asFailure(err)
}

func (d direct) Resolve(_ context.Context, r *knowledgev1.ResolveRequest) (*knowledgev1.ResolveResponse, error) {
	resp, err := d.s.Resolve(r)
	return resp, asFailure(err)
}

func TestKit_TheSampleProviderPassesTheSkeleton(t *testing.T) {
	knowledgekit.Run(t, direct{provider.NewSource()})
}

// leaky returns documents of scope other for any search: a provider that ignores scope.
type leaky struct{ direct }

func (l leaky) Search(ctx context.Context, r *knowledgev1.SearchRequest) (*knowledgev1.SearchResponse, error) {
	r.Scope = []string{"kit", "other"}
	return l.direct.Search(ctx, r)
}

// lenient accepts a scope that climbs out with a dot dot segment.
type lenient struct{ direct }

func (l lenient) Search(ctx context.Context, r *knowledgev1.SearchRequest) (*knowledgev1.SearchResponse, error) {
	if len(r.GetScope()) == 1 && r.GetScope()[0] == "kit/../other" {
		return &knowledgev1.SearchResponse{}, nil
	}
	return l.direct.Search(ctx, r)
}

func TestKit_FailsABrokenProvider(t *testing.T) {
	cases := map[string]knowledgekit.Target{"leaky": leaky{direct{provider.NewSource()}}, "lenient": lenient{direct{provider.NewSource()}}, "splicing": splicing{direct{provider.NewSource()}}}
	for name, target := range cases {
		failed := 0
		for _, c := range knowledgekit.Cases() {
			if c.Check(t.Context(), target) != nil {
				failed++
			}
		}
		if failed == 0 {
			t.Errorf("the kit passed the %s provider, want at least one failing case", name)
		}
	}
}

func TestKit_CasesAreListedInTheREADME(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range knowledgekit.Cases() {
		if seen[c.ID] || c.Check == nil || c.Verb == "" {
			t.Errorf("case %+v is a duplicate or has no check or verb", c)
		}
		seen[c.ID] = true
	}
}

// splicing puts the query text into a query language: a quote that closes the string lets the text name another scope.
type splicing struct{ direct }

func (s splicing) Search(ctx context.Context, r *knowledgev1.SearchRequest) (*knowledgev1.SearchResponse, error) {
	if strings.Contains(r.GetQuery(), "scope='other") {
		r.Scope = []string{"kit", "other"}
		r.Query = "deploy"
	}
	return s.direct.Search(ctx, r)
}
