// ticketingkit:scan-exempt: this file holds the hostile inputs of the fixture scan's own mutation tests.

package ticketingkit_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
	annotationsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/annotations/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"
)

func subject(t ticketingkit.Target) ticketingkit.Subject {
	return ticketingkit.Subject{
		Target: t, Project: "PAY", OtherProject: "OPS",
		Found: "PAY-1", FoundState: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN, FoundOperationIDs: []string{"op_0123456789abcdef"},
		Long: "PAY-2", Dirty: "PAY-3", Moved: "PAY-5", Hidden: "PAY-4", Missing: "PAY-999",
		SearchText: "invoice", OtherProjectHit: "OPS-1",
		TransitionOpen: "PAY-6", TransitionStuck: "PAY-7", UnreachableTarget: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED,
		LinkBlocker: "PAY-8", LinkBlocked: "PAY-9", CrossBlocked: "OPS-2",
		ForbiddenProject: "SEC", ForbiddenIssue: "SEC-1",
		HiddenMatch: "PAY-14", HiddenMatchTitle: "Zeppelin",
		TrackerTexts: trackerTexts(),
	}
}

// trackerTexts is every title and body of the fixture: no error message may hold any of them.
func trackerTexts() []string {
	raw, err := os.ReadFile("testdata/tracker.json")
	if err != nil {
		panic(err)
	}
	var f fixtureFile
	if err := json.Unmarshal(raw, &f); err != nil {
		panic(err)
	}
	var out []string
	for _, is := range f.Issues {
		out = append(out, is.Title)
		if is.Body != "" {
			out = append(out, is.Body)
		}
	}
	return out
}

// TestKit_ConformingProviderPasses runs every case against the conforming fake: the kit has no false positive on it.
func TestKit_ConformingProviderPasses(t *testing.T) {
	t.Parallel()
	ticketingkit.Run(t, subject(newFake(t, "")))
}

func failedCases(t *testing.T, bug string) []string {
	t.Helper()
	s := subject(newFake(t, bug))
	var failed []string
	for _, c := range ticketingkit.Cases() {
		if err := c.Check(context.Background(), s); err != nil {
			failed = append(failed, c.ID)
		}
	}
	return failed
}

