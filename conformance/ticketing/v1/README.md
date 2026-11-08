# ticketing conformance kit, v1

Replay kit. The acceptance tests for a provider of `numinance.ports.ticketing.v1.TicketingService`
(`proto/numinance/ports/ticketing/v1/ticketing.proto`). The kit version tracks the port minor; this kit tracks 1.2.0. The runnable
kit is the Go package `ticketingkit` in this directory (`kit.go`): it runs TICKETING-001 to TICKETING-024 against any `Target` with the
fixtures a publisher supplies (`Subject`). `ValidateIssue` and `ValidateSummary` are the port's response rules as functions a host or
a test can apply to any provider. Modes are those of ADR 0015: replay, live publisher-run, live registry-run.

There is no ticketing provider in this tree yet. `kit_test.go` proves the kit against a small in-memory fake (`fake_test.go`, with the
fixture `testdata/tracker.json`): the conforming fake passes every case, and each of thirty-seven fakes broken in one way fails exactly the cases
that guard that fault (and no other); a fake with four faults at once fails several. A provider's own test builds a `Target` over its replaying build and calls `ticketingkit.Run`, exactly as the
`scmkit` and `cikit` tests do. The official Jira provider is a later card.

## Core

| Id | Verb | Case | Expect |
| --- | --- | --- | --- |
| TICKETING-001 | GetIssue | an existing issue | the requested key and project; a title of 1 to 1024 printable bytes; a body within `max_body_bytes`; a state of the four categories; bounded type, labels and operation ids (`ValidateIssue`) |
| TICKETING-002 | GetIssue | an issue that does not exist | NOT_FOUND |
| TICKETING-003 | GetIssue | an issue that moved to another project, asked under the old project and under the new one | NOT_FOUND under the old project; the issue, with its new project, under the new one |
| TICKETING-004 | GetIssue, CommentOnIssue, TransitionIssue, LinkIssue | a missing issue; a hidden one; a moved one asked under its old project; one asked under another project; one in a forbidden project; a comment, a transition and a dry-run transition on an issue under another project, in a forbidden project, and (dry run) on a hidden issue; a blocked issue that is missing, hidden, in a forbidden project, or not in `blocked_project` | NOT_FOUND for each, with ONE byte-identical message that holds neither the key, the project nor any tracker text (no masking: the kit compares the raw message); PERMISSION_DENIED fails |
| TICKETING-005 | GetIssue | a long body: `max_body_bytes` 65536; exactly the body length; one under it; every limit from 1 to 40 (the body holds a two-byte character, so most limits fall inside it) | the whole body and `truncated` false; the whole body and `truncated` false; `truncated` true; `truncated` true and the longest prefix that ends at a character boundary |
| TICKETING-006 | GetIssue, SearchIssues | the state category of an issue; a search in OPEN and in DONE | the issue's category; every hit in the requested category |
| TICKETING-007 | GetIssue | an issue whose raw text holds control, bidirectional and zero-width characters in the title, body, type, a label and an operation id | `ValidateIssue` passes (a label that is empty after the strip is dropped, not returned) and none of U+0000 to U+001F (a body keeps its line feed and tab), U+007F to U+009F, U+061C, U+200B to U+200F, U+202A to U+202E, U+2060, U+2066 to U+2069, U+FEFF is returned; the provider strips them (ADR 0060 decision 7), and the Engine strips them again |
| TICKETING-008 | GetIssue | an issue with recorded operation ids | exactly the ids on the issue; the Engine, not the provider, accepts one only if it equals the id it computed |
| TICKETING-009 | GetIssue | a malformed project; a malformed key (empty, a space, `#`, over 64 bytes); `max_body_bytes` 0 and 65537 | INVALID_ARGUMENT for each |
| TICKETING-010 | SearchIssues | `max_results` 0 and 51; text over 256 bytes; `max_results` 2 over four matches; 50 | INVALID_ARGUMENT; INVALID_ARGUMENT; INVALID_ARGUMENT; two hits and `truncated`; every match and not `truncated`, none from another project |
| TICKETING-011 | SearchIssues | text that looks like a JQL clause, a quote breaker, `*`, `~`, a bracket and a field query | no error and no hit: the text is matched as plain text and never read as a query |
| TICKETING-012 | CommentOnIssue | one key twice, then another key | one comment id with `existing` false then true; a new id for the other key: never posted twice |
| TICKETING-013 | CommentOnIssue | an empty body; a body over 8192 bytes; a hidden, a moved and a missing issue | INVALID_ARGUMENT; INVALID_ARGUMENT; NOT_FOUND for each |
| TICKETING-014 | TransitionIssue | dry run to DONE; the call; a repeat; a transition back to the category before | the dry run answers previous OPEN, state DONE, changed, and the issue stays OPEN; the call moves it; the repeat answers changed false; the transition back works (the compensation) |
| TICKETING-015 | TransitionIssue | a category no transition reaches, dry run and call; the unspecified target and an unknown number; a hidden and a moved issue | CONFLICT (FAILED_PRECONDITION) and the issue unchanged; INVALID_ARGUMENT; NOT_FOUND |
| TICKETING-016 | LinkIssue | the same BLOCKS link twice; a self link; no link type; a missing, a hidden and a moved issue | `existing` false then true; INVALID_ARGUMENT; INVALID_ARGUMENT; NOT_FOUND for each |
| TICKETING-017 | CommentOnIssue, TransitionIssue, LinkIssue | each write twice (a made change and a found one), and a dry run | exactly one `ticketing-action` record with an id per call, none for the dry run; a call without it is PARTIAL |
| TICKETING-018 | all | the descriptor of the port, not a provider | five verbs and no other (no VendorCall); GetIssue and SearchIssues READ; CommentOnIssue LOW, no dry run; TransitionIssue MEDIUM with dry run and the compensation `TicketingService.TransitionIssue`; LinkIssue LOW, no dry run; every write declares `ticketing-action`, every verb is SYNC, idempotent, described and has a `since`, and none names a capability; `project` carries the `resource_name` mark on every request and `blocked_project` also on LinkIssue, and no other field does (`CheckPort`, which a test also runs on mutated copies of the descriptor) |
| TICKETING-019 | all five | a malformed project, key, blocked project or blocked key (empty, `../x`, a control character, a line feed, a space, a leading `-`, a bidirectional control, a fullwidth letter, over the bound; a path-like project: `a/../b`, `a/./b`, `a//b`, `a/`, `PAY/..`, `/a`, `a/b/c`, `..`, `a/..`); search text with a control, bidirectional or zero-width character or over 256 bytes; an unknown search state, transition target or link type | INVALID_ARGUMENT for each, before any lookup, and the message never contains the offending value |
| TICKETING-020 | SearchIssues | a text that a hidden issue of the project matches, and a word only the hidden issue holds | the hidden issue is not listed; `max_results` equal to the visible matches answers `truncated` false (a hidden issue is not counted); the word finds nothing |
| TICKETING-021 | LinkIssue | a link from one project into another; a blocked issue in a forbidden project; a blocker in a forbidden project; a blocked key that is not in `blocked_project`; a blocker key that is not in `project`; a moved blocked issue | the cross-project link works and is idempotent; every other call is NOT_FOUND with the one message of TICKETING-004: a link is authorised against both projects |
| TICKETING-022 | CommentOnIssue, TransitionIssue, LinkIssue | the evidence record of a call whose comment text and issue titles and bodies the kit knows | no id, type or field of the record holds a title, a body or the comment text (it carries the verb, project, key and result) |
| TICKETING-023 | CommentOnIssue | one key used for one issue, then for another issue | a new comment on the other issue (a different id, `existing` false) or CONFLICT; never the first issue's comment id |
| TICKETING-024 | CommentOnIssue | a body with Jira account-mention syntax, `@name`, wiki and HTML markup | only for a Target that implements `Observer` (the Jira provider must); without one `Run` reports the case SKIPPED with the reason, never passed: the outbound text is the body unchanged and activates no mention or notification |

