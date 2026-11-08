# Receipt records

Status: normative for `evidence/v1` (spec 1.0.0). Read [records and chain](records-and-chain.md) first.

The trusted components of a deployment produce receipts of what they did: the credential broker states the handles it issued and revoked, the egress proxy the requests it judged. A receipt is a record of the change's chain whose actor is the component, so an auditor reads what the components did from the chain and not from a log. A receipt names refs, numbers, classes and digests. It never holds a handle, a credential, a body, a header, a query or a URL.

## 1. Shape

A receipt record has one of four actions, an `actor_kind` of the component that authored it, outcome `succeeded`, no `inputs`, no `lifecycle` and no `corrects_record_hash`. The layout is the record's own; `canon_version` is unchanged. Its facts are `outputs`. A marker output is `{"ref": "receipt/<key>/<value>", "sha256": sha256 of the ref text}`; the value is made of `A-Z a-z 0-9 . _ -` (a request line also has `/` between its fields).

| Action | `actor_kind` | Record fields | Outputs |
|---|---|---|---|
| `HandleIssued.v1` | `broker` | `grant_id` required, `policy_decision_id` optional | markers `epoch`, `number`, `handle`, `invocation`, `intent`, `delivery`, `expires`; then up to 256 `receipt/scope/<i>` outputs, `i` from 0, whose `sha256` is the digest of the i-th scope line |
| `HandleRevoked.v1` | `broker` | `grant_id` required | markers `epoch`, `number`, `handle`, `invocation`, `reason` |
| `NetworkRequests.v1` | `egress-proxy` | none | marker `epoch`, then 1 to 256 outputs `receipt/request/<n>/<allow\|deny>/<rule>/<class>/<port>/<step>/<dest>/<status>/<bytes>/<handle or none>`, with the optional eleventh field `/injected` (added in 1.1.0, section 1a) |
| `ReceiptsOverflow.v1` | `broker` or `egress-proxy` | none | markers `overflow` (`ledger_full`, `chain_cap`, `component_restarted`, `component_started` or `denied_overflow`), `epoch`, `first`, `last`, `dropped` |

`epoch` is the instant the component began recording, in Unix nanoseconds, a canonical positive integer (a token, not a JSON number). `number` and the request numbers are the component's numbers for its receipts of the change under that epoch, from 1; a record states its numbers in ascending order. `handle` is a handle ref `nhref_` and 12 lowercase hex characters, the digest prefix of the handle and never the handle. `rule` and `class` are 1 to 32 bytes of `[a-z_]`. The port of a request the allow-list refused is `0`: like the host, the sandboxed agent chooses it, and it is not recorded. `dest` is a host the manifest allow-list admitted, or `h-` and 16 hex characters (the digest of any other host: the host of a refused request is chosen by the sandboxed agent and can carry data, and a chain cannot be cleaned; an unsalted 64-bit hash only confirms a hostname someone already guessed). A request line is the `NETWORK_REQUEST` fact of the design (host, status and bytes, never bodies), read as above for "host".

### 1a. The `injected` marker on a request line (added in 1.1.0)

A request line has ten fields, or eleven when the eleventh is the literal `injected`:

```
receipt/request/<n>/<allow|deny>/<rule>/<class>/<port>/<step>/<dest>/<status>/<bytes>/<handle or none>[/injected]
```

- A ten-field line is unchanged, byte for byte, and is the only form of a request on which the proxy selected no rule-bound injection. There is no token for "not injected": one request has one encoding, so every record written before 1.1.0 keeps its bytes, its `record_hash` and its `ev_id`.
- Any eleventh token other than `injected` (an empty one and `Injected` included) and any twelfth field make the record `record_invalid`.
- `injected` is valid only when the handle field is a handle ref. `injected` with `none` is `record_invalid`, on an `allow` line and on a `deny` line.
- `injected` may stand on an `allow` line and on a `deny` line.

What it states: the egress proxy selected a rule-bound injection for this request, so a credential the sandbox did not hold was attached to the outgoing request if one was sent. On an `allow` line the request was forwarded with it. On a `deny` line neither the rule, the status nor the marker says whether any byte reached the upstream: a refusal after the proxy selected the injection (the address judgement, the body read, the byte quota, the upstream, an incomplete upload, a withdrawal) keeps the handle ref and the marker. It never states that the upstream accepted the credential. It names no handle value, header, encoding, value, path or repository, and it adds no free text: it is one bit.