// TestKit_EachBrokenProviderFailsExactlyItsCases proves each case can fail: a provider broken in one way fails the cases
// that guard that fault and no other case. A fault reached through several verbs (a NOT_FOUND that differs, a project that is
// ignored) fails every case that calls one of them; the list is the whole set, so a case that stops detecting it, or one that
// starts to fail for another reason, is a test failure.
func TestKit_EachBrokenProviderFailsExactlyItsCases(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		bug  string
		want []string
	}{
		{"state-filter-ignored", []string{"TICKETING-006"}},
		{"ignore-project", []string{"TICKETING-003", "TICKETING-004", "TICKETING-006", "TICKETING-013", "TICKETING-015", "TICKETING-016", "TICKETING-021"}},
		{"hidden-permission-denied", []string{"TICKETING-004", "TICKETING-013", "TICKETING-015", "TICKETING-016", "TICKETING-021"}},
		{"hidden-message", []string{"TICKETING-004"}},
		{"moved-message", []string{"TICKETING-004"}},
		{"key-in-message", []string{"TICKETING-002", "TICKETING-003", "TICKETING-004", "TICKETING-013", "TICKETING-015", "TICKETING-016", "TICKETING-021"}},
		{"tracker-text-in-message", []string{"TICKETING-004", "TICKETING-013", "TICKETING-015", "TICKETING-016", "TICKETING-021"}},
		{"forbidden-project-message", []string{"TICKETING-004"}},
		{"no-truncated-flag", []string{"TICKETING-005"}},
		{"split-character", []string{"TICKETING-005"}},
		{"no-strip", []string{"TICKETING-007", "TICKETING-010"}},
		{"keep-empty-label", []string{"TICKETING-007"}},
		{"drop-operation-ids", []string{"TICKETING-008"}},
		{"ignore-max-results", []string{"TICKETING-010"}},
		{"splice-query", []string{"TICKETING-011"}},
		{"search-lists-hidden", []string{"TICKETING-020"}},
		{"search-counts-hidden", []string{"TICKETING-020"}},
		{"comment-twice", []string{"TICKETING-012"}},
		{"comment-key-global", []string{"TICKETING-023"}},
		{"accept-empty-comment", []string{"TICKETING-013"}},
		{"accept-oversized-comment", []string{"TICKETING-013"}},
		{"comment-active-mention", []string{"TICKETING-024"}},
		{"dry-run-mutates", []string{"TICKETING-014"}},
		{"silent-unreachable", []string{"TICKETING-015"}},
		{"link-twice", []string{"TICKETING-016", "TICKETING-021"}},
		{"accept-self-link", []string{"TICKETING-016"}},
		{"link-ignores-blocked-project", []string{"TICKETING-004", "TICKETING-016", "TICKETING-021"}},
		{"no-evidence", []string{"TICKETING-017", "TICKETING-022"}},
		{"evidence-carries-body", []string{"TICKETING-022"}},
		{"write-splices-input", []string{"TICKETING-019"}},
		{"project-traversal", []string{"TICKETING-019"}},
		{"transition-ignores-project", []string{"TICKETING-004", "TICKETING-006", "TICKETING-015"}},
		{"comment-forbidden-allowed", []string{"TICKETING-004"}},
		{"comment-forbidden-denied", []string{"TICKETING-004"}},
		{"transition-forbidden-denied", []string{"TICKETING-004"}},
		{"dry-hidden", []string{"TICKETING-004"}},
		{"rejection-echoes-value", []string{"TICKETING-009", "TICKETING-010", "TICKETING-013", "TICKETING-015", "TICKETING-019"}},
		{"rejection-echoes-encoded", []string{"TICKETING-009", "TICKETING-010", "TICKETING-013", "TICKETING-015", "TICKETING-019"}},
		{"rejection-echoes-casefolded", []string{"TICKETING-009", "TICKETING-010", "TICKETING-013", "TICKETING-019"}},
		{"rejection-echoes-base64", []string{"TICKETING-009", "TICKETING-010", "TICKETING-013", "TICKETING-019"}},
		{"rejection-echoes-partial", []string{"TICKETING-009", "TICKETING-010", "TICKETING-013", "TICKETING-015", "TICKETING-019"}},
		{"dry-missing-message", []string{"TICKETING-004"}},
		{"dry-moved-message", []string{"TICKETING-004"}},
		{"dry-forbidden-oracle", []string{"TICKETING-004"}},
		{"dry-forbidden-message", []string{"TICKETING-004"}},
		{"search-forbidden-oracle", []string{"TICKETING-004"}},
		{"search-forbidden-message", []string{"TICKETING-004"}},
		{"search-missing-empty", []string{"TICKETING-004"}},
		{"conflict-echo", []string{"TICKETING-015"}},
		{"conflict-dry-differs", []string{"TICKETING-015"}},
		{"conflict-quoted", []string{"TICKETING-015"}},
		{"search-raw-title", []string{"TICKETING-007", "TICKETING-010"}},
		{"evidence-prefix", []string{"TICKETING-022"}},
		{"evidence-title-prefix", []string{"TICKETING-022"}},
		{"evidence-extra-field", []string{"TICKETING-022"}},
		{"evidence-wrong-key", []string{"TICKETING-022"}},
	} {
		t.Run(tc.bug, func(t *testing.T) {
			t.Parallel()
			if got := failedCases(t, tc.bug); !slices.Equal(got, tc.want) {
				t.Errorf("provider with bug %q fails %v, want exactly %v", tc.bug, got, tc.want)
			}
		})
	}
}

// noObserver hides the Observer of the fake: a Target that cannot show its outbound comment.
type noObserver struct{ f *fake }

func (n noObserver) GetIssue(ctx context.Context, o ticketingkit.Options, r *ticketingv1.GetIssueRequest) (*ticketingv1.GetIssueResponse, error) {
	return n.f.GetIssue(ctx, o, r)
}

func (n noObserver) SearchIssues(ctx context.Context, o ticketingkit.Options, r *ticketingv1.SearchIssuesRequest) (*ticketingv1.SearchIssuesResponse, error) {
	return n.f.SearchIssues(ctx, o, r)
}

func (n noObserver) CommentOnIssue(ctx context.Context, o ticketingkit.Options, r *ticketingv1.CommentOnIssueRequest) (*ticketingv1.CommentOnIssueResponse, []ticketingkit.Evidence, error) {
	return n.f.CommentOnIssue(ctx, o, r)
}

