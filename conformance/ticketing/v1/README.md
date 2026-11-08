# ticketing conformance kit, v1

Replay kit. The acceptance tests for a provider of `numinance.ports.ticketing.v1.TicketingService`
(`proto/numinance/ports/ticketing/v1/ticketing.proto`). The kit version tracks the port minor; this kit tracks 1.2.0. The runnable
kit is the Go package `ticketingkit` in this directory (`kit.go`): it runs TICKETING-001 to TICKETING-024 against any `Target` with the
fixtures a publisher supplies (`Subject`). `ValidateIssue` and `ValidateSummary` are the port's response rules as functions a host or
a test can apply to any provider. Modes are those of ADR 0015: replay, live publisher-run, live registry-run.

`kit_test.go` proves the kit against a small in-memory fake (`fake_test.go`, with the
fixture `testdata/tracker.json`): the conforming fake passes every case, and each of fifty-six fakes broken in one way fails exactly the cases
that guard that fault (and no other); a fake with four faults at once fails several. A provider's own test builds a `Target` over its replaying build and calls `ticketingkit.Run`, exactly as the
`scmkit` and `cikit` tests do. The official provider `plugins/official/ticketing-jira` runs all 24 cases against its replaying build.

## Core

| Id | Verb | Case | Expect |
| --- | --- | --- | --- |
| TICKETING-001 | GetIssue | an existing issue | the requested key and project; a title of 1 to 1024 printable bytes; a body within `max_body_bytes`; a state of the four categories; bounded type, labels and operation ids (`ValidateIssue`) |
| TICKETING-002 | GetIssue | an issue that does not exist | NOT_FOUND |
| TICKETING-003 | GetIssue | an issue that moved to another project, asked under the old project and under the new one | NOT_FOUND under the old project; the issue, with its new project, under the new one |
| TICKETING-004 | GetIssue, CommentOnIssue, TransitionIssue, LinkIssue | a missing issue; a hidden one; a moved one asked under its old project; one asked under another project; one in a forbidden project; a comment, a transition and a dry-run transition on an issue under another project, in a forbidden project, and (dry run) on a hidden issue; a blocked issue that is missing, hidden, in a forbidden project, or not in `blocked_project`; the dry-run transition of a missing, a moved, a hidden, a wrong-project and a forbidden-project issue; a search in a forbidden project and in a project that does not exist | NOT_FOUND for each, with ONE byte-identical message that holds neither the key, the project nor any tracker text (no masking: the kit compares the raw message) and stays in the closed vocabulary of a fixed message (see below); PERMISSION_DENIED fails |
| TICKETING-005 | GetIssue | a long body: `max_body_bytes` 65536; exactly the body length; one under it; every limit from 1 to 40 (the body holds a two-byte character, so most limits fall inside it) | the whole body and `truncated` false; the whole body and `truncated` false; `truncated` true; `truncated` true and the longest prefix that ends at a character boundary |
| TICKETING-006 | GetIssue, SearchIssues | the state category of an issue; a search in OPEN and in DONE | the issue's category; every hit in the requested category |
| TICKETING-007 | GetIssue | an issue whose raw text holds control, bidirectional and zero-width characters in the title, body, type, a label and an operation id | `ValidateIssue` passes (a label that is empty after the strip is dropped, not returned) and none of U+0000 to U+001F (a body keeps its line feed and tab), U+007F to U+009F, U+061C, U+200B to U+200F, U+202A to U+202E, U+2060, U+2066 to U+2069, U+FEFF is returned; the provider strips them (ADR 0060 decision 7), and the Engine strips them again. A search that matches the dirty issue returns its summary with the same stripping and bounds (`ValidateSummary`) |
| TICKETING-008 | GetIssue | an issue with recorded operation ids | exactly the ids on the issue; the Engine, not the provider, accepts one only if it equals the id it computed |
| TICKETING-009 | GetIssue | a malformed project; a malformed key (empty, a space, `#`, over 64 bytes); `max_body_bytes` 0 and 65537 | INVALID_ARGUMENT for each |
| TICKETING-010 | SearchIssues | `max_results` 0 and 51; text over 256 bytes; `max_results` 2 over four matches; 50 | INVALID_ARGUMENT; INVALID_ARGUMENT; INVALID_ARGUMENT; two hits and `truncated`; every match and not `truncated`, none from another project |
| TICKETING-011 | SearchIssues | text that looks like a JQL clause, a quote breaker, `*`, `~`, a bracket and a field query | no error and no hit: the text is matched as plain text and never read as a query |
| TICKETING-012 | CommentOnIssue | one key twice, then another key | one comment id with `existing` false then true; a new id for the other key: never posted twice |
| TICKETING-013 | CommentOnIssue | an empty body; a body over 8192 bytes; a hidden, a moved and a missing issue | INVALID_ARGUMENT; INVALID_ARGUMENT; NOT_FOUND for each |
| TICKETING-014 | TransitionIssue | dry run to DONE; the call; a repeat; a transition back to the category before | the dry run answers previous OPEN, state DONE, changed, and the issue stays OPEN; the call moves it; the repeat answers changed false; the transition back works (the compensation) |
| TICKETING-015 | TransitionIssue | a category no transition reaches, dry run and call; the unspecified target and an unknown number; a hidden and a moved issue | CONFLICT (FAILED_PRECONDITION) with one fixed message (the same for the dry run and the call) that holds no key, project, title or tracker text, and the issue unchanged; INVALID_ARGUMENT; NOT_FOUND |
| TICKETING-016 | LinkIssue | the same BLOCKS link twice; a self link; no link type; a missing, a hidden and a moved issue | `existing` false then true; INVALID_ARGUMENT; INVALID_ARGUMENT; NOT_FOUND for each |
| TICKETING-017 | CommentOnIssue, TransitionIssue, LinkIssue | each write twice (a made change and a found one), and a dry run | exactly one `ticketing-action` record with an id per call, none for the dry run; a call without it is PARTIAL |
| TICKETING-018 | all | the descriptor of the port, not a provider | five verbs and no other (no VendorCall); GetIssue and SearchIssues READ; CommentOnIssue LOW, no dry run; TransitionIssue MEDIUM with dry run and the compensation `TicketingService.TransitionIssue`; LinkIssue LOW, no dry run; every write declares `ticketing-action`, every verb is SYNC, idempotent, described and has a `since`, and none names a capability; `project` carries the `resource_name` mark on every request and `blocked_project` also on LinkIssue, and no other field does (`CheckPort`, which a test also runs on mutated copies of the descriptor) |
| TICKETING-019 | all five | a malformed project, key, blocked project or blocked key (empty, `../x`, a control character, a line feed, a space, a leading `-`, a bidirectional control, a fullwidth letter, over the bound; a path-like project: `a/../b`, `a/./b`, `a//b`, `a/`, `PAY/..`, `/a`, `a/b/c`, `..`, `a/..`); search text with a control, bidirectional or zero-width character or over 256 bytes; an unknown search state, transition target or link type | INVALID_ARGUMENT for each, before any lookup, with a fixed message (see below): it never repeats the offending value, whole or in part, in any spelling, and is the same message for the value and for its twin |
| TICKETING-020 | SearchIssues | a text that a hidden issue of the project matches, and a word only the hidden issue holds | the hidden issue is not listed; `max_results` equal to the visible matches answers `truncated` false (a hidden issue is not counted); the word finds nothing |
| TICKETING-021 | LinkIssue | a link from one project into another; a blocked issue in a forbidden project; a blocker in a forbidden project; a blocked key that is not in `blocked_project`; a blocker key that is not in `project`; a moved blocked issue | the cross-project link works and is idempotent; every other call is NOT_FOUND with the one message of TICKETING-004: a link is authorised against both projects |
| TICKETING-022 | CommentOnIssue, TransitionIssue, LinkIssue | the evidence record of a call whose comment text and issue titles and bodies the kit knows | an allowlist: exactly one record of type `ticketing-action`; an id that is one opaque token (`[A-Za-z0-9_-]{1,64}`) holding no run of four letters or digits of a title, a body or the comment text; exactly the fields `verb=`, `project=`, `key=` and `result=`, each once; the first three equal the call, and the result is the comment id (CommentOnIssue), a category or `before>after` (TransitionIssue) or the blocked key (LinkIssue). A call that found its change already made records the same shape |
| TICKETING-023 | CommentOnIssue | one key used for one issue, then for another issue | a new comment on the other issue (a different id, `existing` false) or CONFLICT; never the first issue's comment id |
| TICKETING-024 | CommentOnIssue | a body with Jira account-mention syntax, `@name`, wiki and HTML markup | only for a Target that implements `Observer` (the Jira provider must); without one `Run` reports the case SKIPPED with the reason, never passed: the outbound text is the body unchanged and activates no mention or notification |

