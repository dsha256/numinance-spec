# knowledge conformance kit, v1

Skeleton that runs. The acceptance tests for a provider of `numinance.ports.knowledge.v1.KnowledgeService` (`proto/numinance/ports/knowledge/v1/knowledge.proto`). The kit version tracks the port minor; this skeleton tracks 1.0.0. The runnable skeleton is the Go package `knowledgekit` in this directory (`kit.go`): it runs CORE-001, 002, 003, 004, 005, 007, 008, 012, 013, 014 and 015 against any `Target`, and `kit_test.go` runs it against the sample provider (`plugins/examples/sample-knowledge`); the plugin host's acceptance test runs the same kit through a real launch and `Invoke`. The remaining cases need fixtures the skeleton does not carry yet (principals, the 1 MiB document, a broken source, an unreachable source) and stay in the table below as the list the kit must hold; the Host checks are enforced and tested by the plugin host itself. Modes are those of ADR 0015: replay, live publisher-run in an attested runner, live registry-run.

Each case runs against a source the kit seeds: a scope `kit` with documents `doc-a` (text, two versions, a title holding U+200B), `doc-b` (text over 1 MiB, multi-byte characters around the 1 MiB mark) and `doc-c` (visible to principal P1 only), and a scope `other` that the connection's `spec.scope` does not list, with a document `doc-o` that matches the query of CORE-001. Principals P1 and P2 each have a user-delegated connection. Every request names scope [`kit`] unless the case says otherwise. Replay fixtures are redacted.

## Core

| Id | Verb | Case | Expect |
| --- | --- | --- | --- |
| CORE-001 | Search | a query matching `doc-a` and `doc-o` | OK; a result with the ref of `doc-a`, title and snippet within bounds, the title's U+200B replaced by U+FFFD; no result naming `doc-o` |
| CORE-002 | Search | a query matching nothing | OK with no results |
| CORE-003 | Search | `max_results` 1 for a query matching two documents | exactly one result |
| CORE-004 | Search | bounds: `query` empty and of 1025 bytes; `scope` empty, duplicated, 257 entries, `kit/../other`, `/kit` and `kit\x`; `max_results` 0 and 101 | INVALID_ARGUMENT for each |
| CORE-004a | Search | sent to the provider directly, past the host (which refuses them, HOST-001): scope [`kit/%2E%2E/other`] (single-encoded), [`kit/%252E%252E/other`] (double-encoded), [`kit/．．/other`] (U+FF0E) and [`kit/..;/other`], each with the query of CORE-001 | INVALID_ARGUMENT for each (each breaks the scope grammar); no result from `other`: the provider never decodes or normalises an entry and percent-encodes it per segment, '%' included, when placing it in a URL |
| CORE-005 | Search | a query with the source's query-language operators | treated as text: no result outside scope `kit` |
| CORE-006 | Search | P2 searching for `doc-c` | no result naming `doc-c` |
| CORE-007 | Fetch | `doc-a`, latest | OK; `digest` matches content; `version` is the latest; `total_bytes` equals the content length; `media_type` set |
| CORE-008 | Fetch | `doc-a` at its first version | OK; the first version's content; `version` equal to the request's |
| CORE-009 | Fetch | `doc-b` with `max_bytes` 1048576 | at most 1048576 bytes of valid UTF-8, cut at a character boundary; `total_bytes` above the content length |
| CORE-010 | Fetch | P2 fetching `doc-c` | NOT_FOUND or PERMISSION_DENIED; no content |
| CORE-011 | Fetch | a version the source no longer serves | FAILED_PRECONDITION |
| CORE-012 | Fetch | a ref never written; the ref of `doc-o` | NOT_FOUND for each |
| CORE-013 | Fetch | bounds: `ref` empty and of 1025 bytes; `version` of 257 bytes; `max_bytes` 0 and 1048577; `scope` empty and `kit/../other` | INVALID_ARGUMENT for each |
| CORE-013a | Fetch | the ref of `doc-o` with each scope of CORE-004a, sent the same way; where the source's refs are paths, a ref `kit/../other/doc-o` in the forms of CORE-004a with scope [`kit`] | INVALID_ARGUMENT for each scope variant; NOT_FOUND for each path-ref variant; no content from `other` |
| CORE-014 | Resolve | two refs to `doc-a` (as Search returns it and in another form the source accepts) | the same `canonical_ref`; resolving it again returns it unchanged |
| CORE-015 | Resolve | a ref never written; the ref of `doc-o`; `ref` empty; `scope` empty | NOT_FOUND; NOT_FOUND; INVALID_ARGUMENT; INVALID_ARGUMENT |
| CORE-015a | Resolve | the requests of CORE-013a | INVALID_ARGUMENT for each scope variant; NOT_FOUND for each path-ref variant; no `canonical_ref` |
| CORE-016 | Resolve | P2 resolving `doc-c` | NOT_FOUND or PERMISSION_DENIED; no `canonical_ref` |
| CORE-017 | all | the source unreachable | UNAVAILABLE, never an empty result |
| CORE-018 | all | the source rate-limiting the provider | RESOURCE_EXHAUSTED |
| CORE-019 | all | every call of the kit | the source saw reads only: no write, comment or state change |

