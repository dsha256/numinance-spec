# annotationlint

Checks that every RPC and field of the Numinance protos carries its governance annotations (`numinance/annotations/v1/annotations.proto`, ADR 0012). Buf's built-in rules cannot require a custom option, so `just lint` runs `cmd/annotationlint` on `buf build --exclude-imports --as-file-descriptor-set`. Standard library and protobuf-go only. Output: `file:line:column:RULE message`; exit 0 clean, 1 on violations, 2 on a usage or read error; input capped at 64 MiB.

| Rule | Fails when |
| --- | --- |
| `ACTION_ANNOTATION_REQUIRED` | an RPC has no `(numinance.annotations.v1.action)` option |
| `ACTION_RISK_REQUIRED` / `ACTION_EXEC_REQUIRED` | `risk` / `exec` is the UNSPECIFIED zero value |
| `ACTION_SINCE_REQUIRED` | the action's `since` is not a strict `x.y.z` (no leading zeros, prefix, pre-release or build) |
| `ACTION_LLM_DESCRIPTION_REQUIRED` | `llm_description` is empty or blank |
| `ACTION_SCOPE_INVALID` | an `action.scopes` entry is not a scope name of `authority.Grant.Validate` (`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`, at most 256 bytes) or repeats another entry; the shared case table `testdata/scope_grammar.json` is also read by the kernel's `scope_grammar_mirror_test.go` |
| `FIELD_SINCE_REQUIRED` | a field or extension has no strict `x.y.z` `(numinance.annotations.v1.field).since` |
| `HOST_ONLY_INVALID` | `host_only` is set on anything but a `numinance.types.v1.Reference` field, or a message-typed wrapper that reaches one in the set, of a message that an RPC input of the set reaches, in a `numinance.ports.*` file (RFC-0001) |
| `RESOURCE_NAME_INVALID` | `resource_name` is set on anything but a `string` or `repeated string` field of a message that an RPC input of the set reaches, in a `numinance.ports.*` file (RFC-0002) |
| `FIELD_OPTION_DUPLICATE` | a field has more than one `(field)` entry, or more than one `host_only` entry in one; found on the image decoded with an empty resolver, because a runtime merges them and a later `false` would strip the mark (RFC-0001). `buf` cannot emit such an image from source, so only the Go test (`TestLint_DuplicateOptions`) covers it, not `fixture.golden` |

Not checked: `idempotent` and `supports_dry_run` (proto3 bools without presence: an explicit `false` is not serialized); empty `evidence` (PluginService RPCs declare none); compiler-synthesized map entry messages (the map field is checked); the two extension declarations in `annotations.proto` itself (`action`, `field`; that file only, not its package), which the file states are since 1.0.0.

Tests: `go test ./sdk/go/internal/annotationlint/...`, and `tools/scripts/test-annotation-lint.sh` (in `just hooks-test`), which builds `testdata/fixture.proto` with buf in a scratch module and compares the output with `testdata/fixture.golden`.
