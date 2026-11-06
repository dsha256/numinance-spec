// Package verifierkit is the runnable skeleton of the verifier conformance kit, version 1.0.0 (README.md in this directory
// lists the full case table; this package runs the subset a provider can pass with the fixtures a publisher supplies). It is
// the acceptance harness for a provider of numinance.ports.verifier.v1: the harness reaches the provider however it likes (a
// host dispatching Invoke, a replay, a direct call) through Target, and the cases judge only what comes back. It imports the
// public spec only.
//
// The kit is generic: a publisher supplies a Subject, the candidates and configs the provider's checks can be run on. The
// clean candidate has no violation; the seeded candidate has the violations Subject.Seeded lists; the suppressed candidate
// carries in-tree directives that would hide a violation; the crashing candidate makes the check crash; the malformed
// candidate makes the tool print output that is not its own. Response rules (ValidateResponse) are the port's own and are
// also what a host enforces on any provider.
package verifierkit

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	verifierv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/verifier/v1"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

// CallError is the error a Target returns for a call the provider answered with FAILED; Reason is the reason of the
// diagnostic it carried. Any other error means the harness could not get an answer.
type CallError struct{ Reason pluginv1.Reason }

// Error implements error.
func (f *CallError) Error() string { return "provider failed the call: " + f.Reason.String() }

// Target is the provider under test.
type Target interface {
	Run(context.Context, *verifierv1.RunRequest) (*verifierv1.RunResponse, error)
}

// Fixture is one candidate with the config the check runs under. The references are the harness's: it resolves them
// however a host would.
type Fixture struct {
	Candidate, Config *typesv1.Reference
}

// Expected is a violation the seeded candidate holds.
type Expected struct {
	RuleID   string
	Severity typesv1.FindingSeverity
	Path     string
}

// Subject is what a publisher supplies for the kit.
type Subject struct {
	Target Target
	// Clean runs the check on a candidate with no violation.
	Clean Fixture
	// Seeded runs it on a candidate that holds every violation of Expect.
	Seeded Fixture
	Expect []Expected
	// Suppressed is a candidate whose violation an in-tree directive would hide; the check must still report the violation or
	// report the directive as a finding with a rule id starting DirectivePrefix.
	Suppressed Fixture
	// Crashing and Malformed make the check crash and print output that is not its own.
	Crashing, Malformed Fixture
	// UnknownCheck is a config naming a check the provider cannot run.
	UnknownCheck *typesv1.Reference
}

// DirectivePrefix starts the rule id of a finding that reports a suppression directive the tool cannot ignore.
const DirectivePrefix = "numinance.directive/"

// Case is one conformance case: an id of the README table and the check.
type Case struct {
	ID, Name string
	Check    func(ctx context.Context, s Subject) error
}

// Cases returns the cases this skeleton runs, in README order.
func Cases() []Case {
	return []Case{
		{"CORE-001", "the clean candidate: no items, a summary with no counts", core001},
		{"CORE-002", "the seeded candidate: every seeded violation with its rule id, severity and path; counts equal the items", core002},
		{"CORE-003", "the seeded candidate twice: the same fingerprints", core003},
		{"CORE-005", "every item within its bounds", core005},
		{"CORE-006", "an absent or malformed reference is INVALID_ARGUMENT", core006},
		{"CORE-008", "the crashing candidate is FAILED, never a partial list", core008},
		{"CORE-009", "a config naming a check the provider cannot run is UNAVAILABLE", core009},
		{"CORE-010", "suppression in the candidate does not hide a violation", core010},
		{"CORE-014", "output that is malformed or declares an entity is FAILED", core014},
	}
}

// Run runs every case against the subject as subtests of t.
func Run(t *testing.T, s Subject) {
	t.Helper()
	for _, c := range Cases() {
		t.Run(c.ID, func(t *testing.T) {
			if err := c.Check(t.Context(), s); err != nil {
				t.Errorf("%s (%s): %v", c.ID, c.Name, err)
			}
		})
	}
}

func run(ctx context.Context, s Subject, f Fixture) (*verifierv1.RunResponse, error) {
	return s.Target.Run(ctx, &verifierv1.RunRequest{CandidateRef: f.Candidate, ConfigRef: f.Config})
}