func (n noObserver) TransitionIssue(ctx context.Context, o ticketingkit.Options, r *ticketingv1.TransitionIssueRequest) (*ticketingv1.TransitionIssueResponse, []ticketingkit.Evidence, error) {
	return n.f.TransitionIssue(ctx, o, r)
}

func (n noObserver) LinkIssue(ctx context.Context, o ticketingkit.Options, r *ticketingv1.LinkIssueRequest) (*ticketingv1.LinkIssueResponse, []ticketingkit.Evidence, error) {
	return n.f.LinkIssue(ctx, o, r)
}

// TestKit_CommentMarkupNeedsAnObserver: without an Observer, TICKETING-024 is not applicable (Run skips it with the reason,
// it is never a pass); with the fake's Observer it runs, passes, and still fails for a provider that activates a mention.
func TestKit_CommentMarkupNeedsAnObserver(t *testing.T) {
	t.Parallel()
	var c24 ticketingkit.Case
	for _, c := range ticketingkit.Cases() {
		if c.ID == "TICKETING-024" {
			c24 = c
		}
	}
	if err := c24.Check(context.Background(), subject(noObserver{newFake(t, "")})); !errors.Is(err, ticketingkit.ErrNotApplicable) {
		t.Errorf("TICKETING-024 without an Observer = %v, want ErrNotApplicable (a skip, not a pass)", err)
	}
	if err := c24.Check(context.Background(), subject(newFake(t, ""))); err != nil {
		t.Errorf("TICKETING-024 with an Observer = %v, want a pass", err)
	}
	if err := c24.Check(context.Background(), subject(newFake(t, "comment-active-mention"))); err == nil || errors.Is(err, ticketingkit.ErrNotApplicable) {
		t.Errorf("TICKETING-024 against a provider that activates a mention = %v, want a failure", err)
	}
	// Run reports the case as skipped: the subtest of a Target without Observer is skipped.
	t.Run("run skips", func(t *testing.T) {
		t.Parallel()
		ticketingkit.Run(t, subject(noObserver{newFake(t, "")}))
	})
}

// TestKit_CombinedFaultsFailManyCases is the reviewers' scenario: one provider with four faults at once (it echoes the key
// in NOT_FOUND, answers a moved issue differently, rewrites blocked_project to project, and appends the comment body to its
// evidence id) must not conform.
func TestKit_CombinedFaultsFailManyCases(t *testing.T) {
	t.Parallel()
	got := failedCases(t, "key-in-message,moved-message,link-ignores-blocked-project,evidence-carries-body")
	if len(got) < 4 {
		t.Errorf("the combined provider fails %v, want at least four distinct cases", got)
	}
}

// TestKit_PortDescriptorMutations shows that TICKETING-018 reads the descriptor: each mutation of a copy fails it.
func TestKit_PortDescriptorMutations(t *testing.T) {
	t.Parallel()
	if err := ticketingkit.CheckPort(ticketingv1.File_numinance_ports_ticketing_v1_ticketing_proto); err != nil {
		t.Fatalf("CheckPort(real descriptor) = %v, want nil", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*descriptorpb.FileDescriptorProto)
	}{
		{"the resource mark removed from project", func(f *descriptorpb.FileDescriptorProto) { unmark(t, f, "GetIssueRequest", "project") }},
		{"the resource mark removed from blocked_project", func(f *descriptorpb.FileDescriptorProto) { unmark(t, f, "LinkIssueRequest", "blocked_project") }},
		{"the resource mark added to issue_key", func(f *descriptorpb.FileDescriptorProto) { mark(t, f, "CommentOnIssueRequest", "issue_key") }},
		{"the risk of LinkIssue lowered to READ", func(f *descriptorpb.FileDescriptorProto) {
			m := f.GetService()[0].GetMethod()[4]
			a := proto.GetExtension(m.GetOptions(), annotationsv1.E_Action).(*annotationsv1.ActionOptions)
			a.Risk = annotationsv1.RiskClass_RISK_CLASS_READ
			proto.SetExtension(m.GetOptions(), annotationsv1.E_Action, a)
		}},
		{"evidence removed from CommentOnIssue", func(f *descriptorpb.FileDescriptorProto) {
			m := f.GetService()[0].GetMethod()[2]
			a := proto.GetExtension(m.GetOptions(), annotationsv1.E_Action).(*annotationsv1.ActionOptions)
			a.Evidence = nil
			proto.SetExtension(m.GetOptions(), annotationsv1.E_Action, a)
		}},
	} {
		fdp := protodesc.ToFileDescriptorProto(ticketingv1.File_numinance_ports_ticketing_v1_ticketing_proto)
		fdp = proto.Clone(fdp).(*descriptorpb.FileDescriptorProto)
		tc.mutate(fdp)
		file, err := protodesc.NewFile(fdp, protoregistry.GlobalFiles)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if err := ticketingkit.CheckPort(file); err == nil {
			t.Errorf("CheckPort(%s) = nil, want an error", tc.name)
		}
	}
}

