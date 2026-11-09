# Route performance

Status: normative for `evidence/v1` from spec 1.1.0 (the unreleased minor that also carries `postgres_transport`). Read [canonical form](canonical-form.md) and [bundle layout](bundle-layout.md) first.

A router may order the candidates of a route by the figures it has measured. When it does, the figures it read are part of the evidence, so a reader can see what the order was based on and can tell a route that moved from one that did not. This page fixes the record input that names those figures, the document it names, and the file that carries it in an export.

## 1. Terms

- **Candidate.** An executor and one of its models, as the router's customer-owned policy lists them. The list is in policy order, and that order is authoritative.
- **Rank.** A positive integer on each candidate of the list. Equal Rank means the policy treats the candidates as interchangeable; Ranks do not decrease along the list, and the members of one Rank are next to each other. A candidate with no Rank written takes its 1-based position in the list, so a list that sets no Rank has only groups of one. The router may exchange the places of candidates only inside one group.
- **Performance snapshot.** What the router read once for one route, before it ranked: the measured figures of the strata of one task class and language, the position of the observations they fold (`as_of`), and the `n` they were counted against. A figure is `accepted` of `n` comparable outcomes. A candidate with no figure has no data.

The format records what the router read. It does not define how the router ranks.

## 2. The record input

A record that carries the input `execution/considered` (the record of a route that ran, the denied record of a route no candidate met the floor for, and the refusal that came before any policy decision) MAY carry one more input:

| Input ref | sha256 |
|---|---|
| `execution/performance` | the sha256 of the canonical form of the snapshot document (section 3) |

The writer includes the input exactly when some group of the list has two or more members and the snapshot holds at least one valid figure for a member of such a group, whether or not any candidate moved. Otherwise the input is absent, and the record is what it was before this page existed. A figure is valid when its `n` is positive and its `accepted` is at most its `n`.

A verifier does not decide whether the input should have been written: it reads the records as they are. It binds every digest a record names to a document in the export (section 4).

## 3. The snapshot document

A canonical JSON object ([canonical form](canonical-form.md)):

| Key | Type | Meaning |
|---|---|---|
| `as_of` | integer, 0 to 2^53-1 | the position of the observations the figures fold |
| `n` | integer, 1 to 4294967295 | the outcomes a candidate needs before its figure may move it |
| `task_class` | string | the task class of the strata; empty when unknown; 0 to 256 bytes |
| `language` | string | the language of the strata; same rules |
| `candidates` | array, at most 256 | the whole list before ranking, in policy order |

Each candidate is an object `{executor, model, rank, figure?}`: `executor` and `model` are 1 to 256 bytes, `rank` is 1 to 4294967295 and is the normalized Rank of section 1, and `figure` is `{accepted, n}` (each 0 to 4294967295, `n` at least 1, `accepted` at most `n`) and is absent when the snapshot holds no valid figure for the candidate. All text is printable UTF-8 without a leading or trailing space ([records](records-and-chain.md) rule 3). The list is the full list the policy gave, not only the candidates the router walked.

When a record names a snapshot, its `execution/considered` digest changes in the same case: the considered line of a candidate carries the suffix ` rank=<n>` when any two candidates of the full normalized list share a Rank, and carries nothing otherwise. A verifier does not re-derive the considered digest (the executor streams and their routes are the host's statements, [receipts](receipts.md)).

## 4. The file

`provenance/route-performance.json` is one canonical object `{"export_version": 1, "snapshots": [ ... ]}`. Each element of `snapshots` is a snapshot document of section 3. An exporter writes the elements sorted by their digest. The schema is `route-performance.schema.json`.

A verifier applies these rules to the distinct digests that the `execution/performance` inputs of `records.ndjson` name (the input is read on every record, whatever its action):

- no digest named and the file present: `digest_mismatch`;
- one or more digests named and the file absent: `part_missing`;
- the file present and not a canonical object of the shape above (an unknown key, a wrong type, an out-of-range value, a figure with more accepted outcomes than outcomes, white space or a different key order): `digest_mismatch`;
- the sha256 of the canonical form of two elements the same: `digest_mismatch`;
- an element whose sha256 no record names (an unnamed entry): `digest_mismatch`;
- a digest a record names for which no element has that sha256: `part_missing`.

The file is in the directory `provenance/`, so it is covered by the part `provenance/` and is listed in the index and the manifest like any other file. A verifier does not check the order of the elements or of the candidates, and does not check that the figures are true or that the ranks follow the policy: the digests in the signed records bind the documents, and the documents are the router's statements.

## 5. What it proves

The check proves that the export carries, for each snapshot a record names, the document whose digest the signed record holds, and nothing else. It does not prove the figures are correct, that the router ranked by them, or that the candidate list is the customer's policy. A verifier older than 1.1.0 does not check this binding: it refuses the file as `unknown_file`, so an export that carries it fails closed there ([verification](verification.md) section 9, item 10).
