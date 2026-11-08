package ticketingkit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode"
	"unicode/utf8"

	ticketingkit "github.com/numinance/numinance-spec/conformance/ticketing/v1"
	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"
)

// The fake is the minimal in-memory provider the kit is proved against: a conforming one (no bug) and one broken in
// exactly one way for each name in bug (a comma-separated list combines faults). There is no real ticketing provider yet;
// the Jira provider card runs the same kit against its own build.

type fixtureIssue struct {
	Project, Key, Title, Body string
	BodyRepeat                *struct {
		Text  string
		Times int
	} `json:"body_repeat"`
	State        string
	Type         string
	Labels       []string
	OperationIDs []string `json:"operation_ids"`
	Hidden       bool
	Reachable    []string
}

type fixtureFile struct {
	Issues            []fixtureIssue
	ForbiddenProjects []string `json:"forbidden_projects"`
}

const notFoundMessage = "not found"

type fake struct {
	mu        sync.Mutex
	bugs      []string
	issues    []*fixtureIssue
	forbidden []string
	state     map[string]ticketingv1.IssueStateCategory
	comments  map[string]string
	links     map[string]bool
	next      int
	last      ticketingkit.Outbound
}

func newFake(tb testing.TB, bug string) *fake {
	tb.Helper()
	raw, err := os.ReadFile("testdata/tracker.json")
	if err != nil {
		tb.Fatal(err)
	}
	var f fixtureFile
	if err := json.Unmarshal(raw, &f); err != nil {
		tb.Fatal(err)
	}
	p := &fake{forbidden: f.ForbiddenProjects, state: map[string]ticketingv1.IssueStateCategory{}, comments: map[string]string{}, links: map[string]bool{}}
	if bug != "" {
		p.bugs = strings.Split(bug, ",")
	}
	for i := range f.Issues {
		is := &f.Issues[i]
		if is.BodyRepeat != nil {
			is.Body = strings.Repeat(is.BodyRepeat.Text, is.BodyRepeat.Times)
		}
		p.issues = append(p.issues, is)
		p.state[is.Key] = category(is.State)
	}
	return p
}

func (p *fake) has(bug string) bool { return slices.Contains(p.bugs, bug) }

func category(s string) ticketingv1.IssueStateCategory {
	return ticketingv1.IssueStateCategory(ticketingv1.IssueStateCategory_value["ISSUE_STATE_CATEGORY_"+s])
}

func fail(reason pluginv1.Reason, msg string) error {
	return &ticketingkit.CallError{Reason: reason, Message: msg}
}

// invalid is the one INVALID_ARGUMENT of the fake. The conforming fake never names the offending value.
func (p *fake) invalid(vals ...string) error {
	if p.has("rejection-echoes-value") {
		return fail(pluginv1.Reason_REASON_INVALID_ARGUMENT, "invalid request: "+strings.Join(vals, " "))
	}
	return fail(pluginv1.Reason_REASON_INVALID_ARGUMENT, "invalid request")
}

// validProject is the project grammar of the proto: one or two segments, each a token, no empty, dot or dot-dot segment.
func validProject(s string) bool {
	segs := strings.Split(s, "/")
	if len(segs) > 2 || strings.Contains(s, "..") {
		return false
	}
	for _, seg := range segs {
		if !matchGrammar(seg, 100, "._-") {
			return false
		}
	}
	return true
}

// vp is validProject, except for the fake that accepts any path-like project.
func (p *fake) vp(s string) bool {
	if p.has("project-traversal") {
		return matchGrammar(s, 100, "._/-")
	}
	return validProject(s)
}

func validKey(s string) bool { return matchGrammar(s, 64, "._-") }

func matchGrammar(s string, limit int, extra string) bool {
	if s == "" || len(s) > limit {
		return false
	}
	for i, r := range s {
		alnum := r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r))
		if !alnum && (i == 0 || !strings.ContainsRune(extra, r)) {
			return false
		}
	}
	return true
}

