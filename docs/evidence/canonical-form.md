# Canonical form, digests and signatures

Status: normative for `evidence/v1` (spec 1.0.0). "MUST", "MUST NOT" and "MAY" have their usual meaning. This page records the form the shipped evidence context writes today. It does not change it.

Every digest and every signature in an export is computed over bytes. This page says which bytes. A verifier that cannot reproduce these bytes exactly cannot verify an export.

## 1. Digests

- A digest is the text `sha256:` followed by 64 lowercase hexadecimal characters. Uppercase hexadecimal is not a digest.
- `sha256` is SHA-256 (FIPS 180-4) over the bytes named by the rule that uses it.
- A key id (`signing_key_id`, `key_id`) is the digest of the raw 32-byte Ed25519 public key, not of its text form.

## 2. Canonical JSON

The canonical form of a document is a UTF-8 byte string. It is what Go's `encoding/json` writes for the document's typed model, rewritten as follows. It is deterministic. It is not RFC 8785 (JCS): the key order and the number rules differ.

1. Objects have their members sorted by key, comparing the key's UTF-8 bytes. (JCS sorts by UTF-16 code units; the two orders differ only for keys with supplementary-plane characters, and no evidence key has one.)
2. There is no whitespace outside strings and no trailing newline.
3. Strings are written as `encoding/json` writes them with HTML escaping off:
   - The quote and the backslash are written as backslash-quote and backslash-backslash.
   - Line feed, carriage return and tab are written as backslash-`n`, `r` and `t`. Backspace and form feed are written as backslash-`b` and backslash-`f` (Go 1.22 and later). Every other control character below U+0020 is written as backslash, `u` and four lowercase hexadecimal digits.
   - U+2028 and U+2029 are written as backslash, `u`, `2028` or `2029`.
   - The characters `<`, `>` and `&` are written as themselves. An export that writes them as backslash-`u003c` and the like is not canonical.
   - Every other character, including every non-ASCII character, is written as its UTF-8 bytes. DEL (U+007F) is written as itself.
   - A string that is not valid UTF-8 would be written with U+FFFD in place of each bad byte. No evidence field may contain one: producers and verifiers refuse such a field before it is hashed, so two byte strings never digest alike.
4. Numbers in evidence documents are integers: written in decimal, no sign for unsigned fields, no leading zero, no fraction, no exponent. `sequence`, `tree_size`, `leaf_index`, `first_sequence`, `last_sequence`, `row_version`, `canon_version`, `records`, `denied`, `size` and the other counters are such numbers. A document that writes `1.0`, `1e0` or `01` for one is refused. Every counter a producer writes is at most 2^53-1, so that a verifier that reads numbers as floating point stays exact, and `sequence`, `tree_size`, `leaf_index`, `row_version` and the attestation and manifest sequences are refused above it. Two counters are only unsigned 64-bit when decoded: the `size` of an index or manifest entry and `records` and `denied` of a summary; a value above 2^53-1 there decodes and then never equals a real file size or record count (`digest_mismatch`, `summary_mismatch`).
5. Booleans are `true` and `false`. `null` is never written.
6. A field that is empty is absent. An empty string, an empty array, a zero digest, a zero identifier, a zero enum and an absent optional object are all written as no key. A document that writes `"inputs":[]` or `"executor_id":""` is not canonical. (The exceptions are fields that are always written, which each document's table marks as required: they can hold an empty string or a zero number only where the document's rules allow it.)
7. Enums are written as their names, as strings (`"READ"`, `"runner"`, `"succeeded"`). A number never stands for an enum.
8. Bytes (`signature`, `public_key`) are written as standard base64 with padding (RFC 4648 section 4). A 64-byte signature is 88 characters ending in `==`. A 32-byte public key is 44 characters ending in `=`.
9. Timestamps are RFC 3339 in UTC with the suffix `Z`, one second resolution, and a fraction only when the time is not a whole second. The fraction has 1 to 9 digits and no trailing zero: `2026-11-01T12:00:00Z`, `2026-11-01T12:00:02.5Z`, `2026-11-01T12:00:03.000000001Z`. A numeric offset (`+00:00`), a fraction with a trailing zero (`.500Z`) or a fraction of a whole second (`.0Z`) is not canonical. The year is 1 to 9999.

