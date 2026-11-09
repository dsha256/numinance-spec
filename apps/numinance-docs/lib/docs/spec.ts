/**
 * Public specification provenance. Every technical page cites paths inside the public
 * `numinance-spec` repository (Apache-2.0). Bump SPEC_REVISION only after re-verifying the
 * commands and snippets that cite it (see scripts/ and the "Verified" labels in the MDX).
 */
export const SPEC_REPO = 'https://github.com/dsha256/numinance-spec';
/** Commit of numinance-spec the docs were last verified against. */
export const SPEC_REVISION = '3c74d594f670d8e17e93d519020ef8feae268480';
export const SPEC_REVISION_SHORT = SPEC_REVISION.slice(0, 7);
export const SPEC_VERIFIED_ON = '2026-11-09';
/** Go module path declared by numinance-spec/go.mod (NOT the GitHub path). */
export const SPEC_GO_MODULE = 'github.com/numinance/numinance-spec';

/** Link to a file or directory in numinance-spec at the verified revision. */
export function specUrl(path: string): string {
  const clean = path.replace(/^\/+/, '');
  const isDir = clean.endsWith('/');
  return `${SPEC_REPO}/${isDir ? 'tree' : 'blob'}/${SPEC_REVISION_SHORT}/${clean.replace(/\/$/, '')}`;
}
