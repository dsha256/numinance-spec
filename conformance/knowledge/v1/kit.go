// Package knowledgekit is the runnable skeleton of the knowledge conformance kit, version 1.0.0 (README.md in this
// directory lists the full case table; this package runs the subset a provider with the kit's seeded source can pass
// without further fixtures). It is the acceptance harness for a provider of numinance.ports.knowledge.v1: the harness
// reaches the provider however it likes (a host dispatching Invoke, a replay, a direct call) through Target, and the cases
// judge only what comes back. It imports the public spec only.
//
// The seeded source is the one the README describes: scope "kit" with documents "doc-a" (two versions, "v1" and
// "v2") and "doc-b", and scope "other" with "doc-o", which matches the query of CORE-001 and must never be returned for a
// request that names scope "kit". Every request names scope ["kit"] unless a case says otherwise.
package knowledgekit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	knowledgev1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/knowledge/v1"
)

// CallError is the error a Target returns for a call the provider answered with FAILED; Reason is the reason of the
// diagnostic it carried (INVALID_ARGUMENT, NOT_FOUND, ...). Any other error means the harness could not get an answer.
type CallError struct{ Reason pluginv1.Reason }

// Error implements error.
func (f *CallError) Error() string { return "provider failed the call: " + f.Reason.String() }

// Target is the provider under test.
type Target interface {
	Search(context.Context, *knowledgev1.SearchRequest) (*knowledgev1.SearchResponse, error)
	Fetch(context.Context, *knowledgev1.FetchRequest) (*knowledgev1.FetchResponse, error)
	Resolve(context.Context, *knowledgev1.ResolveRequest) (*knowledgev1.ResolveResponse, error)
}

// Case is one conformance case: an id of the README table, the verb and the check.
type Case struct {
	ID, Verb, Name string
	Check          func(ctx context.Context, t Target) error
}

// kit is the scope every request names unless a case says otherwise; a function, so no case can edit a shared slice.
func kit() []string { return []string{"kit"} }

// Cases returns the cases this skeleton runs, in README order. The README lists the rest; a case moves here when its
// fixture (documents, principals, a broken source) is added to the kit.
func Cases() []Case {
	return []Case{
		{"CORE-001", "Search", "a query matching doc-a and doc-o returns doc-a and nothing of scope other", core001},
		{"CORE-002", "Search", "a query matching nothing is OK with no results", core002},
		{"CORE-003", "Search", "max_results 1 returns exactly one result", core003},
		{"CORE-004", "Search", "every bound violation is INVALID_ARGUMENT", core004},
		{"CORE-005", "Search", "a query with the source's query-language operators is text: nothing outside scope kit", core005},
		{"CORE-007", "Fetch", "doc-a latest: digest matches content, version is the latest, total_bytes is the length", core007},
		{"CORE-008", "Fetch", "doc-a at its first version returns that version's content", core008},
		{"CORE-012", "Fetch", "a ref never written and a ref of another scope are NOT_FOUND", core012},
		{"CORE-013", "Fetch", "every bound violation is INVALID_ARGUMENT", core013},
		{"CORE-014", "Resolve", "two refs to doc-a resolve to one canonical ref, unchanged when resolved again", core014},
		{"CORE-015", "Resolve", "an unknown ref is NOT_FOUND and a missing ref or scope is INVALID_ARGUMENT", core015},
	}
}

// Run runs every case against target as subtests of t.
func Run(t *testing.T, target Target) {
	t.Helper()
	for _, c := range Cases() {
		t.Run(c.ID, func(t *testing.T) {
			if err := c.Check(t.Context(), target); err != nil {
				t.Errorf("%s %s (%s): %v", c.ID, c.Verb, c.Name, err)
			}
		})
	}
}

func wantReason(err error, reason pluginv1.Reason) error {
	var f *CallError
	if !errors.As(err, &f) || f.Reason != reason {
		return fmt.Errorf("error = %w, want a FAILED call with reason %s", err, reason)
	}
	return nil
}

