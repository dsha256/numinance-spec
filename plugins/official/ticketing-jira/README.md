# ticketing-jira

The ticketing provider for Jira Cloud: a capability provider of `numinance.ports.ticketing.v1` (plugin id `dev.numinance.ticketing-jira`)
over the Jira Cloud platform REST API, version 3. It is written against the public SDK only and names no vendor in the core: the change
context knows only the port. There are two builds: the replay build (ADR 0060 card 3) and the **live build** (this card, "Live mode"
below), which reaches one named Jira Cloud instance through the egress proxy with a credential handle and sends reads only unless it
is launched with `-write-back`.

## What a call does

Five verbs, each one `PluginService.Invoke`; the host refuses a provider that leaves a side-effecting RPC of its port without an entry, so
all five are served, but no shipped capability set contains the three writes until the write-back card (ADR 0060 decision 9).

| Verb | Risk | What it sends |
|---|---|---|
| `GetIssue` | READ | one `GET` of the issue with a fixed field list and the operation-id property |
| `SearchIssues` | READ | one `POST` of an enhanced JQL search built from the project and an escaped text |
| `CommentOnIssue` | LOW, keyed | one `GET` of the issue with the key's property; if it names a comment, a `GET` of that comment and of the caller's own account, and the call answers "existing" only if the comment is ours and says what was asked; otherwise a comment and a property that records its id |
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
  matched as plain text and never as a query. A quote is escaped for the literal and again for the text search (so it neither ends the
  literal nor opens a phrase that cannot be parsed), and the words AND, OR and NOT, which the text search reads as operators only in
  capitals, are written in lower case (the search ignores case, so the match is unchanged). Query languages are the later capability
  `ticketing.query_language`.
- **Comments** are Atlassian Document Format with one paragraph per line and one unmarked text node per paragraph. Wiki markup, HTML and
  an account mention in the text stay characters; only a `mention` node notifies a person and the provider never builds one. The host's
  idempotency key is hashed and recorded as an issue property after the comment is added. The property can be written by anyone who
  can edit the issue, so it only names a comment: "existing" needs the comment to be on the issue, written by the account of the
  credential in use and made of unmarked text paragraphs equal to the text asked for. A comment that fails any of these (planted
  property, edited or deleted comment) ends the call `UPSTREAM_ERROR` with nothing posted: no false claim, and no second comment.
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

A read-scoped token or a method-restricted injection rule is required before any live card (ADR 0060 decision 9). The decisions
rows `2026-11-09/ticketing-jira-network-intent` and `2026-11-09/ticketing-jira-credentials` settle it: see "Live mode".

## Builds

| Build | Package | Use |
|---|---|---|
| `ticketing-jira-replay` | `cmd/replay` | the provider over the embedded cassette; no network, no token; its manifest declares no network destination and no credential intent and the dev environment only |
| `ticketing-jira` | `cmd/live` | the provider for one named instance through the egress proxy; launched with `-host` (and optionally `-write-back`); its manifest declares the instance host on port 443 and the credential intent |

Build into a temporary directory, never into the tree: `go build -o "$(mktemp -d)/ticketing-jira" ./plugins/official/ticketing-jira/cmd/live`
from `numinance-spec`. Run by hand either build refuses (wrong magic cookie).

## Live mode

Settled by the decisions rows `2026-11-09/ticketing-jira-network-intent` and `2026-11-09/ticketing-jira-credentials` (not ADRs).

- **Instance.** `-host` is the exact host name of the one instance, for example `acme.example.test` in tests. It is checked by
  `ValidHost` (lower-case DNS name of two or more labels, no wildcard, port, scheme, path, IP literal or local name). The manifest is
  built in memory at launch with that one host on port 443 and the credential intent `ticketing.jira.api` for every capability. Nothing
  a request or an answer carries can change the destination.
- **Egress.** The transport (`provider/live.go`) accepts only `https` requests for that host with a credential handle the host gave a
  proxy for, and sends each as an absolute-form request for `host:443` to that proxy. Anything else is refused before a connection is
  made, there is no direct connection and no ambient proxy, every redirect is refused (the 3xx is an unexpected answer and no second
  request is made), and no URL of an answer is fetched. Every call has a 30 s timeout and the caller's deadline; an answer is read
  through a 4 MiB limit; a request body is at most 1 MiB; error messages are the fixed ones of `provider/errors.go`.
- **Credential.** The provider asks the host for a handle per call and sends only that handle. The broker's catalog entry (the
  operator's, not shipped here) is intent `ticketing.jira.api`, header `Authorization`, prefix `Basic `, a secret whose value is the
  base64 of `email:token`, and the one destination `{instance host, 443, path prefix /rest/api/3}`. No token, e-mail address or
  `Authorization` header is ever in this process, a log or a fixture.
- **Read only by default.** The injection rule cannot restrict a method, so the build does: without `-write-back` the transport sends
  only `GET` and the search `POST`, and every write ends `PERMISSION_DENIED`. Provision the token for an account with read permission
  only; both are needed (ADR 0060 decision 9). No shipped capability set enables the three writes before the write-back card.
- **Not an official signed install.** The host is per customer and the manifest is built at launch, so the live build is installed by
  the operator against one instance. The instance-bound flag on `NetworkIntent` that ADR 0060 ruling OQ2 describes is not added by this
  card.
- **Tests.** `provider/live_test.go` runs the whole kit (24 cases, none skipped) against the live build with a fake egress proxy and a
  fake Jira over TLS on the reserved name `acme.example.test`, which resolves to a loopback listener only inside the test. The fake Jira
  answers from the cassette, requires the injected credential and fails any request that still carries a handle. The same file checks
  that the provider never holds a credential header, that every request is for the instance host on port 443, the read-only default,
  the refusals of the transport, a missing handle and a redirect.

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

- **Fixed in this card (review findings of PR 582).** `CommentOnIssue` no longer believes the property alone (see "Comments"); the
  text search escapes a quote for the text search and neutralises AND, OR and NOT. Both rest on documented Jira behaviour (operators
  are read in capitals only; the search ignores case) that no tenant has confirmed: a text-caused 400 still maps to NOT_FOUND by design.
- **The instance-bound intent flag and org-scoped host binding** of ADR 0060 ruling OQ2 are not built. The live build is installed by
  the operator against one instance, not as an official signed plugin, until they are.
- **Not verified against a tenant.** The live build ran only against the fake of `provider/live_test.go`; the request shapes follow
  the public documentation. Points to verify at the first run against a tenant: that `expand=transitions` on the issue read returns
  each transition's target status; the `properties` parameter on the issue read; the enhanced JQL search (`/search/jql`) response
  fields; that the comment read returns the author account and an unchanged document, and the identity read the account of the
  token; that Basic e-mail and token authentication is accepted on the instance host; and that an account with read permission
  only gets 403 on the writes.
- **A deleted or edited comment blocks its key.** The comment a property names must still match; if not, the call fails closed rather
  than post a second comment, and the property has to be removed by hand.
- **The comment record is not atomic.** The comment and the property that records the host key are two requests. If the second fails the
  call fails and a retry can add a second comment. Finding a comment by a marker inside it (as the GitHub provider does) would need the
  marker in the text; the kit requires the text to go out unchanged. A comment property search may replace this at write-back.
- **Cancel detection** reads names, so a workflow with other words for "canceled" maps them to `DONE`.
- Custom fields, the `ticketing.query_language` capability, link types other than `Blocks`, and a Data Center or Server instance.