The gRPC status names of the proto comments map to the plugin reasons the kit checks as in the scm kit: INVALID_ARGUMENT to INVALID_ARGUMENT,
NOT_FOUND to NOT_FOUND, FAILED_PRECONDITION to CONFLICT, RESOURCE_EXHAUSTED to RATE_LIMITED, UNAVAILABLE to UPSTREAM_UNAVAILABLE.

Fixed messages. The message of every error the kit provokes (INVALID_ARGUMENT, NOT_FOUND, CONFLICT) is a fixed message: it uses only
ASCII letters, digits, a space and `. , : ; ( ) [ ] { } / _ ^ $ * | ? < > -` (at most 200 bytes), so a quote, a backslash, a percent
sign, a plus sign, an equals sign, an at sign and any non-ASCII character are outside it; it holds no key, project, title, body or
other tracker text (`Subject.TrackerTexts`); a CONFLICT is the same for the dry run and the call; and an INVALID_ARGUMENT repeats
nothing of the offending value: not whole, not case-folded, not quoted, URL-escaped, JSON-escaped, base64 or hex encoded, not any
run of four characters of it, and it is the same message when every letter and digit of the value is replaced by the next one (the
value's twin, which breaks the same bound). `ValidateErrorMessage` applies the vocabulary and the secrets check to any message; a
provider's tests call it for RATE_LIMITED, UPSTREAM_UNAVAILABLE and UPSTREAM_ERROR.

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

## Known gaps: obligations of the Jira provider card

The kit hardening card (after the ticketing port card) closed the holes of the first kit: existence oracles on dry runs and on a
search, the message check of every error class, search summaries, the evidence allowlist, the fixture scan and the encoded
echoes of a rejection. What the kit still cannot see is an obligation of the Jira provider card, which must hold and test each
of these in its own replay tests:

- Timing parity, above.
- Mentions and notifications (TICKETING-024 through `Observer`), above.
- Egress rules. The kit cannot see the network. The provider's manifest declares only the exact host and port of the instance
  and the host enforces them (ADR 0060 ruling OQ2); a read-scoped token or a method-restricted injection rule is required before
  any live card (decision 9).
- Redirects. The provider never follows a redirect with the injected credential.
- Ticket-supplied URLs. No URL inside ticket content or a response (ADF media, `self`, pagination, inline cards) is ever
  fetched.
- The error classes the kit cannot provoke: RATE_LIMITED, UPSTREAM_UNAVAILABLE and UPSTREAM_ERROR. The provider's tests apply
  `ValidateErrorMessage` to each (a fixed message of the closed vocabulary that holds no key, project, title or tracker text).
- Dry runs of the verbs that declare none (CommentOnIssue, LinkIssue) are not probed: the proto gives them no dry run.

## Fixture rule

A fixture names only reserved example hosts and tenants: a host is `example.test` or a name under it (RFC 2606 reserves `.test`) and a
tenant is one of the names that `TenantAllowed` accepts. Fixtures are written by hand from the public API documentation of the tracker or
recorded from a throwaway tenant, never from a customer or employer tenant, because this tree is public and mirrored on every tag.

`ScanFixtures` and `ScanDir` (and `ScanFixturesWith` and `ScanDirWith`, which take the closed set of a provider's own fixtures as an
`Allow`) are an allowlist, not a deny-pattern. A fixture is a JSON file under `testdata/`, parsed strictly (no duplicate key, no
trailing data).

- Every JSON key is one of a closed set (`Allow.Keys`). Every string value is one of a closed set of structural tokens
  (`Allow.Tokens`: the projects, the issue keys and the state names the fixtures use), an allowed host or an allowed `https` URL. An
  arbitrary alphanumeric, a tenant name or a vendor name is not a structural token. A key that names an identity (tenant, account,
  cloud, instance, org, site, workspace, host, domain, url, email, server, endpoint, address, origin, remote, href, uri, proxy, base,
  self or ip, in any spelling) must hold exactly an allowed tenant, host or URL; a number there is a problem.
- Free text (title, body, type, labels, operation ids, notes) may hold the control, bidirectional and zero-width characters that a
  provider must strip, and nothing else invisible: a Hangul, Khmer or braille filler, a variation selector, a tag, a soft hyphen or any
  other format character outside the strip set is a problem anywhere in a string. Free text is then read in every decoded form (JSON,
  percent, HTML entity and backslash escapes, repeated until nothing changes: a double-encoded dot, an entity inside an entity) with
  the invisible and combining characters removed, the dot, colon, at-sign and slash look-alikes and the fullwidth forms read as
  ASCII, and defanged separators closed up (a bracketed or parenthesised dot or the word dot, a dot with a space on either side, the
  word dot between spaces). It may hold no at sign, no IP literal in any spelling (dotted, hex, decimal, octal, IPv6), no URL and no
  dotted name outside the reserved host space.
- Any `.md` file in the kit directory and every string literal of a Go file (parsed with `go/parser`, import paths and struct tags
  excepted) are read the same way, except that a dotted name listed in `Allow.Dotted` (file names, Go and proto identifiers,
  versions) is accepted and an at sign is a problem only after a name (an e-mail address). A comment of a Go file is read the same
  way, so a host with any ending fails there too.
- A `_test.go` file that carries the comment `ticketingkit:scan-exempt` before its package clause holds the hostile inputs of the
  scan's own mutation tests and is skipped. The marker is honoured only in the base names listed in `Allow.Exempt` (`kit_test.go` and `scan_go_test.go` by default), in the kit directory; in any other file it is a problem.
- A string with leading or trailing white space, a Go file under any `testdata` directory, and any file that is not
  `testdata/**/*.json`, a `.md` file or Go source is a problem.
- What the scan cannot do: read a company name written in prose, or a host in base64 or another encoding it does not decode. A
  reviewer reads the fixtures; the scan keeps the known spellings out.

`TestFixtures_OnlyReservedHostsAndTenants` runs the scan over this directory, so a disallowed host fails `just test`. `gitleaks` does
not catch tenant names. `TestFixtures_ScanRejectsDisallowedNames` and `TestFixtures_ScanReadsGoSource` mutate the real fixture, the
README and Go source with each known bypass (defanged and spaced hosts, dot look-alikes, fillers, double encodings, at-sign
look-alikes, tenant and vendor names as structural tokens, numeric IP spellings, single-label internal hosts under host-like keys,
bare hosts in markdown, hosts in Go literals and comments, the marker in the wrong place) and require each to fail. A provider that
ships its own replay fixtures runs `ScanDirWith` over them in its own test.

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