## Host checks

The host enforces these on any provider; the kit runs them with a scripted provider.

| Id | Case | Expect |
| --- | --- | --- |
| HOST-001 | a request whose scope names `other`, or `kit2` when `spec.scope` lists `kit`, or breaks the scope grammar (`kit/../other`, `/kit`, `kit\x`), or is `kit%2F..%2Fother` (one segment, neither `kit` nor below it), or holds a '%' or a segment whose decoding or NFKC form is a dot segment: `kit/%2E%2E/other`, `kit/%252E%252E/other`, `kit/．．/other` (U+FF0E), or a segment of only dots and spaces, with a leading or trailing U+0020, or starting with '..': `kit/. ./other`, `kit/ x/other`, `kit/..;/other` | the host refuses each before `Invoke`. The provider's encoding rule is tested on its own (CORE-004a, 013a, 015a) |
| HOST-002 | a `digest` that does not match content; content longer than `max_bytes` or `total_bytes`; uncut content shorter than `total_bytes`; invalid UTF-8 for a text media type | the host rejects the response and records the call failed |
| HOST-003 | more results than `max_results`; two results with one ref; a title, snippet, ref or version out of bounds; a `version` other than the requested one; a `media_type` outside its grammar | the host rejects the response and records the call failed |
| HOST-004 | a response carrying an `access_basis` or an authorization field as an unknown field | the host rejects the response (unknown field) and records the call failed; a receipt's `access_basis` always comes from `spec.auth`. The same for any field unknown to the host's descriptor copy (plugin/v1 `Invoke` strict decoding), as a provider built against a later minor would send if it ignored `HandshakeRequest.spec_version` |
| HOST-005 | a provider whose Fetch and Resolve dereference URL refs naming 169.254.169.254, loopback, a private range and a host the connection does not list; a 3xx redirect to IMDS; a 3xx redirect to an allow-listed name resolving to a denied range; an allow-listed name that rebinds between lookups; `fd00:ec2::254`, `::ffff:169.254.169.254`, `2852039166`, `0251.0376.0251.0376`, `169.254.169.254.`, `0.0.0.0`, `[::]`, `64:ff9b::a9fe:a9fe`, `2002:a9fe:a9fe::1` (6to4), `::169.254.169.254` (IPv4-compatible), `::ffff:0:a9fe:a9fe` (IPv4-translated), `64:ff9b:1::a9fe:a9fe` (local-use NAT64: denied as part of 64:ff9b:1::/48 with no prefix length configured, and judged as 169.254.169.254 with a /96 configured) and a Teredo `2001::/32` address; an HTTPS-to-HTTP redirect on a public allow-listed host whose connection does not approve plain HTTP; an approval of plain HTTP for a connection with a credential (user-delegated or service-account auth); a credentialed connection redirected from HTTPS to plain HTTP that is approved for another, credential-free connection; an approval of an on-premises host whose trusted resolver returns 17 addresses; another host name, or a raw URL, aimed at a pinned private address of an on-premises host; the pinned on-premises name, after approval, now resolving to an address outside the pinned set; plain HTTP to the pinned host | every connection to a denied address fails in the sandbox, including the other host name and the raw URL aimed at the pinned address, which is denied for any other destination; the rebinding name is reached only at the address the proxy checked; the address outside the pinned set and plain HTTP fail; the downgrading redirect is refused and no request, and no credential, is sent over HTTP; the plain-HTTP approval for a credentialed connection is refused; the credentialed redirect never carries the credential; the approval with 17 addresses is refused; no content and no `canonical_ref` is returned from them; the pinned set in evidence is unchanged. The allow-listed on-premises host is reached under its own name at each address of its pinned set (several A/AAAA records rotating), over HTTPS verified against that name, and each pair (host, address) is in evidence |

## Extended

None in 1.0.0. Semantic search, section-aware and paginated fetch, and vendor query languages are later capabilities.

## Not covered by this kit

- Tool pinning, drift quarantine and the broker's refusal of non-READ tools belong to the bridge and the knowledge context.
- The AuthorizationContext, the cache key, revocation and receipts are the knowledge context's (V1-D, V1-E, V1-J), as is the rule that Fetch and Resolve take only refs a Search on the same connection returned in the same `context_generation`, or repository docs.
- Sanitization and redaction of returned text happen in the knowledge context before persistence.
