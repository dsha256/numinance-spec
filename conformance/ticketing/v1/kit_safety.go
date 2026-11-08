package ticketingkit

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"
)

// Outbound is what a provider asked the tracker to store for its last comment.
type Outbound struct {
	// Text is the plain text the tracker will show as the comment.
	Text string
	// Mentions are the users, groups or notifications the payload would activate (a Jira account mention, an @name that
	// the tracker resolves).
	Mentions []string
}

// Observer is implemented by a Target that can show the outbound payload of its last CommentOnIssue (a replay transport
// can record it). TICKETING-024 runs only for a Target that implements it; the Jira provider card must implement it.
type Observer interface {
	LastComment() Outbound
}

// secrets are the strings no error message may contain: the project, the key and every tracker text of the subject.
func (s Subject) secrets(project, key string) []string {
	return append([]string{project, key}, s.TrackerTexts...)
}

// missingProject is a well-formed project that does not exist.
func (s Subject) missingProject() string {
	if s.MissingProject != "" {
		return s.MissingProject
	}
	return "kitnoproject"
}

type probe struct {
	name, project, key string
	err                error
}

// notFoundProbes makes every call that the port answers with the one NOT_FOUND: a missing, a hidden, a moved, a
// wrong-project and a forbidden-project issue, on every verb that takes an issue.
func (s Subject) notFoundProbes(ctx context.Context) []probe {
	var out []probe
	add := func(name, project, key string, err error) { out = append(out, probe{name, project, key, err}) }
	getErr := func(project, key string) error { _, err := get(ctx, s, project, key, small); return err }
	commentErr := func(project, key string) error {
		_, _, err := s.Target.CommentOnIssue(ctx, Options{Key: "kit-nf"}, &ticketingv1.CommentOnIssueRequest{Project: project, IssueKey: key, Body: "x"})
		return err
	}
	transErr := func(project, key string) error {
		_, _, err := s.Target.TransitionIssue(ctx, Options{Key: "kit-nf"}, &ticketingv1.TransitionIssueRequest{Project: project, IssueKey: key, Target: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE})
		return err
	}
	linkErr := func(blockerProject, blocker, blockedProject, blocked string) error {
		_, _, err := s.Target.LinkIssue(ctx, Options{Key: "kit-nf"}, &ticketingv1.LinkIssueRequest{
			Project: blockerProject, IssueKey: blocker, BlockedProject: blockedProject, BlockedIssueKey: blocked, LinkType: ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS,
		})
		return err
	}
	add("GetIssue of a missing issue", s.Project, s.Missing, getErr(s.Project, s.Missing))
	add("GetIssue of a hidden issue", s.Project, s.Hidden, getErr(s.Project, s.Hidden))
	add("GetIssue of a moved issue under its old project", s.Project, s.Moved, getErr(s.Project, s.Moved))
	add("GetIssue of an issue under another project", s.OtherProject, s.Found, getErr(s.OtherProject, s.Found))
	add("GetIssue in a forbidden project", s.ForbiddenProject, s.ForbiddenIssue, getErr(s.ForbiddenProject, s.ForbiddenIssue))
	add("CommentOnIssue of a hidden issue", s.Project, s.Hidden, commentErr(s.Project, s.Hidden))
	add("CommentOnIssue of a moved issue", s.Project, s.Moved, commentErr(s.Project, s.Moved))
	add("CommentOnIssue of a missing issue", s.Project, s.Missing, commentErr(s.Project, s.Missing))
	add("TransitionIssue of a hidden issue", s.Project, s.Hidden, transErr(s.Project, s.Hidden))
	add("TransitionIssue of a moved issue", s.Project, s.Moved, transErr(s.Project, s.Moved))
	add("TransitionIssue of a missing issue", s.Project, s.Missing, transErr(s.Project, s.Missing))
	dryTransErr := func(project, key string) error {
		_, _, err := s.Target.TransitionIssue(ctx, Options{DryRun: true, Key: "kit-nf"}, &ticketingv1.TransitionIssueRequest{Project: project, IssueKey: key, Target: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE})
		return err
	}
	add("CommentOnIssue of an issue under another project", s.OtherProject, s.Found, commentErr(s.OtherProject, s.Found))
	add("CommentOnIssue in a forbidden project", s.ForbiddenProject, s.ForbiddenIssue, commentErr(s.ForbiddenProject, s.ForbiddenIssue))
	add("TransitionIssue of an issue under another project", s.OtherProject, s.Found, transErr(s.OtherProject, s.Found))
	add("TransitionIssue in a forbidden project", s.ForbiddenProject, s.ForbiddenIssue, transErr(s.ForbiddenProject, s.ForbiddenIssue))
	searchErr := func(project string) error {
		_, err := s.Target.SearchIssues(ctx, Options{}, &ticketingv1.SearchIssuesRequest{Project: project, Text: s.SearchText, MaxResults: 5})
		return err
	}
	// A search is no oracle for the existence of a project: a project the credential may not use answers exactly as a
	// project that does not exist.
	add("SearchIssues in a forbidden project", s.ForbiddenProject, "", searchErr(s.ForbiddenProject))
	add("SearchIssues in a project that does not exist", s.missingProject(), "", searchErr(s.missingProject()))
	// The dry run of a verb is probed on the same five cases as the call: missing, hidden, moved, wrong project, forbidden.
	add("dry-run TransitionIssue of a missing issue", s.Project, s.Missing, dryTransErr(s.Project, s.Missing))
	add("dry-run TransitionIssue of a moved issue", s.Project, s.Moved, dryTransErr(s.Project, s.Moved))
	add("dry-run TransitionIssue of a hidden issue", s.Project, s.Hidden, dryTransErr(s.Project, s.Hidden))
	add("dry-run TransitionIssue of an issue under another project", s.OtherProject, s.Found, dryTransErr(s.OtherProject, s.Found))
	add("dry-run TransitionIssue in a forbidden project", s.ForbiddenProject, s.ForbiddenIssue, dryTransErr(s.ForbiddenProject, s.ForbiddenIssue))
	add("LinkIssue with a missing blocked issue", s.Project, s.Missing, linkErr(s.Project, s.LinkBlocker, s.Project, s.Missing))
	add("LinkIssue with a hidden blocker", s.Project, s.Hidden, linkErr(s.Project, s.Hidden, s.Project, s.LinkBlocked))
	add("LinkIssue with a blocked issue in a forbidden project", s.ForbiddenProject, s.ForbiddenIssue, linkErr(s.Project, s.LinkBlocker, s.ForbiddenProject, s.ForbiddenIssue))
	add("LinkIssue with a blocker in a forbidden project", s.ForbiddenProject, s.ForbiddenIssue, linkErr(s.ForbiddenProject, s.ForbiddenIssue, s.Project, s.LinkBlocked))
	add("LinkIssue with a blocked key that is not in blocked_project", s.OtherProject, s.LinkBlocked, linkErr(s.Project, s.LinkBlocker, s.OtherProject, s.LinkBlocked))
	return out
}