## 3. Strict decoding

A verifier MUST read a JSON document of the format with the strict procedure below, and MUST treat any failure as the document being invalid. The shipped decoders do exactly this for every document except `tool-events.ndjson`, whose lines are digested as raw bytes (see [derivations](derivations.md)).

1. Read at most the document's size bound (the bound is part of each document's description). A document over its bound is refused before it is parsed.
2. The text MUST be one JSON object followed by nothing. Trailing data is refused. (A record line is one object; its line feed is not part of it.)
3. At every object level, a key MUST NOT repeat, MUST NOT be unknown for that object, and MUST be spelled exactly (a key that matches a known key only when case is ignored is refused). This is stricter than JSON: a last-wins reader and a first-wins reader would otherwise digest two different documents for one byte string.
4. Values MUST have the type the document defines. A number where a string is expected, a fraction where an integer is expected and a string that does not parse as its type (a digest, an identifier, an enum name, a timestamp) are refused.
5. Arrays that have a bound are checked while they are read: an array over its bound is refused at the first entry past the bound, without decoding the rest.
6. The decoded model MUST pass the document's field rules (each page lists them).
7. The decoded model, written in canonical form, MUST equal the input byte for byte. This is the step that refuses key order, whitespace, escapes, number forms, timestamp forms, empty-but-present fields and every other non-canonical spelling at once.

Nesting deeper than 64 levels is refused. No evidence document nests more than five levels.

## 4. Record hash and record signature

For a record `r` (see [records and chain](records-and-chain.md)):

```
record_hash = sha256( canonical(r with record_hash and signature removed) )
```

`signing_key_id` stays in the hashed form: it is set before hashing and never changes after.

```
signing_digest   = sha256( canonical(r with signature removed) )     (record_hash is present)
context          = the UTF-8 bytes of the Record context name in section 5
signature        = Ed25519( runner_key, context || signing_digest_bytes )
```

where `||` is plain concatenation of the context's UTF-8 bytes and the 32 raw bytes of the digest, with no separator and no length prefix. The signature therefore binds the chain fields (`sequence`, `previous_record_hash`, `record_hash`) as well as the content. A verifier MUST first re-derive `record_hash` from the record and compare, and only then check the signature against the key `signing_key_id` names.

## 5. Signing contexts

The signing input of every signed message is the message's fully qualified name followed by the raw digest of its canonical form with the signature field removed. The contexts are:

| Message | Context | Signed by | `canon_version` |
|---|---|---|---|
| Record | `numinance.evidence.v1.Record` | runner key | 2 |
| Attestation | `numinance.evidence.v1.Attestation` | runner key | 1 |
| BundleManifest | `numinance.evidence.v1.BundleManifest` | runner key | 1 |
| Checkpoint | `numinance.evidence.v1.Checkpoint` | log key | 1 |

For an Attestation, a BundleManifest and a Checkpoint the digest is `sha256(canonical(message with signature removed))` and `signing_key_id` stays in the hashed form. A message signed under one context never verifies under another: the context is inside the signed bytes.

A verifier MUST accept only `canon_version` values it knows. In `evidence/v1` a record carries 2 (version 1 plus the optional `lifecycle` object); the other three messages carry 1. Any other value is refused.

## 6. Key material

- Signatures are Ed25519 (RFC 8032). The public key is 32 bytes, the signature 64 bytes.
- A signature that is not 64 bytes, or a public key that is not 32 bytes, fails verification.
- A key file read by `numi evidence verify` holds the 32-byte public key as 64 hexadecimal characters, with optional surrounding whitespace, in at most 1024 bytes.
- The bundle's `provenance/signers.json` lists public keys. It is a claim. It is never a trust anchor: see [verification](verification.md), "Key modes".
