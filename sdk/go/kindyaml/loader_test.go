package kindyaml_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
	"github.com/numinance/numinance-spec/sdk/go/schema"
)

const indexer = `apiVersion: numinance.dev/v1
kind: IndexerPlugin
metadata: {name: polyglot, scope: SCOPE_REPO}
spec:
  indexers:
    - provider: dev.numinance.official/scip-go
      languages: [go]
`

const budget = `apiVersion: numinance.dev/v1
kind: BudgetPolicy
metadata: {name: org-default, scope: SCOPE_ORG}
spec: {tokens: 400000, wall_clock: 7200s, max_risk_escalation: RISK_CLASS_MEDIUM}
`

func newLoader(t *testing.T) *kindyaml.Loader {
	t.Helper()
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatalf("NewLoader() = %v", err)
	}
	return l
}

func TestLoader_Decode_Valid(t *testing.T) {
	l := newLoader(t)
	m, err := l.Decode([]byte(indexer))
	if err != nil {
		t.Fatalf("Decode(indexer) = %v", err)
	}
	got, ok := m.(*kindsv1.IndexerPlugin)
	if !ok {
		t.Fatalf("Decode(indexer) = %T, want *kindsv1.IndexerPlugin", m)
	}
	if p := got.GetSpec().GetIndexers()[0].GetProvider(); p != "dev.numinance.official/scip-go" {
		t.Errorf("Decode(indexer) provider = %q, want %q", p, "dev.numinance.official/scip-go")
	}
	if s := got.GetMetadata().GetScope(); s != kindsv1.Scope_SCOPE_REPO {
		t.Errorf("Decode(indexer) scope = %v, want %v", s, kindsv1.Scope_SCOPE_REPO)
	}
	b, err := l.Decode([]byte(budget))
	if err != nil {
		t.Fatalf("Decode(budget) = %v", err)
	}
	if d := b.(*kindsv1.BudgetPolicy).GetSpec().GetWallClock().AsDuration().Seconds(); d != 7200 {
		t.Errorf("Decode(budget) wall_clock = %vs, want 7200s", d)
	}
}