func t019(ctx context.Context, s Subject) error {
	const key = "kit-bad"
	badProjects := []string{"", "../x", "a\x00b", "a\nb", "a b", "-a", "a\u202eb", "\uff41b", strings.Repeat("a", 101),
		"a/../b", "a/./b", "a//b", "a/", "PAY/..", "/a", "a/b/c", "..", "a/.."}
	badKeys := []string{"", "../x", "PAY-1\x00", "PAY\n1", "PAY 1", "PAY#1", "-1", "PAY\u200b1", "\uff21-1", strings.Repeat("a", 65)}
	badTexts := []string{"a\x00b", "a\nb", "a\u202eb", "a\u200bb", strings.Repeat("a", 257)}
	const unknown = ticketingv1.IssueStateCategory(99)
	// The rejection of each bad value is checked against the secrets of the valid request and, for a value with letters or
	// digits, against the rejection of its twin.
	secrets := s.secrets(s.Project, s.Found)
	inv := func(what, offending string, call func(string) error) error {
		if e := wantInvalidTwin(call, offending, secrets...); e != nil {
			return fmt.Errorf("%s %q: %w", what, offending, e)
		}
		return nil
	}
	for _, c := range []struct {
		what string
		call func(p string) error
	}{
		{"GetIssue project", func(p string) error { _, err := get(ctx, s, p, s.Found, small); return err }},
		{"SearchIssues project", func(p string) error {
			_, err := s.Target.SearchIssues(ctx, Options{}, &ticketingv1.SearchIssuesRequest{Project: p, MaxResults: 5})
			return err
		}},
		{"CommentOnIssue project", func(p string) error {
			_, _, err := s.Target.CommentOnIssue(ctx, Options{Key: key}, &ticketingv1.CommentOnIssueRequest{Project: p, IssueKey: s.Found, Body: "x"})
			return err
		}},
		{"TransitionIssue project", func(p string) error {
			_, _, err := s.Target.TransitionIssue(ctx, Options{Key: key}, &ticketingv1.TransitionIssueRequest{Project: p, IssueKey: s.Found, Target: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN})
			return err
		}},
		{"LinkIssue project", func(p string) error {
			_, _, err := link2(ctx, s, p, s.LinkBlocker, s.Project, s.LinkBlocked)
			return err
		}},
		{"LinkIssue blocked_project", func(p string) error {
			_, _, err := link2(ctx, s, s.Project, s.LinkBlocker, p, s.LinkBlocked)
			return err
		}},
	} {
		for _, p := range badProjects {
			if e := inv(c.what, p, c.call); e != nil {
				return e
			}
		}
	}
	for _, c := range []struct {
		what string
		call func(k string) error
	}{
		{"GetIssue key", func(k string) error { _, err := get(ctx, s, s.Project, k, small); return err }},
		{"CommentOnIssue key", func(k string) error {
			_, _, err := s.Target.CommentOnIssue(ctx, Options{Key: key}, &ticketingv1.CommentOnIssueRequest{Project: s.Project, IssueKey: k, Body: "x"})
			return err
		}},
		{"TransitionIssue key", func(k string) error {
			_, _, err := s.Target.TransitionIssue(ctx, Options{Key: key}, &ticketingv1.TransitionIssueRequest{Project: s.Project, IssueKey: k, Target: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN})
			return err
		}},
		{"LinkIssue key", func(k string) error { _, _, err := link2(ctx, s, s.Project, k, s.Project, s.LinkBlocked); return err }},
		{"LinkIssue blocked_issue_key", func(k string) error { _, _, err := link2(ctx, s, s.Project, s.LinkBlocker, s.Project, k); return err }},
	} {
		for _, k := range badKeys {
			if e := inv(c.what, k, c.call); e != nil {
				return e
			}
		}
	}
	for _, text := range badTexts {
		if e := inv("SearchIssues text", text, func(v string) error { _, err := search(ctx, s, v, 5); return err }); e != nil {
			return e
		}
	}
	_, err := s.Target.SearchIssues(ctx, Options{}, &ticketingv1.SearchIssuesRequest{Project: s.Project, State: unknown, MaxResults: 5})
	if e := wantInvalid(err, "99", secrets...); e != nil {
		return fmt.Errorf("SearchIssues state: %w", e)
	}
	_, _, err = s.Target.TransitionIssue(ctx, Options{Key: key}, &ticketingv1.TransitionIssueRequest{Project: s.Project, IssueKey: s.Found, Target: -1})
	if e := wantInvalid(err, "-1", secrets...); e != nil {
		return fmt.Errorf("TransitionIssue target: %w", e)
	}
	_, _, err = s.Target.LinkIssue(ctx, Options{Key: key}, &ticketingv1.LinkIssueRequest{
		Project: s.Project, IssueKey: s.LinkBlocker, BlockedProject: s.Project, BlockedIssueKey: s.LinkBlocked, LinkType: 99,
	})
	if e := wantInvalid(err, "99", secrets...); e != nil {
		return fmt.Errorf("LinkIssue link_type: %w", e)
	}
	return nil
}