func isStripped(r rune) bool {
	return unicode.Is(unicode.Cc, r) || r == 0x061C || r == 0x200E || r == 0x200F || r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069 ||
		r >= 0x200B && r <= 0x200D || r == 0x2060 || r == 0xFEFF
}

func (p *fake) strip(s string, layout bool) string {
	if p.has("no-strip") {
		return s
	}
	return strings.Map(func(r rune) rune {
		if (r == '\n' || r == '\t') && layout {
			return r
		}
		if isStripped(r) {
			return -1
		}
		return r
	}, s)
}

func (p *fake) validText(s string) bool {
	return len(s) <= 256 && utf8.ValidString(s) && !strings.ContainsFunc(s, isStripped)
}

func (p *fake) isForbidden(project string) bool { return slices.Contains(p.forbidden, project) }

// find returns the issue the credential may see in that project, or the one NOT_FOUND answer for every other case.
func (p *fake) find(verb, project, key string) (*fixtureIssue, error) {
	msg := notFoundMessage
	if p.has("key-in-message") {
		msg = "issue " + key + " not found"
	}
	var found *fixtureIssue
	for _, is := range p.issues {
		if is.Key == key {
			found = is
		}
	}
	switch {
	case found == nil:
		return nil, fail(pluginv1.Reason_REASON_NOT_FOUND, msg)
	case found.Hidden || p.isForbidden(found.Project):
		forbiddenProject := p.isForbidden(found.Project)
		switch {
		case p.has("comment-forbidden-allowed") && verb == "comment" && forbiddenProject:
			return found, nil
		case p.has("comment-forbidden-denied") && verb == "comment" && forbiddenProject,
			p.has("transition-forbidden-denied") && strings.HasPrefix(verb, "transition") && forbiddenProject,
			p.has("dry-hidden") && verb == "transition-dry" && found.Hidden:
			return nil, fail(pluginv1.Reason_REASON_PERMISSION_DENIED, "permission denied")
		case p.has("hidden-permission-denied"):
			return nil, fail(pluginv1.Reason_REASON_PERMISSION_DENIED, "permission denied")
		case p.has("hidden-message"):
			return nil, fail(pluginv1.Reason_REASON_NOT_FOUND, "not visible to this credential")
		case p.has("tracker-text-in-message"):
			return nil, fail(pluginv1.Reason_REASON_NOT_FOUND, found.Title)
		case p.has("forbidden-project-message") && p.isForbidden(found.Project):
			return nil, fail(pluginv1.Reason_REASON_NOT_FOUND, "forbidden project")
		}
		return nil, fail(pluginv1.Reason_REASON_NOT_FOUND, msg)
	case found.Project != project && !p.has("ignore-project"):
		if p.has("moved-message") {
			return nil, fail(pluginv1.Reason_REASON_NOT_FOUND, "moved")
		}
		return nil, fail(pluginv1.Reason_REASON_NOT_FOUND, msg)
	}
	return found, nil
}

func (p *fake) GetIssue(_ context.Context, _ ticketingkit.Options, r *ticketingv1.GetIssueRequest) (*ticketingv1.GetIssueResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.vp(r.GetProject()) || !validKey(r.GetIssueKey()) || r.GetMaxBodyBytes() < 1 || r.GetMaxBodyBytes() > 65536 {
		return nil, p.invalid(r.GetProject(), r.GetIssueKey())
	}
	is, err := p.find("get", r.GetProject(), r.GetIssueKey())
	if err != nil {
		return nil, err
	}
	body := p.strip(is.Body, true)
	truncated := false
	if len(body) > int(r.GetMaxBodyBytes()) {
		body, truncated = body[:r.GetMaxBodyBytes()], !p.has("no-truncated-flag")
		for !p.has("split-character") && !utf8.ValidString(body) {
			body = body[:len(body)-1]
		}
	}
	ops := is.OperationIDs
	if p.has("drop-operation-ids") {
		ops = nil
	}
	resp := &ticketingv1.GetIssueResponse{
		IssueKey: is.Key, Project: is.Project, Title: p.strip(is.Title, false), Body: body, Truncated: truncated,
		State: p.state[is.Key], IssueType: p.strip(is.Type, false),
	}
	for _, l := range is.Labels {
		if v := p.strip(l, false); v != "" || p.has("keep-empty-label") {
			resp.Labels = append(resp.Labels, v)
		}
	}
	for _, o := range ops {
		if v := p.strip(o, false); v != "" {
			resp.OperationIds = append(resp.OperationIds, v)
		}
	}
	return resp, nil
}

