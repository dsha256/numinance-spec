import type { StatusKind } from './status';

/**
 * Source-controlled capability readiness matrix. This is the single place to update when a
 * release artifact appears; /docs/status, the CapabilityMatrix diagram and the SDK pages read it.
 * Paths are relative to the root of the public numinance-spec repository.
 */
export interface Capability {
  id: string;
  name: string;
  group: 'Open CLI' | 'Protocol' | 'SDKs' | 'Evidence' | 'Integrations' | 'Runtime' | 'Distribution';
  state: StatusKind;
  surface: string;
  howToTry: string;
  source?: string;
  next: string;
}

export const CAPABILITIES: Capability[] = [
  {
    id: 'cli-version', name: 'numi version', group: 'Open CLI', state: 'public-spec',
    surface: 'Open build of numi', howToTry: 'go run ./cli/numi version', source: 'cli/numi/main.go',
    next: 'Tagged releases',
  },
  {
    id: 'cli-init', name: 'numi init', group: 'Open CLI', state: 'public-spec',
    surface: 'Scaffolds .numinance/ for a Python repository', howToTry: 'go run ./cli/numi init --ci github-actions <repo>',
    source: 'cli/README.md', next: 'More languages and SCM providers',
  },
  {
    id: 'cli-validate-files', name: 'numi validate --files', group: 'Open CLI', state: 'public-spec',
    surface: 'Strict schema + loader check of kind documents', howToTry: 'go run ./cli/numi validate --files schemas/examples',
    source: 'cli/README.md', next: 'None: profile resolution lives in the engine build',
  },
  {
    id: 'cli-validate', name: 'numi validate (repository)', group: 'Open CLI', state: 'public-spec',
    surface: 'Schema pass, then exit 7: profile resolution needs the engine build', howToTry: 'go run ./cli/numi validate <repo>',
    source: 'cli/README.md', next: 'Full resolution ships only in the private engine build',
  },
  {
    id: 'cli-evidence-verify', name: 'numi evidence verify', group: 'Evidence', state: 'public-spec',
    surface: 'Offline verifier over an evidence/v1 export', howToTry: 'go run ./cli/numi evidence verify --runner-key … --log-key … --log-origin … DIR',
    source: 'sdk/go/evidenceverify/', next: 'Engine build delegates to the same package',
  },
  {
    id: 'cli-run', name: 'numi run / connect / plan / approve / doctor', group: 'Open CLI', state: 'designed',
    surface: 'Designed command surface of the unified CLI', howToTry: 'Not in the open build', source: undefined,
    next: 'Engine build; interface subject to change',
  },
  {
    id: 'cli-evidence-export', name: 'numi evidence export', group: 'Open CLI', state: 'designed',
    surface: 'Engine-side export of an evidence bundle', howToTry: 'Unknown subcommand in the open build (exit 2)', source: 'cli/README.md',
    next: 'Engine build',
  },
  {
    id: 'proto', name: 'Protobuf contracts', group: 'Protocol', state: 'pre-alpha',
    surface: 'PluginService, HostService, ports, annotations, kinds', howToTry: 'Read proto/numinance/', source: 'proto/numinance/',
    next: 'First tagged release (see VERSIONING.md)',
  },
  {
    id: 'schemas', name: 'JSON Schemas and kind examples', group: 'Protocol', state: 'public-spec',
    surface: 'Generated kind schemas and schemas/examples/', howToTry: 'numi validate --files schemas/examples', source: 'schemas/',
    next: 'Additional kinds as they are accepted',
  },
  {
    id: 'sdk-go', name: 'Go SDK', group: 'SDKs', state: 'pre-alpha',
    surface: 'Handwritten plugin, kindyaml, schema, cli, evidenceverify + generated stubs', howToTry: 'go test ./sdk/go/...', source: 'sdk/go/',
    next: 'Tagged module release',
  },
  {
    id: 'sdk-python', name: 'Python SDK', group: 'SDKs', state: 'pre-alpha',
    surface: 'Generated Protobuf/gRPC stubs (version 0.0.0)', howToTry: 'Generate from source; not a verified PyPI package', source: 'sdk/python/',
    next: 'Ergonomic plugin SDK; publication when verified',
  },
  {
    id: 'sdk-ts', name: 'TypeScript SDK', group: 'SDKs', state: 'pre-alpha',
    surface: 'Generated protobuf-es code, private package at 0.0.0', howToTry: 'Build from source; not a verified npm package', source: 'sdk/typescript/',
    next: 'Ergonomic plugin SDK; publication when verified',
  },
  {
    id: 'sdk-rust', name: 'Rust SDK', group: 'SDKs', state: 'coming-soon',
    surface: 'No source distribution yet', howToTry: 'Generate clients from the .proto files with your own toolchain', source: undefined,
    next: 'Roadmap; no date, crate name or version promised',
  },
  {
    id: 'sdk-java', name: 'Java SDK', group: 'SDKs', state: 'coming-soon',
    surface: 'No source distribution yet', howToTry: 'Generate clients from the .proto files with your own toolchain', source: undefined,
    next: 'Roadmap; no date, coordinates or version promised',
  },
  {
    id: 'evidence-spec', name: 'evidence/v1 specification + conformance vectors', group: 'Evidence', state: 'public-spec',
    surface: 'docs/evidence/, schemas/evidence/v1/, conformance/evidence/v1/', howToTry: 'Verify a golden export offline', source: 'docs/evidence/',
    next: 'Independent third-party implementations',
  },
  {
    id: 'plugin-sample', name: 'Sample knowledge plugin', group: 'Integrations', state: 'public-spec',
    surface: 'A host-launched Go provider of numinance.ports.knowledge.v1', howToTry: 'go build ./plugins/examples/sample-knowledge', source: 'plugins/examples/sample-knowledge/',
    next: 'Conformance replay wiring',
  },
  {
    id: 'plugin-official', name: 'Official providers (GitHub SCM, GitHub Actions, verifiers)', group: 'Integrations', state: 'pre-alpha',
    surface: 'Source and tests under plugins/official/', howToTry: 'go test ./plugins/official/...', source: 'plugins/official/',
    next: 'Live-run verification; presence is not a readiness claim',
  },
  {
    id: 'engine', name: 'Numinance engine (autonomous runtime)', group: 'Runtime', state: 'designed',
    surface: 'Private, proprietary; not generally available', howToTry: 'Not installable', source: undefined,
    next: 'Not scheduled publicly',
  },
  {
    id: 'managed', name: 'Managed and sovereign deployment', group: 'Runtime', state: 'designed',
    surface: 'Customer data plane + thin vendor control plane', howToTry: 'Not installable', source: undefined, next: 'Design partners (pre-alpha)',
  },
  {
    id: 'airgap', name: 'Air-gapped deployment', group: 'Runtime', state: 'coming-soon',
    surface: 'Later design milestone', howToTry: 'Not available', source: undefined, next: 'After managed and sovereign',
  },
  {
    id: 'dist-source', name: 'GitHub source build of the open CLI', group: 'Distribution', state: 'public-spec',
    surface: 'git clone + go run / go build', howToTry: 'See Install', source: 'go.mod', next: 'Tagged releases',
  },
  {
    id: 'dist-releases', name: 'GitHub Releases binaries', group: 'Distribution', state: 'planned',
    surface: 'Signed binaries with checksums', howToTry: 'Not published; no command given', source: undefined, next: 'Verify release artifacts first',
  },
  {
    id: 'dist-brew', name: 'Homebrew', group: 'Distribution', state: 'planned',
    surface: 'Tap/formula', howToTry: 'Not published; no command given', source: undefined, next: 'Verify formula first',
  },
];