func link2(ctx context.Context, s Subject, blockerProject, blocker, blockedProject, blocked string) (*ticketingv1.LinkIssueResponse, []Evidence, error) {
	return s.Target.LinkIssue(ctx, Options{Key: "kit-bad"}, &ticketingv1.LinkIssueRequest{
		Project: blockerProject, IssueKey: blocker, BlockedProject: blockedProject, BlockedIssueKey: blocked, LinkType: ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS,
	})
}

func t020(ctx context.Context, s Subject) error {
	all, err := search(ctx, s, s.SearchText, 50)
	if err != nil {
		return err
	}
	for _, h := range all.GetIssues() {
		if h.GetIssueKey() == s.HiddenMatch || (s.HiddenMatchTitle != "" && strings.Contains(h.GetTitle(), s.HiddenMatchTitle)) {
			return fmt.Errorf("a search listed %s, an issue the credential cannot see", h.GetIssueKey())
		}
	}
	// A provider that counts hidden matches would say there is more when the visible ones are all returned.
	exact, err := search(ctx, s, s.SearchText, size(len(all.GetIssues())))
	if err != nil {
		return err
	}
	if exact.GetTruncated() || len(exact.GetIssues()) != len(all.GetIssues()) {
		return fmt.Errorf("max_results equal to the %d visible matches answered %d hits, truncated=%v, want them all and not truncated: a hidden issue was counted",
			len(all.GetIssues()), len(exact.GetIssues()), exact.GetTruncated())
	}
	if s.HiddenMatchTitle != "" {
		byTitle, err := search(ctx, s, s.HiddenMatchTitle, 50)
		if err != nil {
			return err
		}
		if len(byTitle.GetIssues()) != 0 {
			return fmt.Errorf("a search for %q, a word that only a hidden issue holds, listed %d issues", s.HiddenMatchTitle, len(byTitle.GetIssues()))
		}
	}
	return nil
}