func core001(ctx context.Context, t Target) error {
	resp, err := t.Search(ctx, &knowledgev1.SearchRequest{Query: "deploy", Scope: kit(), MaxResults: 10})
	if err != nil {
		return err
	}
	found := false
	for _, r := range resp.GetResults() {
		found = found || r.GetRef() == "kit/doc-a"
		if strings.HasPrefix(r.GetRef(), "other/") {
			return fmt.Errorf("a result of scope other was returned for scope kit")
		}
		if len(r.GetTitle()) > 1024 || len(r.GetSnippet()) > 4096 || r.GetRef() == "" {
			return fmt.Errorf("a result is out of its bounds")
		}
	}
	if !found {
		return fmt.Errorf("doc-a is not in the results")
	}
	return nil
}

func core002(ctx context.Context, t Target) error {
	resp, err := t.Search(ctx, &knowledgev1.SearchRequest{Query: "zzz-no-such-text", Scope: kit(), MaxResults: 10})
	if err != nil {
		return err
	}
	if n := len(resp.GetResults()); n != 0 {
		return fmt.Errorf("%d results, want none", n)
	}
	return nil
}

func core003(ctx context.Context, t Target) error {
	resp, err := t.Search(ctx, &knowledgev1.SearchRequest{Query: "e", Scope: kit(), MaxResults: 1})
	if err != nil {
		return err
	}
	if n := len(resp.GetResults()); n != 1 {
		return fmt.Errorf("%d results, want exactly 1", n)
	}
	return nil
}