// TestLoader_Decode_Rejects has one failing case per loader check and per
// refused YAML construct.
func TestLoader_Decode_Rejects(t *testing.T) {
	l := newLoader(t)
	tests := []struct {
		name string
		doc  string
		want error
	}{
		{name: "unknown top-level field", doc: indexer + "colour: blue\n", want: kindyaml.ErrSchema},
		{name: "unknown spec field", doc: strings.Replace(indexer, "  indexers:", "  colour: blue\n  indexers:", 1), want: kindyaml.ErrSchema},
		{name: "proto name instead of apiVersion", doc: strings.Replace(indexer, "apiVersion:", "api_version:", 1), want: kindyaml.ErrSchema},
		{name: "enum as a number", doc: strings.Replace(indexer, "SCOPE_REPO", "2", 1), want: kindyaml.ErrSchema},
		{name: "duration not in protobuf JSON form", doc: strings.Replace(budget, "7200s", "2h", 1), want: kindyaml.ErrSchema},
		{name: "missing kind", doc: strings.Replace(indexer, "kind: IndexerPlugin\n", "", 1), want: kindyaml.ErrKind},
		{name: "unknown kind", doc: strings.Replace(indexer, "kind: IndexerPlugin", "kind: Nope", 1), want: kindyaml.ErrKind},
		{name: "kind names a message that is not a kind", doc: strings.Replace(indexer, "kind: IndexerPlugin", "kind: Metadata", 1), want: kindyaml.ErrKind},
		{name: "kind is not a string", doc: strings.Replace(indexer, "kind: IndexerPlugin", "kind: [IndexerPlugin]", 1), want: kindyaml.ErrKind},
		{name: "wrong apiVersion", doc: strings.Replace(indexer, "numinance.dev/v1", "numinance.dev/v2", 1), want: kindyaml.ErrInvalid},
		{name: "missing metadata", doc: strings.Replace(indexer, "metadata: {name: polyglot, scope: SCOPE_REPO}\n", "", 1), want: kindyaml.ErrInvalid},
		{name: "missing spec", doc: before(indexer, "spec:"), want: kindyaml.ErrInvalid},
		{name: "empty metadata name", doc: strings.Replace(indexer, "name: polyglot", `name: ""`, 1), want: kindyaml.ErrInvalid},
		{name: "metadata name with NUL", doc: strings.Replace(indexer, "name: polyglot", `name: "a\0b"`, 1), want: kindyaml.ErrInvalid},
		{name: "metadata name with an escape sequence", doc: strings.Replace(indexer, "name: polyglot", `name: "a\e[0m"`, 1), want: kindyaml.ErrInvalid},
		{name: "metadata name with a zero-width space", doc: strings.Replace(indexer, "name: polyglot", "name: a\u200bb", 1), want: kindyaml.ErrInvalid},
		{name: "metadata name with a tag character", doc: strings.Replace(indexer, "name: polyglot", "name: a\U000e0041b", 1), want: kindyaml.ErrInvalid},
		{name: "metadata name with a bidi override", doc: strings.Replace(indexer, "name: polyglot", "name: a\u202eb", 1), want: kindyaml.ErrInvalid},
		{name: "metadata name over 253 bytes", doc: strings.Replace(indexer, "name: polyglot", "name: "+strings.Repeat("a", 254), 1), want: kindyaml.ErrInvalid},
		{name: "empty required scalar", doc: strings.Replace(indexer, "provider: dev.numinance.official/scip-go", `provider: ""`, 1), want: kindyaml.ErrInvalid},
		{name: "zero enum, implicit presence", doc: strings.Replace(indexer, "SCOPE_REPO", "SCOPE_UNSPECIFIED", 1), want: kindyaml.ErrInvalid},
		{name: "zero enum, optional field", doc: strings.Replace(budget, "RISK_CLASS_MEDIUM", "RISK_CLASS_UNSPECIFIED", 1), want: kindyaml.ErrInvalid},
		{name: "empty required list", doc: before(indexer, "  indexers:") + "  indexers: []\n", want: kindyaml.ErrInvalid},
		{name: "extensions key with one label", doc: indexer + "extensions: {nightly: true}\n", want: kindyaml.ErrInvalid},
		{name: "extensions key in upper case", doc: indexer + "extensions: {Com.example: 1}\n", want: kindyaml.ErrInvalid},
		{name: "extensions key with an empty label", doc: indexer + "extensions: {com..example: 1}\n", want: kindyaml.ErrInvalid},
		{name: "extensions label starts with a digit", doc: indexer + "extensions: {com.1example: 1}\n", want: kindyaml.ErrInvalid},
		{name: "extensions label ends with a hyphen", doc: indexer + "extensions: {com.example-: 1}\n", want: kindyaml.ErrInvalid},
		{name: "extensions key over 253 bytes", doc: indexer + "extensions: {com." + strings.Repeat("a", 250) + ": 1}\n", want: kindyaml.ErrInvalid},
		{name: "anchor", doc: strings.Replace(indexer, "languages: [go]", "languages: &l [go]", 1), want: kindyaml.ErrUnsupported},
		{name: "alias", doc: indexer + "extensions: {com.example.a: &a 1, com.example.b: *a}\n", want: kindyaml.ErrUnsupported},
		{name: "merge key", doc: indexer + "extensions:\n  <<: {com.example.a: 1}\n", want: kindyaml.ErrUnsupported},
		{name: "standard tag", doc: strings.Replace(indexer, "name: polyglot", "name: !!str polyglot", 1), want: kindyaml.ErrUnsupported},
		{name: "local tag", doc: strings.Replace(indexer, "name: polyglot", "name: !secret polyglot", 1), want: kindyaml.ErrUnsupported},
		{name: "integer key", doc: indexer + "1: x\n", want: kindyaml.ErrUnsupported},
		{name: "null key", doc: indexer + "null: x\n", want: kindyaml.ErrUnsupported},
		{name: "sequence key", doc: indexer + "? [a]\n: x\n", want: kindyaml.ErrUnsupported},
		{name: "duplicate key", doc: indexer + "kind: IndexerPlugin\n", want: kindyaml.ErrDuplicateKey},
		{name: "duplicate key in flow mapping", doc: strings.Replace(indexer, "scope: SCOPE_REPO", "scope: SCOPE_REPO, name: x", 1), want: kindyaml.ErrDuplicateKey},
		{name: "two documents", doc: indexer + "---\n" + indexer, want: kindyaml.ErrUnsupported},
		{name: "syntax error in a second document", doc: indexer + "---\na: [\n", want: kindyaml.ErrSyntax},
		{name: "root is a sequence", doc: "- a\n", want: kindyaml.ErrUnsupported},
		{name: "root is a scalar", doc: "a\n", want: kindyaml.ErrUnsupported},
		{name: "hexadecimal number", doc: strings.Replace(budget, "400000", "0x10", 1), want: kindyaml.ErrUnsupported},
		{name: "number with underscores", doc: strings.Replace(budget, "400000", "400_000", 1), want: kindyaml.ErrUnsupported},
		{name: "number with a plus sign", doc: strings.Replace(budget, "400000", "+4", 1), want: kindyaml.ErrUnsupported},
		{name: "infinity", doc: strings.Replace(budget, "400000", ".inf", 1), want: kindyaml.ErrUnsupported},
		{name: "not a number", doc: strings.Replace(budget, "400000", ".nan", 1), want: kindyaml.ErrUnsupported},
		{name: "boolean in title case", doc: indexer + "extensions: {com.example.a: True}\n", want: kindyaml.ErrUnsupported},
		{name: "empty input", doc: "", want: kindyaml.ErrSyntax},
		{name: "only a comment", doc: "# nothing\n", want: kindyaml.ErrSyntax},
		{name: "bad indentation", doc: "a: b\n  c: d\n", want: kindyaml.ErrSyntax},
		{name: "invalid UTF-8", doc: indexer + "extensions: {com.example.a: \"\xff\"}\n", want: kindyaml.ErrSyntax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := l.Decode([]byte(tt.doc))
			if !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) = %v, %v; want error %v", tt.doc, m, err, tt.want)
			}
		})
	}
}

