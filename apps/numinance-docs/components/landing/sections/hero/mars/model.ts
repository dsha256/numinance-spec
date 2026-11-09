import { mulberry32 } from '@/lib/landing/rng';
import type { Orbit } from './dynamics';

/** Data and timing for the illustrative Mars sequence. Everything here is a simulation. */

export interface SourceDef {
  id: 'repository' | 'architecture' | 'docs' | 'knowledge' | 'graph';
  label: string;
  /** receipt chip once accepted */
  receipt: string;
  /** landing site on the visible hemisphere at activation: view-frame longitude / latitude (rad) */
  lon: number;
  lat: number;
  orbit: Orbit;
  version: string;
}

const o = (a: number, e: number, incl: number, node: number, peri: number, m0: number, n = 1.25): Orbit => ({ a, e, incl, node, peri, m0, n });

/** five authentic source categories; inclined Kepler ellipses, each approaching from its own side */
export const SOURCES: readonly SourceDef[] = [
  { id: 'repository', label: 'REPOSITORY', receipt: 'repo docs / indexed', lon: -0.5, lat: 0.3, version: 'v12', orbit: o(1.1, 0.3, 0.95, 0.4, 0.2, 2.9) },
  { id: 'architecture', label: 'ARCHITECTURE', receipt: 'architecture / matched', lon: 0.3, lat: 0.42, version: 'v3', orbit: o(1.14, 0.28, -0.7, 2.2, 1.0, 3.2) },
  { id: 'docs', label: 'DOCS', receipt: 'docs / fetched', lon: 0.62, lat: -0.12, version: 'v7', orbit: o(1.08, 0.32, 1.2, 4.0, 0.6, 2.7) },
  { id: 'knowledge', label: 'AUTHORIZED KNOWLEDGE', receipt: 'approved source / fetched', lon: 0.1, lat: -0.46, version: 'v2', orbit: o(1.12, 0.26, 0.5, 5.0, 2.1, 3.3) },
  { id: 'graph', label: 'CODE GRAPH', receipt: 'code graph / indexed', lon: -0.46, lat: -0.22, version: 'v12', orbit: o(1.06, 0.3, -1.1, 1.2, 0.3, 3.0) },
];
export const STALE_INDEX = 1;
export const STALE_NOTE = 'v3 / v5 current';

/** demonstration digests: deterministic, obviously not real hashes */
const hex = (seed: number) => {
  const r = mulberry32(seed);
  let s = '';
  for (let i = 0; i < 4; i++) s += Math.floor(r() * 16).toString(16);
  return s;
};
export const DIGESTS = SOURCES.map((_, i) => hex(7000 + i * 31));
export const REFETCH_DIGEST = hex(9191);
export const BUNDLE_DIGEST = hex(4242);

// ---- timeline (ms at 1x, per state clock) ------------------------------------------------------
export const BOOT_MS = 650;
export const SPAWN_STEP = 230;
export const CAPTURE_AT = 480; // after a source's spawn: the descent begins
export const CAPTURE_MS = 820;
export const RETRIEVE_MS = 4 * SPAWN_STEP + CAPTURE_AT + CAPTURE_MS + 200;
export const acceptedAt = (i: number) => i * SPAWN_STEP + CAPTURE_AT + CAPTURE_MS;

export const CHECKS = ['provenance', 'permission basis', 'freshness', 'dedupe / redaction', 'conflicts', 'blocking questions'] as const;
export const CHECK_MS = 420;
export const FRESH = 2;
/** [start, end] of each check on the validation clock. After a re-fetch the freshness check is replaced by the re-fetch itself. */
export function checkWindow(i: number, resumed: boolean): [number, number] {
  if (!resumed) return [i * CHECK_MS, (i + 1) * CHECK_MS];
  if (i < FRESH) return [-CHECK_MS, 0];
  if (i === FRESH) return [0, 900];
  const s = 900 + (i - FRESH - 1) * 380;
  return [s, s + 380];
}
export const validateEnd = (resumed: boolean) => checkWindow(CHECKS.length - 1, resumed)[1] + 170;
/** the freshness check ends here on a first pass: the stale source is found */
export const STALE_AT = CHECK_MS * (FRESH + 1);

/** autoplay driver: how long a flagged stale source stays visible before the runtime re-fetches it by itself */
export const AUTO_REFETCH_MS = 1000;
/** CONTEXT READY is its own beat (ready is not authority), then the external policy is evaluated */
export const READY_MS = 800;
/** the policy verdict ("permits this scope, no per-change approval") stays readable before the run begins */
export const PERMIT_MS = 1500;

export const POLICY_RING_MS = 700;
export const POLICY_MS = 1400;

export const SEQ = {
  morph: 1700,
  /** start / end of each waypoint on the sequence clock */
  steps: [
    { id: 'understand', at: 300, to: 1500 },
    { id: 'plan', at: 1500, to: 2800 },
    { id: 'act', at: 2800, to: 4300 },
    { id: 'verify', at: 4300, to: 5700 },
    { id: 'release', at: 5700, to: 6600 },
    { id: 'evidence', at: 6600, to: 7800 },
  ],
  total: 7800,
} as const;

/** the five public waypoints (ring nodes 0-4) plus the two separate identities (5, 6) */
export const NODE_TIPS = [
  { label: 'UNDERSTAND', tip: (d: string) => `understand / bundle ref ${d} / sim` },
  { label: 'PLAN', tip: () => 'plan / scope bounded to 3 paths' },
  { label: 'ACT', tip: () => 'act / grant: paths + tools + env' },
  { label: 'VERIFY', tip: () => 'verify / independent reviewer' },
  { label: 'EVIDENCE', tip: () => 'evidence / hash-linked, illustrative' },
  { label: 'REVIEW', tip: () => 'review / read-only identity, not the author' },
  { label: 'RELEASE', tip: () => 'release / pipeline identity deploys' },
] as const;

/** the review identity's own small orbit and the release identity's slot on the external policy ring */
export const REVIEW_ORBIT: Orbit = { a: 1.16, e: 0.12, incl: 1.3, node: 0.9, peri: 0.4, m0: 1.0, n: -0.55 };
export const POLICY_RING = { a: 1.3, incl: 0.62, node: 0.12, roll: 0.0 };
