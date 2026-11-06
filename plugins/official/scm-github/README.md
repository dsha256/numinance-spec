# scm-github

The scm provider for GitHub: a capability provider of `numinance.ports.scm.v1` over the GitHub REST API (version 2022-11-28). It is
written against the public SDK only and names no vendor in the core: the release context knows only the port.

## What a call does

Eleven verbs, each one `PluginService.Invoke`; `VendorCall` does not exist in 1.0.0. The provider asks the host for a credential
handle for the intent `scm.github.api` (`HostService.RequestCredential`) and sends it in the `Numinance-Handle` header to the egress
proxy, which injects the token. No token is in the provider's process; redirects are refused; the manifest declares one
network destination, `api.github.com:443`.

- **Requests** are validated against the grammar of `scm.proto` (full repository name, full commit ids, branch names, paths) before
  anything is sent; a branch or tag never stands for a revision. A repository with a `.` or `..` segment never reaches a request path.
- **Answers** are untrusted: at most 4 MiB, decoded without trust, every field checked (commit ids, branch names, URLs, check names,
  counts) and never repaired or truncated; a violation ends the call `UPSTREAM_ERROR`.
- **OpenPullRequest** checks that the head branch points at `head_revision` (the pull request is the verified candidate), finds an open
  pull request for the same head and base before it creates one, and emits one `pr-ref` receipt (repository, number, head, base; never
  the title or body). Dry run opens nothing.
- **MergePullRequest** (PRIVILEGED) merges only at `expected_head_revision` (squash), answers a merge already done as done, refuses a
  closed or moved pull request. Dry run merges nothing.
- **CommentOnPullRequest** finds its own earlier comment by a marker derived from the host's idempotency key.
- **SearchCode** wraps the text in one quoted phrase (a qualifier such as `repo:other/x` cannot widen the search) and drops any hit
  of another repository.
- **GetPullRequestStatus** reads every page of reviews and check runs (at most five pages of 100; a full last page is
  `LIMIT_EXCEEDED`, never a silent cut) and counts an approval only from a reviewer who holds write access (`collaborators/<login>/permission`)
  and is not the pull request's author. Every query is built with `url.Values`, so a branch name with `&` or `=` stays one value.
- **GetBranchProtection** reads the host's state and changes nothing; no verb edits protection, a workflow file or a ruleset.

## Builds

| Build | Package | Use |
|---|---|---|
| `scm-github` | `.` | talks to GitHub through the egress proxy; launched by a plugin host |
| `scm-github-replay` | `cmd/replay` | the same provider over recorded exchanges (`replay/cassettes`); no network, no token; its manifest declares no network destination and no credential intent and the dev environment only; `-cassette FILE` (absolute, repeatable) adds a cassette to the embedded ones |
| `scm-github-record` | `cmd/record` | the live provider whose client also writes a redacted cassette (`-out`, `-rename from=to`); the host launches it with those arguments |

Build into a temporary directory, never into the tree: `go build -o "$(mktemp -d)/scm-github-replay" ./plugins/official/scm-github/cmd/replay`.
Run by hand either refuses (wrong magic cookie).

## Cassettes

`replay/cassettes/scm.json` holds the exchanges (method, path, request body, status, answer). Exchanges with the same request answer in
order and the last repeats; a request with no exchange fails closed (`UPSTREAM_UNAVAILABLE`), so a replay never invents an answer.
`replay.PullRequestCassette` builds the cassette of one pull request opening from its exact values (repository, base, head branch, head
commit, title, body, number): the ref of the head branch, the lookup for an open pull request and the creation. A run whose ids and head
commit are not known until it runs (the engine's dry run) builds it at the first call from the repository it pushed
to and passes it with `-cassette`. Matching is exact on every one of those fields, never a wildcard, so a request for another head, branch,
base, repository, title or body has no recording and fails. The builder refuses a value that the recorder's redaction would change (a
credential pattern, an e-mail address). A test checks that every embedded cassette is already redacted and says how it was made.

**The embedded cassettes are not recordings**: they are written from the REST documentation with invented names (`acme/shop`, `reviewer-one`) and fixed
commit ids, and the file's `redaction` field says so. Replace them with recordings when a live run exists.

## Reaching the host

The live build asks the host for a handle (`RequestCredential`); the answer carries `egress_proxy`, the address of the proxy listener of the
invocation's step. `ghprovider.Live` sends each request there, as an absolute-form `http://api.github.com:443/...` request with the handle in
`Numinance-Handle`; the proxy applies the token it holds and re-originates it over TLS. The transport refuses a request with no handle, a handle with no
proxy address (or an expired one), and any destination other than `api.github.com:443`, and it opens no other connection. The provider's environment is the
three handshake variables; there is no ambient proxy setting.

## Recording cassettes

Run the live test with the recording build against a repository you own (needs the local `gh` login and the network):

```
NUMINANCE_LIVE_GITHUB=1 NUMINANCE_LIVE_GITHUB_REPO=owner/repo \
NUMINANCE_LIVE_GITHUB_RECORD=/absolute/path/outside/the/tree/scm-live.json \
NUMINANCE_LIVE_GITHUB_RENAME=owner/repo=acme/shop,owner=owner-one \
  go test -count=1 -run TestLiveGitHub ./cmd/internal/credentialwire    # from numinance/
```

The recorder keeps no header (the handle and every credential travel in headers), replaces token patterns and e-mail addresses, and applies the renames
(longest first) to paths, request bodies and answers; the file's `redaction` field says how many names were renamed and never names the originals. Before a
cassette is committed under `replay/cassettes/` a person reads it: commit ids, user logins, URLs and any free text in an answer are not renamed unless
listed. A request path in the recording is the renamed one, so a replay of it needs the renamed repository in the request.

## Tests

`go test ./plugins/official/scm-github/... ./conformance/scm/...` (from `numinance-spec`): the provider over the replay (bounds, untrusted
answers, no handle means no call, the handle header and no other credential, the manifest), the scenario cassette (a wrong request fails) and the conformance kit. The host-level
acceptance (the real plugin host launching the replaying build) is the reference implementation's release acceptance test.

## Not done

- Committed recordings: `replay/cassettes/scm.json` is still hand-written. A recording exists only outside the tree until a person checks its redaction (below).
- The live test covers `GetRepository` and `GetBranchProtection`; the other nine verbs are exercised in replay only. Planned: the first end to end run covers the pull request path (`OpenPullRequest`, `GetPullRequestStatus`); the rest are owed to later work that needs them.
- Verbs the host offers beyond the eleven (GitHub Enterprise server URL, GraphQL); the credential intent `scm.github.api` is in a broker catalog only as the `-intent` flag (no public catalog entry exists yet).
- The SBOM and provenance references in the manifest are placeholders, as in the other official providers; the install-time checks refuse them.
