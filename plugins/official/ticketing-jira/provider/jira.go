package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/plugin/v1"
	ticketingv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/ports/ticketing/v1"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// Every verb that names an issue first fetches that issue with ONE request, whatever the issue is, and decides afterwards: a
// missing issue, one the credential may not see, one that moved and one in another project all cost the same request and take
// the same path to the same NOT_FOUND. A provider that answered earlier for one of them would be an oracle for the existence of
// issues by timing (ADR 0060, owed card 4 checks it end to end). LinkIssue fetches both of its issues before it looks at either.

// The fields each fetch asks for. Query values are built with url.Values, so none of them can add a parameter.
func readQuery() url.Values {
	return url.Values{
		"fields":     {"summary,description,status,issuetype,labels,project,resolution"},
		"properties": {operationIDsProperty},
	}
}

func transitionQuery() url.Values {
	return url.Values{"fields": {"project,status,resolution"}, "expand": {"transitions"}}
}

func linkQuery() url.Values { return url.Values{"fields": {"project,issuelinks"}} }

func commentQuery(property string) url.Values {
	return url.Values{"fields": {"project"}, "properties": {property}}
}

type nameField struct {
	Name string `json:"name"`
}

type categoryField struct {
	Key string `json:"key"`
}

type statusField struct {
	Name           string         `json:"name"`
	StatusCategory *categoryField `json:"statusCategory"`
}

type issueLink struct {
	Type         nameField `json:"type"`
	OutwardIssue *struct {
		Key string `json:"key"`
	} `json:"outwardIssue"`
}

type transitionDecl struct {
	ID          string `json:"id"`
	HasScreen   bool   `json:"hasScreen"`
	IsAvailable *bool  `json:"isAvailable"`
	To          *struct {
		Name           string         `json:"name"`
		StatusCategory *categoryField `json:"statusCategory"`
	} `json:"to"`
}

// issue is the part of Jira's answer this provider reads. Every field is untrusted.
type issue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Status      *statusField    `json:"status"`
		IssueType   *nameField      `json:"issuetype"`
		Labels      []string        `json:"labels"`
		Project     *struct {
			Key string `json:"key"`
		} `json:"project"`
		Resolution *nameField  `json:"resolution"`
		IssueLinks []issueLink `json:"issuelinks"`
	} `json:"fields"`
	Properties  map[string]json.RawMessage `json:"properties"`
	Transitions []transitionDecl           `json:"transitions"`
}

// in reports whether the answer is the issue asked for, now in the project asked for. A key the tracker renamed or an issue
// that moved is not.
func (i *issue) in(project, key string) bool {
	return i != nil && i.Key == key && i.Fields.Project != nil && i.Fields.Project.Key == project
}

// canceledNames are status and resolution names that mean "finished without being done". Jira has one "done" status category
// for both; the port keeps them apart, so the provider reads the names. A name outside the list is DONE.
var canceledNames = //nolint:gochecknoglobals // read-only table
[]string{"canceled", "cancelled", "won't do", "wont do", "won't fix", "wontfix", "declined", "rejected", "duplicate", "obsolete", "abandoned"}

func canceled(name string) bool {
	return slices.Contains(canceledNames, strings.ToLower(strings.TrimSpace(name)))
}

// category maps a status category key (and for the done category, the names) to the port's closed category.
func category(key, status, resolution string) (ticketingv1.IssueStateCategory, bool) {
	switch key {
	case "new":
		return ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN, true
	case "indeterminate":
		return ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_IN_PROGRESS, true
	case "done":
		if canceled(status) || canceled(resolution) {
			return ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED, true
		}
		return ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE, true
	}
	return ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED, false
}

func (i *issue) state() (ticketingv1.IssueStateCategory, bool) {
	s := i.Fields.Status
	if s == nil || s.StatusCategory == nil {
		return ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED, false
	}
	res := ""
	if i.Fields.Resolution != nil {
		res = i.Fields.Resolution.Name
	}
	return category(s.StatusCategory.Key, s.Name, res)
}

