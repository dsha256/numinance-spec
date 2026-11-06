package kindyaml_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

const evalSetExample = "EvalSet/core-replay.yaml"

func alphaLoader(t *testing.T) *kindyaml.Loader {
	t.Helper()
	l, err := kindyaml.NewLoader(kindyaml.WithAlphaKinds())
	if err != nil {
		t.Fatalf("NewLoader(WithAlphaKinds) = %v", err)
	}
	return l
}

func TestEvalSet_Decode_Example(t *testing.T) {
	m, err := alphaLoader(t).Decode([]byte(example(t, evalSetExample)))
	if err != nil {
		t.Fatalf("Decode(%s) = %v", evalSetExample, err)
	}
	got, ok := m.(*kindsv1.EvalSet)
	if !ok {
		t.Fatalf("Decode(%s) = %T, want *kindsv1.EvalSet", evalSetExample, m)
	}
	spec := got.GetSpec()
	if spec.GetVersion() != "1.0.0" || len(spec.GetCases()) != 3 {
		t.Errorf("Decode(%s) = version %q with %d cases, want 1.0.0 with 3", evalSetExample, spec.GetVersion(), len(spec.GetCases()))
	}
	c := spec.GetCases()[2]
	if c.GetCategory() != kindsv1.EvalCategory_EVAL_CATEGORY_PLANTED_SECURITY_BUG || c.GetSeeded().GetCaughtBy() != "security-scan" {
		t.Errorf("Decode(%s) case 3 = %v, want the planted security bug caught by security-scan", evalSetExample, c)
	}
}

// TestEvalSet_RoundTrip is the lossless round trip of the kind: Decode(Encode(m)) equals m.
func TestEvalSet_RoundTrip(t *testing.T) {
	l := alphaLoader(t)
	m, err := l.Decode([]byte(example(t, evalSetExample)))
	if err != nil {
		t.Fatalf("Decode(%s) = %v", evalSetExample, err)
	}
	out, err := l.Encode(m)
	if err != nil {
		t.Fatalf("Encode(%s) = %v", evalSetExample, err)
	}
	back, err := l.Decode(out)
	if err != nil || !proto.Equal(back, m) {
		t.Errorf("Decode(Encode(m)) = %v, %v; want %v", back, err, m)
	}
	again, err := l.Encode(back)
	if err != nil || string(again) != string(out) {
		t.Errorf("Encode(Decode(Encode(m))) differs from Encode(m): err %v", err)
	}
}

// TestEvalSet_AlphaGate: the kind loads only through WithAlphaKinds, and only at its alpha apiVersion.
func TestEvalSet_AlphaGate(t *testing.T) {
	doc := example(t, evalSetExample)
	t.Run("the default loader refuses it", func(t *testing.T) {
		m, err := newLoader(t).Decode([]byte(doc))
		if !errors.Is(err, kindyaml.ErrAlpha) || !errors.Is(err, kindyaml.ErrKind) || m != nil {
			t.Errorf("Decode(EvalSet) = %v, %v; want ErrAlpha wrapping ErrKind", m, err)
		}
	})
	t.Run("the default loader refuses to encode it", func(t *testing.T) {
		m, err := alphaLoader(t).Decode([]byte(doc))
		if err != nil {
			t.Fatalf("Decode(EvalSet) = %v", err)
		}
		if out, err := newLoader(t).Encode(m); !errors.Is(err, kindyaml.ErrAlpha) {
			t.Errorf("Encode(EvalSet) = %q, %v; want ErrAlpha", out, err)
		}
	})
	t.Run("the stable apiVersion is refused for it", func(t *testing.T) {
		_, err := alphaLoader(t).Decode([]byte(strings.Replace(doc, "numinance.dev/v1alpha1", "numinance.dev/v1", 1)))
		if !errors.Is(err, kindyaml.ErrInvalid) {
			t.Errorf("Decode(EvalSet at v1) = %v, want %v", err, kindyaml.ErrInvalid)
		}
	})
	t.Run("the alpha apiVersion is refused for a stable kind", func(t *testing.T) {
		b := strings.Replace(example(t, "BudgetPolicy/org-default.yaml"), "numinance.dev/v1", "numinance.dev/v1alpha1", 1)
		_, err := alphaLoader(t).Decode([]byte(b))
		if !errors.Is(err, kindyaml.ErrInvalid) {
			t.Errorf("Decode(BudgetPolicy at v1alpha1) = %v, want %v", err, kindyaml.ErrInvalid)
		}
	})
}

