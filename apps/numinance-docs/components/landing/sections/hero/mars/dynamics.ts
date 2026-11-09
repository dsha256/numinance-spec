/** Second-order dynamics and orbital mechanics for the Mars hero experiment. Pure, allocation-free. */

export const TAU = Math.PI * 2;
export const clamp = (v: number, a = 0, b = 1) => (v < a ? a : v > b ? b : v);
export const smooth = (v: number) => ((v = clamp(v)), v * v * (3 - 2 * v));
export const mix = (a: number, b: number, t: number) => a + (b - a) * t;
export const wrapPi = (a: number) => ((((a + Math.PI) % TAU) + TAU) % TAU) - Math.PI;
/** longest frame we integrate: a tab that slept resumes with a bounded step, never a leap */
export const DT_MAX = 0.064;

/** state of a critically damped scalar: x'' + 2w x' + w^2 (x - target) = 0 */
export interface Spring {
  x: number;
  v: number;
}
export const spring = (x = 0, v = 0): Spring => ({ x, v });

/**
 * Exact (analytic) step of the critically damped oscillator, unconditionally stable for any dt:
 *   a = x - target;  x' = target + (a + (v + w a) dt) e^{-w dt};  v' = (v - w (v + w a) dt) e^{-w dt}.
 * Position AND velocity are carried, so retargeting mid-flight never snaps.
 */
export function stepSpring(s: Spring, target: number, omega: number, dtSec: number) {
  const dt = Math.min(dtSec, DT_MAX);
  const e = Math.exp(-omega * dt);
  const a = s.x - target;
  const j = s.v + omega * a;
  s.x = target + (a + j * dt) * e;
  s.v = (s.v - omega * j * dt) * e;
}

/** like stepSpring but takes the shortest way round a circle of period `period` */
export function stepAngle(s: Spring, target: number, omega: number, dtSec: number, period = TAU) {
  const d = ((((target - s.x) % period) + period * 1.5) % period) - period / 2;
  stepSpring(s, s.x + d, omega, dtSec);
}

// ---- Kepler -----------------------------------------------------------------------------------

/** E - e sin E = M by Newton iterations (bounded; e < 0.9) */
export function solveKepler(M: number, e: number): number {
  M = ((M % TAU) + TAU) % TAU;
  let E = e < 0.8 ? M : Math.PI;
  for (let i = 0; i < 8; i++) {
    const f = E - e * Math.sin(E) - M;
    const d = f / (1 - e * Math.cos(E));
    E -= d;
    if (Math.abs(d) < 1e-9) break;
  }
  return E;
}

export interface Orbit {
  /** semi-major axis, in sphere radii */
  a: number;
  e: number;
  /** inclination, longitude of node, argument of periapsis (rad) */
  incl: number;
  node: number;
  peri: number;
  /** mean anomaly at t = 0 and mean motion (rad/s) */
  m0: number;
  n: number;
}

export interface Camera {
  sp: number;
  cp: number;
  sr: number;
  cr: number;
}
/** orthographic view from slightly above the equatorial plane, rolled in the screen plane */
export const camera = (pitch: number, roll: number): Camera => ({
  sp: Math.sin(pitch),
  cp: Math.cos(pitch),
  sr: Math.sin(roll),
  cr: Math.cos(roll),
});

export interface Proj {
  x: number;
  y: number;
  /** toward the viewer: > 0 is in front of the sphere's centre plane */
  z: number;
}

/**
 * Point on the Kepler ellipse at eccentric anomaly E (focus at the sphere's centre), as a scene
 * vector in sphere radii: plane coords -> Ry(peri) -> Rx(incl) -> Ry(node).
 */
export function orbitVec(o: Orbit, E: number, out: [number, number, number]) {
  const b = o.a * Math.sqrt(1 - o.e * o.e);
  const xp = o.a * (Math.cos(E) - o.e);
  const zp = b * Math.sin(E);
  const cw = Math.cos(o.peri),
    sw = Math.sin(o.peri);
  const x1 = xp * cw + zp * sw;
  const z1 = -xp * sw + zp * cw;
  const ci = Math.cos(o.incl),
    si = Math.sin(o.incl);
  const y2 = -z1 * si;
  const z2 = z1 * ci;
  const cn = Math.cos(o.node),
    sn = Math.sin(o.node);
  out[0] = x1 * cn + z2 * sn;
  out[1] = y2;
  out[2] = -x1 * sn + z2 * cn;
}

/** camera: pitch about X (Y up), then roll in the screen plane; scene units -> pixels about (cx, cy) */
export function project(v: readonly [number, number, number], cam: Camera, cx: number, cy: number, R: number, out: Proj) {
  const y1 = v[1] * cam.cp - v[2] * cam.sp;
  const z1 = v[1] * cam.sp + v[2] * cam.cp;
  const sx = v[0];
  const sy = -y1; // screen y grows downward
  out.x = cx + (sx * cam.cr - sy * cam.sr) * R;
  out.y = cy + (sx * cam.sr + sy * cam.cr) * R;
  out.z = z1;
}

/** on the sphere's disk and behind its centre plane: hidden by the sphere */
export const occluded = (p: Proj, cx: number, cy: number, R: number) => p.z < 0 && Math.hypot(p.x - cx, p.y - cy) < R;

// ---- sphere surface ---------------------------------------------------------------------------

/** unit vector in the body frame (+y north) for latitude / longitude (radians) */
export function lonLat(lon: number, lat: number, out: [number, number, number]) {
  const c = Math.cos(lat);
  out[0] = c * Math.sin(lon);
  out[1] = Math.sin(lat);
  out[2] = c * Math.cos(lon);
}

/** spherical linear interpolation of unit vectors (a great circle) */
export function slerp(a: readonly number[], b: readonly number[], t: number, out: [number, number, number]) {
  const d = clamp(a[0] * b[0] + a[1] * b[1] + a[2] * b[2], -1, 1);
  const om = Math.acos(d);
  const so = Math.sin(om);
  if (so < 1e-5) {
    out[0] = a[0];
    out[1] = a[1];
    out[2] = a[2];
    return;
  }
  const wa = Math.sin((1 - t) * om) / so;
  const wb = Math.sin(t * om) / so;
  out[0] = a[0] * wa + b[0] * wb;
  out[1] = a[1] * wa + b[1] * wb;
  out[2] = a[2] * wa + b[2] * wb;
}

/** angular distance between unit vectors */
export const angle = (a: readonly number[], b: readonly number[]) => Math.acos(clamp(a[0] * b[0] + a[1] * b[1] + a[2] * b[2], -1, 1));

export interface SphereFrame {
  /** body spin about its polar axis, in turns */
  rot: number;
  /** tilt of the polar axis in the screen plane (same constant the halftone tables use) */
  ca: number;
  sa: number;
}

/**
 * Body-frame unit vector -> view-space unit normal (x right, y down, z toward the viewer),
 * exactly the inverse of the per-pixel lookup the halftone tables use (so features stay glued).
 */
export function bodyToView(b: readonly number[], f: SphereFrame, out: [number, number, number]) {
  const th = f.rot * TAU;
  const ct = Math.cos(th),
    st = Math.sin(th);
  const x = b[0] * ct + b[2] * st;
  const z = -b[0] * st + b[2] * ct;
  const mx = x,
    my = -b[1];
  out[0] = mx * f.ca - my * f.sa;
  out[1] = mx * f.sa + my * f.ca;
  out[2] = z;
}
