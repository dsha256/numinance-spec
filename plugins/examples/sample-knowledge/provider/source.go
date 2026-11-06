// Package provider is the sample knowledge provider: a read-only provider of numinance.ports.knowledge.v1 over a small
// in-memory source, written against the public SDK only. It exists to be launched, validated and invoked by a plugin
// host, and to be the target of the knowledge conformance kit; it is not a product connector.
package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	knowledgev1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/knowledge/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// CallError is a call the provider answers with FAILED and one diagnostic reason, as the port's status codes map onto
// plugin/v1 reasons: INVALID_ARGUMENT and NOT_FOUND.
type CallError struct{ Reason pluginv1.Reason }

// Error implements error.
func (f *CallError) Error() string { return f.Reason.String() }

// document is one seeded document.
type document struct {
	scope, ref, title, mediaType string
	versions                     []string // oldest first; the last is the latest
	content                      map[string]string
	modified                     time.Time
}

// Source is the seeded source of the knowledge conformance kit: scope "kit" with documents "doc-a" (two versions) and
// "doc-b", and scope "other" with "doc-o", which matches the same query as doc-a and must never be returned for scope
// "kit".
type Source struct{ docs []document }

// NewSource returns the seeded source.
func NewSource() *Source {
	t := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return &Source{docs: []document{
		{scope: "kit", ref: "kit/doc-a", title: "Alpha guide", mediaType: "text/plain", versions: []string{"v1", "v2"}, modified: t,
			content: map[string]string{"v1": "alpha first draft: deploy notes", "v2": "alpha second draft: deploy notes and rollback"}},
		{scope: "kit", ref: "kit/doc-b", title: "Beta reference", mediaType: "text/plain", versions: []string{"v1"}, modified: t,
			content: map[string]string{"v1": "beta reference: monitoring"}},
		{scope: "other", ref: "other/doc-o", title: "Omega deploy notes", mediaType: "text/plain", versions: []string{"v1"}, modified: t,
			content: map[string]string{"v1": "omega deploy notes: not for scope kit"}},
	}}
}

const (
	maxScopes   = 256
	maxScopeLen = 256
	maxQuery    = 1024
	maxVersion  = 256
	maxFetch    = 1 << 20
)

// scopeSegments checks the scope grammar: 1 to 256 unique entries, each a "/"-joined list of non-empty segments of
// [A-Za-z0-9_-]. No segment can be "." or "..", and no entry is decoded or normalised, so an encoded dot segment
// (kit/%2E%2E/other) breaks the grammar too.
func scopeOK(scope []string) bool {
	if len(scope) < 1 || len(scope) > maxScopes {
		return false
	}
	seen := map[string]bool{}
	for _, e := range scope {
		if e == "" || len(e) > maxScopeLen || seen[e] {
			return false
		}
		seen[e] = true
		for seg := range strings.SplitSeq(e, "/") {
			if seg == "" {
				return false
			}
			for i := range len(seg) {
				c := seg[i]
				if !segmentByte(c) {
					return false
				}
			}
		}
	}
	return true
}

// segmentByte reports whether c may appear in a scope segment: [A-Za-z0-9_-].
func segmentByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// inScope reports whether a document lies in one of the scope entries, comparing whole segments.
func inScope(d document, scope []string) bool {
	for _, e := range scope {
		if d.ref == e || strings.HasPrefix(d.ref, e+"/") || d.scope == e {
			return true
		}
	}
	return false
}

func invalid() error  { return &CallError{Reason: pluginv1.Reason_REASON_INVALID_ARGUMENT} }
func notFound() error { return &CallError{Reason: pluginv1.Reason_REASON_NOT_FOUND} }

// printableRune reports whether r is a letter, mark, number, punctuation, symbol or the space: the control, format
// (zero-width and bidi), line and paragraph separator characters are not.
func printableRune(r rune) bool {
	return r == ' ' || unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S)
}

// clean keeps U+0009 and U+000A when allowed, replaces every other character that is not printable with U+FFFD, and
// shortens at a character boundary.
func clean(s string, limit int, keepNewline bool) string {
	var b strings.Builder
	for _, r := range s {
		if (!keepNewline || (r != '\t' && r != '\n')) && !printableRune(r) {
			r = utf8.RuneError
		}
		if b.Len()+utf8.RuneLen(r) > limit {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Search returns the documents of the requested scope whose title or latest content contains the query as plain text.
func (s *Source) Search(req *knowledgev1.SearchRequest) (*knowledgev1.SearchResponse, error) {
	q := req.GetQuery()
	if q == "" || len(q) > maxQuery || !scopeOK(req.GetScope()) || req.GetMaxResults() < 1 || req.GetMaxResults() > 100 {
		return nil, invalid()
	}
	out := &knowledgev1.SearchResponse{}
	for _, d := range s.docs {
		if !inScope(d, req.GetScope()) {
			continue
		}
		latest := d.versions[len(d.versions)-1]
		if !strings.Contains(strings.ToLower(d.title+" "+d.content[latest]), strings.ToLower(q)) {
			continue
		}
		if len(out.Results) >= int(req.GetMaxResults()) {
			break
		}
		out.Results = append(out.Results, &knowledgev1.SearchResult{
			Ref: d.ref, Title: clean(d.title, 1024, false), Snippet: clean(d.content[latest], 4096, true), SourceVersion: latest,
		})
	}
	return out, nil
}

func (s *Source) find(ref string, scope []string) (document, bool) {
	for _, d := range s.docs {
		if d.ref == ref && inScope(d, scope) {
			return d, true
		}
	}
	return document{}, false
}

// Fetch returns the content of a document, cut at max_bytes on a character boundary.
func (s *Source) Fetch(req *knowledgev1.FetchRequest) (*knowledgev1.FetchResponse, error) {
	if req.GetRef() == "" || len(req.GetRef()) > 1024 || len(req.GetVersion()) > maxVersion || req.GetMaxBytes() < 1 || req.GetMaxBytes() > maxFetch || !scopeOK(req.GetScope()) {
		return nil, invalid()
	}
	d, ok := s.find(req.GetRef(), req.GetScope())
	if !ok {
		return nil, notFound()
	}
	version := req.GetVersion()
	if version == "" {
		version = d.versions[len(d.versions)-1]
	}
	content, ok := d.content[version]
	if !ok {
		return nil, notFound()
	}
	total := len(content)
	if len(content) > int(req.GetMaxBytes()) {
		content = content[:cutAt(content, int(req.GetMaxBytes()))]
	}
	sum := sha256.Sum256([]byte(content))
	return &knowledgev1.FetchResponse{
		Content: []byte(content), Digest: &typesv1.Digest{Value: "sha256:" + hex.EncodeToString(sum[:])}, TotalBytes: uint64(total), //nolint:gosec // a length
		Version: version, LastModified: timestamppb.New(d.modified), MediaType: d.mediaType,
	}, nil
}

// cutAt returns the largest index at most n that is a character boundary of s.
func cutAt(s string, n int) int {
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return n
}

// Resolve returns the canonical ref of a document.
func (s *Source) Resolve(req *knowledgev1.ResolveRequest) (*knowledgev1.ResolveResponse, error) {
	if req.GetRef() == "" || len(req.GetRef()) > 1024 || !scopeOK(req.GetScope()) {
		return nil, invalid()
	}
	ref := strings.TrimSuffix(req.GetRef(), "/")
	d, ok := s.find(ref, req.GetScope())
	if !ok {
		return nil, notFound()
	}
	return &knowledgev1.ResolveResponse{CanonicalRef: d.ref}, nil
}