// TestEvalSet_Decode_Rejects has one failing case per loader check of RFC-0004.
func TestEvalSet_Decode_Rejects(t *testing.T) {
	doc := example(t, evalSetExample)
	tests := []struct {
		name string
		old  string
		new  string
		want error
	}{
		{name: "unknown field", old: "  version: 1.0.0\n", new: "  version: 1.0.0\n  note: free text\n", want: kindyaml.ErrSchema},
		{name: "unknown case field", old: "      category: EVAL_CATEGORY_PLANTED_SECURITY_BUG\n", new: "      category: EVAL_CATEGORY_PLANTED_SECURITY_BUG\n      description: free text\n", want: kindyaml.ErrSchema},
		{name: "extension key that is not reverse-DNS", old: "spec:\n", new: "extensions: {vendor: 1}\nspec:\n", want: kindyaml.ErrInvalid},
		{name: "version with a pre-release part", old: "version: 1.0.0", new: "version: 1.0.0-rc.1", want: kindyaml.ErrInvalid},
		{name: "version with a leading zero", old: "version: 1.0.0", new: "version: 1.00.0", want: kindyaml.ErrInvalid},
		{name: "version with two parts", old: "version: 1.0.0", new: `version: "1.0"`, want: kindyaml.ErrInvalid},
		{name: "case name with an upper-case letter", old: "name: bugfix-clean", new: "name: Bugfix-clean", want: kindyaml.ErrInvalid},
		{name: "case name starting with a separator", old: "name: bugfix-clean", new: "name: _bugfix-clean", want: kindyaml.ErrInvalid},
		{name: "case name over 64 bytes", old: "name: bugfix-clean", new: "name: " + strings.Repeat("a", 65), want: kindyaml.ErrInvalid},
		{name: "cases not sorted by name", old: "name: bugfix-clean", new: "name: zzz-clean", want: kindyaml.ErrInvalid},
		{name: "duplicate case name", old: "name: bugfix-lint-retry", new: "name: bugfix-clean", want: kindyaml.ErrInvalid},
		{name: "outcome that is not a digest", old: "0b2958497ee0c936c59fa8dfb74b43077ad626e5dafd62df6521c4dd1e581242", new: "0B2958497EE0C936C59FA8DFB74B43077AD626E5DAFD62DF6521C4DD1E581242", want: kindyaml.ErrInvalid},
		{name: "acceptance of the zero value", old: "acceptance: EVAL_ACCEPTANCE_REJECTED", new: "acceptance: EVAL_ACCEPTANCE_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "unknown acceptance", old: "acceptance: EVAL_ACCEPTANCE_REJECTED", new: "acceptance: EVAL_ACCEPTANCE_MAYBE", want: kindyaml.ErrSchema},
		{name: "merged omitted", old: "expect: {acceptance: EVAL_ACCEPTANCE_ACCEPTED, merged: true, severe: false, verification_failures: []}", new: "expect: {acceptance: EVAL_ACCEPTANCE_ACCEPTED, severe: false, verification_failures: []}", want: kindyaml.ErrSchema},
		{name: "layer count of zero", old: "{layer: lint, count: 1}", new: "{layer: lint, count: 0}", want: kindyaml.ErrInvalid},
		{name: "layer that is not a token", old: "{layer: lint, count: 1}", new: "{layer: Lint, count: 1}", want: kindyaml.ErrInvalid},
		{name: "layers not sorted", old: "          - {layer: security-scan, count: 1}\n          - {layer: tests, count: 2}\n", new: "          - {layer: tests, count: 2}\n          - {layer: security-scan, count: 1}\n", want: kindyaml.ErrInvalid},
		{name: "duplicate layer", old: "{layer: tests, count: 2}", new: "{layer: security-scan, count: 2}", want: kindyaml.ErrInvalid},
		{name: "category of the zero value", old: "category: EVAL_CATEGORY_PLANTED_SECURITY_BUG", new: "category: EVAL_CATEGORY_UNSPECIFIED", want: kindyaml.ErrInvalid},
		{name: "seeded without a category", old: "      category: EVAL_CATEGORY_PLANTED_SECURITY_BUG\n", new: "", want: kindyaml.ErrInvalid},
		{name: "planted security bug that is not seeded", old: "      seeded: {defect: sql-injection, caught_by: security-scan}\n", new: "", want: kindyaml.ErrInvalid},
		{name: "seeded defect that is not a token", old: "defect: sql-injection", new: "defect: SQL injection", want: kindyaml.ErrInvalid},
		{name: "seeded case expecting acceptance", old: "acceptance: EVAL_ACCEPTANCE_REJECTED", new: "acceptance: EVAL_ACCEPTANCE_ACCEPTED", want: kindyaml.ErrInvalid},
		{name: "seeded case expecting a merge", old: "        merged: false\n        severe: false\n        verification_failures:\n          - {layer: security-scan", new: "        merged: true\n        severe: false\n        verification_failures:\n          - {layer: security-scan", want: kindyaml.ErrInvalid},
		{name: "seeded case expecting a severity event", old: "        merged: false\n        severe: false\n        verification_failures:\n          - {layer: security-scan", new: "        merged: false\n        severe: true\n        verification_failures:\n          - {layer: security-scan", want: kindyaml.ErrInvalid},
		{name: "catching layer not among the failures", old: "caught_by: security-scan", new: "caught_by: fuzz", want: kindyaml.ErrInvalid},
		{name: "scope unspecified", old: "scope: SCOPE_REPO", new: "scope: SCOPE_UNSPECIFIED", want: kindyaml.ErrInvalid},
	}
	l := alphaLoader(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(doc, tt.old) {
				t.Fatalf("the example has no %q", tt.old)
			}
			b := strings.Replace(doc, tt.old, tt.new, 1)
			m, err := l.Decode([]byte(b))
			if !errors.Is(err, tt.want) {
				t.Errorf("Decode(%s) = %v, %v; want error %v", tt.name, m, err, tt.want)
			}
		})
	}
}