func (p *fake) SearchIssues(_ context.Context, _ ticketingkit.Options, r *ticketingv1.SearchIssuesRequest) (*ticketingv1.SearchIssuesResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	limit := int(r.GetMaxResults())
	st := r.GetState()
	bad := !p.vp(r.GetProject()) || !p.validText(r.GetText()) || st < 0 || st > 4 || (!p.has("ignore-max-results") && (limit < 1 || limit > 50))
	if bad {
		return nil, p.invalid(r.GetProject(), r.GetText())
	}
	if p.has("ignore-max-results") {
		limit = 1000
	}
	text := strings.ToLower(r.GetText())
	spliced := p.has("splice-query") && strings.Contains(text, " or ")
	var hits []*fixtureIssue
	for _, is := range p.issues {
		match := strings.Contains(strings.ToLower(is.Title+"\n"+is.Body), text) || spliced
		if !match || p.isForbidden(is.Project) || (is.Project != r.GetProject() && !spliced) {
			continue
		}
		if st != ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED && p.state[is.Key] != st && !p.has("state-filter-ignored") {
			continue
		}
		if is.Hidden && !p.has("search-lists-hidden") && !p.has("search-counts-hidden") {
			continue
		}
		hits = append(hits, is)
	}
	resp := &ticketingv1.SearchIssuesResponse{Truncated: len(hits) > limit}
	for _, is := range hits[:min(limit, len(hits))] {
		if is.Hidden && p.has("search-counts-hidden") {
			continue
		}
		resp.Issues = append(resp.Issues, &ticketingv1.IssueSummary{IssueKey: is.Key, Title: p.strip(is.Title, false), State: p.state[is.Key]})
	}
	return resp, nil
}

// evidence is the record of a write: the verb, the project, the key and the result, and never a title or a body.
func (p *fake) evidence(o ticketingkit.Options, verb, project, key, result, leak string) []ticketingkit.Evidence {
	if o.DryRun || p.has("no-evidence") {
		return nil
	}
	p.next++
	id := fmt.Sprintf("ev_%d", p.next)
	if p.has("evidence-carries-body") {
		id += ":" + leak
	}
	return []ticketingkit.Evidence{{Type: ticketingkit.EvidenceType, ID: id, Fields: []string{"verb=" + verb, "project=" + project, "key=" + key, "result=" + result}}}
}

func (p *fake) CommentOnIssue(_ context.Context, o ticketingkit.Options, r *ticketingv1.CommentOnIssueRequest) (*ticketingv1.CommentOnIssueResponse, []ticketingkit.Evidence, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	badBody := (r.GetBody() == "" && !p.has("accept-empty-comment")) || (len(r.GetBody()) > 8192 && !p.has("accept-oversized-comment"))
	if (!p.has("write-splices-input") && (!p.vp(r.GetProject()) || !validKey(r.GetIssueKey()))) || badBody {
		return nil, nil, p.invalid(r.GetProject(), r.GetIssueKey(), r.GetBody())
	}
	is, err := p.find("comment", r.GetProject(), r.GetIssueKey())
	if err != nil {
		return nil, nil, err
	}
	idem := is.Key + "\x00" + o.Key
	if p.has("comment-key-global") {
		idem = o.Key
	}
	id, existing := p.comments[idem]
	if !existing || p.has("comment-twice") {
		p.next++
		id = fmt.Sprintf("c_%d", p.next)
		p.comments[idem] = id
		existing = false
		p.last = ticketingkit.Outbound{Text: r.GetBody()}
		if p.has("comment-active-mention") {
			for _, w := range strings.Fields(r.GetBody()) {
				if strings.HasPrefix(w, "@") || strings.HasPrefix(w, "[~") {
					p.last.Mentions = append(p.last.Mentions, w)
				}
			}
		}
	}
	return &ticketingv1.CommentOnIssueResponse{CommentId: id, Existing: existing}, p.evidence(o, "comment", is.Project, is.Key, id, r.GetBody()), nil
}