func categoryName(s ticketingv1.IssueStateCategory) string {
	return strings.TrimPrefix(s.String(), "ISSUE_STATE_CATEGORY_")
}

// jira is the provider's view of one instance.
type jira struct {
	host string
}

// client returns the REST client of a call. A live call asks the host for the credential handle of the manifest's intent first;
// a host that cannot give one ends the call as unavailable and nothing is sent. The handle is the only thing that authenticates:
// the egress proxy strips it and injects the credential the broker holds. A replay asks for none.
func (p *jira) client(ctx context.Context, c *ghprovider.Call) (client, error) {
	gc, err := c.Client(ctx)
	if err != nil {
		return client{}, fail(pluginv1.Reason_REASON_UPSTREAM_UNAVAILABLE, msgUnavailable)
	}
	return client{rt: gc.Transport, host: p.host, handle: gc.Handle}, nil
}

func (p *jira) fetch(ctx context.Context, cl client, key string, q url.Values) (*issue, error) {
	var is issue
	if err := cl.do(ctx, http.MethodGet, []string{"issue", key}, q, nil, &is); err != nil {
		return nil, failure(ctx, err, true)
	}
	return &is, nil
}

// receipt submits the compact ticketing-action record: the verb, the project, the issue key and the result, never a title, a
// body or a comment text. A submission that fails leaves the call PARTIAL.
func receipt(ctx context.Context, c *ghprovider.Call, verb, project, key, result string) {
	_ = c.Emit(ctx, "action", "ticketing-action", &structpb.Struct{Fields: map[string]*structpb.Value{
		"verb":    structpb.NewStringValue(verb),
		"project": structpb.NewStringValue(project),
		"key":     structpb.NewStringValue(key),
		"result":  structpb.NewStringValue(result),
	}})
}