func field(t *testing.T, f *descriptorpb.FileDescriptorProto, message, name string) *descriptorpb.FieldDescriptorProto {
	t.Helper()
	for _, m := range f.GetMessageType() {
		if m.GetName() == message {
			for _, fl := range m.GetField() {
				if fl.GetName() == name {
					return fl
				}
			}
		}
	}
	t.Fatalf("no field %s.%s", message, name)
	return nil
}

func unmark(t *testing.T, f *descriptorpb.FileDescriptorProto, message, name string) {
	t.Helper()
	fl := field(t, f, message, name)
	fl.Options = &descriptorpb.FieldOptions{}
	proto.SetExtension(fl.Options, annotationsv1.E_Field, &annotationsv1.FieldAnnotations{Since: "1.2.0"})
}

func mark(t *testing.T, f *descriptorpb.FileDescriptorProto, message, name string) {
	t.Helper()
	fl := field(t, f, message, name)
	fl.Options = &descriptorpb.FieldOptions{}
	proto.SetExtension(fl.Options, annotationsv1.E_Field, &annotationsv1.FieldAnnotations{Since: "1.2.0", ResourceName: true})
}

// TestKit_CasesAreTheREADMETable keeps the case ids of the code and of the README table equal, in both directions.
func TestKit_CasesAreTheREADMETable(t *testing.T) {
	t.Parallel()
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	inCode := map[string]bool{}
	for _, c := range ticketingkit.Cases() {
		inCode[c.ID] = true
		if !strings.Contains(string(readme), "| "+c.ID+" |") {
			t.Errorf("case %s is not a row of the README table", c.ID)
		}
	}
	for _, m := range regexp.MustCompile(`(?m)^\| (TICKETING-\d{3}) \|`).FindAllStringSubmatch(string(readme), -1) {
		if !inCode[m[1]] {
			t.Errorf("the README has a row for %s, which is not a case of the kit", m[1])
		}
	}
}