func core004(ctx context.Context, t Target) error {
	scopes := map[string][]string{
		"empty scope": nil, "duplicate entry": {"kit", "kit"}, "257 entries": manyScopes(257), "dot dot": {"kit/../other"},
		"leading slash": {"/kit"}, "backslash": {`kit\x`}, "encoded dot dot": {"kit/%2E%2E/other"},
	}
	for name, scope := range scopes {
		if err := search(ctx, t, "deploy", scope, 10); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	for name, c := range map[string]struct {
		q string
		n uint32
	}{"empty query": {"", 10}, "query of 1025 bytes": {strings.Repeat("a", 1025), 10}, "max_results 0": {"deploy", 0}, "max_results 101": {"deploy", 101}} {
		if err := search(ctx, t, c.q, kit(), c.n); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

// core005 sends the operators of the common query languages (CQL, JQL, SQL-like and glob forms) as the query text. A provider
// that splices the text into a query would let it widen the search past scope kit; one that passes it as a parameter finds
// nothing, since no seeded document holds the text. It never returns a document of scope other.
func core005(ctx context.Context, t Target) error {
	for _, q := range []string{
		`deploy OR space = "other"`, `text ~ "deploy" ORDER BY created`, `deploy' OR scope='other`, `space in ("kit","other") AND type = page`, `*`, `deploy*`, `kit/../other deploy`,
	} {
		resp, err := t.Search(ctx, &knowledgev1.SearchRequest{Query: q, Scope: kit(), MaxResults: 10})
		if err != nil {
			return fmt.Errorf("query %q: %w", q, err)
		}
		for _, r := range resp.GetResults() {
			if strings.HasPrefix(r.GetRef(), "other/") {
				return fmt.Errorf("query %q returned a result of scope other: the operators were not treated as text", q)
			}
		}
	}
	return nil
}

func search(ctx context.Context, t Target, q string, scope []string, n uint32) error {
	_, err := t.Search(ctx, &knowledgev1.SearchRequest{Query: q, Scope: scope, MaxResults: n})
	return wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT)
}

func manyScopes(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("s%d", i)
	}
	return out
}

func fetch(ctx context.Context, t Target, ref, version string) (*knowledgev1.FetchResponse, error) {
	return t.Fetch(ctx, &knowledgev1.FetchRequest{Ref: ref, Version: version, MaxBytes: 1 << 20, Scope: kit()})
}

func core007(ctx context.Context, t Target) error {
	resp, err := fetch(ctx, t, "kit/doc-a", "")
	if err != nil {
		return err
	}
	sum := sha256.Sum256(resp.GetContent())
	switch {
	case resp.GetDigest().GetValue() != "sha256:"+hex.EncodeToString(sum[:]):
		return fmt.Errorf("the digest does not match the content")
	case resp.GetVersion() != "v2":
		return fmt.Errorf("version %q, want the latest, v2", resp.GetVersion())
	case resp.GetTotalBytes() != uint64(len(resp.GetContent())):
		return fmt.Errorf("total_bytes %d, want the content length %d", resp.GetTotalBytes(), len(resp.GetContent()))
	case resp.GetMediaType() == "":
		return fmt.Errorf("no media type")
	}
	return nil
}

func core008(ctx context.Context, t Target) error {
	first, err := fetch(ctx, t, "kit/doc-a", "v1")
	if err != nil {
		return err
	}
	latest, err := fetch(ctx, t, "kit/doc-a", "")
	if err != nil {
		return err
	}
	if first.GetVersion() != "v1" || string(first.GetContent()) == string(latest.GetContent()) {
		return fmt.Errorf("the first version is not served as such")
	}
	return nil
}

func core012(ctx context.Context, t Target) error {
	for _, ref := range []string{"kit/never-written", "other/doc-o"} {
		_, err := fetch(ctx, t, ref, "")
		if err := wantReason(err, pluginv1.Reason_REASON_NOT_FOUND); err != nil {
			return fmt.Errorf("ref %q: %w", ref, err)
		}
	}
	return nil
}

func core013(ctx context.Context, t Target) error {
	bad := map[string]*knowledgev1.FetchRequest{
		"empty ref":        {Ref: "", MaxBytes: 1, Scope: kit()},
		"ref of 1025":      {Ref: strings.Repeat("a", 1025), MaxBytes: 1, Scope: kit()},
		"version of 257":   {Ref: "kit/doc-a", Version: strings.Repeat("v", 257), MaxBytes: 1, Scope: kit()},
		"max_bytes 0":      {Ref: "kit/doc-a", MaxBytes: 0, Scope: kit()},
		"max_bytes 1 MiB+": {Ref: "kit/doc-a", MaxBytes: 1<<20 + 1, Scope: kit()},
		"empty scope":      {Ref: "kit/doc-a", MaxBytes: 1},
		"dot dot scope":    {Ref: "kit/doc-a", MaxBytes: 1, Scope: []string{"kit/../other"}},
	}
	for name, r := range bad {
		_, err := t.Fetch(ctx, r)
		if err := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func core014(ctx context.Context, t Target) error {
	a, err := t.Resolve(ctx, &knowledgev1.ResolveRequest{Ref: "kit/doc-a", Scope: kit()})
	if err != nil {
		return err
	}
	b, err := t.Resolve(ctx, &knowledgev1.ResolveRequest{Ref: "kit/doc-a/", Scope: kit()})
	if err != nil {
		return err
	}
	again, err := t.Resolve(ctx, &knowledgev1.ResolveRequest{Ref: a.GetCanonicalRef(), Scope: kit()})
	if err != nil {
		return err
	}
	if a.GetCanonicalRef() == "" || a.GetCanonicalRef() != b.GetCanonicalRef() || again.GetCanonicalRef() != a.GetCanonicalRef() {
		return fmt.Errorf("canonical refs %q, %q, %q, want one value, unchanged by resolving it again", a.GetCanonicalRef(), b.GetCanonicalRef(), again.GetCanonicalRef())
	}
	return nil
}

func core015(ctx context.Context, t Target) error {
	for name, c := range map[string]struct {
		req    *knowledgev1.ResolveRequest
		reason pluginv1.Reason
	}{
		"never written": {&knowledgev1.ResolveRequest{Ref: "kit/never-written", Scope: kit()}, pluginv1.Reason_REASON_NOT_FOUND},
		"another scope": {&knowledgev1.ResolveRequest{Ref: "other/doc-o", Scope: kit()}, pluginv1.Reason_REASON_NOT_FOUND},
		"empty ref":     {&knowledgev1.ResolveRequest{Scope: kit()}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		"empty scope":   {&knowledgev1.ResolveRequest{Ref: "kit/doc-a"}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
		"dot dot scope": {&knowledgev1.ResolveRequest{Ref: "kit/doc-a", Scope: []string{"kit/../other"}}, pluginv1.Reason_REASON_INVALID_ARGUMENT},
	} {
		_, err := t.Resolve(ctx, c.req)
		if err := wantReason(err, c.reason); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}
