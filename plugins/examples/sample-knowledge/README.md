# sample-knowledge

A sample capability provider of `numinance.ports.knowledge.v1`: three READ verbs (`Search`, `Fetch`, `Resolve`) over a
small seeded in-memory source. It is written against the public SDK only (`sdk/go/plugin`, the generated stubs) and has
no vendor code. It is the provider a plugin host launches in its acceptance test and the target of the knowledge
conformance kit (`conformance/knowledge/v1`).

- The seeded source follows the kit README: scope `kit` with `doc-a` (two versions) and `doc-b`; scope `other` with
  `doc-o`, which matches the query of the kit's first case and is never returned for scope `kit`.
- The manifest declares a real content digest (the sha256 of the running executable); the SBOM and provenance
  references are placeholders of a sample, which the extensions context would refuse at install.
- Run by hand the binary refuses (wrong magic cookie): it is started by the host.

Build: `go build ./plugins/examples/sample-knowledge` from `numinance-spec/`. Test: `go test ./plugins/examples/...`.