func t021(ctx context.Context, s Subject) error {
	first, _, err := link2(ctx, s, s.Project, s.LinkBlocker, s.OtherProject, s.CrossBlocked)
	if err != nil {
		return fmt.Errorf("a link into %s: %w", s.OtherProject, err)
	}
	again, _, err := link2(ctx, s, s.Project, s.LinkBlocker, s.OtherProject, s.CrossBlocked)
	if err != nil {
		return err
	}
	if first.GetExisting() || !again.GetExisting() {
		return fmt.Errorf("a cross-project link answered existing = %v then %v, want false then true", first.GetExisting(), again.GetExisting())
	}
	for _, c := range []struct {
		name                                                  string
		blockerProject, blocker, blockedProject, blockedIssue string
	}{
		{"a blocked issue in a forbidden project", s.Project, s.LinkBlocker, s.ForbiddenProject, s.ForbiddenIssue},
		{"a blocker in a forbidden project", s.ForbiddenProject, s.ForbiddenIssue, s.Project, s.LinkBlocked},
		{"a blocked key that is not in blocked_project", s.Project, s.LinkBlocker, s.OtherProject, s.LinkBlocked},
		{"a blocker key that is not in project", s.OtherProject, s.LinkBlocker, s.Project, s.LinkBlocked},
		{"a blocked issue that moved", s.Project, s.LinkBlocker, s.Project, s.Moved},
	} {
		_, _, err := link2(ctx, s, c.blockerProject, c.blocker, c.blockedProject, c.blockedIssue)
		if _, e := notFound(err, s.secrets(c.blockedProject, c.blockedIssue)...); e != nil {
			return fmt.Errorf("%s: %w", c.name, e)
		}
	}
	return nil
}

// evidenceTokenRe is the form of an evidence record id: one opaque token.
var evidenceTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const categoryNames = `(?:OPEN|IN_PROGRESS|DONE|CANCELED)`

var transitionResultRe = regexp.MustCompile(`^` + categoryNames + `(?:>` + categoryNames + `)?$`)