func wantReason(err error, reason pluginv1.Reason) error {
	var f *CallError
	if !errors.As(err, &f) || f.Reason != reason {
		return fmt.Errorf("error = %w, want a FAILED call with reason %s", err, reason)
	}
	return nil
}

func core001(ctx context.Context, s Subject) error {
	resp, err := run(ctx, s, s.Clean)
	if err != nil {
		return err
	}
	if len(resp.GetItems()) != 0 || resp.GetSummary() == nil || len(resp.GetSummary().GetCounts()) != 0 {
		return fmt.Errorf("items = %d, summary = %v, want no items and a summary with no counts", len(resp.GetItems()), resp.GetSummary())
	}
	return nil
}

func core002(ctx context.Context, s Subject) error {
	resp, err := run(ctx, s, s.Seeded)
	if err != nil {
		return err
	}
	if err := ValidateResponse(resp); err != nil {
		return err
	}
	for _, e := range s.Expect {
		if !slices.ContainsFunc(resp.GetItems(), func(f *verifierv1.Finding) bool {
			return f.GetRuleId() == e.RuleID && f.GetSeverity() == e.Severity && f.GetLocation().GetPath() == e.Path
		}) {
			return fmt.Errorf("the seeded violation %s (%s in %s) is not reported", e.RuleID, e.Severity, e.Path)
		}
	}
	return nil
}

func core003(ctx context.Context, s Subject) error {
	a, err := run(ctx, s, s.Seeded)
	if err != nil {
		return err
	}
	b, err := run(ctx, s, s.Seeded)
	if err != nil {
		return err
	}
	fps := func(r *verifierv1.RunResponse) []string {
		out := make([]string, 0, len(r.GetItems()))
		for _, f := range r.GetItems() {
			out = append(out, f.GetFingerprint())
		}
		slices.Sort(out)
		return out
	}
	if !slices.Equal(fps(a), fps(b)) {
		return errors.New("two runs of the seeded candidate report different fingerprints")
	}
	return nil
}

func core005(ctx context.Context, s Subject) error {
	resp, err := run(ctx, s, s.Seeded)
	if err != nil {
		return err
	}
	return ValidateResponse(resp)
}

func core006(ctx context.Context, s Subject) error {
	ref := &typesv1.Reference{ArtifactRef: "x", Sha256: &typesv1.Digest{Value: "not-a-digest"}}
	for name, req := range map[string]*verifierv1.RunRequest{
		"no candidate":       {ConfigRef: s.Seeded.Config},
		"no config":          {CandidateRef: s.Seeded.Candidate},
		"a malformed ref":    {CandidateRef: ref, ConfigRef: s.Seeded.Config},
		"a malformed config": {CandidateRef: s.Seeded.Candidate, ConfigRef: ref},
	} {
		_, err := s.Target.Run(ctx, req)
		if werr := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); werr != nil {
			return fmt.Errorf("%s: %w", name, werr)
		}
	}
	return nil
}

func core008(ctx context.Context, s Subject) error {
	resp, err := run(ctx, s, s.Crashing)
	if err == nil {
		return fmt.Errorf("a crashing check was SUCCEEDED with %d items", len(resp.GetItems()))
	}
	return wantReason(err, pluginv1.Reason_REASON_UPSTREAM_ERROR)
}

func core009(ctx context.Context, s Subject) error {
	_, err := run(ctx, s, Fixture{Candidate: s.Seeded.Candidate, Config: s.UnknownCheck})
	return wantReason(err, pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE)
}

func core010(ctx context.Context, s Subject) error {
	resp, err := run(ctx, s, s.Suppressed)
	if err != nil {
		return err
	}
	if len(resp.GetItems()) == 0 {
		return errors.New("a candidate that suppresses its violation reports nothing")
	}
	return ValidateResponse(resp)
}

func core014(ctx context.Context, s Subject) error {
	resp, err := run(ctx, s, s.Malformed)
	if err == nil {
		return fmt.Errorf("malformed output was SUCCEEDED with %d items", len(resp.GetItems()))
	}
	return wantReason(err, pluginv1.Reason_REASON_UPSTREAM_ERROR)
}