func TestLoader_Decode_SchemaViolationIsWrapped(t *testing.T) {
	_, err := newLoader(t).Decode([]byte(indexer + "colour: blue\n"))
	var verr *schema.ValidationError
	if !errors.As(err, &verr) || verr.Violations[0].Keyword != "additionalProperties" {
		t.Errorf("Decode(unknown field) = %v, want a *schema.ValidationError on additionalProperties", err)
	}
}

func TestLoader_Decode_ErrorsDoNotQuoteTheDocument(t *testing.T) {
	const secret = "s3cr3t-value"
	l := newLoader(t)
	for _, doc := range []string{
		strings.Replace(budget, "7200s", secret, 1),
		indexer + "extensions: {" + secret + ": 1}\n",
		strings.Replace(indexer, "kind: IndexerPlugin", "kind: "+secret, 1),
		strings.Replace(indexer, "name: polyglot", "name: "+secret+"\u202e", 1),
		indexer + "extensions: {com.example.a: *" + secret + "}\n", // undefined alias: go-yaml quotes its name
	} {
		if _, err := l.Decode([]byte(doc)); err == nil || strings.Contains(err.Error(), secret) {
			t.Errorf("Decode(%q) = %v, want an error that does not quote the document", doc, err)
		}
	}
	// An unknown key is named, quoted, in the violation path: the documented exception.
	doc := indexer + secret + ": 1\n"
	if _, err := l.Decode([]byte(doc)); err == nil || !strings.Contains(err.Error(), `"/`+secret+`"`) {
		t.Errorf("Decode(%q) = %v, want the unknown key quoted in the violation path", doc, err)
	}
}

func TestLoader_Decode_MaxBytes(t *testing.T) {
	l := newLoader(t)
	pad := func(n int) []byte {
		return []byte(indexer + "#" + strings.Repeat("x", n-len(indexer)-2) + "\n")
	}
	if b := pad(kindyaml.MaxBytes); len(b) != kindyaml.MaxBytes {
		t.Fatalf("pad(MaxBytes) is %d bytes", len(b))
	}
	if _, err := l.Decode(pad(kindyaml.MaxBytes)); err != nil {
		t.Errorf("Decode(MaxBytes bytes) = %v, want nil", err)
	}
	if _, err := l.Decode(pad(kindyaml.MaxBytes + 1)); !errors.Is(err, kindyaml.ErrTooLarge) {
		t.Errorf("Decode(MaxBytes+1 bytes) = %v, want %v", err, kindyaml.ErrTooLarge)
	}
}