The gRPC status names of the proto comments map to the plugin reasons the kit checks as in the scm kit: INVALID_ARGUMENT to INVALID_ARGUMENT,
NOT_FOUND to NOT_FOUND, FAILED_PRECONDITION to CONFLICT, RESOURCE_EXHAUSTED to RATE_LIMITED, UNAVAILABLE to UPSTREAM_UNAVAILABLE.

Cases the kit does not run and a provider must hold: an answer over 4 MiB (LIMIT_EXCEEDED), an answer out of bounds (UPSTREAM_ERROR),
a tracker rate limit (RESOURCE_EXHAUSTED), an outage (UNAVAILABLE, never an empty answer), ADF, wiki markup and HTML rendered to
plain text, a URL inside ticket content or in a response (a media link, a `self` link, a pagination link) never fetched, a
redirect never followed with an injected credential, and the following gaps.

- Timing parity. The not-found answer must be identical in timing as well as in status and message for no permission, does not
  exist, moved and wrong project (a timing difference is an existence oracle). A `Target` cannot observe timing, so the kit cannot
  test it; the engine intake card checks it (ADR 0060, owed card 4) and a provider must not branch on existence before it answers.
- Mentions and notifications. TICKETING-024 runs only for a Target that implements `Observer`. It is an OBLIGATION of the Jira
  provider card to implement it and to escape `[~accountid:...]`, `@user`, wiki markup, HTML and ADF control characters, because a
  comment body is plain text and must never notify or format; a `Target` that cannot show its outbound payload cannot be tested
  for it.
