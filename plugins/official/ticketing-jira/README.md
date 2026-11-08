# ticketing-jira

The ticketing provider for Jira Cloud: a capability provider of `numinance.ports.ticketing.v1` (plugin id `dev.numinance.ticketing-jira`)
over the Jira Cloud platform REST API, version 3. It is written against the public SDK only and names no vendor in the core: the change
context knows only the port. **This build is replay only** (ADR 0060, owed card 3). There is no live mode, no credential code and no
network intent yet; those are owed cards 5 and 6.

## What a call does

Five verbs, each one `PluginService.Invoke`; the host refuses a provider that leaves a side-effecting RPC of its port without an entry, so
all five are served, but no shipped capability set contains the three writes until the write-back card (ADR 0060 decision 9).

| Verb | Risk | What it sends |
|---|---|---|
| `GetIssue` | READ | one `GET` of the issue with a fixed field list and the operation-id property |
| `SearchIssues` | READ | one `POST` of an enhanced JQL search built from the project and an escaped text |
| `CommentOnIssue` | LOW, keyed | one `GET` of the issue with the key's property, then (if none) a comment and a property that records its id |
| `TransitionIssue` | MEDIUM, natural, dry run | one `GET` of the issue with its transitions, then (if it changes) one transition by id |
| `LinkIssue` | LOW, natural | two `GET`s (both issues, always), then (if no such link) one `Blocks` link |

- **Requests** are validated against the grammar of `ticketing.proto` (project, key, bounds, text) before anything is sent. A rejection
  is one fixed message per field and never repeats the value or a bound as a number.
- **Answers** are untrusted: at most 4 MiB, decoded without trust, rendered to plain text (Atlassian Document Format, or a plain string),
  stripped of control, bidirectional, zero-width and other format characters, and bounded to the port's limits. A title that is empty
  after the strip, an unknown status category or a document it cannot render ends the call `UPSTREAM_ERROR`; nothing is repaired.
  Links, media and cards of a document render as the placeholders `[attachment]` and `[card]`: no URL of an answer is read.