// nested returns indexer with an extensions value whose deepest node sits at
// nesting level depth: the root mapping is 1, extensions 2, and each flow
// sequence below it one more.
func nested(depth int) []byte {
	n := depth - 2
	return []byte(indexer + "extensions: {com.example.a: " + strings.Repeat("[", n) + strings.Repeat("]", n) + "}\n")
}

func TestLoader_Decode_MaxDepth(t *testing.T) {
	l := newLoader(t)
	if _, err := l.Decode(nested(kindyaml.MaxDepth)); err != nil {
		t.Errorf("Decode(depth MaxDepth) = %v, want nil", err)
	}
	if _, err := l.Decode(nested(kindyaml.MaxDepth + 1)); !errors.Is(err, kindyaml.ErrDepth) {
		t.Errorf("Decode(depth MaxDepth+1) = %v, want %v", err, kindyaml.ErrDepth)
	}
	// Deeper than the YAML parser's own limit and still under MaxBytes: the
	// parser refuses it before the loader's recursion starts.
	deep := []byte(strings.Repeat("[", 20000))
	if _, err := l.Decode(deep); !errors.Is(err, kindyaml.ErrSyntax) {
		t.Errorf("Decode(20000 open brackets) = %v, want %v", err, kindyaml.ErrSyntax)
	}
}

func TestLoader_Decode_AcceptedScalars(t *testing.T) {
	l := newLoader(t)
	doc := indexer + "extensions:\n  com.example.a: [true, false, null, -1.5e3, 0, \"0x10\", 2026-10-18, ~]\n"
	m, err := l.Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode(JSON scalars in extensions) = %v, want nil", err)
	}
	got, err := protojson.Marshal(m.(*kindsv1.IndexerPlugin).GetExtensions()["com.example.a"])
	if err != nil {
		t.Fatalf("protojson.Marshal(extension) = %v", err)
	}
	var vals []any
	if err := json.Unmarshal(got, &vals); err != nil {
		t.Fatalf("json.Unmarshal(%s) = %v", got, err)
	}
	want := []any{true, false, nil, -1500.0, 0.0, "0x10", "2026-10-18", nil}
	if !reflect.DeepEqual(vals, want) {
		t.Errorf("Decode(JSON scalars) extension = %v, want %v", vals, want)
	}
}

func TestEncode_RoundTrip(t *testing.T) {
	l := newLoader(t)
	doc := strings.Replace(indexer, "languages: [go]", `languages: ["true", "123", "a: b", "", "<<", "a\Nb", "\L", "\P", "\x7f", "\t", " a\n\n"]`, 1) +
		"extensions: {com.example.a: {n: 1.5, s: \"null\", l: [x], \"<<\": \"<<\", \"\\N\": 1}}\n"
	for _, in := range []string{doc, budget} {
		m, err := l.Decode([]byte(in))
		if err != nil {
			t.Fatalf("Decode(%q) = %v", in, err)
		}
		out, err := l.Encode(m)
		if err != nil {
			t.Fatalf("Encode(%v) = %v", m, err)
		}
		if block := strings.ReplaceAll(string(out), "{}", ""); strings.Contains(block, "{") || strings.Contains(block, `"apiVersion"`) {
			t.Errorf("Encode(%v) = %q, want block style with plain keys", m, out)
		}
		back, err := l.Decode(out)
		if err != nil {
			t.Fatalf("Decode(Encode(%q)) = %v; output %q", in, err, out)
		}
		if !proto.Equal(back, m) {
			t.Errorf("Decode(Encode(m)) = %v, want %v", back, m)
		}
	}
}

