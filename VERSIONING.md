# Versioning policy

Identifiers carry only the major (`numinance.ports.ciexecutor.v1`, `apiVersion: numinance.dev/v1`); releases carry x.y.z.

| Artifact | MAJOR when | MINOR | PATCH |
|---|---|---|---|
| Wire protocol `numinance.plugin.vN` | RPC or field removed/renamed; semantic change | new RPC/field behind a protocol capability | docs |
| Port `numinance.ports.<p>.vN` | any `buf breaking` FILE break; Core verb change; risk lowered | new capability, optional field, enum value | docs |
| Config kind `numinance.dev/vN` | field removed/renamed/retyped; default changed | optional field with `since` | messages |
| Plugin | drops a port/protocol major, capability or config field; widens permissions | new capability or action | fix |
| Pack / policy / playbook | new deny; component removed | additions; equal or looser decisions | fix |

Minors are additive and every addition carries `since`. Pre-GA: `v1alpha1` = `1.0.0-alpha.1` (off by default), `v1beta1` = `1.0.0-beta.1` (on by default). Unknown YAML fields are errors except under a reverse-DNS `extensions:` map; unknown wire fields are preserved.

Before the first tag, nothing is released: untagged SDK packages (for example `sdk/go/cli`) may change in breaking ways, and the PR declares it. After the first tag the table above applies.

Compatibility: the Engine serves protocol majors N and N-1 for 24 months after N+1 GA; port majors M and M-1 for 12 months via shims; CI tests official plugins against Engine N, N-1, N-2. Deprecations ship with `deprecated_in`/`replaced_by`, a `DEPRECATED` diagnostic, a codemod and a conformance marker. Windows: GA 12 months or 3 Engine minors; beta 9 months; alpha none.

Lockfiles pin version, digest, installed-tree hashes (`zh` and `h1`) and signer identity. Permission widening always requires re-approval.
