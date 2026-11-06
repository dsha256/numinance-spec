# Log leaves, Merkle rules and checkpoints

Status: normative for `evidence/v1` (spec 1.0.0). Read [canonical form](canonical-form.md) first.

A transparency log commits the head of each chain, and each attestation, so that an export cannot be replaced wholesale by a second consistent export. The log is a Merkle tree over leaves (RFC 6962 hashing). A verifier needs no network access: the export carries a signed checkpoint and an inclusion proof, and the verifier holds the log's key and origin.

## 1. Leaves

A leaf is exactly 33 bytes: one kind byte followed by the 32 raw bytes of a hash.

| Kind byte | Name | Hash |
|---|---|---|
| `0x48` | head leaf | `head_record_hash`: the `record_hash` of the head record (see section 5) |
| `0x41` | attestation leaf | `sha256(canonical(attestation with signature removed))`, the attestation's signing digest |

A verifier MUST refuse any other length or kind.

## 2. Tree hashing

```
leaf_hash(leaf)    = sha256( 0x00 || leaf )
node_hash(l, r)    = sha256( 0x01 || l || r )
MTH([leaf_hash])   = the single leaf hash
MTH(n >= 2 leaves) = node_hash( MTH(first k), MTH(rest) )   with k the largest power of two below n
```

This is the Merkle Tree Hash of RFC 6962 section 2.1 and RFC 9162 section 2.1. `l`, `r` and the leaf hashes are the 32 raw bytes, not text. An empty log has no root and no checkpoint.

## 3. Inclusion proof

An inclusion proof is the JSON object `{"leaf_index": i, "tree_size": n, "audit_path": [digest, ...]}` (schema `inclusion-proof.schema.json`). `audit_path` is absent when empty. Each entry is a digest.

Shape rules, all of which a verifier MUST check before it computes anything:

- `tree_size` is 1 to 2^53-1, `leaf_index` is below `tree_size`.
- The path has exactly the length that `(leaf_index, tree_size)` imply: with `inner = bit_length(leaf_index XOR (tree_size - 1))`, the length is `inner + popcount(leaf_index >> inner)`. It has no empty entry and at most 64 entries (a 65th is refused while the proof is read).
- The encoded proof is at most 8192 bytes and is canonical.

Root recomputation (RFC 9162 section 2.1.3.2 and RFC 6962 audit path rules):

```
fn = leaf_index; sn = tree_size - 1; r = leaf_hash(leaf)
for p in audit_path:
    if sn == 0: fail
    if fn is odd or fn == sn:
        r = node_hash(p, r)
        while fn is even and fn != 0: fn >>= 1; sn >>= 1
    else:
        r = node_hash(r, p)
    fn >>= 1; sn >>= 1
pass only if sn == 0 and r == checkpoint.root_hash
```

## 4. Checkpoint

A checkpoint (`head-checkpoint.json`, `attestation-checkpoint.json`; schema `checkpoint.schema.json`) is the canonical JSON object:

| Key | Type | Presence | Rule |
|---|---|---|---|
| `origin` | string | required | the log's name: 1 to 256 bytes of printable UTF-8, no leading or trailing space |
| `tree_size` | integer | required | 1 to 2^53-1 |
| `root_hash` | digest | required | `MTH` over the first `tree_size` leaves |
| `timestamp` | string | required | RFC 3339 UTC |
| `canon_version` | integer | required | 1 |
| `signature` | base64 | required | 64 bytes, Ed25519 by the log key |
| `signing_key_id` | digest | required | the key id of the log key, inside the signed digest |

The signature is `Ed25519(log_key, "numinance.evidence.v1.Checkpoint" || sha256(canonical(checkpoint with signature removed)))`. The encoded checkpoint is at most 4096 bytes. An export holds only checkpoints that the Numinance ledger signed; a checkpoint with a `signed_note` (the form of a public log such as Rekor) is a different form of the same message, is not part of `evidence/v1` export verification, and is refused by the decoder of a stored checkpoint.

Verifying a leaf under a checkpoint (`InclusionProof.Verify`), in this order:

1. The checkpoint signature verifies under the pinned log key, and `signing_key_id` is that key's id.
2. `checkpoint.origin` equals the pinned log origin, byte for byte.
3. The leaf is a valid 33-byte leaf and `proof.tree_size` equals `checkpoint.tree_size`.
4. The proof shape rules of section 3 hold.
5. The recomputed root equals `checkpoint.root_hash`.

Any failure is a failed proof. A verifier that holds no pinned log key and origin neither decodes nor checks the two anchor files: it only requires that they are present as a pair (a lone checkpoint or proof is `anchor_invalid`) and, for a Closed change, that the manifest names the checkpoint by digest. Anchor files that are garbage are therefore accepted without a pin, and the head is reported as not anchored (see [verification](verification.md)).

The log key and the log origin are trusted only through the caller. A checkpoint, a key or an origin that the export itself states proves nothing.

## 5. Which hash a head leaf holds

The head leaf of a chain holds the `record_hash` of the last record of the chain, except for a sealed chain. A sealed chain ends with a seal record, and the head leaf is committed before the seal record exists, so the head leaf holds the `record_hash` of the terminal record that the seal record follows (the record at `last_sequence` of the attestation). This is the "anchored hash" of the chain:

```
anchored_hash(chain) = record_hash of record (n-1)   if the chain is sealed (n records)
                     = record_hash of record n        otherwise
```

An unsealed chain that ended Closed also has the terminal record as its last record, so the two cases agree there.

## 6. Where proofs appear in an export

| File or field | Leaf | Checkpoint |
|---|---|---|
| `head-inclusion-proof.json` | head leaf of `anchored_hash` | `head-checkpoint.json` |
| `attestation.json` field `inclusion_proof` | head leaf of the attestation's `head_record_hash` | `attestation-checkpoint.json`, named by the attestation's `checkpoint_ref` |
| `bundle-manifest.json` field `inclusion_proof` | the same head leaf; MUST equal `head-inclusion-proof.json` | `head-checkpoint.json`, named by the manifest's `checkpoint_ref` |
| `bundle-manifest.json` field `attestation_inclusion_proof` | the attestation leaf | `head-checkpoint.json` |

The head checkpoint is at or after the checkpoint that holds the attestation leaf; the attestation checkpoint is the per-change checkpoint taken before the attestation was signed.

## 7. Consistency proofs

The log port also defines consistency proofs between two checkpoints (RFC 6962 section 2.1.2). They are used between the log and the runner, to refuse a log that rewrites history. An export of `evidence/v1` carries none, and a verifier is not required to check any.