func TestEncode_ZeroImplicitFieldReadsBack(t *testing.T) {
	l := newLoader(t)
	meta := &kindsv1.Metadata{Name: "x", Scope: kindsv1.Scope_SCOPE_REPO}
	for _, m := range []proto.Message{
		// languages and edge_overlays are zero (empty) implicit-presence fields.
		&kindsv1.IndexerPlugin{
			ApiVersion: kindyaml.APIVersion,
			Kind:       "IndexerPlugin",
			Metadata:   meta,
			Spec:       &kindsv1.IndexerPluginSpec{Indexers: []*kindsv1.Indexer{{Provider: "p"}}},
		},
		&kindsv1.BudgetPolicy{
			ApiVersion: kindyaml.APIVersion,
			Kind:       "BudgetPolicy",
			Metadata:   meta,
			Spec:       &kindsv1.BudgetPolicySpec{Tokens: proto.Uint32(0)},
		},
	} {
		out, err := l.Encode(m)
		if err != nil {
			t.Fatalf("Encode(%v) = %v", m, err)
		}
		if strings.Contains(string(out), "null") {
			t.Errorf("Encode(%v) = %q, want no nulls", m, out)
		}
		if back, err := l.Decode(out); err != nil || !proto.Equal(back, m) {
			t.Errorf("Decode(Encode(%v)) = %v, %v, want %v", m, back, err, m)
		}
	}
}

func TestLoader_Encode_Rejects(t *testing.T) {
	l := newLoader(t)
	tests := []struct {
		name string
		m    proto.Message
		want error
	}{
		{name: "nil message", m: nil, want: kindyaml.ErrKind},
		{name: "typed nil message", m: (*kindsv1.IndexerPlugin)(nil), want: kindyaml.ErrKind},
		{name: "message without api_version", m: &kindsv1.Metadata{Name: "x"}, want: kindyaml.ErrKind},
		{name: "empty required provider", m: &kindsv1.IndexerPlugin{
			ApiVersion: kindyaml.APIVersion,
			Kind:       "IndexerPlugin",
			Metadata:   &kindsv1.Metadata{Name: "x", Scope: kindsv1.Scope_SCOPE_REPO},
			Spec:       &kindsv1.IndexerPluginSpec{Indexers: []*kindsv1.Indexer{{}}},
		}, want: kindyaml.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if out, err := l.Encode(tt.m); !errors.Is(err, tt.want) {
				t.Errorf("Encode(%v) = %q, %v; want error %v", tt.m, out, err, tt.want)
			}
		})
	}
}

func TestLoader_Decode_MaxNodes(t *testing.T) {
	l := newLoader(t)
	// indexer has 12 values; extensions and its list add 2, then n elements.
	doc := func(n int) []byte {
		return []byte(indexer + "extensions: {com.example.a: [" + strings.Repeat("0,", n) + "]}\n")
	}
	if _, err := l.Decode(doc(kindyaml.MaxNodes - 14)); err != nil {
		t.Errorf("Decode(MaxNodes values) = %v, want nil", err)
	}
	if _, err := l.Decode(doc(kindyaml.MaxNodes - 13)); !errors.Is(err, kindyaml.ErrTooManyNodes) {
		t.Errorf("Decode(MaxNodes+1 values) = %v, want %v", err, kindyaml.ErrTooManyNodes)
	}
}

// FuzzDecode checks that Decode never panics and that every message it
// accepts survives Encode and Decode.
func FuzzDecode(f *testing.F) {
	examples := os.DirFS("../../../schemas/examples")
	files, err := fs.Glob(examples, "*/*.yaml")
	if err != nil || len(files) == 0 {
		f.Fatalf("Glob(examples) = %v, %v", files, err)
	}
	for _, p := range files {
		b, err := fs.ReadFile(examples, p)
		if err != nil {
			f.Fatalf("ReadFile(%s) = %v", p, err)
		}
		f.Add(b)
	}
	f.Add([]byte(indexer))
	f.Add([]byte(budget))
	l, err := kindyaml.NewLoader()
	if err != nil {
		f.Fatalf("NewLoader() = %v", err)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := l.Decode(b)
		if err != nil {
			return
		}
		out, err := l.Encode(m)
		if err != nil {
			t.Fatalf("Encode(Decode(%q)) = %v", b, err)
		}
		if back, err := l.Decode(out); err != nil || !proto.Equal(back, m) {
			t.Errorf("Decode(Encode(%v)) = %v, %v, want %v; output %q", m, back, err, m, out)
		}
	})
}

