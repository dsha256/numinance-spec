# evidencestore conformance kit, v1

Skeleton. The acceptance tests for a provider of `numinance.ports.evidencestore.v1.EvidenceStoreService` (`proto/numinance/ports/evidencestore/v1/evidencestore.proto`). The kit version tracks the port minor; this skeleton tracks 1.0.0. The runnable kit (replay fixtures, live kit, harness) is added by the plugin host work package; until then this file is the list of cases the kit must hold. Modes are those of ADR 0015: replay, live publisher-run in an attested runner, live registry-run.

Every case runs against an empty store with a fresh `change_id` of the form `chg_` plus 32 lowercase hex characters, and sets up its own records. Objects are opaque bytes; the kit never needs a valid evidence record, only bytes and their sha256. Each create above sequence 1 carries the stored predecessor's digest as `previous_content_sha256`, unless a case says otherwise. The seal of a Change is its last record, an ordinary create; the provider cannot tell it from another record, so no case below is about seals. "Bounds" below means each listed input alone, every other field valid. "Concurrent" means released together by a start barrier after all requests are built.

## Core

| Id | Verb | Case | Expect |
| --- | --- | --- | --- |
| CORE-001 | AppendLocked | create at sequence 1, then GetRecord | OK, `replayed` false, `content_sha256` of the bytes; the same bytes and digest come back |
| CORE-002 | AppendLocked | the same bytes again at the same key | OK, `replayed` true, the same `content_sha256`; nothing written |
| CORE-003 | AppendLocked | other bytes at a taken key | ALREADY_EXISTS; GetRecord returns the first bytes |
| CORE-004 | AppendLocked | `content_sha256` absent; malformed (`sha256:` plus 63 hex, uppercase hex, another algorithm); the digest of other bytes | INVALID_ARGUMENT for each; nothing stored |
| CORE-005 | AppendLocked | 16 concurrent creates of different bytes at one key | exactly one OK, the rest ALREADY_EXISTS; the stored bytes are the winner's |
| CORE-006 | AppendLocked | 16 concurrent creates of the same bytes at one key | all OK, exactly one with `replayed` false; one object stored |
| CORE-007 | AppendLocked | bounds: `change_id` empty, `chg_` plus 31 hex, uppercase hex, another prefix; sequence 0 and 2^53; content empty and of 3145729 bytes; `previous_content_sha256` absent at sequence 2 with record 1 stored, malformed, or set at sequence 1 | INVALID_ARGUMENT for each; nothing stored, Head unchanged |
| CORE-008 | AppendLocked | sequence 2 with sequence 1 absent (a well-formed `previous_content_sha256`); sequence 2 with a `previous_content_sha256` other than record 1's | FAILED_PRECONDITION for each; nothing stored |
| CORE-009 | AppendLocked | sequence 2 with sequence 1 absent and a wrong `content_sha256` | INVALID_ARGUMENT (the integrity rule comes before the order rule) |
| CORE-010 | AppendLocked | identical bytes at a taken key with a malformed `content_sha256`; the same at sequence 2 with `previous_content_sha256` absent or malformed; the same at sequence 3 with `content_sha256` absent or malformed | INVALID_ARGUMENT for each, not a replay |
| CORE-011 | AppendLocked | records 1 to 3 stored: a create at sequence 5; the bytes of record 3 again at sequence 3; other bytes at sequence 3 | FAILED_PRECONDITION, Head unchanged; OK with `replayed` true; ALREADY_EXISTS, Head unchanged |
| CORE-012 | AppendLocked | a create, then at once Head and ListRange over its sequence | both return the new record |
| CORE-013 | GetRecord | a sequence never written | NOT_FOUND |
| CORE-014 | GetRecord | bounds: `change_id` as in CORE-007; sequence 0 and 2^53 | INVALID_ARGUMENT for each |
| CORE-015 | ListRange | records 1 to 5 stored, span 2 to 4 | sequences 2, 3, 4 in order, bytes as stored |
| CORE-016 | ListRange | records 1 to 3 stored, span 3 to 10; span 4 to 10 | sequence 3 only; then OK with no records |
| CORE-017 | ListRange | bounds: span of 257 sequences; to below from; from 0; to of 2^53; `change_id` as in CORE-007 | INVALID_ARGUMENT for each |
| CORE-018 | ListRange | records 1 to 3 of 1572864 bytes (1.5 MiB) each, span 1 to 3 | a page of sequences 1 and 2 (3145728 bytes); span 3 to 3 returns sequence 3 |
| CORE-019 | ListRange | record 1 of 3145728 bytes, record 2 of 1 byte, span 1 to 2 | a page of sequence 1 only; span 2 to 2 returns sequence 2 |
| CORE-020 | Head | no records | sequence 0, `head_record` unset |
| CORE-021 | Head | records 1 to 3 stored | sequence 3, `head_record` equal to record 3 |
| CORE-022 | Head | bounds: `change_id` as in CORE-007 | INVALID_ARGUMENT for each |
| CORE-023 | AppendLocked, Head | records 1 to 3 stored, a record 4 created last (the seal record of the evidence context), then Head | sequence 4, `head_record` equal to record 4 |
| CORE-024 | AppendLocked | one round of many contenders: 16 concurrent creates of different bytes at sequence 4 after records 1 to 3 | exactly one OK, the rest ALREADY_EXISTS; Head returns the winner's record |
| CORE-025 | AppendLocked | many rounds of the smallest race, which a single round of CORE-024 can miss: records 1 to 3 stored, then a create at sequence 4 and a create at sequence 4 of other bytes, concurrent, repeated 100 times with fresh Change ids | in each round exactly one OK and the other ALREADY_EXISTS; Head shows sequence 4 |
| CORE-026 to CORE-029 | retired | cases of the removed Seal verb (the seal is an ordinary record, CORE-023) | the ids are never reused |
| CORE-030 | all | one store, two Change ids | no verb on one Change returns an object of the other |
| CORE-031 | Head | record 1 stored: creates at sequences 2 to 64 in order while Head is called repeatedly and concurrently | every Head response has `head_record` equal to the bytes GetRecord returns at its `sequence` (one snapshot) |
| CORE-032 | ListRange | an empty store, a span of exactly 256 sequences | OK with no records |

## Extended

None in 1.0.0.

## Not covered by this kit

- Chain rules are checked by the evidence context, not the provider (see the file comment of the proto). These are sequence continuity in content, `previous_record_hash`, signatures, terminal records, the seal record, which follows the terminal record, and that nothing follows it, the requested key against the decoded record, and page linkage.
- The fenced single-writer lease per Change, the Head and checkpoint freshness checks and the read-back of Head after each create, the seal record included, are evidence-context duties. A failure of any of them is an evidence-write failure that trips the circuit breaker.
- DATA_LOSS needs stored bytes corrupted beneath the provider, so the provider's deployment test covers it.
- Write-once on the storage medium itself (Object Lock and its equivalents) is also checked by the provider's deployment test, not by a protocol case.