// LastComment implements ticketingkit.Observer.
func (p *fake) LastComment() ticketingkit.Outbound {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last
}

func (p *fake) TransitionIssue(_ context.Context, o ticketingkit.Options, r *ticketingv1.TransitionIssueRequest) (*ticketingv1.TransitionIssueResponse, []ticketingkit.Evidence, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if (!p.has("write-splices-input") && (!p.vp(r.GetProject()) || !validKey(r.GetIssueKey()))) || r.GetTarget() < 1 || r.GetTarget() > 4 {
		return nil, nil, p.invalid(r.GetProject(), r.GetIssueKey())
	}
	verb, project := "transition", r.GetProject()
	if o.DryRun {
		verb = "transition-dry"
	}
	if p.has("transition-ignores-project") {
		for _, is := range p.issues {
			if is.Key == r.GetIssueKey() {
				project = is.Project
			}
		}
	}
	is, err := p.find(verb, project, r.GetIssueKey())
	if err != nil {
		return nil, nil, err
	}
	prev := p.state[is.Key]
	resp := &ticketingv1.TransitionIssueResponse{Previous: prev, State: r.GetTarget(), Changed: prev != r.GetTarget()}
	result := r.GetTarget().String()
	if !resp.Changed {
		return resp, p.evidence(o, "transition", is.Project, is.Key, result, is.Title), nil
	}
	reachable := false
	for _, c := range is.Reachable {
		reachable = reachable || category(c) == r.GetTarget()
	}
	if !reachable && !p.has("silent-unreachable") {
		return nil, nil, fail(pluginv1.Reason_REASON_CONFLICT, "no transition reaches the category")
	}
	if !o.DryRun || p.has("dry-run-mutates") {
		p.state[is.Key] = r.GetTarget()
	}
	return resp, p.evidence(o, "transition", is.Project, is.Key, result, is.Title), nil
}

func (p *fake) LinkIssue(_ context.Context, o ticketingkit.Options, r *ticketingv1.LinkIssueRequest) (*ticketingv1.LinkIssueResponse, []ticketingkit.Evidence, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	self := r.GetProject() == r.GetBlockedProject() && r.GetIssueKey() == r.GetBlockedIssueKey() && !p.has("accept-self-link")
	if (!p.has("write-splices-input") && (!p.vp(r.GetProject()) || !p.vp(r.GetBlockedProject()) || !validKey(r.GetIssueKey()) || !validKey(r.GetBlockedIssueKey()))) ||
		r.GetLinkType() != ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS || self {
		return nil, nil, p.invalid(r.GetProject(), r.GetBlockedProject(), r.GetIssueKey(), r.GetBlockedIssueKey())
	}
	blocker, err := p.find("link", r.GetProject(), r.GetIssueKey())
	if err != nil {
		return nil, nil, err
	}
	blockedProject := r.GetBlockedProject()
	if p.has("link-ignores-blocked-project") {
		blockedProject = r.GetProject()
	}
	blocked, err := p.find("link", blockedProject, r.GetBlockedIssueKey())
	if err != nil {
		return nil, nil, err
	}
	k := blocker.Key + ">" + blocked.Key
	existing := p.links[k] && !p.has("link-twice")
	p.links[k] = true
	return &ticketingv1.LinkIssueResponse{Existing: existing}, p.evidence(o, "link", blocker.Project, blocker.Key, blocked.Key, blocker.Title), nil
}