// before returns doc up to the first sep, which the test documents contain.
func before(doc, sep string) string {
	b, _, _ := strings.Cut(doc, sep)
	return b
}

func TestLoader_Decode_ProducedKindWrapsErrProduced(t *testing.T) {
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.Decode([]byte("apiVersion: numinance.dev/v1\nkind: ContextBundle\nmetadata: {name: x}\n"))
	if !errors.Is(err, kindyaml.ErrProduced) || !errors.Is(err, kindyaml.ErrKind) {
		t.Errorf("Decode(ContextBundle) = %v, want kindyaml.ErrProduced and kindyaml.ErrKind", err)
	}
	if _, err = l.Decode([]byte("kind: Nope\n")); errors.Is(err, kindyaml.ErrProduced) {
		t.Errorf("Decode(unknown kind) = %v, must not be kindyaml.ErrProduced", err)
	}
}

func TestLoader_CheckProduced(t *testing.T) {
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatal(err)
	}
	ex, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "examples", "ContextBundle", "implement-stage.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		doc  string
		want error
	}{
		{name: "the example passes", doc: string(ex)},
		{name: "bogus apiVersion", doc: strings.Replace(string(ex), "numinance.dev/v1", "bogus/v9", 1), want: kindyaml.ErrInvalid},
		{name: "unknown field", doc: string(ex) + "surprise: 1\n", want: kindyaml.ErrSchema},
		{name: "camelCase key only the schema rejects", doc: strings.Replace(string(ex), "change_id:", "changeId:", 1), want: kindyaml.ErrSchema},
		{name: "negative context_generation", doc: strings.Replace(string(ex), "context_generation: 1", "context_generation: -1", 1), want: kindyaml.ErrSchema},
		{name: "missing change_id", doc: strings.Replace(string(ex), "  change_id: chg_01JB2Q8W3N5M7K9P1R3T5V7X9Z\n", "", 1), want: kindyaml.ErrSchema},
		{name: "garbage digest", doc: strings.Replace(string(ex), "sha256:5c2e8a1f3b7d9e0c4a6f8b2d1e3c5a7f9b0d2e4c6a8f1b3d5e7c9a0f2b4d6e8c", "garbage", 1), want: kindyaml.ErrInvalid},
		{name: "zero enum", doc: strings.Replace(string(ex), "staleness: SOURCE_STALENESS_FRESH", "staleness: SOURCE_STALENESS_UNSPECIFIED", 1), want: kindyaml.ErrInvalid},
		{name: "retrieved source claiming signed policy trust", doc: strings.Replace(string(ex), "trust: TRUST_CLASS_UNTRUSTED_RETRIEVED}", "trust: TRUST_CLASS_SIGNED_POLICY}", 1), want: kindyaml.ErrInvalid},
		{name: "missing work_unit_id", doc: strings.Replace(string(ex), "  work_unit_id: wu_01JB2Q8Z4P6R8T0V2X4Z6B8D0F\n", "", 1), want: kindyaml.ErrSchema},
		{name: "missing authz_context_digest", doc: strings.Replace(string(ex), "  authz_context_digest: {value: \"sha256:5c2e8a1f3b7d9e0c4a6f8b2d1e3c5a7f9b0d2e4c6a8f1b3d5e7c9a0f2b4d6e8c\"}\n", "", 1), want: kindyaml.ErrInvalid},
		{name: "zero context_generation", doc: strings.Replace(string(ex), "context_generation: 1", "context_generation: 0", 1), want: kindyaml.ErrInvalid},
		{name: "missing fetched_at", doc: strings.Replace(string(ex), "fetched_at: \"2026-10-18T09:12:44Z\",", "", 1), want: kindyaml.ErrInvalid},
		{name: "empty stage", doc: strings.Replace(string(ex), "stage: implement", "stage: \"\"", 1), want: kindyaml.ErrInvalid},
		{name: "empty change_id", doc: strings.Replace(string(ex), "change_id: chg_01JB2Q8W3N5M7K9P1R3T5V7X9Z", "change_id: \"\"", 1), want: kindyaml.ErrInvalid},
		{name: "empty source ref", doc: strings.Replace(string(ex), "{ref: confluence-eng/ENG/pages/4211, system:", "{ref: \"\", system:", 1), want: kindyaml.ErrInvalid},
		{name: "empty model_id", doc: strings.Replace(string(ex), "model_id: eu.anthropic.claude-haiku-v1:0", "model_id: \"\"", 1), want: kindyaml.ErrInvalid},
		{name: "empty tool", doc: strings.Replace(string(ex), "tool: searchConfluenceUsingCql", "tool: \"\"", 1), want: kindyaml.ErrInvalid},
		{name: "empty selection_reason", doc: strings.Replace(string(ex), "selection_reason: Names the retry limits the task changes.", "selection_reason: \"\"", 1), want: kindyaml.ErrInvalid},
		{name: "empty conflict topic", doc: strings.Replace(string(ex), "topic: refund retry limit", "topic: \"\"", 1), want: kindyaml.ErrInvalid},
		{name: "missing resolver", doc: strings.Replace(string(ex), "  resolver: {model_id: eu.anthropic.claude-haiku-v1:0, prompt_version: resolver-3}\n", "", 1), want: kindyaml.ErrInvalid},
		{name: "missing budget_used", doc: strings.Replace(string(ex), "  budget_used: {tokens: 5400, calls: 4, ms: 8200}\n", "", 1), want: kindyaml.ErrInvalid},
		{name: "missing args_digest", doc: strings.Replace(string(ex), "args_digest: {value: \"sha256:7a9c1e3b5d7f9a2c4e6b8d0f1a3c5e7b9d2f4a6c8e0b1d3f5a7c9e2b4d6f8a0c\"}, ", "", 1), want: kindyaml.ErrInvalid},
		{name: "missing source digest", doc: strings.Replace(string(ex), "digest: {value: \"sha256:1e3c5a7f9b2d4e6c8a0f1b3d5e7c9a2f4b6d8e0c1a3f5b7d9e2c4a6f8b0d1e3c\"}, ", "", 1), want: kindyaml.ErrInvalid},
		{name: "empty prompt_version", doc: strings.Replace(string(ex), "prompt_version: resolver-3", "prompt_version: \"\"", 1), want: kindyaml.ErrInvalid},
		{name: "empty system", doc: strings.Replace(string(ex), "system: confluence-eng,", "system: \"\",", 1), want: kindyaml.ErrInvalid},
		{name: "empty work_unit_id", doc: strings.Replace(string(ex), "work_unit_id: wu_01JB2Q8Z4P6R8T0V2X4Z6B8D0F", "work_unit_id: \"\"", 1), want: kindyaml.ErrInvalid},
		{name: "missing spec", doc: "apiVersion: numinance.dev/v1\nkind: ContextBundle\n", want: kindyaml.ErrInvalid},
		{name: "a loaded kind is not produced", doc: "apiVersion: numinance.dev/v1\nkind: LanguageToolchain\n", want: kindyaml.ErrKind},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, err := l.CheckProduced([]byte(tc.doc))
			if !errors.Is(err, tc.want) || (tc.want == nil && m == nil) {
				t.Errorf("CheckProduced = %v, %v; want error %v", m, err, tc.want)
			}
		})
	}
}

func TestLoader_CheckProduced_OnlyUntrustedRetrievedFitsASource(t *testing.T) {
	l, err := kindyaml.NewLoader()
	if err != nil {
		t.Fatal(err)
	}
	ex, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "examples", "ContextBundle", "implement-stage.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const pinned = "trust: TRUST_CLASS_UNTRUSTED_RETRIEVED}"
	for n, name := range typesv1.TrustClass_name {
		if typesv1.TrustClass(n) == typesv1.TrustClass_TRUST_CLASS_UNTRUSTED_RETRIEVED {
			continue
		}
		t.Run(name, func(t *testing.T) {
			doc := strings.Replace(string(ex), pinned, "trust: "+name+"}", 1)
			if _, err := l.CheckProduced([]byte(doc)); !errors.Is(err, kindyaml.ErrInvalid) {
				t.Errorf("CheckProduced(source trust %s) = %v, want ErrInvalid", name, err)
			}
		})
	}
}