// TestFixtures_OnlyReservedHostsAndTenants scans every file of the kit directory (ADR 0060 decision 10).
func TestFixtures_OnlyReservedHostsAndTenants(t *testing.T) {
	t.Parallel()
	problems, err := ticketingkit.ScanDir(os.DirFS("."))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

// TestFixtures_ScanRejectsDisallowedNames is the mutation test: each bypass of a deny-pattern fails the allowlist scan.
func TestFixtures_ScanRejectsDisallowedNames(t *testing.T) {
	t.Parallel()
	fixture, err := os.ReadFile("testdata/tracker.json")
	if err != nil {
		t.Fatal(err)
	}
	const name = "testdata/tracker.json"
	if got := ticketingkit.ScanFixtures(map[string][]byte{name: fixture}); len(got) != 0 {
		t.Fatalf("ScanFixtures(real fixture) = %v, want none", got)
	}
	mutate := func(from, to string) map[string][]byte {
		m := strings.Replace(string(fixture), from, to, 1)
		if m == string(fixture) {
			t.Fatalf("mutation %q did not apply", from)
		}
		return map[string][]byte{name: []byte(m)}
	}
	const host = `"host": "acme.example.test"`
	free := func(text string) map[string][]byte { return mutate("Fix the cart total", text) }
	for _, tc := range []struct {
		name  string
		files map[string][]byte
	}{
		{"a real vendor URL", mutate("https://acme.example.test", "https://acme.atlassian.net")},
		{"a real vendor host", mutate(host, `"host": "jira.realco.de"`)},
		{"an unusual top-level domain", mutate(host, `"host": "realco.atlassian.xyz"`)},
		{"an e-mail address as a value", mutate(host, `"host": "bob@realco.de"`)},
		{"an e-mail address in free text", mutate("Fix the cart total", "mail bob@realco.de")},
		{"a host in free text", mutate("Fix the cart total", "see realco.atlassian.xyz")},
		{"a URL in free text", mutate("Fix the cart total", "see https://realco.example.org/x")},
		{"a host that only ends like the reserved one", mutate(host, `"host": "acme.example.test.evil.com"`)},
		{"a lookalike host", mutate(host, `"host": "acme-example.test"`)},
		{"userinfo and a backslash", mutate(host, `"host": "https://evil.de\\@example.test"`)},
		{"a fullwidth dot", mutate(host, `"host": "https://example.test\u3002evil.de"`)},
		{"a fullwidth dot in free text", mutate("Fix the cart total", `see example.test\u3002evil.de`)},
		{"an escaped dot", mutate(host, `"host": "jira\u002erealco\u002ede"`)},
		{"a percent-escaped dot in free text", mutate("Fix the cart total", "see realco%2Ede")},
		{"a bare IPv6 literal", mutate(host, `"host": "fe80::1"`)},
		{"an IPv4 literal", mutate("https://acme.example.test", "http://10.0.0.5")},
		{"an IPv6 URL", mutate("https://acme.example.test", "http://[::1]")},
		{"a tenant name", mutate(`"tenant": "acme"`, `"tenant": "realco"`)},
		{"a tenant with leading white space", mutate(`"tenant": "acme"`, `"tenant": " realco"`)},
		{"an allowed tenant with trailing white space", mutate(`"tenant": "acme"`, `"tenant": "acme "`)},
		{"a tenant_id key", mutate(`"tenant": "acme"`, `"tenant_id": "realco"`)},
		{"an account_id key", mutate(`"tenant": "acme"`, `"account_id": "realco"`)},
		{"a cloudId key", mutate(`"tenant": "acme"`, `"cloudId": "realco"`)},
		{"an instance key", mutate(`"tenant": "acme"`, `"instance": "realco"`)},
		{"a site key", mutate(`"tenant": "acme"`, `"site": "realco"`)},
		{"an identity key holding a number", mutate(`"tenant": "acme"`, `"tenant": 12345`)},
		{"a host as a key", mutate(`"tenant": "acme"`, `"jira.realco.de": "acme"`)},
		{"a duplicate key", mutate(`"tenant": "acme"`, `"tenant": "acme", "tenant": "globex"`)},
		{"data after the document", map[string][]byte{name: append(append([]byte{}, fixture...), []byte("{}")...)}},
		{"a file that is not JSON", map[string][]byte{name: []byte("host: realco.de\n")}},
		{"a new file of another kind", map[string][]byte{name: fixture, "testdata/new.yaml": []byte("url: https://example.test\n")}},
		{"a Go file under testdata", map[string][]byte{name: fixture, "testdata/hosts.go": []byte("package x\n")}},
		{"a Go file under a nested testdata directory", map[string][]byte{name: fixture, "sub/testdata/deep/hosts.go": []byte("package x\n")}},
		{"a real host in a nested new subdirectory", map[string][]byte{name: fixture, "testdata/new/dir/more.json": []byte(`{"host": "jira.realco.de"}`)}},
		{"a fixture outside testdata", map[string][]byte{name: fixture, "more.json": []byte(`{}`)}},
		{"a real URL in the README", map[string][]byte{name: fixture, "README.md": []byte("see https://realco.atlassian.net/x\n")}},

		// Defanged and spaced hosts.
		{"a host defanged with square brackets", free("see acme[.]corp")},
		{"a host defanged with (dot)", free("see acme (dot) corp")},
		{"a host with a spaced dot", free("see acme . corp")},
		{"a host defanged with [dot]", free("see acme [dot] corp")},
		{"a host defanged with (.)", free("see acme(.)corp")},
		{"a host with the word dot", free("see acme dot corp")},
		{"a host with a dot after a space", free("see acme .corp")},
		{"an address defanged with [at]", free("mail bob[at]acme.corp")},
		{"a defanged host as an identity value", mutate(host, `"host": "acme[.]corp"`)},
		// Dot look-alikes.
		{"a small full stop U+FE52", free("see acme\ufe52corp")},
		{"a Syriac supralinear full stop U+0701", free("see acme\u0701corp")},
		{"a Syriac sublinear full stop U+0702", free("see acme\u0702corp")},
		{"a one dot leader U+2024", free("see acme\u2024corp")},
		{"a fullwidth full stop U+FF0E", free("see acme\uff0ecorp")},
		{"an ideographic full stop U+3002", free("see acme\u3002corp")},
		{"a halfwidth ideographic full stop U+FF61", free("see acme\uff61corp")},
		{"an Arabic full stop U+06D4", free("see acme\u06d4corp")},
		{"an Armenian full stop U+0589", free("see acme\u0589corp")},
		{"fullwidth letters in a host", free("see \uff41\uff43\uff4d\uff45.corp")},
		{"an internationalised host", free("see \u043f\u0440\u0438\u043c\u0435\u0440.\u0440\u0444")},
		// Filler and invisible characters, between the labels and anywhere in a string.
		{"a Hangul filler U+3164 after the dot", free("see acme.\u3164corp")},
		{"a halfwidth Hangul filler U+FFA0", free("see acme.\uffa0corp")},
		{"a Hangul choseong filler U+115F", free("see acme.\u115fcorp")},
		{"a Hangul jungseong filler U+1160", free("see acme.\u1160corp")},
		{"a Khmer inherent vowel U+17B4", free("see acme.\u17b4corp")},
		{"a combining grapheme joiner U+034F", free("see acme.\u034fcorp")},
		{"a braille blank U+2800", free("see acme.\u2800corp")},
		{"a variation selector U+FE0F", free("see acme.\ufe0fcorp")},
		{"a soft hyphen U+00AD", free("see acme.\u00adcorp")},
		{"an invisible plus U+2064", free("see acme.\u2064corp")},
		{"a filler alone in prose", free("Fix the\u3164 cart total")},
		{"a combining mark between the labels", free("see acme.\u0301corp")},
		// Encodings, repeated.
		{"a double percent-encoded dot", free("see acme%252ecorp")},
		{"a triple percent-encoded dot", free("see acme%25252ecorp")},
		{"an HTML entity inside an entity", free("see acme&amp;#46;corp")},
		{"a decimal HTML entity", free("see acme&#46;corp")},
		{"a hex HTML entity", free("see acme&#x2e;corp")},
		{"a named HTML entity", free("see acme&period;corp")},
		{"a backslash escape in the text", free(`see acme\\u002ecorp`)},
		{"a percent-encoded at sign", free("mail bob%40acme.corp")},
		// At-sign look-alikes.
		{"a small commercial at U+FE6B", free("mail bob\ufe6bacme.corp")},
		{"a fullwidth commercial at U+FF20", free("mail bob\uff20acme.corp")},
		{"a fullwidth at before an allowed host", free("mail bob\uff20example.test")},
		// Tenant and other names as structural tokens.
		{"a vendor name as a project", mutate(`"project": "SEC"`, `"project": "realco"`)},
		{"an allowed tenant name as a project", mutate(`"project": "SEC"`, `"project": "acme"`)},
		{"a vendor name as a key", mutate(`"key": "OPS-2"`, `"key": "REALCO-2"`)},
		{"an arbitrary alphanumeric as a project", mutate(`"project": "SEC"`, `"project": "x1y2"`)},
		{"an arbitrary alphanumeric as a state", mutate(`"state": "DONE"`, `"state": "realco"`)},
		{"a vendor name as a forbidden project", mutate(`"forbidden_projects": ["SEC"]`, `"forbidden_projects": ["realco"]`)},
		{"a vendor name as a reachable category", mutate(`"reachable": ["DONE"]`, `"reachable": ["realco"]`)},
		{"a key that is not in the closed set", mutate(`"note"`, `"realco"`)},
		// Numeric IP spellings.
		{"a hex IP in free text", free("see 0x7f000001")},
		{"a decimal IP in free text", free("see 2130706433")},
		{"an octal dotted IP in free text", free("see 0177.0.0.1")},
		{"an octal IP in free text", free("see 017700000001")},
		{"a hex dotted IP in free text", free("see 0x7f.0.0.1")},
		{"a short dotted IP in free text", free("see 127.1")},
		{"a decimal IP as the host", mutate(host, `"host": "2130706433"`)},
		{"a hex IP as the host", mutate(host, `"host": "0x7f000001"`)},
		{"a single-label internal host", mutate(`"tenant": "acme"`, `"server": "jira-internal"`)},
		{"a single-label internal endpoint", mutate(`"tenant": "acme"`, `"endpoint": "jira-internal"`)},
		{"an address key", mutate(`"tenant": "acme"`, `"address": "10.0.0.5"`)},
		{"an ip key", mutate(`"tenant": "acme"`, `"ip": "10.0.0.5"`)},
		{"an origin key", mutate(`"tenant": "acme"`, `"origin": "realco"`)},
		// Markdown.
		{"a bare host in the README", map[string][]byte{name: fixture, "README.md": []byte("see jira.realco.de for the tracker\n")}},
		{"a defanged host in the README", map[string][]byte{name: fixture, "README.md": []byte("see realco[.]de\n")}},
		{"a filler in a README host", map[string][]byte{name: fixture, "README.md": []byte("see realco.\u3164de\n")}},
		{"an e-mail address in the README", map[string][]byte{name: fixture, "README.md": []byte("write bob@realco.de\n")}},
		{"an IP in the README", map[string][]byte{name: fixture, "README.md": []byte("see 10.0.0.5\n")}},
		{"a hex IP in the README", map[string][]byte{name: fixture, "README.md": []byte("see 0x7f000001\n")}},
		{"a bare host in another markdown file", map[string][]byte{name: fixture, "docs/notes.md": []byte("see realco.de\n")}},
		{"a host in a markdown file under testdata", map[string][]byte{name: fixture, "testdata/notes.md": []byte("see realco.de\n")}},
	} {
		if got := ticketingkit.ScanFixtures(tc.files); len(got) == 0 {
			t.Errorf("ScanFixtures(%s) = none, want a problem", tc.name)
		}
	}
	// The scan decodes escapes before it checks: an allowed host spelled with JSON and percent escapes passes, and the
	// vendor host spelled only with escapes (no literal dot in the file) fails.
	escaped := mutate(host, `"host": "acme\u002eexample\u002etest"`)
	if got := ticketingkit.ScanFixtures(escaped); len(got) != 0 {
		t.Errorf("ScanFixtures(an allowed host spelled with JSON escapes) = %v, want none", got)
	}
	vendor := mutate(host, `"host": "jira\u002erealco\u002ede"`)
	if strings.Contains(string(vendor[name]), "jira.realco") || len(ticketingkit.ScanFixtures(vendor)) == 0 {
		t.Error("the escape-only vendor host must contain no literal dot and must fail the scan")
	}
	if got := ticketingkit.ScanFixtures(mutate("Fix the cart total", "see acme%2Eexample%2Etest")); len(got) != 0 {
		t.Errorf("ScanFixtures(an allowed host with percent escapes in free text) = %v, want none", got)
	}
	for _, ok := range []map[string][]byte{
		{"testdata/new/ok.json": []byte(`{"tenant": "globex", "base_url": "https://jira.acme.example.test/rest/api", "host": "example.test", "title": "Mail the team. Then ship it"}`)},
		{"README.md": []byte("see https://acme.example.test/x and kit.go, tracker.json and 1.2.0; e.g. the @name syntax. A sentence. Another.\n")},
		{"kit.go": []byte("package x // a comment with no host, v1.2 and e.g. a note\n")},
		{"kit.go": []byte("package x\n\nimport \"google.golang.org/protobuf/proto\"\n\ntype T struct {\n\tA string `json:\"a.b\"`\n}\n\nvar _ = proto.Marshal\nvar a = \"https://acme.example.test/x\"\nvar b = \"kit.go\"\nvar c = \"@alice\"\n")},
		{"kit_test.go": []byte("// ticketingkit:scan-exempt: mutation inputs\n\npackage x\n\nvar bad = \"jira.realco.de\"\n")},
	} {
		if got := ticketingkit.ScanFixtures(ok); len(got) != 0 {
			t.Errorf("ScanFixtures(%v) = %v, want none", ok, got)
		}
	}
}