func (p *jira) getIssue(ctx context.Context, c *ghprovider.Call, m *ticketingv1.GetIssueRequest) (*ticketingv1.GetIssueResponse, error) {
	switch {
	case !validProject(m.GetProject()):
		return nil, invalid(msgProject)
	case !validIssueKey(m.GetIssueKey()):
		return nil, invalid(msgIssueKey)
	case m.GetMaxBodyBytes() < 1 || m.GetMaxBodyBytes() > maxBodyBound:
		return nil, invalid(msgBodyBound)
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	is, err := p.fetch(ctx, cl, m.GetIssueKey(), readQuery())
	if err != nil {
		return nil, err
	}
	if !is.in(m.GetProject(), m.GetIssueKey()) {
		return nil, notFound()
	}
	return render(is, m.GetProject(), int(m.GetMaxBodyBytes()))
}

// render turns the tracker's issue into the port's answer, stripped and bounded. It never repairs a field the port cannot hold:
// an empty or oversized title, an unknown status category or a description it cannot render ends the call UPSTREAM_ERROR.
func render(is *issue, project string, maxBody int) (*ticketingv1.GetIssueResponse, error) {
	bad := fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
	title := sanitize(is.Fields.Summary, false)
	if title == "" || len(title) > 1024 {
		return nil, bad
	}
	state, ok := is.state()
	if !ok {
		return nil, bad
	}
	text, err := renderDescription(is.Fields.Description)
	if err != nil {
		return nil, bad
	}
	body, truncated := cutAt(sanitize(text, true), maxBody)
	out := &ticketingv1.GetIssueResponse{
		IssueKey: is.Key, Project: project, Title: title, Body: body, Truncated: truncated, State: state,
	}
	if is.Fields.IssueType != nil {
		out.IssueType, _ = cutAt(sanitize(is.Fields.IssueType.Name, false), 128)
	}
	for _, l := range is.Fields.Labels {
		if l = sanitize(l, false); l != "" && len(l) <= 128 && len(out.Labels) < 64 {
			out.Labels = append(out.Labels, l)
		}
	}
	out.OperationIds = operationIDs(is.Properties[operationIDsProperty])
	return out, nil
}

// operationIDs reads the property that records operation ids. Anyone who can edit the issue can write it, so it is read
// leniently (a value that is not a list of strings is no ids) and bounded; the Engine accepts only the id it computed itself.
func operationIDs(raw json.RawMessage) []string {
	var ids []string
	if len(raw) == 0 || json.Unmarshal(raw, &ids) != nil {
		return nil
	}
	var out []string
	for _, id := range ids {
		if id = sanitize(id, false); id != "" && len(id) <= 128 && len(out) < 64 {
			out = append(out, id)
		}
	}
	return out
}

// jqlSpecial are the characters that Jira's text search reads as syntax; each is escaped, so the text is matched as plain
// text and never as a wildcard, a fuzzy match, a phrase, a boolean or a field query.
const jqlSpecial = `+-&|!(){}[]^~*?\:/`

// operatorWords are the words the text search reads as boolean operators when they stand alone in capitals.
var operatorWords = //nolint:gochecknoglobals // read-only table
[]string{"AND", "OR", "NOT"}

// escapeText returns text ready for the inside of a JQL string literal in a text search. A special character gets a backslash
// for the text search (written as two in the literal). A quote is escaped for both layers, so it neither ends the literal nor
// opens a phrase the search cannot parse. The words AND, OR and NOT are operators only in capitals and the search ignores case,
// so each is written in lower case: still matched as a word, never read as an operator.
func escapeText(t string) string {
	var b, word strings.Builder
	flush := func() {
		w := word.String()
		if slices.Contains(operatorWords, w) {
			w = strings.ToLower(w)
		}
		b.WriteString(w)
		word.Reset()
	}
	for _, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			word.WriteRune(r)
			continue
		}
		flush()
		switch {
		case r == '"':
			b.WriteString(`\\\"`)
		case strings.ContainsRune(jqlSpecial, r):
			b.WriteString(`\\`)
			if r == '\\' {
				b.WriteString(`\\`)
			} else {
				b.WriteRune(r)
			}
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return b.String()
}

// JQL builds the query of a search from the validated project, the text and the state category. The project is a quoted
// literal of the project grammar (no quote or backslash can occur in it); the text is escaped by escapeText. It is exported so
// the replay cassette and its tests name the same query.
func JQL(project, text string, state ticketingv1.IssueStateCategory) string {
	q := `project = "` + project + `"`
	if text != "" {
		q += ` AND text ~ "` + escapeText(text) + `"`
	}
	switch state {
	case ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_OPEN:
		q += ` AND statusCategory = "To Do"`
	case ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_IN_PROGRESS:
		q += ` AND statusCategory = "In Progress"`
	case ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_DONE, ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_CANCELED:
		q += ` AND statusCategory = "Done"`
	}
	return q + ` ORDER BY key ASC`
}

func (p *jira) searchIssues(ctx context.Context, c *ghprovider.Call, m *ticketingv1.SearchIssuesRequest) (*ticketingv1.SearchIssuesResponse, error) {
	switch {
	case !validProject(m.GetProject()):
		return nil, invalid(msgProject)
	case !validSearchText(m.GetText()):
		return nil, invalid(msgText)
	case m.GetState() != ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED && !knownState(m.GetState()):
		return nil, invalid(msgState)
	case m.GetMaxResults() < 1 || m.GetMaxResults() > maxResults:
		return nil, invalid(msgResultsBound)
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	req := map[string]any{
		"jql": JQL(m.GetProject(), m.GetText(), m.GetState()), "maxResults": m.GetMaxResults(),
		"fields": []string{"summary", "status", "project", "resolution"},
	}
	var ans struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary    string       `json:"summary"`
				Status     *statusField `json:"status"`
				Resolution *nameField   `json:"resolution"`
				Project    *struct {
					Key string `json:"key"`
				} `json:"project"`
			} `json:"fields"`
		} `json:"issues"`
		IsLast        *bool  `json:"isLast"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := cl.do(ctx, http.MethodPost, []string{"search", "jql"}, nil, req, &ans); err != nil {
		// A project the credential may not use and a project that does not exist both make Jira refuse the query (400): one
		// answer, the one of a missing issue, so a search is no oracle for the existence of a project.
		var ae *apiError
		if errors.As(err, &ae) && (ae.status == http.StatusBadRequest || ae.status == http.StatusForbidden) {
			return nil, notFound()
		}
		return nil, failure(ctx, err, true)
	}
	out := &ticketingv1.SearchIssuesResponse{}
	// The next page is never fetched, and a pagination token or link of the answer is never read as a URL: the answer says there
	// is more, and the caller narrows its search.
	out.Truncated = ans.NextPageToken != "" || (ans.IsLast != nil && !*ans.IsLast)
	if len(ans.Issues) > int(m.GetMaxResults()) {
		return nil, fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
	}
	for _, h := range ans.Issues {
		f := h.Fields
		if !validIssueKey(h.Key) || f.Project == nil {
			return nil, fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
		}
		if f.Project.Key != m.GetProject() {
			continue
		}
		is := &issue{Key: h.Key}
		is.Fields.Status, is.Fields.Resolution = f.Status, f.Resolution
		st, ok := is.state()
		title := sanitize(f.Summary, false)
		if !ok || title == "" || len(title) > 1024 {
			return nil, fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
		}
		if m.GetState() != ticketingv1.IssueStateCategory_ISSUE_STATE_CATEGORY_UNSPECIFIED && st != m.GetState() {
			continue
		}
		out.Issues = append(out.Issues, &ticketingv1.IssueSummary{IssueKey: h.Key, Title: title, State: st})
	}
	return out, nil
}

func (p *jira) commentOnIssue(ctx context.Context, c *ghprovider.Call, m *ticketingv1.CommentOnIssueRequest) (*ticketingv1.CommentOnIssueResponse, error) {
	key := c.IdempotencyKey()
	switch {
	case !validProject(m.GetProject()):
		return nil, invalid(msgProject)
	case !validIssueKey(m.GetIssueKey()):
		return nil, invalid(msgIssueKey)
	case !validCommentBody(m.GetBody()):
		return nil, invalid(msgComment)
	case key == "":
		return nil, invalid(msgKey)
	case c.DryRun():
		return nil, invalid(msgDryRun)
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	// The host's key names the comment. An issue property, set after the comment is added, records which comment the key made:
	// the same fetch that checks the issue reads it, so a retry finds the first comment instead of adding another. (The window
	// between the two writes is the known limit of this approach, README.)
	prop := CommentProperty(key)
	is, err := p.fetch(ctx, cl, m.GetIssueKey(), commentQuery(prop))
	if err != nil {
		return nil, err
	}
	if !is.in(m.GetProject(), m.GetIssueKey()) {
		return nil, notFound()
	}
	if raw, ok := is.Properties[prop]; ok {
		var rec struct {
			CommentID string `json:"commentId"`
		}
		if json.Unmarshal(raw, &rec) != nil || !idRe.MatchString(rec.CommentID) {
			return nil, fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
		}
		// The property is written by anyone who can edit the issue, so it only names a comment: the call answers Existing when
		// that comment is ours (our credential's account wrote it) and says what was asked. Anything else fails closed, with
		// nothing posted: no false claim of a comment, and no second comment either.
		if err := p.recorded(ctx, cl, m.GetIssueKey(), rec.CommentID, m.GetBody()); err != nil {
			return nil, err
		}
		receipt(ctx, c, "comment", m.GetProject(), m.GetIssueKey(), rec.CommentID)
		return &ticketingv1.CommentOnIssueResponse{CommentId: rec.CommentID, Existing: true}, nil
	}
	var made struct {
		ID string `json:"id"`
	}
	if err := cl.do(ctx, http.MethodPost, []string{"issue", m.GetIssueKey(), "comment"}, nil, map[string]any{"body": commentDocument(m.GetBody())}, &made); err != nil {
		return nil, failure(ctx, err, false)
	}
	if !idRe.MatchString(made.ID) {
		return nil, fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
	}
	c.Wrote(pluginv1.SideEffect_SIDE_EFFECT_WRITE, m.GetProject())
	receipt(ctx, c, "comment", m.GetProject(), m.GetIssueKey(), made.ID)
	if err := cl.do(ctx, http.MethodPut, []string{"issue", m.GetIssueKey(), "properties", prop}, nil, map[string]string{"commentId": made.ID}, nil); err != nil {
		return nil, failure(ctx, err, false)
	}
	return &ticketingv1.CommentOnIssueResponse{CommentId: made.ID}, nil
}

// recorded checks that the comment an issue property names is the one this call would make: it exists on the issue, its author is
// the account of the credential in use and its text is the text asked for, plain paragraphs only. The tracker's answer is
// untrusted: the check reads two fields and compares; nothing else of the comment is used.
func (p *jira) recorded(ctx context.Context, cl client, issueKey, commentID, body string) error {
	var found struct {
		ID     string `json:"id"`
		Author *struct {
			AccountID string `json:"accountId"`
		} `json:"author"`
		Body json.RawMessage `json:"body"`
	}
	if err := cl.do(ctx, http.MethodGet, []string{"issue", issueKey, "comment", commentID}, nil, nil, &found); err != nil {
		var ae *apiError
		if errors.As(err, &ae) && ae.status == http.StatusNotFound {
			return fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
		}
		return failure(ctx, err, false)
	}
	var me struct {
		AccountID string `json:"accountId"`
	}
	if err := cl.do(ctx, http.MethodGet, []string{"myself"}, nil, nil, &me); err != nil {
		return failure(ctx, err, false)
	}
	text, plain := plainParagraphs(found.Body)
	if found.ID != commentID || found.Author == nil || me.AccountID == "" || found.Author.AccountID != me.AccountID || !plain || text != body {
		return fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
	}
	return nil
}

func (p *jira) transitionIssue(ctx context.Context, c *ghprovider.Call, m *ticketingv1.TransitionIssueRequest) (*ticketingv1.TransitionIssueResponse, error) {
	switch {
	case !validProject(m.GetProject()):
		return nil, invalid(msgProject)
	case !validIssueKey(m.GetIssueKey()):
		return nil, invalid(msgIssueKey)
	case !knownState(m.GetTarget()):
		return nil, invalid(msgTarget)
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	is, err := p.fetch(ctx, cl, m.GetIssueKey(), transitionQuery())
	if err != nil {
		return nil, err
	}
	if !is.in(m.GetProject(), m.GetIssueKey()) {
		return nil, notFound()
	}
	before, ok := is.state()
	if !ok {
		return nil, fail(pluginv1.Reason_REASON_UPSTREAM_ERROR, msgUpstream)
	}
	if before == m.GetTarget() {
		if !c.DryRun() {
			receipt(ctx, c, "transition", m.GetProject(), m.GetIssueKey(), categoryName(before))
		}
		return &ticketingv1.TransitionIssueResponse{Previous: before, State: before}, nil
	}
	id, ok := pick(is.Transitions, m.GetTarget())
	if !ok {
		return nil, fail(pluginv1.Reason_REASON_CONFLICT, msgNoTrans)
	}
	resp := &ticketingv1.TransitionIssueResponse{Previous: before, State: m.GetTarget(), Changed: true}
	if c.DryRun() {
		return resp, nil
	}
	body := map[string]any{"transition": map[string]string{"id": id}}
	if err := cl.do(ctx, http.MethodPost, []string{"issue", m.GetIssueKey(), "transitions"}, nil, body, nil); err != nil {
		return nil, refused(ctx, err)
	}
	c.Wrote(pluginv1.SideEffect_SIDE_EFFECT_WRITE, m.GetProject())
	receipt(ctx, c, "transition", m.GetProject(), m.GetIssueKey(), categoryName(before)+">"+categoryName(m.GetTarget()))
	return resp, nil
}

// pick chooses the transition that reaches the target category: the available one with no screen (a transition that asks for
// fields cannot be made without inventing values) and the lowest numeric id, so the choice is the same every time.
func pick(ts []transitionDecl, target ticketingv1.IssueStateCategory) (string, bool) {
	best, found := "", false
	for _, t := range ts {
		if t.To == nil || t.To.StatusCategory == nil || t.HasScreen || (t.IsAvailable != nil && !*t.IsAvailable) || !idRe.MatchString(t.ID) {
			continue
		}
		if st, ok := category(t.To.StatusCategory.Key, t.To.Name, ""); !ok || st != target {
			continue
		}
		if !found || lessID(t.ID, best) {
			best, found = t.ID, true
		}
	}
	return best, found
}

func lessID(a, b string) bool {
	x, ea := strconv.ParseUint(a, 10, 64)
	y, eb := strconv.ParseUint(b, 10, 64)
	if ea == nil && eb == nil && x != y {
		return x < y
	}
	return a < b
}

// linkTypeBlocks is the name of the link type Jira ships for "blocks / is blocked by".
const linkTypeBlocks = "Blocks"

func (p *jira) linkIssue(ctx context.Context, c *ghprovider.Call, m *ticketingv1.LinkIssueRequest) (*ticketingv1.LinkIssueResponse, error) {
	switch {
	case !validProject(m.GetProject()):
		return nil, invalid(msgProject)
	case !validIssueKey(m.GetIssueKey()):
		return nil, invalid(msgIssueKey)
	case !validProject(m.GetBlockedProject()):
		return nil, invalid(msgBlockedProject)
	case !validIssueKey(m.GetBlockedIssueKey()):
		return nil, invalid(msgBlockedKey)
	case m.GetLinkType() != ticketingv1.IssueLinkType_ISSUE_LINK_TYPE_BLOCKS:
		return nil, invalid(msgLinkType)
	case m.GetProject() == m.GetBlockedProject() && m.GetIssueKey() == m.GetBlockedIssueKey():
		return nil, invalid(msgSelfLink)
	}
	cl, err := p.client(ctx, c)
	if err != nil {
		return nil, err
	}
	// Both issues are fetched before either is judged: the answer for a blocker the credential cannot see costs the same two
	// requests as any other.
	blocker, errA := p.fetch(ctx, cl, m.GetIssueKey(), linkQuery())
	blocked, errB := p.fetch(ctx, cl, m.GetBlockedIssueKey(), linkQuery())
	for _, e := range []error{errA, errB} {
		if e != nil {
			return nil, e
		}
	}
	if !blocker.in(m.GetProject(), m.GetIssueKey()) || !blocked.in(m.GetBlockedProject(), m.GetBlockedIssueKey()) {
		return nil, notFound()
	}
	for _, l := range blocker.Fields.IssueLinks {
		if l.Type.Name == linkTypeBlocks && l.OutwardIssue != nil && l.OutwardIssue.Key == m.GetBlockedIssueKey() {
			receipt(ctx, c, "link", m.GetProject(), m.GetIssueKey(), m.GetBlockedIssueKey())
			return &ticketingv1.LinkIssueResponse{Existing: true}, nil
		}
	}
	// In Jira's request the outward issue is the one that blocks and the inward issue the one that is blocked by it.
	body := map[string]any{
		"type":         map[string]string{"name": linkTypeBlocks},
		"outwardIssue": map[string]string{"key": m.GetIssueKey()},
		"inwardIssue":  map[string]string{"key": m.GetBlockedIssueKey()},
	}
	if err := cl.do(ctx, http.MethodPost, []string{"issueLink"}, nil, body, nil); err != nil {
		return nil, refused(ctx, err)
	}
	c.Wrote(pluginv1.SideEffect_SIDE_EFFECT_WRITE, m.GetProject())
	receipt(ctx, c, "link", m.GetProject(), m.GetIssueKey(), m.GetBlockedIssueKey())
	return &ticketingv1.LinkIssueResponse{}, nil
}