- Authorisation before `Invoke`. A call outside the grant's resource scope is denied by the host; TICKETING-018 only proves that
  the descriptor marks the right fields, and TICKETING-021 that a provider does not widen a link.

## Known gaps: ticketing kit hardening (owed before the Jira plugin card)

The kit has these known holes. They are one follow-up card, "ticketing kit hardening", which must merge before the Jira provider
card starts, because the Jira provider's replay tests rely on this kit.

- (a) Dry-run existence oracle beyond TransitionIssue: the dry run of every verb must be probed with the same NOT_FOUND message.
  A search in a forbidden project must not answer PERMISSION_DENIED (a project-existence oracle).
- (b) CONFLICT messages: no key, title or tracker text in any error, not only in NOT_FOUND (a conflict-echo provider passes).
- (c) A search case that returns the Dirty issue, so `SearchIssues` summaries are checked for stripping as `GetIssue` is.
- (d) Evidence: an allowlist (verb, project, key and result as fixed fields) instead of the deny-substring check of TICKETING-022
  (a provider that prefixes an id with other text passes).
- (e) Fixture scan: defanged hosts (`acme[.]corp`, `acme (dot) corp`), dot look-alikes U+FE52 and U+0701, filler characters such
  as U+3164, double percent escapes (`%252e`), HTML entities (`&amp;#46;`), a tenant name used as a structural token (a
  `project` or `key` value such as a real company name), hex and decimal IP literals, U+FE6B, bare hosts in README.md, and string
  literals in the kit's own Go files are not caught.
- (f) `wantInvalid` checks the raw offending value only; a rejection that echoes it encoded (`%q`, URL-escaped) passes.

## Fixture rule

A fixture names only reserved example hosts and tenants: a host is `example.test` or a name under it (RFC 2606 reserves `.test`) and a
tenant is one of the names that `TenantAllowed` accepts. Fixtures are written by hand from the public API documentation of the tracker or
recorded from a throwaway tenant, never from a customer or employer tenant, because this tree is public and mirrored on every tag.
`ScanFixtures` and `ScanDir` are an allowlist, not a deny-pattern: every fixture is a JSON file under `testdata/`, parsed strictly
(no duplicate key, no trailing data), and every key and every string value is decoded (JSON and percent escapes) and then must be a
structural token, an allowed tenant, an allowed host or an allowed `https` URL. A key that names an identity (tenant, account,
cloud, instance, org, site, workspace, host, domain, url, email, in any spelling) must hold exactly an allowed value. Free text
(title, body, type, labels, operation ids, notes) may hold the characters a provider must strip, but after they are removed
and the fullwidth and ideographic dots are read as dots it may hold no at sign, no IP literal, no URL and no dotted name outside
`example.test`. A string with leading or trailing white space, a Go file under any `testdata` directory, and any file that is
not `testdata/**/*.json`, `README.md` or Go source is a problem. The kit's own Go files are out of scope. `TestFixtures_OnlyReservedHostsAndTenants`
runs the scan over this directory, so a disallowed host fails `just test`. `gitleaks` does not catch tenant names.
`TestFixtures_ScanRejectsDisallowedNames` mutates the real fixture with each known bypass (a vendor host, an unusual top-level
domain, an e-mail address, userinfo with a backslash, a fullwidth dot, an escaped dot, an IPv6 literal, an identity key under
another spelling, padded names, a Go file under `testdata`, a nested new directory) and requires each to fail. A provider that ships its own replay fixtures runs `ScanDir` over them in its own test.

## Host checks

Enforced by the policy decision point, the plugin host and the change context, not by a provider; this kit does not run them. They are
the cases a host kit must hold.

| Id | Case | Expect |
| --- | --- | --- |
| HOST-001 | a request whose `project` (or, for LinkIssue, `blocked_project`) is outside the grant's resource scope, or that names none | the host denies before `Invoke`; run by the host's own tests, not here. What this kit runs is TICKETING-018 (the marks are in the descriptor) and TICKETING-021 (the provider does not widen a link) |
| HOST-002 | a manifest declaring IDEMPOTENCY_NONE for a write verb, a risk floor below the port's, or a write verb with no declared evidence | the host refuses it at handshake |
| HOST-003 | a response with a field unknown to the host's descriptor copy | the host rejects it and records the call as failed |
| HOST-004 | an operation id on an issue that is not the `operation_id` the Engine computed | the Engine ignores it |

## Extended

None in 1.2.0. The capability `ticketing.query_language` (JQL, WIQL) and any custom-field capability are later additions.
