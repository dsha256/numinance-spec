package ticketingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
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

// secrets are the strings no NOT_FOUND message may contain: the project, the key and every tracker text of the subject.
func (s Subject) secrets(project, key string) []string {
	return append([]string{project, key}, s.TrackerTexts...)
}

// notFound checks that err is a NOT_FOUND whose message holds none of the secrets, and returns the message.
func notFound(err error, secrets ...string) (string, error) {
	if e := wantReason(err, pluginv1.Reason_REASON_NOT_FOUND); e != nil {
		return "", e
	}
	var f *CallError
	errors.As(err, &f)
	for _, sec := range secrets {
		if sec != "" && strings.Contains(f.Message, sec) {
			return "", fmt.Errorf("the NOT_FOUND message %q holds %q: it must hold neither the key, the project nor any tracker text", f.Message, sec)
		}
	}
	return f.Message, nil
}

// wantInvalid checks that err is INVALID_ARGUMENT and that its message does not name the offending value.
func wantInvalid(err error, offending string) error {
	if e := wantReason(err, pluginv1.Reason_REASON_INVALID_ARGUMENT); e != nil {
		return e
	}
	var f *CallError
	errors.As(err, &f)
	if offending != "" && strings.Contains(f.Message, offending) {
		return fmt.Errorf("the rejection %q names the offending value", f.Message)
	}
	return nil
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
	inv := func(what, offending string, err error) error {
		if e := wantInvalid(err, offending); e != nil {
			return fmt.Errorf("%s %q: %w", what, offending, e)
		}
		return nil
	}
	for _, p := range badProjects {
		for _, call := range []struct {
			what string
			err  func() error
		}{
			{"GetIssue project", func() error { _, err := get(ctx, s, p, s.Found, small); return err }},
			{"SearchIssues project", func() error {
				_, err := s.Target.SearchIssues(ctx, Options{}, &ticketingv1.SearchIssuesRequest{Project: p, MaxResults: 5})
				return err
			}},
			{"CommentOnIssue project", func() error {
				_, _, err := s.Target.CommentOnIssue(ctx, Options{Key: key}, &ticketingv1.CommentOnIssueRequest{Project: p, IssueKey: s.Found, Body: "x"})
				return err
			}},
			{"TransitionIssue project", func() error {
				_, _, err := s.Target.TransitionIssue(ctx, Options{Key: key}, &ticketingv1.TransitionIssueRequest{Project: p, IssueKey: s.Found, Target: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN})
				return err
			}},
			{"LinkIssue project", func() error {
				_, _, err := link2(ctx, s, p, s.LinkBlocker, s.Project, s.LinkBlocked)
				return err
			}},
			{"LinkIssue blocked_project", func() error {
				_, _, err := link2(ctx, s, s.Project, s.LinkBlocker, p, s.LinkBlocked)
				return err
			}},
		} {
			if e := inv(call.what, p, call.err()); e != nil {
				return e
			}
		}
	}
	for _, k := range badKeys {
		for _, call := range []struct {
			what string
			err  func() error
		}{
			{"GetIssue key", func() error { _, err := get(ctx, s, s.Project, k, small); return err }},
			{"CommentOnIssue key", func() error {
				_, _, err := s.Target.CommentOnIssue(ctx, Options{Key: key}, &ticketingv1.CommentOnIssueRequest{Project: s.Project, IssueKey: k, Body: "x"})
				return err
			}},
			{"TransitionIssue key", func() error {
				_, _, err := s.Target.TransitionIssue(ctx, Options{Key: key}, &ticketingv1.TransitionIssueRequest{Project: s.Project, IssueKey: k, Target: ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN})
				return err
			}},
			{"LinkIssue key", func() error { _, _, err := link2(ctx, s, s.Project, k, s.Project, s.LinkBlocked); return err }},
			{"LinkIssue blocked_issue_key", func() error { _, _, err := link2(ctx, s, s.Project, s.LinkBlocker, s.Project, k); return err }},
		} {
			if e := inv(call.what, k, call.err()); e != nil {
				return e
			}
		}
	}
	for _, text := range badTexts {
		_, err := search(ctx, s, text, 5)
		if e := inv("SearchIssues text", text, err); e != nil {
			return e
		}
	}
	_, err := s.Target.SearchIssues(ctx, Options{}, &ticketingv1.SearchIssuesRequest{Project: s.Project, State: unknown, MaxResults: 5})
	if e := inv("SearchIssues state", "99", err); e != nil {
		return e
	}
	_, _, err = s.Target.TransitionIssue(ctx, Options{Key: key}, &ticketingv1.TransitionIssueRequest{Project: s.Project, IssueKey: s.Found, Target: -1})
	if e := inv("TransitionIssue target", "-1", err); e != nil {
		return e
	}
	_, _, err = s.Target.LinkIssue(ctx, Options{Key: key}, &ticketingv1.LinkIssueRequest{
		Project: s.Project, IssueKey: s.LinkBlocker, BlockedProject: s.Project, BlockedIssueKey: s.LinkBlocked, LinkType: 99,
	})
	return inv("LinkIssue link_type", "99", err)
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

func t022(ctx context.Context, s Subject) error {
	const secretBody = "kit-secret-comment-body-7f3a91"
	var texts []string
	for _, key := range []string{s.Found, s.TransitionOpen, s.LinkBlocker, s.LinkBlocked} {
		r, err := get(ctx, s, s.Project, key, small)
		if err != nil {
			return err
		}
		texts = append(texts, r.GetTitle())
		if b := firstWords(r.GetBody()); b != "" {
			texts = append(texts, b)
		}
	}
	texts = append(texts, secretBody)
	check := func(what string, ev []Evidence, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		for _, e := range ev {
			for _, v := range append([]string{e.ID, e.Type}, e.Fields...) {
				for _, t := range texts {
					if t != "" && strings.Contains(v, t) {
						return fmt.Errorf("%s: an evidence record holds %q, a title, a body or a comment text: it may carry the verb, the project, the key and the result only", what, t)
					}
				}
			}
		}
		return nil
	}
	_, ev, err := s.Target.CommentOnIssue(ctx, Options{Key: "kit-ev-secret"}, &ticketingv1.CommentOnIssueRequest{Project: s.Project, IssueKey: s.Found, Body: secretBody})
	if e := check("CommentOnIssue", ev, err); e != nil {
		return e
	}
	_, ev, err = transition(ctx, s, Options{Key: "kit-ev-secret"}, s.TransitionOpen, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN)
	if e := check("TransitionIssue", ev, err); e != nil {
		return e
	}
	_, ev, err = link(ctx, s, "kit-ev-secret", s.LinkBlocker, s.LinkBlocked, ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS)
	return check("LinkIssue", ev, err)
}

// firstWords is the first 12 bytes of a body when they are a safe word to look for, else nothing.
func firstWords(body string) string {
	if len(body) < 12 {
		return ""
	}
	return body[:12]
}

func t023(ctx context.Context, s Subject) error {
	first, _, err := comment(ctx, s, "kit-reuse", "First issue.")
	if err != nil {
		return err
	}
	other, _, err := s.Target.CommentOnIssue(ctx, Options{Key: "kit-reuse"}, &ticketingv1.CommentOnIssueRequest{Project: s.Project, IssueKey: s.TransitionOpen, Body: "Another issue."})
	if err != nil {
		// A refusal of a key that names another operation is allowed; only a conflict is.
		return wantReason(err, pluginv1.Reason_REASON_CONFLICT)
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