// TestEvalSet_Decode_CaseBounds pins the 1 to 256 case count at its edges.
func TestEvalSet_Decode_CaseBounds(t *testing.T) {
	build := func(n int) string {
		var b strings.Builder
		b.WriteString("apiVersion: numinance.dev/v1alpha1\nkind: EvalSet\nmetadata: {name: s, scope: SCOPE_REPO}\nspec:\n  version: 1.0.0\n  cases:")
		if n == 0 {
			b.WriteString(" []\n")
		}
		b.WriteString("\n")
		for i := range n {
			fmt.Fprintf(&b, "    - name: c%04d\n      outcome: {value: \"sha256:%064x\"}\n      expect: {acceptance: EVAL_ACCEPTANCE_ACCEPTED, merged: true, severe: false}\n", i, i)
		}
		return b.String()
	}
	l := alphaLoader(t)
	tests := []struct {
		cases int
		want  error
	}{
		{cases: 0, want: kindyaml.ErrInvalid},
		{cases: 1, want: nil},
		{cases: kindyaml.MaxEvalCases, want: nil},
		{cases: kindyaml.MaxEvalCases + 1, want: kindyaml.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d cases", tt.cases), func(t *testing.T) {
			m, err := l.Decode([]byte(build(tt.cases)))
			if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
				t.Errorf("Decode(%d cases) = %v, %v; want error %v", tt.cases, m, err, tt.want)
			}
		})
	}
}

// TestEvalSet_Decode_ErrorsDoNotQuoteTheDocument: the errors of the loader checks name a rule, never a value.
func TestEvalSet_Decode_ErrorsDoNotQuoteTheDocument(t *testing.T) {
	const marker = "Zq9-marker-name"
	doc := strings.Replace(example(t, evalSetExample), "name: bugfix-clean", "name: "+marker, 1)
	_, err := alphaLoader(t).Decode([]byte(doc))
	if !errors.Is(err, kindyaml.ErrInvalid) || strings.Contains(err.Error(), marker) {
		t.Errorf("Decode(bad case name) = %v; want ErrInvalid that does not quote the name", err)
	}
}
