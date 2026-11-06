package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

// PullRequestScenario is one pull request opening, as GitHub documents the answers to its three requests: the head branch's ref,
// the lookup of an open pull request for the head and base, and the creation. Every field is matched exactly: the cassette built
// from it answers a request for this repository, base branch, head branch, head revision, title and body and no other, so a wrong
// request has no recording and fails closed. Nothing here is a wildcard.
type PullRequestScenario struct {
	// Repository is "owner/name".
	Repository string
	// BaseBranch and HeadBranch are the branch names; Head is the 40 hex commit the head branch points at.
	BaseBranch, HeadBranch, Head string
	// Title and Body are the text the opener sends.
	Title, Body string
	// Number is the number the host gives the pull request.
	Number uint64
}

var headRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// PullRequestCassette returns the cassette (JSON) of s. It is synthetic, never a recording, and says so in its redaction field. A
// value that the recorder's redaction would change (a credential pattern, an e-mail address) is refused, so a scenario built from a
// real environment cannot write personal data or a token into a file.
func PullRequestCassette(s PullRequestScenario) ([]byte, error) {
	owner, name, ok := strings.Cut(s.Repository, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return nil, errors.New("replay: the repository is owner/name")
	}
	if s.BaseBranch == "" || s.HeadBranch == "" || s.Title == "" || !headRe.MatchString(s.Head) || s.Number < 1 {
		return nil, errors.New("replay: a scenario needs both branches, a title, a full lowercase commit id and a number")
	}
	clean := ghprovider.Redactor{}
	for _, v := range []string{s.Repository, s.BaseBranch, s.HeadBranch, s.Head, s.Title, s.Body} {
		if clean.Scrub(v) != v {
			return nil, errors.New("replay: a scenario value looks like a credential or an e-mail address and is not written to a cassette")
		}
	}
	repo := "/repos/" + ghprovider.Seg(owner) + "/" + ghprovider.Seg(name)
	segs := strings.Split(s.HeadBranch, "/")
	for i, seg := range segs {
		segs[i] = ghprovider.Seg(seg)
	}
	lookup := repo + "/pulls" + ghprovider.Query("state", "open", "head", owner+":"+s.HeadBranch, "base", s.BaseBranch)
	pull := map[string]any{
		"number": s.Number, "state": "open", "merged": false, "merged_at": nil, "merge_commit_sha": nil,
		"html_url":  "https://github.com/" + s.Repository + "/pull/" + strconv.FormatUint(s.Number, 10),
		"mergeable": true, "user": map[string]any{"login": "author-bot"},
		"head": map[string]any{"ref": s.HeadBranch, "sha": s.Head}, "base": map[string]any{"ref": s.BaseBranch},
	}
	var merr error
	raw := func(v any) json.RawMessage {
		b, err := json.Marshal(v)
		merr = errors.Join(merr, err) // strings, numbers and nil cannot fail; checked all the same
		return b
	}
	c := ghprovider.Cassette{
		Redaction: "Synthetic, not recorded: written from the GitHub REST documentation (2022-11-28) for one pull request opening by the " +
			"scenario's own exact values (repository, branches, head commit, title, body). The login is invented; no header, token, " +
			"e-mail address or real person appears, and the builder refuses a value that the recorder's redaction would change.",
		Interactions: []ghprovider.Interaction{
			{Method: "GET", Path: repo + "/git/ref/heads/" + strings.Join(segs, "/"), Status: 200,
				Body: raw(map[string]any{"ref": "refs/heads/" + s.HeadBranch, "object": map[string]any{"sha": s.Head}})},
			// The first lookup finds none; the repeat (a retry after a lost answer) finds the pull request this scenario created.
			{Method: "GET", Path: lookup, Status: 200, Body: raw([]any{})},
			{Method: "GET", Path: lookup, Status: 200, Body: raw([]any{pull})},
			{Method: "POST", Path: repo + "/pulls", Status: 201,
				Request: raw(map[string]string{"title": s.Title, "body": s.Body, "head": s.HeadBranch, "base": s.BaseBranch}), Body: raw(pull)},
		},
	}
	if merr != nil {
		return nil, fmt.Errorf("replay: %w", merr)
	}
	out, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return nil, fmt.Errorf("replay: %w", err)
	}
	return out, nil
}