- **States.** Jira's status categories map to `OPEN`, `IN_PROGRESS` and `DONE`. Jira has one "done" category, so `CANCELED` is a
  "done" issue whose status or resolution name is in a short list (cancelled, won't do, declined, duplicate, rejected and similar).
- **Search** puts the text in a quoted JQL literal after escaping every character that Jira's text search reads as syntax, so the text is
  matched as plain text and never as a query. Query languages are the later capability `ticketing.query_language`.
- **Comments** are Atlassian Document Format with one paragraph per line and one unmarked text node per paragraph. Wiki markup, HTML and
  an account mention in the text stay characters; only a `mention` node notifies a person and the provider never builds one. The host's
  idempotency key is hashed and recorded as an issue property after the comment is added.
- **Transitions** choose the available transition with no screen whose target maps to the category, lowest id first; none is
  `CONFLICT`, for the dry run and the call alike. A dry run changes nothing.
- **Evidence.** Each write that made the change or found it made emits one `ticketing-action` record: `verb`, `project`, `key`, `result`
  (the comment id, a category or `before>after`, or the blocked key). Never a title, a body or a comment text.

## Obligations of the kit's "Known gaps", and where each is tested

| Obligation | How it holds | Test |
|---|---|---|
| Timing parity of not found | every verb fetches the issue with one request whatever it is and decides after; `LinkIssue` fetches both issues before it looks at either | `TestTiming_NotFoundMakesTheSameRequestsAsFound` |
| Mentions and notifications (TICKETING-024) | the replay target implements `ticketingkit.Observer`, reading the outbound payload with a walker of its own | kit case TICKETING-024 runs, never skipped (`TestConformance_ReplayingJiraProviderPassesEveryCase`) |
| Egress rules | a live manifest declares exactly the instance host on port 443 and the host grammar refuses a wildcard, a port, an IP literal and local names; the replay manifest declares none | `TestEgress_ManifestDeclaresOnlyTheInstanceHost`, `TestValidHost_RefusesWhatIsNotOneExactPublicName` |
| Redirects | the HTTP client refuses every redirect; a 3xx is an unexpected answer and no second request is made | `TestRedirect_IsNeverFollowed` |
| Ticket-supplied URLs | URLs are built from the configured host and path segments only; `self`, pagination, media and card URLs of an answer are never read | `TestURLsInAnswers_AreNeverFetched`, `TestRequests_StayOnTheInstanceWithoutCredentials` |
| RATE_LIMITED, UPSTREAM_UNAVAILABLE, UPSTREAM_ERROR | each ends with a constant message of the closed vocabulary | `TestErrorClasses_AreFixedMessagesOfTheClosedVocabulary` (`ValidateErrorMessage`) |
| Dry runs of verbs with none | not probed; the proto gives them no dry run | |

A read-scoped token or a method-restricted injection rule is required before any live card (ADR 0060 decision 9). That belongs to the
credentials and network intent decisions, not to this build.

## Builds

| Build | Package | Use |
|---|---|---|
| `ticketing-jira-replay` | `cmd/replay` | the provider over the embedded cassette; no network, no token; its manifest declares no network destination and no credential intent and the dev environment only |

Build into a temporary directory, never into the tree: `go build -o "$(mktemp -d)/ticketing-jira-replay" ./plugins/official/ticketing-jira/cmd/replay`
from `numinance-spec`. Run by hand it refuses (wrong magic cookie). There is no live build yet.

## The cassette

`replay/testdata/jira.json` is a set of HTTP exchanges for one invented tracker (the projects PAY, OPS and SEC) on the reserved host
`acme.example.test`. It is **not a recording**: it is written by hand from the public documentation of the Jira Cloud platform REST API,
and the file's `redaction` field says so. No customer, employer or vendor name appears in it.

A tracker is stateful, so an exchange may say which values a few named variables must hold (`given`) and what it sets afterwards
(`then`); of the exchanges that match, the one with the most `given` entries answers. A request with no exchange fails closed with
`UPSTREAM_UNAVAILABLE`, so a replay never invents an answer. Matching is exact on method, path, query and (when the exchange has one)
the JSON body. A request that names another host, uses plain HTTP or carries an `Authorization`, `Cookie` or handle header has no
recording.

The cassette is generated from the tables in `replay/world_test.go` (`go test -run TestCassette_IsTheGeneratedOne -update ./replay` from
this directory) so its lines stay consistent, and a test fails if the file differs from them. The conformance run walks the kit's cases in
their order against one provider: a comment, a transition and a link change what the next read answers, as on a real tracker.

The fixture scan of ADR 0060 decision 10 (`ticketingkit.ScanDirWith`) runs over the cassette and this README with this plugin's own
closed set of names (`TestCassette_OnlyReservedHostsAndTenants`). `TestCassette_ScanRejectsOtherHosts` mutates the cassette with a real-looking
host, a tenant name, an e-mail address and an IP literal and requires each to fail.

## Tests

From `numinance-spec`: `go test -race ./plugins/official/ticketing-jira/...`. They run the whole ticketing kit (all 24 cases) against the
replaying provider, the timing, redirect, URL, error-class and egress checks above, the manifest against the rules the plugin host applies
at handshake, and unit tests of the text rendering, the JQL escaping and the grammar. The host-level acceptance (the real plugin host launching the
replaying build) is the reference implementation's.

## Not done

- **Owed to the write-back card (review findings of PR 582, low, not exploitable in this replay-only build).** (a) `CommentOnIssue` answers `Existing` from an issue property without fetching the recorded comment, so anyone who can edit the issue and knows the idempotency key can plant a property and make the call report a comment that was never posted; before answering `Existing` it must fetch the comment and check its author and text. (b) The text search escapes JQL and Lucene punctuation but not the words AND, OR and NOT, and a bare quote may make Jira answer a 400 that maps to NOT_FOUND; the text clause is ANDed to the project clause, so it never widens a search beyond the project. Both need a live tenant to confirm.
- **Live mode.** No credential handle request, no egress proxy transport, no live build. It needs the credentials ADR (a Jira `email:token`
  or a bearer token, ruling OQ3) and the instance-bound network intent ADR (ruling OQ2), then the live cards.
- **Not verified against a tenant.** The request shapes follow the public documentation and were not run against a Jira Cloud tenant.
  Points to verify at the first live card: that `expand=transitions` on the issue read returns each transition's target status; the
  `properties` parameter on the issue read; the enhanced JQL search (`/search/jql`) response fields; and which hosts serve scoped tokens.
- **The comment record is not atomic.** The comment and the property that records the host key are two requests. If the second fails the
  call fails and a retry can add a second comment. Finding a comment by a marker inside it (as the GitHub provider does) would need the
  marker in the text; the kit requires the text to go out unchanged. A comment property search may replace this at write-back.
- **Cancel detection** reads names, so a workflow with other words for "canceled" maps them to `DONE`.
- Custom fields, the `ticketing.query_language` capability, link types other than `Blocks`, and a Data Center or Server instance.