var (
	ruleRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)
	fpRe   = regexp.MustCompile(`^[\x21-\x7e]{1,256}$`)
)

func printable(r rune) bool {
	return r == ' ' || unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S)
}

// ValidateResponse checks a RunResponse against the rules of verifier.proto: the bounds of every field, unique fingerprints,
// items most severe first and a summary that counts them (HOST-005 and HOST-006). A host enforces the same rules on any provider.
func ValidateResponse(r *verifierv1.RunResponse) error {
	items := r.GetItems()
	if len(items) > 1024 || r.GetSummary() == nil {
		return errors.New("more than 1024 items, or no summary")
	}
	seen := map[string]bool{}
	listed := map[typesv1.FindingSeverity]uint64{}
	for i, f := range items {
		switch {
		case !ruleRe.MatchString(f.GetRuleId()):
			return fmt.Errorf("items[%d]: rule_id out of bounds", i)
		case f.GetSeverity() < typesv1.FindingSeverity_FINDING_SEVERITY_INFO || f.GetSeverity() > typesv1.FindingSeverity_FINDING_SEVERITY_CRITICAL:
			return fmt.Errorf("items[%d]: severity is zero or unknown", i)
		case !validMessage(f.GetMessage()):
			return fmt.Errorf("items[%d]: message out of bounds", i)
		case !fpRe.MatchString(f.GetFingerprint()) || seen[f.GetFingerprint()]:
			return fmt.Errorf("items[%d]: fingerprint out of bounds or repeated", i)
		case i > 0 && items[i-1].GetSeverity() < f.GetSeverity():
			return fmt.Errorf("items[%d]: not most severe first", i)
		}
		seen[f.GetFingerprint()] = true
		listed[f.GetSeverity()]++
		if loc := f.GetLocation(); loc != nil {
			if !validPath(loc.GetPath()) || loc.GetEndLine() < loc.GetStartLine() || (loc.GetStartLine() == 0) != (loc.GetEndLine() == 0) {
				return fmt.Errorf("items[%d]: location out of bounds", i)
			}
		}
	}
	return validateCounts(r.GetSummary().GetCounts(), listed, len(items))
}

func validateCounts(counts []*verifierv1.SeverityCount, listed map[typesv1.FindingSeverity]uint64, items int) error {
	var total uint64
	seen := map[typesv1.FindingSeverity]bool{}
	least := typesv1.FindingSeverity_FINDING_SEVERITY_CRITICAL + 1
	for _, c := range counts {
		if c.GetCount() < 1 || c.GetCount() >= 1<<53 || seen[c.GetSeverity()] || c.GetSeverity() < typesv1.FindingSeverity_FINDING_SEVERITY_INFO || c.GetSeverity() > typesv1.FindingSeverity_FINDING_SEVERITY_CRITICAL {
			return errors.New("summary: a count is zero, repeated or of an unknown severity")
		}
		seen[c.GetSeverity()] = true
		total += c.GetCount()
		if listed[c.GetSeverity()] > 0 && c.GetSeverity() < least {
			least = c.GetSeverity()
		}
	}
	for sev, n := range listed {
		if !seen[sev] {
			return errors.New("summary: an item of a severity with no count")
		}
		for _, c := range counts {
			if c.GetSeverity() != sev {
				continue
			}
			if c.GetCount() < n || (sev > least && c.GetCount() != n) {
				return errors.New("summary: a count is below its listed items, or above them for a severity above the least severe listed")
			}
		}
	}
	if want := min(total, 1024); uint64(items) != want { //nolint:gosec // items is a length
		return fmt.Errorf("summary: %d items for a total of %d", items, total)
	}
	return nil
}

func validMessage(s string) bool {
	if len(s) < 1 || len(s) > 1024 || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r != '\t' && r != '\n' && !printable(r) && r != utf8.RuneError {
			return false
		}
	}
	return true
}

func validPath(p string) bool {
	if p == "" || len(p) > 1024 || strings.HasPrefix(p, "/") || strings.ContainsRune(p, '\\') {
		return false
	}
	for seg := range strings.SplitSeq(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
		for _, r := range seg {
			if !printable(r) {
				return false
			}
		}
	}
	return true
}