// alnum is the lower-cased letters and digits of s: the form in which a slice of a text is compared with an id, so that a
// space, a hyphen or an underscore does not hide a prefix.
func alnum(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

// leakedSlice returns a run of four or more letters or digits of v that is also a run of one of the texts.
func leakedSlice(v string, texts []string) (string, bool) {
	runes := []rune(alnum(v))
	for _, t := range texts {
		folded := alnum(t)
		for i := 0; i+4 <= len(runes); i++ {
			if w := string(runes[i : i+4]); strings.Contains(folded, w) {
				return w, true
			}
		}
	}
	return "", false
}

// evidenceWant is what one write call must have recorded.
type evidenceWant struct {
	verb, project, key string
	// result says whether a result value is in the closed set of the verb.
	result func(string) bool
}

// checkEvidence applies the allowlist of the ticketing-action record: exactly one record of the type, an id that is one
// opaque token holding no slice of a known text, and exactly the fields verb, project, key and result, each once, the
// first three equal to the request and the result in the closed set of the verb. Nothing else is allowed in any field.
func checkEvidence(what string, ev []Evidence, err error, w evidenceWant, texts []string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if len(ev) != 1 {
		return fmt.Errorf("%s emitted %d records, want exactly one %s", what, len(ev), EvidenceType)
	}
	e := ev[0]
	if e.Type != EvidenceType {
		return fmt.Errorf("%s: the record type is %q, want %q", what, e.Type, EvidenceType)
	}
	if !evidenceTokenRe.MatchString(e.ID) {
		return fmt.Errorf("%s: the record id %q is not one opaque token of letters, digits, underscore and hyphen (1 to 64 bytes): it may carry no other text", what, e.ID)
	}
	if t, ok := leakedSlice(e.ID, texts); ok {
		return fmt.Errorf("%s: the record id %q holds %q, a slice of a title, a body or a comment text", what, e.ID, t)
	}
	// The only fields of a ticketing-action record, each exactly once, as name=value.
	evidenceFieldNames := []string{"verb", "project", "key", "result"}
	got := map[string]string{}
	for _, f := range e.Fields {
		name, value, ok := strings.Cut(f, "=")
		if _, dup := got[name]; !ok || dup || !slices.Contains(evidenceFieldNames, name) {
			return fmt.Errorf("%s: the evidence field %q is not one of %v as name=value, each once: a record carries the verb, the project, the key and the result only", what, f, evidenceFieldNames)
		}
		got[name] = value
	}
	if len(got) != len(evidenceFieldNames) {
		return fmt.Errorf("%s: the evidence fields are %q, want exactly %v", what, e.Fields, evidenceFieldNames)
	}
	if got["verb"] != w.verb || got["project"] != w.project || got["key"] != w.key {
		return fmt.Errorf("%s: the evidence says verb %q, project %q, key %q, want %q, %q, %q", what, got["verb"], got["project"], got["key"], w.verb, w.project, w.key)
	}
	if !w.result(got["result"]) {
		return fmt.Errorf("%s: the evidence result %q is not in the closed set of the verb (the comment id; a category or before>after; the blocked key)", what, got["result"])
	}
	return nil
}

func t022(ctx context.Context, s Subject) error {
	const secretBody = "kit-secret-comment-body-qzxj"
	texts := append([]string{secretBody}, s.TrackerTexts...)
	for _, key := range []string{s.Found, s.TransitionOpen, s.LinkBlocker, s.LinkBlocked} {
		r, err := get(ctx, s, s.Project, key, small)
		if err != nil {
			return err
		}
		texts = append(texts, r.GetTitle(), r.GetBody())
	}
	// Each write is made twice: the first call makes the change, the second finds it made; both record the same shape.
	for round := range 2 {
		c, ev, err := s.Target.CommentOnIssue(ctx, Options{Key: "kit-ev-secret"}, &ticketingv1.CommentOnIssueRequest{Project: s.Project, IssueKey: s.Found, Body: secretBody})
		if err == nil {
			err = checkEvidence(fmt.Sprintf("CommentOnIssue (call %d)", round+1), ev, nil,
				evidenceWant{"comment", s.Project, s.Found, func(r string) bool { return r == c.GetCommentId() && evidenceTokenRe.MatchString(r) }}, texts)
		} else {
			err = fmt.Errorf("CommentOnIssue: %w", err)
		}
		if err != nil {
			return err
		}
		_, ev, err = transition(ctx, s, Options{Key: "kit-ev-secret"}, s.TransitionOpen, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN)
		if e := checkEvidence(fmt.Sprintf("TransitionIssue (call %d)", round+1), ev, err,
			evidenceWant{"transition", s.Project, s.TransitionOpen, transitionResultRe.MatchString}, texts); e != nil {
			return e
		}
		_, ev, err = link(ctx, s, "kit-ev-secret", s.LinkBlocker, s.LinkBlocked, ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS)
		if e := checkEvidence(fmt.Sprintf("LinkIssue (call %d)", round+1), ev, err,
			evidenceWant{"link", s.Project, s.LinkBlocker, func(r string) bool { return r == s.LinkBlocked }}, texts); e != nil {
			return e
		}
	}
	return nil
}

func t023(ctx context.Context, s Subject) error {
	first, _, err := comment(ctx, s, "kit-reuse", "First issue.")
	if err != nil {
		return err
	}
	other, _, err := s.Target.CommentOnIssue(ctx, Options{Key: "kit-reuse"}, &ticketingv1.CommentOnIssueRequest{Project: s.Project, IssueKey: s.TransitionOpen, Body: "Another issue."})
	if err != nil {
		// A refusal of a key that names another operation is allowed; only a conflict is.
		_, e := conflict(err, s.secrets(s.Project, s.TransitionOpen)...)
		return e
	}
	if other.GetCommentId() == first.GetCommentId() || other.GetExisting() {
		return fmt.Errorf("a key reused for %s answered comment %q (existing %v), the comment of %s: a comment was not posted on the issue named",
			s.TransitionOpen, other.GetCommentId(), other.GetExisting(), s.Found)
	}
	return nil
}

func t024(ctx context.Context, s Subject) error {
	obs, ok := s.Target.(Observer)
	if !ok {
		return fmt.Errorf("%w: the Target does not implement Observer, so the outbound comment cannot be seen", ErrNotApplicable)
	}
	const body = "[~accountid:5b10ac8d82e05b22cc7d4ef5] @alice {code}x{code} *bold* <b>y</b> [link|about:blank] #123"
	if _, _, err := s.Target.CommentOnIssue(ctx, Options{Key: "kit-markup"}, &ticketingv1.CommentOnIssueRequest{Project: s.Project, IssueKey: s.Found, Body: body}); err != nil {
		return err
	}
	out := obs.LastComment()
	if out.Text != body || len(out.Mentions) != 0 {
		return fmt.Errorf("the outbound comment is %q with mentions %q, want the text unchanged and no active mention: the body is plain text", out.Text, out.Mentions)
	}
	return nil
}