What its absence does not state: a line without the marker does not say that no credential was attached. A handle ref without the marker means the proxy attached that handle's credential on a route that is not rule-bound.

Version: this is a minor of the specification (1.1.0, additive; `canon_version` is unchanged, and the line is a string of an output ref, so the record layout and hashes are untouched). A verifier of spec 1.1.0 or newer is needed for an export whose lines carry `injected`. An older verifier takes exactly ten fields, fails the record with `record_invalid` and so fails the export; it does not ignore or mis-parse the line. That is the intended fail-closed behaviour. A new verifier reads every older export unchanged.

Who sets it: only the egress proxy, from the route's configured injection when it selects the injection, never from request content. Like every receipt it proves what the runner signed (section 2).

A scope line is the line of the broker's scope statement (`<handle ref> <kind>:<value>`). A reader holding the attestation's `credential_scopes` lines checks that the digest of each line is the `sha256` of one `receipt/scope/<i>` output of a `HandleIssued.v1` record of the same handle.

For `ledger_full` and `chain_cap`, `dropped` is 1 or more and at most `last - first + 1`. For `component_restarted`, `first` and `last` are equal (the next number the chain expected from the old instance), `dropped` is `0`, and `epoch` is the new instance. For `component_started`, the baseline, `first` and `last` are both `1`, `dropped` is `0` and `epoch` is the instance the component was in when the chain first heard of it for the change. For `denied_overflow`, `first` and `last` are equal (the proxy's last number then) and `dropped` is the count, 1 or more, of policy denials the proxy counted and did not store.

## 2. Rules

A record with a receipt action and an `actor_kind` other than the component kinds of its row, or a component kind and another action, is `record_invalid`. So are an output that is not under `receipt/`, a marker whose digest is not the digest of its ref, a repeated or missing key, and a value outside its grammar. A request line outside the grammar of sections 1 and 1a is `record_invalid` (rule 12 of [records and chain](records-and-chain.md)): a field count other than ten or eleven, an eleventh field that is not `injected`, and `injected` with the handle `none`. No new finding code is added. A verifier checks the shape of each record; it does not check that the numbers are continuous across records, and it does not yet derive the attestation's `credential_scopes` or `network_policy` from the receipts. An `actor_kind` a verifier does not know fails the record; after the first tag a new actor kind is contract-breaking for verifiers.

What a receipt record proves is what the runner signed. The checks that only the runner appends it, for the configured component, are the writer's, in its process: they are no cryptographic boundary against a compromised writer, which holds the runner key and can sign any well-formed receipt and leave real ones out. That is the trust the design gives the runner for every record; a sign-only or KMS signer is a later decision.

## 3. Bounds, overflow and restarts

A component never acts without a receipt. It keeps at most 4096 receipts per change and reserves a slot before it acts: the broker refuses to issue a handle and the proxy denies a request when no slot is free, and the refusal is counted. `ledger_full` therefore counts only refused events, never ones that happened. A chain holds at most 8192 receipt records: 8190 ordinary ones, and one `chain_cap` statement per component in two reserved slots. A `chain_cap` statement says the component's receipts from `first` on are not written, and writing it stops the Change (the writer trips the circuit breaker). The first time the chain hears of a component for a change, with or without a receipt pending, a `component_started` baseline names its epoch, so a later change of epoch is seen from the chain alone. A component whose lists are process-lifetime state numbers from 1 again after a restart under a new `epoch`; a `component_restarted` statement marks the point, and the receipts the old instance had not handed over are lost. A proxy stores at most 1024 policy denials per change; the ones past that are counted and stated as `denied_overflow`, so a flood of refused requests does not take the slots of the requests that go on and the count of denials stays whole. A reader of the chain sees, for each component and epoch, its receipts, or an explicit statement of a restart, a refusal count, or a cap. A request still in flight when the last pull before the terminal record runs is recorded when its response ends and is then not in the chain.
