package kindyaml

import (
	"fmt"
	"strings"

	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
)

// MaxEvalCases bounds the cases of one EvalSet.
const MaxEvalCases = 256

// MaxTokenBytes bounds an EvalSet token: a case name, a layer or a seeded defect.
const MaxTokenBytes = 64

// checkEvalSet enforces what the schema and the per-field rules cannot (RFC-0004): 1 to MaxEvalCases cases sorted by name
// without duplicates, expected layers sorted without duplicates, a seeded case that has a category and expects its defect
// caught (not accepted, not merged, not severe, and the catching layer among its expected failures), and a planted security
// bug or migration hazard that is seeded.
func checkEvalSet(spec *kindsv1.EvalSetSpec) error {
	cases := spec.GetCases()
	if len(cases) > MaxEvalCases {
		return fmt.Errorf("%w: an EvalSet has 1 to %d cases", ErrInvalid, MaxEvalCases)
	}
	for i, c := range cases {
		if i > 0 && cases[i-1].GetName() >= c.GetName() {
			return fmt.Errorf("%w: EvalSet cases are sorted by name without duplicates", ErrInvalid)
		}
		fails := c.GetExpect().GetVerificationFailures()
		for j := 1; j < len(fails); j++ {
			if fails[j-1].GetLayer() >= fails[j].GetLayer() {
				return fmt.Errorf("%w: expected verification failures are sorted by layer without duplicates", ErrInvalid)
			}
		}
		if err := checkEvalAdversarial(c); err != nil {
			return err
		}
	}
	return nil
}

func checkEvalAdversarial(c *kindsv1.EvalCase) error {
	if c.Category == nil {
		if c.GetSeeded() != nil {
			return fmt.Errorf("%w: an EvalSet case that plants a defect names a category", ErrInvalid)
		}
		return nil
	}
	s := c.GetSeeded()
	if s == nil {
		if cat := c.GetCategory(); cat == kindsv1.EvalCategory_EVAL_CATEGORY_PLANTED_SECURITY_BUG || cat == kindsv1.EvalCategory_EVAL_CATEGORY_MIGRATION_HAZARD {
			return fmt.Errorf("%w: an EvalSet case of a planted security bug or migration hazard is seeded", ErrInvalid)
		}
		return nil
	}
	e := c.GetExpect()
	if e.GetMerged() || e.GetSevere() || e.GetAcceptance() == kindsv1.EvalAcceptance_EVAL_ACCEPTANCE_ACCEPTED {
		return fmt.Errorf("%w: a seeded EvalSet case expects the change not accepted, not merged and not severe", ErrInvalid)
	}
	for _, f := range e.GetVerificationFailures() {
		if f.GetLayer() == s.GetCaughtBy() {
			return nil
		}
	}
	return fmt.Errorf("%w: a seeded EvalSet case expects its catching layer among the failures", ErrInvalid)
}

// isToken reports whether s is an EvalSet token: 1 to MaxTokenBytes bytes of lowercase ASCII letters, digits and ". _ -",
// starting with a letter or digit.
func isToken(s string) bool {
	if s == "" || len(s) > MaxTokenBytes {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || i > 0 && strings.IndexByte("._-", c) >= 0 {
			continue
		}
		return false
	}
	return true
}

// isSemver reports whether s is a strict x.y.z version: three decimal numbers of at most nine digits, no sign, no leading
// zero, no pre-release or build part.
func isSemver(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 9 || p[0] == '0' && len(p) > 1 {
			return false
		}
		for i := range len(p) {
			if p[i] < '0' || p[i] > '9' {
				return false
			}
		}
	}
	return true
}
